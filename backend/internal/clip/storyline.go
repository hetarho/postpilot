package clip

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

// A clip's storyline (CLIP-178): the clip told in order, paragraph by paragraph, each naming
// the observed scenes it uses. It names scenes, not files: an observation id
// (`ObservationID(sourceID, index)`) is a scene's identity within its source's analysis.
type StorylineParagraph struct {
	Text           string
	ObservationIDs []string `json:",omitempty"`
}

type Storyline struct {
	Paragraphs []StorylineParagraph
	// EditedByHand is true once the owner changed it; a written storyline starts false.
	EditedByHand bool `json:",omitempty"`
	// MadeWithSources are the sources whose analyses it was written from, so a source added
	// afterwards reads as added and a scene no paragraph holds as taken out.
	MadeWithSources []string `json:",omitempty"`
}

// The bounds a written storyline is held to. The byte cap keeps the widest narration request —
// the widest flow plus the widest storyline — inside the frozen input allowance (CLIP-90);
// thirty paragraphs of two or three sentences each sit well inside it.
const (
	StorylineParagraphMax = 30
	StorylineTextMaxChars = 1000
	StorylineMaxBytes     = 12_000
)

// BoundStoryline keeps what a written storyline may hold, in order: each text trimmed and cut at
// StorylineTextMaxChars, at most StorylineParagraphMax paragraphs and StorylineMaxBytes of text,
// and only ids of observed scenes — an unknown id is dropped and an id already used stays in
// the first paragraph that named it. A paragraph left with neither text nor a scene is dropped.
func BoundStoryline(paragraphs []StorylineParagraph, observed map[string]bool) []StorylineParagraph {
	used := map[string]bool{}
	var out []StorylineParagraph
	bytes := 0
	for _, p := range paragraphs {
		text := cutRunes(strings.TrimSpace(p.Text), StorylineTextMaxChars)
		kept := StorylineParagraph{Text: text}
		for _, id := range p.ObservationIDs {
			if observed[id] && !used[id] {
				used[id] = true
				kept.ObservationIDs = append(kept.ObservationIDs, id)
			}
		}
		if kept.Text == "" && len(kept.ObservationIDs) == 0 {
			continue
		}
		if bytes+len(kept.Text) > StorylineMaxBytes {
			break
		}
		bytes += len(kept.Text)
		out = append(out, kept)
		if len(out) == StorylineParagraphMax {
			break
		}
	}
	return out
}

// ObservedScenes is every observation id the analyses hold, the set a storyline may name.
func ObservedScenes(analyses []SourceAnalysis) map[string]bool {
	out := map[string]bool{}
	for _, a := range analyses {
		for i := range a.Segments {
			out[ObservationID(a.Source.ID, i)] = true
		}
	}
	return out
}

// AddedSources are the current sources the storyline was not written from, in the order given.
func (s Storyline) AddedSources(current []string) []string {
	var out []string
	for _, id := range current {
		if !slices.Contains(s.MadeWithSources, id) {
			out = append(out, id)
		}
	}
	return out
}

// TakenOutObservations are the scenes of the made-with sources' current analyses that no
// paragraph holds, in analysis order.
func (s Storyline) TakenOutObservations(analyses []SourceAnalysis) []string {
	held := map[string]bool{}
	for _, p := range s.Paragraphs {
		for _, id := range p.ObservationIDs {
			held[id] = true
		}
	}
	var out []string
	for _, a := range analyses {
		if !slices.Contains(s.MadeWithSources, a.Source.ID) {
			continue
		}
		for i := range a.Segments {
			if id := ObservationID(a.Source.ID, i); !held[id] {
				out = append(out, id)
			}
		}
	}
	return out
}

// EncodeStoryline is the stored form; nil and an empty storyline are none, stored as "".
func EncodeStoryline(s *Storyline) (string, error) {
	if s == nil || len(s.Paragraphs) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(s)
	return string(raw), err
}

// DecodeStoryline reads the stored form; "" is none.
func DecodeStoryline(raw string) (*Storyline, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var s Storyline
	if err := StrictJSON(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// cutRunes keeps at most max runes of text, never splitting one.
func cutRunes(text string, max int) string {
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	count := 0
	for i := range text {
		if count == max {
			return text[:i]
		}
		count++
	}
	return text
}
