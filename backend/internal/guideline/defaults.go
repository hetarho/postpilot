package guideline

// Kind is which writing a guideline directs (GUIDE-14): a post's (지침) or a clip's (영상 지침).
type Kind string

const (
	KindPost Kind = "post"
	KindClip Kind = "clip"
)

// Valid reports whether k is one of the two kinds.
func (k Kind) Valid() bool { return k == KindPost || k == KindClip }

// Language is the target language a 기본 지침 text is rendered in. Only a run's target picks
// a text; the UI language never reaches here, because the list carries both copies.
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

// DefaultCopy is one language's name and text of a 기본 지침.
type DefaultCopy struct {
	Name string
	Text string
}

// DefaultGuideline is one of the product's own 기본 지침 (GUIDE-16): a code constant with a
// stable ASCII key, never a row, never edited, learned or model-written. The owner can only
// switch it off for the whole account (GUIDE-43).
type DefaultGuideline struct {
	Key  string
	Kind Kind
	Ko   DefaultCopy
	En   DefaultCopy
	// KoreanTargetOnly entries reach a Korean-target run alone (GUIDE-41).
	KoreanTargetOnly bool
	// MemoriesOnly entries reach a run whose prompt carries a [기억] section alone (GEN-73): a
	// line about a source the post has none of must not be sent (MEM-21).
	MemoriesOnly bool
}

// Text is the entry's prompt text for a target language, and false when the entry does not
// reach that target at all.
func (d DefaultGuideline) Text(target Language) (string, bool) {
	switch target {
	case LanguageKorean:
		return d.Ko.Text, true
	case LanguageEnglish:
		if d.KoreanTargetOnly {
			return "", false
		}
		return d.En.Text, true
	default:
		return "", false
	}
}

// Defaults is the ordered registry of one kind's 기본 지침 — GUIDE-41 for posts and GUIDE-42
// for clips, in the product's order, which is also their injection order (GUIDE-14). The
// slice is a copy, so no caller can reorder the registry.
func Defaults(kind Kind) []DefaultGuideline {
	var source []DefaultGuideline
	switch kind {
	case KindPost:
		source = postDefaults
	case KindClip:
		source = clipDefaults
	}
	return append([]DefaultGuideline(nil), source...)
}

// DefaultFor finds one entry of a kind by its key.
func DefaultFor(kind Kind, key string) (DefaultGuideline, bool) {
	for _, d := range Defaults(kind) {
		if d.Key == key {
			return d, true
		}
	}
	return DefaultGuideline{}, false
}

func post(key, koName, koText, enName, enText string) DefaultGuideline {
	return DefaultGuideline{Key: key, Kind: KindPost, Ko: DefaultCopy{koName, koText}, En: DefaultCopy{enName, enText}}
}

func clipEntry(key, koName, koText, enName, enText string) DefaultGuideline {
	return DefaultGuideline{Key: key, Kind: KindClip, Ko: DefaultCopy{koName, koText}, En: DefaultCopy{enName, enText}}
}

// naturalKorean is the Korean naturalness baseline (GEN-17) as a 기본 지침: one text whose
// lines each carry one rule. Its markers and its 700-rune bound are pinned in the generation
// tests, which read it through the prompt.
const naturalKorean = "아래 기준은 새로 쓰거나 수정 요청으로 손대는 TEXT 본문에만 적용하세요. 제목·요약·HEADING·LIST에는 적용하지 말고, 수정에서는 요청 밖의 기존 문장을 그대로 두세요.\n대조 수사는 글 전체에서 “A가 아니라 B”, “~것이 아니라” 꼴을 합쳐 한 번만 쓰세요.\n문단을 “필요한·중요한·핵심은 …이다”, “결국 …로 이어진다”, “~하는 이유다”로 닫지 마세요. “중요한 것은 실행력이다”보다 “오늘 할 일을 바로 적고 실행하세요”처럼 사실과 동작을 직접 쓰세요.\n구체적 시점 없는 “향후·앞으로” 전망이나 내용 없는 “과제도 남아 있다”로 문단을 닫지 마세요. “~해야 한다”로 끝나는 문단은 글 전체에서 하나만 허용합니다.\n연결어미 -고/-며/-지만/-면서/-아서 바로 뒤에는 쉼표를 놓지 말고, 대부분 문장은 쉼표 없이 쓰세요.\n한 문단 안에서 짧은 문장과 긴 복문, 단문과 복문을 섞어 길이와 구조에 변화를 주세요.\n확대·강화·개선·확보·구축 같은 포괄적 동사를 되풀이하지 말고 구체적인 동작을 쓰세요. 잠식·청사진·신호탄 같은 지어낸 비유를 겹치거나 과장 형용사를 쌓지 말고, “~적 명사”가 이어지지 않게 하세요.\n메모가 요구하지 않은 수사·경구를 덧붙이지 마세요.\n위에 말투가 있으면, 말투와 충돌할 때 말투를 따르세요."

