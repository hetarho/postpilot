package clip

import (
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip/design"
)

// Grounding (CDS-42, V11). Every Arabic number and every proper noun a caption
// states must already exist in the owner's answers: a wrong price or name loses
// the advertiser at once, and the model is the only thing here that could
// invent one.
//
// Proper-noun detection is deliberately narrow — the 상호 and 메뉴 answers plus
// capitalised Latin tokens. A general Korean NER is out of scope, and every
// NUMBER is checked exactly, which is where the risk actually sits.
func Grounded(text string, answers []Answer) bool {
	if strings.TrimSpace(text) == "" {
		return true
	}
	if design.Banned(text) {
		return false
	}
	haystack := ""
	for _, a := range answers {
		haystack += " " + a.Text
	}
	for _, number := range numbers(text) {
		if !strings.Contains(digitsOnly(haystack), digitsOnly(number)) {
			return false
		}
	}
	for _, token := range latinTokens(text) {
		if !strings.Contains(strings.ToLower(haystack), strings.ToLower(token)) {
			return false
		}
	}
	return true
}

// Digit runs, with thousands separators dropped so "9,900" matches "9900".
func numbers(text string) []string {
	out, current := []string{}, strings.Builder{}
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsDigit(r) || (r == ',' && current.Len() > 0) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}
func digitsOnly(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// A capitalised Latin token is the only proper noun this check can see without
// guessing at Korean morphology.
func latinTokens(text string) []string {
	out := []string{}
	for _, field := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	}) {
		runes := []rune(field)
		if len(runes) < 2 || !unicode.IsUpper(runes[0]) || runes[0] > unicode.MaxASCII {
			continue
		}
		out = append(out, field)
	}
	return out
}

// Composition is what the compiler decided for one cut, recorded so the owner's
// correction step and the manifest can say WHY a copy looks the way it does
// rather than only what it looks like.
type Composition struct {
	Class string
	Scene string
	// Fallback is every step taken away from the first choice, comma-joined in
	// order: the compiler's own "short_text" / "extended_cut" / "dropped", and
	// the repair ladder's "contrast" (V3, after sampling), "style", "anchor" and
	// "dropped" (CDS-55). Step ② reads it to say what happened.
	Fallback string
	// Whether CDS-43's second copy was placed on this cut, so the correction
	// step can say why a cut carries two.
	Second bool
}

