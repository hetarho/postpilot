package llm

import (
	"errors"
	"regexp"
)

// CallDiagnostic is server-log-only metadata, never a public Failure or ledger
// field. Its projection accepts code-owned classifications and opaque IDs only.
type CallDiagnostic struct {
	Operation    string
	Class        string
	HTTPStatus   int
	UpstreamCode int
	RequestID    string
}

type diagnosticError struct {
	cause error
	info  CallDiagnostic
}

func (e *diagnosticError) Error() string { return e.cause.Error() }
func (e *diagnosticError) Unwrap() error { return e.cause }

// WithCallDiagnostic adds no text to the error and preserves existing error
// classification. Adapters must still sanitize their underlying error as before.
func WithCallDiagnostic(err error, info CallDiagnostic) error {
	if err == nil {
		return nil
	}
	return &diagnosticError{cause: err, info: safeDiagnostic(info)}
}

func DiagnosticOf(err error) (CallDiagnostic, bool) {
	var diagnostic *diagnosticError
	if !errors.As(err, &diagnostic) {
		return CallDiagnostic{}, false
	}
	return safeDiagnostic(diagnostic.info), true
}

// Reject arbitrary strings, URLs, prose, credentials and truncated header values.
// Unknown request-ID formats are omitted rather than guessed or copied raw.
var diagnosticRequestID = regexp.MustCompile(`^(?:[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}|[a-fA-F0-9]{16,64}(?:-[A-Z]{3})?|req[-_][a-zA-Z0-9]{8,96}|gen-[0-9]{8,16}-[a-zA-Z0-9]{8,96})$`)

func DiagnosticRequestID(value string) string {
	if len(value) > 128 || !diagnosticRequestID.MatchString(value) {
		return ""
	}
	return value
}

func safeDiagnostic(info CallDiagnostic) CallDiagnostic {
	switch info.Operation {
	case "preflight", "metadata", "body", "transport", "response":
	default:
		info.Operation = "unknown"
	}
	switch info.Class {
	case "http_error", "provider_error", "network_error", "timeout", "canceled", "body_read", "invalid_response", "response_limit", "unsupported", "unknown":
	default:
		info.Class = "unknown"
	}
	if info.HTTPStatus < 100 || info.HTTPStatus > 599 {
		info.HTTPStatus = 0
	}
	if info.UpstreamCode < 100 || info.UpstreamCode > 599 {
		info.UpstreamCode = 0
	}
	info.RequestID = DiagnosticRequestID(info.RequestID)
	return info
}
