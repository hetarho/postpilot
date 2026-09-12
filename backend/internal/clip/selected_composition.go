package clip

import (
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
)

// ResolveSelectedComposition keeps real footage even when its item is unknown.
// Only elements that need the missing association are omitted. Literal authored
// text and context survive; no dummy item or fabricated answer enters Resolve.
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
			needsItem := element.Kind == "ai" && section.Scope == "item"
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
