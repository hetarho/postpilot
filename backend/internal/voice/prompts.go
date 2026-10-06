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
	Key     string
	Part    PromptPart
	Photo   bool
	Text    string
	Scene   string
	Hint    string
	Starter bool
}

// VOICE_PROMPT_COUNT (20): 4 openings, 12 descriptions (6 on a photo, 6 on a situation) and 4
// closings, each asking for 2~5 sentences.
var legacyPrompts = []Prompt{
	{Key: "opening_greeting", Part: PartOpening, Scene: "친구에게 오늘 하루 이야기를 들려주려고 해요.", Text: "첫인사를 하고 이야기를 시작해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "opening_topic", Part: PartOpening, Scene: "요즘 마음에 드는 작은 물건을 하나 소개하려고 해요.", Text: "어떤 물건인지 먼저 꺼내 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "opening_reason", Part: PartOpening, Scene: "산책하다가 처음 보는 가게에 잠깐 들어가기로 했어요.", Text: "왜 눈길이 갔는지 이야기의 첫 부분을 써 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "opening_return", Part: PartOpening, Scene: "한동안 연락하지 못한 친구에게 근황을 전하려고 해요.", Text: "반가운 인사로 이야기를 시작해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "photo_food", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 음식이나 음료 사진 한 장을 골라 주세요.", Text: "사진 속 맛과 느낌을 친구에게 이야기해 보세요. 1~3문장이면 충분해요."},
	{Key: "photo_space", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 가게나 공간 사진 한 장을 골라 주세요.", Text: "사진에서 보이는 분위기를 소개해 보세요. 1~3문장이면 충분해요."},
	{Key: "photo_item", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 물건 사진 한 장을 골라 주세요.", Text: "이 물건을 써 본 느낌을 이야기해 보세요. 1~3문장이면 충분해요."},
	{Key: "photo_scenery", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 풍경 사진 한 장을 골라 주세요.", Text: "이 풍경을 보고 든 마음을 이야기해 보세요. 1~3문장이면 충분해요."},
	{Key: "photo_info", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 메뉴나 안내판 사진 한 장을 골라 주세요.", Text: "눈에 띄는 정보를 친구에게 쉽게 설명해 보세요. 1~3문장이면 충분해요."},
	{Key: "photo_favorite", Part: PartDescription, Photo: true, Hint: "직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.", Scene: "직접 찍은 사진 중 마음에 드는 한 장을 골라 주세요.", Text: "이 사진이 좋은 이유를 이야기해 보세요. 1~3문장이면 충분해요."},
	{Key: "situation_first_visit", Part: PartDescription, Scene: "처음 들어간 가게에 은은한 음악이 흐르고 있어요.", Text: "문을 열었을 때 느낀 분위기를 써 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "situation_better", Part: PartDescription, Scene: "별 기대 없이 고른 간식이 생각보다 맛있어요.", Text: "한입 먹고 든 생각을 전해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "situation_worse", Part: PartDescription, Scene: "큰 기대를 하고 산 물건이 막상 써 보니 조금 불편해요.", Text: "아쉬운 점을 솔직하게 전해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "situation_recommend", Part: PartDescription, Scene: "친구가 주말에 뭘 할지 고민하고 있어요.", Text: "부담 없이 해 볼 만한 일을 하나 추천해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "situation_waiting", Part: PartDescription, Scene: "약속 장소를 찾다가 같은 골목을 두 번 돌았어요.", Text: "그때의 마음을 친구에게 이야기해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "situation_value", Part: PartDescription, Scene: "양이 넉넉한 간식과 조금 비싼 작은 간식 중 고르려 해요.", Text: "어느 쪽이 마음에 드는지 이유를 써 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "closing_greeting", Part: PartClosing, Scene: "친구에게 오늘 이야기를 다 들려줬어요.", Text: "가벼운 끝인사로 마무리해 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "closing_return", Part: PartClosing, Scene: "우연히 들른 가게가 꽤 마음에 들었어요.", Text: "다시 들르고 싶은지 마지막 한마디를 써 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "closing_reader", Part: PartClosing, Scene: "친구가 바쁜 와중에도 이야기를 끝까지 들어줬어요.", Text: "친구에게 건네고 싶은 마지막 말을 써 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
	{Key: "closing_summary", Part: PartClosing, Scene: "짧은 산책 이야기를 마무리하려고 해요.", Text: "오늘 느낀 점으로 끝맺어 보세요. 평소 말투로 1~3문장이면 충분해요.", Hint: "실제 경험이 없어도 이 상황을 가볍게 상상해 보세요."},
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
