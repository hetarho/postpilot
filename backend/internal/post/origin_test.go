package post

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSemanticOriginSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/semantic-origin.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version int
		Current OriginResultIdentity
		Cases   []struct {
			Name               string
			Content            PostContent
			Sources            []OriginSource
			Candidates         *[]OriginCandidate
			Spans              []OriginSpan
			ExpectedSpans      []OriginSpan
			ExpectedIssueCodes []OriginIssueCode
			InvalidUnicode     bool
			Result             *OriginResultIdentity
			Version            *int
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != OriginVersion || len(fixture.Cases) == 0 {
		t.Fatalf("invalid shared fixture version/count: %d/%d", fixture.Version, len(fixture.Cases))
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			var got OriginResolution
			if test.InvalidUnicode {
				// JSON decoders repair unpaired surrogate escapes differently. The
				// shared fixture adapter supplies the same invalid scalar directly.
				invalid := string([]byte{0xed, 0xa0, 0x80})
				test.Content.Title = invalid
				(*test.Candidates)[0].Quote = invalid
			}
			before, err := json.Marshal(test.Content)
			if err != nil {
				t.Fatal(err)
			}
			if test.Candidates != nil {
				got = ResolveOriginCandidates(test.Content, fixture.Current, test.Sources, *test.Candidates)
			} else {
				version, result := fixture.Version, fixture.Current
				if test.Version != nil {
					version = *test.Version
				}
				if test.Result != nil {
					result = *test.Result
				}
				got = ValidateOriginReview(test.Content, fixture.Current, &OriginReview{
					Version: version, Result: result, Sources: test.Sources, Spans: test.Spans,
				})
			}
			if !slices.EqualFunc(got.Review.Spans, test.ExpectedSpans, func(a, b OriginSpan) bool {
				return reflect.DeepEqual(a, b)
			}) {
				t.Fatalf("spans = %+v, want %+v", got.Review.Spans, test.ExpectedSpans)
			}
			var codes []OriginIssueCode
			for _, issue := range got.Issues {
				codes = append(codes, issue.Code)
			}
			if !slices.Equal(codes, test.ExpectedIssueCodes) {
				t.Fatalf("issues = %+v, want codes %v", got.Issues, test.ExpectedIssueCodes)
			}
			after, err := json.Marshal(test.Content)
			if err != nil || !slices.Equal(before, after) {
				t.Fatalf("origin-only validation changed canonical content: %s -> %s (%v)", before, after, err)
			}
		})
	}
}

func TestOriginLegacyContentIsUnconfirmedWithoutCanonicalChanges(t *testing.T) {
	content := PostContent{Title: "Legacy", Blocks: []Block{{Type: BlockText, Content: "readable body"}}}
	current := OriginResultIdentity{ContentRevision: 7, ContentHash: "current"}
	got := ValidateOriginReview(content, current, nil)
	if len(got.Review.Spans) != 0 || !reflect.DeepEqual(got.Issues, []OriginIssue{{Index: -1, Code: OriginIssueMissing}}) {
		t.Fatalf("legacy resolution = %+v", got)
	}
	if err := ValidateContent(content, nil, nil); err != nil {
		t.Fatalf("missing origin information invalidated canonical content: %v", err)
	}
}

func TestOriginMetadataBoundsAreIndependentOfCanonicalContent(t *testing.T) {
	content := PostContent{Title: "ab", Blocks: []Block{{Type: BlockText, Content: "c"}}}
	current := OriginResultIdentity{ContentRevision: 7, ContentHash: "current"}
	candidate := OriginCandidate{Field: OriginFieldLocator{Kind: OriginFieldTitle}, Quote: "a", Category: OriginAIAdded}
	validSource := OriginSource{ID: "source", Kind: OriginSourceMemo, Text: "owner supplied text", Available: true}
	tooManySources := make([]OriginSource, OriginMaxSources+1)
	tooLongText := validSource
	tooLongText.Text = strings.Repeat("가", OriginMaxSourceTextChars+1)
	tooLongID := validSource
	tooLongID.ID = strings.Repeat("가", OriginMaxSourceIDChars+1)
	tooManyRefs := candidate
	tooManyRefs.SourceRefs = make([]string, OriginMaxRefsPerSpan+1)
	for _, test := range []struct {
		name       string
		sources    []OriginSource
		candidates []OriginCandidate
	}{
		{"catalog count", tooManySources, []OriginCandidate{candidate}},
		{"source text scalars", []OriginSource{tooLongText}, []OriginCandidate{candidate}},
		{"source id scalars", []OriginSource{tooLongID}, []OriginCandidate{candidate}},
		{"references per span", nil, []OriginCandidate{tooManyRefs}},
		{"annotations per readable scalar", nil, []OriginCandidate{candidate, candidate, candidate, candidate}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveOriginCandidates(content, current, test.sources, test.candidates)
			if len(got.Review.Spans) != 0 || len(got.Issues) != 1 || got.Issues[0].Code != OriginIssueMetadataLimit {
				t.Fatalf("excessive metadata resolution = %+v", got)
			}
			if err := ValidateContent(content, nil, nil); err != nil {
				t.Fatalf("excessive metadata invalidated usable content: %v", err)
			}
		})
	}
}

