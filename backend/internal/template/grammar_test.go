package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fixture shape is shared with the frontend parser. Both suites read the SAME file, so a
// grammar rule cannot land on one side only (spec/legacy/tech/post-template-grammar.md §4).
type fixtureNode struct {
	T        string        `json:"t"`
	Raw      string        `json:"raw"`
	Text     string        `json:"text"`
	Kind     string        `json:"kind"`
	Label    string        `json:"label"`
	Count    int           `json:"count"`
	Each     string        `json:"each"`
	Children []fixtureNode `json:"children"`
}

// fixtureParseOptions is the ceilings the SHARED fixture declares (`photoRowMax`,
// `askMaxPerBody`), not the ones this deployment configured. A fixture that read the
// environment would pass or fail depending on where it ran, and the TypeScript harness could
// not reproduce it at all.
var fixtureParseOptions = ParseOptions{PhotoRowMax: 4, AskMaxPerBody: 3}

type fixtureCase struct {
	Name string `json:"name"`
	// TitleArea is "" when the case has none (TMPL-50).
	TitleArea  string        `json:"titleArea"`
	Body       string        `json:"body"`
	TitleNodes []fixtureNode `json:"titleNodes"`
	Nodes      []fixtureNode `json:"nodes"`
	Error      *struct {
		Line   int    `json:"line"`
		Reason string `json:"reason"`
		// Area is where the refusal sits; absent means the body.
		Area string `json:"area"`
	} `json:"error"`
}

func loadFixtures(t *testing.T) []fixtureCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "grammar", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		PhotoRowMax   int           `json:"photoRowMax"`
		AskMaxPerBody int           `json:"askMaxPerBody"`
		Cases         []fixtureCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("grammar fixtures are empty")
	}
	// The two harnesses must run the same ceiling or a `count` case means different things
	// on the two sides, which is the exact drift this file exists to prevent.
	if file.PhotoRowMax != fixtureParseOptions.PhotoRowMax {
		t.Fatalf("fixture photoRowMax = %d, want %d", file.PhotoRowMax, fixtureParseOptions.PhotoRowMax)
	}
	if file.AskMaxPerBody != fixtureParseOptions.AskMaxPerBody {
		t.Fatalf("fixture askMaxPerBody = %d, want %d", file.AskMaxPerBody, fixtureParseOptions.AskMaxPerBody)
	}
	return file.Cases
}

