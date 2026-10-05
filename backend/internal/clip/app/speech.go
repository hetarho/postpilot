package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

const SpeechJournalTimeout = 10 * time.Second

// These ports expose owned behavior, never the reusable voice context's tables.
type SpeechVoice = clip.SpeechVoice
type SpeechVoices interface {
	ResolveSpeechVoice(context.Context, string, plan.Plan, string) (SpeechVoice, error)
}
type SpeechPrices interface {
	Budget(context.Context, string, plan.Plan, string, int64, string, string, llm.SpeechInput, int) (usage.UnitBudget, error)
}
type SpeechLedger interface {
	QuoteUnits(context.Context, string, plan.Plan, string, []usage.UnitBudget) (usage.UnitQuote, error)
	ReservationForUnits(context.Context, string, plan.Plan, string, usage.UnitApproval, []usage.UnitBudget) (*usage.Reservation, error)
	AdmissionForJob(context.Context, string) (usage.Admission, bool, error)
}
type SpeechPlanReader interface {
	PlanOf(context.Context, string) (plan.Plan, error)
}
type SpeechObjects interface {
	PutClipSpeechAudio(context.Context, string, []byte) error
	DeleteClipSpeechAudio(context.Context, string) error
}
type SpeechCall = clip.SpeechCall
type SpeechRun = clip.SpeechRun
type SpeechStorage interface {
	ListIncompleteSpeechRuns(context.Context) ([]SpeechRun, error)
	GetProject(context.Context, string, string) (clip.Project, error)
	ReserveSpeechRun(context.Context, SpeechRun) (SpeechRun, error)
	FindSpeechRun(context.Context, string, string, string) (SpeechRun, error)
	GetSpeechRun(context.Context, string, string) (SpeechRun, error)
	BindSpeechRun(context.Context, string, string, string) error
	ClaimSpeechCall(context.Context, SpeechRun, SpeechCall) error
	FinishSpeechCall(context.Context, SpeechRun, SpeechCall, string, string) error
	InsertSpeechAsset(context.Context, clip.SpeechAsset) error
	FindSpeechAsset(context.Context, string, string, string, string) (clip.SpeechAsset, error)
	PublishSpeechAsset(context.Context, string, string, string, int, string, string, string, string) (clip.Project, error)
}
type SpeechJobMetadata interface {
	GetByID(context.Context, string) (job.Job, error)
	AuthorizeDispatch(context.Context, string, string) error
	Finish(context.Context, string, string, *job.Failure, time.Time) error
}
type SpeechTxPorts struct {
	Store SpeechStorage
	Jobs  SpeechJobMetadata
}
type SpeechTransactions interface {
	WriteSpeech(context.Context, func(SpeechTxPorts) error) error
}
type SpeechDeps struct {
	Store        SpeechStorage
	Voices       SpeechVoices
	Plans        SpeechPlanReader
	Prices       SpeechPrices
	Ledger       SpeechLedger
	Models       llm.SpeechSynthesizer
	Objects      SpeechObjects
	Queue        Queue
	Transactions SpeechTransactions
}
type SpeechService struct {
	SpeechDeps
	playbackMu sync.Mutex
	playback   map[string]speechPlayback
}

func NewSpeechService(d SpeechDeps) *SpeechService {
	if d.Store == nil || d.Voices == nil || d.Plans == nil || d.Prices == nil || d.Ledger == nil || d.Models == nil || d.Objects == nil || d.Queue == nil || d.Transactions == nil {
		panic("clip speech dependencies required")
	}
	return &SpeechService{SpeechDeps: d, playback: make(map[string]speechPlayback)}
}
func speechID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}

type SpeechQuote struct {
	ID             string
	MaximumCredits int
	ExpiresAt      time.Time
	SegmentIDs     []string
	Revision       int
}

