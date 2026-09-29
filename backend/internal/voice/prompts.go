package voice

// PromptPart is where in a post a prompt's answer belongs (VOICE-60). The readiness meter
// needs at least one of each (VOICE-32).
type PromptPart string

const (
	PartOpening     PromptPart = "opening"
	PartDescription PromptPart = "description"
	PartClosing     PromptPart = "closing"
)

// Parts lists the three parts in reading order.
func Parts() []PromptPart { return []PromptPart{PartOpening, PartDescription, PartClosing} }

// Prompt is one of the product's shared prompts. Key is stable; Text is product copy, Korean in
// both UI locales because a voice is Korean (LANG-14). A photo prompt is answered on a photo the
// owner picks (VOICE-60).
type Prompt struct {
	Key   string
	Part  PromptPart
	Photo bool
	Text  string
}

// VOICE_PROMPT_COUNT (20): 4 openings, 12 descriptions (6 on a photo, 6 on a situation) and 4
// closings, each asking for 2~5 sentences.
var prompts = []Prompt{
	{Key: "opening_greeting", Part: PartOpening, Text: "블로그 글을 시작할 때 쓰는 첫인사를 평소처럼 2~5문장으로 써 보세요."},
	{Key: "opening_topic", Part: PartOpening, Text: "오늘 소개할 곳이나 물건을 꺼내며 글을 시작해 보세요. 2~5문장이면 충분해요."},
	{Key: "opening_reason", Part: PartOpening, Text: "그곳에 가게 된 이유나 그 물건을 사게 된 이유로 글을 열어 보세요. 2~5문장이면 충분해요."},
	{Key: "opening_return", Part: PartOpening, Text: "오랜만에 올리는 글이라면 어떻게 시작할지 2~5문장으로 써 보세요."},
	{Key: "photo_food", Part: PartDescription, Photo: true, Text: "음식이나 음료 사진 한 장을 골라, 블로그에 쓰듯 2~5문장으로 써 보세요."},
	{Key: "photo_space", Part: PartDescription, Photo: true, Text: "가게나 공간의 분위기가 보이는 사진을 골라 2~5문장으로 소개해 보세요."},
	{Key: "photo_item", Part: PartDescription, Photo: true, Text: "최근에 산 물건 사진을 골라 써 본 느낌을 2~5문장으로 써 보세요."},
	{Key: "photo_scenery", Part: PartDescription, Photo: true, Text: "여행이나 나들이에서 찍은 풍경 사진을 골라 2~5문장으로 써 보세요."},
	{Key: "photo_info", Part: PartDescription, Photo: true, Text: "메뉴판·가격표·설명서처럼 정보가 담긴 사진을 골라, 읽는 사람에게 알려 주듯 2~5문장으로 써 보세요."},
	{Key: "photo_favorite", Part: PartDescription, Photo: true, Text: "요즘 가장 마음에 드는 사진 한 장을 골라 왜 좋은지 2~5문장으로 써 보세요."},
	{Key: "situation_first_visit", Part: PartDescription, Text: "처음 가 본 곳에 들어섰을 때를 2~5문장으로 써 보세요."},
	{Key: "situation_better", Part: PartDescription, Text: "기대했던 것보다 훨씬 좋았던 경험을 2~5문장으로 써 보세요."},
	{Key: "situation_worse", Part: PartDescription, Text: "기대에 못 미쳤던 경험을 솔직하게 2~5문장으로 써 보세요."},
	{Key: "situation_recommend", Part: PartDescription, Text: "친구에게 꼭 추천하고 싶은 것을 2~5문장으로 소개해 보세요."},
	{Key: "situation_waiting", Part: PartDescription, Text: "기다리거나 헤맸던 순간을 2~5문장으로 써 보세요."},
	{Key: "situation_value", Part: PartDescription, Text: "가격이나 양, 가성비에 대한 생각을 2~5문장으로 써 보세요."},
	{Key: "closing_greeting", Part: PartClosing, Text: "글을 마무리할 때 쓰는 끝인사를 평소처럼 2~5문장으로 써 보세요."},
	{Key: "closing_return", Part: PartClosing, Text: "다시 가고 싶은지, 다시 살 건지로 글을 마무리해 보세요. 2~5문장이면 충분해요."},
	{Key: "closing_reader", Part: PartClosing, Text: "글을 읽어 준 사람에게 건네는 마지막 말을 2~5문장으로 써 보세요."},
	{Key: "closing_summary", Part: PartClosing, Text: "오늘 글을 짧게 정리하며 2~5문장으로 마무리해 보세요."},
}

// Prompts returns the shared prompt set in its display order: openings, descriptions, closings.
func Prompts() []Prompt { return append([]Prompt(nil), prompts...) }

// PromptByKey resolves a stable key; false for a key the set does not hold.
func PromptByKey(key string) (Prompt, bool) {
	for _, prompt := range prompts {
		if prompt.Key == key {
			return prompt, true
		}
	}
	return Prompt{}, false
}
