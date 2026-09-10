package openaicompat

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func strictDiagnostic(err error, info llm.CallDiagnostic) error {
	if _, ok := llm.DiagnosticOf(err); ok {
		return err // Keep the inner metadata operation, not the outer completion.
	}
	var provider *llm.ProviderError
	if errors.As(err, &provider) {
		info.UpstreamCode = provider.Code
	}
	if info.Class == "" {
		info.Class = strictFailureClass(err, info)
	}
	return llm.WithCallDiagnostic(sanitizedStrictError(err), info)
}

func strictFailureClass(err error, info llm.CallDiagnostic) string {
	var provider *llm.ProviderError
	var network net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
		return "timeout"
	case errors.Is(err, errStrictBody):
		return "body_read"
	case info.HTTPStatus >= 300:
		return "http_error"
	case errors.As(err, &provider):
		return "provider_error"
	case errors.Is(err, llm.ErrBadOutput), errors.Is(err, llm.ErrOutputTruncated):
		return "invalid_response"
	case errors.Is(err, llm.ErrUnsupported):
		return "unsupported"
	case network != nil || info.Operation == "transport":
		return "network_error"
	default:
		return "unknown"
	}
}

func (c *Client) responseRequestID(header http.Header) string {
	for _, name := range []string{"X-Request-ID", "Request-ID", "CF-Ray"} {
		value := header.Get(name)
		if c.apiKey != "" && strings.Contains(value, c.apiKey) {
			continue // Never log even a credential reflected by a response header.
		}
		if id := llm.DiagnosticRequestID(value); id != "" {
			return id
		}
	}
	return ""
}

func numericErrorCode(value any) int {
	var code int
	switch value := value.(type) {
	case float64:
		if value < 100 || value > 599 || value != float64(int(value)) {
			return 0
		}
		code = int(value)
	case string:
		if len(value) != 3 {
			return 0
		}
		code, _ = strconv.Atoi(value)
	}
	if code < 100 || code > 599 {
		return 0
	}
	return code
}
