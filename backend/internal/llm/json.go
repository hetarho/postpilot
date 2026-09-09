package llm

import (
	"encoding/json"
	"strings"
)

// JSONCandidate extracts the first complete JSON object, including the existing
// fenced/prose fallback used by generation. It performs no provider repair call.
func JSONCandidate(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		trimmed = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(trimmed), "```"))
	}
	if json.Valid([]byte(trimmed)) && strings.HasPrefix(trimmed, "{") {
		return trimmed, true
	}
	start := strings.IndexByte(trimmed, '{')
	if start < 0 {
		return "", false
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(trimmed); i++ {
		c := trimmed[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := trimmed[start : i+1]
				return candidate, json.Valid([]byte(candidate))
			}
		}
	}
	return "", false
}

// ResponseParseError preserves billable truncation diagnostics when a partial
// completion cannot be parsed or validated. Usable length-limited JSON stays valid.
func ResponseParseError(response Response, err error) error {
	if err == nil || response.FinishReason != "length" {
		return err
	}
	if response.Usage.ReasoningTokens > 0 {
		return &TruncatedError{ReasoningTokens: response.Usage.ReasoningTokens, CompletionTokens: response.Usage.CompletionTokens}
	}
	return ErrOutputTruncated
}
