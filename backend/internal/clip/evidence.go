package clip

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/composition"
)

type ObservedEvidence struct {
	ID      string
	Source  SourceEvidence
	Segment Segment
}

// CutEvidence covers every frame with recorded observations. It never bridges
// an unobserved gap or treats a filename as an observation.
func CutEvidence(analyses []SourceAnalysis, cut Cut) ([]ObservedEvidence, bool) {
	for _, analysis := range analyses {
		if analysis.Source.ID != cut.SourceID || analysis.Source.Fingerprint != cut.Fingerprint {
			continue
		}
		end := cut.StartMS
		var out []ObservedEvidence
		for i, segment := range analysis.Segments {
			start, stop := max(segment.StartMS, cut.StartMS), min(segment.EndMS, cut.EndMS)
			if start >= stop {
				continue
			}
			if start > end {
				return nil, false
			}
			out = append(out, ObservedEvidence{ID: ObservationID(cut.SourceID, i), Source: SourceEvidence{cut.SourceID, cut.Fingerprint, start, stop}, Segment: segment})
			end = max(end, stop)
		}
		return out, end == cut.EndMS && len(out) > 0
	}
	return nil, false
}

func ObservationID(source string, index int) string { return fmt.Sprintf("%s/%d", source, index) }

type ItemBinding struct {
	GroupID, ItemID, Reason string
	Owner                   bool
}

// Name/alias IDs are optional identity hints, never required information or
// inserted text. Other authored fields can always use explicit owner bindings.
func itemAliases(item composition.Item) []string {
	var out []string
	for _, key := range []string{"name", "alias", "aliases"} {
		for _, value := range strings.Split(item.Values[key], "\n") {
			value = strings.TrimSpace(value)
			if value != "" && strings.ContainsFunc(value, unicode.IsLetter) {
				out = append(out, value)
			}
		}
	}
	return out
}

