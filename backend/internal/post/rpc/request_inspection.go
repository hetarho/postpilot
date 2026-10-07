package rpc

import (
	"fmt"
	"slices"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RequestInspectionToProto publishes only the validated product projection. It
// does not accept a runtime request or supplier usage/pricing payload.
func RequestInspectionToProto(value llm.RequestInspection) (*postpilotv1.RequestInspection, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	status, err := inspectionStatusToProto(value.Status)
	if err != nil {
		return nil, err
	}
	result := &postpilotv1.RequestInspection{
		Version: uint32(value.Version), Status: status,
		Stage: value.Stage, Mode: value.Mode, PromptVersion: value.PromptVersion, SchemaVersion: value.SchemaVersion,
		SelectedRuleIds: slices.Clone(value.SelectedRuleIDs),
		Composer:        value.Composer, Parser: value.Parser, Consumer: value.Consumer, Activation: value.Activation,
		SourceFiles: slices.Clone(value.SourceFiles), UnavailableReason: value.UnavailableReason, CallId: value.CallID,
	}
	if value.Fragments != nil {
		result.Fragments = make([]*postpilotv1.RequestFragment, len(value.Fragments))
	}
	for index, fragment := range value.Fragments {
		role, err := inspectionRoleToProto(fragment.Role)
		if err != nil {
			return nil, err
		}
		authorship, err := fragmentAuthorshipToProto(fragment.Authorship)
		if err != nil {
			return nil, err
		}
		result.Fragments[index] = &postpilotv1.RequestFragment{
			Id: fragment.ID, Role: role, Authorship: authorship, MaterialRole: fragment.MaterialRole,
			Text: fragment.Text, SourceRefs: slices.Clone(fragment.SourceRefs),
			SourceFiles: slices.Clone(fragment.SourceFiles), Activation: fragment.Activation,
		}
	}
	for _, field := range value.NativeFields {
		author, err := fragmentAuthorshipToProto(field.Authorship)
		if err != nil {
			return nil, err
		}
		result.NativeFields = append(result.NativeFields, &postpilotv1.RequestNativeField{Id: field.ID, Authorship: author, MaterialRole: field.MaterialRole, Text: field.Text, Activation: field.Activation, SourceRefs: slices.Clone(field.SourceRefs), SourceFiles: slices.Clone(field.SourceFiles)})
	}
	for _, omission := range value.Omissions {
		result.Omissions = append(result.Omissions, &postpilotv1.RequestOmission{Id: omission.ID, Reason: omission.Reason, Activation: omission.Activation, SourceFiles: slices.Clone(omission.SourceFiles)})
	}
	for _, attachment := range value.Attachments {
		result.Attachments = append(result.Attachments, &postpilotv1.InspectionAttachment{Id: attachment.ID, Kind: attachment.Kind})
	}
	if value.Output != (llm.OutputContractInspection{}) {
		result.Output = &postpilotv1.OutputContractInspection{
			Name: value.Output.Name, Version: value.Output.Version, Schema: value.Output.Schema,
		}
	}
	if value.Conditions != nil {
		result.Conditions = &postpilotv1.EffectiveRequestConditions{
			MaxCompletionTokens: inspectionCopyOptional(value.Conditions.MaxCompletionTokens),
			StructuredOutput:    inspectionCopyOptional(value.Conditions.StructuredOutput),
			DisableReasoning:    inspectionCopyOptional(value.Conditions.DisableReasoning), FreeCall: inspectionCopyOptional(value.Conditions.FreeCall), DefaultBudget: inspectionCopyOptional(value.Conditions.DefaultBudget), FrozenExecution: inspectionCopyOptional(value.Conditions.FrozenExecution), ReasoningOmitted: inspectionCopyOptional(value.Conditions.ReasoningOmitted),
		}
		if value.Conditions.Model != nil {
			result.Conditions.Model = &postpilotv1.ModelRef{ProviderId: value.Conditions.Model.ProviderID, ModelId: value.Conditions.Model.ModelID}
		}
		if value.Conditions.ReasoningEffort != nil {
			effort := string(*value.Conditions.ReasoningEffort)
			result.Conditions.ReasoningEffort = &effort
		}
	}
	if value.Measures != (llm.InspectionMeasures{}) {
		result.Measures = &postpilotv1.InspectionMeasures{
			Characters:               inspectionCopyOptional(value.Measures.Characters),
			Utf8Bytes:                inspectionCopyOptional(value.Measures.UTF8Bytes),
			ReferenceTokenEstimate:   inspectionCopyOptional(value.Measures.ReferenceTokenEstimate),
			ProviderPromptTokens:     inspectionCopyOptional(value.Measures.ProviderPromptTokens),
			ProviderCompletionTokens: inspectionCopyOptional(value.Measures.ProviderCompletionTokens),
			ProviderReasoningTokens:  inspectionCopyOptional(value.Measures.ProviderReasoningTokens),
		}
	}
	if value.IssuedAt != nil {
		result.IssuedAt = timestamppb.New(*value.IssuedAt)
		if err := result.IssuedAt.CheckValid(); err != nil {
			return nil, invalidInspectionMapping("issued timestamp is invalid")
		}
	}
	return result, nil
}

// RequestInspectionFromProto maps an absent historical annotation to unavailable.
// A present malformed annotation is rejected rather than silently relabelled.
func RequestInspectionFromProto(value *postpilotv1.RequestInspection) (llm.RequestInspection, error) {
	if value == nil {
		return llm.UnavailableRequestInspection("", ""), nil
	}
	status, err := inspectionStatusFromProto(value.Status)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	result := llm.RequestInspection{
		Version: int(value.Version), Status: status,
		Stage: value.Stage, Mode: value.Mode, PromptVersion: value.PromptVersion, SchemaVersion: value.SchemaVersion,
		SelectedRuleIDs: slices.Clone(value.SelectedRuleIds),
		Composer:        value.Composer, Parser: value.Parser, Consumer: value.Consumer, Activation: value.Activation,
		SourceFiles: slices.Clone(value.SourceFiles), UnavailableReason: value.UnavailableReason, CallID: value.CallId,
	}
	if value.Fragments != nil {
		result.Fragments = make([]llm.RequestFragment, len(value.Fragments))
	}
	for index, fragment := range value.Fragments {
		if fragment == nil {
			return llm.RequestInspection{}, invalidInspectionMapping("fragment is absent")
		}
		role, err := inspectionRoleFromProto(fragment.Role)
		if err != nil {
			return llm.RequestInspection{}, err
		}
		authorship, err := fragmentAuthorshipFromProto(fragment.Authorship)
		if err != nil {
			return llm.RequestInspection{}, err
		}
		result.Fragments[index] = llm.RequestFragment{
			ID: fragment.Id, Role: role, Authorship: authorship, MaterialRole: fragment.MaterialRole,
			Text: fragment.Text, SourceRefs: slices.Clone(fragment.SourceRefs),
			SourceFiles: slices.Clone(fragment.SourceFiles), Activation: fragment.Activation,
		}
	}
	for _, field := range value.NativeFields {
		if field == nil {
			return llm.RequestInspection{}, invalidInspectionMapping("native field is absent")
		}
		author, err := fragmentAuthorshipFromProto(field.Authorship)
		if err != nil {
			return llm.RequestInspection{}, err
		}
		result.NativeFields = append(result.NativeFields, llm.RequestNativeField{ID: field.Id, Authorship: author, MaterialRole: field.MaterialRole, Text: field.Text, Activation: field.Activation, SourceRefs: slices.Clone(field.SourceRefs), SourceFiles: slices.Clone(field.SourceFiles)})
	}
	for _, omission := range value.Omissions {
		if omission == nil {
			return llm.RequestInspection{}, invalidInspectionMapping("omission is absent")
		}
		result.Omissions = append(result.Omissions, llm.RequestOmission{ID: omission.Id, Reason: omission.Reason, Activation: omission.Activation, SourceFiles: slices.Clone(omission.SourceFiles)})
	}
	for _, attachment := range value.Attachments {
		if attachment == nil {
			return llm.RequestInspection{}, invalidInspectionMapping("attachment is absent")
		}
		result.Attachments = append(result.Attachments, llm.InspectionAttachment{ID: attachment.Id, Kind: attachment.Kind})
	}
	if value.Output != nil {
		result.Output = llm.OutputContractInspection{Name: value.Output.Name, Version: value.Output.Version, Schema: value.Output.Schema}
	}
	if value.Conditions != nil {
		result.Conditions = &llm.EffectiveRequestConditions{
			MaxCompletionTokens: inspectionCopyOptional(value.Conditions.MaxCompletionTokens),
			StructuredOutput:    inspectionCopyOptional(value.Conditions.StructuredOutput),
			DisableReasoning:    inspectionCopyOptional(value.Conditions.DisableReasoning), FreeCall: inspectionCopyOptional(value.Conditions.FreeCall), DefaultBudget: inspectionCopyOptional(value.Conditions.DefaultBudget), FrozenExecution: inspectionCopyOptional(value.Conditions.FrozenExecution), ReasoningOmitted: inspectionCopyOptional(value.Conditions.ReasoningOmitted),
		}
		if value.Conditions.Model != nil {
			result.Conditions.Model = &llm.ModelRef{ProviderID: value.Conditions.Model.ProviderId, ModelID: value.Conditions.Model.ModelId}
		}
		if value.Conditions.ReasoningEffort != nil {
			effort := llm.ReasoningEffort(*value.Conditions.ReasoningEffort)
			result.Conditions.ReasoningEffort = &effort
		}
	}
	if value.Measures != nil {
		result.Measures = llm.InspectionMeasures{
			Characters:               inspectionCopyOptional(value.Measures.Characters),
			UTF8Bytes:                inspectionCopyOptional(value.Measures.Utf8Bytes),
			ReferenceTokenEstimate:   inspectionCopyOptional(value.Measures.ReferenceTokenEstimate),
			ProviderPromptTokens:     inspectionCopyOptional(value.Measures.ProviderPromptTokens),
			ProviderCompletionTokens: inspectionCopyOptional(value.Measures.ProviderCompletionTokens),
			ProviderReasoningTokens:  inspectionCopyOptional(value.Measures.ProviderReasoningTokens),
		}
	}
	if value.IssuedAt != nil {
		if err := value.IssuedAt.CheckValid(); err != nil {
			return llm.RequestInspection{}, invalidInspectionMapping("issued timestamp is invalid")
		}
		issued := value.IssuedAt.AsTime()
		result.IssuedAt = &issued
	}
	if err := result.Validate(); err != nil {
		return llm.RequestInspection{}, err
	}
	return result, nil
}

func inspectionStatusToProto(value llm.InspectionStatus) (postpilotv1.InspectionStatus, error) {
	switch value {
	case llm.InspectionCurrent:
		return postpilotv1.InspectionStatus_INSPECTION_STATUS_CURRENT, nil
	case llm.InspectionPrepared:
		return postpilotv1.InspectionStatus_INSPECTION_STATUS_PREPARED, nil
	case llm.InspectionCaptured:
		return postpilotv1.InspectionStatus_INSPECTION_STATUS_CAPTURED, nil
	case llm.InspectionUnavailable:
		return postpilotv1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE, nil
	default:
		return 0, invalidInspectionMapping("inspection status is unsupported")
	}
}

func inspectionStatusFromProto(value postpilotv1.InspectionStatus) (llm.InspectionStatus, error) {
	switch value {
	case postpilotv1.InspectionStatus_INSPECTION_STATUS_CURRENT:
		return llm.InspectionCurrent, nil
	case postpilotv1.InspectionStatus_INSPECTION_STATUS_PREPARED:
		return llm.InspectionPrepared, nil
	case postpilotv1.InspectionStatus_INSPECTION_STATUS_CAPTURED:
		return llm.InspectionCaptured, nil
	case postpilotv1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE:
		return llm.InspectionUnavailable, nil
	case postpilotv1.InspectionStatus_INSPECTION_STATUS_UNSPECIFIED:
		return "", invalidInspectionMapping("inspection status is absent")
	default:
		return "", invalidInspectionMapping("inspection status is unsupported")
	}
}

func inspectionRoleToProto(value llm.InspectionRole) (postpilotv1.InspectionRole, error) {
	switch value {
	case llm.InspectionRoleSystem:
		return postpilotv1.InspectionRole_INSPECTION_ROLE_SYSTEM, nil
	case llm.InspectionRoleUser:
		return postpilotv1.InspectionRole_INSPECTION_ROLE_USER, nil
	case llm.InspectionRoleAssistant:
		return postpilotv1.InspectionRole_INSPECTION_ROLE_ASSISTANT, nil
	default:
		return 0, invalidInspectionMapping("inspection role is unsupported")
	}
}

func inspectionRoleFromProto(value postpilotv1.InspectionRole) (llm.InspectionRole, error) {
	switch value {
	case postpilotv1.InspectionRole_INSPECTION_ROLE_SYSTEM:
		return llm.InspectionRoleSystem, nil
	case postpilotv1.InspectionRole_INSPECTION_ROLE_USER:
		return llm.InspectionRoleUser, nil
	case postpilotv1.InspectionRole_INSPECTION_ROLE_ASSISTANT:
		return llm.InspectionRoleAssistant, nil
	case postpilotv1.InspectionRole_INSPECTION_ROLE_UNSPECIFIED:
		return "", invalidInspectionMapping("inspection role is absent")
	default:
		return "", invalidInspectionMapping("inspection role is unsupported")
	}
}

func fragmentAuthorshipToProto(value llm.FragmentAuthorship) (postpilotv1.FragmentAuthorship, error) {
	switch value {
	case llm.FragmentAuthorshipCode:
		return postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_CODE, nil
	case llm.FragmentAuthorshipAccount:
		return postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_ACCOUNT, nil
	default:
		return 0, invalidInspectionMapping("fragment authorship is unsupported")
	}
}

func fragmentAuthorshipFromProto(value postpilotv1.FragmentAuthorship) (llm.FragmentAuthorship, error) {
	switch value {
	case postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_CODE:
		return llm.FragmentAuthorshipCode, nil
	case postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_ACCOUNT:
		return llm.FragmentAuthorshipAccount, nil
	case postpilotv1.FragmentAuthorship_FRAGMENT_AUTHORSHIP_UNSPECIFIED:
		return "", invalidInspectionMapping("fragment authorship is absent")
	default:
		return "", invalidInspectionMapping("fragment authorship is unsupported")
	}
}

func invalidInspectionMapping(reason string) error {
	return fmt.Errorf("%w: %s", llm.ErrInvalidInspection, reason)
}

func inspectionCopyOptional[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
