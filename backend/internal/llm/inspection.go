package llm

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// RequestInspectionVersion versions the product projection, independently of the
// composing context's prompt and output-schema versions.
const RequestInspectionVersion = 1

var ErrInvalidInspection = errors.New("request inspection is invalid")

// InspectionStatus distinguishes a configuration read, an unissued preview and a
// recorded issued request. Missing historical payload is unavailable; callers must
// never reconstruct it from current configuration and label it captured.
type InspectionStatus string

const (
	InspectionCurrent     InspectionStatus = "current"
	InspectionPrepared    InspectionStatus = "prepared"
	InspectionCaptured    InspectionStatus = "captured"
	InspectionUnavailable InspectionStatus = "unavailable"
)

func (s InspectionStatus) Valid() bool {
	switch s {
	case InspectionCurrent, InspectionPrepared, InspectionCaptured, InspectionUnavailable:
		return true
	default:
		return false
	}
}

// InspectionRole is the application's message role, not the fragment's author or
// semantic source. System is separate here because Request.System is not a Message.
type InspectionRole string

const (
	InspectionRoleSystem    InspectionRole = "system"
	InspectionRoleUser      InspectionRole = "user"
	InspectionRoleAssistant InspectionRole = "assistant"
)

func (r InspectionRole) Valid() bool {
	switch r {
	case InspectionRoleSystem, InspectionRoleUser, InspectionRoleAssistant:
		return true
	default:
		return false
	}
}

// FragmentAuthorship records who supplied the product content. A code-authored
// fragment can have the User role, and an account-authored one can have System role.
type FragmentAuthorship string

const (
	FragmentAuthorshipCode    FragmentAuthorship = "code"
	FragmentAuthorshipAccount FragmentAuthorship = "account"
)

func (a FragmentAuthorship) Valid() bool {
	switch a {
	case FragmentAuthorshipCode, FragmentAuthorshipAccount:
		return true
	default:
		return false
	}
}

// RequestFragment is one ordered product fragment. ID and MaterialRole are stable
// names owned by the composing context; SourceRefs are opaque frozen catalog keys.
// Text is preserved exactly. Media is represented only through SourceRefs, never a
// Request.Part, storage key, signed link or media byte payload.
type RequestFragment struct {
	ID           string
	Role         InspectionRole
	Authorship   FragmentAuthorship
	MaterialRole string
	Text         string
	SourceRefs   []string
	SourceFiles  []string
	Activation   string
}

// RequestNativeField describes native description/text/settings without
// inventing chat roles for speech or other non-chat operations.
type RequestNativeField struct {
	ID                             string
	Authorship                     FragmentAuthorship
	MaterialRole, Text, Activation string
	SourceRefs, SourceFiles        []string
}
type RequestOmission struct {
	ID, Reason, Activation string
	SourceFiles            []string
}

// RequestComposition is a small descriptor, not a prompt DSL. Safe Text and
// SourceRefs come only from the owning assembler; runtime transport parts,
// credentials, endpoints and supplier prices have no field here.
type RequestComposition struct {
	Stage                  string
	Mode                   string
	PromptVersion          string
	SchemaVersion          string
	Composer               string
	Parser                 string
	Consumer               string
	Activation             string
	SourceFiles            []string
	Fragments              []RequestFragment
	NativeFields           []RequestNativeField
	SelectedRuleIDs        []string
	Omissions              []RequestOmission
	Output                 OutputContractInspection
	ReferenceTokenEstimate *int64
}

// OutputContractInspection contains the product-owned schema, not provider/SDK
// request configuration. Schema can be empty for a plain-text output contract.
type OutputContractInspection struct {
	Name    string
	Version string
	Schema  string
}

