package generation

import (
	"log/slog"
	"sort"
	"strings"
)

// ApplyTemplateSlots is the one template-aware pass over a model result. It runs after block
// validation and attachment filtering, and it does exactly two things:
//
//   - turns the copy tokens the model was asked to reproduce into slot blocks, and
//   - inserts any slot whose token never came back.
//
// What it deliberately does NOT do is verify the result against the template. Literal
// fidelity, section order and heading levels are not checked, and no drift fails a run or
// triggers a retry: a generation that came back usable is never thrown away over a
// punctuation difference (change 25 AC10). A slot, on the other hand, cannot be allowed to
// go missing — it is the one thing in the post that a person still has to act on, and a
// silently dropped one is a post that looks finished and is not.
func ApplyTemplateSlots(content PostContent, brief *TemplateBrief) PostContent {
	if brief == nil || len(brief.Slots) == 0 {
		return content
	}
	blocks := make([]Block, 0, len(content.Blocks)+len(brief.Slots))
	// at[i] is where slot i+1's block landed, or -1 while it has not been seen. The model may
	// echo a token twice; the second occurrence is dropped rather than duplicating the slot.
	at := make([]int, len(brief.Slots))
	for i := range at {
		at[i] = -1
	}

	for _, block := range content.Blocks {
		found, rest, exact := slotTokensIn(block, len(brief.Slots))
		if len(found) == 0 {
			blocks = append(blocks, block)
			continue
		}
		// Every slot the block named resolves, in the order the tokens appeared. A model that
		// crammed two tokens into one paragraph must not leave the second one in the prose.
		for _, index := range found {
			if at[index] >= 0 {
				slog.Warn("dropping a repeated template slot token", "slot", index+1)
				continue
			}
			blocks = append(blocks, slotBlock(brief.Slots[index], index+1))
			at[index] = len(blocks) - 1
		}
		if exact {
			continue
		}
		// The tokens arrived inside a longer paragraph. The prose around them is kept: the
		// model wrote something real there, and dropping it would lose more than it saves.
		stripped := block
		stripped.Content = strings.TrimSpace(rest)
		if stripped.Content != "" {
			blocks = append(blocks, stripped)
		}
	}

	// Whatever never came back is inserted at its TEMPLATE position, which is the only position
	// information available: after the nearest preceding slot that did resolve, otherwise before
	// the nearest following one, otherwise at the end. Inserting relative to "the last slot
	// seen" would put slot 1 after slot 2 whenever the model echoed only the later token.
	for i, slot := range brief.Slots {
		if at[i] >= 0 {
			continue
		}
		slog.Warn("inserting a template slot the model omitted", "slot", i+1, "kind", slot.Kind)
		insertAt := len(blocks)
		if before := previousResolved(at, i); before >= 0 {
			insertAt = at[before] + 1
		} else if after := nextResolved(at, i); after >= 0 {
			insertAt = at[after]
		}
		blocks = append(blocks, Block{})
		copy(blocks[insertAt+1:], blocks[insertAt:])
		blocks[insertAt] = slotBlock(slot, i+1)
		at[i] = insertAt
		// Every slot that sat at or after the insertion point moved one block along.
		for j := range at {
			if j != i && at[j] >= insertAt {
				at[j]++
			}
		}
	}

	content.Blocks = blocks
	return content
}

func previousResolved(at []int, from int) int {
	for j := from - 1; j >= 0; j-- {
		if at[j] >= 0 {
			return j
		}
	}
	return -1
}

func nextResolved(at []int, from int) int {
	for j := from + 1; j < len(at); j++ {
		if at[j] >= 0 {
			return j
		}
	}
	return -1
}

// slotBlock is the canonical unfilled slot: a TEXT block whose CONTENT IS THE TOKEN, plus
// the typed marker the slot-aware surfaces read.
//
// The content is the token rather than the author's label for one load-bearing reason: a
// revision receives the current content and hands untouched blocks back byte for byte, but
// the structured-output schema cannot carry the `slot` marker (its block object is closed).
// A token in the content survives that round trip, which is what lets this pass find the slot
// again AT ITS OWN POSITION instead of re-appending it at the end of the post on every
// revision. The label is what slot-aware surfaces display; the token is how it is recognized.
func slotBlock(slot TemplateSlot, number int) Block {
	label := strings.TrimSpace(slot.Label)
	if label == "" {
		label = slot.Kind
	}
	return Block{
		Type:    BlockText,
		Content: slotToken(number),
		Slot:    &BlockSlot{Kind: slot.Kind, Label: label},
	}
}

// slotTokensIn reports every slot a block's content refers to, in the order the tokens appear,
// together with the content left after removing them. `exact` says the block was ONLY a token —
// the requested shape, and what this pass itself produces.
//
// Matching is by token even on a block that already carries a marker, which is what makes the
// pass idempotent: it runs once on a write and again on every revision of that post.
func slotTokensIn(block Block, slots int) (found []int, rest string, exact bool) {
	if block.Type != BlockText {
		return nil, block.Content, false
	}
	rest = block.Content
	type hit struct {
		index int
		at    int
	}
	hits := make([]hit, 0, 2)
	for i := 0; i < slots; i++ {
		if at := strings.Index(rest, slotToken(i+1)); at >= 0 {
			hits = append(hits, hit{index: i, at: at})
		}
	}
	if len(hits) == 0 {
		return nil, rest, false
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].at < hits[b].at })
	for _, h := range hits {
		found = append(found, h.index)
		rest = strings.ReplaceAll(rest, slotToken(h.index+1), "")
	}
	return found, rest, strings.TrimSpace(rest) == ""
}

// slotToken mirrors the template context's token form. It is duplicated rather than imported
// because the two contexts must not depend on each other (ARCHITECTURE §2.2); the shared
// truth is the grammar spec, and the golden prompts fail loudly if the two ever disagree.
func slotToken(n int) string {
	return "{{slot:" + itoa(n) + "}}"
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
