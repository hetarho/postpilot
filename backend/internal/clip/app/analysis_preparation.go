package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// These clip-owned behaviors join parent/continuation guards through the same
// writer. Neither port has provider or credit authority.
type AnalysisPreparationTx interface {
	GetAnalysisPreparation(context.Context, string, string) (clip.AnalysisPreparation, error)
	AnalysisPreparationForQuote(context.Context, string, string) (clip.AnalysisPreparation, error)
	AnalysisPreparationInputs(context.Context, string, clip.AnalysisPreparationInput, time.Time) (clip.GenerationQuote, clip.SourceBatch, *clip.RecoveryState, error)
	AuthorizeAnalysisPreparation(context.Context, clip.AnalysisPreparation, time.Time) error
	BeginAnalysisPreparation(context.Context, clip.AnalysisPreparation, clip.AnalysisPreparationLimits, time.Time) error
	ReserveAnalysisCopy(context.Context, clip.AnalysisPreparation, string, int64, string, string, time.Time, time.Time) (clip.AnalysisCopy, error)
	BindAnalysisPreparation(context.Context, clip.AnalysisPreparation, string, time.Time) error
	SubmitAnalysisPreparation(context.Context, clip.AnalysisPreparation, string, time.Time, time.Time, time.Time) error
	ClaimAnalysisVerification(context.Context, clip.MediaWorkerProfile, clip.AnalysisPreparationLimits, time.Time) (*clip.MediaLease, error)
	AuthorizeAnalysisLease(context.Context, clip.MediaLeaseCredentials, time.Time, bool) (clip.AnalysisPreparation, error)
	RenewAnalysisLease(context.Context, clip.MediaLeaseCredentials, int, clip.AnalysisPreparationLimits, time.Time) (time.Time, error)
	AcceptAnalysisReceipt(context.Context, clip.MediaLeaseCredentials, string, clip.AnalysisVerificationResult, time.Time) error
	SetAnalysisPreparationState(context.Context, string, string, string) error
	ConsumeAnalysisPreparation(context.Context, string, string, time.Time) error
	StopAnalysisAttempt(context.Context, string, string, time.Time) error
	AnalysisLeaseStopped(context.Context, clip.AnalysisPreparation, time.Time) (bool, error)
	RetireAnalysisPreparation(context.Context, string, time.Time) error
	MarkAnalysisPreparationReconciled(context.Context, string, time.Time) error
}
type AnalysisPreparationStore interface {
	GetAnalysisPreparation(context.Context, string, string) (clip.AnalysisPreparation, error)
	AnalysisPreparationsForRecovery(context.Context, string) ([]clip.AnalysisPreparation, error)
	DueAnalysisCopyCleanup(context.Context, time.Time) ([]clip.MediaDeletion, error)
	FinishAnalysisCopyCleanup(context.Context, string) error
	QueueAnalysisOrphan(context.Context, string, time.Time) error
	AnalysisVerificationStatus(context.Context, string, int, time.Time) (clip.MediaRuntimeStatus, error)
}
type AnalysisPreparationObjects interface {
	MediaArtifactObjects
	Delete(context.Context, string) error
	ListAnalysisCopies(context.Context) ([]clip.StoredObject, error)
}
type AnalysisPreparationJobs interface {
	CancelJob(context.Context, string, string, string) error
	FailWait(context.Context, string, string, job.Failure) (bool, error)
	AcknowledgeWaitCancellation(context.Context, string, string) (bool, error)
}

type AnalysisPreparations struct {
	writer         *sql.DB
	bind           Binder
	store          AnalysisPreparationStore
	objects        AnalysisPreparationObjects
	jobs           AnalysisPreparationJobs
	cfg            clip.MediaConfig
	limits         clip.AnalysisPreparationLimits
	now            func() time.Time
	qualified      func(string) bool
	recoveryMu     sync.Mutex
	recoveryCursor string
}

