package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	"math/big"
	"strings"
	"time"
)

var ErrQualificationEvidence = errors.New("complete live spoken qualification evidence required")
var ErrQualificationBudget = spoken.ErrQualificationBudget

// These are private operator ports. No customer DTO or capability checkbox can
// supply supplier evidence or publish readiness.
type QualificationSession struct {
	ID, OwnerID, ProfileID, MaximumUSD, WorstMaximumUSD string
	Revision                                            int64
	ExpiresAt                                           time.Time
}
type QualificationReader interface {
	Session(context.Context, string, string) (QualificationSession, error)
}
type QualificationSessions interface {
	QualificationReader
	Publish(context.Context, QualificationEvidence) error
}
type QualificationJob struct {
	Budgets    []usage.UnitBudget
	ActualUSD  string
	Settled    bool
	Settlement usage.Settlement
}
type QualificationAccounting interface {
	Job(context.Context, string, string) (QualificationJob, error)
}
type QualificationInput struct {
	OwnerID, SessionID, DesignOperationID, ConfirmOperationID string
	FirstText, SecondText, ChangedFirstText                   string
}
type QualificationReview struct {
	Reviewer                                                                                                              string
	ReviewedAt                                                                                                            time.Time
	AuditionAccepted, KoreanAccepted, ContinuityAccepted                                                                  bool
	Names, PricesNumbers, Units, Punctuation, MixedLanguage, OmissionsRepeats, Clipping, Continuity, ConfirmationCapacity string
}
type QualificationEvidence struct {
	OwnerID, SessionID, ReportID, DesignRequestID, ConfirmRequestID, ConfirmedVoiceID string
	SpeechRequestIDs                                                                  []string
	Review                                                                            QualificationReview
}
type QualificationPlan struct {
	Input                                  QualificationInput
	Digest, MaximumUSD, ApprovedMaximumUSD string
	Profile                                spoken.Profile
	NewCalls                               int
}
type QualificationSample struct {
	AssetID, SHA256, RequestID string
	DurationMS                 int64
	CharacterTiming            []llm.CharacterTiming
}
type QualificationReport struct {
	Audition             QualificationSample
	Plan                 QualificationPlan
	Design, Confirmation spoken.Operation
	Probes               [2]spoken.Operation
	Samples              [3]QualificationSample
	ActualUSD            string
	Review               QualificationReview
}
type QualificationHarness struct {
	Generation *GenerationService
	Sessions   QualificationSessions
	Accounting QualificationAccounting
	Poll       time.Duration
}

