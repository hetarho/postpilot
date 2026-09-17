package design

func ValidPace(pace string) bool { return pace == "" || pace == "steady" || pace == "rapid" }

// CaptionMotion is the motion one caption carries: the one its style declares
// (CDS-4, CDS-80), except that a rapid phrase replaces its neighbour with no
// fade and no movement whatever style it carries. A style id the set does not
// carry takes the default style's motion, because nothing else has drawn it.
func CaptionMotion(style, pace string) MotionTokens {
	if pace == "rapid" {
		return MotionTokens{}
	}
	if s, ok := LookupCaptionStyle(style); ok {
		return s.Motion
	}
	return Motion
}

func PhraseDuration(chars int) int {
	if chars <= Rapid.ShortChars {
		return Rapid.ShortMS
	}
	if chars <= Rapid.TargetChars {
		return Rapid.MediumMS
	}
	return Rapid.LongMS
}
