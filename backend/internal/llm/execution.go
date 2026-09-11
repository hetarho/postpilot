package llm

import (
	"context"
	"io"
)

type VideoSampling string

const VideoSamplingFixed VideoSampling = "fixed"

// InlineVideo is supplied only by trusted media preparation. Open must return a
// fresh reader whose Close interrupts Read. Size is exact, not an upper estimate.
// It deliberately cannot enter a serialized job, RPC or cache.
type InlineVideo struct {
	MIME       string
	Size       int64
	DurationMS int64
	Sampling   VideoSampling
	Open       func(context.Context) (io.ReadCloser, error)
}

func (InlineVideo) MarshalJSON() ([]byte, error) { return nil, ErrUnsupported }

func InlineVideoPart(video InlineVideo) Part { return Part{InlineVideo: &video} }

func (p Part) Valid() bool {
	variants := 0
	if p.Text != "" || p.textSet {
		variants++
	}
	if p.Image != nil {
		if len(p.Image) == 0 {
			return false
		}
		variants++
	}
	if p.VideoURL != "" {
		variants++
	}
	if p.InlineVideo != nil {
		variants++
		v := p.InlineVideo
		if v.MIME != "video/mp4" || v.Size <= 0 || v.DurationMS <= 0 || v.DurationMS > 60_000 || v.Open == nil || v.Sampling != VideoSamplingFixed || p.MIME != "" {
			return false
		}
	}
	return variants == 1
}

func (r Request) ValidateParts() error {
	for _, m := range r.Messages {
		for _, p := range m.Parts {
			if !p.Valid() {
				return ErrUnsupported
			}
			if p.InlineVideo != nil && r.Execution == nil {
				return ErrUnsupported
			}
		}
	}
	return nil
}

// VideoDelivery is adapter-owned readiness, not a catalog capability. A newly
// advertised modality never silently enables a transport path.
type VideoDelivery struct {
	SignedVideoURL    bool
	InlineStaticVideo bool
}

type VideoDeliveryProvider interface {
	VideoDelivery(model string) VideoDelivery
}

type ExecutionDelivery string

const (
	ExecutionTextOnly     ExecutionDelivery = "text_only"
	ExecutionInlineStatic ExecutionDelivery = "inline_static_video"
)

type ExecutionPolicy struct {
	Call              CallPolicy
	Delivery          ExecutionDelivery
	NoFallback        bool
	RequireParameters bool
}

func (p ExecutionPolicy) Matches(ref ModelRef, r Request) bool {
	if !p.Call.Valid() || !p.Call.Pricing.Valid() || p.Call.Pricing.Delivery != p.Delivery || p.Call.Ref != ref || p.Call.Stage != r.Stage || p.Call.CompletionTokens != r.MaxTokens || p.Call.Reasoning != r.Reasoning || p.Call.DisableReasoning != r.DisableReasoning || !p.NoFallback || !p.RequireParameters {
		return false
	}
	if r.ValidateParts() != nil || r.HasImages() || (len(r.JSONSchema) > 0) != p.Call.StructuredOutput {
		return false
	}
	inline := 0
	for _, m := range r.Messages {
		for _, part := range m.Parts {
			if part.VideoURL != "" {
				return false
			}
			if part.InlineVideo != nil {
				inline++
			}
		}
	}
	return (p.Delivery == ExecutionTextOnly && inline == 0) || (p.Delivery == ExecutionInlineStatic && inline == 1)
}

// cachedEndpointsKey marks a context whose endpoint-document reads may be
// served from an adapter's unexpired cache.
type cachedEndpointsKey struct{}

// AllowCachedEndpoints marks a read-only qualification — the eligibility list a
// picker shows — as one an adapter may answer from an unexpired document it
// already read. A quote, the admission before a job's first call and the
// recheck before every completion never carry it: they read the live document,
// because a price that moved since the quote is exactly what they exist to
// refuse (QUOTA-47).
func AllowCachedEndpoints(ctx context.Context) context.Context {
	return context.WithValue(ctx, cachedEndpointsKey{}, true)
}

// CachedEndpointsAllowed reports whether AllowCachedEndpoints marked the context.
func CachedEndpointsAllowed(ctx context.Context) bool {
	allowed, _ := ctx.Value(cachedEndpointsKey{}).(bool)
	return allowed
}