func NewQualificationHarness(g *GenerationService, sessions QualificationSessions, accounting QualificationAccounting) *QualificationHarness {
	if g == nil || sessions == nil || accounting == nil {
		panic("spoken qualification dependencies required")
	}
	return &QualificationHarness{g, sessions, accounting, 200 * time.Millisecond}
}
func qualificationUSD(s string) (*big.Rat, error) {
	n, ok := new(big.Rat).SetString(s)
	if !ok || n.Sign() < 0 {
		return nil, ErrQualificationBudget
	}
	return n, nil
}
func budgetsUSD(bs []usage.UnitBudget) (*big.Rat, error) {
	n := new(big.Rat)
	for _, b := range bs {
		c, e := b.MaximumUSD()
		if e != nil {
			return nil, e
		}
		n.Add(n, c)
	}
	return n, nil
}
func (h *QualificationHarness) Preflight(ctx context.Context, owner, session string) (QualificationSession, error) {
	q, err := h.Sessions.Session(ctx, owner, session)
	if err != nil {
		return q, err
	}
	if q.OwnerID != owner || !q.ExpiresAt.After(time.Now()) {
		return q, ErrQualificationEvidence
	}
	maximum, err := qualificationUSD(q.MaximumUSD)
	if err != nil {
		return q, err
	}
	profile, err := h.Generation.Profiles.ResolveSpokenProfile(ctx, owner, plan.Master, q.ProfileID, q.Revision, q.ID)
	if err != nil || len(profile.ConnectionScope) != 64 {
		return q, ErrQualificationEvidence
	}
	// Conservative documented ceilings at the profile's input limits. These
	// synthetic digests are never admitted, claimed or sent to a supplier.
	worst := new(big.Rat)
	for _, operation := range []string{spoken.JobKindDesign, spoken.JobKindConfirm, "speech"} {
		ref, count, characters, auxiliary := profile.Design, 1, profile.PreviewMax, profile.DescriptionMax
		if operation == spoken.JobKindConfirm {
			characters, auxiliary = 0, 0
		}
		if operation == "speech" {
			ref, count, characters, auxiliary = profile.Synthesis, 3, profile.SpeechMax, 0
		}
		input := llm.SpeechInput{Ref: ref, Operation: operation, InputCharacters: characters, AuxiliaryCharacters: auxiliary, ParametersDigest: profile.Settings.Digest(), Digest: usage.UnitDigest("qualification-upper-only", owner, q.ID, operation)}
		budget, err := h.Generation.Prices.Budget(ctx, owner, plan.Master, profile.ID, profile.Revision, q.ID, usage.UnitDigest("qualification-upper-only", q.ID), input, count)
		if err != nil {
			return q, err
		}
		cap, err := budget.MaximumUSD()
		if err != nil {
			return q, err
		}
		worst.Add(worst, cap)
	}
	q.WorstMaximumUSD = worst.RatString()
	if worst.Cmp(maximum) > 0 {
		return q, ErrQualificationBudget
	}
	return q, nil
}
func (h *QualificationHarness) source(ctx context.Context, in QualificationInput) (QualificationSession, spoken.Operation, spoken.Operation, spoken.Voice, error) {
	q, e := h.Preflight(ctx, in.OwnerID, in.SessionID)
	if e != nil {
		return q, spoken.Operation{}, spoken.Operation{}, spoken.Voice{}, e
	}
	if q.OwnerID != in.OwnerID || !q.ExpiresAt.After(time.Now()) {
		return q, spoken.Operation{}, spoken.Operation{}, spoken.Voice{}, ErrQualificationEvidence
	}
	d, e := h.Generation.Get(ctx, in.OwnerID, in.DesignOperationID)
	if e != nil {
		return q, d, spoken.Operation{}, spoken.Voice{}, e
	}
	c, e := h.Generation.Get(ctx, in.OwnerID, in.ConfirmOperationID)
	if e != nil {
		return q, d, c, spoken.Voice{}, e
	}
	v, e := h.Generation.Library.GetVoice(ctx, in.OwnerID, c.ResultID)
	if e != nil {
		return q, d, c, v, e
	}
	draft, e := h.Generation.Library.GetDraft(ctx, in.OwnerID, d.DraftID)
	if e != nil {
		return q, d, c, v, e
	}
	var chosen *spoken.Candidate
	for i := range draft.Candidates {
		if draft.Candidates[i].ID == c.CandidateID {
			chosen = &draft.Candidates[i]
		}
	}
	if d.State != spoken.OperationPublished || c.State != spoken.OperationPublished || d.Kind != spoken.JobKindDesign || c.Kind != spoken.JobKindConfirm || d.QualificationSessionID != q.ID || c.QualificationSessionID != q.ID || d.Profile != c.Profile || v.Profile != d.Profile || v.Profile.ID != q.ProfileID || v.Profile.Revision != q.Revision || v.RemovedAt != nil || c.DraftID != d.DraftID || draft.GenerationID != d.ID || draft.ConfirmedVoiceID != v.ID || chosen == nil || chosen.AuditionedAt == nil || chosen.Handle != c.CandidateHandle || chosen.AssetID != c.SampleAssetID || v.SampleAssetID != chosen.AssetID || v.Handle != c.ReceivedHandle || len(d.Evidence) != 1 || len(c.Evidence) != 1 || d.Evidence[0].RequestID == "" || c.Evidence[0].RequestID == "" {
		return q, d, c, v, ErrQualificationEvidence
	}
	return q, d, c, v, nil
}
func (h *QualificationHarness) probeInput(in QualificationInput, v spoken.Voice, changed bool) GenerationInput {
	first := in.FirstText
	if changed {
		first = in.ChangedFirstText
	}
	return GenerationInput{Kind: spoken.JobKindProbe, VoiceID: v.ID, Revision: v.Revision, QualificationSessionID: in.SessionID, Texts: [2]string{first, in.SecondText}}
}
func (h *QualificationHarness) Plan(ctx context.Context, in QualificationInput) (QualificationPlan, error) {
	q, d, c, v, e := h.source(ctx, in)
	if e != nil {
		return QualificationPlan{}, e
	}
	if in.FirstText == in.SecondText || in.ChangedFirstText == in.FirstText || in.ChangedFirstText == in.SecondText || in.FirstText == "" || in.SecondText == "" || in.ChangedFirstText == "" {
		return QualificationPlan{}, spoken.ErrInvalid
	}
	maximum, e := qualificationUSD(q.MaximumUSD)
	if e != nil {
		return QualificationPlan{}, e
	}
	total := new(big.Rat)
	for _, o := range []spoken.Operation{d, c} {
		j, e := h.Accounting.Job(ctx, in.OwnerID, o.JobID)
		if e != nil || !j.Settled {
			return QualificationPlan{}, ErrQualificationEvidence
		}
		if e := verifyQualifiedJob(o, j); e != nil {
			return QualificationPlan{}, e
		}
	}
	prior, err := h.Generation.Operations.ListQualificationOperations(ctx, in.OwnerID, q.ID)
	if err != nil {
		return QualificationPlan{}, err
	}
	for _, operation := range prior {
		limit, ok := new(big.Rat).SetString(operation.QualificationLimitUSD)
		if !ok || limit.Cmp(maximum) != 0 {
			return QualificationPlan{}, ErrQualificationEvidence
		}
		reserved, ok := new(big.Rat).SetString(operation.QualificationReservedUSD)
		if !ok || reserved.Sign() < 0 {
			return QualificationPlan{}, ErrQualificationEvidence
		}
		total.Add(total, reserved)
	}
	fingerprints := []string{"spoken-qualification-v1", in.OwnerID, q.ID, d.ID, c.ID, v.ID, fmt.Sprint(v.Revision), v.Profile.ConnectionScope, in.FirstText, in.SecondText, in.ChangedFirstText, q.MaximumUSD}
	seen := map[string]bool{}
	count := 0
	for _, changed := range []bool{false, true} {
		p, e := h.Generation.prepare(ctx, in.OwnerID, plan.Master, h.probeInput(in, v, changed))
		if e != nil {
			return QualificationPlan{}, e
		}
		for i, input := range p.inputs {
			if seen[input.Digest] {
				continue
			}
			seen[input.Digest] = true
			budget, err := h.Generation.Prices.Budget(ctx, in.OwnerID, plan.Master, v.Profile.ID, v.Profile.Revision, q.ID, p.operation.ScopeDigest, input, 1)
			if err != nil {
				return QualificationPlan{}, err
			}
			cap, err := budget.MaximumUSD()
			if err != nil {
				return QualificationPlan{}, err
			}
			if p.operation.AssetIDs[i] == "" {
				total.Add(total, cap)
			}
			fingerprints = append(fingerprints, budget.Fingerprint())
			if p.operation.AssetIDs[i] == "" {
				count += budget.Count
			}
		}
	}
	if total.Cmp(maximum) > 0 {
		return QualificationPlan{}, ErrQualificationBudget
	}
	fingerprints = append(fingerprints, total.RatString())
	return QualificationPlan{Input: in, Digest: usage.UnitDigest(fingerprints...), MaximumUSD: total.RatString(), ApprovedMaximumUSD: q.MaximumUSD, Profile: v.Profile, NewCalls: count}, nil
}