func (s *SpeechService) prepare(ctx context.Context, owner, project string, revision int) (SpeechRun, plan.Plan, error) {
	p, e := s.Store.GetProject(ctx, owner, project)
	if e != nil {
		return SpeechRun{}, "", e
	}
	if p.Finalized != nil {
		return SpeechRun{}, "", clip.ErrFinalized
	}
	if p.EditPlanRevision != revision {
		return SpeechRun{}, "", clip.ErrPlanConflict
	}
	edit, e := clip.DecodeEditPlan(p.EditPlan)
	if e != nil {
		return SpeechRun{}, "", e
	}
	if e = clip.ValidateNarration(edit); e != nil {
		return SpeechRun{}, "", e
	}
	n := edit.Narration
	if n == nil || !n.Enabled || n.VoiceID == "" || len(n.Segments) == 0 {
		return SpeechRun{}, "", clip.ErrInvalid
	}
	tier, e := s.Plans.PlanOf(ctx, owner)
	if e != nil {
		return SpeechRun{}, "", e
	}
	r := SpeechRun{OwnerID: owner, ProjectID: project, Revision: revision}
	// Stored compatible speech remains playable after voice removal. Only new work resolves a voice.
	for _, seg := range n.Segments {
		if !clip.CompatibleSpeech(n, seg) {
			r.Calls = append(r.Calls, SpeechCall{SegmentID: seg.ID, Text: seg.Text, InputHash: seg.InputHash})
		}
	}
	if len(r.Calls) == 0 {
		return r, tier, nil
	}
	v, e := s.Voices.ResolveSpeechVoice(ctx, owner, tier, n.VoiceID)
	if e != nil {
		return r, tier, e
	}
	if v.Binding.ID != n.VoiceID || v.Binding.Digest != n.BindingDigest {
		return r, tier, clip.ErrPlanConflict
	}
	r.Voice = v
	parts := []string{"clip-speech-v1", owner, project, strconv.Itoa(revision), n.BindingDigest}
	for i := range r.Calls {
		c := &r.Calls[i]
		c.Request = llm.SpeechRequest{Model: v.Model, Voice: v.Handle, Text: c.Text, Settings: v.Settings}
		in, e := c.Request.Input()
		if e != nil {
			return r, tier, e
		}
		parts = append(parts, c.SegmentID, in.Digest)
	}
	r.ScopeDigest = usage.UnitDigest(parts...)
	for i := range r.Calls {
		c := &r.Calls[i]
		in, _ := c.Request.Input()
		c.Budget, e = s.Prices.Budget(ctx, owner, tier, v.ProfileID, v.ProfileRevision, "", r.ScopeDigest, in, 1)
		if e != nil {
			return r, tier, e
		}
	}
	return r, tier, nil
}
func speechBudgets(r SpeechRun) []usage.UnitBudget {
	bs := make([]usage.UnitBudget, 0, len(r.Calls))
	for _, c := range r.Calls {
		found := false
		for i := range bs {
			single := bs[i]
			single.Count = 1
			if single.Fingerprint() == c.Budget.Fingerprint() {
				bs[i].Count++
				found = true
				break
			}
		}
		if !found {
			bs = append(bs, c.Budget)
		}
	}
	return bs
}
func (s *SpeechService) Quote(ctx context.Context, owner, project string, revision int) (SpeechQuote, error) {
	r, tier, e := s.prepare(ctx, owner, project, revision)
	if e != nil {
		return SpeechQuote{}, e
	}
	q := SpeechQuote{Revision: revision}
	for _, c := range r.Calls {
		q.SegmentIDs = append(q.SegmentIDs, c.SegmentID)
	}
	if len(r.Calls) == 0 {
		return q, nil
	}
	priced, e := s.Ledger.QuoteUnits(ctx, owner, tier, clip.JobKindSpeech, speechBudgets(r))
	q.ID, q.MaximumCredits, q.ExpiresAt = priced.ID, priced.MaxCredits, priced.ExpiresAt
	return q, e
}
func (s *SpeechService) Start(ctx context.Context, owner, project string, revision int, key string, a usage.UnitApproval) (string, error) {
	if key == "" || len(key) > 200 {
		return "", clip.ErrInvalid
	}
	maximum := "none"
	if a.ApprovedMaxCredits != nil {
		maximum = strconv.Itoa(*a.ApprovedMaxCredits)
	}
	digest := usage.UnitDigest("clip-speech-start-v1", owner, project, strconv.Itoa(revision), a.QuoteID, maximum, strconv.Itoa(a.CancellationPolicyVersion))
	prior, e := s.Store.FindSpeechRun(ctx, owner, project, key)
	if e == nil {
		if prior.RequestDigest != digest {
			return "", clip.ErrPlanConflict
		}
		if prior.JobID == "" {
			return "", job.ErrDispatchRefused
		}
		return prior.JobID, nil
	}
	if !errors.Is(e, clip.ErrNotFound) {
		return "", e
	}
	r, tier, e := s.prepare(ctx, owner, project, revision)
	if e != nil {
		return "", e
	}
	if len(r.Calls) == 0 {
		return "", nil
	}
	reservation, e := s.Ledger.ReservationForUnits(ctx, owner, tier, clip.JobKindSpeech, a, speechBudgets(r))
	if e != nil {
		return "", e
	}
	r.ID, r.RequestKey, r.RequestDigest = speechID(), key, digest
	r, e = s.Store.ReserveSpeechRun(ctx, r)
	if e != nil {
		return "", e
	}
	if r.JobID != "" {
		return r.JobID, nil
	}
	calls := make([]job.PlannedCall, 0, len(r.Calls))
	for _, c := range r.Calls {
		calls = append(calls, job.PlannedCall{Ref: c.Budget.Ref.String(), Stage: c.Budget.Operation, Count: 1})
	}
	subject := job.Subject{Dimension: clip.JobSubject, ID: project}
	id, e := s.Queue.Enqueue(usage.WithUnitReservation(ctx, reservation), job.NewJob{Kind: clip.JobKindSpeech, UserID: owner, Subjects: []job.Subject{subject}, Guards: []job.Guard{{Subject: subject, Filter: job.Filter{UserID: owner}}}, WriteModel: r.Voice.Model.String(), Payload: []byte(r.ID), PricingCalls: calls, CancellationPolicyVersion: usage.UnitCancellationPolicyVersion})
	if e != nil {
		return "", e
	}
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), SpeechJournalTimeout)
	defer cancel()
	if e = s.Store.BindSpeechRun(audit, owner, r.ID, id); e != nil {
		return "", e
	}
	if e = s.Queue.Activate(audit, owner, id); e != nil {
		return "", e
	}
	return id, nil
}
func matchingSpeechJob(ctx context.Context, p SpeechTxPorts, r SpeechRun) error {
	j, e := p.Jobs.GetByID(ctx, r.JobID)
	if e != nil {
		return e
	}
	if !speechJobIdentity(j, r) || j.Status != job.StatusRunning || j.CancelRequestedAt != nil {
		return job.ErrDispatchRefused
	}
	return nil
}
func (s *SpeechService) Run(ctx context.Context, j job.Job, progress job.Progress) error {
	r, e := s.Store.GetSpeechRun(ctx, j.UserID, string(j.Payload))
	if e != nil {
		return e
	}
	if r.JobID != j.ID || r.ProjectID != j.Subject(clip.JobSubject) {
		return clip.ErrNotFound
	}
	admitted, ok, e := s.Ledger.AdmissionForJob(ctx, j.ID)
	if e != nil {
		return e
	}
	if !ok {
		return usage.ErrUnitCall
	}
	v, e := s.Voices.ResolveSpeechVoice(ctx, r.OwnerID, admitted.AdmittedPlan, r.Voice.Binding.ID)
	if e != nil {
		return e
	}
	if v != r.Voice {
		return clip.ErrPlanConflict
	}
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.JobID != j.ID || work.UserID != r.OwnerID {
		return usage.ErrUnitCall
	}
	work.UnitScopeDigest = r.ScopeDigest
	ctx = usage.WithWork(ctx, work)
	revision := r.Revision
	for i, c := range r.Calls {
		if e = ctx.Err(); e != nil {
			return e
		}
		current, voiceErr := s.Voices.ResolveSpeechVoice(ctx, r.OwnerID, admitted.AdmittedPlan, r.Voice.Binding.ID)
		if voiceErr != nil {
			return voiceErr
		}
		if current != r.Voice {
			return clip.ErrPlanConflict
		}
		progress("speech", i, len(r.Calls))
		e = s.Transactions.WriteSpeech(ctx, func(p SpeechTxPorts) error {
			if e := matchingSpeechJob(ctx, p, r); e != nil {
				return e
			}
			if e := p.Jobs.AuthorizeDispatch(ctx, r.OwnerID, r.JobID); e != nil {
				return e
			}
			project, e := p.Store.GetProject(ctx, r.OwnerID, r.ProjectID)
			if e != nil {
				return e
			}
			if project.EditPlanRevision != revision {
				return clip.ErrPlanConflict
			}
			return p.Store.ClaimSpeechCall(ctx, r, c)
		})
		if e != nil {
			return e
		}
		asset, e := s.synthesizeAsset(ctx, r, c)
		if e != nil {
			s.finishCall(ctx, r, c, "failed", "")
			return e
		}
		id := asset.ID

		s.finishCall(ctx, r, c, "received", id)
		e = s.Transactions.WriteSpeech(ctx, func(p SpeechTxPorts) error {
			if e := matchingSpeechJob(ctx, p, r); e != nil {
				return e
			}
			project, e := p.Store.PublishSpeechAsset(ctx, r.OwnerID, r.ProjectID, r.JobID, revision, c.SegmentID, c.InputHash, r.Voice.Binding.Digest, id)
			if e != nil {
				return e
			}
			if e = p.Store.FinishSpeechCall(ctx, r, c, "published", id); e != nil {
				return e
			}
			revision = project.EditPlanRevision
			return nil
		})
		if e != nil {
			s.finishCall(ctx, r, c, "obsolete", id)
			return e
		}
	}
	progress("save", len(r.Calls), len(r.Calls))
	return nil
}
func (s *SpeechService) finishCall(ctx context.Context, r SpeechRun, c SpeechCall, state, asset string) {
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), SpeechJournalTimeout)
	defer cancel()
	_ = s.Store.FinishSpeechCall(audit, r, c, state, asset)
}
func (s *SpeechService) OnTerminal(ctx context.Context, j job.Job) error {
	var r SpeechRun
	var e error
	if j.Kind == clip.JobKindGenerate {
		var p clip.GenerationPayload
		if json.Unmarshal(j.Payload, &p) != nil || p.Approval == nil || p.Approval.Pricing.Dubbing == nil {
			return nil
		}
		r, e = s.Store.FindSpeechRun(ctx, j.UserID, j.Subject(clip.JobSubject), p.Approval.QuoteID)
	} else {
		r, e = s.Store.GetSpeechRun(ctx, j.UserID, string(j.Payload))
	}
	if errors.Is(e, clip.ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	state := "unresolved"
	if j.Status == job.StatusCancelled {
		state = "cancelled"
	}
	for _, c := range r.Calls {
		if e = s.Store.FinishSpeechCall(ctx, r, c, "obsolete", ""); e != nil {
			return e
		}
		if e = s.Store.FinishSpeechCall(ctx, r, c, state, ""); e != nil {
			return e
		}
	}
	return nil
}

// Interrupted in-process calls are terminal. Recovery never synthesizes or retries paid work.
func (s *SpeechService) Recover(ctx context.Context) error {
	runs, e := s.Store.ListIncompleteSpeechRuns(ctx)
	if e != nil {
		return e
	}
	for _, r := range runs {
		if r.JobID == "" {
			active, e := s.Queue.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: r.ProjectID}, job.Filter{UserID: r.OwnerID})
			if e != nil {
				return e
			}
			if active == nil {
				continue
			}
			j, e := s.Queue.Snapshot(ctx, r.OwnerID, job.Subject{Dimension: clip.JobSubject, ID: r.ProjectID}, active.ID)
			if e != nil {
				return e
			}
			if !speechJobIdentity(*j, r) {
				continue
			}
			r.JobID = j.ID
			if e = s.Store.BindSpeechRun(ctx, r.OwnerID, r.ID, j.ID); e != nil {
				return e
			}
		}
		e = s.Transactions.WriteSpeech(ctx, func(p SpeechTxPorts) error {
			j, e := p.Jobs.GetByID(ctx, r.JobID)
			if e != nil {
				return e
			}
			if !speechJobIdentity(j, r) {
				return clip.ErrNotFound
			}
			if j.Status == job.StatusQueued && j.DispatchReady {
				return nil
			}
			if !job.Terminal(j.Status) {
				if e = p.Jobs.Finish(ctx, j.ID, job.StatusFailed, &job.Failure{Reason: job.FailureReasonInterrupted}, time.Now()); e != nil {
					return e
				}
			}
			for _, c := range r.Calls {
				if e = p.Store.FinishSpeechCall(ctx, r, c, "obsolete", ""); e != nil {
					return e
				}
				if e = p.Store.FinishSpeechCall(ctx, r, c, "unresolved", ""); e != nil {
					return e
				}
			}
			return nil
		})
		if e != nil {
			return e
		}
	}
	return nil
}

