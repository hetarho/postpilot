package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

type analysisGuardInputs struct {
	AnalysisPreparationTx
	preparation clip.AnalysisPreparation
	fenceError  error
	fences      int
}

func (s *analysisGuardInputs) GetAnalysisPreparation(context.Context, string, string) (clip.AnalysisPreparation, error) {
	return s.preparation, nil
}
func (s *analysisGuardInputs) AuthorizeAnalysisPreparation(context.Context, clip.AnalysisPreparation, time.Time) error {
	s.fences++
	return s.fenceError
}

func TestBrowserHoldPreflightRejectsBeforeProviderAccessAndRechecksBeforeCredit(t *testing.T) {
	for _, name := range []string{"preparing", "accepted", "unqualified", "stale", "cancelled", "valid reuse"} {
		t.Run(name, func(t *testing.T) {
			j := prepareJob()
			raw, e := json.Marshal(clip.GenerationPayload{Version: clip.BrowserAnalysisGenerationPayloadVersion, Language: "ko", AnalysisPreparationID: "session", Approval: &clip.GenerationApproval{QuoteID: "quote"}})
			if e != nil {
				t.Fatal(e)
			}
			j.Payload = raw
			inputs := &analysisGuardInputs{preparation: clip.AnalysisPreparation{ID: "session", ProjectID: "clip", ParentJobID: j.ID, QuoteID: "quote", State: "consumed", ProfileVersion: clip.BrowserAnalysisProfileVersion}}
			switch name {
			case "preparing", "accepted":
				inputs.preparation.State = name
			case "unqualified":
				inputs.preparation.Copies = []clip.AnalysisCopy{{Slot: "copy"}}
			case "stale":
				inputs.fenceError = clip.ErrAnalysisPreparationState
			case "cancelled":
				at := time.Now()
				j.CancelRequestedAt = &at
			}
			jobs := &fakeJobs{jobs: map[string]job.Job{j.ID: j}}
			recorder := &txRecorder{}
			writer := sql.OpenDB(recorder)
			t.Cleanup(func() { writer.Close() })
			admission := &txAdmission{writer: recorder}
			access := &accessProbe{writer: recorder}
			bind := func(*sql.Tx) Ports { return Ports{Jobs: jobs, Analysis: inputs, Admission: admission} }
			guard := NewGuard(writer, bind, NewAnalysisDispatchAuthorizer(writer, bind, &fakeAuthorizer{}), access)
			e = guard.Reserve(t.Context(), holdFor(j))
			if name == "valid reuse" {
				if e != nil || access.calls != 1 || access.inTx || len(admission.held) != 1 || inputs.fences != 2 {
					t.Fatal("live reuse was not rechecked around external access", e, access.calls, inputs.fences)
				}
			} else if e == nil || access.calls != 0 || len(admission.held) != 0 {
				t.Fatal("invalid browser input reached provider access or credit hold", e, access.calls, len(admission.held))
			}
		})
	}
}
