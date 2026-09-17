package composition

import (
	"maps"
	"slices"
	"strings"
)

// ConvertLegacyTemplate carries what the template grammar no longer holds —
// footage sections, their guides and every scene-bound or cut-bound text — into
// one invisible guide, so a template saved under the old grammar loses nothing
// silently (CLIP-140). Existing root guides keep their place and the carried
// prose lands where the first removed construct stood. A body the template
// grammar already accepts comes back unchanged with changed=false, and a body
// whose carried prose would not fit GuideChars is refused as guide_limit rather
// than truncated: the owner decides what to drop, never the converter.
func ConvertLegacyTemplate(body string, l Limits) (string, bool, *Problem) {
	if _, problem := ParseTemplate(body, l); problem == nil {
		return body, false, nil
	}
	d, problem := ReadStored(body, l)
	if problem != nil {
		return body, false, problem
	}
	root := &Node{Name: "clip", Attributes: map[string]string{}}
	// The root's design attributes go with the retired constructs: the presets
	// a clip renders in are the project's, so a body that still names them is
	// carrying a value nothing reads (CLIP-14, CLIP-144).
	for k, v := range d.Root.Attributes {
		if !slices.Contains([]string{"styles", "intro", "caption", "outro"}, k) {
			root.Attributes[k] = v
		}
	}
	var carried []string
	var lifted []*Node
	insertAt, firstLine := -1, 1
	remove := func(n *Node) {
		if insertAt < 0 {
			insertAt, firstLine = len(root.Children), n.Span.Line
		}
	}
	for _, n := range d.Root.Children {
		switch n.Name {
		case "scene", "repeat":
			remove(n)
			prose, regions := sectionContents(n)
			carried = append(carried, prose...)
			lifted = append(lifted, regions...)
		case "text":
			if role := n.Attributes["role"]; role == "caption" || role == "info" {
				remove(n)
				if line := textProse(n); line != "" {
					carried = append(carried, line)
				}
				continue
			}
			root.Children = append(root.Children, withoutTiming(withoutStyle(n)))
		default:
			root.Children = append(root.Children, n)
		}
	}
	inserted := []*Node{}
	if len(carried) > 0 {
		prose := strings.Join(carried, "\n\n")
		if scalar(prose) > l.GuideChars {
			return body, false, &Problem{ElementID: "clip", Line: firstLine, Reason: "guide_limit"}
		}
		inserted = append(inserted, &Node{Name: "guide", Attributes: map[string]string{}, Children: []*Node{{Name: "#text", Text: prose}}})
	}
	inserted = append(inserted, lifted...)
	if len(inserted) > 0 {
		root.Children = slices.Insert(root.Children, insertAt, inserted...)
	}
	converted := SerializeNode(root)
	// The carry-over is all this owes. A converted body may still hold an
	// authoring error the owner has to fix — an intro or outro that departs from
	// its preset is exactly CLIP-114's case — and refusing to convert would hide
	// the carried prose behind that error instead of showing both.
	read, problem := ReadStored(converted, l)
	if problem != nil {
		return body, false, problem
	}
	if len(read.Sections) > 0 {
		return body, false, fail("clip", 1, "unsupported_section")
	}
	return converted, true, nil
}

// sectionContents reads a scene, or every scene of a repeat, as guide paragraphs
// — the scene's own guides first, then one line per scene-bound text — and lifts
// the intro or outro elements a template authored inside a scene, because those
// are fixed regions the template keeps rather than scene-bound copy (CLIP-114).
func sectionContents(n *Node) (prose []string, regions []*Node) {
	for _, c := range n.Children {
		switch c.Name {
		case "scene":
			p, r := sectionContents(c)
			prose, regions = append(prose, p...), append(regions, r...)
		case "guide":
			if v := collapse(rawText(c)); v != "" {
				prose = append(prose, v)
			}
		case "text":
			if role := c.Attributes["role"]; role == "hook" || role == "ending" {
				regions = append(regions, liftedRegion(c))
				continue
			}
			if line := textProse(c); line != "" {
				prose = append(prose, line)
			}
		}
	}
	return prose, regions
}

// liftedRegion carries a region element a template authored inside a scene up
// to the root, where an outline entry belongs. Position and alignment are left
// exactly as authored, so a departure stays visible as CLIP-114's authoring
// error rather than being silently corrected.
func liftedRegion(n *Node) *Node { return withoutTiming(withoutStyle(n)) }

// withoutTiming drops the interval an entry used to declare: where an entry
// stands in the outline is the only position it has now, and the intro, the
// outro and the badge take the spans the renderer gives them (CLIP-66,
// CLIP-112).
func withoutTiming(n *Node) *Node {
	if !slices.ContainsFunc([]string{"basis", "start", "end"}, func(k string) bool {
		_, present := n.Attributes[k]
		return present
	}) {
		return n
	}
	out := *n
	out.Attributes = maps.Clone(n.Attributes)
	for _, k := range []string{"basis", "start", "end"} {
		delete(out.Attributes, k)
	}
	return &out
}

// textProse states one element the way a reader would: its id, whether the
// writer or the author owned its words, then the words with every value
// reference kept as {field} so a fact binding survives as a visible reference.
func textProse(n *Node) string {
	var rows []string
	if slices.ContainsFunc(n.Children, func(c *Node) bool { return c.Name == "row" }) {
		for _, c := range n.Children {
			if c.Name == "row" {
				if v := collapse(partsText(c)); v != "" {
					rows = append(rows, v)
				}
			}
		}
	} else if v := collapse(partsText(n)); v != "" {
		rows = append(rows, v)
	}
	if len(rows) == 0 {
		return ""
	}
	kind := n.Attributes["kind"]
	if kind == "" {
		kind = "fixed"
	}
	return n.Attributes["id"] + " [" + kind + "]: " + strings.Join(rows, " · ")
}

func partsText(n *Node) string {
	var b strings.Builder
	for _, c := range n.Children {
		switch c.Name {
		case "#text":
			b.WriteString(c.Text)
		case "value":
			b.WriteString("{" + c.Attributes["field"] + "}")
		}
	}
	return b.String()
}

func rawText(n *Node) string {
	var b strings.Builder
	for _, c := range n.Children {
		if c.Name == "#text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// collapse folds authored whitespace into single spaces between paragraphs'
// words; a guide carries prose, not layout.
func collapse(s string) string {
	paragraphs := []string{}
	for _, p := range strings.Split(s, "\n\n") {
		if v := strings.Join(strings.Fields(p), " "); v != "" {
			paragraphs = append(paragraphs, v)
		}
	}
	return strings.Join(paragraphs, "\n")
}

// withoutStyle drops the retired per-element style attribute a stored draft may
// still carry; ReadStored tolerates it, the template grammar never did.
func withoutStyle(n *Node) *Node {
	if _, ok := n.Attributes["style"]; !ok {
		return n
	}
	copy := *n
	copy.Attributes = map[string]string{}
	for k, v := range n.Attributes {
		if k != "style" {
			copy.Attributes[k] = v
		}
	}
	return &copy
}
