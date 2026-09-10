package rpc

import (
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
	"testing"
)

func TestVideoDeliveryProjectionDoesNotRewriteRawCapability(t *testing.T) {
	for _, delivery := range []llm.VideoDelivery{{}, {InlineStaticVideo: true}, {SignedVideoURL: true}} {
		wire := toProtoModel(provider.CatalogModel{Info: llm.ModelInfo{VideoInput: true, VideoDelivery: delivery}})
		if !wire.VideoInput || wire.SignedVideoUrl != delivery.SignedVideoURL || wire.InlineStaticVideo != delivery.InlineStaticVideo {
			t.Fatalf("projection: %+v", wire)
		}
	}
}
