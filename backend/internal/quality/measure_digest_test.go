package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// measureDigests pins MeasureSelf's answer over testdata/measure/corpus.json per MeasureVersion.
// A stored self-measurement is recomputed only when its version differs (service.go), and a
// published post never changes revision, so a change to the measurement that forgets the bump
// would freeze the old numbers into the aggregate window for good. This table makes forgetting
// impossible: a new answer under an old version fails here, with the row to add.
var measureDigests = map[int]struct{ corpus, output string }{
	// Before T364: emoji tails stayed on tokens.
	1: {corpus: "a54b4a366f98bd6a1818139ab3aae73d3a3d8a5da5e41d4b542db34ce0016a30", output: "a1de633f7538d65a79045bc547b76b882afd18e83edcddfdac7f7cddca9feee3"},
	// T364: variation selectors, ZWJ, format runes and keycaps trim at 어절 edges.
	2: {corpus: "a54b4a366f98bd6a1818139ab3aae73d3a3d8a5da5e41d4b542db34ce0016a30", output: "52960bbc68eea1b9ef8df64ee08d267ebdbbe7b34a1ce1e6cd749e275e7f8ecc"},
}

// measuredSelf is the projection the digest covers: exactly the stored numbers the aggregate
// reads. TopNoun is left out — nothing reads it (review F16).
type measuredSelf struct {
	Share              *float64 `json:"share"`
	TitleRelevance     *float64 `json:"title_relevance"`
	CharCount          int      `json:"char_count"`
	PhotoCount         int      `json:"photo_count"`
	DistinctBlockTypes int      `json:"distinct_block_types"`
	AvgSentenceLength  *float64 `json:"avg_sentence_length"`
}

// corpusPost is the fixture's own shape: quality.Document carries no JSON tags (ARCH-7).
type corpusPost struct {
	Name     string   `json:"name"`
	Language string   `json:"language"`
	Nouns    []string `json:"nouns"`
	Title    string   `json:"title"`
	Blocks   []struct {
		Type    string   `json:"type"`
		Content string   `json:"content"`
		File    string   `json:"file"`
		Items   []string `json:"items"`
	} `json:"blocks"`
}

func TestMeasureSelfDigestIsPinnedToItsMeasureVersion(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "measure", "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus []corpusPost
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	type measured struct {
		Name string       `json:"name"`
		Self measuredSelf `json:"self"`
	}
	answers := make([]measured, 0, len(corpus))
	for _, post := range corpus {
		doc := Document{Title: post.Title}
		for _, b := range post.Blocks {
			doc.Blocks = append(doc.Blocks, Block{Type: BlockType(b.Type), Content: b.Content, File: b.File, Items: b.Items})
		}
		self := MeasureSelf(Sample{Slug: post.Name, Doc: doc, Language: Language(post.Language), Nouns: post.Nouns})
		answers = append(answers, measured{Name: post.Name, Self: measuredSelf{
			Share: self.Repetition.Share, TitleRelevance: self.Repetition.TitleRelevance,
			CharCount: self.Composition.CharCount, PhotoCount: self.Composition.PhotoCount,
			DistinctBlockTypes: self.Composition.DistinctBlockTypes, AvgSentenceLength: self.Composition.AvgSentenceLength,
		}})
	}
	encoded, err := json.Marshal(answers)
	if err != nil {
		t.Fatal(err)
	}
	corpusDigest, outputDigest := sha(raw), sha(encoded)

	row, ok := measureDigests[MeasureVersion]
	switch {
	case !ok:
		t.Fatalf("MeasureVersion %d has no digest row: add %d: {corpus: %q, output: %q}", MeasureVersion, MeasureVersion, corpusDigest, outputDigest)
	case row.corpus != corpusDigest:
		t.Fatalf("testdata/measure/corpus.json changed: re-pin row %d as {corpus: %q, output: %q} — only if this diff leaves MeasureSelf's code unchanged; otherwise bump MeasureVersion and add a row", MeasureVersion, corpusDigest, outputDigest)
	case row.output != outputDigest:
		t.Fatalf("MeasureSelf's answer changed while MeasureVersion is still %d (stored rows of this version would never be recomputed and published posts never change revision): bump MeasureVersion to %d and add %d: {corpus: %q, output: %q}", MeasureVersion, MeasureVersion+1, MeasureVersion+1, corpusDigest, outputDigest)
	}
	// A bump the corpus cannot see is a bump with no evidence: extend the corpus in the same change.
	seen := map[string]int{}
	for version, r := range measureDigests {
		if other, dup := seen[r.output]; dup {
			t.Errorf("versions %d and %d measure the corpus identically: extend the corpus so the change shows", other, version)
		}
		seen[r.output] = version
	}
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
