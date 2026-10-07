package rpc

import (
	"errors"
	"reflect"
	"testing"
	"time"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func requestInspectionFixture() llm.RequestInspection {
	zero, budget := int64(0), int64(8192)
	characters, bytes, estimate := int64(3), int64(10), int64(2)
	completion := int64(400)
	effort := llm.ReasoningUnspecified
	structured := false
	issued := time.Date(2026, time.October, 7, 1, 2, 3, 456, time.UTC)
	return llm.RequestInspection{
		Version: llm.RequestInspectionVersion, Status: llm.InspectionCaptured,
		Stage: "post.write", Mode: "new", PromptVersion: "write-v1", SchemaVersion: "post-v1",
		Fragments: []llm.RequestFragment{
			{ID: "instructions", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "instruction", Text: "Write from evidence.\n"},
			{ID: "memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "evidence", Text: "카페😀", SourceRefs: []string{"memo-1", "photo-1"}},
			{ID: "history", Role: llm.InspectionRoleAssistant, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "history", Text: "  cafe\u0301\t\n"},
		},
		SelectedRuleIDs: []string{"stock-2", "owner-1"},
		Output:          llm.OutputContractInspection{Name: "post", Version: "post-v1", Schema: `{"title":"string"}`},
		Conditions: &llm.EffectiveRequestConditions{
			Model:               &llm.ModelRef{ProviderID: "catalog", ModelID: "writer"},
			MaxCompletionTokens: &budget, ReasoningEffort: &effort, StructuredOutput: &structured,
		},
		Measures: llm.InspectionMeasures{
			Characters: &characters, UTF8Bytes: &bytes, ReferenceTokenEstimate: &estimate,
			ProviderPromptTokens: &zero, ProviderCompletionTokens: &completion, ProviderReasoningTokens: &zero,
		},
		IssuedAt: &issued,
	}
}

func TestRequestInspectionProtoRoundTripPreservesOrderVersionsTextAndPresence(t *testing.T) {
	value := requestInspectionFixture()
	wire, err := RequestInspectionToProto(value)
	if err != nil {
		t.Fatal(err)
	}
	if wire.Conditions.ReasoningEffort == nil || *wire.Conditions.ReasoningEffort != "" || wire.Conditions.StructuredOutput == nil || *wire.Conditions.StructuredOutput {
		t.Fatal("known empty effort or false structured-output decision lost presence")
	}
	if wire.Measures.ProviderPromptTokens == nil || *wire.Measures.ProviderPromptTokens != 0 || wire.Measures.ProviderReasoningTokens == nil || *wire.Measures.ProviderReasoningTokens != 0 {
		t.Fatal("reported actual zero lost presence")
	}
	encoded, err := proto.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &postpilotv1.RequestInspection{}
	if err := proto.Unmarshal(encoded, decoded); err != nil {
		t.Fatal(err)
	}
	restored, err := RequestInspectionFromProto(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, value) {
		t.Fatalf("inspection fields changed across protobuf: got %+v want %+v", restored, value)
	}
}

func TestRequestInspectionProtoPreservesAbsentUnknownConditionsAndMeasures(t *testing.T) {
	for _, status := range []llm.InspectionStatus{llm.InspectionCurrent, llm.InspectionPrepared, llm.InspectionCaptured} {
		t.Run(string(status), func(t *testing.T) {
			value := requestInspectionFixture()
			value.Status, value.Conditions, value.Measures, value.IssuedAt = status, nil, llm.InspectionMeasures{}, nil
			wire, err := RequestInspectionToProto(value)
			if err != nil {
				t.Fatal(err)
			}
			if wire.Conditions != nil || wire.Measures != nil || wire.IssuedAt != nil {
				t.Fatal("absent runtime information acquired presence")
			}
			got, err := RequestInspectionFromProto(wire)
			if err != nil || !reflect.DeepEqual(got, value) {
				t.Fatalf("unknown conditions or measures changed: got %+v err %v", got, err)
			}
		})
	}

	value := requestInspectionFixture()
	value.Conditions = &llm.EffectiveRequestConditions{}
	wire, err := RequestInspectionToProto(value)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RequestInspectionFromProto(wire)
	if err != nil || !reflect.DeepEqual(got.Conditions, value.Conditions) {
		t.Fatalf("present conditions with unknown members changed: %+v %v", got.Conditions, err)
	}
}

func TestRequestInspectionProtoAbsentCaptureIsUnavailableWithoutPayload(t *testing.T) {
	value, err := RequestInspectionFromProto(nil)
	if err != nil || !reflect.DeepEqual(value, llm.UnavailableRequestInspection("", "")) {
		t.Fatalf("absent capture invented historical payload: %+v %v", value, err)
	}
	wire, err := RequestInspectionToProto(value)
	if err != nil {
		t.Fatal(err)
	}
	if wire.Status != postpilotv1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || wire.Version != llm.RequestInspectionVersion || wire.PromptVersion != "" || wire.SchemaVersion != "" || wire.Fragments != nil || wire.SelectedRuleIds != nil || wire.Output != nil || wire.Conditions != nil || wire.Measures != nil || wire.IssuedAt != nil {
		t.Fatal("unavailable projection included private data")
	}
	if _, err := RequestInspectionFromProto(&postpilotv1.RequestInspection{}); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("present malformed capture silently became unavailable: %v", err)
	}
}

func TestRequestInspectionProtoMappingsOwnCopies(t *testing.T) {
	value := requestInspectionFixture()
	wire, err := RequestInspectionToProto(value)
	if err != nil {
		t.Fatal(err)
	}
	wire.SelectedRuleIds[0] = "changed"
	wire.Fragments[1].SourceRefs[0] = "changed"
	*wire.Conditions.MaxCompletionTokens = 1
	*wire.Conditions.StructuredOutput = true
	*wire.Measures.ProviderPromptTokens = 999
	if value.SelectedRuleIDs[0] != "stock-2" || value.Fragments[1].SourceRefs[0] != "memo-1" || *value.Conditions.MaxCompletionTokens != 8192 || *value.Conditions.StructuredOutput || *value.Measures.ProviderPromptTokens != 0 {
		t.Fatal("transport mutation changed private domain projection")
	}

	wire, err = RequestInspectionToProto(requestInspectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	got, err := RequestInspectionFromProto(wire)
	if err != nil {
		t.Fatal(err)
	}
	got.SelectedRuleIDs[0] = "changed"
	got.Fragments[1].SourceRefs[0] = "changed"
	*got.Conditions.MaxCompletionTokens = 1
	*got.Conditions.StructuredOutput = true
	*got.Measures.ProviderPromptTokens = 999
	if wire.SelectedRuleIds[0] != "stock-2" || wire.Fragments[1].SourceRefs[0] != "memo-1" || *wire.Conditions.MaxCompletionTokens != 8192 || *wire.Conditions.StructuredOutput || *wire.Measures.ProviderPromptTokens != 0 {
		t.Fatal("domain mutation changed transport projection")
	}
}

func TestRequestInspectionProtoProjectionDiscardsUnknownTransportPayload(t *testing.T) {
	wire, err := RequestInspectionToProto(requestInspectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	// An additive provider/network field in another transport revision must not
	// hitch a ride back through the customer projection's freshly built messages.
	unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "supplier-private-network-payload")
	wire.ProtoReflect().SetUnknown(unknown)
	wire.Fragments[0].ProtoReflect().SetUnknown(unknown)
	wire.Conditions.ProtoReflect().SetUnknown(unknown)
	wire.Conditions.Model.ProtoReflect().SetUnknown(unknown)
	wire.Measures.ProtoReflect().SetUnknown(unknown)
	wire.Output.ProtoReflect().SetUnknown(unknown)
	value, err := RequestInspectionFromProto(wire)
	if err != nil {
		t.Fatal(err)
	}
	safe, err := RequestInspectionToProto(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []proto.Message{safe, safe.Fragments[0], safe.Conditions, safe.Conditions.Model, safe.Measures, safe.Output} {
		if len(message.ProtoReflect().GetUnknown()) != 0 {
			t.Fatal("projection retained unknown private transport payload")
		}
	}
}

func TestRequestInspectionProtoRejectsMalformedPayloadInBothDirections(t *testing.T) {
	cases := []struct {
		name   string
		change func(*postpilotv1.RequestInspection)
	}{
		{"unknown status", func(r *postpilotv1.RequestInspection) { r.Status = 999 }},
		{"unknown role", func(r *postpilotv1.RequestInspection) { r.Fragments[0].Role = 999 }},
		{"unknown authorship", func(r *postpilotv1.RequestInspection) { r.Fragments[0].Authorship = 999 }},
		{"nil fragment", func(r *postpilotv1.RequestInspection) { r.Fragments[0] = nil }},
		{"unsupported version", func(r *postpilotv1.RequestInspection) { r.Version = 99 }},
		{"missing prompt version", func(r *postpilotv1.RequestInspection) { r.PromptVersion = "" }},
		{"missing output contract", func(r *postpilotv1.RequestInspection) { r.Output = nil }},
		{"unavailable with payload", func(r *postpilotv1.RequestInspection) {
			r.Status = postpilotv1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE
		}},
		{"prepared with usage", func(r *postpilotv1.RequestInspection) {
			r.Status = postpilotv1.InspectionStatus_INSPECTION_STATUS_PREPARED
		}},
		{"negative usage", func(r *postpilotv1.RequestInspection) {
			negative := int64(-1)
			r.Measures.ProviderPromptTokens = &negative
		}},
		{"negative timestamp nanos", func(r *postpilotv1.RequestInspection) { r.IssuedAt.Nanos = -1 }},
		{"timestamp beyond protobuf bound", func(r *postpilotv1.RequestInspection) { r.IssuedAt.Seconds = 253402300800 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			wire, err := RequestInspectionToProto(requestInspectionFixture())
			if err != nil {
				t.Fatal(err)
			}
			test.change(wire)
			if _, err := RequestInspectionFromProto(wire); !errors.Is(err, llm.ErrInvalidInspection) {
				t.Fatalf("malformed wire projection accepted: %v", err)
			}
		})
	}

	invalid := requestInspectionFixture()
	invalid.Fragments[0].Role = "operator"
	if _, err := RequestInspectionToProto(invalid); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("malformed domain role published: %v", err)
	}
	invalid = requestInspectionFixture()
	outside := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	invalid.IssuedAt = &outside
	if _, err := RequestInspectionToProto(invalid); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("timestamp outside protobuf published: %v", err)
	}
	// A valid UTC Unix epoch remains a real timestamp rather than being confused
	// with the domain's absent timestamp or Go's zero time.
	valid, err := RequestInspectionToProto(requestInspectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	valid.IssuedAt = &timestamppb.Timestamp{}
	if _, err := RequestInspectionFromProto(valid); err != nil {
		t.Fatalf("valid Unix epoch rejected: %v", err)
	}
}

func TestRequestInspectionProtoEnumCompleteness(t *testing.T) {
	for number, name := range postpilotv1.InspectionStatus_name {
		t.Run(name, func(t *testing.T) {
			value := postpilotv1.InspectionStatus(number)
			domain, err := inspectionStatusFromProto(value)
			if value == postpilotv1.InspectionStatus_INSPECTION_STATUS_UNSPECIFIED {
				if !errors.Is(err, llm.ErrInvalidInspection) {
					t.Fatal("unspecified status silently mapped")
				}
				return
			}
			if err != nil || !domain.Valid() {
				t.Fatalf("generated status has no explicit domain mapping: %v", err)
			}
			got, err := inspectionStatusToProto(domain)
			if err != nil || got != value {
				t.Fatalf("status roundtrip %v %v", got, err)
			}
		})
	}
	for number, name := range postpilotv1.InspectionRole_name {
		t.Run(name, func(t *testing.T) {
			value := postpilotv1.InspectionRole(number)
			domain, err := inspectionRoleFromProto(value)
			if value == postpilotv1.InspectionRole_INSPECTION_ROLE_UNSPECIFIED {
				if !errors.Is(err, llm.ErrInvalidInspection) {
					t.Fatal("unspecified role silently mapped")
				}
				return
			}
			if err != nil || !domain.Valid() {
				t.Fatalf("generated role has no explicit domain mapping: %v", err)
			}
			got, err := inspectionRoleToProto(domain)
			if err != nil || got != value {
				t.Fatalf("role roundtrip %v %v", got, err)
			}
		})
	}
	for number, name := range postpilotv1.FragmentAuthorship_name {
		t.Run(name, func(t *testing.T) {
			value := postpilotv1.FragmentAuthorship(number)
			domain, err := fragmentAuthorshipFromProto(value)
			if value == postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_UNSPECIFIED {
				if !errors.Is(err, llm.ErrInvalidInspection) {
					t.Fatal("unspecified authorship silently mapped")
				}
				return
			}
			if err != nil || !domain.Valid() {
				t.Fatalf("generated authorship has no explicit domain mapping: %v", err)
			}
			got, err := fragmentAuthorshipToProto(domain)
			if err != nil || got != value {
				t.Fatalf("authorship roundtrip %v %v", got, err)
			}
		})
	}
}
