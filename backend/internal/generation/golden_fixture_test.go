package generation

// Fixed inputs behind the prompt golden files. They are a fixture, not a test: a change
// here rewrites what "byte-identical to today" means, so it must never be edited to make
// a failing prompt test pass.
func goldenProfile() Profile {
	return Profile{
		Styleguide:           "STYLE 스타일가이드",
		ActiveRules:          "ACTIVE 대조 규칙",
		Excerpts:             []string{"EXCERPT-1", "EXCERPT-2"},
		Rules:                "RULES 사용자 규칙",
		EndingMaxConsecutive: 2,
	}
}

func goldenObservations() []Observation {
	return []Observation{{File: "IMG_1.jpg", Scene: "바다", Mood: "잔잔함", Objects: []string{"파도"}}}
}

func goldenContent() PostContent {
	return PostContent{
		Title: "CURRENT TITLE", Summary: "CURRENT SUMMARY", Tags: []string{"태그1", "태그2"},
		Blocks: []Block{{Type: BlockText, Content: "CURRENT BODY"}, {Type: BlockImage, File: "IMG_1.jpg", Caption: "캡션"}},
	}
}