// Run is explicit opt-in over an exact private plan. The producer only enqueues
// production jobs; the deployed worker owns actual provider I/O and settlement.
// It stops on an uncertain/error result and never creates an automatic paid retry.
func (h *QualificationHarness) Run(ctx context.Context, approved QualificationPlan, live bool, approvalDigest string) (QualificationReport, error) {
	if !live || approvalDigest == "" || approvalDigest != approved.Digest {
		return QualificationReport{}, ErrQualificationEvidence
	}
	current, e := h.Plan(ctx, approved.Input)
	if e != nil {
		return QualificationReport{}, e
	}
	if current.Digest != approved.Digest || current.Profile != approved.Profile || current.MaximumUSD != approved.MaximumUSD || current.ApprovedMaximumUSD != approved.ApprovedMaximumUSD {
		return QualificationReport{}, spoken.ErrConflict
	}
	_, d, c, v, e := h.source(ctx, approved.Input)
	if e != nil {
		return QualificationReport{}, e
	}
	report := QualificationReport{Plan: approved, Design: d, Confirmation: c}
	for phase := 0; phase < 2; phase++ {
		current, err := h.Plan(ctx, approved.Input)
		if err != nil {
			return report, err
		}
		if current.Digest != approved.Digest || current.MaximumUSD != approved.MaximumUSD {
			return report, spoken.ErrConflict
		}
		in := h.probeInput(approved.Input, v, phase == 1)
		key := usage.UnitDigest("qualification-probe-v1", approved.Digest, fmt.Sprint(phase))
		o, e := h.Generation.Operations.GetOperationRequest(ctx, approved.Input.OwnerID, spoken.JobKindProbe, key)
		if errors.Is(e, spoken.ErrNotFound) {
			q, quoteErr := h.Generation.Quote(ctx, approved.Input.OwnerID, plan.Master, in)
			if quoteErr != nil {
				return report, quoteErr
			}
			var approval usage.UnitApproval
			if q.ApprovalRequired {
				credits := q.MaximumCredits
				approval = usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &credits, CancellationPolicyVersion: usage.UnitCancellationPolicyVersion}
			}
			o, e = h.Generation.Start(ctx, approved.Input.OwnerID, plan.Master, in, key, approval)
		}
		if e != nil {
			return report, e
		}
		for o.State != spoken.OperationPublished {
			if o.Terminal() || o.State == spoken.OperationReceived {
				return report, ErrQualificationEvidence
			}
			timer := time.NewTimer(h.Poll)
			select {
			case <-ctx.Done():
				timer.Stop()
				return report, ctx.Err()
			case <-timer.C:
			}
			o, e = h.Generation.Get(ctx, approved.Input.OwnerID, o.ID)
			if e != nil {
				return report, e
			}
		}
		report.Probes[phase] = o
		// Confirm each origin is usable and settled before another paid phase.
		if e = h.waitProbeSettlement(ctx, approved.Input.OwnerID, o); e != nil {
			return report, e
		}
		if _, e = h.auditProbe(ctx, approved.Input.OwnerID, o); e != nil {
			return report, e
		}
	}
	return h.Audit(ctx, report)
}
func verifyQualifiedJob(o spoken.Operation, j QualificationJob) error {
	if !j.Settled || len(j.Budgets) == 0 || j.Settlement.Reason != usage.OutcomeSucceeded {
		return ErrQualificationEvidence
	}
	total := new(big.Rat)
	seen := map[string]bool{}
	for _, b := range j.Budgets {
		if b.Count != 1 {
			return ErrQualificationEvidence
		}
		ref, operation := o.Profile.Design, o.Kind
		if o.Kind == spoken.JobKindProbe {
			ref, operation = o.Profile.Synthesis, "speech"
		}
		if b.PolicyID != o.Profile.ID || b.Revision != o.Profile.Revision || b.AuthorizationID != o.QualificationSessionID || b.ScopeDigest != o.ScopeDigest || b.Ref != ref || b.Operation != operation {
			return ErrQualificationEvidence
		}
		index := 0
		if b.Operation == "speech" {
			index = -1
			for i, digest := range o.SpeechInputs {
				if digest == b.InputDigest {
					index = i
					break
				}
			}
		}
		if index < 0 || index >= len(o.Evidence) {
			return ErrQualificationEvidence
		}
		e := o.Evidence[index]
		if e.RequestID == "" || seen[e.RequestID] {
			return ErrQualificationEvidence
		}
		seen[e.RequestID] = true
		// A reported dollar cost does not replace the applicable unit evidence
		// this milestone must qualify. Verified zero tariffs remain zero.
		unitUSD, unitSource := usage.UnitCost(b, llm.SpeechEvidence{Units: e.Units})
		if unitSource == llm.CostUnavailable {
			return ErrQualificationEvidence
		}
		unitCost, ok := new(big.Rat).SetString(unitUSD)
		if !ok || unitCost.Sign() < 0 {
			return ErrQualificationEvidence
		}
		usd, source := usage.UnitCost(b, e)
		if source == llm.CostUnavailable {
			return ErrQualificationEvidence
		}
		cost, ok := new(big.Rat).SetString(usd)
		if !ok || cost.Sign() < 0 {
			return ErrQualificationEvidence
		}
		cap, err := b.MaximumUSD()
		if err != nil || cost.Cmp(cap) > 0 || unitCost.Cmp(cap) > 0 {
			return ErrQualificationBudget
		}
		total.Add(total, cost)
	}
	actual, ok := new(big.Rat).SetString(j.ActualUSD)
	if !ok || actual.Cmp(total) != 0 {
		return ErrQualificationEvidence
	}
	return nil
}
func (h *QualificationHarness) sample(ctx context.Context, owner, id, request string, timing []llm.CharacterTiming) (QualificationSample, error) {
	a, e := h.Generation.Operations.GetAsset(ctx, owner, id)
	if e != nil || a.RevokedAt != nil {
		return QualificationSample{}, ErrQualificationEvidence
	}
	bytes, e := h.Generation.Objects.ReadSpokenAudio(ctx, a.ObjectKey, llm.SpeechMaxAudioBytes)
	if e != nil {
		return QualificationSample{}, e
	}
	decoded, e := llm.InspectSpeechAudio(ctx, bytes)
	if e != nil || decoded.SHA256 != a.SHA256 || int64(len(bytes)) != a.Bytes || decoded.Samples != a.Samples || decoded.SampleRate != a.SampleRate || decoded.Channels != a.Channels || decoded.Format != a.Format {
		return QualificationSample{}, ErrQualificationEvidence
	}
	return QualificationSample{AssetID: id, SHA256: a.SHA256, RequestID: request, DurationMS: decoded.Duration().Milliseconds(), CharacterTiming: timing}, nil
}
func (h *QualificationHarness) waitProbeSettlement(ctx context.Context, owner string, o spoken.Operation) error {
	for {
		settled := true
		for _, digest := range o.SpeechInputs {
			audio, err := h.Generation.Operations.GetProbeAudio(ctx, owner, o.VoiceID, digest)
			if err != nil {
				return err
			}
			accounting, err := h.Accounting.Job(ctx, owner, audio.JobID)
			if err != nil {
				return err
			}
			settled = settled && accounting.Settled
		}
		if settled {
			return nil
		}
		timer := time.NewTimer(h.Poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (h *QualificationHarness) auditProbe(ctx context.Context, owner string, o spoken.Operation) ([2]QualificationSample, error) {
	var samples [2]QualificationSample
	if o.State != spoken.OperationPublished || o.Kind != spoken.JobKindProbe {
		return samples, ErrQualificationEvidence
	}
	for i, digest := range o.SpeechInputs {
		audio, e := h.Generation.Operations.GetProbeAudio(ctx, owner, o.VoiceID, digest)
		if e != nil || audio.AssetID != o.AssetIDs[i] || i >= len(o.Evidence) || audio.Evidence.RequestID != o.Evidence[i].RequestID {
			return samples, ErrQualificationEvidence
		}
		origin, e := h.Generation.Get(ctx, owner, audio.OperationID)
		if e != nil {
			return samples, e
		}
		j, e := h.Accounting.Job(ctx, owner, audio.JobID)
		if e != nil {
			return samples, e
		}
		if e = verifyQualifiedJob(origin, j); e != nil {
			return samples, e
		}
		samples[i], e = h.sample(ctx, owner, audio.AssetID, audio.Evidence.RequestID, audio.Timing)
		if e != nil {
			return samples, e
		}
	}
	return samples, nil
}
func (h *QualificationHarness) Audit(ctx context.Context, r QualificationReport) (QualificationReport, error) {
	current, err := h.Plan(ctx, r.Plan.Input)
	if err != nil {
		return r, err
	}
	if current.Digest != r.Plan.Digest || current.MaximumUSD != r.Plan.MaximumUSD {
		return r, spoken.ErrConflict
	}
	q, d, c, v, e := h.source(ctx, r.Plan.Input)
	if e != nil {
		return r, e
	}
	if r.Design.ID != d.ID || r.Confirmation.ID != c.ID || r.Plan.Profile != v.Profile {
		return r, ErrQualificationEvidence
	}
	r.Design, r.Confirmation = d, c
	for i := range r.Probes {
		saved, err := h.Generation.Get(ctx, q.OwnerID, r.Probes[i].ID)
		if err != nil {
			return r, err
		}
		r.Probes[i] = saved
	}
	first, e := h.auditProbe(ctx, q.OwnerID, r.Probes[0])
	if e != nil {
		return r, e
	}
	changed, e := h.auditProbe(ctx, q.OwnerID, r.Probes[1])
	if e != nil {
		return r, e
	}
	if r.Probes[0].Texts != [2]string{r.Plan.Input.FirstText, r.Plan.Input.SecondText} || r.Probes[1].Texts != [2]string{r.Plan.Input.ChangedFirstText, r.Plan.Input.SecondText} || r.Probes[0].VoiceID != v.ID || r.Probes[1].VoiceID != v.ID || r.Probes[0].QualificationSessionID != q.ID || r.Probes[1].QualificationSessionID != q.ID || r.Probes[0].VoiceHandle != v.Handle || r.Probes[1].VoiceHandle != v.Handle || r.Probes[0].Profile != v.Profile || r.Probes[1].Profile != v.Profile || changed[1].AssetID != first[1].AssetID || changed[1].RequestID != first[1].RequestID || changed[0].RequestID == first[0].RequestID {
		return r, ErrQualificationEvidence
	}
	r.Samples = [3]QualificationSample{first[0], first[1], changed[0]}
	if r.Audition, e = h.sample(ctx, q.OwnerID, v.SampleAssetID, d.Evidence[0].RequestID, nil); e != nil {
		return r, e
	}
	seen := map[string]bool{}
	for _, id := range []string{d.Evidence[0].RequestID, c.Evidence[0].RequestID, first[0].RequestID, first[1].RequestID, changed[0].RequestID} {
		if id == "" || seen[id] {
			return r, ErrQualificationEvidence
		}
		seen[id] = true
	}
	jobs := map[string]bool{}
	actual := new(big.Rat)
	for _, o := range []spoken.Operation{d, c, r.Probes[0], r.Probes[1]} {
		ids := []string{o.JobID}
		if o.Kind == spoken.JobKindProbe {
			ids = nil
			for _, digest := range o.SpeechInputs {
				audio, e := h.Generation.Operations.GetProbeAudio(ctx, q.OwnerID, v.ID, digest)
				if e != nil {
					return r, e
				}
				ids = append(ids, audio.JobID)
			}
		}
		for _, id := range ids {
			if jobs[id] {
				continue
			}
			jobs[id] = true
			j, e := h.Accounting.Job(ctx, q.OwnerID, id)
			if e != nil {
				return r, e
			}
			usd, ok := new(big.Rat).SetString(j.ActualUSD)
			if !ok {
				return r, ErrQualificationEvidence
			}
			actual.Add(actual, usd)
		}
	}
	maximum, e := qualificationUSD(q.MaximumUSD)
	if e != nil || actual.Cmp(maximum) > 0 {
		return r, ErrQualificationBudget
	}
	r.ActualUSD = actual.RatString()
	return r, nil
}
func (h *QualificationHarness) Publish(ctx context.Context, r QualificationReport, reportID string) error {
	if reportID == "" || r.Review.Reviewer == "" || r.Review.ReviewedAt.IsZero() || r.Review.ReviewedAt.After(time.Now()) || !r.Review.AuditionAccepted || !r.Review.KoreanAccepted || !r.Review.ContinuityAccepted {
		return ErrQualificationEvidence
	}
	for _, note := range []string{r.Review.Names, r.Review.PricesNumbers, r.Review.Units, r.Review.Punctuation, r.Review.MixedLanguage, r.Review.OmissionsRepeats, r.Review.Clipping, r.Review.Continuity, r.Review.ConfirmationCapacity} {
		if strings.TrimSpace(note) == "" {
			return ErrQualificationEvidence
		}
	}
	verified, e := h.Audit(ctx, r)
	if e != nil {
		return e
	}
	for _, operation := range verified.Probes {
		if r.Review.ReviewedAt.Before(operation.UpdatedAt) {
			return ErrQualificationEvidence
		}
	}
	return h.Sessions.Publish(ctx, QualificationEvidence{OwnerID: r.Plan.Input.OwnerID, SessionID: r.Plan.Input.SessionID, ReportID: reportID, DesignRequestID: verified.Design.Evidence[0].RequestID, ConfirmRequestID: verified.Confirmation.Evidence[0].RequestID, ConfirmedVoiceID: verified.Confirmation.ResultID, SpeechRequestIDs: []string{verified.Samples[0].RequestID, verified.Samples[1].RequestID, verified.Samples[2].RequestID}, Review: r.Review})
}
