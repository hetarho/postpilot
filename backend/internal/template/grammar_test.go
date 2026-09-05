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
	Each     string        `json:"each"`
	Children []fixtureNode `json:"children"`
}

type fixtureCase struct {
	Name  string        `json:"name"`
	Body  string        `json:"body"`
	Nodes []fixtureNode `json:"nodes"`
	Error *struct {
		Line   int    `json:"line"`
		Reason string `json:"reason"`
	} `json:"error"`
}

func loadFixtures(t *testing.T) []fixtureCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "grammar", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []fixtureCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("grammar fixtures are empty")
	}
	return file.Cases
}

func TestParseAgainstSharedFixtures(t *testing.T) {
	for _, tc := range loadFixtures(t) {
		t.Run(tc.Name, func(t *testing.T) {
			nodes, err := Parse(tc.Body)
			if tc.Error != nil {
				var parseErr *ParseError
				if err == nil {
					t.Fatalf("expected %s on line %d, parsed %d nodes", tc.Error.Reason, tc.Error.Line, len(nodes))
				}
				parseErr, ok := err.(*ParseError)
				if !ok {
					t.Fatalf("error %v is not a ParseError", err)
				}
				if parseErr.Reason != tc.Error.Reason || parseErr.Line != tc.Error.Line {
					t.Fatalf("got %s on line %d, want %s on line %d",
						parseErr.Reason, parseErr.Line, tc.Error.Reason, tc.Error.Line)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			assertNodes(t, nodes, tc.Nodes, "")
			// Every accepted body must serialize back byte-for-byte: this is the round-trip
			// guarantee the builder's 원문 toggle rests on (change 25 AC8).
			if round := Serialize(nodes); round != tc.Body {
				t.Fatalf("round trip changed the body:\n got %q\nwant %q", round, tc.Body)
			}
		})
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
