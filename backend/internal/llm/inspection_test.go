package llm_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

func inspectionFixture() llm.RequestInspection {
	budget := int64(8192)
	effort := llm.ReasoningLow
	structured := true
	return llm.RequestInspection{
		Version: llm.RequestInspectionVersion, Status: llm.InspectionPrepared,
		Stage: "post.write", Mode: "new", PromptVersion: "write-v1", SchemaVersion: "post-v1",
		Fragments: []llm.RequestFragment{
			{ID: "core", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "instruction", Text: "Write from the supplied material."},
			{ID: "memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "evidence", Text: "  카페😀\nCafe\u0301\t", SourceRefs: []string{"memo-1"}},
			{ID: "image", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "observation_input", SourceRefs: []string{"photo-1"}},
		},
		SelectedRuleIDs: []string{"owner-rule-1"},
		Output:          llm.OutputContractInspection{Name: "post", Version: "post-v1", Schema: `{"type":"object","properties":{"title":{"type":"string"}}}`},
		Conditions: &llm.EffectiveRequestConditions{
			Model:               &llm.ModelRef{ProviderID: "catalog", ModelID: "writer"},
			MaxCompletionTokens: &budget, ReasoningEffort: &effort, StructuredOutput: &structured,
		},
	}
}

func TestRequestInspectionPreservesOrderedExactFragmentsAndUnknownMeasures(t *testing.T) {
	inspection := inspectionFixture()
	before := inspection.Fragments[1].Text
	if err := inspection.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := []string{inspection.Fragments[0].ID, inspection.Fragments[1].ID, inspection.Fragments[2].ID}; !reflect.DeepEqual(got, []string{"core", "memo", "image"}) {
		t.Fatalf("composition order changed: %v", got)
	}
	if inspection.Fragments[1].Text != before || inspection.Fragments[2].Text != "" || !reflect.DeepEqual(inspection.Fragments[2].SourceRefs, []string{"photo-1"}) {
		t.Fatal("exact text or media reference changed")
	}
	if inspection.Measures != (llm.InspectionMeasures{}) {
		t.Fatal("an unmeasured preview must keep measurements unknown")
	}

	// Message role does not infer authorship or the role of the supplied material.
	inspection.Fragments[1].Role = llm.InspectionRoleSystem
	inspection.Fragments[0].Role = llm.InspectionRoleUser
	if err := inspection.Validate(); err != nil {
		t.Fatalf("independent authorship and wire role rejected: %v", err)
	}
}

func TestRequestInspectionDistinguishesViewsAndNeverReconstructsMissingCapture(t *testing.T) {
	for _, status := range []llm.InspectionStatus{llm.InspectionCurrent, llm.InspectionPrepared, llm.InspectionCaptured} {
		t.Run(string(status), func(t *testing.T) {
			inspection := inspectionFixture()
			inspection.Status = status
			if status == llm.InspectionCaptured {
				now := time.Date(2026, time.October, 7, 0, 0, 0, 0, time.UTC)
				reportedZero := int64(0)
				inspection.IssuedAt = &now
				inspection.Measures.ProviderPromptTokens = &reportedZero
				inspection.Measures.ProviderCompletionTokens = &reportedZero
			}
			if err := inspection.Validate(); err != nil {
				t.Fatal(err)
			}
			if status == llm.InspectionCaptured && (inspection.Measures.ProviderPromptTokens == nil || *inspection.Measures.ProviderPromptTokens != 0 || inspection.Measures.ProviderReasoningTokens != nil) {
				t.Fatal("reported zero was confused with missing actual usage")
			}
		})
	}
	for _, view := range []llm.RequestInspection{llm.UnavailableRequestInspection("post.write", "new"), llm.UnavailableRequestInspection("", "")} {
		if err := view.Validate(); err != nil {
			t.Fatal(err)
		}
		if view.Status != llm.InspectionUnavailable || view.PromptVersion != "" || view.SchemaVersion != "" || view.Conditions != nil || len(view.Fragments) != 0 {
			t.Fatal("missing capture was reconstructed")
		}
	}
}

func TestUnavailableInspectionExplainsAbsenceWithoutReconstructingPayload(t *testing.T) {
	view := llm.UnavailableRequestInspection("post-writing", "direct")
	view.UnavailableReason = "Historical private request capture is absent or purged."
	if err := view.Validate(); err != nil {
		t.Fatal(err)
	}
	view.UnavailableReason = string([]byte{0xff})
	if err := view.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("invalid unavailable explanation accepted: %v", err)
	}
	available := inspectionFixture()
	available.UnavailableReason = "absent"
	if err := available.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("available request claimed missing evidence: %v", err)
	}
}

func TestInspectionMediaAndDurableCallIdentityAreSafeAndStatusBound(t *testing.T) {
	view := inspectionFixture()
	view.Attachments = []llm.InspectionAttachment{{ID: "owned-opaque-photo", Kind: "photo"}}
	if err := view.Validate(); err != nil {
		t.Fatal(err)
	}
	view.CallID = "job-observe-1"
	if err := view.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("preview claimed a durable issued call: %v", err)
	}
	view.Status = llm.InspectionCaptured
	if err := view.Validate(); err != nil {
		t.Fatal(err)
	}
	view.Attachments = append(view.Attachments, view.Attachments[0])
	if err := view.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("ambiguous duplicate attachment admitted: %v", err)
	}
	view.Attachments = []llm.InspectionAttachment{{ID: "owned-opaque-photo", Kind: "storage-key"}}
	if err := view.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("unknown media kind admitted: %v", err)
	}
}

