package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func languageFromProto(value v1.ContentLanguage) (string, error) {
	if language, ok := rpcserver.ContentLanguageFromProto(value); ok {
		return language, nil
	}
	return "", clip.ErrInvalid
}

func languageToProto(value string) v1.ContentLanguage {
	return rpcserver.ContentLanguageToProto(value)
}
