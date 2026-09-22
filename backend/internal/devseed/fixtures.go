package devseed

import (
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// Password is what every seeded account logs in with. One shared value, and an obviously
// fake one, because these accounts exist to be logged into by whoever is looking at the
// screen — a per-account secret would have to be read out of the command's output every
// time, and a plausible-looking password invites being reused somewhere it matters.
const Password = "seed-only"

// Statuses a seeded article is written in. They are devseed's own spellings, mapped to the
// post context's at the adapter — this package describes an installation, and importing
// the drafting context's constants would make the fixture depend on its vocabulary.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusFinalized = "finalized"
)

// Account is one seeded login and the shape of what sits behind it.
//
// The post counts are given per status rather than as a total with a rule for splitting
// them, because the split IS the fixture: whoever changes these numbers is deciding what
// the list screen looks like, and a total plus a modulo hides that decision.
type Account struct {
	LoginID string
	Plan    plan.Plan
	// Drafts have never been generated, Reviews carry a machine baseline awaiting
	// confirmation, and Finalized ones have been confirmed.
	Drafts, Reviews, Finalized int
}

// Posts is the account's total.
func (a Account) Posts() int { return a.Drafts + a.Reviews + a.Finalized }

// Fixtures is the installation a seed produces: five accounts that differ in the two ways
// worth differing in, the plan and how much work sits behind them.
//
// The spread is chosen so that every state a screen has to handle is reachable without
// creating anything by hand. `free` is empty, which is the state every new account
// opens in and the one most easily broken by a change that assumes at least one row.
// `base` holds so few posts that a list, a count and a pagination control are all
// trivially checkable by eye. `master` holds enough that a long list, its scrolling
// and its ordering are exercised, and it sits on the plan with no credit ceiling so the
// generation screens can be driven without the balance running out mid-review.
var Fixtures = []Account{
	{LoginID: "free", Plan: plan.Free, Drafts: 0, Reviews: 0, Finalized: 0},
	{LoginID: "base", Plan: plan.Basic, Drafts: 2, Reviews: 1, Finalized: 0},
	{LoginID: "pro", Plan: plan.Pro, Drafts: 3, Reviews: 2, Finalized: 3},
	{LoginID: "max", Plan: plan.Max, Drafts: 4, Reviews: 3, Finalized: 7},
	{LoginID: "master", Plan: plan.Master, Drafts: 5, Reviews: 4, Finalized: 14},
}

// Article is one seeded post, complete: what it was created as, and — unless it is a draft
// that was never generated — the content it holds.
type Article struct {
	UserID   string
	VoiceID  string
	Title    string
	Memo     string
	Status   string
	Language string
	// CreatedAt doubles as the post's updated_at. Seeded posts are spread backwards in
	// time so the list has a real ordering to sort and group by rather than five rows
	// stamped the same second.
	CreatedAt time.Time
	// Content is nil for a draft and set for a review or a finalized post, which is the
	// distinction the product itself draws: NULL content means "never generated", and it
	// is what the editor reads to decide whether there is anything to confirm.
	Content *Content
}

// Content mirrors what a generation run produces, in devseed's own types.
type Content struct {
	Title   string
	Summary string
	Tags    []string
	Blocks  []Block
}

// Block is one canonical content block ([I2]: the post is a block array and every export
// derives from it). Only the text kinds appear in a fixture — an image block must name an
// attached photo, and the seed uploads no bytes to object storage.
type Block struct {
	Type    string
	Content string
	Level   int32
	Items   []string
}

// Block types a fixture uses, in the LLM/protojson spelling the drafting context stores.
const (
	BlockText    = "TEXT"
	BlockHeading = "HEADING"
	BlockList    = "LIST"
)

// language is what every seeded post targets. Korean rather than a mix: the two-language
// half of the product is worth a fixture of its own eventually, and a seed that split its
// posts across both would make every list screen a language test nobody asked for.
const language = "ko"

// topic is the raw material one article is built from. They are ordinary Korean posts about
// ordinary days, which is what this product is for — lorem ipsum would make every screen
// look correct at a glance and hide the line breaks, lengths and word wraps that real text
// finds.
type topic struct {
	title    string
	memo     string
	summary  string
	tags     []string
	heading  string
	body     [2]string
	takeaway []string
}

