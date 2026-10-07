package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/plan"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

func speechJobIdentity(j job.Job, r SpeechRun) bool {
	if j.UserID != r.OwnerID || j.Subject(clip.JobSubject) != r.ProjectID {
		return false
	}
	if !r.ParentGeneration {
		return j.Kind == clip.JobKindSpeech && string(j.Payload) == r.ID
	}
	var p clip.GenerationPayload
	return j.Kind == clip.JobKindGenerate && json.Unmarshal(j.Payload, &p) == nil && p.ProjectID == r.ProjectID && p.Approval != nil && p.Approval.Pricing.Dubbing != nil && reflect.DeepEqual(p.Approval.Pricing.Dubbing.Voice, r.Voice) && p.Approval.QuoteID == r.RequestKey
}

// QuoteInitial freezes an enforceable unknown corpus; the outer generation approval
// carries this internal quotation and makes the single mixed admission.
func (s *SpeechService) QuoteInitial(ctx context.Context, owner string, p clip.Project, batch string, prior *clip.NarratedPricing, complete bool, draft *clip.SpokenDraft) (*clip.NarratedPricing, error) {
	// Completing a frozen draft with all its immutable recordings needs no usable
	// supplier identity, including after the account removes the reusable voice.
	if prior != nil && prior.Voice.Binding.ID == p.Dubbing.VoiceID && prior.Voice.Binding.Digest == p.Dubbing.BindingDigest {
		ready := complete
		if draft != nil && len(draft.Narration.Segments) > 0 {
			ready = true
			for _, seg := range draft.Narration.Segments {
				ready = ready && clip.CompatibleSpeech(&draft.Narration, seg)
			}
		}
		if ready {
			kept := *prior
			kept.Units = nil
			kept.QuoteID = ""
			kept.ExpiresAt = time.Time{}
			kept.Rate = plan.RateSnapshot{}
			return &kept, nil
		}
	}
	tier, e := s.Plans.PlanOf(ctx, owner)
	if e != nil {
		return nil, e
	}
	v, e := s.Voices.ResolveSpeechVoice(ctx, owner, tier, p.Dubbing.VoiceID)
	if e != nil {
		return nil, e
	}
	if v.Binding.Digest != p.Dubbing.BindingDigest {
		return nil, clip.ErrQuoteChanged
	}
	if prior != nil {
		if prior.Version != 1 || prior.Voice != v {
			return nil, clip.ErrQuoteChanged
		}
		for _, b := range prior.Units {
			current, e := s.Prices.Budget(ctx, owner, tier, v.ProfileID, v.ProfileRevision, "", b.ScopeDigest, llm.SpeechInput{Ref: b.Ref, Operation: b.Operation, Digest: b.InputDigest, InputCharacters: b.InputCharacters, ParametersDigest: b.ParametersDigest}, b.Count)
			if e != nil {
				return nil, e
			}
			current.BoundedInput = b.BoundedInput
			current.TotalInputCharacters = b.TotalInputCharacters
			current.InputIdentityDigest = b.InputIdentityDigest
			if current.Fingerprint() != b.Fingerprint() {
				return nil, clip.ErrQuoteChanged
			}
		}
		return prior, nil
	}
	priced := &clip.NarratedPricing{Version: 1, Voice: v}
	if complete {
		return priced, nil
	}
	if draft != nil {
		all := len(draft.Narration.Segments) > 0
		for _, seg := range draft.Narration.Segments {
			all = all && clip.CompatibleSpeech(&draft.Narration, seg)
		}
		if all && draft.Narration.BindingDigest == v.Binding.Digest {
			return priced, nil
		}
	}
	request := llm.SpeechRequest{Model: v.Model, Voice: v.Handle, Settings: v.Settings, Text: strings.Repeat("가", clip.MaxSpokenSegmentRunes)}
	input, e := request.Input()
	if e != nil {
		return nil, e
	}
	scope := usage.UnitDigest("clip-initial-speech-v1", owner, p.ID, batch, strconv.Itoa(p.EditPlanRevision), v.Binding.Digest)
	budget, e := s.Prices.Budget(ctx, owner, tier, v.ProfileID, v.ProfileRevision, "", scope, input, clip.MaxSpokenSegments)
	if e != nil {
		return nil, e
	}
	budget.BoundedInput = true
	budget.TotalInputCharacters = clip.MaxSpokenScriptRunes
	budget.InputIdentityDigest = input.IdentityDigest
	q, e := s.Ledger.QuoteUnits(ctx, owner, tier, clip.JobKindGenerate, []usage.UnitBudget{budget})
	if e != nil {
		return nil, e
	}
	priced.Units, priced.QuoteID, priced.ExpiresAt, priced.Rate = q.Calls, q.ID, q.ExpiresAt, q.Rate
	return priced, nil
}

