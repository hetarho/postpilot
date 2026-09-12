package design

func ValidPace(pace string) bool { return pace == "" || pace == "steady" || pace == "rapid" }

func CaptionMotion(pace string) MotionTokens {
	if pace == "rapid" {
		return MotionTokens{}
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
