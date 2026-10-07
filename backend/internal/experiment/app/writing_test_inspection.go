package app

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/llm"
)

type WritingTestInspection struct {
	store experiment.TestInspectionStorage
}

func NewWritingTestInspection(store experiment.TestInspectionStorage) *WritingTestInspection {
	if store == nil {
		panic("experiment/app: private inspection storage is required")
	}
	return &WritingTestInspection{store: store}
}

func (a *WritingTestInspection) ReadTestRequestInspection(ctx context.Context, user, test, candidate, stage string, status llm.InspectionStatus) (llm.RequestInspection, error) {
	values, err := a.ReadTestRequestInspections(ctx, user, test, candidate, stage, status)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	return values[len(values)-1], nil
}

// Blind tests deny the entire identity-bearing projection, including fragment
// names, source inventories, omissions and effective model conditions. We never
// attempt substring redaction of a model-specific prompt or supplier error.
func (a *WritingTestInspection) ReadTestRequestInspections(ctx context.Context, user, test, candidate, stage string, status llm.InspectionStatus) ([]llm.RequestInspection, error) {
	canonical, ok := testInspectionStage(stage)
	if !ok || (status != llm.InspectionCurrent && status != llm.InspectionPrepared && status != llm.InspectionCaptured) {
		return nil, llm.ErrInvalidInspection
	}
	work, err := a.store.ReadTestInspectionWork(ctx, user, test)
	if err != nil {
		return nil, err
	}
	index, found := 0, false
	var output []byte
	for _, entrant := range work.Test.Candidates {
		if entrant.ID == candidate {
			index, found, output = entrant.SnapshotIndex, true, entrant.Output
			break
		}
	}
	if !found {
		return nil, experiment.ErrTestNotFound
	}
	unavailable := func(reason string) ([]llm.RequestInspection, error) {
		value := llm.UnavailableRequestInspection(canonical, "writing-test")
		value.UnavailableReason = reason
		return []llm.RequestInspection{value}, nil
	}
	if work.Test.PurgeFence != 0 || (work.Test.ContentExpiresAt != nil && !work.Test.ContentExpiresAt.After(time.Now())) {
		return unavailable("private_test_payload_expired_or_purged")
	}
	if work.Test.Status != experiment.TestCompleted && work.Test.Status != experiment.TestCancelled {
		return unavailable("blind_test_identity_hidden_until_reveal")
	}
	if status != llm.InspectionCaptured {
		return unavailable("test_view_requires_retained_execution_capture")
	}
	var values []llm.RequestInspection
	seen := map[string]bool{}
	appendValues := func(inspections []llm.RequestInspection) {
		for _, value := range inspections {
			actual, ok := testInspectionStage(value.Stage)
			if !ok || actual != canonical || value.Status != llm.InspectionCaptured || value.Validate() != nil {
				continue
			}
			if value.CallID != "" && seen[value.CallID] {
				continue
			}
			if value.CallID != "" {
				seen[value.CallID] = true
			}
			values = append(values, llm.CloneRequestInspection(value))
		}
	}
	// Checkpoint evidence is scoped by immutable snapshot hash and entrant slot.
	// Successful output also carries its exact captures for replay/recovery.
	for _, slot := range []struct {
		raw   []byte
		index int
	}{{work.SharedCheckpoint, -1}, {work.CandidateCheckpoints[candidate], index}} {
		checkpoint, err := decodeTestCheckpoint(slot.raw)
		if err == nil && checkpoint != nil && checkpoint.SnapshotHash == work.Test.CommonHash && checkpoint.Index == slot.index {
			appendValues(checkpoint.RequestInspections)
		}
	}
	if value, err := experiment.DecodeTestOutput(output); err == nil {
		appendValues(value.RequestInspections)
	}
	if len(values) == 0 {
		return unavailable("capture_missing_stale_or_purged")
	}
	return values, nil
}

func testInspectionStage(stage string) (string, bool) {
	switch stage {
	case "observe", "post-observation":
		return "observe", true
	case "write", "post-writing":
		return "write", true
	default:
		return "", false
	}
}

var _ experiment.TestRequestInspection = (*WritingTestInspection)(nil)
var _ experiment.TestRequestInspections = (*WritingTestInspection)(nil)