func (s *SpeechService) synthesizeAsset(ctx context.Context, r SpeechRun, c SpeechCall) (clip.SpeechAsset, error) {
	response, e := s.Models.SynthesizeSpeech(ctx, c.Request)
	if e != nil {

		return clip.SpeechAsset{}, e
	}
	// Decode original bytes again at the application seam; supplied metadata cannot bless corrupt media.
	audio, e := llm.InspectSpeechAudio(ctx, response.Audio.Bytes)
	if e != nil {

		return clip.SpeechAsset{}, e
	}
	id := speechID()
	ref := clip.SpeechRef{AssetID: id, VoiceID: r.Voice.Binding.ID, BindingDigest: r.Voice.Binding.Digest, InputHash: c.InputHash, SettingsHash: r.Voice.Settings.Digest(), AudioHash: audio.SHA256, ProfileID: r.Voice.ProfileID, ProfileRevision: r.Voice.ProfileRevision, Samples: audio.Samples, SampleRate: audio.SampleRate, Channels: audio.Channels}
	timing := response.Alignment
	if len(timing) == 0 {
		timing = response.NormalizedAlignment
	}
	if llm.ValidateSpeechAlignment(timing, audio) == nil {
		last := 0
		for _, t := range timing {
			start, end := int(t.StartSeconds*1000), int(t.EndSeconds*1000)
			if start < last || end <= start || end > ref.DurationMS() {
				ref.Timing = nil
				break
			}
			ref.Timing = append(ref.Timing, clip.SpeechTiming{Text: t.Character, StartMS: start, EndMS: end})
			last = end
		}
	}
	asset := clip.SpeechAsset{ID: id, OwnerID: r.OwnerID, ProjectID: r.ProjectID, ObjectKey: clip.SpeechAudioPrefix + id + ".mp3", Bytes: int64(len(audio.Bytes)), Text: c.Text, Speech: ref, CreatedAt: time.Now().UTC()}
	if e = s.Objects.PutClipSpeechAudio(ctx, asset.ObjectKey, audio.Bytes); e != nil {

		return clip.SpeechAsset{}, e
	}
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), SpeechJournalTimeout)
	e = s.Store.InsertSpeechAsset(audit, asset)
	cancel()
	if e != nil {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), SpeechJournalTimeout)
		_ = s.Objects.DeleteClipSpeechAudio(cleanup, asset.ObjectKey)
		stop()

		return clip.SpeechAsset{}, e
	}
	return asset, nil
}