var postDefaults = []DefaultGuideline{
	post("facts", "재료에 있는 사실만",
		"메모, 사진 관찰, 템플릿 입력란, 기억, 스토리라인에 없는 구체적 사실은 쓰지 마세요. 사람과의 상호작용, 시설, 서비스, 대화, 가격처럼 확인되지 않은 내용을 지어내지 마세요. 확인할 수 없는 사실은 생략하거나 관찰된 범위 안에서만 쓰세요.",
		"Facts from the material only",
		"State no concrete fact the memo, the photo observations, the template's fields, the memories and the storyline do not carry. Do not invent interactions with people, facilities, services, conversations, or prices. Omit what you cannot confirm, or keep it within what was observed."),
	post("impressions", "감상은 내가 쓴 것만",
		"감상, 맛, 평가는 글쓴이가 메모, 템플릿 입력란, 스토리라인에 직접 적은 것만 쓰고, 적혀 있지 않은 감상을 지어내지 마세요.",
		"Impressions only as I gave them",
		"Write an impression, a taste or a verdict only as the author gave it in the memo, the template's fields or the storyline, and invent none."),
	// memory_impressions names itself the exception to impressions: guidelinePrecedence lets the
	// earlier of two conflicting 기본 지침 win, and an exception is not a conflict (GEN-73). It
	// reads the 취향: label memory retrieval puts on a preference memory.
	{
		Key: "memory_impressions", Kind: KindPost, MemoriesOnly: true,
		Ko: DefaultCopy{"기억을 통한 감상 추가", "기억에서 '취향:'으로 적힌 것은 글쓴이가 직접 준 감상으로 봅니다. 재료에 적힌 사실에 글쓴이가 감상을 적지 않았다면, 그 취향이 뜻하는 감상을 덧붙여도 됩니다. 예를 들어 매운 음식을 좋아한다는 취향이 있고 메모에 '매웠다'가 있으면 '매워서 좋았다'처럼 쓰세요. 감상을 붙일 사실은 재료에 적힌 것이어야 하고, 취향만으로 사실을 만들지 마세요. 글쓴이가 이 글에 적은 감상이 있으면 그 감상을 따르세요. 이 지침은 '감상은 내가 쓴 것만'의 예외입니다."},
		En: DefaultCopy{"Impressions from memories", "A memory marked '취향:' is a taste the author gave. Where the material states a fact and the author gave no impression of it, you may add the impression that taste implies: with a taste for spicy food and a memo saying it was spicy, write that it was spicy and good. The fact must come from the material; never make one up from a taste alone. Where the author wrote an impression for this post, follow theirs. This is the exception to 'Impressions only as I gave them'."},
	},
	post("naming", "메모의 이름으로",
		"사진 관찰은 사진을 한 장씩 따로 본 결과라 그 대상이 무엇인지 모르는 채 적혀 있습니다. 메모나 템플릿 입력란이 그 대상의 정체나 이름을 알려주면 — 관찰의 \"콘크리트 건물\"이 메모의 \"별채\"라면 — 본문과 IMAGE alt 및 caption에서 관찰의 일반적인 표현 대신 그쪽을 쓰세요. 다만 사진 관찰에 없는 것을 사진 안에 있는 것처럼 쓰지는 마세요.",
		"The memo's names",
		"A photo observation was made one photo at a time, without knowing what its subject is. When the memo or a template field gives that subject an identity or a name — the observation's \"concrete building\" is the memo's \"annex\" — use that in prose and in IMAGE alt and caption instead of the observation's generic wording. Do not write anything the photo observations do not show as being in the frame."),
	post("order", "일어난 순서대로",
		"그날 일어난 순서대로 이야기하세요. 템플릿이 있으면 템플릿의 각 자리 안에서 그 순서를 지키세요.",
		"In the order it happened",
		"Tell it in the order it happened; with a template, keep that order inside each of its places."),
	post("opening", "첫머리에 이유와 기대",
		"메모에 간 이유나 기대가 있으면 글 첫머리에서 꺼내세요.",
		"Open with why",
		"When the memo says why the author went or what they expected, open the post with it."),
	post("photo_moments", "사진은 이야기의 한 장면",
		"사진은 그 순간을 이야기하는 문장들 사이에 놓고, 사진을 설명하는 말로 문단을 시작하지 마세요. 각 문단은 앞 문단을 이어받아 넘어가세요.",
		"A photo is a moment",
		"Place a photo between the sentences about its moment, never open a paragraph by describing a photo, and let each paragraph pick up from the one before."),
	post("closing", "끝에서 한 번 정리",
		"마지막에 그날을 짧게 정리하세요. 총평이나 재방문 의사는 글쓴이가 적은 것이 있을 때만 쓰세요.",
		"Close by drawing it together",
		"Close by drawing the day together briefly; write a verdict or a will to return only if the author gave one."),
	post("no_listing", "관찰을 나열하지 않기",
		"사진 관찰은 글의 근거이자 사진을 놓을 자리를 알려주는 자료이지, 하나씩 묘사해야 할 목록이 아닙니다. 어떤 문단도 사진을 가리키며 설명할 필요가 없습니다. 조명, 벽, 천장, 집기, 공간 배치처럼 글에 아무것도 더하지 않는 시각적 묘사는 쓰지 마세요.",
		"No listing of observations",
		"The photo observations are grounding and placement material, not a list to describe one by one. No paragraph has to point at a photo. Do not write visual detail that carries nothing for the post: lighting, walls, ceilings, fixtures, the arrangement of a room."),
	post("titles", "제목 규칙",
		"제목을 정보 하나만 바꿔 넣은 긴 상투 문구로 쓰지 말고, 한 제목 안에서 같은 키워드를 두 번 이상 쓰지 마세요. 템플릿에 제목 형식이 있으면 그 형식을 따르고, 이 규칙은 형식의 <write> 안에서 직접 쓰는 부분에만 적용하세요.",
		"Title rules",
		"Do not write the title as long boilerplate in which only a single piece of information changes, and do not use any keyword twice or more within one title. When the template has a title form, follow it and apply this only to what you write inside its <write>."),
	post("tags", "태그 규칙",
		"tags는 먼저 떠오르는 단어를 그대로 쓰지 말고, 이 글의 내용에 이미 들어맞는 이름 가운데서 고르세요.",
		"Tag rule",
		"Choose the tags among labels already true of this post, not the first words that come to mind."),
	{
		Key: "natural_korean", Kind: KindPost, KoreanTargetOnly: true,
		Ko: DefaultCopy{"자연스러운 한국어 문체", naturalKorean},
		En: DefaultCopy{"Natural Korean style", "Caps stock contrasts, formulaic closers, uniform sentences, hype and piled metaphors in Korean prose; where a voice is given above, the voice outranks it."},
	},
}

