package voice

import (
	"fmt"
	"strings"
)

// AssembleCorpus is the analysis call's input: every 학습 글's prose lines under a labelled
// separator, so the model can tell a habit repeated across 학습 글 from one piece's local
// phrasing. Non-prose lines stay out (VOICE-61).
func AssembleCorpus(samples []Sample) string {
	var out strings.Builder
	for i, sample := range samples {
		if i > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "===== 학습 글 %d: %s =====\n%s", i+1, sample.Title(), ProseText(sample.Body))
	}
	return out.String()
}
