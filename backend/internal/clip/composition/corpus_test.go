package composition_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

type corpusCase struct {
	Name, Body       string
	Error            *composition.Problem
	Inputs           *composition.Inputs
	Summary          map[string]any
	RootSpan         *composition.Span
	DurationMS       int
	ResolvedCount    *int
	Resolved         []map[string]any
	MaxExpandedBytes int
}

func corpus(t *testing.T) (composition.Limits, []corpusCase) {
	t.Helper()
	b, e := os.ReadFile("testdata/corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Limits composition.Limits
		Cases  []corpusCase
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	return f.Limits, f.Cases
}

// The corpus uses language-neutral lowerCamelCase; production domain types need
// no serialization tags merely to support an internal grammar test.
func normalized(t *testing.T, value any) any {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	var v any
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	var norm func(any) any
	norm = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			m := map[string]any{}
			for k, v := range x {
				k = strings.ReplaceAll(strings.ReplaceAll(k, "ID", "Id"), "MS", "Ms")
				if k != "" {
					k = strings.ToLower(k[:1]) + k[1:]
				}
				m[k] = norm(v)
			}
			return m
		case []any:
			for i := range x {
				x[i] = norm(x[i])
			}
			return x
		}
		return v
	}
	return norm(v)
}
func summary(d *composition.Document) map[string]any {
	fields := []map[string]any{}
	for _, f := range d.Fields {
		fields = append(fields, map[string]any{"id": f.ID, "group": f.Group, "label": f.Label, "prompt": f.Prompt, "required": f.Required})
	}
	sections := []map[string]any{}
	for _, s := range d.Sections {
		sections = append(sections, map[string]any{"id": s.ID, "scope": s.Scope, "repeat": s.Repeat})
	}
	elements := []string{}
	for _, e := range d.Elements {
		elements = append(elements, e.ID)
	}
	return map[string]any{"styles": d.Styles, "accent": d.Accent, "pace": d.Pace, "fields": fields, "groups": append([]string{}, d.Groups...), "sections": sections, "elements": elements, "guidance": append([]string{}, d.Guidance...)}
}
func resolution(t *testing.T, out composition.Timeline) []map[string]any {
	t.Helper()
	rows := []map[string]any{}
	for _, r := range out.Elements {
		rows = append(rows, map[string]any{"instanceId": r.InstanceID, "text": r.Text, "startMs": r.StartMS, "endMs": r.EndMS, "authoredTiming": r.AuthoredTiming, "facts": normalized(t, append([]composition.Fact{}, r.Facts...)), "rows": normalized(t, append([]composition.ResolvedRow{}, r.Rows...))})
	}
	return rows
}
func TestSharedCorpus(t *testing.T) {
	l, cases := corpus(t)
	if l != config.ClipCompositionLimits() {
		t.Fatalf("configuration differs from shared contract: %+v", config.ClipCompositionLimits())
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			d, e := composition.Parse(c.Body, l)
			var timeline composition.Timeline
			if e == nil && c.Inputs != nil {
				budget := c.MaxExpandedBytes
				if budget == 0 {
					budget = 1 << 20
				}
				timeline, e = composition.Resolve(d, *c.Inputs, l, budget)
			}
			if c.Error != nil {
				if !reflect.DeepEqual(e, c.Error) {
					t.Fatalf("error got %+v, want %+v", e, c.Error)
				}
				return
			}
			if e != nil {
				t.Fatalf("unexpected error %+v", e)
			}
			if d.Source != c.Body {
				t.Fatal("source changed")
			}
			if c.RootSpan != nil && d.Root.Span != *c.RootSpan {
				t.Fatalf("span %+v, want %+v", d.Root.Span, *c.RootSpan)
			}
			if c.Summary != nil && !reflect.DeepEqual(normalized(t, summary(d)), c.Summary) {
				t.Fatalf("summary %+v, want %+v", summary(d), c.Summary)
			}
			if c.Inputs != nil {
				if timeline.DurationMS != c.DurationMS || c.Resolved != nil && !reflect.DeepEqual(normalized(t, resolution(t, timeline)), normalized(t, c.Resolved)) {
					t.Fatalf("resolution %+v (%d), want %+v (%d)", resolution(t, timeline), timeline.DurationMS, c.Resolved, c.DurationMS)
				}
				if c.ResolvedCount != nil && len(timeline.Elements) != *c.ResolvedCount {
					t.Fatalf("elements: %d, want %d", len(timeline.Elements), *c.ResolvedCount)
				}
			}
			canonical := composition.SerializeNode(d.Root)
			again, err := composition.Parse(canonical, l)
			if err != nil || composition.SerializeNode(again.Root) != canonical {
				t.Fatalf("canonical round trip failed: %+v", err)
			}
			if c.Inputs != nil {
				budget := c.MaxExpandedBytes
				if budget == 0 {
					budget = 1 << 20
				}
				againOut, err := composition.Resolve(again, *c.Inputs, l, budget)
				if err != nil || !reflect.DeepEqual(resolution(t, timeline), resolution(t, againOut)) {
					t.Fatalf("semantic round trip failed: %+v", err)
				}
			}
		})
	}
}
func TestSubtreeEditPreservesUntouchedSource(t *testing.T) {
	l := config.ClipCompositionLimits()
	source := " \n<clip version='1'>\n <guide>🧑‍🍳 keep &amp; spacing</guide>\n <text id='copy' kind='fixed' role='caption' basis='whole'>old</text>\n</clip>\n"
	d, e := composition.Parse(source, l)
	if e != nil {
		t.Fatal(e)
	}
	n := d.Root.Children[3]
	if n.Name != "text" {
		t.Fatal(n)
	}
	replacement := *n
	replacement.Children = []*composition.Node{{Name: "#text", Text: "새 <문구>"}}
	updated, e := composition.ReplaceNode(d, "copy", &replacement, l)
	if e != nil {
		t.Fatal(e)
	}
	before, after := []rune(source)[:n.Span.Start], []rune(source)[n.Span.End:]
	if !strings.HasPrefix(updated.Source, string(before)) || !strings.HasSuffix(updated.Source, string(after)) {
		t.Fatal("untouched source changed")
	}
	out, e := composition.Resolve(updated, composition.Inputs{Cuts: []composition.Cut{{ID: "a", SourceID: "source", EndMS: 4000}}}, l, 10000)
	if e != nil || out.Elements[0].Text != "새 <문구>" {
		t.Fatalf("edited value %+v, %+v", out, e)
	}
}
func TestExactSecondsAndAutomaticShortCutRefusal(t *testing.T) {
	for _, c := range []struct {
		s  string
		ms int
		ok bool
	}{{"0.001", 1, true}, {"01.020", 1020, true}, {"-2.5", -2500, true}, {"90", 90000, true}, {"90.001", 0, false}, {"1e2", 0, false}, {"NaN", 0, false}, {"0.0001", 0, false}, {"999999999999999999999999", 0, false}} {
		v, ok := composition.Milliseconds(c.s, 90000)
		if v != c.ms || ok != c.ok {
			t.Fatalf("%s = %d %v", c.s, v, ok)
		}
	}
	l := config.ClipCompositionLimits()
	d, e := composition.Parse(`<clip version="1"><scene id="s"><text id="c" kind="fixed" role="caption" basis="cut">x</text></scene></clip>`, l)
	if e != nil {
		t.Fatal(e)
	}
	_, e = composition.Resolve(d, composition.Inputs{Cuts: []composition.Cut{{ID: "a", SourceID: "v", SectionID: "s", EndMS: 200}}}, l, 10000)
	if e == nil || e.Reason != "interval_outside" {
		t.Fatalf("short cut silently adjusted: %+v", e)
	}
}
func FuzzCompositionParser(f *testing.F) {
	f.Add(`<clip version="1"/>`)
	f.Add(`<clip version="1"><text id="x" kind="fixed" role="caption" basis="whole">한 &amp; 😀</text></clip>`)
	f.Add(`<!DOCTYPE clip [<!ENTITY a SYSTEM "file:///secret">]>`)
	f.Fuzz(func(t *testing.T, s string) {
		d, e := composition.Parse(s, config.ClipCompositionLimits())
		if e != nil {
			return
		}
		if d.Source != s {
			t.Fatal("source changed")
		}
		serialized := composition.SerializeNode(d.Root)
		again, e := composition.Parse(serialized, config.ClipCompositionLimits())
		if e == nil && composition.SerializeNode(again.Root) != serialized {
			t.Fatal("unstable serializer")
		}
	})
}

