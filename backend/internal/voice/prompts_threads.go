package voice

import "fmt"

// ExperienceKind is one of the six top-level review kinds the learning chat asks about first
// (VOICE-78, →TMPL-72).
type ExperienceKind string

const (
	ExperienceFood    ExperienceKind = "food"
	ExperienceCafe    ExperienceKind = "cafe"
	ExperienceBeauty  ExperienceKind = "beauty"
	ExperienceStay    ExperienceKind = "stay"
	ExperienceLeisure ExperienceKind = "leisure"
	ExperienceProduct ExperienceKind = "product"
)

// ExperienceKindInfo is a kind with its Korean label and the plain word a question reads when
// the owner gives the experience no name (VOICE-78).
type ExperienceKindInfo struct {
	Kind  ExperienceKind
	Label string
	Plain string
}

// ExperienceToken is where a thread question names the experience: the owner's name for it, or
// the kind's plain word. It stands at a sentence start before a comma or at a sentence end, never
// before a particle, so either reading stays grammatical (VOICE-78).
const ExperienceToken = "{experience}"

var experienceKinds = []ExperienceKindInfo{
	{ExperienceFood, "맛집", "그곳"},
	{ExperienceCafe, "카페", "그곳"},
	{ExperienceBeauty, "뷰티", "그곳"},
	{ExperienceStay, "숙소", "그곳"},
	{ExperienceLeisure, "체험·여가", "그곳"},
	{ExperienceProduct, "제품", "그 제품"},
}

// ExperienceKinds lists the six kinds in display order.
func ExperienceKinds() []ExperienceKindInfo {
	return append([]ExperienceKindInfo(nil), experienceKinds...)
}

// threadQuestion is one question of a kind's thread. A thread runs in the kind's review order —
// 계기 → 첫인상/분위기 → 핵심 → 좋았던 점 → 아쉬운 점 → 가격·양 → 추천 대상 → 다시 갈지/쓸지 —
// and needs no photo. Examples speak about the kind in general, since the owner's own place is
// unknown (VOICE-79).
type threadQuestion struct {
	Part     PromptPart
	Text     string
	Examples []PromptExample
}

// threadPrompts returns every kind's thread as catalog prompts keyed thread_<kind>_<part>_<n>.
func threadPrompts() []Prompt {
	var out []Prompt
	for _, info := range experienceKinds {
		counts := map[PromptPart]int{}
		for _, question := range experienceThreads[info.Kind] {
			counts[question.Part]++
			out = append(out, Prompt{
				Key:  fmt.Sprintf("thread_%s_%s_%d", info.Kind, question.Part, counts[question.Part]),
				Part: question.Part,
				Text: question.Text,
				Kind: info.Kind,
				// withExamples re-attaches these from promptExamples, where init files them.
				Examples: question.Examples,
			})
		}
	}
	return out
}

// init files each thread question's examples beside the situations' (prompt_examples.go).
func init() {
	for _, prompt := range threadPrompts() {
		promptExamples[prompt.Key] = prompt.Examples
	}
}

func opening(text string, examples ...PromptExample) threadQuestion {
	return threadQuestion{PartOpening, text, examples}
}

func description(text string, examples ...PromptExample) threadQuestion {
	return threadQuestion{PartDescription, text, examples}
}

func closing(text string, examples ...PromptExample) threadQuestion {
	return threadQuestion{PartClosing, text, examples}
}

