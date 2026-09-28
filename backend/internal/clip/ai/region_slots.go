package ai

import (
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// regionSlotJSON is one intro/outro slot the writer answers, by the id it was
// given (CLIP-187).
type regionSlotJSON struct {
	SlotID    string `json:"slot_id"`
	Text      string `json:"text"`
	ShortText string `json:"short_text"`
}

// regionSlotRule is how a writing call that drafts the intro and the outro
// writes them (CLIP-186, CLIP-187). An empty instruction asks for what the slot's
// role holds in this clip, never for a label nobody asked for.
const regionSlotRule = "intro_outro lists the clip's intro and outro slots in the order they stand. Answer every slot with write true in region_slots, once, with its slot_id: text of at most max_syllables syllables (spaces and punctuation excluded) on at most lines lines and with no newline, and short_text saying the same in fewer syllables, or empty. Follow the slot's instruction; with an empty instruction, write what that slot's role holds in this clip, from the material and the storyline — never a label, name or fact the material does not state. Leave text empty where nothing fits. A slot with write false already shows its own words: never answer it, and do not repeat its words. region_slots is [] when intro_outro is absent.\n"

// regionRewriteRule is what a storyline request adds (CLIP-181): the slot words it
// does not ask about stay.
const regionRewriteRule = "A slot's current_text is what it shows now: keep it wherever the request does not ask for other words.\n"

// regionSlotsPayload is the project's intro and outro as a drafting call reads
// them: every active slot of an enabled region in slot order, carrying its own
// words where the owner fixed them or an answer or a template entry supplies
// them, and otherwise the instruction it is drafted from, with the bounds its
// preset gives it (CLIP-186, CDS-86). A rewrite also states what a drafted slot
// shows now. Nil is a request without an intro or an outro to write.
func regionSlotsPayload(in clip.PlanningInput, limits composition.Limits, rewrite bool) []map[string]any {
	if in.Regions == nil || in.Composition == nil {
		return nil
	}
	var out []map[string]any
	for _, w := range clip.RegionWritingSlots(*in.Regions, in.Design.RegionPresets(), in.Ratio, in.Composition.Snapshot.Body, limits) {
		entry := map[string]any{"slot_id": w.Slot.ID, "region": w.Kind, "order": w.Index + 1, "role": w.Spec.Role, "lines": w.Spec.MaxLines(), "max_syllables": w.Budget(), "write": w.Write}
		if !w.Write {
			entry["text"] = w.Slot.Text
		} else {
			entry["instruction"] = w.Slot.Instruction
			if rewrite && strings.TrimSpace(w.Slot.Text) != "" {
				entry["current_text"] = w.Slot.Text
			}
		}
		out = append(out, entry)
	}
	return out
}

// regionTextPayload is what the intro and the outro show, for a call that writes
// around them and not them (CLIP-135): every active slot with words, in order.
func regionTextPayload(in clip.PlanningInput, limits composition.Limits) []map[string]any {
	if in.Regions == nil || in.Composition == nil {
		return nil
	}
	var out []map[string]any
	for _, w := range clip.RegionWritingSlots(*in.Regions, in.Design.RegionPresets(), in.Ratio, in.Composition.Snapshot.Body, limits) {
		if strings.TrimSpace(w.Slot.Text) != "" {
			out = append(out, map[string]any{"region": w.Kind, "order": w.Index + 1, "text": w.Slot.Text})
		}
	}
	return out
}

// regionDrafts fits the writer's answers to the slots it was asked to write,
// once (CLIP-187, CDS-77): a slot keeps words that fit it, takes the shorter text
// where only that fits, and draws nothing where neither does, the change noticed
// on the slot; one the writer left empty draws nothing and notices nothing. An
// id nobody asked for, a second answer to a slot and an answer to a slot whose
// words are the owner's or an answer's are never taken (CLIP-65).
func regionDrafts(in clip.PlanningInput, written []regionSlotJSON, limits composition.Limits) []clip.RegionDraft {
	if in.Regions == nil || in.Composition == nil {
		return nil
	}
	answers := map[string]regionSlotJSON{}
	for _, answer := range written {
		if _, repeated := answers[answer.SlotID]; !repeated {
			answers[answer.SlotID] = answer
		}
	}
	var out []clip.RegionDraft
	for _, w := range clip.RegionWritingSlots(*in.Regions, in.Design.RegionPresets(), in.Ratio, in.Composition.Snapshot.Body, limits) {
		if !w.Write {
			continue
		}
		answer := answers[w.Slot.ID]
		draft := clip.RegionDraft{SlotID: w.Slot.ID}
		switch {
		case strings.TrimSpace(answer.Text) == "" && strings.TrimSpace(answer.ShortText) == "":
		case w.Fits(answer.Text):
			draft.Text = answer.Text
		case w.Fits(answer.ShortText):
			draft.Text, draft.Notice = answer.ShortText, clip.NoticeShortened(w.Kind)
		default:
			draft.Notice = clip.NoticeOmitted(w.Kind)
		}
		out = append(out, draft)
	}
	return out
}
