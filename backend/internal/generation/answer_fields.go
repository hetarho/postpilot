package generation

import (
	"encoding/json"
	"strings"
)

// These are defensive decode bounds, independent of admitted completion budgets.
// An answer remains a single flat object; no nested object is searched or repaired.
const answerDecodeMaxBytes = 8 << 20
const answerDecodeMaxMembers = 128

type answerFields struct {
	values     map[string]json.RawMessage
	originTail bool
}

// decodeAnswerFields decodes each complete top-level member exactly once. A
// malformed final origins value can leave canonical members usable; the caller
// must still validate those members with the ordinary stage parser.
func decodeAnswerFields(raw string, required, allowed []string) (answerFields, error) {
	fail := func() (answerFields, error) { return answerFields{}, badOutput(raw) }
	if len(raw) > answerDecodeMaxBytes {
		return fail()
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		newline := strings.IndexByte(trimmed, '\n')
		if newline < 0 {
			return fail()
		}
		trimmed = strings.TrimSpace(trimmed[newline+1:])
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "```"))
	}
	if strings.HasPrefix(trimmed, "[") {
		return fail()
	}
	start := strings.IndexByte(trimmed, '{')
	if start < 0 || !plainAnswerWrapper(trimmed[:start]) {
		return fail()
	}
	candidate := trimmed[start:]
	decoder := json.NewDecoder(strings.NewReader(candidate))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return fail()
	}
	fields := make(map[string]json.RawMessage)
	known := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		known[name] = true
	}
	unknown, duplicateOrigins := false, false
	lastKey, originOffset := "", int64(-1)
	for count := 0; decoder.More(); count++ {
		if count >= answerDecodeMaxMembers {
			return fail()
		}
		token, err := decoder.Token()
		if err != nil {
			return fail()
		}
		key, ok := token.(string)
		if !ok {
			return fail()
		}
		if _, duplicate := fields[key]; duplicate {
			if key != "origins" {
				return fail()
			}
			duplicateOrigins = true
		}
		if key != "origins" && !known[key] {
			unknown = true
		}
		lastKey = key
		valueOffset := decoder.InputOffset()
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			if key == "origins" && !unknown && hasFields(fields, required...) && originOnlyTail(candidate[valueOffset:]) {
				delete(fields, "origins")
				return answerFields{values: fields, originTail: true}, nil
			}
			return fail()
		}
		fields[key] = value
		if key == "origins" {
			originOffset = valueOffset
		}
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		if lastKey == "origins" && !unknown && hasFields(fields, required...) && originOffset >= 0 && originOnlyTail(candidate[originOffset:]) {
			delete(fields, "origins")
			return answerFields{values: fields, originTail: true}, nil
		}
		return fail()
	}
	if duplicateOrigins {
		delete(fields, "origins")
	}
	if !plainAnswerWrapper(candidate[decoder.InputOffset():]) {
		return fail()
	}
	return answerFields{values: fields}, nil
}

// The historical prose/fence wrapper is allowed, but another JSON structure
// around or after the object is not prose. This never searches for a later object.
func plainAnswerWrapper(text string) bool {
	return !strings.ContainsAny(text, "[]{}\"") && !strings.HasPrefix(strings.TrimSpace(text), ",")
}

// originOnlyTail tracks string and delimiter boundaries, never JSON meaning. A
// subsequent outer member or conflicting closing delimiter prevents salvage.
func originOnlyTail(tail string) bool {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return true
	}
	if !strings.HasPrefix(tail, ":") {
		return false
	}
	tail = strings.TrimSpace(tail[1:])
	var stack []byte
	inString, escaped := false, false
	for i := 0; i < len(tail); i++ {
		c := tail[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			if len(stack) == 0 {
				return c == '}' && plainAnswerWrapper(tail[i+1:])
			}
			open := stack[len(stack)-1]
			if open == '{' && c != '}' || open == '[' && c != ']' {
				return false
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				return false
			}
		}
	}
	return true
}
