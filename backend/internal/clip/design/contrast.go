package design

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// WCAG 2.1's contrast arithmetic, which V3 gates every pairing on (CDS-16,
// CDS-52). It lives here, beside the tokens it reads, so the verifier needs no
// dependency to answer a contrast question and the renderer and the verifier
// cannot disagree about a ratio.

// Relative is WCAG 2.1's relative luminance of an sRGB hex: each channel
// linearised, then weighted by the Rec.709 coefficients.
func Relative(hex string) (float64, bool) {
	r, g, b, ok := channels(hex)
	if !ok {
		return 0, false
	}
	return Luminance(r, g, b), true
}

// Luminance is the same formula on channels already in 0..1, for a caller that
// sampled pixels rather than read a token.
func Luminance(r, g, b float64) float64 {
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func linear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Contrast is the ratio between two colours, lighter over darker, 1..21.
func Contrast(a, b string) (float64, bool) {
	la, ok := Relative(a)
	lb, okb := Relative(b)
	if !ok || !okb {
		return 0, false
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05), true
}

// Over composites a colour at an alpha over another and returns the hex of the
// result: the effective background a text is actually read against when what
// sits under it is a translucent plate, a scrim or a card (CDS-45).
func Over(top string, alpha float64, bottom string) (string, bool) {
	tr, tg, tb, ok := channels(top)
	br, bg, bb, okb := channels(bottom)
	if !ok || !okb {
		return "", false
	}
	alpha = math.Min(1, math.Max(0, alpha))
	mix := func(t, b float64) float64 { return alpha*t + (1-alpha)*b }
	return Hex(mix(tr, br), mix(tg, bg), mix(tb, bb)), true
}

// Hex renders channels in 0..1 as an sRGB hex.
func Hex(r, g, b float64) string {
	byteOf := func(c float64) int { return int(math.Round(math.Min(1, math.Max(0, c)) * 255)) }
	return fmt.Sprintf("#%02X%02X%02X", byteOf(r), byteOf(g), byteOf(b))
}

func channels(hex string) (float64, float64, float64, bool) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	out := [3]float64{}
	for i := range out {
		v, err := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		if err != nil {
			return 0, 0, 0, false
		}
		out[i] = float64(v) / 255
	}
	return out[0], out[1], out[2], true
}