// Compose turns one written cut into a placed one: it classifies the sentence,
// selects the style and the anchor from the CDS tables, fits the exposure and
// drops what it cannot ground. Every decision is the design system's; nothing
// here is a judgement, which is what makes the same input render the same clip
// (CDS-7).
//
// measure returns the plate a candidate would occupy, or ok=false when the copy
// cannot be placed there at all.
func Compose(
	canvas Canvas,
	cut Cut,
	written Written,
	scene string,
	readableText bool,
	subject Region,
	placed Manifest,
	allowed []string,
	accent string,
	history design.StyleHistory,
	previousAnchor string,
	limit int,
	measure func(Caption) (Region, bool, error),
) (Cut, Composition, error) {
	out, decision := cut, Composition{Scene: design.Scene(scene)}
	// A dropped or absent copy carries nothing: no placement, no accent, no
	// keyword. The cut simply shows its footage.
	out.Copies = nil
	text := strings.TrimSpace(written.Text)
	if text == "" {
		return out, decision, nil
	}
	// Grounding first: an ungrounded sentence is not worth placing. The short
	// alternative is tried before the copy is dropped, and the plan is never
	// re-requested — the credit ceiling counted every planned call.
	if !Grounded(text, written.Answers) {
		if short := strings.TrimSpace(written.ShortText); short != "" && Grounded(short, written.Answers) {
			text, decision.Fallback = short, "short_text"
		} else {
			decision.Fallback = "dropped"
			return out, decision, nil
		}
	}
	if written.Pace == "rapid" {
		rapid, ok, err := composeRapid(cut, written, text, accent, allowed, placed, subject, readableText, previousAnchor, measure)
		if err != nil {
			return out, decision, err
		}
		if ok {
			decision.Class = design.Classify(text)
			return rapid, decision, nil
		}
		decision.Fallback = "sentence_pace"
	}
	// Exposure (CDS-41) comes before the class, because it may shorten the text
	// and the class is a property of the words: the compiler shortens, then
	// extends the cut, then drops — in that order, recording which it used.
	fitted, end, fallback := fitExposure(cut, text, written, decision.Fallback, limit)
	if fitted == "" {
		decision.Fallback = "dropped"
		return out, decision, nil
	}
	text, out.EndMS, decision.Fallback = fitted, end, fallback
	// The style's own line and character limits (CDS-20, CDS-23..26) are the
	// second thing short_text is for: a sentence past them is shortened, not
	// regenerated — the credit ceiling counted every planned call, so there is
	// no second call to make (CLIP-19, QUOTA-45).
	text, style, decision := selectFitting(text, written, allowed, history, decision)
	if text == "" {
		decision.Fallback = "dropped"
		return out, decision, nil
	}
	written.Keyword = keywordIn(text, written.Keyword)
	// Anchor: the style's own candidates, measured, then CDS-38's walk.
	rule := design.Styles[style]
	candidates, captions := []design.Candidate{}, []Caption{}
	for _, anchor := range []string{rule.Anchor, rule.AnchorAlt} {
		if anchor == "" {
			continue
		}
		c := Caption{Text: text, Anchor: anchor, Align: rule.Align, Style: style, Accent: accent, Keyword: written.Keyword}
		plate, ok, err := measure(c)
		if err != nil {
			// A measurement that FAILED is not a placement that does not fit:
			// the renderer could not shape the text at all, which is the
			// caller's problem, not a design-system fallback.
			return out, decision, err
		}
		candidates = append(candidates, design.Candidate{Anchor: anchor, Align: rule.Align, Plate: design.Region(plate), Fits: ok})
		captions = append(captions, c)
	}
	chosen := design.SelectAnchor(candidates, design.Region(subject), placed, readableText, previousAnchor)
	if chosen < 0 {
		decision.Fallback = "dropped"
		out.Copies, out.EndMS = nil, cut.EndMS
		return out, decision, nil
	}
	// The window is CDS-27's, always: the copy's own start and end stay zero so
	// CaptionWindow resolves them to cut start + 120 ms … cut end − 120 ms. The
	// model's caption times are advisory and are not carried through.
	out.Copies = []Caption{captions[chosen]}
	// CDS-43's second copy, on a long enough cut and only from words the model
	// already wrote. It never costs another call: the number is lifted out of
	// the sentence the compiler was handed, or is the short alternative itself.
	if first, second, ok := secondCopy(out, written, allowed, append(history, captions[chosen].Style), accent, measure); ok {
		// Both windows become explicit: two CDS-27 defaults would run over each
		// other, and CDS-43 admits no overlap.
		out.Copies = []Caption{first, second}
		decision.Second = true
	}
	return out, decision, nil
}

// secondCopy is CDS-43: a cut of 4 s or more whose sentence describes may also
// state the number it leads to, 120 ms after the description has left and never
// beside it. The number has to stand alone — the short alternative, or a clause
// of the sentence — in the style's own handful of characters; anything longer is
// a second sentence, which is not what CDS-43 offers.
func secondCopy(cut Cut, written Written, allowed []string, history design.StyleHistory, accent string, measure func(Caption) (Region, bool, error)) (Caption, Caption, bool) {
	first := cut.FirstCopy()
	if cut.EndMS-cut.StartMS < design.Copy.SecondMinCutMS() || design.Classify(first.Text) != design.ClassDesc {
		return Caption{}, Caption{}, false
	}
	text := numericClause(first.Text, written)
	if text == "" || !Grounded(text, written.Answers) {
		return Caption{}, Caption{}, false
	}
	style := design.SelectStyle(design.Scene(""), design.ClassNum, allowed, history, keywords(text, written.Keyword))
	if !withinStyle(text, style) {
		return Caption{}, Caption{}, false
	}
	// Both windows come out of the one CDS-27 window, split by what each
	// sentence earns and parted by the same 120 ms lead (CDS-41, CDS-43).
	lead := design.Timing.CopyLeadMS
	room := cut.EndMS - cut.StartMS - 3*lead
	a, b := MinExposureMS(first.Text), MinExposureMS(text)
	if a+b > room {
		return Caption{}, Caption{}, false
	}
	share := max(a, room*a/(a+b))
	if room-share < b {
		share = room - b
	}
	first.StartMS, first.EndMS = lead, lead+share
	rule := design.Styles[style]
	for _, anchor := range []string{rule.Anchor, rule.AnchorAlt} {
		if anchor == "" {
			continue
		}
		c := Caption{Text: text, Anchor: anchor, Align: rule.Align, Style: style, Accent: accent,
			Keyword: keywordIn(text, written.Keyword), StartMS: first.EndMS + lead, EndMS: cut.EndMS - cut.StartMS - lead}
		// A measurement failure here is not worth the whole plan: the cut keeps
		// the one copy it already has.
		if _, ok, err := measure(c); err == nil && ok {
			return first, c, true
		}
	}
	return Caption{}, Caption{}, false
}