func NewAnalysisPreparations(writer *sql.DB, bind Binder, store AnalysisPreparationStore, objects AnalysisPreparationObjects, jobs AnalysisPreparationJobs, cfg clip.MediaConfig, limits clip.AnalysisPreparationLimits, now func() time.Time) *AnalysisPreparations {
	if writer == nil || bind == nil || store == nil || objects == nil || jobs == nil || limits.Validate() != nil {
		panic("clip: analysis preparations require bounded owned collaborators")
	}
	if now == nil {
		now = time.Now
	}
	return &AnalysisPreparations{writer: writer, bind: bind, store: store, objects: objects, jobs: jobs, cfg: cfg, limits: limits, now: now, qualified: clip.BrowserAnalysisQualified}
}

func analysisManifestDigest(p clip.AnalysisPreparation) string {
	data, _ := json.Marshal(struct {
		Version                                                                 int
		ID, UserID, ProjectID, BatchID, QuoteID, ProfileVersion, RecoveryDigest string
		Revision                                                                int
		Originals                                                               []clip.BrowserOriginalMeasurement
		Sources                                                                 []clip.AnalysisSource
		Reused                                                                  []clip.AnalysisChunk
		Copies                                                                  []clip.AnalysisCopy
	}{1, p.ID, p.UserID, p.ProjectID, p.BatchID, p.QuoteID, p.ProfileVersion, p.RecoveryDigest, p.ExpectedRevision, p.Originals, p.Sources, p.Reused, p.Copies})
	return clip.MediaPayloadDigest(string(data))
}
func AnalysisPreparationWaitKey(id string) string { return "clip-browser-analysis:" + id }

func (a *AnalysisPreparations) parent(ctx context.Context, p Ports, s clip.AnalysisPreparation, allowQueued bool) error {
	if s.ParentJobID == "" {
		return nil
	}
	j, e := p.Jobs.GetByID(ctx, s.ParentJobID)
	if e != nil {
		return clip.ErrAnalysisPreparationState
	}
	if j.UserID != s.UserID || j.Subject(clip.JobSubject) != s.ProjectID || !clip.PreparesMedia(j.Kind) || j.CancelRequestedAt != nil || j.Status != job.StatusRunning && (!allowQueued || j.Status != job.StatusQueued) {
		return clip.ErrMediaCancelled
	}
	var payload clip.GenerationPayload
	if clip.StrictJSON(string(j.Payload), &payload) != nil || payload.AnalysisPreparationID != s.ID || payload.Batch.ID != s.BatchID || payload.Approval == nil || payload.Approval.QuoteID != s.QuoteID {
		return clip.ErrAnalysisPreparationState
	}
	return nil
}
func (a *AnalysisPreparations) owned(ctx context.Context, p Ports, user, id string) (clip.AnalysisPreparation, error) {
	if p.Analysis == nil {
		return clip.AnalysisPreparation{}, clip.ErrMediaUnsupported
	}
	s, e := p.Analysis.GetAnalysisPreparation(ctx, user, id)
	if e != nil {
		return s, e
	}
	if e = p.Analysis.AuthorizeAnalysisPreparation(ctx, s, a.now().UTC()); e != nil {
		return s, e
	}
	e = a.parent(ctx, p, s, true)
	return s, e
}

