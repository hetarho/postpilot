package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) uploadJob(ctx context.Context, user, project, id string) (*clip.ClipJob, error) {
	reader, ok := s.jobs.(clipJobSnapshotReader)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	return reader.Snapshot(ctx, user, project, id)
}

func (s *GenerationService) uploadStore() (clip.UploadTextStore, error) {
	store, ok := s.store.(clip.UploadTextStore)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	return store, nil
}
func (s *GenerationService) UploadTexts(ctx context.Context, user, id string) ([]clip.UploadText, error) {
	if _, err := s.projects.store.GetProject(ctx, user, id); err != nil {
		return nil, err
	}
	store, err := s.uploadStore()
	if err != nil {
		return nil, err
	}
	return store.ListUploadTexts(ctx, user, id)
}
func (s *GenerationService) SaveUploadText(ctx context.Context, user, id, channel, body string, tags []string, revision int) (clip.UploadText, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.UploadText{}, err
	}
	if _, err = clip.UploadTextMaterial(p, channel); err != nil {
		return clip.UploadText{}, err
	}
	checked, err := clip.ValidateUploadTextEdit(channel, p.Language, p.Campaign, body, tags)
	if err != nil {
		return clip.UploadText{}, err
	}
	store, err := s.uploadStore()
	if err != nil {
		return clip.UploadText{}, err
	}
	return store.SaveUploadText(ctx, clip.UploadTextSave{UserID: user, ProjectID: id, Channel: channel, Body: checked.Body, Tags: checked.Tags, ExpectedRevision: revision, Edited: true, Now: s.now()})
}
func (s *GenerationService) uploadInputs(ctx context.Context, user, id, channel string, confirmed bool) (clip.Project, clip.UploadTextInput, clip.UploadText, error) {
	var empty clip.UploadTextInput
	var row clip.UploadText
	if s.uploadWriter == nil || s.uploadVoice == nil {
		return clip.Project{}, empty, row, clip.ErrPricingUnavailable
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return clip.Project{}, empty, row, err
	}
	if active != nil {
		return clip.Project{}, empty, row, clip.ErrBusy
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return p, empty, row, err
	}
	if p, err = s.projects.freezeLinkedPost(ctx, user, p); err != nil {
		return p, empty, row, err
	}
	in, err := clip.UploadTextMaterial(p, channel)
	if err != nil {
		return p, in, row, err
	}
	if _, err = clip.RequiredUploadTags(in.AdvertiserGuide); err != nil {
		return p, in, row, err
	}
	store, err := s.uploadStore()
	if err != nil {
		return p, in, row, err
	}
	rows, err := store.ListUploadTexts(ctx, user, id)
	if err != nil {
		return p, in, row, err
	}
	for _, text := range rows {
		if text.Channel == channel {
			row = text
			break
		}
	}
	if row.Edited && !confirmed {
		return p, in, row, clip.ErrUploadTextConfirmation
	}
	in.Guidelines, err = s.videoGuidelines(ctx, p)
	if err != nil {
		return p, in, row, err
	}
	in.Voice, err = s.uploadVoice.DefaultClipVoice(ctx, user, p.Language)
	return p, in, row, err
}
func (s *GenerationService) uploadPricing(ctx context.Context, user, write string) (clip.GenerationPricing, error) {
	pricing, ok := s.pricing.(clip.UploadTextPricing)
	if !ok {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	if err := s.checkQuoteAccess(ctx, user, "", write, 0, 1); err != nil {
		return clip.GenerationPricing{}, err
	}
	p, err := pricing.FreezeUploadText(ctx, modelRef(write))
	if err != nil {
		return p, err
	}
	if !p.UploadText || !p.Valid() || p.Plan.Ref != modelRef(write) || p.PlanCalls() != 1 {
		return p, clip.ErrPricingUnavailable
	}
	return p, nil
}
func (s *GenerationService) QuoteUploadText(ctx context.Context, user, id, channel, write string, confirmed bool) (clip.UploadTextQuote, error) {
	_, in, row, err := s.uploadInputs(ctx, user, id, channel, confirmed)
	if err != nil {
		return clip.UploadTextQuote{}, err
	}
	pricing, err := s.uploadPricing(ctx, user, write)
	if err != nil {
		return clip.UploadTextQuote{}, err
	}
	in.Policy = pricing.Plan
	if err = s.uploadWriter.ValidateUploadText(ctx, modelRef(write), in); err != nil {
		return clip.UploadTextQuote{}, err
	}
	if s.cfg.QuoteTTL <= 0 {
		return clip.UploadTextQuote{}, clip.ErrPricingUnavailable
	}
	now := s.now()
	quote := clip.UploadTextQuote{GenerationQuote: clip.GenerationQuote{ID: newID(), UserID: user, ProjectID: id, InputDigest: clip.UploadQuoteDigest(in, row.Revision, confirmed, pricing), Pricing: pricing, ExpiresAt: now.Add(s.cfg.QuoteTTL)}, Channel: channel, ExpectedRevision: row.Revision}
	store, _ := s.uploadStore()
	if err = store.SaveUploadTextQuote(ctx, quote, now); err != nil {
		return clip.UploadTextQuote{}, err
	}
	return quote, nil
}
func (s *GenerationService) StartUploadText(ctx context.Context, user, id, channel, write string, confirmed bool, a clip.QuoteApproval) (string, error) {
	if a.QuoteID == "" || a.MaxCredits == nil || *a.MaxCredits < 0 {
		return "", clip.ErrQuoteRequired
	}
	if a.CancellationPolicyVersion != clip.CancellationPolicyVersion {
		return "", clip.ErrCancellationPolicy
	}
	store, err := s.uploadStore()
	if err != nil {
		return "", err
	}
	quote, err := store.GetUploadTextQuote(ctx, user, a.QuoteID)
	if err != nil {
		return "", err
	}
	if quote.ProjectID != id || quote.Channel != channel || quote.Pricing.Plan.Ref != modelRef(write) || quote.Pricing.MaxCredits != *a.MaxCredits {
		return "", clip.ErrQuoteChanged
	}
	if quote.ConsumedJobID != "" {
		j, e := s.uploadJob(ctx, user, id, quote.ConsumedJobID)
		if e != nil {
			return "", e
		}
		var frozen clip.UploadTextSnapshot
		if j == nil || j.Kind != clip.JobKindUploadText || clip.StrictJSON(string(j.Payload), &frozen) != nil || frozen.UserID != user || frozen.ProjectID != id || frozen.Input.Channel != channel || frozen.ConfirmEdited != confirmed {
			return "", clip.ErrQuoteChanged
		}
		return j.ID, nil
	}
	if !s.now().Before(quote.ExpiresAt) {
		return "", clip.ErrQuoteExpired
	}
	p, in, row, err := s.uploadInputs(ctx, user, id, channel, confirmed)
	if err != nil {
		return "", err
	}
	pricing, err := s.uploadPricing(withFrozenQuoteRate(ctx, quote.Pricing), user, write)
	if err != nil {
		return "", err
	}
	in.Policy = pricing.Plan
	if row.Revision != quote.ExpectedRevision || !reflect.DeepEqual(quote.Pricing, pricing) || quote.InputDigest != clip.UploadQuoteDigest(in, row.Revision, confirmed, pricing) {
		return "", clip.ErrQuoteChanged
	}
	if err = s.uploadWriter.ValidateUploadText(ctx, modelRef(write), in); err != nil {
		return "", err
	}
	frozen := clip.UploadTextSnapshot{Version: clip.UploadTextSnapshotVersion, UserID: user, ProjectID: id, PlanRevision: p.EditPlanRevision, TextRevision: row.Revision, MaterialDigest: clip.UploadMaterialDigest(in), ConfirmEdited: confirmed, Input: in, Approval: &clip.GenerationApproval{QuoteID: quote.ID, MaxCredits: pricing.MaxCredits, Pricing: pricing}}
	payload, err := json.Marshal(frozen)
	if err != nil {
		return "", err
	}
	job, err := s.jobs.Enqueue(ctx, clip.GenerationStart{UserID: user, ProjectID: id, Write: write, Kind: clip.JobKindUploadText, Payload: payload, Quote: &quote.GenerationQuote})
	if err != nil {
		return "", err
	}
	if err = store.LinkUploadTextJob(ctx, quote, job, frozen, s.now()); err == nil {
		err = s.jobs.Activate(ctx, user, job)
	}
	if err == nil {
		return job, nil
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
	defer cancel()
	failed, failErr := s.jobs.FailQueued(cleanup, user, job)
	if failErr != nil {
		return "", errors.Join(err, failErr)
	}
	if !failed {
		j, e := s.jobs.Get(cleanup, user, job)
		if e == nil && j != nil && (j.Status == "running" || j.Status == "done") {
			return job, nil
		}
	}
	return "", err
}
func (s *GenerationService) RunUploadText(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	defer func() {
		if err != nil {
			err = &clip.StageFailure{Stage: stage, Cause: err}
		}
	}()
	var frozen clip.UploadTextSnapshot
	if clip.StrictJSON(string(payload), &frozen) != nil || !clip.SupportedUploadTextSnapshot(frozen.Version) || frozen.UserID != user || frozen.ProjectID != project || frozen.Approval == nil {
		return clip.ErrInvalid
	}
	pricing := frozen.Approval.Pricing
	if !pricing.UploadText || !pricing.Valid() || frozen.Input.Policy != pricing.Plan || frozen.Approval.MaxCredits != pricing.MaxCredits {
		return clip.ErrPricingUnavailable
	}
	finisher, ok := s.finisher.(clip.UploadTextFinisher)
	if !ok || s.uploadWriter == nil {
		return clip.ErrCompositionUnavailable
	}
	j, err := s.uploadJob(ctx, user, project, job)
	if err != nil {
		return err
	}
	if j == nil || j.Kind != clip.JobKindUploadText || !bytes.Equal(j.Payload, payload) {
		return clip.ErrNotFound
	}
	if j.Status == "done" {
		return nil
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	// The worker keeps the text approved at start, even if the owner edits the post later.
	p.LinkedPostText = frozen.Input.LinkedPost
	material, err := clip.UploadTextMaterial(p, frozen.Input.Channel)
	if err != nil {
		return err
	}
	if p.EditPlanRevision != frozen.PlanRevision || clip.UploadMaterialDigest(material) != frozen.MaterialDigest {
		return clip.ErrPlanConflict
	}
	rows, err := s.UploadTexts(ctx, user, project)
	if err != nil {
		return err
	}
	revision := 0
	for _, r := range rows {
		if r.Channel == frozen.Input.Channel {
			revision = r.Revision
		}
	}
	if revision != frozen.TextRevision {
		return clip.ErrPlanConflict
	}
	if err = s.uploadWriter.ValidateUploadText(ctx, pricing.Plan.Ref, frozen.Input); err != nil {
		return err
	}
	ctx, err = s.jobs.ReserveApproved(ctx, user, job, *frozen.Approval, 0)
	if err != nil {
		return err
	}
	stage = "upload_text"
	if progress != nil {
		progress(stage, 0, 1)
	}
	text, _, err := s.uploadWriter.WriteUploadText(ctx, pricing.Plan.Ref, frozen.Input)
	if err != nil {
		return err
	}
	text, err = clip.BoundGeneratedUploadText(frozen.Input, text.Body, text.Tags)
	if err != nil {
		return err
	}
	stage = "save"
	_, err = finisher.CompleteUploadText(ctx, clip.UploadTextSave{UserID: user, ProjectID: project, Channel: frozen.Input.Channel, Body: text.Body, Tags: text.Tags, ExpectedRevision: frozen.TextRevision, ExpectedPlanRevision: frozen.PlanRevision, JobID: job, Now: s.now()})
	return err
}