// numericClause is the number the sentence turns on, standing on its own: the
// short alternative when that is what it already is, else the one clause of the
// sentence that states a number. Anything over the design system's own ceiling
// is a sentence rather than a number and is refused.
//
// In practice it is almost always the short alternative: CDS-39 reads a number
// FIRST, so a sentence carrying one is classified NUM and is never the
// description CDS-43 splits. The clause walk is what answers for a sentence the
// classifier reads as a description anyway.
func numericClause(text string, written Written) string {
	candidates := []string{strings.TrimSpace(written.ShortText)}
	for _, clause := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '.' || r == '·'
	}) {
		candidates = append(candidates, strings.TrimSpace(clause))
	}
	for _, candidate := range candidates {
		if candidate == "" || candidate == text || design.Chars(candidate) > design.Copy.SecondMaxChars {
			continue
		}
		if design.Classify(candidate) == design.ClassNum {
			return candidate
		}
	}
	return ""
}

// selectFitting chooses the style for the text, and shortens the text when its
// style's own limits refuse it — re-selecting the style for the shorter words,
// because a shorter sentence can be a different class.
func selectFitting(text string, written Written, allowed []string, history design.StyleHistory, decision Composition) (string, string, Composition) {
	for _, candidate := range []string{text, strings.TrimSpace(written.ShortText)} {
		if candidate == "" || (candidate != text && !Grounded(candidate, written.Answers)) {
			continue
		}
		class := design.Classify(candidate)
		style := design.SelectStyle(decision.Scene, class, allowed, history, keywords(candidate, written.Keyword))
		if withinStyle(candidate, style) {
			decision.Class = class
			if candidate != text {
				decision.Fallback = "short_text"
			}
			return candidate, style, decision
		}
	}
	return "", "", decision
}

// withinStyle is the style's own line and character limit.
func withinStyle(text, style string) bool {
	rule := design.Styles[style]
	lines := strings.Split(text, "\n")
	if len(lines) > rule.Lines {
		return false
	}
	for _, line := range lines {
		if design.Chars(line) > rule.Chars {
			return false
		}
	}
	return true
}
func keywords(text, keyword string) int {
	if keywordIn(text, keyword) == "" {
		return 0
	}
	return 1
}

// A keyword the text no longer carries is not a keyword.
func keywordIn(text, keyword string) string {
	if keyword == "" || !strings.Contains(text, keyword) {
		return ""
	}
	return keyword
}

// Written is what the model actually wrote for one cut: the words, a shorter
// alternative of the same fact, the keyword the sentence turns on, and the
// answers every number and proper noun must come from.
type Written struct {
	Pace                     string
	Text, ShortText, Keyword string
	Answers                  []Answer
}

// fitExposure applies CDS-41's order — shorten, extend, drop — and returns the
// text, the cut end and which fallback it used.
func fitExposure(cut Cut, text string, written Written, fallback string, limit int) (string, int, string) {
	// The window every copy gets (CDS-27), whatever the model asked for.
	window := func(end int) int { return end - cut.StartMS - 2*design.Timing.CopyLeadMS }
	if MinExposureMS(text) <= window(cut.EndMS) {
		return text, cut.EndMS, fallback
	}
	// ① the same fact in fewer characters.
	if short := strings.TrimSpace(written.ShortText); short != "" && short != text && Grounded(short, written.Answers) && MinExposureMS(short) <= window(cut.EndMS) {
		return short, cut.EndMS, "short_text"
	}
	// ② extend the cut end inside the source, to the minimum plus 240 ms.
	need := cut.StartMS + MinExposureMS(text) + design.Timing.SubExtendMS + 2*design.Timing.CopyLeadMS
	if need <= limit {
		return text, need, "extended_cut"
	}
	if short := strings.TrimSpace(written.ShortText); short != "" && Grounded(short, written.Answers) {
		if need := cut.StartMS + MinExposureMS(short) + design.Timing.SubExtendMS + 2*design.Timing.CopyLeadMS; need <= limit {
			return short, need, "extended_cut"
		}
	}
	// ③ drop the copy.
	return "", cut.EndMS, "dropped"
}

// Recorded appends one repair step to a composition's record, once.
func (c *Composition) Recorded(step string) {
	for _, taken := range strings.Split(c.Fallback, ",") {
		if taken == step {
			return
		}
	}
	if c.Fallback == "" {
		c.Fallback = step
		return
	}
	c.Fallback += "," + step
}
