package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type uploadMemory struct {
	clip.Store
	clip.GenerationStore
	project clip.Project
	rows    map[string]clip.UploadText
	quotes  map[string]clip.UploadTextQuote
}

func (s *uploadMemory) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	if user != s.project.UserID || id != s.project.ID {
		return clip.Project{}, clip.ErrNotFound
	}
	return s.project, nil
}
func (s *uploadMemory) ListUploadTexts(ctx context.Context, user, id string) ([]clip.UploadText, error) {
	if _, e := s.GetProject(ctx, user, id); e != nil {
		return nil, e
	}
	out := []clip.UploadText{}
	for _, v := range s.rows {
		out = append(out, v)
	}
	return out, nil
}
func (s *uploadMemory) SaveUploadText(_ context.Context, in clip.UploadTextSave) (clip.UploadText, error) {
	row := s.rows[in.Channel]
	if row.Revision != in.ExpectedRevision {
		return row, clip.ErrPlanConflict
	}
	row = clip.UploadText{ProjectID: in.ProjectID, UserID: in.UserID, Channel: in.Channel, Body: in.Body, Tags: in.Tags, Revision: row.Revision + 1, Edited: in.Edited, LastJobID: in.JobID, UpdatedAt: in.Now}
	s.rows[in.Channel] = row
	return row, nil
}
func (s *uploadMemory) SaveUploadTextQuote(_ context.Context, q clip.UploadTextQuote, _ time.Time) error {
	s.quotes[q.ID] = q
	return nil
}
func (s *uploadMemory) GetUploadTextQuote(_ context.Context, user, id string) (clip.UploadTextQuote, error) {
	q, ok := s.quotes[id]
	if !ok || q.UserID != user {
		return q, clip.ErrNotFound
	}
	return q, nil
}
func (s *uploadMemory) LinkUploadTextJob(_ context.Context, q clip.UploadTextQuote, id string, _ clip.UploadTextSnapshot, _ time.Time) error {
	q.ConsumedJobID = id
	s.quotes[q.ID] = q
	return nil
}

type uploadHeldKey struct{}
type uploadJobs struct {
	clip.GenerationJobs
	jobs  map[string]*clip.ClipJob
	holds int
}

func (j *uploadJobs) Enqueue(_ context.Context, in clip.GenerationStart) (string, error) {
	id := fmt.Sprintf("job-%d", len(j.jobs)+1)
	j.jobs[id] = &clip.ClipJob{ID: id, Kind: in.Kind, Status: "queued", Payload: append([]byte{}, in.Payload...)}
	return id, nil
}
func (j *uploadJobs) Activate(_ context.Context, _ string, id string) error {
	j.jobs[id].DispatchReady = true
	return nil
}
func (j *uploadJobs) Active(context.Context, string, string) (*clip.ClipJob, error) {
	for _, v := range j.jobs {
		if v.Status == "queued" || v.Status == "running" {
			return v, nil
		}
	}
	return nil, nil
}
func (j *uploadJobs) Get(_ context.Context, _ string, id string) (*clip.ClipJob, error) {
	v := j.jobs[id]
	if v == nil {
		return nil, nil
	}
	return &clip.ClipJob{ID: id, Status: v.Status}, nil
}
func (j *uploadJobs) Snapshot(_ context.Context, user, project, id string) (*clip.ClipJob, error) {
	if user != "alice" || project != "project" {
		return nil, clip.ErrNotFound
	}
	v := j.jobs[id]
	if v == nil {
		return nil, clip.ErrNotFound
	}
	copy := *v
	return &copy, nil
}
func (j *uploadJobs) FailQueued(_ context.Context, _ string, id string) (bool, error) {
	j.jobs[id].Status = "failed"
	return true, nil
}
func (j *uploadJobs) ReserveApproved(ctx context.Context, _ string, _ string, a clip.GenerationApproval, chunks int) (context.Context, error) {
	calls, r, e := PlanReservation(a, chunks)
	if e != nil {
		return nil, e
	}
	if len(calls) != 1 || calls[0].Count != 1 || calls[0].Stage != "write" || len(r.Calls) != 1 {
		return nil, clip.ErrCreditAllowance
	}
	j.holds++
	return context.WithValue(ctx, uploadHeldKey{}, true), nil
}

