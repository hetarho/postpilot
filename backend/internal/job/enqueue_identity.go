package job

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrEnqueueIdentityConflict = errors.New("job enqueue identity conflicts with persisted work")

// EnqueueWithID recovers one caller-owned admission even after a lost response,
// completion or payload purge. It never reissues a terminal or uncertain job.
func (q *Queue) EnqueueWithID(ctx context.Context, id string, input NewJob) (string, error) {
	if id == "" || len(id) > 256 || strings.TrimSpace(id) != id {
		return "", ErrEnqueueIdentityConflict
	}
	if err := validateNewJob(input); err != nil {
		return "", err
	}
	store, ok := q.store.(EnqueueIdentityStore)
	if !ok {
		return "", fmt.Errorf("enqueue stable job: identity persistence is required")
	}
	fingerprint, err := enqueueFingerprint(input)
	if err != nil {
		return "", err
	}
	identity := EnqueueIdentity{JobID: id, UserID: input.UserID, Fingerprint: fingerprint}
	// A queue process serializes its own admission window. The ledger's job-ID
	// idempotency and atomic identity insert cover another process's same-ID race.
	q.enqueueMu.Lock()
	defer q.enqueueMu.Unlock()
	if recovered, err := q.recoverEnqueue(ctx, store, identity); recovered || err != nil {
		return idIfRecovered(id, recovered), err
	}
	if err := store.ClaimEnqueueIdentity(ctx, identity, q.now()); err != nil {
		return "", err
	}
	if recovered, err := q.recoverEnqueue(ctx, store, identity); recovered || err != nil {
		return idIfRecovered(id, recovered), err
	}
	active, err := q.activeForInput(ctx, input)
	if err != nil {
		return "", err
	}
	if active != nil {
		if active.ID == id {
			recovered, err := q.recoverEnqueue(ctx, store, identity)
			if recovered || err != nil {
				return idIfRecovered(id, recovered), err
			}
		}
		return "", &ErrAlreadyInProgress{ActiveID: active.ID}
	}
	now := q.now()
	found := Job{ID: id, UserID: input.UserID, Kind: input.Kind, Subjects: cloneSubjects(input.Subjects), Status: StatusQueued,
		CancellationPolicyVersion: input.CancellationPolicyVersion, ObserveModel: input.ObserveModel, WriteModel: input.WriteModel,
		TargetLanguage: input.TargetLanguage, Payload: append([]byte(nil), input.Payload...), CreatedAt: now, UpdatedAt: now}
	if q.admitter != nil && !input.DeferHold && !input.NonMetered {
		if err := q.admitter.Hold(ctx, Start{UserID: input.UserID, Kind: input.Kind, JobID: id, Calls: input.plannedCalls()}); err != nil {
			if q.releaseAbandonedIdentity(ctx, store, identity) {
				return "", ErrEnqueueIdentityConflict
			}
			return "", err
		}
	}
	if err := store.InsertWithIdentity(ctx, found, identity); err != nil {
		// A commit can return an error after becoming durable. In particular, never
		// release this stable ID's hold while another matching writer may own it.
		// A remaining orphan is reconciled by SweepOpenHolds or reused by this ID.
		recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer cancel()
		if recovered, readErr := q.recoverEnqueue(recoveryCtx, store, identity); recovered || readErr != nil {
			if readErr != nil {
				q.releaseAbandonedIdentity(recoveryCtx, store, identity)
			}
			return idIfRecovered(id, recovered), readErr
		}
		return "", fmt.Errorf("insert stable job: %w", err)
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return id, nil
}

func (q *Queue) releaseAbandonedIdentity(ctx context.Context, store EnqueueIdentityStore, want EnqueueIdentity) bool {
	identity, err := store.GetEnqueueIdentity(ctx, want.JobID)
	if err == nil && identity.Abandoned && identity.UserID == want.UserID && identity.Fingerprint == want.Fingerprint {
		// The permanent abandonment CAS forbids every future row commit for
		// this identity, so it also fences a hold that raced the boot release.
		q.releaseAdmission(ctx, want.JobID)
		return true
	}
	return false
}

func idIfRecovered(id string, recovered bool) string {
	if recovered {
		return id
	}
	return ""
}
func (q *Queue) recoverEnqueue(ctx context.Context, store EnqueueIdentityStore, want EnqueueIdentity) (bool, error) {
	got, err := store.GetEnqueueIdentity(ctx, want.JobID)
	if errors.Is(err, ErrNotFound) {
		_, jobErr := q.store.GetByID(ctx, want.JobID)
		if errors.Is(jobErr, ErrNotFound) {
			return false, nil
		}
		if jobErr != nil {
			return false, jobErr
		}
		return false, ErrEnqueueIdentityConflict
	}
	if err != nil {
		return false, err
	}
	if got.Abandoned {
		return false, ErrEnqueueIdentityConflict
	}
	if got.JobID != want.JobID || got.UserID != want.UserID || got.Fingerprint != want.Fingerprint {
		return false, ErrEnqueueIdentityConflict
	}
	found, err := q.store.GetByID(ctx, want.JobID)
	if errors.Is(err, ErrNotFound) {
		if !got.Committed {
			return false, nil
		}
		return false, ErrEnqueueIdentityConflict
	}
	if err != nil {
		return false, err
	}
	if found.UserID != want.UserID {
		return false, ErrEnqueueIdentityConflict
	}
	return true, nil
}

func enqueueFingerprint(input NewJob) (string, error) {
	inputs := struct {
		Version                                                             int
		UserID, Kind, ObserveModel, WriteModel, TargetLanguage, PayloadHash string
		CancellationPolicyVersion                                           int
		NonMetered, DeferHold                                               bool
		Subjects                                                            []Subject
		Guards                                                              []Guard
		Calls                                                               []PlannedCall
	}{Version: 1, UserID: input.UserID, Kind: input.Kind, ObserveModel: input.ObserveModel, WriteModel: input.WriteModel,
		TargetLanguage: input.TargetLanguage, CancellationPolicyVersion: input.CancellationPolicyVersion, NonMetered: input.NonMetered, DeferHold: input.DeferHold,
		Subjects: cloneSubjects(input.Subjects), Guards: append([]Guard(nil), input.Guards...), Calls: input.plannedCalls()}
	hash := sha256.Sum256(input.Payload)
	inputs.PayloadHash = hex.EncodeToString(hash[:])
	sort.Slice(inputs.Subjects, func(i, j int) bool {
		a, b := inputs.Subjects[i], inputs.Subjects[j]
		if a.Dimension == b.Dimension {
			return a.ID < b.ID
		}
		return a.Dimension < b.Dimension
	})
	sort.Slice(inputs.Calls, func(i, j int) bool {
		a, b := inputs.Calls[i], inputs.Calls[j]
		if a.Ref != b.Ref {
			return a.Ref < b.Ref
		}
		if a.Stage != b.Stage {
			return a.Stage < b.Stage
		}
		if a.PromptTokens != b.PromptTokens {
			return a.PromptTokens < b.PromptTokens
		}
		return a.CompletionTokens < b.CompletionTokens
	})
	raw, err := json.Marshal(inputs)
	if err != nil {
		return "", err
	}
	hash = sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}
