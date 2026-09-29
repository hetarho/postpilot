package rpc

import (
	"github.com/postpilot/backend/internal/experiment"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func fromProtoSource(source postpilotv1.ExperimentSource) experiment.Source {
	switch source {
	case postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_POST:
		return experiment.SourcePost
	case postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_VOICE:
		return experiment.SourceVoice
	}
	return ""
}

func toProtoSource(source experiment.Source) postpilotv1.ExperimentSource {
	if source == experiment.SourceVoice {
		return postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_VOICE
	}
	return postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_POST
}

// The voice context's item and unit names, as the shared comparison message spells them
// (VOICE-62). An item or unit this edge does not know stays UNSPECIFIED, which the client
// refuses rather than guessing.
var protoItems = map[string]postpilotv1.FingerprintItem{
	"endings":  postpilotv1.FingerprintItem_FINGERPRINT_ITEM_ENDINGS,
	"marks":    postpilotv1.FingerprintItem_FINGERPRINT_ITEM_MARKS,
	"emoji":    postpilotv1.FingerprintItem_FINGERPRINT_ITEM_EMOJI,
	"shape":    postpilotv1.FingerprintItem_FINGERPRINT_ITEM_SHAPE,
	"openings": postpilotv1.FingerprintItem_FINGERPRINT_ITEM_OPENINGS,
	"adverbs":  postpilotv1.FingerprintItem_FINGERPRINT_ITEM_ADVERBS,
	"person":   postpilotv1.FingerprintItem_FINGERPRINT_ITEM_PERSON,
	"headings": postpilotv1.FingerprintItem_FINGERPRINT_ITEM_HEADINGS,
}

var protoUnits = map[string]postpilotv1.FingerprintFacetUnit{
	"share":       postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_SHARE,
	"per_hundred": postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_PER_HUNDRED,
	"chars":       postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_CHARS,
	"sentences":   postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_SENTENCES,
	"text":        postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_TEXT,
}

func toProtoComparisons(items []experiment.ItemComparison) []*postpilotv1.FingerprintItemComparison {
	out := make([]*postpilotv1.FingerprintItemComparison, 0, len(items))
	for _, item := range items {
		facets := make([]*postpilotv1.FingerprintFacet, 0, len(item.Facets))
		for _, facet := range item.Facets {
			facets = append(facets, &postpilotv1.FingerprintFacet{
				Key: facet.Key, Unit: protoUnits[facet.Unit],
				Voice: facetValue(facet.Unit, facet.Voice, facet.VoiceTerms),
				Text:  facetValue(facet.Unit, facet.Text, facet.TextTerms),
			})
		}
		out = append(out, &postpilotv1.FingerprintItemComparison{
			Item: protoItems[item.Item], Unknown: item.Unknown, Distance: item.Distance, Headline: item.Headline, Facets: facets,
		})
	}
	return out
}

func facetValue(unit string, number float64, terms []string) *postpilotv1.FingerprintFacetValue {
	if unit == "text" {
		return &postpilotv1.FingerprintFacetValue{Value: &postpilotv1.FingerprintFacetValue_Terms{Terms: &postpilotv1.FingerprintTerms{Terms: terms}}}
	}
	return &postpilotv1.FingerprintFacetValue{Value: &postpilotv1.FingerprintFacetValue_Number{Number: number}}
}
