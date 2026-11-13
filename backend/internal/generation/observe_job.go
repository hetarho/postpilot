package generation

import (
	"context"
	"encoding/json"
	"fmt"
)

// StartObservationRequest is 관찰하기 or 다시 관찰하기 on the way in (GEN-89). ObserveFiles is the
// selection with GEN-9's presence: nil observes every attachment, present observes its
// intersection with them plus the ones with nothing to reuse. ObserveCalls and
// OriginProtocolVersion are resolved by StartObservation for the row and the hold.
type StartObservationRequest struct {
	OriginProtocolVersion int
	UserID                string
	PostSlug              string
	ObserveModel          string
	ObserveFiles          *[]string
	ObserveCalls          int
}

// ObservationJob is one queued observation job as the worker hands it over.
type ObservationJob struct {
	ID           string
	UserID       string
	PostSlug     string
	ObserveModel string
	Payload      []byte
}

// observeOptions is what an observation job froze (GEN-30): the selection, carrying presence,
// and the reusable snapshot from the same read, and nothing a writer reads.
type observeOptions struct {
	OriginProtocolVersion int
	ObserveFiles          *[]string
	Observations          []Observation
}

// observePayload keeps ObserveFiles without omitempty for the reason generationPayload does: an
// empty set encoded as absent would decode as "observe everything".
type observePayload struct {
	OriginProtocolVersion int                  `json:"origin_protocol_version,omitempty"`
	ObserveFiles          *[]string            `json:"observe_files"`
	Observations          []observationPayload `json:"observations,omitempty"`
}

// missingObservationsOnly is every writing run's selection: present and empty, which observes only
// the attachments with nothing to reuse (GEN-8, GEN-89).
func missingObservationsOnly() *[]string {
	none := []string{}
	return &none
}

// StartObservation freezes an observation job under the ordinary start's ownership check, the
// published lock and the one-active-job guard (GEN-23, GEN-56, POST-74). It writes nothing a
// writer reads, so it takes no write model, voice, language or template check; an observe model
// is required only when something is attached.
func (s *Service) StartObservation(ctx context.Context, request StartObservationRequest) (string, error) {
	in, err := s.startPreconditions(ctx, request.UserID, request.PostSlug, startStage{
		observeOnly: true,
		observe:     observePicked, observeModel: request.ObserveModel, observeFiles: request.ObserveFiles,
	})
	if err != nil {
		return "", err
	}
	request.OriginProtocolVersion = s.originProtocol
	request.ObserveModel = in.observe.model
	request.ObserveCalls = in.observe.calls
	payload, err := encodeObservePayload(observeOptions{
		OriginProtocolVersion: s.originProtocol,
		ObserveFiles:          in.observe.files, Observations: in.observe.observations,
	})
	if err != nil {
		return "", fmt.Errorf("encode observation payload: %w", err)
	}
	id, err := s.jobs.EnqueueObservation(ctx, request, payload)
	if err != nil {
		return "", fmt.Errorf("enqueue observation: %w", err)
	}
	return id, nil
}

// Observe handles one observation job: the observe step a writing run takes, over the frozen
// set and snapshot, persisting the merged snapshot after every batch (GEN-11). Content, storyline,
// status, revisions and baseline are never touched; a rotation follows the observation through
// the post's own SetObservations (GEN-79, POST-107).
func (s *Service) Observe(ctx context.Context, job ObservationJob, progress Progress) error {
	options, err := decodeObservePayload(job.Payload)
	if err != nil {
		return err
	}
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load observation input: %w", err)
	}
	ctx, finishCapture := s.beginRequestCapture(ctx, job.ID, post)
	defer finishCapture()
	if post.Published {
		return ErrPostPublished
	}
	post.OriginProtocolVersion = options.OriginProtocolVersion
	_, _, err = s.observeForRun(ctx, post, job.ObserveModel, options.ObserveFiles, options.Observations, progress)
	return err
}

func encodeObservePayload(options observeOptions) ([]byte, error) {
	return json.Marshal(observePayload{
		OriginProtocolVersion: options.OriginProtocolVersion,
		ObserveFiles:          cloneOptionalTexts(options.ObserveFiles),
		Observations:          encodeObservations(options.Observations),
	})
}

func decodeObservePayload(raw []byte) (observeOptions, error) {
	var payload observePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return observeOptions{}, fmt.Errorf("decode observation payload: %w", err)
	}
	if payload.OriginProtocolVersion != 0 && payload.OriginProtocolVersion != OriginProtocolVersion {
		return observeOptions{}, fmt.Errorf("decode observation payload: unknown origin protocol")
	}
	return observeOptions{
		OriginProtocolVersion: payload.OriginProtocolVersion,
		ObserveFiles:          cloneOptionalTexts(payload.ObserveFiles),
		Observations:          decodeObservations(payload.Observations),
	}, nil
}
