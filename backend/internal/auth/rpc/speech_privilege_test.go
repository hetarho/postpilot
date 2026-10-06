package rpc

import (
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"testing"
)

func TestSpeechCurationAndQualificationAreInTheClosedMasterSet(t *testing.T) {
	for _, procedure := range []string{postpilotv1connect.SpeechProfileServiceAdminListSpeechProfilesProcedure, postpilotv1connect.SpeechProfileServiceSaveSpeechProfileProcedure, postpilotv1connect.SpeechProfileServiceStartSpeechQualificationProcedure, postpilotv1connect.SpeechProfileServiceRegisterSpeechCombinationProcedure, postpilotv1connect.SpeechProfileServiceSaveSpeechTariffProcedure} {
		if !masterProcedures[procedure] {
			t.Errorf("unprivileged speech operator procedure: %s", procedure)
		}
	}
	if masterProcedures[postpilotv1connect.SpeechProfileServiceListSpeechProfilesProcedure] {
		t.Fatal("customer picker incorrectly requires master")
	}
	for _, procedure := range []string{postpilotv1connect.SpokenVoiceGenerationServiceQuoteVoiceReuseProbeProcedure, postpilotv1connect.SpokenVoiceGenerationServiceStartVoiceReuseProbeProcedure} {
		if !masterProcedures[procedure] {
			t.Fatal("probe not master-only", procedure)
		}
	}
	if masterProcedures[postpilotv1connect.SpokenVoiceGenerationServiceStartVoiceCandidatesProcedure] {
		t.Fatal("customer voice creation incorrectly requires master")
	}
}