// AssembleInitial never creates a child job. Each request has a durable claim in
// its parent's journal, and a returned asset survives any later composition failure.
func (s *SpeechService) AssembleInitial(ctx context.Context, owner, project, id, quote string, revision int, p *clip.NarratedPricing, d *clip.SpokenDraft, keep func() error, progress func(string, int, int)) error {
	if p == nil || d.Version != 1 {
		return clip.ErrInvalid
	}
	d.Narration.VoiceID, d.Narration.BindingDigest = p.Voice.Binding.ID, p.Voice.Binding.Digest
	if e := clip.ValidateNarration(clip.EditPlan{Narration: &d.Narration}); e != nil {
		return e
	}
	admission, ok, e := s.Ledger.AdmissionForJob(ctx, id)
	if e != nil {
		return e
	}
	if !ok {
		return usage.ErrUnitCall
	}
	r := SpeechRun{ParentGeneration: true, ID: speechID(), OwnerID: owner, ProjectID: project, JobID: id, RequestKey: quote, RequestDigest: usage.UnitDigest("initial-manifest", quote), Revision: revision, Voice: p.Voice}
	if len(p.Units) > 0 {
		r.ScopeDigest = p.Units[0].ScopeDigest
	}
	// Assets are immutable owned checkpoints. A completed exact input needs no new call.
	for i := range d.Narration.Segments {
		seg := &d.Narration.Segments[i]
		if clip.CompatibleSpeech(&d.Narration, *seg) {
			continue
		}
		a, e := s.Store.FindSpeechAsset(ctx, owner, project, seg.InputHash, p.Voice.Binding.Digest)
		if e == nil {
			if a.Text == seg.Text && a.Speech.ProfileID == p.Voice.ProfileID && a.Speech.ProfileRevision == p.Voice.ProfileRevision && a.Speech.SettingsHash == p.Voice.Settings.Digest() {
				ref := a.Speech
				seg.Speech = &ref
				continue
			}
		} else if !errors.Is(e, clip.ErrNotFound) {
			return e
		}
		if len(p.Units) != 1 {
			return usage.ErrUnitCall
		}
		r.Calls = append(r.Calls, SpeechCall{SegmentID: seg.ID, Text: seg.Text, InputHash: seg.InputHash, Request: initialSegmentSpeechRequest(p.Voice.Model, p.Voice.Handle, seg.Text, p.Voice.Settings, seg.ID), Budget: p.Units[0]})
	}
	if e := keep(); e != nil {
		return e
	}
	if len(r.Calls) == 0 {
		return nil
	}
	r, e = s.Store.ReserveSpeechRun(ctx, r)
	if e != nil {
		return e
	}
	if e = s.Store.BindSpeechRun(ctx, owner, r.ID, id); e != nil {
		return e
	}
	r.JobID = id
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.JobID != id || work.UserID != owner {
		return usage.ErrUnitCall
	}
	work.UnitScopeDigest = r.ScopeDigest
	ctx = usage.WithWork(ctx, work)
	for index, c := range r.Calls {
		if e = ctx.Err(); e != nil {
			return e
		}
		v, e := s.Voices.ResolveSpeechVoice(ctx, owner, admission.AdmittedPlan, p.Voice.Binding.ID)
		if e != nil {
			return e
		}
		if v != p.Voice {
			return clip.ErrPlanConflict
		}
		if progress != nil {
			progress("speech", index, len(r.Calls))
		}
		e = s.Transactions.WriteSpeech(ctx, func(tx SpeechTxPorts) error {
			if e := matchingSpeechJob(ctx, tx, r); e != nil {
				return e
			}
			if e := tx.Jobs.AuthorizeDispatch(ctx, owner, id); e != nil {
				return e
			}
			current, e := tx.Store.GetProject(ctx, owner, project)
			if e != nil {
				return e
			}
			if current.EditPlanRevision != revision {
				return clip.ErrPlanConflict
			}
			return tx.Store.ClaimSpeechCall(ctx, r, c)
		})
		if e != nil {
			return e
		}
		asset, e := s.synthesizeAsset(ctx, r, c)
		if e != nil {
			s.finishCall(ctx, r, c, "failed", "")
			return e
		}
		s.finishCall(ctx, r, c, "received", asset.ID)
		e = s.Transactions.WriteSpeech(ctx, func(tx SpeechTxPorts) error {
			if e := matchingSpeechJob(ctx, tx, r); e != nil {
				return e
			}
			return tx.Store.FinishSpeechCall(ctx, r, c, "published", asset.ID)
		})
		if e != nil {
			s.finishCall(ctx, r, c, "obsolete", asset.ID)
			return e
		}
		for i := range d.Narration.Segments {
			if d.Narration.Segments[i].ID == c.SegmentID {
				ref := asset.Speech
				d.Narration.Segments[i].Speech = &ref
			}
		}
		if e = keep(); e != nil {
			return e
		}
	}
	return nil
}

func (s *SpeechService) ValidateInitialVoice(ctx context.Context, owner string, p *clip.NarratedPricing) error {
	if len(p.Units) == 0 {
		return nil
	}
	tier, e := s.Plans.PlanOf(ctx, owner)
	if e != nil {
		return e
	}
	v, e := s.Voices.ResolveSpeechVoice(ctx, owner, tier, p.Voice.Binding.ID)
	if e != nil {
		return e
	}
	if v != p.Voice {
		return clip.ErrQuoteChanged
	}
	return nil
}
