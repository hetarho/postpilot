package rpcserver

import (
	"strings"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// ModelRefToProto splits a stored "provider/model" reference at the first slash. It is
// nil unless both halves are non-empty.
func ModelRefToProto(value string) *postpilotv1.ModelRef {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return nil
	}
	return &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
}

// ModelRefFromProto joins a wire reference back to "provider/model"; a nil ref or an
// empty half is "".
func ModelRefFromProto(ref *postpilotv1.ModelRef) string {
	if ref == nil || ref.GetProviderId() == "" || ref.GetModelId() == "" {
		return ""
	}
	return ref.GetProviderId() + "/" + ref.GetModelId()
}