type uploadVoice struct{ text string }

func (v *uploadVoice) DefaultClipVoice(context.Context, string, string) (string, error) {
	return v.text, nil
}

type uploadWriter struct {
	calls int
	input clip.UploadTextInput
}

func (w *uploadWriter) ValidateUploadText(context.Context, llm.ModelRef, clip.UploadTextInput) error {
	return nil
}
func (w *uploadWriter) WriteUploadText(ctx context.Context, _ llm.ModelRef, in clip.UploadTextInput) (clip.UploadText, llm.Usage, error) {
	if ctx.Value(uploadHeldKey{}) != true {
		return clip.UploadText{}, llm.Usage{}, clip.ErrCreditAllowance
	}
	w.calls++
	w.input = in
	return clip.UploadText{Body: "내 방문 기록", Tags: []string{"방문"}}, llm.Usage{}, nil
}

type uploadFinisher struct {
	clip.ClipFinisher
	rows *uploadMemory
	jobs *uploadJobs
}

func (f uploadFinisher) CompleteUploadText(ctx context.Context, in clip.UploadTextSave) (clip.UploadText, error) {
	row, e := f.rows.SaveUploadText(ctx, in)
	if e == nil {
		f.jobs.jobs[in.JobID].Status = "done"
	}
	return row, e
}
func uploadFixture(t *testing.T) (*GenerationService, *uploadMemory, *uploadJobs, *uploadWriter, *uploadVoice, *fakeFreezer) {
	t.Helper()
	raw, e := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "expired-original", Fingerprint: "fp", EndMS: 15000, Copies: []clip.Copy{{Text: "내가 쓴 자막"}}}}})
	if e != nil {
		t.Fatal(e)
	}
	store := &uploadMemory{project: clip.Project{ID: "project", UserID: "alice", Language: "ko", Channel: clip.ChannelNaver, Title: "내 방문", Instruction: "겪은 그대로", Campaign: true, AdvertiserGuide: clip.AdvertiserGuide{Text: "광고주의 안내", Hashtags: []string{"필수"}}, EditPlan: raw, EditPlanRevision: 1}, rows: map[string]clip.UploadText{}, quotes: map[string]clip.UploadTextQuote{}}
	jobs := &uploadJobs{jobs: map[string]*clip.ClipJob{}}
	writer := &uploadWriter{}
	voice := &uploadVoice{"짧은 기본 말투"}
	freezer := &fakeFreezer{}
	pricing := NewPricing(freezer, Budgets{FlowCompletionTokens: 32768}, &changingRates{rate: testQuoteRate})
	service := &GenerationService{store: store, projects: &Service{store: store}, jobs: jobs, uploadWriter: writer, uploadVoice: voice, pricing: pricing, cfg: clip.GenerationConfig{QuoteTTL: time.Minute, CleanupTimeout: time.Second}, now: time.Now, finisher: uploadFinisher{rows: store, jobs: jobs}}
	return service, store, jobs, writer, voice, freezer
}
func uploadStart(t *testing.T, s *GenerationService, channel string, confirmed bool) (string, clip.UploadTextQuote) {
	t.Helper()
	q, e := s.QuoteUploadText(t.Context(), "alice", "project", channel, "p/w", confirmed)
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.StartUploadText(t.Context(), "alice", "project", channel, "p/w", confirmed, clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits, CancellationPolicyVersion: 1})
	if e != nil {
		t.Fatal(e)
	}
	return id, q
}
func runUpload(t *testing.T, s *GenerationService, jobs *uploadJobs, id string) {
	t.Helper()
	j := jobs.jobs[id]
	j.Status = "running"
	j.Stage = "prepare"
	if e := s.RunUploadText(t.Context(), "alice", id, "project", j.Payload, nil); e != nil {
		t.Fatal(e)
	}
}

