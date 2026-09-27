package clip

import "strings"

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

// A keyword the text no longer carries is not a keyword.
func keywordIn(text, keyword string) string {
	if keyword == "" || !strings.Contains(text, keyword) {
		return ""
	}
	return keyword
}