func TestRequestInspectionRejectsFalseExecutionEvidenceAndMalformedStructure(t *testing.T) {
	negative := int64(-1)
	zero := int64(0)
	now := time.Now()
	badEffort := llm.ReasoningEffort("automatic")
	cases := []struct {
		name   string
		change func(*llm.RequestInspection)
	}{
		{"unsupported version", func(r *llm.RequestInspection) { r.Version++ }},
		{"absent status", func(r *llm.RequestInspection) { r.Status = "" }},
		{"unsupported status", func(r *llm.RequestInspection) { r.Status = "historical" }},
		{"absent stage", func(r *llm.RequestInspection) { r.Stage = "" }},
		{"absent mode", func(r *llm.RequestInspection) { r.Mode = " " }},
		{"absent prompt version", func(r *llm.RequestInspection) { r.PromptVersion = "" }},
		{"absent schema version", func(r *llm.RequestInspection) { r.SchemaVersion = "" }},
		{"absent output name", func(r *llm.RequestInspection) { r.Output.Name = "" }},
		{"absent output version", func(r *llm.RequestInspection) { r.Output.Version = "" }},
		{"invalid output UTF-8", func(r *llm.RequestInspection) { r.Output.Schema = string([]byte{0xff}) }},
		{"duplicate fragment", func(r *llm.RequestInspection) { r.Fragments[1].ID = "core" }},
		{"unsupported role", func(r *llm.RequestInspection) { r.Fragments[0].Role = "developer" }},
		{"unsupported author", func(r *llm.RequestInspection) { r.Fragments[0].Authorship = "provider" }},
		{"absent material role", func(r *llm.RequestInspection) { r.Fragments[0].MaterialRole = "" }},
		{"invalid fragment UTF-8", func(r *llm.RequestInspection) { r.Fragments[1].Text = string([]byte{0xff}) }},
		{"absent source reference", func(r *llm.RequestInspection) { r.Fragments[1].SourceRefs = []string{""} }},
		{"duplicate source reference", func(r *llm.RequestInspection) { r.Fragments[1].SourceRefs = []string{"memo-1", "memo-1"} }},
		{"duplicate selected rule", func(r *llm.RequestInspection) { r.SelectedRuleIDs = []string{"rule", "rule"} }},
		{"incomplete model", func(r *llm.RequestInspection) { r.Conditions.Model.ModelID = "" }},
		{"zero completion budget", func(r *llm.RequestInspection) { r.Conditions.MaxCompletionTokens = &zero }},
		{"unsupported effort", func(r *llm.RequestInspection) { r.Conditions.ReasoningEffort = &badEffort }},
		{"negative characters", func(r *llm.RequestInspection) { r.Measures.Characters = &negative }},
		{"negative bytes", func(r *llm.RequestInspection) { r.Measures.UTF8Bytes = &negative }},
		{"negative estimate", func(r *llm.RequestInspection) { r.Measures.ReferenceTokenEstimate = &negative }},
		{"negative actual prompt", func(r *llm.RequestInspection) {
			r.Status = llm.InspectionCaptured
			r.Measures.ProviderPromptTokens = &negative
		}},
		{"negative actual completion", func(r *llm.RequestInspection) {
			r.Status = llm.InspectionCaptured
			r.Measures.ProviderCompletionTokens = &negative
		}},
		{"negative actual reasoning", func(r *llm.RequestInspection) {
			r.Status = llm.InspectionCaptured
			r.Measures.ProviderReasoningTokens = &negative
		}},
		{"preview issued time", func(r *llm.RequestInspection) { r.IssuedAt = &now }},
		{"preview actual prompt", func(r *llm.RequestInspection) { r.Measures.ProviderPromptTokens = &zero }},
		{"preview actual completion", func(r *llm.RequestInspection) { r.Measures.ProviderCompletionTokens = &zero }},
		{"preview actual reasoning", func(r *llm.RequestInspection) { r.Measures.ProviderReasoningTokens = &zero }},
		{"current actual usage", func(r *llm.RequestInspection) {
			r.Status = llm.InspectionCurrent
			r.Measures.ProviderPromptTokens = &zero
		}},
		{"captured zero time", func(r *llm.RequestInspection) { r.Status = llm.InspectionCaptured; r.IssuedAt = &time.Time{} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			inspection := inspectionFixture()
			test.change(&inspection)
			if err := inspection.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
				t.Fatalf("malformed projection accepted: %v", err)
			} else if strings.Contains(err.Error(), "카페") {
				t.Fatal("validation error leaked private prompt material")
			}
		})
	}
}

