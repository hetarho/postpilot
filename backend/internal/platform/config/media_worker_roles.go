package config

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// Roles are deployment authorization, not a claim a worker may choose in its
// request. Existing credentials default to the native role for compatibility.
func ParseMediaWorkerRoles(raw string, credentials map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for id := range credentials {
		out[id] = "native"
	}
	if raw == "" {
		return out, nil
	}
	invalid := errors.New("MEDIA_WORKER_ROLES requires unique configured worker ids with native or analysis-verification roles")
	if len(raw) > 16<<10 {
		return nil, invalid
	}
	d := json.NewDecoder(strings.NewReader(raw))
	opening, e := d.Token()
	if e != nil || opening != json.Delim('{') {
		return nil, invalid
	}
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return nil, invalid
		}
		id, ok := key.(string)
		var role string
		if !ok || d.Decode(&role) != nil || credentials[id] == "" || seen[id] || role != "native" && role != "analysis-verification" {
			return nil, invalid
		}
		seen[id] = true
		out[id] = role
	}
	if end, e := d.Token(); e != nil || end != json.Delim('}') {
		return nil, invalid
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, invalid
	}
	return out, nil
}