// clipDefaults are the 영상 지침 기본 지침 (GUIDE-42); the clip writing calls render them (T440).
var clipDefaults = []DefaultGuideline{
	clipEntry("clip_facts", "입력한 사실만",
		"이름, 숫자, 단위, 통화, 가격은 입력한 정보나 항목 정보에 적힌 그대로만 쓰고, 여러 답을 조합해 만들지 마세요. 어느 항목의 것인지 헷갈릴 수 있으면 그 항목의 이름을 함께 쓰세요. 지시, 입력한 정보, 관찰, 스토리라인에 없는 사실은 쓰지 마세요.",
		"Entered facts only",
		"State a name, number, unit, currency or price only exactly as one entered fact or item fact states it, never assembled from several answers, and name the item it belongs to where it could be ambiguous. State no other fact the instruction, the entered facts, the observations and the storyline do not carry."),
	clipEntry("clip_impressions", "감상은 내가 쓴 것만",
		"맛, 분위기, 만족 같은 감상은 지시나 스토리라인에 사용자가 적은 것만 쓰세요.",
		"Impressions only as I gave them",
		"Write a taste, a mood or a satisfaction only as the owner gave it in the instruction or the storyline."),
	clipEntry("clip_hook", "첫 자막에 무엇을 보여줄지",
		"첫 자막은 이 영상이 무엇에 대한 것인지 한 줄로 말하세요 (예: 구리에서 찾은 숨은 오리집).",
		"Say what it is about first",
		"Let the first caption say in one line what the clip is about."),
	clipEntry("clip_continuity", "자막은 이어지는 말",
		"각 자막은 앞 자막을 이어받는 말로 쓰고, 화면에 보이는 것의 이름표처럼 쓰지 마세요. 자막은 컷을 넘어가도 됩니다.",
		"Captions that carry on",
		"Let each caption carry on from the one before rather than label what is on screen; a caption may run across cuts."),
	clipEntry("clip_order", "일어난 순서대로",
		"컷과 자막은 그날 일어난 순서를 따르세요. 템플릿이 있으면 각 구성 단계 안에서 그 순서를 지키세요.",
		"In the order it happened",
		"Let cuts and captions follow the order it happened; with a template, keep it inside each of its stages."),
	clipEntry("clip_wrap_up", "끝에서 정보 정리",
		"마지막 자막들에서 가게 이름, 위치, 가격처럼 사용자가 입력한 정보를 모아 말하세요.",
		"Wrap up the facts",
		"In the last captions, draw together what the owner entered, such as the name, the place and the price."),
	clipEntry("clip_no_repeated_promotion", "같은 홍보 문구 되풀이하지 않기",
		"같은 뜻의 홍보 문구를 두 번 쓰지 마세요.",
		"No repeated promotion",
		"Do not say the same promotional line twice."),
}
