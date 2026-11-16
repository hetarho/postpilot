package rpcserver

import postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"

// FailureToProto maps a stored failure to the wire message. An empty reason is no failure
// (nil); params are copied so the wire message is never nil-mapped.
func FailureToProto(reason string, params map[string]string, detail string) *postpilotv1.Failure {
	if reason == "" {
		return nil
	}
	copied := make(map[string]string, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return &postpilotv1.Failure{Reason: reason, Params: copied, TechnicalDetail: detail}
}