// Every case is a (title area, body) pair parsed as one template. A case with no title area
// must also give the body-only Parse's identical verdict, which is what keeps every template
// saved before title areas existed exactly where it was.
func TestParseAgainstSharedFixtures(t *testing.T) {
	for _, tc := range loadFixtures(t) {
		t.Run(tc.Name, func(t *testing.T) {
			title, nodes, err := ParseTemplate(tc.TitleArea, tc.Body, fixtureParseOptions)
			if tc.TitleArea == "" {
				bodyOnly, bodyErr := Parse(tc.Body, fixtureParseOptions)
				if !sameVerdict(nodes, err, bodyOnly, bodyErr) {
					t.Fatalf("ParseTemplate with no title area = (%v, %v), Parse = (%v, %v)", kinds(nodes), err, kinds(bodyOnly), bodyErr)
				}
			}
			if tc.Error != nil {
				if err == nil {
					t.Fatalf("expected %s on line %d, parsed %d title and %d body nodes", tc.Error.Reason, tc.Error.Line, len(title), len(nodes))
				}
				parseErr, ok := err.(*ParseError)
				if !ok {
					t.Fatalf("error %v is not a ParseError", err)
				}
				wantArea := tc.Error.Area
				if wantArea == "" {
					wantArea = AreaBody
				}
				if parseErr.Reason != tc.Error.Reason || parseErr.Line != tc.Error.Line || parseErr.Area != wantArea {
					t.Fatalf("got %s on %s line %d, want %s on %s line %d",
						parseErr.Reason, parseErr.Area, parseErr.Line, tc.Error.Reason, wantArea, tc.Error.Line)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			assertNodes(t, title, tc.TitleNodes, "titleNodes")
			assertNodes(t, nodes, tc.Nodes, "")
			// Every accepted area must serialize back byte-for-byte: this is the round-trip
			// guarantee the builder's 원문 toggle rests on (change 25 AC8).
			if round := Serialize(title); round != tc.TitleArea {
				t.Fatalf("round trip changed the title area:\n got %q\nwant %q", round, tc.TitleArea)
			}
			if round := Serialize(nodes); round != tc.Body {
				t.Fatalf("round trip changed the body:\n got %q\nwant %q", round, tc.Body)
			}
		})
	}
}

// sameVerdict compares two parses by acceptance, node kinds and the error each refused with.
func sameVerdict(nodes []Node, err error, otherNodes []Node, otherErr error) bool {
	if (err == nil) != (otherErr == nil) {
		return false
	}
	if err != nil {
		first, firstOK := err.(*ParseError)
		second, secondOK := otherErr.(*ParseError)
		return firstOK && secondOK && *first == *second
	}
	return Serialize(nodes) == Serialize(otherNodes) && len(kinds(nodes)) == len(kinds(otherNodes))
}

// A body parsed on its own names the body, and one parsed as a title area names the title.
func TestParseNamesTheAreaItParsed(t *testing.T) {
	for _, body := range []string{"<writer>", "<write>메뉴", "<ask label=\"a\"/><ask label=\"a\"/>", "</write>"} {
		_, err := Parse(body, fixtureParseOptions)
		parseErr, ok := err.(*ParseError)
		if !ok || parseErr.Area != AreaBody {
			t.Errorf("Parse(%q) error = %#v, want the body", body, err)
		}
		titleOpts := fixtureParseOptions
		titleOpts.TitleArea = true
		_, err = Parse(body, titleOpts)
		if parseErr, ok := err.(*ParseError); !ok || parseErr.Area != AreaTitle {
			t.Errorf("Parse(%q) as a title area error = %#v, want the title area", body, err)
		}
	}
	if _, err := Parse(`<slot kind="photo"/>`, fixtureParseOptions); err != nil {
		t.Fatalf("a body still admits a photo position: %v", err)
	}
}

func assertNodes(t *testing.T, got []Node, want []fixtureNode, path string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d nodes, want %d (%v)", path, len(got), len(want), kinds(got))
	}
	for i := range want {
		at := path + "[" + itoa(i) + "]"
		if string(got[i].Kind) != want[i].T {
			t.Fatalf("%s: kind %q, want %q", at, got[i].Kind, want[i].T)
		}
		switch want[i].T {
		case "literal":
			if got[i].Text != want[i].Raw {
				t.Fatalf("%s: literal %q, want %q", at, got[i].Text, want[i].Raw)
			}
		case "write", "note":
			if Decode(got[i].Text) != want[i].Text {
				t.Fatalf("%s: text %q, want %q", at, Decode(got[i].Text), want[i].Text)
			}
		case "slot":
			if string(got[i].SlotKind) != want[i].Kind {
				t.Fatalf("%s: slot kind %q, want %q", at, got[i].SlotKind, want[i].Kind)
			}
			if Decode(got[i].Label) != want[i].Label {
				t.Fatalf("%s: slot label %q, want %q", at, Decode(got[i].Label), want[i].Label)
			}
			if got[i].Count != want[i].Count {
				t.Fatalf("%s: slot count %d, want %d", at, got[i].Count, want[i].Count)
			}
		case "ask":
			if Decode(got[i].Label) != want[i].Label {
				t.Fatalf("%s: ask label %q, want %q", at, Decode(got[i].Label), want[i].Label)
			}
			if Decode(got[i].Text) != want[i].Text {
				t.Fatalf("%s: ask text %q, want %q", at, Decode(got[i].Text), want[i].Text)
			}
		case "repeat":
			if got[i].Each != want[i].Each {
				t.Fatalf("%s: each %q, want %q", at, got[i].Each, want[i].Each)
			}
			assertNodes(t, got[i].Children, want[i].Children, at)
		}
	}
}

func kinds(nodes []Node) []NodeKind {
	out := make([]NodeKind, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.Kind)
	}
	return out
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