// EffectiveRequestConditions deliberately does not embed ModelInfo, Request,
// ExecutionPolicy or Usage: those contain supplier prices, endpoints or media.
// Pointers preserve unknown versus a known zero/false/no-effort decision. Model IDs
// are the same public selection identities shown in the application's model picker.
type EffectiveRequestConditions struct {
	Model               *ModelRef
	MaxCompletionTokens *int64
	ReasoningEffort     *ReasoningEffort
	StructuredOutput    *bool
	DisableReasoning    *bool
	FreeCall            *bool
	DefaultBudget       *bool
	FrozenExecution     *bool
	ReasoningOmitted    *bool
}

// InspectionMeasures keeps differently derived measures separate. Characters are
// Unicode scalars, UTF8Bytes are bytes, ReferenceTokenEstimate is an application
// estimate, and Provider* fields are reported actual usage. Nil means unknown,
// including an absent provider report; a non-nil zero means a reported zero.
type InspectionMeasures struct {
	Characters               *int64
	UTF8Bytes                *int64
	ReferenceTokenEstimate   *int64
	ProviderPromptTokens     *int64
	ProviderCompletionTokens *int64
	ProviderReasoningTokens  *int64
}

// RequestInspection is a private product projection. The slice order is request
// composition order and must not be sorted by ID, role or author. Owning contexts
// apply their authenticated ownership, retention and blind-comparison fences before
// publishing it; this contract grants no access and performs no execution.
//
// Stage/Mode and PromptVersion/SchemaVersion are composing-context identifiers,
// independent of the registry's observe/write/analyze purpose names. No provider
// payload, credential, base URL, signed media link or supplier cost has a field here.
type RequestInspection struct {
	Version         int
	Status          InspectionStatus
	Stage           string
	Mode            string
	PromptVersion   string
	SchemaVersion   string
	Fragments       []RequestFragment
	SelectedRuleIDs []string
	Output          OutputContractInspection
	Conditions      *EffectiveRequestConditions
	Measures        InspectionMeasures
	IssuedAt        *time.Time
	Composer        string
	Parser          string
	Consumer        string
	Activation      string
	SourceFiles     []string
	NativeFields    []RequestNativeField
	Omissions       []RequestOmission
}

// UnavailableRequestInspection projects an absent, purged or uncaptured request.
// Stage and mode may identify the requested view, but no past prompt/version or
// runtime condition is inferred. Empty identifiers are valid when even that is lost.
func UnavailableRequestInspection(stage, mode string) RequestInspection {
	return RequestInspection{
		Version: RequestInspectionVersion, Status: InspectionUnavailable,
		Stage: stage, Mode: mode,
	}
}