func TestQualifiedFieldEditAndUnlabelledGuide(t *testing.T) {
	l := config.ClipCompositionLimits()
	source := `<clip version="1"><guide>keep</guide><group id="a"><field id="price" label="A"/></group><group id="b"><field id="price" label="B"/></group></clip>`
	d, e := composition.Parse(source, l)
	if e != nil {
		t.Fatal(e)
	}
	replacement := &composition.Node{Name: "field", Attributes: map[string]string{"id": "price", "label": "changed"}}
	d, e = composition.ReplaceNode(d, "a.price", replacement, l)
	if e != nil || d.Fields[0].Label != "changed" || d.Fields[1].Label != "B" {
		t.Fatalf("wrong field changed: %+v %v", d, e)
	}
	guide := d.Root.Children[0]
	d, e = composition.ReplaceSpan(d, guide.Span, &composition.Node{Name: "guide", Attributes: map[string]string{}, Children: []*composition.Node{{Name: "#text", Text: "new guidance"}}}, l)
	if e != nil || d.Guidance[0] != "new guidance" {
		t.Fatalf("guide edit: %+v %v", d, e)
	}
	if _, e = composition.ReplaceSpan(d, composition.Span{Start: 1, End: 3, Line: 1}, replacement, l); e == nil {
		t.Fatal("arbitrary span accepted")
	}
}