var experienceThreads = map[ExperienceKind][]threadQuestion{
	ExperienceFood: {
		opening("{experience}, 어쩌다 가게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			haeyo("haeyo-verdict-first", "친구 추천으로 찾아간 솔직 후기예요. 메뉴 고른 얘기까지 다 적어서 TMI지만 ㅎㅎ 결론은, 또 가고 싶은 칼국숫집이에요."),
			da("da-contrast-open", "점심은 대충 때우는 편이다. 그러다 끼니를 아쉽게 넘긴 날도 많았다. 그런데 이곳은 달랐다."),
			eumseum("eumseum-walked-in", "줄이 길어서 궁금해서 들어감. 제대로 된 칼국숫집 찾았다는~~"),
		),
		opening("{experience}, 처음 들어섰을 때 어떤 느낌이었어요? 이웃에게 첫인상을 들려주세요.",
			da("da-kick", "겉보기엔 평범한 식당인데 들어가니 분위기가 다르다. 주방에서 나는 국물 냄새가 킥!"),
			haeyo("haeyo-verdict-detail", "들어서자마자 마음이 편해진 곳이에요. 인사를 반갑게 건네주셔서 기분 좋게 자리에 앉을 수 있었어요."),
			eumseum("eumseum-walked-in", "간판이 정겨워 보여서 들어감. 제대로 된 동네 밥집 찾았다는~~"),
		),
		description("{experience}, 안의 분위기는 어땠어요? 자리나 소리처럼 떠오르는 걸 이웃에게 소개해 주세요.",
			haeyo("haeyo-stretched-aside", "테이블 간격이 넓어서 옆 소리가 전~~~혀 신경 안 쓰여요 (점심시간에도요)."),
			da("da-plain-verdicts", "인테리어는 무난하다. 그리고 생각보다 조명이 아늑하다."),
			hamnida("hamnida-problem-open", "식당은 시끄러운 분위기 때문에 대화가 어려운 분들이 한 번쯤 고민하게 되는 곳입니다. 결론부터 말하면, 이곳은 조용히 이야기 나누기 좋았습니다."),
		),
		description("{experience}, 대표로 소개하고 싶은 메뉴는 뭐예요? 그 맛을 이웃에게 들려주세요.",
			da("da-exclaim", "인생 칼국수를 만났다!"),
			haeyo("haeyo-verdict-detail", "칼국수가 깔끔하게 맛있는 곳이에요. 국물 간을 잘 맞춰주셔서 끝까지 시원하게 먹을 수 있었어요."),
			eumseum("eumseum-oneliner", "넘 시원함 국물 세 번 리필함"),
		),
		description("{experience}, 특히 좋았던 점 하나를 이웃에게 꼽아 줄래요?",
			da("da-grateful-emoji", "면이 살짝 많긴 하지만 김치 리필까지 챙겨 주는 데에 감사를☺️"),
			haeyo("haeyo-verdict-detail", "응대가 정말 친절한 곳이에요. 덜 맵게 맞춰주셔서 아이랑도 편하게 먹을 수 있었어요."),
			eumseum("eumseum-oneliner", "넘 친절함 반찬 두 번 더 주심"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "맛은 좋지만 주차가 어려운 편이라 기회가 되면 걸어서 가고 아님 못 갈 듯싶다."),
			hamnida("hamnida-condition-recommend", "기다림에 아주 민감하신 분이 아니라면, 따끈한 국물을 좋아하는 분에게는 무조건 추천하고 싶은 식당입니다."),
			haeyo("haeyo-verdict-first", "제 돈 주고 다녀온 솔직 후기예요. 웨이팅까지 다 적어서 TMI지만 ㅎㅎ 결론은, 점심엔 조금 기다려야 해요."),
		),
		description("{experience}, 가격이나 양은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 동네에서 즐기는 만 원 안쪽 푸짐한 칼국수"),
			da("da-plain-verdicts", "가격은 무난하다. 그리고 생각보다 양이 넉넉하다."),
			hamnida("hamnida-problem-open", "외식은 오른 가격 때문에 메뉴 앞에서 망설이는 분들이 한 번쯤 고민하게 되는 일입니다. 결론부터 말하면, 이 정도 양이면 충분히 납득할 만했습니다."),
		),
		closing("{experience}, 어떤 이웃에게 추천하고 싶어요? 마지막 한마디처럼 들려주세요.",
			da("da-uses-recommend", "혼밥도, 가족 외식도, 친구 모임도 다 어울렸다. 부담 없는 한 끼를 찾고 있다면 추천하고 싶은 곳이다."),
			hamnida("hamnida-condition-recommend", "웨이팅에 아주 민감하신 분이 아니라면, 국물 요리를 좋아하는 분에게는 무조건 추천하고 싶은 맛집입니다."),
			haeyo("haeyo-companion-next", "같이 간 동생도 국물이 괜찮다는 평이었어요. 다음엔 만두도 시켜 보고 싶어요 :)"),
		),
		closing("{experience}, 다시 가고 싶은 곳인가요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 간 엄마도 반찬이 괜찮다는 평이었어요. 다음엔 수제비도 먹어 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 맛있음 벌써 두 번 감"),
			da("da-conditional-return", "맛은 좋지만 집에서 먼 편이라 기회가 되면 또 가고 아님 못 갈 듯싶다."),
		),
	},
	ExperienceCafe: {
		opening("{experience}, 어쩌다 가게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			eumseum("eumseum-walked-in", "창가 자리가 예뻐 보여서 들어감. 제대로 된 동네 카페 찾았다는~~"),
			haeyo("haeyo-verdict-first", "주말 오후에 다녀온 솔직 후기예요. 메뉴판 구경한 얘기까지 적어서 TMI지만 ㅎㅎ 결론은, 오래 머물고 싶은 카페예요."),
			da("da-contrast-open", "카페는 늘 가던 곳만 가는 편이다. 새로 가 봤다가 실망한 적도 있었다. 그런데 이곳은 달랐다."),
		),
		opening("{experience}, 문을 열고 들어갔을 때 첫인상은 어땠어요? 이웃에게 들려주세요.",
			da("da-kick", "외관은 소박한데 안은 생각보다 넓다. 통창으로 드는 햇살이 킥!"),
			haeyo("haeyo-stretched-aside", "들어가자마자 음악이 전~~~혀 시끄럽게 안 들려요 (딱 좋은 볼륨이에요)."),
			eumseum("eumseum-oneliner", "넘 아늑함 들어가자마자 두 번 감탄함"),
		),
		description("{experience}, 공간과 좌석은 어땠어요? 머물기 좋았는지 이웃에게 소개해 주세요.",
			da("da-uses-recommend", "혼자 책 읽기도, 친구와 수다도, 노트북 작업도 다 어울렸다. 오래 앉아 있을 곳을 찾고 있다면 추천하고 싶은 곳이다."),
			haeyo("haeyo-verdict-detail", "의자가 편안한 곳이에요. 테이블 높이를 잘 맞춰주셔서 오래 앉아도 허리가 편할 수 있었어요."),
			hamnida("hamnida-problem-open", "카페는 좁은 좌석 때문에 오래 머물기 어려운 분들이 한 번쯤 고민하게 되는 곳입니다. 결론부터 말하면, 이곳은 자리 간격이 넉넉했습니다."),
		),
		description("{experience}, 주문한 음료나 디저트는 어땠어요? 그 맛을 이웃에게 들려주세요.",
			da("da-plain-verdicts", "라테는 무난하다. 그리고 생각보다 디저트가 맛있다."),
			haeyo("haeyo-verdict-detail", "라테가 부드럽게 맛있는 곳이에요. 단맛을 잘 맞춰주셔서 끝까지 질리지 않게 마실 수 있었어요."),
			eumseum("eumseum-oneliner", "넘 촉촉함 스콘 두 번 시킴"),
		),
		description("{experience}, 가장 좋았던 점 하나를 이웃에게 꼽아 줄래요?",
			da("da-grateful-emoji", "자리가 살짝 좁긴 하지만 물까지 먼저 챙겨 주는 데에 감사를☺️"),
			haeyo("haeyo-stretched-aside", "사장님이 전~~~혀 눈치 안 주세요 (오래 앉아 있어도요)."),
			eumseum("eumseum-summary", "한 줄 평: 골목에서 즐기는 조용한 오후 한 잔"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "분위기는 좋지만 주차가 안 되는 편이라 기회가 되면 걸어서 가고 아님 못 갈 듯싶다."),
			hamnida("hamnida-condition-recommend", "주말 붐빔에 아주 민감하신 분이 아니라면, 디저트를 좋아하는 분에게는 무조건 추천하고 싶은 카페입니다."),
			haeyo("haeyo-verdict-first", "주말에 다녀온 솔직 후기예요. 자리 잡기까지 다 적어서 TMI지만 ㅎㅎ 결론은, 주말엔 자리가 금방 차요."),
		),
		description("{experience}, 가격은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 동네에서 즐기는 오천 원대 고소한 라테"),
			da("da-plain-verdicts", "음료 가격은 무난하다. 그리고 생각보다 디저트 크기가 크다."),
			hamnida("hamnida-problem-open", "디저트는 비싼 가격 때문에 하나만 고르는 분들이 한 번쯤 고민하게 되는 메뉴입니다. 결론부터 말하면, 이곳은 두 개 시켜도 아깝지 않았습니다."),
		),
		closing("{experience}, 어떤 이웃에게 추천하고 싶어요? 마지막 한마디처럼 들려주세요.",
			da("da-uses-recommend", "혼자 쉬기도, 데이트도, 가족 나들이도 다 어울렸다. 조용한 카페를 찾고 있다면 추천하고 싶은 곳이다."),
			hamnida("hamnida-condition-recommend", "좁은 계단에 아주 민감하신 분이 아니라면, 창가 풍경을 좋아하는 분에게는 무조건 추천하고 싶은 카페입니다."),
			haeyo("haeyo-companion-next", "같이 간 친구도 분위기가 괜찮다는 평이었어요. 다음엔 시즌 음료도 마셔 보고 싶어요 :)"),
		),
		closing("{experience}, 다시 가고 싶은 곳인가요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 간 언니도 디저트가 괜찮다는 평이었어요. 다음엔 브런치도 먹어 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 좋음 벌써 세 번 감"),
			da("da-exclaim", "인생 카페를 만났다!"),
		),
	},
	ExperienceBeauty: {
		opening("{experience}, 어쩌다 가게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			haeyo("haeyo-humble-open", "머리 손질 못 하는 사람도 되는 커트 후기인데요, 저도 아침이 편해졌어요."),
			da("da-contrast-open", "미용실은 한 곳만 다니는 편이다. 옮겼다가 후회한 적도 있었다. 그런데 이곳은 달랐다."),
			eumseum("eumseum-walked-in", "후기가 좋아서 들어감. 제대로 된 네일숍 찾았다는~~"),
		),
		opening("{experience}, 처음 들어갔을 때 매장 분위기나 깔끔함은 어땠어요?",
			haeyo("haeyo-verdict-detail", "들어서자마자 깔끔한 곳이에요. 도구를 하나하나 소독해 주셔서 마음 놓고 맡길 수 있었어요."),
			da("da-kick", "입구는 평범한데 안은 정돈이 잘 돼 있다. 은은한 향이 킥!"),
			hamnida("hamnida-problem-open", "미용실은 낯선 분위기 때문에 처음 가기 망설이는 분들이 한 번쯤 고민하게 되는 곳입니다. 결론부터 말하면, 이곳은 편하게 들어설 수 있었습니다."),
		),
		description("{experience}, 상담은 어땠어요? 원하는 걸 잘 들어줬는지 이웃에게 들려주세요.",
			haeyo("haeyo-verdict-detail", "상담이 꼼꼼한 곳이에요. 제 머리 상태를 잘 봐주셔서 원하는 길이로 자를 수 있었어요."),
			da("da-plain-verdicts", "상담 시간은 무난하다. 그리고 생각보다 질문을 많이 해 준다."),
			eumseum("eumseum-oneliner", "넘 꼼꼼함 사진 세 번 보여 드림"),
		),
		description("{experience}, 머리나 손톱, 피부 관리는 어떻게 진행됐어요? 이웃에게 차근차근 소개해 주세요.",
			haeyo("haeyo-stretched-aside", "시술하는 동안 전~~~혀 지루하지 않아요 (중간중간 설명해 주셔서요)."),
			da("da-contrast-open", "관리 받을 땐 잠드는 편이다. 그러다 결과에 놀란 날도 있었다. 그런데 이곳은 과정부터 달랐다."),
			hamnida("hamnida-problem-open", "젤네일은 손톱 손상 때문에 망설이는 분들이 한 번쯤 고민하게 되는 관리입니다. 결론부터 말하면, 이곳은 제거까지 꼼꼼히 해 주셨습니다."),
		),
		description("{experience}, 결과에서 가장 마음에 든 점은 뭐예요?",
			da("da-exclaim", "인생 단발을 만났다!"),
			haeyo("haeyo-humble-open", "드라이 못 하는 사람도 되는 펌 후기인데요, 손으로 털기만 해도 모양이 살아요."),
			eumseum("eumseum-oneliner", "넘 예쁨 거울 다섯 번 봄"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "결과는 좋지만 예약이 어려운 편이라 기회가 되면 또 가고 아님 못 갈 듯싶다."),
			hamnida("hamnida-condition-recommend", "긴 시술 시간에 아주 민감하신 분이 아니라면, 손상 모발이 고민인 분에게는 무조건 추천하고 싶은 미용실입니다."),
			haeyo("haeyo-verdict-first", "예약하고 다녀온 솔직 후기예요. 기다린 시간까지 다 적어서 TMI지만 ㅎㅎ 결론은, 시술이 생각보다 오래 걸려요."),
		),
		description("{experience}, 가격은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 동네에서 즐기는 오만 원대 꼼꼼한 펌"),
			da("da-plain-verdicts", "가격은 무난하다. 그리고 생각보다 추가 비용이 없다."),
			hamnida("hamnida-problem-open", "속눈썹 펌은 가격 차이 때문에 매장을 고르는 분들이 한 번쯤 고민하게 되는 관리입니다. 결론부터 말하면, 이 정도면 합리적인 편이었습니다."),
		),
		closing("{experience}, 어떤 이웃에게 추천하고 싶어요? 마지막 한마디처럼 들려주세요.",
			hamnida("hamnida-condition-recommend", "예약 대기에 아주 민감하신 분이 아니라면, 손이 덜 가는 머리를 원하는 분에게는 무조건 추천하고 싶은 곳입니다."),
			da("da-uses-recommend", "출근룩에도, 주말 나들이에도, 경조사에도 다 어울렸다. 손질 쉬운 머리를 찾고 있다면 추천하고 싶은 곳이다."),
			haeyo("haeyo-companion-next", "같이 간 친구도 결과가 괜찮다는 평이었어요. 다음엔 두피 관리도 받아 보고 싶어요 :)"),
		),
		closing("{experience}, 다음에도 또 맡기고 싶은가요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 간 동생도 디자인이 괜찮다는 평이었어요. 다음엔 다른 색도 해 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 만족함 벌써 두 번 예약함"),
			da("da-kick", "처음엔 반신반의했는데 지금은 단골이다. 꼼꼼한 마무리가 킥!"),
		),
	},
	ExperienceStay: {
		opening("{experience}, 어쩌다 예약하게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			da("da-contrast-open", "여행 숙소는 가격만 보고 고르는 편이다. 그러다 후회한 밤도 많았다. 그런데 이곳은 달랐다."),
			haeyo("haeyo-verdict-first", "기념일에 다녀온 솔직 후기예요. 예약부터 체크아웃까지 적어서 TMI지만 ㅎㅎ 결론은, 또 묵고 싶은 숙소예요."),
			eumseum("eumseum-walked-in", "창밖 사진이 예뻐서 들어감. 제대로 된 바닷가 숙소 찾았다는~~"),
		),
		opening("{experience}, 도착해서 체크인할 때 첫인상은 어땠어요?",
			haeyo("haeyo-verdict-detail", "체크인이 빠른 곳이에요. 도착 시간을 잘 맞춰주셔서 짐 풀고 바로 쉴 수 있었어요."),
			da("da-kick", "로비는 평범한데 객실 문을 여니 다르다. 창밖 풍경이 킥!"),
			eumseum("eumseum-oneliner", "넘 친절함 웰컴 인사 두 번 들음"),
		),
		description("{experience}, 객실이나 공간은 어땠어요? 이웃에게 분위기를 들려주세요.",
			haeyo("haeyo-stretched-aside", "방음이 잘돼서 옆방 소리가 전~~~혀 안 들려요 (주말이었는데도요)."),
			da("da-plain-verdicts", "객실 크기는 무난하다. 그리고 생각보다 침구가 포근하다."),
			hamnida("hamnida-problem-open", "펜션은 오래된 시설 때문에 예약을 망설이는 분들이 한 번쯤 고민하게 되는 숙소입니다. 결론부터 말하면, 이곳은 관리가 잘되어 있었습니다."),
		),
		description("{experience}, 부대시설이나 식사 중 꼭 소개하고 싶은 건 뭐예요?",
			da("da-exclaim", "인생 조식을 만났다!"),
			haeyo("haeyo-verdict-detail", "수영장이 깨끗한 곳이에요. 물 온도를 잘 맞춰주셔서 아이랑 오래 놀 수 있었어요."),
			eumseum("eumseum-oneliner", "넘 좋음 수영장 세 번 감"),
		),
		description("{experience}, 머무는 동안 가장 좋았던 점은 뭐예요?",
			da("da-grateful-emoji", "방이 살짝 작긴 하지만 늦은 체크아웃까지 챙겨 주는 데에 감사를☺️"),
			haeyo("haeyo-humble-open", "캠핑 못 하는 사람도 되는 글램핑 후기인데요, 장비 걱정 없이 쉬기만 했어요."),
			eumseum("eumseum-summary", "한 줄 평: 바닷가에서 즐기는 하룻밤 조용한 휴식"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "풍경은 좋지만 역에서 먼 편이라 기회가 되면 차로 가고 아님 못 갈 듯싶다."),
			hamnida("hamnida-condition-recommend", "얇은 벽에 아주 민감하신 분이 아니라면, 바다 풍경을 좋아하는 분에게는 무조건 추천하고 싶은 숙소입니다."),
			haeyo("haeyo-verdict-first", "주말에 묵은 솔직 후기예요. 주차한 얘기까지 다 적어서 TMI지만 ㅎㅎ 결론은, 주차장이 좀 좁아요."),
		),
		description("{experience}, 가격은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 바닷가에서 즐기는 십만 원대 깔끔한 오션뷰 객실"),
			da("da-plain-verdicts", "가격은 무난하다. 그리고 생각보다 조식이 알차다."),
			hamnida("hamnida-problem-open", "주말 숙소는 비싼 요금 때문에 평일로 미루는 분들이 한 번쯤 고민하게 되는 곳입니다. 결론부터 말하면, 이 정도 시설이면 값을 했습니다."),
		),
		closing("{experience}, 어떤 이웃에게 추천하고 싶어요? 마지막 한마디처럼 들려주세요.",
			da("da-uses-recommend", "가족 여행도, 커플 여행도, 혼자 쉬기도 다 어울렸다. 조용한 숙소를 찾고 있다면 추천하고 싶은 곳이다."),
			hamnida("hamnida-condition-recommend", "긴 이동 시간에 아주 민감하신 분이 아니라면, 아이와 함께하는 분에게는 무조건 추천하고 싶은 리조트입니다."),
			haeyo("haeyo-companion-next", "같이 간 남편도 객실이 괜찮다는 평이었어요. 다음엔 바비큐도 해 보고 싶어요 :)"),
		),
		closing("{experience}, 다시 묵고 싶은 곳인가요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 간 부모님도 조식이 괜찮다는 평이었어요. 다음엔 다른 계절에도 와 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 편함 벌써 두 번 묵음"),
			hamnida("hamnida-nostalgic", "예전엔 안 그랬는데.. 여기 바다 풍경 진짜 좋았는데.. 탁 트인 예전 모습이 그립습니다.ㅠ"),
		),
	},
	ExperienceLeisure: {
		opening("{experience}, 어쩌다 참여하게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			haeyo("haeyo-humble-open", "손재주 없는 사람도 되는 도자기 클래스 후기인데요, 저도 컵 하나는 완성했어요."),
			da("da-contrast-open", "주말엔 집에서 쉬는 편이다. 나가 봐도 금방 지친 날이 많았다. 그런데 이곳은 달랐다."),
			eumseum("eumseum-walked-in", "포스터가 눈에 띄어서 들어감. 제대로 된 전시 찾았다는~~"),
		),
		opening("{experience}, 도착했을 때 공간의 첫인상은 어땠어요?",
			da("da-kick", "건물은 평범한데 안은 꽤 넓다. 높은 천장이 킥!"),
			haeyo("haeyo-verdict-detail", "공방이 아늑한 곳이에요. 자리마다 도구를 잘 챙겨주셔서 바로 시작할 수 있었어요."),
			hamnida("hamnida-problem-open", "공방은 좁은 공간 때문에 단체로 가기 망설이는 분들이 한 번쯤 고민하게 되는 곳입니다. 결론부터 말하면, 이곳은 여럿이 가도 넉넉했습니다."),
		),
		description("{experience}, 어떻게 진행됐어요? 이웃에게 순서대로 들려주세요.",
			haeyo("haeyo-stretched-aside", "선생님이 전~~~혀 서두르지 않아요 (못 따라가면 다시 보여 주세요)."),
			da("da-plain-verdicts", "진행 순서는 무난하다. 그리고 생각보다 혼자 해 보는 시간이 길다."),
			hamnida("hamnida-problem-open", "원데이 클래스는 빠른 진도 때문에 처음 해 보는 분들이 한 번쯤 고민하게 되는 수업입니다. 결론부터 말하면, 이곳은 천천히 따라갈 수 있었습니다."),
		),
		description("{experience}, 가장 기억에 남는 순간이나 결과물은 뭐예요?",
			da("da-exclaim", "인생 전시를 만났다!"),
			haeyo("haeyo-verdict-detail", "결과물이 뿌듯한 수업이에요. 색을 잘 맞춰주셔서 집에 두고 쓸 수 있는 컵을 만들 수 있었어요."),
			eumseum("eumseum-oneliner", "넘 신남 마지막 코스 세 번 탐"),
		),
		description("{experience}, 특히 좋았던 점 하나를 이웃에게 꼽아 줄래요?",
			da("da-grateful-emoji", "시간이 살짝 짧긴 하지만 포장까지 꼼꼼히 해 주는 데에 감사를☺️"),
			haeyo("haeyo-humble-open", "그림 못 그리는 사람도 되는 드로잉 후기인데요, 생각보다 그럴듯하게 나왔어요."),
			eumseum("eumseum-summary", "한 줄 평: 실내에서 즐기는 두 시간짜리 알찬 체험"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "체험은 좋지만 예약이 빨리 차는 편이라 기회가 되면 또 가고 아님 못 갈 듯싶다."),
			hamnida("hamnida-condition-recommend", "북적이는 대기 줄에 아주 민감하신 분이 아니라면, 몸 쓰는 놀이를 좋아하는 분에게는 무조건 추천하고 싶은 곳입니다."),
			haeyo("haeyo-verdict-first", "주말에 다녀온 솔직 후기예요. 줄 선 시간까지 다 적어서 TMI지만 ㅎㅎ 결론은, 오전에 가야 덜 기다려요."),
		),
		description("{experience}, 가격이나 걸린 시간은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 동네에서 즐기는 삼만 원대 두 시간 클래스"),
			da("da-plain-verdicts", "가격은 무난하다. 그리고 생각보다 재료가 넉넉하다."),
			hamnida("hamnida-problem-open", "체험은 입장료 때문에 망설이는 분들이 한 번쯤 고민하게 되는 나들이입니다. 결론부터 말하면, 반나절 놀기엔 충분했습니다."),
		),
		closing("{experience}, 누구와 함께 가면 좋을 것 같아요? 마지막 한마디처럼 들려주세요.",
			da("da-uses-recommend", "아이와도, 연인과도, 친구들과도 다 어울렸다. 비 오는 날 갈 곳을 찾고 있다면 추천하고 싶은 곳이다."),
			hamnida("hamnida-condition-recommend", "손에 흙 묻는 것에 아주 민감하신 분이 아니라면, 새로운 취미를 찾는 분에게는 무조건 추천하고 싶은 클래스입니다."),
			haeyo("haeyo-companion-next", "같이 간 친구도 수업이 괜찮다는 평이었어요. 다음엔 다른 클래스도 들어 보고 싶어요 :)"),
		),
		closing("{experience}, 다시 해 보고 싶은가요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 간 조카도 체험이 괜찮다는 평이었어요. 다음엔 다른 코스도 타 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 재밌음 벌써 두 번 감"),
			da("da-kick", "처음엔 시간 때우기였는데 끝나고 나니 아쉽다. 직접 만든 결과물이 킥!"),
		),
	},
	ExperienceProduct: {
		opening("{experience}, 어쩌다 쓰게 됐어요? 이웃에게 소개하듯 첫마디를 꺼내 주세요.",
			haeyo("haeyo-verdict-first", "직접 사서 쓴 솔직 후기예요. 한 달 동안 쓴 기록이라 TMI지만 ㅎㅎ 결론은, 생각보다 손이 자주 가요."),
			hamnida("hamnida-problem-open", "무선 청소기는 무거운 무게 때문에 손목이 약한 분들이 한 번쯤 고민하게 되는 가전입니다. 결론부터 말하면, 이 제품은 가볍게 쓸 수 있었습니다."),
			da("da-contrast-open", "가전은 오래 고민하고 사는 편이다. 급하게 샀다가 후회한 적도 있었다. 그런데 이번엔 달랐다."),
		),
		opening("{experience}, 처음 받아 봤을 때 디자인이나 첫인상은 어땠어요?",
			da("da-kick", "상자는 평범한데 꺼내 보니 마감이 다르다. 손에 착 붙는 무게감이 킥!"),
			haeyo("haeyo-stretched-aside", "책상 위에 두어도 전~~~혀 튀지 않아요 (색이 차분해서요)."),
			eumseum("eumseum-oneliner", "넘 예쁨 상자 열자마자 세 번 만져 봄"),
		),
		description("{experience}, 주로 어떤 기능을 써요? 이웃에게 쉽게 소개해 주세요.",
			haeyo("haeyo-verdict-detail", "버튼이 단순한 제품이에요. 꼭 필요한 기능만 잘 맞춰서 처음 써도 헤매지 않을 수 있었어요."),
			da("da-plain-verdicts", "기본 기능은 무난하다. 그리고 생각보다 소음이 작다."),
			hamnida("hamnida-problem-open", "주방 가전은 복잡한 설정 때문에 손이 안 가는 분들이 한 번쯤 고민하게 되는 물건입니다. 결론부터 말하면, 이 제품은 버튼 두 개면 충분했습니다."),
		),
		description("{experience}, 실제로 써 보니 어땠어요? 쓰는 순간을 이웃에게 들려주세요.",
			haeyo("haeyo-humble-open", "요리 못 하는 사람도 되는 에어프라이어 후기인데요, 저도 저녁 한 끼는 거뜬해요."),
			da("da-contrast-open", "화장품은 바르고 금방 잊는 편이다. 바꿔 봐도 차이를 못 느낀 적도 많았다. 그런데 이번엔 달랐다."),
			eumseum("eumseum-oneliner", "넘 촉촉함 하루 세 번 바름"),
		),
		description("{experience}, 써 보니 특히 좋았던 점은 뭐예요?",
			da("da-exclaim", "인생 청소기를 만났다!"),
			haeyo("haeyo-stretched-aside", "하루 종일 써도 배터리가 전~~~혀 걱정 안 돼요 (이틀에 한 번 충전해요)."),
			eumseum("eumseum-summary", "한 줄 평: 원룸에서 쓰는 조용하고 가벼운 청소기"),
		),
		description("{experience}, 아쉬웠던 점도 있었나요? 이웃이 참고할 수 있게 솔직하게 들려주세요.",
			da("da-conditional-return", "성능은 좋지만 크기가 큰 편이라 기회가 되면 거실에 두고 아님 못 쓸 듯싶다."),
			hamnida("hamnida-condition-recommend", "조금 큰 소음에 아주 민감하신 분이 아니라면, 간편한 요리를 원하는 분에게는 무조건 추천하고 싶은 제품입니다."),
			haeyo("haeyo-verdict-first", "한 달 써 본 솔직 후기예요. 세척하는 과정까지 다 적어서 TMI지만 ㅎㅎ 결론은, 씻기가 조금 번거로워요."),
		),
		description("{experience}, 가격은 어땠어요? 이웃이 가늠할 수 있게 이야기해 주세요.",
			eumseum("eumseum-summary", "한 줄 평: 집에서 즐기는 십만 원대 든든한 주방 도우미"),
			da("da-plain-verdicts", "가격은 무난하다. 그리고 생각보다 구성품이 알차다."),
			hamnida("hamnida-problem-open", "육아용품은 비싼 가격 때문에 오래 비교하는 분들이 한 번쯤 고민하게 되는 물건입니다. 결론부터 말하면, 오래 쓸 수 있어 값을 했습니다."),
		),
		closing("{experience}, 어떤 이웃에게 추천하고 싶어요? 마지막 한마디처럼 들려주세요.",
			hamnida("hamnida-condition-recommend", "조금 큰 크기에 아주 민감하신 분이 아니라면, 집안일을 줄이고 싶은 분에게는 무조건 추천하고 싶은 가전입니다."),
			da("da-uses-recommend", "아침 출근 준비에도, 주말 대청소에도, 손님 오기 전에도 다 어울렸다. 가벼운 청소기를 찾고 있다면 추천하고 싶은 제품이다."),
			haeyo("haeyo-companion-next", "같이 쓰는 가족도 성능이 괜찮다는 평이었어요. 다음엔 다른 색상도 써 보고 싶어요 :)"),
		),
		closing("{experience}, 앞으로도 계속 쓸 것 같아요? 이웃에게 끝인사처럼 남겨 주세요.",
			haeyo("haeyo-companion-next", "같이 사는 동생도 사용감이 괜찮다는 평이었어요. 다음엔 큰 용량도 써 보고 싶어요 :)"),
			eumseum("eumseum-oneliner", "넘 편함 벌써 두 번 재구매함"),
			da("da-kick", "처음엔 반신반의했는데 이제는 매일 쓴다. 손이 덜 가는 편리함이 킥!"),
		),
	},
}