func (a *AnalysisPreparations) Begin(ctx context.Context, user string, in clip.AnalysisPreparationInput) (out clip.AnalysisPreparation, err error) {
	if in.ProfileVersion != clip.BrowserAnalysisProfileVersion || !clip.ValidMediaLabel(user) || !clip.ValidMediaLabel(in.ProjectID) || !clip.ValidMediaLabel(in.BatchID) || !clip.ValidMediaLabel(in.QuoteID) || in.ExpectedRevision < 0 {
		return out, clip.ErrInvalid
	}
	for i := range in.Originals {
		in.Originals[i].Provenance = clip.BrowserOriginalProvenance
	}
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		if p.Analysis == nil {
			return clip.ErrMediaUnsupported
		}
		old, e := p.Analysis.AnalysisPreparationForQuote(ctx, user, in.QuoteID)
		if e == nil {
			if old.ProjectID != in.ProjectID || old.BatchID != in.BatchID || old.ExpectedRevision != in.ExpectedRevision || old.ProfileVersion != in.ProfileVersion || !reflect.DeepEqual(old.Originals, in.Originals) {
				return clip.ErrMediaConflict
			}
			out, e = a.owned(ctx, p, user, old.ID)
			return e
		}
		if !errors.Is(e, clip.ErrNotFound) {
			return e
		}
		now := a.now().UTC()
		q, b, r, e := p.Analysis.AnalysisPreparationInputs(ctx, user, in, now)
		if e != nil {
			return e
		}
		if e = clip.ValidateBrowserOriginals(b, in.Originals, a.cfg); e != nil {
			return e
		}
		project, e := p.Clips.GetProject(ctx, user, in.ProjectID)
		if e != nil {
			return e
		}
		busy, e := p.Jobs.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: in.ProjectID}, job.Filter{})
		if e != nil {
			return e
		}
		if busy != nil {
			return clip.ErrBusy
		}
		out = clip.AnalysisPreparation{ID: newID(), UserID: user, ProjectID: in.ProjectID, BatchID: in.BatchID, QuoteID: in.QuoteID, ProfileVersion: in.ProfileVersion, ExpectedRevision: in.ExpectedRevision, OriginalDigest: q.InputDigest, RecoveryDigest: q.Pricing.RecoveryDigest, Originals: in.Originals, State: "preparing", CreatedAt: now, ExpiresAt: now.Add(a.limits.TTL), QueueDeadlineAt: now.Add(a.limits.TTL), DeadlineAt: now.Add(a.limits.TTL)}
		coverage := slices.Clone(in.Originals)
		for i, m := range in.Originals {
			source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: m.SourceID, Fingerprint: m.Fingerprint, Info: m.Info}, Filename: b.Sources[i].Filename, OriginalMeasurementProvenance: clip.BrowserOriginalProvenance}
			if r != nil && r.Version == 1 && r.Contract == clip.AnalysisContractVersion && r.Observe == q.Pricing.Observe.Ref && r.Language == project.Language {
				for _, cached := range r.Sources {
					if !recoverySourceMatches(cached, b.Sources[i]) || clip.ValidateAnalysisSources(clip.DefaultAnalysisLimits(), []clip.AnalysisSource{cached}) != nil {
						continue
					}
					if cached.Info.HasAudio != m.Info.HasAudio {
						return clip.ErrInvalidMedia
					}
					source = cached
					coverage[i].Info = cached.Info
					for _, c := range r.Chunks {
						if c.SourceID == cached.ID && c.Fingerprint == cached.Fingerprint && clip.ValidateChunkInput(clip.DefaultAnalysisLimits(), clip.ChunkInput{Source: cached, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}) == nil && clip.ValidateSegments(clip.DefaultAnalysisLimits(), c.Segments, c.OffsetMS, c.OffsetMS+c.DurationMS) == nil {
							found := slices.ContainsFunc(out.Reused, func(v clip.AnalysisChunk) bool { return v.SourceID == c.SourceID && v.Index == c.Index })
							if !found {
								out.Reused = append(out.Reused, clip.AnalysisChunk{SourceID: c.SourceID, Fingerprint: c.Fingerprint, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS})
							}
						}
					}
				}
			}
			out.Sources = append(out.Sources, source)
		}
		out.Copies, e = clip.ExpectedAnalysisCopies(coverage, out.Reused, a.cfg)
		if e != nil {
			return e
		}
		if len(out.Reused) != q.Pricing.ReusedChunks || len(out.Copies) > q.Pricing.ObservationCalls {
			return clip.ErrQuoteChanged
		}
		out.QuoteManifestDigest = analysisManifestDigest(out)
		out.ManifestDigest = out.QuoteManifestDigest
		out.BoundQuoteDigest = clip.AnalysisBoundQuoteDigest(out.OriginalDigest, out.QuoteManifestDigest)
		return p.Analysis.BeginAnalysisPreparation(ctx, out, a.limits, now)
	})
	return
}

