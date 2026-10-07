package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/llm"
)

const writingTestSnapshotVersion = 1
const writingTestPromptVersion = "generation-full-writing-test-v2-origins"
const legacyWritingTestPromptVersion = "generation-full-writing-test-v1"
const writingTestRequestMaxBytes = 1 << 20

func EncodeWritingTestMaterialRequest(in WritingTestMaterialRequest) ([]byte, error) {
	return json.Marshal(in)
}
func EncodeWritingTestReference(in WritingTestReference) ([]byte, error) { return json.Marshal(in) }

func decodeWritingTestJSON(raw []byte, value any) error {
	if len(raw) == 0 || len(raw) > writingTestRequestMaxBytes {
		return ErrWritingTestMaterial
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("%w: %w", ErrWritingTestMaterial, err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return ErrWritingTestMaterial
	}
	return nil
}

// Generation's established snapshot mappers retain omission and nil/empty semantics.
// Plain domain structs at the public port remain independent of persistence tags.
type writingTestCommonWire struct {
	Version                 int                        `json:"version"`
	Factor                  string                     `json:"factor"`
	ModelStage              string                     `json:"model_stage"`
	Post                    snapshotPost               `json:"post"`
	Profile                 *snapshotProfile           `json:"profile,omitempty"`
	ObserveModel            string                     `json:"observe_model"`
	ObserveFiles            *[]string                  `json:"observe_files"`
	Observations            []snapshotObservation      `json:"observations"`
	Prepared                bool                       `json:"prepared"`
	Fictional               bool                       `json:"fictional"`
	BatchSize               int                        `json:"batch_size"`
	ObserveCompletionTokens int                        `json:"observe_completion_tokens"`
	ObservePromptTokens     int                        `json:"observe_prompt_tokens"`
	WritePromptTokens       int                        `json:"write_prompt_tokens"`
	ObserveStructuredOutput bool                       `json:"observe_structured_output"`
	Reasoning               ReasoningPolicy            `json:"reasoning"`
	PromptVersion           string                     `json:"prompt_version"`
	SchemaVersion           string                     `json:"schema_version"`
	SourceRevision          string                     `json:"source_revision"`
	AssignmentsHash         string                     `json:"assignments_hash"`
	VoiceRevision           string                     `json:"voice_revision"`
	TemplateRevision        string                     `json:"template_revision"`
	InputRevision           int64                      `json:"input_revision"`
	ContentRevision         int64                      `json:"content_revision"`
	Attachments             []WritingTestAttachment    `json:"attachments"`
	Rules                   writingTestRulesWire       `json:"rules"`
	RequiredFields          []WritingTestTemplateField `json:"required_fields"`
	QualityRuleIDs          []string                   `json:"quality_rule_ids"`
}
type writingTestRulesWire struct {
	Defaults []string              `json:"Defaults"`
	Owner    []WritingTestRule     `json:"Owner"`
	Stock    *[]stockGuidelineJSON `json:"stock,omitempty"`
}

func encodeWritingTestRules(r WritingTestRules) writingTestRulesWire {
	return writingTestRulesWire{Defaults: r.Defaults, Owner: r.Owner, Stock: encodeStockGuidelines(r.Stock)}
}
func decodeWritingTestRules(r writingTestRulesWire) WritingTestRules {
	return WritingTestRules{Defaults: r.Defaults, Owner: r.Owner, Stock: decodeStockGuidelines(r.Stock)}
}

type writingTestVariantWire struct {
	Reference               WritingTestReference `json:"reference"`
	Revision                string               `json:"revision"`
	SemanticKey             string               `json:"semantic_key"`
	Synthetic               bool                 `json:"synthetic"`
	Snapshot                json.RawMessage      `json:"snapshot"`
	ObserveModel            string               `json:"observe_model"`
	WriteModel              string               `json:"write_model"`
	ObservePromptTokens     int                  `json:"observe_prompt_tokens"`
	ObserveCompletionTokens int                  `json:"observe_completion_tokens"`
	WriteCompletionTokens   int                  `json:"write_completion_tokens"`
	WritePromptTokens       int                  `json:"write_prompt_tokens"`
	ObserveStructuredOutput bool                 `json:"observe_structured_output"`
	WriteStructuredOutput   bool                 `json:"write_structured_output"`
	Payload                 []byte               `json:"publication_payload"`
}

func encodeWritingTestCommon(in writingTestCommon) ([]byte, error) {
	return json.Marshal(writingTestCommonWire{
		Version: in.Version, Factor: in.Factor, ModelStage: in.ModelStage, Post: toSnapshotPost(in.Post), Profile: toSnapshotProfile(in.Profile),
		ObserveModel: testModelString(in.ObserveModel), ObserveFiles: copyOptionalTexts(in.ObserveFiles), Observations: mapSlice(in.Observations, toSnapshotObservation),
		Prepared: in.Prepared, Fictional: in.Fictional, BatchSize: in.BatchSize, ObserveCompletionTokens: in.ObserveCompletionTokens, ObservePromptTokens: in.ObservePromptTokens, WritePromptTokens: in.WritePromptTokens,
		ObserveStructuredOutput: in.ObserveStructuredOutput, Reasoning: in.Reasoning, PromptVersion: in.PromptVersion, SchemaVersion: in.SchemaVersion, SourceRevision: in.SourceRevision, AssignmentsHash: in.AssignmentsHash,
		Attachments: in.Attachments, Rules: encodeWritingTestRules(in.Rules), RequiredFields: in.RequiredFields, QualityRuleIDs: in.QualityRuleIDs,
		VoiceRevision: in.VoiceRevision, TemplateRevision: in.TemplateRevision, InputRevision: in.InputRevision, ContentRevision: in.ContentRevision,
	})
}
func encodeWritingTestVariant(in writingTestVariant) ([]byte, error) {
	raw, err := encodeWriteSnapshot(in.Snapshot)
	if err != nil {
		return nil, err
	}
	return json.Marshal(writingTestVariantWire{
		Reference: in.Reference, Revision: in.Revision, SemanticKey: in.SemanticKey, Synthetic: in.Synthetic, Snapshot: raw,
		ObserveModel: testModelString(in.ObserveModel), WriteModel: testModelString(in.WriteModel), ObservePromptTokens: in.ObservePromptTokens, ObserveCompletionTokens: in.ObserveCompletionTokens,
		WriteCompletionTokens: in.WriteCompletionTokens, WritePromptTokens: in.WritePromptTokens, ObserveStructuredOutput: in.ObserveStructuredOutput, WriteStructuredOutput: in.WriteStructuredOutput, Payload: in.Payload,
	})
}
func decodeWritingTestSnapshot(snapshot WritingTestSnapshot) (writingTestCommon, []writingTestVariant, error) {
	var wire writingTestCommonWire
	if err := decodeWritingTestJSON(snapshot.Common, &wire); err != nil {
		return writingTestCommon{}, nil, err
	}
	if wire.Version != writingTestSnapshotVersion || !validWritingTestFactor(wire.Factor, wire.ModelStage) || !validWritingTestCount(len(snapshot.Variants)) || wire.BatchSize <= 0 || wire.Post.UserID == "" || !Language(wire.Post.TargetLanguage).Valid() || wire.Post.TagCount <= 0 {
		return writingTestCommon{}, nil, ErrWritingTestMaterial
	}
	observe, err := testOptionalModel(wire.ObserveModel)
	if err != nil {
		return writingTestCommon{}, nil, err
	}
	common := writingTestCommon{
		Version: wire.Version, Factor: wire.Factor, ModelStage: wire.ModelStage, Post: fromSnapshotPost(wire.Post), Profile: fromSnapshotProfile(wire.Profile),
		ObserveModel: observe, ObserveFiles: copyOptionalTexts(wire.ObserveFiles), Observations: mapSlice(wire.Observations, fromSnapshotObservation), Prepared: wire.Prepared, Fictional: wire.Fictional,
		BatchSize: wire.BatchSize, ObserveCompletionTokens: wire.ObserveCompletionTokens, ObservePromptTokens: wire.ObservePromptTokens, WritePromptTokens: wire.WritePromptTokens,
		ObserveStructuredOutput: wire.ObserveStructuredOutput, Reasoning: wire.Reasoning, PromptVersion: wire.PromptVersion, SchemaVersion: wire.SchemaVersion, SourceRevision: wire.SourceRevision, AssignmentsHash: wire.AssignmentsHash,
		Attachments: wire.Attachments, Rules: decodeWritingTestRules(wire.Rules), RequiredFields: wire.RequiredFields, QualityRuleIDs: wire.QualityRuleIDs,
		VoiceRevision: wire.VoiceRevision, TemplateRevision: wire.TemplateRevision, InputRevision: wire.InputRevision, ContentRevision: wire.ContentRevision,
	}
	variants := make([]writingTestVariant, len(snapshot.Variants))
	for index, raw := range snapshot.Variants {
		var value writingTestVariantWire
		if err := decodeWritingTestJSON(raw, &value); err != nil {
			return writingTestCommon{}, nil, err
		}
		write, ok := parseModelRef(value.WriteModel)
		if !ok || value.WriteCompletionTokens <= 0 || value.WritePromptTokens <= 0 || value.Revision == "" || value.SemanticKey == "" {
			return writingTestCommon{}, nil, ErrWritingTestMaterial
		}
		observe, err := testOptionalModel(value.ObserveModel)
		if err != nil {
			return writingTestCommon{}, nil, err
		}
		input, err := decodeWriteSnapshot(value.Snapshot)
		if err != nil || input.Post.OriginProtocolVersion != common.Post.OriginProtocolVersion || input.TargetLanguage != common.Post.TargetLanguage || input.Post.UserID != common.Post.UserID || !input.SnapshotOnly {
			return writingTestCommon{}, nil, ErrWritingTestMaterial
		}
		variants[index] = writingTestVariant{
			Reference: value.Reference, Revision: value.Revision, SemanticKey: value.SemanticKey, Synthetic: value.Synthetic, Snapshot: input,
			ObserveModel: observe, WriteModel: write, ObservePromptTokens: value.ObservePromptTokens, ObserveCompletionTokens: value.ObserveCompletionTokens,
			WriteCompletionTokens: value.WriteCompletionTokens, WritePromptTokens: value.WritePromptTokens, ObserveStructuredOutput: value.ObserveStructuredOutput, WriteStructuredOutput: value.WriteStructuredOutput, Payload: value.Payload,
		}
	}
	if snapshot.Hash == "" || snapshot.PromptVersion != common.PromptVersion || snapshot.AssignmentsHash != common.AssignmentsHash || common.PromptVersion == "" || common.SchemaVersion == "" {
		return writingTestCommon{}, nil, ErrWritingTestMaterial
	}
	if snapshot.Hash != writingTestHash(snapshot.Common, snapshot.Variants) {
		return writingTestCommon{}, nil, ErrWritingTestMaterial
	}
	return common, variants, nil
}

func writingTestHash(common []byte, variants [][]byte) string {
	raw, _ := json.Marshal(struct {
		Common   json.RawMessage
		Variants [][]byte
	}{common, variants})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func writingTestSemanticKey(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func writingTestSchemaVersion() string {
	sum := sha256.Sum256(append(append(append(WriteAnswerSchema(), ObservationsSchema()...), VideoObservationsSchema()...), StorylineAnswerSchema()...))
	return hex.EncodeToString(sum[:])
}

func legacyWritingTestSchemaVersion() string {
	sum := sha256.Sum256(append(append(append(LegacyWriteAnswerSchema(), LegacyObservationsSchema()...), LegacyVideoObservationsSchema()...), LegacyStorylineAnswerSchema()...))
	return hex.EncodeToString(sum[:])
}

func testModelString(ref llm.ModelRef) string {
	if ref.ProviderID == "" && ref.ModelID == "" {
		return ""
	}
	return ref.String()
}
func testOptionalModel(value string) (llm.ModelRef, error) {
	if value == "" {
		return llm.ModelRef{}, nil
	}
	ref, ok := parseModelRef(value)
	if !ok {
		return llm.ModelRef{}, ErrWritingTestReference
	}
	return ref, nil
}
