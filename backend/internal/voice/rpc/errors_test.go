package rpc

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
)

func TestVoiceSynchronousErrorsHaveStableDetails(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		mapErr func(error) error
		code   connect.Code
		reason string
		params map[string]string
	}{
		{name: "sample too short", err: &voice.SampleTooShortError{Chars: 17}, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeInvalidArgument, reason: "VOICE_SAMPLE_TOO_SHORT", params: map[string]string{"actual": "17", "min": "200"}},
		{name: "name required", err: &voice.VoiceNameError{}, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeInvalidArgument, reason: "VOICE_NAME_REQUIRED"},
		{name: "name too long", err: &voice.VoiceNameError{Chars: 51}, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeInvalidArgument, reason: "VOICE_NAME_TOO_LONG", params: map[string]string{"actual": "51", "max": "50"}},
		{name: "not made", err: voice.ErrVoiceNotMade, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeFailedPrecondition, reason: "VOICE_NOT_MADE"},
		{name: "sample mutation", err: errors.Join(voice.ErrSampleMutation, errors.New("private database state")), mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeInternal, reason: "VOICE_SAMPLE_MUTATION_FAILED"},
		{name: "lifecycle", err: voice.ErrInvalidLifecycle, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeFailedPrecondition, reason: "VOICE_INVALID_LIFECYCLE"},
		{name: "unknown profile version", err: voice.ErrLearningNotFound, mapErr: func(err error) error { return toConnectError("test", err) }, code: connect.CodeNotFound, reason: "VOICE_LEARNING_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapped := test.mapErr(test.err)
			if got := connect.CodeOf(mapped); got != test.code {
				t.Fatalf("code = %v, want %v", got, test.code)
			}
			detail := voiceAppErrorDetail(t, mapped)
			if detail.GetReason() != test.reason || !reflect.DeepEqual(detail.GetParams(), test.params) {
				t.Fatalf("detail = %#v, want reason %q params %#v", detail, test.reason, test.params)
			}
		})
	}
}

func TestVoiceUnknownErrorDoesNotLeakPrivateText(t *testing.T) {
	mapped := toConnectError("analyze voice", errors.New("private SQL DSN and provider payload"))
	detail := voiceAppErrorDetail(t, mapped)
	if connect.CodeOf(mapped) != connect.CodeInternal || detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("mapped error = %v, detail = %#v", mapped, detail)
	}
	if strings.Contains(mapped.Error(), "private") || strings.Contains(mapped.Error(), "provider payload") {
		t.Fatalf("private text leaked: %v", mapped)
	}
}

func voiceAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatal(valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	return detail
}