func (a *AnalysisPreparations) Reserve(ctx context.Context, user, id, slot string, bytes int64, digest string) (access clip.MediaArtifactAccess, expires time.Time, err error) {
	var session clip.AnalysisPreparation
	var copy clip.AnalysisCopy
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		var e error
		session, e = a.owned(ctx, p, user, id)
		if e != nil {
			return e
		}
		if session.State != "preparing" || !slices.ContainsFunc(session.Copies, func(c clip.AnalysisCopy) bool { return c.Slot == slot }) {
			return clip.ErrAnalysisPreparationState
		}
		now := a.now().UTC()
		expires = minAnalysisTime(now.Add(a.limits.PutTTL), session.ExpiresAt)
		key := clip.AnalysisPreparationPrefix + url.PathEscape(user) + "/" + session.ID + "/" + newID() + ".mp4"
		copy, e = p.Analysis.ReserveAnalysisCopy(ctx, session, slot, bytes, digest, key, expires, now)
		return e
	})
	if err != nil {
		return
	}
	access, err = a.objects.PresignMediaWrite(ctx, copy.ObjectKey, "video/mp4", copy.Bytes, max(expires.Sub(a.now()), time.Millisecond))
	if err != nil {
		return clip.MediaArtifactAccess{}, time.Time{}, errors.New("analysis copy upload signing failed")
	}
	access.Slot = copy.Slot
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error { _, e := a.owned(ctx, p, user, id); return e })
	return
}
func minAnalysisTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Complete is also polling. Storage observations are followed by the same
// owner/revision/parent guard in the transaction that freezes the handoff.
func (a *AnalysisPreparations) Complete(ctx context.Context, user, id string) (out clip.AnalysisPreparation, err error) {
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error { var e error; out, e = a.owned(ctx, p, user, id); return e })
	if err != nil {
		return
	}
	if out.State != "preparing" {
		return
	}
	if out.ParentJobID == "" {
		return out, clip.ErrAnalysisPreparationState
	}
	for _, c := range out.Copies {
		if c.State != "reserved" || c.Bytes <= 0 || !clip.ValidSHA256(c.Digest) {
			return out, clip.ErrInvalidMedia
		}
		h, e := a.objects.HeadMediaArtifact(ctx, c.ObjectKey)
		if e != nil || h.Bytes != c.Bytes || h.ContentType != "video/mp4" {
			return out, clip.ErrInvalidMedia
		}
	}
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		current, e := a.owned(ctx, p, user, id)
		if e != nil {
			return e
		}
		if current.State != "preparing" {
			out = current
			return nil
		}
		if !reflect.DeepEqual(current.Copies, out.Copies) {
			return clip.ErrMediaConflict
		}
		now := a.now().UTC()
		digest := analysisManifestDigest(current)
		if e = p.Analysis.SubmitAnalysisPreparation(ctx, current, digest, now.Add(a.limits.Stages.WaitTimeout), now.Add(a.limits.Stages.StageTimeout), now); e != nil {
			return e
		}
		out, e = p.Analysis.GetAnalysisPreparation(ctx, user, id)
		if e != nil {
			return e
		}
		if out.State == "accepted" {
			_, e = p.Waits.Wake(ctx, out.ParentJobID, AnalysisPreparationWaitKey(out.ID), now)
		}
		return e
	})
	return
}

func (a *AnalysisPreparations) Cancel(ctx context.Context, user, id string) (out clip.AnalysisPreparation, err error) {
	out, err = a.store.GetAnalysisPreparation(ctx, user, id)
	if err != nil {
		return
	}
	if !clip.AnalysisPreparationLive(out.State) {
		return
	}
	if out.ParentJobID != "" {
		if err = a.jobs.CancelJob(ctx, user, out.ProjectID, out.ParentJobID); err != nil {
			return
		}
	}
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		current, e := p.Analysis.GetAnalysisPreparation(ctx, user, id)
		if e != nil {
			return e
		}
		if !clip.AnalysisPreparationLive(current.State) {
			out = current
			return nil
		}
		if current.ParentJobID != "" {
			j, e := p.Jobs.GetByID(ctx, current.ParentJobID)
			if e != nil {
				return e
			}
			if job.Terminal(j.Status) && j.CancelRequestedAt == nil {
				out = current
				return nil
			}
			if j.CancelRequestedAt == nil {
				return clip.ErrAnalysisPreparationState
			}
		}
		if e = p.Analysis.SetAnalysisPreparationState(ctx, id, "cancelled", ""); e != nil {
			return e
		}
		out, e = p.Analysis.GetAnalysisPreparation(ctx, user, id)
		return e
	})
	return
}
