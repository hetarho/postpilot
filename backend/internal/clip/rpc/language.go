package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func languageFromProto(value v1.ContentLanguage) (string, error) {
	switch value {
	case v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		return "ko", nil
	case v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		return "en", nil
	default:
		return "", clip.ErrInvalid
	}
}

func languageToProto(value string) v1.ContentLanguage {
	switch value {
	case "ko":
		return v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case "en":
		return v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return v1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}
