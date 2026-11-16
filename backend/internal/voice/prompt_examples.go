package voice

import "strings"

// Register is the sentence-ending register an example reply is written in (VOICE-79).
type Register string

const (
	RegisterHaeyo   Register = "haeyo"
	RegisterDa      Register = "da"
	RegisterEumseum Register = "eumseum"
	RegisterHamnida Register = "hamnida"
)

// Registers lists every register an example may take.
func Registers() []Register {
	return []Register{RegisterHaeyo, RegisterDa, RegisterEumseum, RegisterHamnida}
}

// PromptExample is one example reply under a learning-chat question (VOICE-79). Frame names the
// published review frame it keeps the shape of; it stays in Go for traceability and tests.
// Examples are code: never 학습 글, readiness or fingerprint input (VOICE-59).
type PromptExample struct {
	Register Register
	Frame    string
	Text     string
}

// exampleFrame is a published Korean review's sentence frame: an example keeps its shape,
// ending and mark habits and replaces its content. Avoid lists the source's content words,
// which no example may carry.
type exampleFrame struct {
	ID       string
	Register Register
	Source   string
	Skeleton string
	Avoid    []string
}

var exampleFrames = []exampleFrame{
	{"da-contrast-open", RegisterDa, "https://mochidal.tistory.com/201", "[평소 습관]하는 편이다. [그동안의 실망]도 있었다. 그런데 이곳은 달랐다.", []string{"네이버 방문자 후기", "모란"}},
	{"haeyo-verdict-first", RegisterHaeyo, "https://oiad.kr/article/%EC%83%81%ED%92%88-%EC%82%AC%EC%9A%A9%ED%9B%84%EA%B8%B0/4/12107/", "[구매 방식] 솔직 후기예요. [기록 범위]라 TMI지만 ㅎㅎ 결론은, [결론].", []string{"내돈내산", "오전 9시부터 6시까지"}},
	{"hamnida-problem-open", RegisterHamnida, "https://yellowit.co.kr/tag/%ed%9b%84%ea%b8%b0/", "[대상]은 [불편] 때문에 [상황]인 분들이 한 번쯤 고민하게 되는 [물건]입니다. 결론부터 말하면, [결론].", []string{"쿨매트", "에어컨", "잠을 설치는"}},
	{"haeyo-humble-open", RegisterHaeyo, "https://tofu.goorumee.com/category/%EB%82%B4%EB%8F%88%EB%82%B4%EC%82%B0%20%EC%86%94%EC%A7%81%20%EB%A6%AC%EB%B7%B0", "[못하는 사람]도 되는 [대상] 후기인데요, [겸손 한마디].", []string{"똥손", "아이라이너"}},
	{"haeyo-verdict-detail", RegisterHaeyo, "https://polle.com/kiwimix/posts/1", "[대상]이 [판정]한 곳이에요. [디테일]을 잘 맞춰주셔서 [좋은 결과]할 수 있었어요.", []string{"커피", "우유 비율"}},
	{"da-plain-verdicts", RegisterDa, "https://polle.com/solitude_dog/posts/21", "[대상] 무난하다. 그리고 생각보다 [의외].", []string{"아메리카노", "케이크"}},
	{"da-kick", RegisterDa, "https://polle.com/jennykim", "[첫 판정]인데 [반전 판정]. [포인트]가 킥!", []string{"꽈배기", "시즈닝"}},
	{"eumseum-walked-in", RegisterEumseum, "https://polle.com/jennykim", "[이유]해서 들어감. 제대로 된 [대상] 찾았다는~~", []string{"베이커리", "빵"}},
	{"da-exclaim", RegisterDa, "https://polle.com/posts/tag/%EC%9E%AC%EB%B0%A9%EB%AC%B8", "인생 [대상]을 만났다!", []string{"뇨끼"}},
	{"da-conditional-return", RegisterDa, "https://polle.com/solitude_dog/posts/21", "[좋음]이지만 [불편]이라 기회가 되면 [행동] 아님 못 [행동] 듯싶다.", []string{"위치"}},
	{"da-grateful-emoji", RegisterDa, "https://polle.com/posts/tag/%EC%9E%AC%EB%B0%A9%EB%AC%B8", "[대상]이 살짝 [아쉬움]긴 하지만 [서비스]까지 [해 주는] 데에 감사를☺️", []string{"볶음밥", "달걀"}},
	{"hamnida-nostalgic", RegisterHamnida, "https://polle.com/jennykim", "예전엔 안 그랬는데.. 여기 [대상] 진짜 [좋았는데].. [예전 모습]이 그립습니다.ㅠ", []string{"소보로빵"}},
	{"da-uses-recommend", RegisterDa, "https://mochidal.tistory.com/201", "[용도1]도, [용도2]도, [용도3]도 다 어울렸다. [조건]을 찾고 있다면 추천하고 싶은 곳이다.", []string{"카공", "성남"}},
	{"haeyo-companion-next", RegisterHaeyo, "https://polle.com/kiwimix/posts/1", "같이 간 [일행]도 [대상]이 괜찮다는 평이었어요. 다음엔 [다른 것]도 [해] 보고 싶어요 :)", []string{"코스타리카", "아포가토"}},
	{"eumseum-oneliner", RegisterEumseum, "https://polle.com/posts/tag/%EC%9E%AC%EB%B0%A9%EB%AC%B8", "넘 [판정]함 [n]번 감", []string{"냉면"}},
	{"eumseum-summary", RegisterEumseum, "https://polle.com/syh5586", "한 줄 평: [장소]에서 즐기는 [가격대] [특징] [대상]", []string{"라멘", "오니기리", "충남대"}},
	{"hamnida-condition-recommend", RegisterHamnida, "https://rentre.kr/product/1600003816 (search excerpt)", "[단점]에 아주 민감하신 분이 아니라면, [대상]에게는 무조건 추천하고 싶은 [물건]입니다.", []string{"얼음", "정수기"}},
	{"haeyo-stretched-aside", RegisterHaeyo, "https://oiad.kr/article/%EC%83%81%ED%92%88-%EC%82%AC%EC%9A%A9%ED%9B%84%EA%B8%B0/4/12107/", "[대상]이 전~~~혀 [나쁜 일] 안 해요 ([괄호 부연]).", []string{"립", "텀블러"}},
}