// Match complete Latin words and Korean names with an explicit particle, never
// the digits or an arbitrary substring of another supplied name.
func containsIdentity(text, name string) bool {
	text, name = strings.ToLower(text), strings.ToLower(name)
	for offset := 0; offset <= len(text)-len(name); {
		at := strings.Index(text[offset:], name)
		if at < 0 {
			return false
		}
		at += offset
		before, after := text[:at], text[at+len(name):]
		left := true
		if before != "" {
			r, _ := utf8.DecodeLastRuneInString(before)
			left = !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}
		word := strings.FieldsFunc(after, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		right := after == ""
		if !right {
			r, _ := utf8.DecodeRuneInString(after)
			right = !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}
		if !right && len(word) > 0 {
			right = slices.Contains([]string{"은", "는", "이", "가", "을", "를", "과", "와", "의", "에", "에서", "에는", "으로", "로", "랑", "도", "만", "처럼", "이라는", "입니다", "이에요", "예요", "이고", "이며"}, word[0])
		}
		if left && right {
			return true
		}
		offset = at + len(name)
	}
	return false
}

func uncertainObservation(s Segment) bool {
	text := strings.ToLower(strings.Join(append([]string{s.Event, s.Speech, s.Quality}, s.Subjects...), " "))
	for _, marker := range []string{"불확실", "추정", "확인 불가", "확인할 수 없", "아마", "maybe", "possibly", "uncertain", "cannot identify", "unidentified"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// BindCutItem ignores a writer's proposed identity. A complete owner range wins;
// otherwise every overlapping observation must name the same unique supplied
// item. Conflicts and partial owner ranges remain unassigned.
func BindCutItem(inputs CompositionInputs, evidence []ObservedEvidence, cut Cut, group string) ItemBinding {
	owned := map[string]ItemBinding{}
	partial := false
	for _, a := range inputs.Associations {
		if a.SourceID != cut.SourceID || a.Fingerprint != cut.Fingerprint || a.StartMS >= cut.EndMS || a.EndMS <= cut.StartMS {
			continue
		}
		if a.StartMS > cut.StartMS || a.EndMS < cut.EndMS {
			partial = true
			continue
		}
		for _, item := range inputs.Items[a.GroupID] {
			if item.ID == a.ItemID {
				owned[a.GroupID+"/"+a.ItemID] = ItemBinding{GroupID: a.GroupID, ItemID: a.ItemID, Owner: true}
			}
		}
	}
	if len(owned) == 1 && !partial {
		for _, binding := range owned {
			if group != "" && group != binding.GroupID {
				return ItemBinding{Reason: "item_binding_conflict"}
			}
			return binding
		}
	}
	if len(owned) > 1 || partial {
		return ItemBinding{Reason: "item_binding_conflict"}
	}
	if len(evidence) == 0 {
		return ItemBinding{Reason: "item_unassigned"}
	}
	var found ItemBinding
	for _, observation := range evidence {
		if uncertainObservation(observation.Segment) {
			return ItemBinding{Reason: "item_uncertain"}
		}
		text := strings.Join(append([]string{observation.Segment.Event, observation.Segment.Speech}, observation.Segment.Subjects...), " ")
		matches := map[string]ItemBinding{}
		for g, items := range inputs.Items {
			for _, item := range items {
				for _, name := range itemAliases(item) {
					if containsIdentity(text, name) {
						matches[g+"/"+item.ID] = ItemBinding{GroupID: g, ItemID: item.ID}
					}
				}
			}
		}
		if len(matches) != 1 {
			return ItemBinding{Reason: "item_unassigned"}
		}
		for _, match := range matches {
			if group != "" && match.GroupID != group {
				return ItemBinding{Reason: "item_unassigned"}
			}
			if found.ItemID != "" && (found.GroupID != match.GroupID || found.ItemID != match.ItemID) {
				return ItemBinding{Reason: "item_binding_conflict"}
			}
			found = match
		}
	}
	return found
}

type FactReference struct{ FieldID, GroupID, ItemID string }

func ScopedFact(doc *composition.Document, inputs CompositionInputs, ref FactReference, scope string, binding ItemBinding) (composition.Fact, bool) {
	var field *composition.Field
	for i := range doc.Fields {
		f := &doc.Fields[i]
		if f.ID == ref.FieldID && f.Group == ref.GroupID {
			field = f
			break
		}
	}
	if field == nil {
		return composition.Fact{}, false
	}
	value := ""
	if ref.GroupID == "" {
		if scope != "context" || ref.ItemID != "" {
			return composition.Fact{}, false
		}
		value = inputs.Values[ref.FieldID]
	} else {
		if scope == "context" || binding.ItemID == "" || ref.GroupID != binding.GroupID || ref.ItemID != binding.ItemID {
			return composition.Fact{}, false
		}
		for _, item := range inputs.Items[ref.GroupID] {
			if item.ID == ref.ItemID {
				value = item.Values[ref.FieldID]
				break
			}
		}
	}
	if strings.TrimSpace(value) == "" {
		return composition.Fact{}, false
	}
	return composition.Fact{FieldID: ref.FieldID, GroupID: ref.GroupID, ItemID: ref.ItemID, Value: value}, true
}

type measuredClaim struct{ Number, Unit string }

var measuredNumber = regexp.MustCompile(`(?i)([₩$€¥£]|KRW|USD|EUR|JPY|GBP)?\s*([+−-]?[0-9]+(?:[.,][0-9]+)*)(?:\s*([\p{L}%]+))?`)
var groupedNumber = regexp.MustCompile(`^[+−-]?[0-9]{1,3}(?:,[0-9]{3})+(?:\.[0-9]+)?$`)
var priceBasis = regexp.MustCompile(`(?i)(?:per|/)\s*(person|serving|night|item|hour|day|month|year)\b`)
var koreanPriceBasis = regexp.MustCompile(`[0-9]+\s*(?:인분|인|박|개|잔|시간|개월|일|회|명)|(?:인당|개당|잔당|박당|시간당|일당)`)

func normalizedUnit(value string) string {
	for _, ending := range []string{"입니다", "이에요", "예요", "이고", "이며", "으로", "부터", "까지", "은", "는", "이", "을", "에", "의"} {
		if strings.HasSuffix(value, ending) {
			value = strings.TrimSuffix(value, ending)
			break
		}
	}
	switch strings.ToLower(value) {
	case "per":
		return ""
	case "₩", "krw", "원":
		return "KRW"
	case "$", "usd", "달러":
		return "USD"
	case "€", "eur", "유로":
		return "EUR"
	case "¥", "jpy", "엔":
		return "JPY"
	case "£", "gbp":
		return "GBP"
	}
	return strings.ToLower(value)
}
func measuredClaims(text string) []measuredClaim {
	var out []measuredClaim
	for _, match := range measuredNumber.FindAllStringSubmatch(text, -1) {
		prefix, suffix := normalizedUnit(match[1]), normalizedUnit(match[3])
		unit := suffix
		if prefix != "" {
			unit = prefix
			if suffix != "" && suffix != prefix {
				unit = prefix + "/" + suffix
			}
		}
		number := strings.ReplaceAll(match[2], "−", "-")
		if groupedNumber.MatchString(number) {
			number = strings.ReplaceAll(number, ",", "")
		}
		out = append(out, measuredClaim{number, unit})
	}
	return out
}
func bases(text string) []string {
	var out []string
	for _, match := range priceBasis.FindAllStringSubmatch(text, -1) {
		out = append(out, strings.ToLower(match[1]))
	}
	for _, match := range koreanPriceBasis.FindAllString(text, -1) {
		out = append(out, strings.ReplaceAll(match, " ", ""))
	}
	return out
}

// GroundScopedText deliberately checks each numeric token against one referenced
// fact. It never concatenates digits, borrows another item's price, or treats a
// missing currency/basis as measured evidence. Fixed authored text bypasses AI
// grounding entirely.
func GroundScopedText(text string, facts []composition.Fact, inputs CompositionInputs, binding ItemBinding, scope string) string {
	for _, claim := range measuredClaims(text) {
		supported := false
		for _, fact := range facts {
			if !slices.Contains(measuredClaims(fact.Value), claim) {
				continue
			}
			allBases := true
			for _, basis := range bases(fact.Value) {
				if !slices.Contains(bases(text), basis) {
					allBases = false
				}
			}
			if allBases {
				supported = true
				break
			}
		}
		if !supported {
			return "unsupported_number_unit"
		}
	}
	for _, basis := range bases(text) {
		supported := false
		for _, fact := range facts {
			supported = supported || slices.Contains(bases(fact.Value), basis)
		}
		if !supported {
			return "unsupported_price_basis"
		}
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{"맛있", "맛없", "고소", "먹어", "먹었", "다녀왔", "써봤", "사용해보니", "느꼈", "만족", "효능", "효과", "치료", "i tried", "i loved", "i tasted", "i visited", "i ate", "i felt", "we tried", "we visited", "delicious", "tasty", "my experience", "cured", "effective"} {
		if !strings.Contains(lower, marker) {
			continue
		}
		supported := false
		for _, fact := range facts {
			// Reusing only the positive adjective from a negative experience is
			// not support. Require the complete experiential phrase in an owner
			// fact; paraphrases without checkable support are omitted.
			supported = supported || strings.TrimSpace(strings.ToLower(fact.Value)) == strings.TrimSpace(lower)
		}
		if !supported {
			return "unsupported_experience"
		}
	}
	for group, items := range inputs.Items {
		for _, item := range items {
			if scope != "context" && group == binding.GroupID && item.ID == binding.ItemID {
				continue
			}
			for _, name := range itemAliases(item) {
				if containsIdentity(text, name) {
					return "cross_item_identity"
				}
			}
		}
	}
	if scope == "context" && len(measuredClaims(text)) > 0 {
		for _, phrase := range []string{"이 메뉴", "이 음식", "이 제품", "이 상품", "this dish", "this item", "this product"} {
			if strings.Contains(lower, phrase) {
				return "context_item_claim"
			}
		}
	}
	return ""
}
