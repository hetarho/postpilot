// Package llm is the port every model call goes through (PRD §6.4). It is a boundary,
// not a preference: nothing above it learns which vendor answered, and no adapter or
// SDK type appears above it (ARCHITECTURE §2.1). The model is an input to every call —
// observation and writing choose theirs per stage ([I3]) — so the port reads no default.
package llm

import (
	"context"
	"time"
)

// ReasoningEffort is the provider-neutral reasoning strength accepted by the port.
// Unset is a registry override that deliberately leaves the wire request untouched;
// None is different because it asks the provider to disable reasoning explicitly.
type ReasoningEffort string

const (
	ReasoningUnspecified ReasoningEffort = ""
	ReasoningUnset       ReasoningEffort = "unset"
	ReasoningNone        ReasoningEffort = "none"
	ReasoningMinimal     ReasoningEffort = "minimal"
	ReasoningLow         ReasoningEffort = "low"
	ReasoningMedium      ReasoningEffort = "medium"
	ReasoningHigh        ReasoningEffort = "high"
	ReasoningXHigh       ReasoningEffort = "xhigh"
	ReasoningMax         ReasoningEffort = "max"
)

// Valid reports whether the value is a supported policy or wire value.
func (e ReasoningEffort) Valid() bool {
	switch e {
	case ReasoningUnspecified, ReasoningUnset, ReasoningNone, ReasoningMinimal, ReasoningLow,
		ReasoningMedium, ReasoningHigh, ReasoningXHigh, ReasoningMax:
		return true
	default:
		return false
	}
}

// Role is who said a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Part is one piece of a message: text, one image, or one video. Exactly one is set.
type Part struct {
	Text string
	// Image is the encoded bytes of one image with its MIME type — the JPEG the browser
	// pipeline produced, read back from object storage by the caller.
	Image []byte
	// VideoURL is a short-lived signed URL the PROVIDER fetches, never bytes this process
	// carries: a clip is up to 200 MiB and the largest thing an RPC here ever holds stays a
	// memo (VIDEO-10). The URL outlives the call by design — it lives the presign TTL — so a
	// stale one fails the call rather than leaking a standing link.
	VideoURL string
	MIME     string
	// InlineVideo is a runtime-only, bounded analysis copy, never a post attachment.
	InlineVideo *InlineVideo
	textSet     bool
}

// TextPart returns a text part.
func TextPart(text string) Part { return Part{Text: text, textSet: true} }

// ImagePart returns an image part.
func ImagePart(image []byte, mime string) Part { return Part{Image: image, MIME: mime} }

// VideoPart returns a video part addressed by URL.
func VideoPart(url, mime string) Part { return Part{VideoURL: url, MIME: mime} }

// IsImage reports whether the part carries an image.
func (p Part) IsImage() bool { return len(p.Image) > 0 }

// IsVideo reports whether the part carries a video.
func (p Part) IsVideo() bool { return p.VideoURL != "" || p.InlineVideo != nil }

// Message is one turn of the conversation.
type Message struct {
	Role  Role
	Parts []Part
}

// Request is one completion call. `Model` is filled in by the registry from the ref the
// caller resolved; callers set everything else.
type Request struct {
	Model    string
	System   string
	Messages []Message
	// JSONSchema, when non-nil, asks for structured output conforming to it. The registry
	// refuses it for a model that does not declare `structured_output`, before any network
	// call, so a caller that wants a plain-text fallback checks the model's flag first.
	JSONSchema []byte
	// MaxTokens caps the completion. Zero means the registry's default.
	MaxTokens int
	// Reasoning is the STAGE policy's intent. The registry replaces it with the operator's
	// override for Stage when the catalog holds one.
	Reasoning ReasoningEffort
	// DisableReasoning tells OpenRouter to send reasoning.enabled=false instead of an effort.
	// The registry derives it when `none` is requested for a model whose recorded effort
	// vocabulary does not contain `none`; callers never set it.
	DisableReasoning bool
	// Stage names the user-facing stage this call is for, in the stable form
	// StageNameObserve/Write/Analyze carry. It is what makes the override resolvable: the
	// operator curates an effort per (model, purpose), and one model may observe at one
	// strength and write at another in a single run. Empty means "no stage in particular",
	// which resolves to no override and keeps whatever Reasoning the caller set.
	Stage string
	// Execution is mandatory for paid clip calls. It preserves the admitted policy;
	// neither the registry nor an adapter may relax it or apply a newer override.
	Execution *ExecutionPolicy
}

// HasImages reports whether any message carries an image part. It stays image-only: it is
// what the vision check reads, and a video must not make a model look image-capable.
func (r Request) HasImages() bool {
	for _, m := range r.Messages {
		for _, p := range m.Parts {
			if p.IsImage() {
				return true
			}
		}
	}
	return false
}

// HasVideos reports whether any message carries a video part.
func (r Request) HasVideos() bool {
	for _, m := range r.Messages {
		for _, p := range m.Parts {
			if p.IsVideo() {
				return true
			}
		}
	}
	return false
}

// Usage is what the provider reported, when it did. Zero values mean "not reported".
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	// ReasoningTokens is the part of CompletionTokens the provider attributes to reasoning.
	// Without it "wrote 8,192 tokens of post" and "spent 8,192 tokens thinking and wrote
	// nothing" are the same row — the 2026-09-03 failures had to be inferred from an empty
	// body. Zero means not reported, like the other fields.
	ReasoningTokens int
	// CostMicrousd is the provider-reported charged amount in millionths of one USD.
	// CostReported distinguishes a real zero charge from a provider that omitted cost.
	CostMicrousd int64
	CostReported bool
}

// Response is the completion.
type Response struct {
	// Text is the raw completion, or the JSON document when JSONSchema was set.
	Text         string
	Usage        Usage
	FinishReason string
}

// Provider is one registered vendor endpoint. Implementations live in sub-packages and
// are handed to the registry by the composition root; nothing else constructs one.
type Provider interface {
	Name() string
	// Complete may return reported Usage and FinishReason together with an error. A
	// failed provider call can still have consumed billable tokens.
	Complete(ctx context.Context, req Request) (Response, error)
}

// AdapterConfig is what an adapter needs from a providers.yaml entry. The key is
// resolved from the environment by the registry, never read from the file.
type AdapterConfig struct {
	ProviderID      string
	BaseURL         string
	APIKey          string
	ReasoningFormat string
	// See Options.EndpointCacheTTL and Options.EndpointFetchTimeout.
	EndpointCacheTTL     time.Duration
	EndpointFetchTimeout time.Duration
}

// AdapterFactory builds a Provider for one yaml entry. It must validate what it needs
// (an `openai_compatible` entry needs a `base_url`) and fail loudly — a broken entry
// stops the process at boot. It is called even when the key is unset, so the entry is
// validated regardless; the registry marks such a provider disabled.
type AdapterFactory func(cfg AdapterConfig) (Provider, error)
