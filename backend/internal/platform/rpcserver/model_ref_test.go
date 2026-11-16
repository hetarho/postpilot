package rpcserver

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestModelRefToProto(t *testing.T) {
	for value, want := range map[string]string{"p/m": "p/m", "p/m/x": "p/m/x", "p/": "", "/m": "", "pm": "", "": ""} {
		got := ModelRefToProto(value)
		if want == "" {
			if got != nil {
				t.Errorf("ModelRefToProto(%q) = %v, want nil", value, got)
			}
			continue
		}
		if got == nil || ModelRefFromProto(got) != want {
			t.Errorf("ModelRefToProto(%q) = %v, want %q", value, got, want)
		}
	}
	if got := ModelRefToProto("p/m/x"); got.GetProviderId() != "p" || got.GetModelId() != "m/x" {
		t.Errorf("split at first slash: %v", got)
	}
}

func TestModelRefFromProto(t *testing.T) {
	if got := ModelRefFromProto(nil); got != "" {
		t.Errorf("nil ref = %q", got)
	}
	if got := ModelRefFromProto(&postpilotv1.ModelRef{ProviderId: "p"}); got != "" {
		t.Errorf("half ref = %q", got)
	}
	if got := ModelRefFromProto(&postpilotv1.ModelRef{ProviderId: "p", ModelId: "m"}); got != "p/m" {
		t.Errorf("ref = %q", got)
	}
}

func TestFailureToProto(t *testing.T) {
	if FailureToProto("", map[string]string{"k": "v"}, "d") != nil {
		t.Error("empty reason must be nil")
	}
	got := FailureToProto("BOOM", nil, "d")
	if got.GetReason() != "BOOM" || got.GetTechnicalDetail() != "d" || got.Params == nil || len(got.Params) != 0 {
		t.Errorf("nil params: %+v", got)
	}
	in := map[string]string{"k": "v"}
	got = FailureToProto("BOOM", in, "")
	in["k"] = "changed"
	if got.Params["k"] != "v" {
		t.Errorf("params not copied: %+v", got.Params)
	}
}
