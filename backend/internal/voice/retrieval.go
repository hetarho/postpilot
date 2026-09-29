package voice

import "strings"

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func excerptAroundTarget(body string, target, limit int) string {
	body = strings.TrimSpace(body)
	if target <= 0 || limit < target {
		return firstRunes(body, limit)
	}
	runes := []rune(body)
	if len(runes) <= target {
		return body
	}
	end := min(limit, len(runes))
	for i := target - 1; i < end; i++ {
		if strings.ContainsRune(".!?。\n", runes[i]) {
			return strings.TrimSpace(string(runes[:i+1]))
		}
	}
	return strings.TrimSpace(string(runes[:end]))
}