func TestUnavailableInspectionRejectsPrivatePayloadAndRuntimeEvidence(t *testing.T) {
	zero := int64(0)
	now := time.Now()
	cases := []struct {
		name   string
		change func(*llm.RequestInspection)
	}{
		{"prompt version", func(r *llm.RequestInspection) { r.PromptVersion = "current-version" }},
		{"schema version", func(r *llm.RequestInspection) { r.SchemaVersion = "current-version" }},
		{"fragment", func(r *llm.RequestInspection) { r.Fragments = inspectionFixture().Fragments }},
		{"selected rule", func(r *llm.RequestInspection) { r.SelectedRuleIDs = []string{"owner-rule"} }},
		{"output contract", func(r *llm.RequestInspection) { r.Output = inspectionFixture().Output }},
		{"model conditions", func(r *llm.RequestInspection) { r.Conditions = inspectionFixture().Conditions }},
		{"character measure", func(r *llm.RequestInspection) { r.Measures.Characters = &zero }},
		{"byte measure", func(r *llm.RequestInspection) { r.Measures.UTF8Bytes = &zero }},
		{"reference estimate", func(r *llm.RequestInspection) { r.Measures.ReferenceTokenEstimate = &zero }},
		{"provider prompt", func(r *llm.RequestInspection) { r.Measures.ProviderPromptTokens = &zero }},
		{"provider completion", func(r *llm.RequestInspection) { r.Measures.ProviderCompletionTokens = &zero }},
		{"provider reasoning", func(r *llm.RequestInspection) { r.Measures.ProviderReasoningTokens = &zero }},
		{"issued time", func(r *llm.RequestInspection) { r.IssuedAt = &now }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			inspection := llm.UnavailableRequestInspection("post.write", "new")
			test.change(&inspection)
			if err := inspection.Validate(); !errors.Is(err, llm.ErrInvalidInspection) {
				t.Fatalf("unavailable view carried inferred payload: %v", err)
			}
		})
	}
}

func TestInspectionTextMeasuresSeparateScalarsBytesAndRuntimeUsage(t *testing.T) {
	for _, test := range []struct {
		text       string
		characters int64
		bytes      int64
	}{
		{"", 0, 0},
		{"ABC", 3, 3},
		{"카페😀", 3, 10},
		{"Cafe\u0301", 5, 6},
		{"👩‍💻", 3, 11},
		{"\r\n\t ", 4, 4},
	} {
		t.Run(test.text, func(t *testing.T) {
			measures, err := llm.MeasureInspectionText(test.text)
			if err != nil {
				t.Fatal(err)
			}
			if measures.Characters == nil || *measures.Characters != test.characters || measures.UTF8Bytes == nil || *measures.UTF8Bytes != test.bytes {
				t.Fatalf("scalar/byte measure differs: %+v", measures)
			}
			if measures.ReferenceTokenEstimate != nil || measures.ProviderPromptTokens != nil || measures.ProviderCompletionTokens != nil || measures.ProviderReasoningTokens != nil {
				t.Fatal("text measures invented token estimates or actual usage")
			}
		})
	}
	if _, err := llm.MeasureInspectionText(string([]byte{0xff})); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("invalid UTF-8 silently counted: %v", err)
	}
}

func TestInspectionShapeCannotEmbedProviderOrMediaPayloads(t *testing.T) {
	unsafeTypes := map[reflect.Type]bool{
		reflect.TypeFor[llm.Request]():         true,
		reflect.TypeFor[llm.Response]():        true,
		reflect.TypeFor[llm.Usage]():           true,
		reflect.TypeFor[llm.Part]():            true,
		reflect.TypeFor[llm.ModelInfo]():       true,
		reflect.TypeFor[llm.SourceModel]():     true,
		reflect.TypeFor[llm.AdapterConfig]():   true,
		reflect.TypeFor[llm.ExecutionPolicy](): true,
	}
	seen := map[reflect.Type]bool{}
	var inspect func(reflect.Type)
	inspect = func(shape reflect.Type) {
		if seen[shape] {
			return
		}
		seen[shape] = true
		if unsafeTypes[shape] {
			t.Fatalf("customer inspection includes unsafe provider type %s", shape)
		}
		switch shape.Kind() {
		case reflect.Pointer:
			inspect(shape.Elem())
		case reflect.Slice:
			if shape.Elem().Kind() == reflect.Uint8 {
				t.Fatalf("customer inspection includes raw bytes %s", shape)
			}
			inspect(shape.Elem())
		case reflect.Map, reflect.Interface:
			t.Fatalf("customer inspection permits unbounded provider payload %s", shape)
		case reflect.Struct:
			if shape == reflect.TypeFor[time.Time]() {
				return
			}
			for index := range shape.NumField() {
				field := shape.Field(index)
				name := strings.ToLower(field.Name)
				for _, forbidden := range []string{"credential", "apikey", "cost", "baseurl", "signed", "endpoint", "mediaurl", "storagekey"} {
					if strings.Contains(name, forbidden) {
						t.Fatalf("customer inspection includes unsafe field %s.%s", shape, field.Name)
					}
				}
				inspect(field.Type)
			}
		}
	}
	inspect(reflect.TypeFor[llm.RequestInspection]())
}
