package rpcserver

import postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"

// ContentLanguageFromProto translates the shared transport enum to the canonical language
// tag used by each pure domain context. False covers both UNSPECIFIED and future enum values;
// callers retain responsibility for choosing their context-specific domain error.
func ContentLanguageFromProto(value postpilotv1.ContentLanguage) (string, bool) {
	switch value {
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		return "ko", true
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		return "en", true
	default:
		return "", false
	}
}

// ContentLanguageToProto translates a canonical language tag to the shared transport enum.
// Invalid or absent domain values stay invalid on the wire instead of silently becoming a
// valid business language.
func ContentLanguageToProto(value string) postpilotv1.ContentLanguage {
	switch value {
	case "ko":
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case "en":
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}