func TestOriginFrozenResultDoesNotAliasMutableInput(t *testing.T) {
	content := PostContent{Tags: []string{"owner"}, Blocks: []Block{{Type: BlockText, Content: "body"}}}
	current := OriginResultIdentity{ContentRevision: 7, ContentHash: "current"}
	index := 0
	sources := []OriginSource{{ID: "memo-1", Kind: OriginSourceMemo, Text: "owner", Available: true}}
	candidates := []OriginCandidate{{Field: OriginFieldLocator{Kind: OriginFieldTag, TagIndex: &index},
		Quote: "owner", Category: OriginOwnerInput, SourceRefs: []string{"memo-1"}}}
	got := ResolveOriginCandidates(content, current, sources, candidates)
	if len(got.Review.Spans) != 1 || len(got.Issues) != 0 {
		t.Fatalf("valid result = %+v", got)
	}
	sources[0].Text = "new unrelated memo"
	sources[0].ID = "new-id"
	candidates[0].SourceRefs[0] = "new-id"
	index = 9
	span := got.Review.Spans[0]
	if got.Review.Sources[0].Text != "owner" || got.Review.Sources[0].ID != "memo-1" ||
		span.SourceRefs[0] != "memo-1" || *span.Field.TagIndex != 0 {
		t.Fatalf("frozen result retargeted to mutable inputs: %+v", got.Review)
	}
	validated := ValidateOriginReview(content, current, &got.Review)
	got.Review.Sources[0].Text = "changed"
	got.Review.Spans[0].SourceRefs[0] = "changed"
	*got.Review.Spans[0].Field.TagIndex = 8
	if validated.Review.Sources[0].Text != "owner" || validated.Review.Spans[0].SourceRefs[0] != "memo-1" ||
		*validated.Review.Spans[0].Field.TagIndex != 0 {
		t.Fatalf("validated result aliases persisted input: %+v", validated.Review)
	}
}

func TestOriginOverlapInvalidatesContainingClusterAndKeepsOtherFields(t *testing.T) {
	content := PostContent{Title: "abcdefghij", Summary: "abc", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	current := OriginResultIdentity{ContentRevision: 7, ContentHash: "current"}
	candidates := []OriginCandidate{
		{Field: OriginFieldLocator{Kind: OriginFieldTitle}, Quote: "c", Category: OriginAIAdded},
		{Field: OriginFieldLocator{Kind: OriginFieldSummary}, Quote: "abc", Category: OriginAIAdded},
		{Field: OriginFieldLocator{Kind: OriginFieldTitle}, Quote: "abcdefghij", Category: OriginAIAdded},
		{Field: OriginFieldLocator{Kind: OriginFieldTitle}, Quote: "i", Category: OriginAIAdded},
	}
	got := ResolveOriginCandidates(content, current, nil, candidates)
	want := []OriginIssue{{Index: 0, Code: OriginIssueOverlap}, {Index: 2, Code: OriginIssueOverlap}, {Index: 3, Code: OriginIssueOverlap}}
	if !reflect.DeepEqual(got.Issues, want) || len(got.Review.Spans) != 1 || got.Review.Spans[0].Field.Kind != OriginFieldSummary {
		t.Fatalf("overlap cluster resolution = %+v, want issues %+v and unchanged summary span", got, want)
	}
}

func TestOriginAnnotationLimitCountsOnlyCanonicalHumanReadableScalars(t *testing.T) {
	content := PostContent{Title: "😀", Summary: "한", Tags: []string{"a"}, Blocks: []Block{
		{Type: BlockText, Content: "b", Alt: "stray text", Caption: "stray text"},
		{Type: BlockHeading, Content: "c"}, {Type: BlockQuote, Content: "d"},
		{Type: BlockList, Items: []string{"e", "f"}, Content: "stray text"},
		{Type: BlockImage, File: "not text.jpg", Alt: "g", Caption: "h"},
		{Type: BlockGallery, Files: []string{"not1.jpg", "not2.jpg"}, Layout: GalleryCollage, Alt: "i", Caption: "j"},
		{Type: BlockVideo, File: "not text.mp4", Alt: "k", Caption: "l"},
	}}
	if got := OriginAnnotationLimit(content); got != 14 {
		t.Fatalf("annotation scalar limit = %d, want 14", got)
	}
}

func TestOriginChecksCorrespondenceWithoutCertifyingSemanticSupport(t *testing.T) {
	content := PostContent{Title: "sweet aroma", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	current := OriginResultIdentity{ContentRevision: 7, ContentHash: "current"}
	// The frozen memo does not prove this sensory claim. Pure structural validation
	// must not invent factual certification or secretly relabel its category.
	sources := []OriginSource{{ID: "memo", Kind: OriginSourceMemo, Text: "tasted good", Available: true}}
	candidate := OriginCandidate{Field: OriginFieldLocator{Kind: OriginFieldTitle}, Quote: "sweet aroma",
		Category: OriginOwnerInput, SourceRefs: []string{"memo"}}
	got := ResolveOriginCandidates(content, current, sources, []OriginCandidate{candidate})
	if len(got.Issues) != 0 || len(got.Review.Spans) != 1 || got.Review.Spans[0].ReviewState != OriginUnreviewed ||
		got.Review.Spans[0].Category != OriginOwnerInput {
		t.Fatalf("structural validation implied semantic certification: %+v", got)
	}
}