var topics = []topic{
	{
		title:    "제주 올레길 7코스",
		memo:     "외돌개에서 출발, 바다 계속 왼쪽. 점심은 보말칼국수",
		summary:  "외돌개에서 월평까지 걸으며 본 바다와 중간에 들른 국숫집 이야기입니다.",
		tags:     []string{"제주", "올레길", "걷기여행"},
		heading:  "바다를 왼쪽에 두고 걷는 길",
		body:     [2]string{"외돌개 주차장에서 시작해 한 시간쯤 걸으면 길이 완전히 바다 쪽으로 붙습니다. 파도 소리가 계속 왼쪽에서 들리는데, 그게 생각보다 오래 이어져서 걷는 리듬이 저절로 잡혔습니다.", "중간에 들른 국숫집에서 보말칼국수를 먹었습니다. 국물이 진해서 남은 절반 구간을 걸을 힘이 확실히 붙었습니다."},
		takeaway: []string{"편한 신발은 선택이 아니라 필수입니다", "물은 1리터 이상 챙기는 편이 좋습니다", "정방폭포 쪽 계단 구간은 생각보다 깁니다"},
	},
	{
		title:    "집에서 만든 감바스",
		memo:     "새우 300g, 마늘 한 통, 올리브유 넉넉히. 빵은 따로 구움",
		summary:  "마늘을 태우지 않는 불 조절이 감바스의 거의 전부라는 것을 알게 된 기록입니다.",
		tags:     []string{"요리", "감바스", "집밥"},
		heading:  "마늘을 태우지 않는 것이 전부였다",
		body:     [2]string{"올리브유를 팬에 넉넉히 두르고 마늘을 약불에서 아주 천천히 익혔습니다. 급하게 불을 올리면 마늘이 먼저 타서 기름 전체가 쓴맛이 되는데, 그걸 두 번 겪고 나서야 약불을 지키게 됐습니다.", "새우는 마지막에 넣어 3분만 익혔습니다. 색이 완전히 변한 뒤에도 팬에 두면 금방 질겨지기 때문에, 조금 덜 익은 듯할 때 불을 끄는 편이 낫습니다."},
		takeaway: []string{"마늘은 약불에서 10분 이상", "새우는 넣고 3분이면 충분합니다", "빵은 팬 기름에 따로 굽습니다"},
	},
	{
		title:    "주말 서점 나들이",
		memo:     "합정 쪽 독립서점 세 곳. 시집 두 권 샀다",
		summary:  "합정과 상수 사이의 작은 서점 세 곳을 하루에 돌아본 기록입니다.",
		tags:     []string{"서점", "합정", "책"},
		heading:  "작은 서점이 고르는 방식",
		body:     [2]string{"세 곳 모두 규모는 작았는데, 진열된 책의 결이 완전히 달랐습니다. 한 곳은 번역 시집이 절반이었고, 다른 곳은 지역에서 나온 독립출판물만 모아 두었습니다.", "결국 시집 두 권을 샀습니다. 큰 서점에서라면 검색해서 집어 왔을 책인데, 여기서는 옆에 놓인 책 때문에 집었다는 점이 달랐습니다."},
		takeaway: []string{"세 곳 모두 도보 20분 안에 있습니다", "일요일에는 한 곳이 문을 닫습니다", "현금만 받는 곳이 아직 있습니다"},
	},
	{
		title:    "처음 해본 실내 클라이밍",
		memo:     "2시간, 초급 벽만. 손가락이 제일 먼저 지쳤다",
		summary:  "팔 힘이 아니라 발 위치가 중요하다는 말을 몸으로 이해하게 된 첫 수업입니다.",
		tags:     []string{"클라이밍", "운동", "첫경험"},
		heading:  "팔이 아니라 발로 오르는 운동",
		body:     [2]string{"시작하기 전에는 팔 힘 운동이라고만 생각했습니다. 그런데 강사가 계속 발 위치를 고쳐 줬고, 발을 제대로 올린 뒤에는 같은 구간이 훨씬 쉬워졌습니다.", "두 시간 만에 손가락이 먼저 풀려서 더 오를 수 없었습니다. 다음에는 한 시간만 하고 나오기로 했습니다."},
		takeaway: []string{"첫날은 한 시간이 적당합니다", "암벽화는 대여로 충분합니다", "손가락 스트레칭을 먼저 합니다"},
	},
	{
		title:    "오래된 필름 카메라 정비",
		memo:     "라이트씰 교체, 셔터막 확인. 부품은 온라인 주문",
		summary:  "삼십 년 된 카메라의 라이트씰을 직접 갈아 끼우면서 배운 것들입니다.",
		tags:     []string{"필름카메라", "수리", "취미"},
		heading:  "삼십 년 된 스펀지를 걷어내는 일",
		body:     [2]string{"뒷문을 열자 라이트씰이 이미 가루가 되어 있었습니다. 면봉과 알코올로 걷어내는 데만 한 시간 반이 걸렸고, 이게 작업의 대부분이었습니다.", "새 씰을 붙이고 시험 촬영을 한 롤 돌렸습니다. 빛이 새던 오른쪽 아래가 깨끗해져서, 이 카메라를 다시 들고 나갈 수 있게 됐습니다."},
		takeaway: []string{"걷어내는 시간이 붙이는 시간의 세 배입니다", "부품 값은 만 원이 안 됩니다", "시험 촬영 한 롤은 반드시 필요합니다"},
	},
	{
		title:    "동네 목욕탕이 문을 닫았다",
		memo:     "40년 영업. 마지막 주에 다녀왔다",
		summary:  "사십 년 동안 영업한 동네 목욕탕의 마지막 주에 다녀온 이야기입니다.",
		tags:     []string{"동네", "기록", "목욕탕"},
		heading:  "마지막 주의 탈의실",
		body:     [2]string{"입구에 붙은 안내문은 A4 한 장이었습니다. 사십 년 동안 고맙다는 두 문장이 전부였는데, 그 앞에서 사진을 찍는 사람이 저 말고도 셋 있었습니다.", "안은 평소보다 붐볐습니다. 다들 오래 앉아 있었고, 나가면서 카운터에 인사를 하고 갔습니다."},
		takeaway: []string{"가장 가까운 목욕탕은 이제 지하철 두 정거장입니다", "폐업 안내는 2주 전에 붙었습니다"},
	},
	{
		title:    "작업용 책상 정리",
		memo:     "모니터 암 설치, 케이블 트레이. 서랍 하나 비웠다",
		summary:  "책상에서 물건을 덜어내는 쪽이 새로 사는 쪽보다 효과가 컸던 기록입니다.",
		tags:     []string{"책상", "정리", "작업환경"},
		heading:  "덜어내는 쪽이 효과가 컸다",
		body:     [2]string{"모니터 암과 케이블 트레이를 달아 책상 위 선을 모두 아래로 내렸습니다. 그런데 실제로 체감이 컸던 건 서랍 하나를 완전히 비운 일이었습니다.", "쓰지 않는 케이블과 다 쓴 펜을 버리고 나니, 늘 손이 닿는 자리에 자주 쓰는 것만 남았습니다."},
		takeaway: []string{"케이블 트레이가 모니터 암보다 저렴합니다", "서랍은 한 번에 하나만 비웁니다"},
	},
	{
		title:    "새벽 수영 한 달",
		memo:     "주 3회 6시. 25m 쉬지 않고 가는 게 목표였다",
		summary:  "한 달 동안 새벽 수영을 다니며 호흡이 먼저 늘었던 과정입니다.",
		tags:     []string{"수영", "운동", "새벽"},
		heading:  "속도보다 호흡이 먼저 늘었다",
		body:     [2]string{"첫 주에는 25미터를 한 번에 가지 못했습니다. 팔 힘이 부족한 게 아니라 중간에 호흡이 흐트러져서 멈추게 되는 쪽이었습니다.", "3주째부터 호흡이 일정해지자 같은 거리가 갑자기 편해졌습니다. 기록은 거의 그대로였는데 끝까지 갈 수 있게 됐습니다."},
		takeaway: []string{"주 3회가 주 5회보다 오래갑니다", "6시 타임이 가장 한가합니다"},
	},
	{
		title:    "텃밭 상추 첫 수확",
		memo:     "베란다 화분 네 개. 4주 걸렸다",
		summary:  "베란다 화분에서 상추를 처음 수확하기까지 4주 동안의 기록입니다.",
		tags:     []string{"텃밭", "베란다", "상추"},
		heading:  "물을 덜 주는 게 어려웠다",
		body:     [2]string{"씨를 뿌리고 4주 만에 첫 잎을 땄습니다. 가장 어려웠던 건 물을 덜 주는 일이었는데, 겉흙이 말라 보여도 속은 젖어 있는 경우가 많았습니다.", "한 화분은 물을 너무 줘서 뿌리가 상했습니다. 나머지 세 개는 이틀에 한 번으로 줄이고 나서 잘 자랐습니다."},
		takeaway: []string{"이틀에 한 번이면 충분합니다", "화분은 깊이보다 너비가 중요합니다"},
	},
	{
		title:    "중고로 산 자전거 첫 라이딩",
		memo:     "한강 20km. 안장 높이 두 번 조정",
		summary:  "중고 자전거를 받아 한강을 20킬로미터 달리며 안장 높이를 맞춘 하루입니다.",
		tags:     []string{"자전거", "한강", "라이딩"},
		heading:  "안장 높이 2센티가 전부였다",
		body:     [2]string{"처음 10킬로미터는 무릎이 계속 아팠습니다. 쉬는 곳에서 안장을 2센티 올리고 나니 통증이 사라졌는데, 같은 자전거가 다른 자전거처럼 느껴졌습니다.", "돌아오는 길에는 속도를 올려 봤습니다. 중고로 산 것치고는 변속도 부드러워서 당분간 더 탈 수 있을 것 같습니다."},
		takeaway: []string{"안장 높이는 출발 전에 맞춥니다", "20km면 두 번은 쉬는 편이 좋습니다"},
	},
}

