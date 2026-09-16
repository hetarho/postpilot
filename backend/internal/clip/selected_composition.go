package clip

import (
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
)

// ResolveSelectedComposition keeps real footage even when its item is unknown.
// Only elements that need the missing association are omitted: text bound to one
// of the group's own fields. Generated copy stays, because the scene it describes
// was filmed whether or not the item could be named — CLIP-64 asks such copy to
// fall back to an observed description rather than disappear, and a cut with no
// binding is already held to that: ScopedFact refuses it every item fact, and
// GroundScopedText refuses any sentence naming an item. Literal authored text
// and context survive; no dummy item or fabricated answer enters Resolve.
//
// A plan written today declares no section at all: its cuts carry no section,
// group or item, the template holds only the fixed regions, and the narration
// belongs to no cut (CLIP-134). Every branch below is then skipped and only the
// fixed regions resolve — which is why such a plan records no item notice.
func ResolveSelectedComposition(doc *composition.Document, inputs CompositionInputs, cuts []composition.Cut, limits composition.Limits, maxBytes int) (composition.Timeline, []CopyFallback, error) {
	resolvedDoc := *doc
	resolvedDoc.Sections = slices.Clone(doc.Sections)
	resolvedCuts := slices.Clone(cuts)
	sections := map[string]composition.Section{}
	for _, section := range doc.Sections {
		sections[section.ID] = section
	}
	var fallbacks []CopyFallback
	added := map[string]bool{}
	for i, cut := range cuts {
		section, exists := sections[cut.SectionID]
		if !exists || cut.ItemID != "" {
			continue
		}
		id := "unassigned/" + section.ID
		resolvedCuts[i].SectionID = id
		filtered := section
		filtered.ID, filtered.Repeat, filtered.Elements = id, "scenes", nil
		for _, element := range section.Elements {
			needsItem := false
			parts := slices.Clone(element.Parts)
			for _, row := range element.Rows {
				parts = append(parts, row.Parts...)
			}
			for _, part := range parts {
				needsItem = needsItem || strings.Contains(part.Field, ".")
			}
			if needsItem {
				fallbacks = append(fallbacks, CopyFallback{element.ID, cut.ID, "item_unassigned"})
			} else {
				filtered.Elements = append(filtered.Elements, element)
			}
		}
		if !added[id] {
			resolvedDoc.Sections = append(resolvedDoc.Sections, filtered)
			added[id] = true
		}
	}
	result, problem := composition.Resolve(&resolvedDoc, composition.Inputs{Values: inputs.Values, Items: inputs.Items, Cuts: resolvedCuts}, limits, maxBytes)
	if problem != nil {
		return result, fallbacks, problem
	}
	return result, fallbacks, nil
}
