package voice

import (
	"fmt"
	"strings"
)

// AssembleCorpus preserves every sample body and labels clear boundaries so the model
// can distinguish a habit repeated across posts from one post's local phrasing.
func AssembleCorpus(samples []Sample) string {
	var out strings.Builder
	for i, sample := range samples {
		if i > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "===== 샘플 %d: %s =====\n%s", i+1, sample.Label, sample.Body)
	}
	return out.String()
}