// Articles builds the account's posts: the drafts first, then the reviews, then the
// finalized ones, dated backwards from now so the list has a stable, believable ordering.
//
// The topics cycle, so an account with more posts than there are topics repeats them with
// a suffix. Repeating is on purpose rather than a limitation — a list screen has to survive
// similar titles, and inventing sixty distinct posts would put more prose in this file than
// code.
func (a Account) Articles(voiceID string, now time.Time) []Article {
	articles := make([]Article, 0, a.Posts())
	index := 0
	// Each account starts its own run at a different point in the topic list, so two
	// accounts opened side by side do not look like copies of each other.
	offset := len(a.LoginID)
	for _, group := range []struct {
		status string
		count  int
	}{
		{StatusDraft, a.Drafts},
		{StatusReview, a.Reviews},
		{StatusFinalized, a.Finalized},
	} {
		for range group.count {
			articles = append(articles, a.article(voiceID, group.status, index, offset, now))
			index++
		}
	}
	return articles
}

// article assembles one post. `index` is the account's own running count, which sets both
// the topic and how far back the post is dated.
func (a Account) article(voiceID, status string, index, offset int, now time.Time) Article {
	source := topics[(index+offset)%len(topics)]
	// The round counts how many times THIS account has been round the topic list, not how
	// far the offset has shifted it. Counting the offset in would put "(2)" on the very
	// first post of an account that has no first one, which reads as a bug in the list.
	round := index / len(topics)

	title := source.title
	if round > 0 {
		// The slug minter would disambiguate a repeat on its own, but only in the URL.
		// The title is what the list shows, so it says which one this is.
		title = fmt.Sprintf("%s (%d)", source.title, round+1)
	}

	article := Article{
		UserID:  a.LoginID,
		VoiceID: voiceID,
		Title:   title,
		Memo:    source.memo,
		Status:  status,
		// 19 hours apart rather than 24: consecutive posts then fall on different times
		// of day and occasionally skip a date, which is what a real account looks like
		// and what any date grouping in the list has to handle.
		CreatedAt: now.Add(-time.Duration(index+1) * 19 * time.Hour),
		Language:  language,
	}
	if status == StatusDraft {
		// A draft carries no content at all: that is what distinguishes "never generated"
		// from "generated and empty", and a seeded draft that already held content would
		// hide the empty-editor state entirely.
		return article
	}
	article.Content = &Content{
		Title:   title,
		Summary: source.summary,
		Tags:    source.tags,
		Blocks: []Block{
			{Type: BlockHeading, Content: source.heading, Level: 2},
			{Type: BlockText, Content: source.body[0]},
			{Type: BlockText, Content: source.body[1]},
			{Type: BlockList, Items: source.takeaway},
		},
	}
	return article
}