// Short constructors keep the table below readable.
func haeyo(frame, text string) PromptExample   { return PromptExample{RegisterHaeyo, frame, text} }
func da(frame, text string) PromptExample      { return PromptExample{RegisterDa, frame, text} }
func eumseum(frame, text string) PromptExample { return PromptExample{RegisterEumseum, frame, text} }
func hamnida(frame, text string) PromptExample { return PromptExample{RegisterHamnida, frame, text} }

// promptFamily is the key the examples are filed under: the family prefix for an everyday_*
// situation (its eight scenes share one question), otherwise the prompt key itself.
func promptFamily(key string) string {
	if !strings.HasPrefix(key, "everyday_") {
		return key
	}
	i := strings.LastIndexByte(key, '_')
	if i < 0 || i == len(key)-1 {
		return key
	}
	for _, r := range key[i+1:] {
		if r < '0' || r > '9' {
			return key
		}
	}
	return key[:i]
}

// withExamples attaches the question's example replies; the returned prompt owns its slice.
func withExamples(prompt Prompt) Prompt {
	prompt.Examples = append([]PromptExample(nil), promptExamples[promptFamily(prompt.Key)]...)
	return prompt
}

// promptExamples holds VOICE_PROMPT_EXAMPLES replies per question, each in another register.
// A family's examples stay general enough for every one of its scenes.
var promptExamples = map[string][]PromptExample{
	// starters
	"starter_day_open": {
		da("da-contrast-open", "주말이면 늦잠부터 자는 편이다. 그러다 하루가 흐지부지 끝난 날도 많았다. 그런데 오늘은 좀 달랐다."),
		haeyo("haeyo-verdict-first", "느긋한 주말 기록이에요. 아침부터 저녁까지 다 적을 거라 TMI지만 ㅎㅎ 결론은, 오늘은 집에서 푹 쉬기로 했어요."),
		eumseum("eumseum-walked-in", "날이 좋아서 일찍 나옴. 제대로 된 주말 아침 보내는 중이라는~~"),
	},
	"starter_discovery_open": {
		eumseum("eumseum-walked-in", "간판이 귀여워서 들어감. 동네에 제대로 된 소품 가게 생겼다는~~"),
		da("da-contrast-open", "산책할 땐 가게를 그냥 지나치는 편이다. 들어갔다가 금방 나온 곳도 많았다. 그런데 이곳은 달랐다."),
		hamnida("hamnida-problem-open", "동네 산책길은 볼거리가 적기 때문에 매번 같은 길만 걷는 분들이 한 번쯤 아쉬워하게 되는 코스입니다. 결론부터 말하면, 이 작은 가게 하나로 길이 달라졌습니다."),
	},
	"starter_snack": {
		da("da-plain-verdicts", "맛은 무난하다. 그리고 생각보다 고소한 맛이 오래 남는다."),
		haeyo("haeyo-verdict-detail", "기대 없이 골랐다가 만족한 간식이에요. 단맛을 과하지 않게 맞춰서 끝까지 질리지 않고 먹을 수 있었어요."),
		eumseum("eumseum-oneliner", "넘 고소함 벌써 세 번 사 먹음"),
	},
	"starter_rain": {
		da("da-contrast-open", "비 오는 날은 집에만 있는 편이다. 나갔다가 신발이 다 젖은 날도 있었다. 그런데 오늘은 우산 덕분에 좀 달랐다."),
		haeyo("haeyo-stretched-aside", "가방에 넣어 둔 우산이 전~~~혀 짐처럼 안 느껴져요 (오늘만큼은요)."),
		hamnida("hamnida-condition-recommend", "옷이 조금 젖는 것에 아주 민감하신 분이 아니라면, 빗소리를 좋아하는 분에게는 무조건 추천하고 싶은 산책입니다."),
	},
	"starter_recommend": {
		da("da-uses-recommend", "동네 공원은 아침에도, 해 질 녘에도, 비 갠 오후에도 다 어울렸다. 가볍게 기분을 바꿀 일을 찾고 있다면 추천하고 싶은 곳이다."),
		hamnida("hamnida-condition-recommend", "걷는 수고에 아주 민감하신 분이 아니라면, 주말이 심심한 분에게는 무조건 추천하고 싶은 동네 산책입니다."),
		haeyo("haeyo-companion-next", "같이 간 친구도 낮 산책이 괜찮다는 평이었어요. 다음엔 조금 먼 공원도 걸어 보고 싶어요 :)"),
	},
	"starter_inconvenience": {
		hamnida("hamnida-condition-recommend", "손잡이 모양에 아주 민감하신 분이 아니라면, 매일 차 마시는 분에게는 무조건 추천하고 싶은 컵입니다."),
		da("da-conditional-return", "모양은 예쁘지만 손잡이가 좁은 편이라 손에 익으면 계속 쓰고 아님 못 쓸 듯싶다."),
		haeyo("haeyo-verdict-first", "제가 직접 사서 쓴 솔직 후기예요. 일주일 내내 쓴 기록이라 TMI지만 ㅎㅎ 결론은, 예쁜데 손잡이가 조금 불편해요."),
	},
	"starter_surprise": {
		da("da-kick", "늘 다니던 길인데 오늘은 처음 걷는 것 같다. 텅 빈 인도가 킥!"),
		eumseum("eumseum-walked-in", "사람이 없어서 일부러 돌아감. 제대로 된 산책길 찾았다는~~"),
		haeyo("haeyo-stretched-aside", "오늘은 그 길이 전~~~혀 안 붐벼요 (평소엔 줄 서서 걷는 길이에요)."),
	},
	"starter_choice": {
		da("da-conditional-return", "새 메뉴도 궁금하지만 실패할까 봐 걱정이라 기회가 되면 먹어 보고 아님 못 먹어 볼 듯싶다."),
		haeyo("haeyo-companion-next", "같이 간 친구도 늘 먹던 메뉴가 괜찮다는 평이었어요. 다음엔 새 메뉴도 먹어 보고 싶어요 :)"),
		eumseum("eumseum-oneliner", "늘 먹던 메뉴 넘 든든함 열 번은 시킴"),
	},
	"starter_day_close": {
		hamnida("hamnida-condition-recommend", "다리가 조금 아픈 것에 아주 민감하신 분이 아니라면, 저녁 산책은 무조건 추천하고 싶은 하루의 마무리입니다."),
		haeyo("haeyo-companion-next", "같이 걸은 동생도 오늘 산책이 괜찮다는 평이었어요. 다음엔 다른 길도 걸어 보고 싶어요 :)"),
		da("da-exclaim", "인생 산책길을 만났다!"),
	},
	"starter_friend_close": {
		da("da-grateful-emoji", "오늘 글이 살짝 길긴 하지만 마지막 줄까지 읽어 주시는 데에 감사를☺️"),
		hamnida("hamnida-condition-recommend", "긴 글에 아주 민감하신 분이 아니라면, 다음 이야기도 무조건 들려드리고 싶은 마음입니다."),
		eumseum("eumseum-summary", "한 줄 평: 집 앞에서 즐기는 공짜 행복 가득한 하루"),
	},

	// legacy prompts
	"opening_greeting": {
		haeyo("haeyo-verdict-first", "평범한 하루 솔직 후기예요. 아침부터 밤까지 다 적어서 TMI지만 ㅎㅎ 결론은, 꽤 괜찮은 하루였어요."),
		da("da-contrast-open", "하루 이야기는 잘 안 하는 편이다. 막상 쓰려다 그만둔 날도 있었다. 그런데 오늘은 좀 달랐다."),
		eumseum("eumseum-walked-in", "오늘 할 얘기가 많아서 바로 씀. 제대로 된 하루 보냈다는~~"),
	},
	"opening_topic": {
		haeyo("haeyo-humble-open", "정리 못하는 사람도 되는 수납함 후기인데요, 저도 아직 잘 쓰는지는 모르겠어요."),
		da("da-exclaim", "인생 필통을 만났다!"),
		hamnida("hamnida-problem-open", "작은 소지품은 금방 흩어지기 때문에 책상이 늘 어수선한 분들이 한 번쯤 고민하게 되는 물건입니다. 결론부터 말하면, 이 작은 트레이 하나면 충분합니다."),
	},
	"opening_reason": {
		eumseum("eumseum-walked-in", "창가 화분이 예뻐서 들어감. 제대로 된 동네 가게 찾았다는~~"),
		da("da-contrast-open", "처음 보는 가게는 그냥 지나치는 편이다. 괜히 들어갔다가 어색하게 나온 적도 있었다. 그런데 이곳은 달랐다."),
		haeyo("haeyo-verdict-detail", "문 앞부터 정갈한 가게예요. 조명을 은은하게 맞춰 두셔서 부담 없이 들어가 볼 수 있었어요."),
	},
	"opening_return": {
		haeyo("haeyo-verdict-first", "오랜만에 쓰는 근황 후기예요. 그동안 있었던 일을 다 적어서 TMI지만 ㅎㅎ 결론은, 잘 지내고 있어요."),
		hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 우리 수다 진짜 자주 떨었는데.. 그때 그 저녁들이 그립습니다.ㅠ"),
		eumseum("eumseum-walked-in", "갑자기 생각나서 연락함. 제대로 된 근황 들고 왔다는~~"),
	},
	"photo_food": {
		da("da-plain-verdicts", "사진 속 국수 무난하다. 그리고 생각보다 국물이 깊다."),
		haeyo("haeyo-verdict-detail", "국물이 진하고 깔끔한 집이에요. 간을 세지 않게 맞춰주셔서 끝까지 편하게 먹을 수 있었어요."),
		eumseum("eumseum-oneliner", "이 집 칼국수 넘 시원함 벌써 세 번 감"),
	},
	"photo_space": {
		eumseum("eumseum-summary", "한 줄 평: 골목 끝에서 즐기는 조용하고 아늑한 작은 찻집"),
		da("da-uses-recommend", "혼자 책 읽기도, 둘이 수다 떨기도, 잠깐 쉬어 가기도 다 어울렸다. 조용한 자리를 찾고 있다면 추천하고 싶은 곳이다."),
		haeyo("haeyo-verdict-detail", "햇살이 잘 드는 곳이에요. 테이블 간격을 넉넉하게 맞춰 두셔서 오래 앉아 있어도 편하게 쉴 수 있었어요."),
	},
	"photo_item": {
		haeyo("haeyo-stretched-aside", "이 가방이 전~~~혀 어깨를 안 눌러요 (노트북을 넣어도 그래요)."),
		hamnida("hamnida-condition-recommend", "무게에 아주 민감하신 분이 아니라면, 매일 들고 다니는 분에게는 무조건 추천하고 싶은 가방입니다."),
		da("da-kick", "생긴 건 평범한데 쓸수록 손이 간다. 안쪽 작은 주머니가 킥!"),
	},
	"photo_scenery": {
		da("da-exclaim", "인생 노을을 만났다!"),
		hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 여기 바다 진짜 한적했는데.. 그때 조용하던 모습이 그립습니다.ㅠ"),
		haeyo("haeyo-companion-next", "같이 간 친구도 이 풍경이 괜찮다는 평이었어요. 다음엔 해 뜰 때도 와 보고 싶어요 :)"),
	},
	"photo_info": {
		haeyo("haeyo-verdict-first", "메뉴판 솔직 후기예요. 한 줄씩 다 읽어 봐서 TMI지만 ㅎㅎ 결론은, 세트로 시키는 게 제일 이득이에요."),
		hamnida("hamnida-problem-open", "이 집 메뉴판은 글씨가 빼곡하기 때문에 처음 온 분들이 한 번쯤 헤매게 되는 안내판입니다. 결론부터 말하면, 맨 위 대표 메뉴만 보셔도 됩니다."),
		da("da-plain-verdicts", "메뉴 안내 무난하다. 그리고 생각보다 크기 설명이 친절하다."),
	},
	"photo_favorite": {
		da("da-exclaim", "인생 사진을 만났다!"),
		eumseum("eumseum-oneliner", "이 사진 넘 마음에 듦 하루에 열 번 봄"),
		haeyo("haeyo-stretched-aside", "이 사진은 전~~~혀 꾸민 티가 안 나요 (그냥 걷다가 찍은 거예요)."),
	},
	"situation_first_visit": {
		eumseum("eumseum-walked-in", "음악 소리가 좋아서 들어감. 제대로 된 쉼터 찾았다는~~"),
		da("da-contrast-open", "낯선 가게에선 쭈뼛거리는 편이다. 금방 나오고 싶었던 적도 많았다. 그런데 이곳은 달랐다."),
		haeyo("haeyo-verdict-detail", "들어서자마자 마음이 놓이는 곳이에요. 음악 소리를 작게 맞춰 두셔서 천천히 둘러볼 수 있었어요."),
	},
	"situation_better": {
		da("da-kick", "평범한 과자인 줄 알았는데 계속 손이 간다. 끝에 남는 짭짤함이 킥!"),
		haeyo("haeyo-humble-open", "간식 잘 못 고르는 사람도 되는 간식 후기인데요, 이번엔 그냥 운이 좋았던 것 같아요."),
		eumseum("eumseum-oneliner", "넘 고소함 벌써 두 번 사 옴"),
	},
	"situation_worse": {
		da("da-conditional-return", "디자인은 마음에 들지만 쓰기가 조금 번거로워서 손에 익으면 계속 쓰고 아님 못 쓸 듯싶다."),
		hamnida("hamnida-condition-recommend", "작은 불편에 아주 민감하신 분이 아니라면, 디자인을 먼저 보는 분에게는 무조건 추천하고 싶은 물건입니다."),
		haeyo("haeyo-verdict-first", "큰맘 먹고 산 솔직 후기예요. 일주일 써 본 기록이라 TMI지만 ㅎㅎ 결론은, 기대만큼 편하진 않았어요."),
	},
	"situation_recommend": {
		da("da-uses-recommend", "동네 공원은 혼자도, 친구랑도, 가족이랑도 다 어울렸다. 부담 없는 주말 나들이를 찾고 있다면 추천하고 싶은 곳이다."),
		hamnida("hamnida-condition-recommend", "조금 걷는 수고에 아주 민감하신 분이 아니라면, 주말이 심심한 분에게는 무조건 추천하고 싶은 산책 코스입니다."),
		eumseum("eumseum-oneliner", "동네 공원 넘 좋음 이번 달만 세 번 감"),
	},
	"situation_waiting": {
		da("da-contrast-open", "길은 잘 찾는 편이다. 헤맨 적도 거의 없었다. 그런데 오늘은 달랐다."),
		haeyo("haeyo-stretched-aside", "지도 앱이 전~~~혀 도움이 안 돼요 (같은 골목을 두 번이나 돌았어요)."),
		hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 이 골목 진짜 단순했는데.. 길 하나뿐이던 때가 그립습니다.ㅠ"),
	},
	"situation_value": {
		da("da-conditional-return", "작은 간식이 맛있어 보이지만 가격이 부담이라 기회가 되면 사 보고 아님 못 사 볼 듯싶다."),
		haeyo("haeyo-verdict-detail", "넉넉한 쪽이 실속 있는 선택이에요. 양을 넉넉하게 맞춰 두셔서 여럿이 나눠 먹을 수 있었어요."),
		hamnida("hamnida-condition-recommend", "가격에 아주 민감하신 분이 아니라면, 단맛을 좋아하는 분에게는 무조건 추천하고 싶은 작은 간식입니다."),
	},
	"closing_greeting": {
		da("da-grateful-emoji", "이야기가 살짝 길었긴 하지만 끝까지 들어 주는 데에 감사를☺️"),
		haeyo("haeyo-companion-next", "같이 걸은 친구도 오늘 하루가 괜찮다는 평이었어요. 다음엔 더 재밌는 이야기도 들려주고 싶어요 :)"),
		eumseum("eumseum-summary", "한 줄 평: 집 근처에서 즐기는 돈 안 드는 소소한 하루"),
	},
	"closing_return": {
		da("da-conditional-return", "분위기는 좋지만 집에서 조금 멀어서 기회가 되면 또 가고 아님 못 갈 듯싶다."),
		eumseum("eumseum-oneliner", "넘 편안함 벌써 두 번 감"),
		hamnida("hamnida-condition-recommend", "조금 먼 거리에 아주 민감하신 분이 아니라면, 조용한 곳을 찾는 분에게는 무조건 추천하고 싶은 가게입니다."),
	},
	"closing_reader": {
		da("da-grateful-emoji", "내 얘기가 살짝 길었긴 하지만 바쁜데 끝까지 들어 주는 데에 감사를☺️"),
		haeyo("haeyo-companion-next", "옆에서 들어 준 친구도 오늘 이야기가 괜찮다는 평이었어요. 다음엔 제가 친구 이야기도 들어 보고 싶어요 :)"),
		hamnida("hamnida-condition-recommend", "긴 수다에 아주 민감하신 분이 아니라면, 다음 만남에도 무조건 들려드리고 싶은 이야기입니다."),
	},
	"closing_summary": {
		eumseum("eumseum-summary", "한 줄 평: 동네 한 바퀴로 즐기는 공짜 기분 전환"),
		da("da-exclaim", "인생 동네 길을 만났다!"),
		haeyo("haeyo-verdict-detail", "짧지만 알찬 산책이었어요. 걸음 속도를 천천히 맞추니 주변을 더 많이 볼 수 있었어요."),
	},

	// everyday situation families: eight scenes share one question
	"everyday_food": {
		da("da-plain-verdicts", "첫맛은 무난하다. 그리고 생각보다 뒷맛이 오래 남는다."),
		haeyo("haeyo-verdict-detail", "한입에 기분이 달라지는 맛이에요. 맛의 균형이 잘 맞아서 끝까지 편하게 먹을 수 있었어요."),
		eumseum("eumseum-oneliner", "넘 인상적임 한입 먹고 바로 또 먹음"),
	},
	"everyday_drink": {
		da("da-kick", "평범한 한 잔인데 자꾸 손이 간다. 마지막 한 모금이 킥!"),
		haeyo("haeyo-verdict-first", "한 잔 솔직 후기예요. 한 모금씩 다 적어서 TMI지만 ㅎㅎ 결론은, 한 번쯤 마셔 볼 만해요."),
		eumseum("eumseum-oneliner", "넘 궁금했음 두 모금 마셔 봄"),
	},
	"everyday_commute": {
		da("da-contrast-open", "길에선 늘 서두르는 편이다. 괜히 조급했던 날도 많았다. 그런데 오늘은 좀 달랐다."),
		haeyo("haeyo-stretched-aside", "오늘 이동은 전~~~혀 힘들지 않았어요 (평소보다 마음이 가벼웠어요)."),
		eumseum("eumseum-summary", "한 줄 평: 늘 다니던 길에서 즐기는 공짜 작은 행운"),
	},
	"everyday_shopping": {
		hamnida("hamnida-problem-open", "물건 하나도 색이나 가격 때문에 오래 망설이는 분들이 한 번쯤 고민하게 되는 순간입니다. 결론부터 말하면, 정말 쓸 물건인지 먼저 보는 게 맞습니다."),
		haeyo("haeyo-verdict-first", "고르고 고른 솔직 후기예요. 망설인 과정까지 다 적어서 TMI지만 ㅎㅎ 결론은, 마음이 가는 쪽이 정답이에요."),
		da("da-kick", "처음엔 평범해 보였는데 볼수록 마음이 간다. 오래 쓸 수 있겠다는 느낌이 킥!"),
	},
	"everyday_gift": {
		da("da-grateful-emoji", "내가 건네는 말이 살짝 서툴긴 하지만 늘 마음까지 알아봐 주는 데에 감사를☺️"),
		haeyo("haeyo-companion-next", "같이 고민해 준 동생도 이 마음이 괜찮다는 평이었어요. 다음엔 제가 먼저 더 자주 표현해 보고 싶어요 :)"),
		eumseum("eumseum-walked-in", "네 생각이 나서 바로 연락함. 제대로 된 마음 전한다는~~"),
	},
	"everyday_home": {
		da("da-plain-verdicts", "집 분위기 무난하다. 그리고 생각보다 작은 변화 하나로 기분이 달라진다."),
		haeyo("haeyo-stretched-aside", "이 작은 변화가 전~~~혀 아깝지 않아요 (들인 수고보다 기분이 훨씬 좋아졌어요)."),
		eumseum("eumseum-summary", "한 줄 평: 내 방에서 즐기는 공짜 소소한 기분 전환"),
	},
	"everyday_weekend": {
		da("da-uses-recommend", "늦잠도, 산책도, 집에서 뒹굴기도 다 어울렸다. 느긋한 하루를 찾고 있다면 추천하고 싶은 주말이다."),
		haeyo("haeyo-verdict-first", "제 주말 솔직 후기예요. 아침부터 저녁까지 다 적어서 TMI지만 ㅎㅎ 결론은, 오늘은 천천히 보낼래요."),
		eumseum("eumseum-oneliner", "넘 느긋함 이런 주말 백 번 원함"),
	},
	"everyday_weather": {
		da("da-contrast-open", "날씨에 기분이 잘 휩쓸리는 편이다. 그래서 하루를 망친 날도 있었다. 그런데 오늘은 좀 달랐다."),
		haeyo("haeyo-companion-next", "같이 걷던 친구도 오늘 날씨가 괜찮다는 평이었어요. 다음엔 이런 날 조금 더 멀리 걸어 보고 싶어요 :)"),
		eumseum("eumseum-walked-in", "하늘 보고 마음이 바뀌어서 나가 봄. 제대로 된 날씨 만났다는~~"),
	},
	"everyday_plans": {
		hamnida("hamnida-problem-open", "하루 계획은 할 일이 많기 때문에 늘 시간이 모자란 분들이 한 번쯤 고민하게 되는 문제입니다. 결론부터 말하면, 하나씩만 정해도 충분합니다."),
		da("da-conditional-return", "계획대로 하면 좋겠지만 변수가 많아서 기회가 되면 다 하고 아님 못 할 듯싶다."),
		haeyo("haeyo-humble-open", "계획 못 세우는 사람도 되는 하루 정리 후기인데요, 저도 아직 반은 즉흥이에요."),
	},
	"everyday_friends": {
		haeyo("haeyo-stretched-aside", "네 이야기는 전~~~혀 지루하지 않아요 (언제든 더 들려줘요)."),
		da("da-grateful-emoji", "요즘 서로 살짝 바쁘긴 하지만 이렇게 이야기까지 나눠 주는 데에 감사를☺️"),
		eumseum("eumseum-walked-in", "네 얘기 듣고 바로 답장함. 제대로 된 친구 뒀다는~~"),
	},
	"everyday_surprises": {
		da("da-kick", "평범한 하루인 줄 알았는데 웃음이 난다. 예상 못 한 이 순간이 킥!"),
		haeyo("haeyo-verdict-first", "오늘의 뜻밖 후기예요. 사소한 일까지 다 적어서 TMI지만 ㅎㅎ 결론은, 기분 좋은 반전이었어요."),
		eumseum("eumseum-oneliner", "넘 신기함 두 번 확인함"),
	},
	"everyday_waiting": {
		da("da-conditional-return", "기다리는 건 괜찮지만 길어지면 지루해서 금방 되면 기다리고 아님 못 기다릴 듯싶다."),
		haeyo("haeyo-humble-open", "기다림 못 참는 사람도 되는 기다리기 후기인데요, 저도 오늘은 겨우 참았어요."),
		hamnida("hamnida-condition-recommend", "몇 분 늦어지는 것에 아주 민감하신 분이 아니라면, 급하지 않은 날에는 무조건 추천하고 싶은 쉬어 가는 시간입니다."),
	},
	"everyday_choices": {
		da("da-conditional-return", "둘 다 끌리지만 하나만 고를 수 있어서 기회가 되면 둘 다 하고 아님 못 할 듯싶다."),
		haeyo("haeyo-verdict-first", "오늘 고민 솔직 후기예요. 망설인 이유까지 다 적어서 TMI지만 ㅎㅎ 결론은, 지금 마음이 더 가는 쪽으로 할래요."),
		hamnida("hamnida-problem-open", "이런 선택은 둘 다 장점이 있기 때문에 결정이 늦어지는 분들이 한 번쯤 고민하게 되는 문제입니다. 결론부터 말하면, 오늘 기분에 맞는 쪽을 고르면 됩니다."),
	},
	"everyday_comfort": {
		da("da-plain-verdicts", "지금 이 시간 무난하다. 그리고 생각보다 마음이 가볍다."),
		haeyo("haeyo-verdict-detail", "잠깐이지만 마음이 놓이는 시간이에요. 숨을 천천히 고르니 다시 힘을 낼 수 있었어요."),
		eumseum("eumseum-summary", "한 줄 평: 내 자리에서 즐기는 공짜 작은 휴식"),
	},
	"everyday_everyday_app": {
		hamnida("hamnida-problem-open", "휴대폰 앱은 기능이 많기 때문에 처음 쓰는 분들이 한 번쯤 헤매게 되는 도구입니다. 결론부터 말하면, 필요한 것 하나만 익혀도 충분합니다."),
		haeyo("haeyo-humble-open", "기계 잘 못 다루는 사람도 되는 앱 사용 후기인데요, 저도 아직 반은 감으로 써요."),
		da("da-plain-verdicts", "이 기능 무난하다. 그리고 생각보다 손이 덜 간다."),
	},
	"everyday_stories": {
		da("da-kick", "가볍게 보기 시작했는데 생각이 많아진다. 마지막 장면이 킥!"),
		haeyo("haeyo-companion-next", "같이 본 동생도 이 이야기가 괜찮다는 평이었어요. 다음엔 비슷한 이야기도 찾아보고 싶어요 :)"),
		eumseum("eumseum-oneliner", "넘 여운 남음 두 번 봄"),
	},
	"everyday_photos": {
		da("da-exclaim", "인생 한 장을 만났다!"),
		haeyo("haeyo-stretched-aside", "이 사진은 전~~~혀 애쓴 티가 안 나요 (그래서 더 마음에 들어요)."),
		hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 사진 진짜 대충 찍었는데.. 그래도 그때 그 사진들이 그립습니다.ㅠ"),
	},
	"everyday_small_tasks": {
		eumseum("eumseum-oneliner", "넘 뿌듯함 세 번 체크함"),
		da("da-contrast-open", "작은 일도 자꾸 미루는 편이다. 결국 하나도 못 한 날도 있었다. 그런데 오늘은 좀 달랐다."),
		haeyo("haeyo-humble-open", "꼼꼼하지 못한 사람도 되는 할 일 정리 후기인데요, 저도 하나 끝낸 게 다예요."),
	},
	"everyday_neighborhood": {
		da("da-uses-recommend", "아침 산책도, 저녁 바람 쐬기도, 잠깐 쉬어 가기도 다 어울렸다. 동네에서 숨 돌릴 곳을 찾고 있다면 추천하고 싶은 곳이다."),
		eumseum("eumseum-summary", "한 줄 평: 집 앞에서 즐기는 공짜 작은 산책"),
		haeyo("haeyo-verdict-detail", "생각보다 정이 가는 동네예요. 길이 잘 정돈돼 있어서 천천히 둘러볼 수 있었어요."),
	},
	"everyday_feelings": {
		da("da-plain-verdicts", "오늘 기분 무난하다. 그리고 생각보다 마음이 자주 바뀐다."),
		haeyo("haeyo-stretched-aside", "이런 기분이 전~~~혀 이상하지 않아요 (누구나 그런 날이 있잖아요)."),
		eumseum("eumseum-walked-in", "마음이 싱숭생숭해서 글 씀. 제대로 된 기분 정리 해 본다는~~"),
	},
	"everyday_memories": {
		hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 그때 그 순간 진짜 좋았는데.. 그 시절 모습이 그립습니다.ㅠ"),
		haeyo("haeyo-companion-next", "같이 있던 친구도 그때가 괜찮았다는 평이었어요. 다음엔 그 이야기도 다시 꺼내 보고 싶어요 :)"),
		da("da-contrast-open", "지난 일은 금방 잊는 편이다. 떠올려도 흐릿한 기억이 더 많았다. 그런데 이번엔 달랐다."),
	},
	"everyday_recommendations": {
		da("da-uses-recommend", "잠깐 걷기도, 가볍게 먹기도, 그냥 쉬기도 다 어울렸다. 부담 없는 방법을 찾고 있다면 추천하고 싶은 것들이다."),
		hamnida("hamnida-condition-recommend", "조금 귀찮은 것에 아주 민감하신 분이 아니라면, 지금 지친 분에게는 무조건 추천하고 싶은 방법입니다."),
		eumseum("eumseum-oneliner", "넘 좋음 나도 세 번 해 봄"),
	},
	"everyday_story_openings": {
		haeyo("haeyo-verdict-first", "오늘 하루 솔직 후기예요. 사소한 것까지 다 적어서 TMI지만 ㅎㅎ 결론은, 생각보다 좋은 날이었어요."),
		da("da-contrast-open", "평소엔 별일 없이 지나가는 편이다. 기록할 게 없던 날도 많았다. 그런데 오늘은 좀 달랐다."),
		eumseum("eumseum-walked-in", "할 얘기가 생겨서 바로 씀. 제대로 된 이야기 하나 들고 왔다는~~"),
	},
	"everyday_story_closings": {
		da("da-grateful-emoji", "오늘 이야기가 살짝 길긴 하지만 끝까지 읽어 주는 데에 감사를☺️"),
		hamnida("hamnida-condition-recommend", "조금 긴 글에 아주 민감하신 분이 아니라면, 다음 이야기도 무조건 들려드리고 싶은 하루였습니다."),
		haeyo("haeyo-companion-next", "같이 지낸 친구도 오늘 하루가 괜찮다는 평이었어요. 다음엔 또 다른 이야기도 들려주고 싶어요 :)"),
	},
}