// Validate checks projection structure, not prompt meaning, origin accuracy,
// ownership, model eligibility or whether a provider call actually occurred.
func (r RequestInspection) Validate() error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidInspection, reason) }
	if r.Version != RequestInspectionVersion || !r.Status.Valid() {
		return invalid("unsupported version or status")
	}
	if r.Status == InspectionUnavailable {
		if r.PromptVersion != "" || r.SchemaVersion != "" || len(r.Fragments) != 0 || len(r.SelectedRuleIDs) != 0 ||
			r.Output != (OutputContractInspection{}) || r.Conditions != nil || r.Measures != (InspectionMeasures{}) || r.IssuedAt != nil || r.Composer != "" || r.Parser != "" || r.Consumer != "" || r.Activation != "" || len(r.SourceFiles) != 0 || len(r.NativeFields) != 0 || len(r.Omissions) != 0 {
			return invalid("unavailable request carries private payload or runtime evidence")
		}
		if !utf8.ValidString(r.Stage) || !utf8.ValidString(r.Mode) {
			return invalid("view identifiers are not UTF-8")
		}
		return nil
	}
	for _, name := range []string{r.Stage, r.Mode, r.PromptVersion, r.SchemaVersion, r.Output.Name, r.Output.Version} {
		if !inspectionName(name) {
			return invalid("required composition or output identifier is absent")
		}
	}
	if !utf8.ValidString(r.Output.Schema) {
		return invalid("output schema is not UTF-8")
	}
	seenFragments := make(map[string]bool, len(r.Fragments))
	for _, fragment := range r.Fragments {
		if !inspectionName(fragment.ID) || !inspectionName(fragment.MaterialRole) || !fragment.Role.Valid() || !fragment.Authorship.Valid() || !utf8.ValidString(fragment.Text) {
			return invalid("fragment has an invalid identifier, role, author or text")
		}
		if seenFragments[fragment.ID] {
			return invalid("duplicate fragment identifier")
		}
		seenFragments[fragment.ID] = true
		if !inspectionNames(fragment.SourceRefs) {
			return invalid("fragment source references are invalid or duplicated")
		}
		if !inspectionNames(fragment.SourceFiles) || !utf8.ValidString(fragment.Activation) {
			return invalid("fragment source inventory is invalid")
		}
	}
	for _, field := range r.NativeFields {
		if !inspectionName(field.ID) || !inspectionName(field.MaterialRole) || !field.Authorship.Valid() || !utf8.ValidString(field.Text) || !utf8.ValidString(field.Activation) || !inspectionNames(field.SourceRefs) || !inspectionNames(field.SourceFiles) || seenFragments[field.ID] {
			return invalid("native field metadata is invalid")
		}
		seenFragments[field.ID] = true
	}
	for _, omission := range r.Omissions {
		if !inspectionName(omission.ID) || !inspectionName(omission.Reason) || !utf8.ValidString(omission.Activation) || !inspectionNames(omission.SourceFiles) {
			return invalid("omission inventory is invalid")
		}
	}
	if !inspectionNames(r.SourceFiles) || !utf8.ValidString(r.Composer) || !utf8.ValidString(r.Parser) || !utf8.ValidString(r.Consumer) || !utf8.ValidString(r.Activation) {
		return invalid("composition source inventory is invalid")
	}
	if !inspectionNames(r.SelectedRuleIDs) {
		return invalid("selected rule identifiers are invalid or duplicated")
	}
	if r.Conditions != nil {
		if model := r.Conditions.Model; model != nil && (!inspectionName(model.ProviderID) || !inspectionName(model.ModelID)) {
			return invalid("model identity is incomplete")
		}
		if budget := r.Conditions.MaxCompletionTokens; budget != nil && *budget <= 0 {
			return invalid("known completion budget must be positive")
		}
		if effort := r.Conditions.ReasoningEffort; effort != nil && !effort.Valid() {
			return invalid("reasoning effort is unsupported")
		}
	}
	for _, count := range []*int64{r.Measures.Characters, r.Measures.UTF8Bytes, r.Measures.ReferenceTokenEstimate,
		r.Measures.ProviderPromptTokens, r.Measures.ProviderCompletionTokens, r.Measures.ProviderReasoningTokens} {
		if count != nil && *count < 0 {
			return invalid("measure is negative")
		}
	}
	if r.Status != InspectionCaptured {
		if r.IssuedAt != nil || r.Measures.ProviderPromptTokens != nil || r.Measures.ProviderCompletionTokens != nil || r.Measures.ProviderReasoningTokens != nil {
			return invalid("an unissued view carries issued time or provider usage")
		}
	} else if r.IssuedAt != nil && r.IssuedAt.IsZero() {
		return invalid("issued time is zero")
	}
	return nil
}

// MeasureInspectionText counts exactly this text; it neither normalizes it nor
// estimates provider tokens. Composers choose the measured material explicitly.
func MeasureInspectionText(text string) (InspectionMeasures, error) {
	if !utf8.ValidString(text) {
		return InspectionMeasures{}, fmt.Errorf("%w: text is not UTF-8", ErrInvalidInspection)
	}
	characters, bytes := int64(utf8.RuneCountInString(text)), int64(len(text))
	return InspectionMeasures{Characters: &characters, UTF8Bytes: &bytes}, nil
}

func inspectionName(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != ""
}

func inspectionNames(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if !inspectionName(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
