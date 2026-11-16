package rpc

import (
	"context"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
)

// GetPostFingerprint is ②'s reading of the caller's post against its voice (POST-102).
func (h *Handler) GetPostFingerprint(ctx context.Context, req *connect.Request[postpilotv1.GetPostFingerprintRequest]) (*connect.Response[postpilotv1.GetPostFingerprintResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.PostFingerprint(ctx, userID, req.Msg.GetPostSlug())
	if err != nil {
		return nil, toConnectError("get post fingerprint", err)
	}
	return connect.NewResponse(&postpilotv1.GetPostFingerprintResponse{
		Applicable: found.Applicable, Revision: found.Revision, Items: ToProtoComparisons(found.Items),
		Habits: toProtoHabits(found.Habits),
	}), nil
}

// ToProtoComparisons is the fingerprint comparison on the wire, in the order the domain gives,
// shared by ②, 검증 and 말투 반영 비교 (VOICE-62).
func ToProtoComparisons(items []voice.ItemComparison) []*postpilotv1.FingerprintItemComparison {
	out := make([]*postpilotv1.FingerprintItemComparison, 0, len(items))
	for _, item := range items {
		facets := make([]*postpilotv1.FingerprintFacet, 0, len(item.Facets))
		for _, facet := range item.Facets {
			facets = append(facets, &postpilotv1.FingerprintFacet{
				Key: facet.Key, Unit: toProtoFacetUnit(facet.Unit),
				Voice: toProtoFacetValue(facet.Unit, facet.Voice, facet.VoiceTerms),
				Text:  toProtoFacetValue(facet.Unit, facet.Text, facet.TextTerms),
			})
		}
		out = append(out, &postpilotv1.FingerprintItemComparison{
			Item: toProtoItem(item.Item), Unknown: item.Unknown, Distance: item.Distance, Headline: item.Headline, Facets: facets,
			Verdict: toProtoVerdict(item.Verdict),
		})
	}
	return out
}

func toProtoFacetValue(unit voice.FacetUnit, number float64, terms []string) *postpilotv1.FingerprintFacetValue {
	if unit == voice.UnitText {
		return &postpilotv1.FingerprintFacetValue{Value: &postpilotv1.FingerprintFacetValue_Terms{
			Terms: &postpilotv1.FingerprintTerms{Terms: terms},
		}}
	}
	return &postpilotv1.FingerprintFacetValue{Value: &postpilotv1.FingerprintFacetValue_Number{Number: number}}
}

func toProtoItem(item voice.Item) postpilotv1.FingerprintItem {
	switch item {
	case voice.ItemEndings:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_ENDINGS
	case voice.ItemMarks:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_MARKS
	case voice.ItemEmoji:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_EMOJI
	case voice.ItemShape:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_SHAPE
	case voice.ItemOpenings:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_OPENINGS
	case voice.ItemAdverbs:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_ADVERBS
	case voice.ItemPerson:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_PERSON
	case voice.ItemHeadings:
		return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_HEADINGS
	}
	return postpilotv1.FingerprintItem_FINGERPRINT_ITEM_UNSPECIFIED
}

// toProtoVerdict maps VerdictNone, an item without a verdict, to UNSPECIFIED.
func toProtoVerdict(verdict voice.Verdict) postpilotv1.FingerprintVerdict {
	switch verdict {
	case voice.VerdictSimilar:
		return postpilotv1.FingerprintVerdict_FINGERPRINT_VERDICT_SIMILAR
	case voice.VerdictDifferent:
		return postpilotv1.FingerprintVerdict_FINGERPRINT_VERDICT_DIFFERENT
	}
	return postpilotv1.FingerprintVerdict_FINGERPRINT_VERDICT_UNSPECIFIED
}

func toProtoFacetUnit(unit voice.FacetUnit) postpilotv1.FingerprintFacetUnit {
	switch unit {
	case voice.UnitShare:
		return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_SHARE
	case voice.UnitPerHundred:
		return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_PER_HUNDRED
	case voice.UnitChars:
		return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_CHARS
	case voice.UnitSentences:
		return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_SENTENCES
	case voice.UnitText:
		return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_TEXT
	}
	return postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_UNSPECIFIED
}

// toProtoHabits locates each habit phrase by the field it sits in: a block's content, or one
// item of a LIST (ItemIndex -1 means the block's own content).
func toProtoHabits(habits []voice.HabitPhrase) []*postpilotv1.VoiceHabitPhrase {
	out := make([]*postpilotv1.VoiceHabitPhrase, 0, len(habits))
	for _, habit := range habits {
		block := uint32(habit.BlockIndex)
		field := &postpilotv1.OriginFieldLocator{Kind: postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_CONTENT, BlockIndex: &block}
		if habit.ItemIndex >= 0 {
			item := uint32(habit.ItemIndex)
			field.Kind = postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_ITEM
			field.ItemIndex = &item
		}
		out = append(out, &postpilotv1.VoiceHabitPhrase{
			Field: field, Start: uint32(habit.Start), End: uint32(habit.End), Quote: habit.Quote,
			Kind: toProtoHabitKind(habit.Kind), Term: habit.Term, VoicePerHundred: habit.VoicePerHundred,
			TextCount: uint32(habit.TextCount), Example: habit.Example, ExampleFictional: habit.ExampleFictional,
		})
	}
	return out
}

func toProtoHabitKind(kind voice.HabitKind) postpilotv1.VoiceHabitKind {
	switch kind {
	case voice.HabitEnding:
		return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_ENDING
	case voice.HabitAdverb:
		return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_ADVERB
	case voice.HabitOpening:
		return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_OPENING
	case voice.HabitClosing:
		return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_CLOSING
	case voice.HabitSignature:
		return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_SIGNATURE
	}
	return postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_UNSPECIFIED
}
