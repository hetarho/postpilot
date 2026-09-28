package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrStorylineMissing refuses what needs a storyline on a clip that has none: building from it,
// a storyline request and an owner edit (CLIP-178, CLIP-181).
var ErrStorylineMissing = errors.New("clip has no storyline")

// ErrStorylineInvalid refuses an owner edit that does not keep the storyline's shape: another
// paragraph count, a scene outside the made-with sources' observations, a scene named twice, an
// empty paragraph or a text past its bounds (CLIP-178).
var ErrStorylineInvalid = errors.New("clip storyline edit invalid")

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
	// RegionDrafts are the words the call that wrote this storyline left for the project's
	// generated intro/outro slots (CLIP-187), carried to the save that writes them into the
	// slots. They are never stored with the storyline: the slots are where the words live.
	RegionDrafts []RegionDraft `json:"-"`
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

// ApplyStorylineEdit is the owner's edit of the stored storyline (CLIP-178): the same number of
// paragraphs with their texts and scenes replaced, each scene an observation of the sources it
// was made with and none named twice. The result is marked edited by hand and keeps the sources
// it was made with.
func ApplyStorylineEdit(current *Storyline, edit []StorylineParagraph, analyses []SourceAnalysis) (*Storyline, error) {
	if current == nil {
		return nil, ErrStorylineMissing
	}
	if len(edit) != len(current.Paragraphs) {
		return nil, ErrStorylineInvalid
	}
	allowed := map[string]bool{}
	for _, a := range analyses {
		if !slices.Contains(current.MadeWithSources, a.Source.ID) {
			continue
		}
		for i := range a.Segments {
			allowed[ObservationID(a.Source.ID, i)] = true
		}
	}
	used := map[string]bool{}
	out := &Storyline{EditedByHand: true, MadeWithSources: slices.Clone(current.MadeWithSources)}
	bytes := 0
	for _, p := range edit {
		text := strings.TrimSpace(p.Text)
		if utf8.RuneCountInString(text) > StorylineTextMaxChars {
			return nil, ErrStorylineInvalid
		}
		kept := StorylineParagraph{Text: text}
		for _, id := range p.ObservationIDs {
			if !allowed[id] || used[id] {
				return nil, ErrStorylineInvalid
			}
			used[id] = true
			kept.ObservationIDs = append(kept.ObservationIDs, id)
		}
		if text == "" && len(kept.ObservationIDs) == 0 {
			return nil, ErrStorylineInvalid
		}
		if bytes += len(text); bytes > StorylineMaxBytes {
			return nil, ErrStorylineInvalid
		}
		out.Paragraphs = append(out.Paragraphs, kept)
	}
	return out, nil
}

// WithoutRemovedSources takes a removed source out of the storyline (CLIP-178): its scenes leave
// every paragraph and it leaves the sources the storyline was made with. `kept` are the sources
// that stay. The paragraphs themselves stay, text and all.
func (s Storyline) WithoutRemovedSources(kept map[string]bool) Storyline {
	out := Storyline{EditedByHand: s.EditedByHand}
	for _, id := range s.MadeWithSources {
		if kept[id] {
			out.MadeWithSources = append(out.MadeWithSources, id)
		}
	}
	for _, p := range s.Paragraphs {
		paragraph := StorylineParagraph{Text: p.Text}
		for _, id := range p.ObservationIDs {
			if kept[ObservationSource(id)] {
				paragraph.ObservationIDs = append(paragraph.ObservationIDs, id)
			}
		}
		out.Paragraphs = append(out.Paragraphs, paragraph)
	}
	return out
}

// HeldScenes are the observation ids the storyline's paragraphs hold.
func (s Storyline) HeldScenes() map[string]bool {
	out := map[string]bool{}
	for _, p := range s.Paragraphs {
		for _, id := range p.ObservationIDs {
			out[id] = true
		}
	}
	return out
}

// Digest binds an approval to the storyline it was taken against (QUOTA-45): an owner edit
// between the quote and the start invalidates it. Nil is the empty digest.
func (s *Storyline) Digest() string {
	if s == nil {
		return ""
	}
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// StorylineStore saves what the storyline call wrote (CLIP-177): the analysis it read, the
// storyline and the generated slot words; an existing plan draws the changed slot words and
// nothing else of it changes (CLIP-188). An empty analysis keeps the stored one.
type StorylineStore interface {
	// The storyline with the words its call drafted for the generated intro/outro slots
	// (CLIP-187), saved together.
	SaveStoryline(ctx context.Context, user, id, analysis, storyline string, drafts []RegionDraft, now time.Time) (Project, error)
}

// ObservationSource is the source an observation id belongs to.
func ObservationSource(id string) string {
	if at := strings.LastIndex(id, "/"); at >= 0 {
		return id[:at]
	}
	return ""
}
