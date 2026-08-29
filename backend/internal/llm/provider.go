// Package llm is the port every model call goes through (PRD §6.4). It is a boundary,
// not a preference: nothing above it learns which vendor answered, and no adapter or
// SDK type appears above it (ARCHITECTURE §2.1). The model is an input to every call —
// observation and writing choose theirs per stage ([I3]) — so the port reads no default.
package llm

import "context"

// Role is who said a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Part is one piece of a message: text, or one image. Exactly one is set.
type Part struct {
	Text string
	// Image is the encoded bytes of one image with its MIME type — the JPEG the browser
	// pipeline produced, read back from object storage by the caller.
	Image []byte
	MIME  string
}

// TextPart returns a text part.
func TextPart(text string) Part { return Part{Text: text} }

// ImagePart returns an image part.
func ImagePart(image []byte, mime string) Part { return Part{Image: image, MIME: mime} }

// IsImage reports whether the part carries an image.
func (p Part) IsImage() bool { return len(p.Image) > 0 }

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
}

// HasImages reports whether any message carries an image part.
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

// Usage is what the provider reported, when it did. Zero values mean "not reported".
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	// CostMicrousd is the provider-reported charged amount in millionths of one USD.
	// CostReported distinguishes a real zero charge from a provider that omitted cost.
	CostMicrousd int64
	CostReported bool
}

// Response is the completion.
type Response struct {
	// Text is the raw completion, or the JSON document when JSONSchema was set.
	Text  string
	Usage Usage
}

// Provider is one registered vendor endpoint. Implementations live in sub-packages and
// are handed to the registry by the composition root; nothing else constructs one.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// AdapterConfig is what an adapter needs from a providers.yaml entry. The key is
// resolved from the environment by the registry, never read from the file.
type AdapterConfig struct {
	ProviderID string
	BaseURL    string
	APIKey     string
}

// AdapterFactory builds a Provider for one yaml entry. It must validate what it needs
// (an `openai_compatible` entry needs a `base_url`) and fail loudly — a broken entry
// stops the process at boot. It is called even when the key is unset, so the entry is
// validated regardless; the registry marks such a provider disabled.
type AdapterFactory func(cfg AdapterConfig) (Provider, error)