func TestUploadTextIsOneQuotedCall(t *testing.T) {
	s, store, jobs, writer, voice, _ := uploadFixture(t)
	id, q := uploadStart(t, s, clip.ChannelNaver, false)
	if q.Pricing.PlanCalls() != 1 || q.Pricing.ObservationCalls != 0 || q.Pricing.MaxCredits <= 0 || jobs.holds != 0 || writer.calls != 0 {
		t.Fatal("quote/start did paid work", q.Pricing, jobs.holds, writer.calls)
	}
	var frozen clip.UploadTextSnapshot
	if e := json.Unmarshal(jobs.jobs[id].Payload, &frozen); e != nil {
		t.Fatal(e)
	}
	voice.text = "새 말투"
	runUpload(t, s, jobs, id)
	if writer.calls != 1 || jobs.holds != 1 || writer.input.Voice != "짧은 기본 말투" || writer.input.AdvertiserGuide.Text != "광고주의 안내" || writer.input.Captions[0] != "내가 쓴 자막" {
		t.Fatal("wrong frozen input or call order", writer.input, jobs.holds, writer.calls)
	}
	if store.rows[clip.ChannelNaver].Edited || store.rows[clip.ChannelNaver].Tags[0] != "필수" {
		t.Fatal(store.rows)
	}
	duplicate, e := s.StartUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "p/w", false, clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits, CancellationPolicyVersion: 1})
	if e != nil || duplicate != id || writer.calls != 1 {
		t.Fatal("approval was consumed twice", duplicate, e)
	}
}
func TestTheOtherChannelNeedsNoRender(t *testing.T) {
	s, store, jobs, writer, _, _ := uploadFixture(t)
	store.project.Finalized = &clip.Finalization{}
	before := store.project.EditPlan
	for _, channel := range []string{clip.ChannelNaver, clip.ChannelInstagram} {
		id, _ := uploadStart(t, s, channel, false)
		runUpload(t, s, jobs, id)
	}
	if len(store.rows) != 2 || writer.calls != 2 || store.project.EditPlan != before || store.project.EditPlanRevision != 1 || store.project.Finalized == nil {
		t.Fatal("upload text changed the finished video", store.rows, writer.calls)
	}
	// Media, source and render collaborators are all nil in this fixture.
}
func TestEditedTextNeedsConfirmation(t *testing.T) {
	s, store, jobs, writer, _, _ := uploadFixture(t)
	if _, e := s.SaveUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "내가 고친 문구", nil, 0); e != nil {
		t.Fatal(e)
	}
	if _, e := s.QuoteUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "p/w", false); !errors.Is(e, clip.ErrUploadTextConfirmation) {
		t.Fatal("manual edit was overwritten without confirmation", e)
	}
	if writer.calls != 0 || jobs.holds != 0 || store.rows[clip.ChannelNaver].Body != "내가 고친 문구" {
		t.Fatal("confirmation refusal changed text")
	}
	id, _ := uploadStart(t, s, clip.ChannelNaver, true)
	runUpload(t, s, jobs, id)
	if store.rows[clip.ChannelNaver].Revision != 2 || store.rows[clip.ChannelNaver].Edited {
		t.Fatal(store.rows)
	}
}
func TestImpossibleRequiredHashtagsRefuseBeforeQuoteAndCredits(t *testing.T) {
	s, store, jobs, writer, _, freezer := uploadFixture(t)
	store.project.AdvertiserGuide.Hashtags = []string{"a", "b", "c", "d", "e", "f"}
	if _, e := s.QuoteUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "p/w", false); !errors.Is(e, clip.ErrUploadTextLimit) {
		t.Fatal(e)
	}
	if len(freezer.calls) != 0 || len(store.quotes) != 0 || writer.calls != 0 || jobs.holds != 0 {
		t.Fatal("impossible guide did work")
	}
}
