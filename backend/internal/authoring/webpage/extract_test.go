package webpage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

// A trimmed SmartEditor ONE post: the body lives in se-main-container; 소제목 is a
// se-sectionTitle component; photos are lazy images, alone or in a strip.
const smartEditorPost = `<!doctype html><html><head>
<title>네이버 블로그</title><meta property="og:title" content="성수동 카페 후기">
</head><body>
<div id="header"><nav><a href="/">블로그 홈</a></nav><p>이웃추가</p></div>
<div class="se-main-container">
 <div class="se-component se-text"><div class="se-component-content"><div class="se-section se-section-text"><div class="se-module se-module-text">
  <p class="se-text-paragraph"><span>오늘은 성수동에 다녀왔어요.</span></p>
  <p class="se-text-paragraph"><span>` + "​" + `</span></p>
 </div></div></div></div>
 <div class="se-component se-sectionTitle se-l-default"><div class="se-component-content"><div class="se-section se-section-sectionTitle"><div class="se-module se-module-text">
  <p class="se-text-paragraph"><span>가는 길</span></p>
 </div></div></div></div>
 <div class="se-component se-image"><div class="se-component-content"><div class="se-section se-section-image"><div class="se-module se-module-image">
  <a class="se-module-image-link"><img class="se-image-resource" src="https://postfiles.example/a.jpg" data-lazy-src="https://postfiles.example/a.jpg"></a>
 </div></div></div></div>
 <div class="se-component se-text"><div class="se-component-content"><div class="se-section se-section-text"><div class="se-module se-module-text">
  <p class="se-text-paragraph"><span>역에서 </span><b>5분</b><span> 걸려요.</span></p>
 </div></div></div></div>
 <div class="se-component se-imageStrip"><div class="se-component-content"><div class="se-section se-section-imageStrip">
  <div class="se-module se-module-image"><img data-lazy-src="https://postfiles.example/b.jpg"></div>
  <div class="se-module se-module-image"><img data-lazy-src="https://postfiles.example/c.jpg"></div>
 </div></div></div>
 <div class="se-component se-sectionTitle"><div class="se-component-content"><div class="se-section"><div class="se-module se-module-text">
  <p class="se-text-paragraph"><span>메뉴</span></p>
 </div></div></div></div>
 <div class="se-component se-text"><div class="se-component-content"><div class="se-section se-section-text"><div class="se-module se-module-text">
  <ul class="se-text-list"><li><p><span>아메리카노</span></p></li><li><p><span>라떼</span></p></li></ul>
 </div></div></div></div>
 <div class="se-component se-quotation"><div class="se-component-content"><div class="se-section se-section-quotation">
  <blockquote class="se-quotation-container"><div class="se-module se-module-text se-quote"><p class="se-text-paragraph"><span>커피가 맛있어요.</span></p></div></blockquote>
 </div></div></div>
</div>
<div class="comment_area"><p>댓글 쓰기</p></div>
<footer><p>저작권</p></footer>
</body></html>`

func TestExtractReadsASmartEditorPost(t *testing.T) {
	title, structure, err := Extract(strings.NewReader(smartEditorPost), 12000)
	if err != nil {
		t.Fatal(err)
	}
	if title != "성수동 카페 후기" {
		t.Fatalf("title = %q, want the og:title", title)
	}
	want := strings.Join([]string{
		"오늘은 성수동에 다녀왔어요.",
		"## 가는 길",
		"[사진]",
		"역에서 5분 걸려요.",
		"[사진 2장 묶음]",
		"## 메뉴",
		"- 아메리카노",
		"- 라떼",
		"> 커피가 맛있어요.",
	}, "\n")
	if structure != want {
		t.Fatalf("structure =\n%s\nwant\n%s", structure, want)
	}
}

func TestExtractReadsAGenericArticleAndSkipsWhatIsNotItsBody(t *testing.T) {
	long := strings.Repeat("가", 200)
	doc := `<html><head><title>Example Article</title><script>var secret = "token";</script><style>p { color: red }</style></head>
<body>
<header><h1>Site Name</h1><p>Tagline</p></header>
<nav><ul><li>Home</li><li>About</li></ul></nav>
<main><article>
<h1>How to brew coffee</h1>
<p>Intro paragraph.</p>
<h2>Equipment</h2>
<ul><li>Kettle</li><li>Grinder <b>burr</b></li></ul>
<figure><img src="kettle.jpg"><figcaption>A kettle caption</figcaption></figure>
<h3>Steps</h3>
<ol><li>Boil</li></ol>
<blockquote>Quote here.</blockquote>
<p>` + long + `</p>
<form><p>Subscribe to our newsletter</p><input name="email"><button>Go</button></form>
<aside><p>Related posts</p></aside>
<script>document.write("injected")</script>
<noscript><p>Enable JavaScript</p></noscript>
<div style="display: none"><p>Hidden by style</p></div>
<div style="visibility:hidden"><p>Invisible</p></div>
<p hidden>Hidden attribute</p>
<p aria-hidden="true">Aria hidden</p>
<iframe src="https://ads.example/"></iframe>
<footer><p>Article footer</p></footer>
<h4>Notes</h4>
<div>Line one<br>line two</div>
<h5>` + strings.Repeat("h", 100) + `</h5>
</article></main>
<footer>Copyright</footer>
</body></html>`
	title, structure, err := Extract(strings.NewReader(doc), 12000)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Example Article" {
		t.Fatalf("title = %q, want <title> when there is no og:title", title)
	}
	want := strings.Join([]string{
		"## How to brew coffee",
		"Intro paragraph.",
		"## Equipment",
		"- Kettle",
		"- Grinder burr",
		"[사진]",
		"### Steps",
		"- Boil",
		"> Quote here.",
		strings.Repeat("가", ParagraphExcerptChars) + "…",
		"### Notes",
		"Line one line two",
		"### " + strings.Repeat("h", 80) + "…",
	}, "\n")
	if structure != want {
		t.Fatalf("structure =\n%s\nwant\n%s", structure, want)
	}
	for _, leaked := range []string{"Site Name", "Tagline", "Home", "secret", "caption", "Subscribe", "Related", "injected", "JavaScript", "Hidden", "Invisible", "Aria", "footer", "Copyright"} {
		if strings.Contains(structure, leaked) {
			t.Errorf("structure carries skipped text %q", leaked)
		}
	}
}

func TestExtractTitlePrefersOgTitleThenTitleThenH1(t *testing.T) {
	cases := map[string]string{
		`<html><head><meta property="og:title" content=" OG  title "><title>Tab</title></head><body><h1>Heading</h1></body></html>`: "OG title",
		`<html><head><title> Tab
		title </title></head><body><h1>Heading</h1></body></html>`: "Tab title",
		`<html><body><h1>Heading <em>one</em></h1><h1>Two</h1></body></html>`:       "Heading one",
		`<html><head><title>` + strings.Repeat("t", 300) + `</title></head></html>`: strings.Repeat("t", 200) + "…",
	}
	for doc, want := range cases {
		title, _, err := Extract(strings.NewReader(doc), 12000)
		if err != nil {
			t.Fatal(err)
		}
		if title != want {
			t.Errorf("title = %q, want %q", title, want)
		}
	}
}

func TestExtractPrefersTheBodyContainer(t *testing.T) {
	cases := map[string]string{
		`<body><p>outside</p><div id="postViewArea"><p>legacy naver</p></div></body>`:                "legacy naver",
		`<body><p>outside</p><article><p>in article</p></article><main><p>in main</p></main></body>`: "in article",
		`<body><p>outside</p><main><p>in main</p></main></body>`:                                     "in main",
		`<body><p>only body</p></body>`:                                                              "only body",
	}
	for doc, want := range cases {
		_, structure, err := Extract(strings.NewReader(doc), 12000)
		if err != nil {
			t.Fatal(err)
		}
		if structure != want {
			t.Errorf("structure = %q, want %q", structure, want)
		}
	}
}

// The structure is cut at maxChars characters, between lines: never inside one, and
// nothing — not a pending photo either — after the cut.
func TestExtractCutsAtALineBoundary(t *testing.T) {
	var doc strings.Builder
	doc.WriteString("<html><body>")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&doc, "<p>문단 %02d 내용</p>", i)
		if i == 9 {
			doc.WriteString(`<img src="a.jpg">`)
		}
	}
	doc.WriteString("</body></html>")
	_, full, err := Extract(strings.NewReader(doc.String()), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Split(full, "\n")
	for _, max := range []int{1, 9, 10, 11, 100, 101, 117, 333} {
		_, cut, err := Extract(strings.NewReader(doc.String()), max)
		if err != nil {
			t.Fatal(err)
		}
		if n := utf8.RuneCountInString(cut); n > max {
			t.Fatalf("max %d: %d characters", max, n)
		}
		if cut == "" {
			continue
		}
		lines := strings.Split(cut, "\n")
		// Whole lines, in order, and as many as fit.
		if strings.Join(all[:len(lines)], "\n") != cut {
			t.Fatalf("max %d: %q is not a run of whole lines", max, cut)
		}
		used := utf8.RuneCountInString(cut) + 1
		if len(lines) < len(all) && used+utf8.RuneCountInString(all[len(lines)])+1 <= max {
			t.Fatalf("max %d: stopped early at %d lines", max, len(lines))
		}
	}
}

func TestReadPageDecodesEUCKR(t *testing.T) {
	body, err := korean.EUCKR.NewEncoder().Bytes([]byte(`<html><head><title>옛 인코딩 글</title></head><body><h2>첫 소제목</h2><p>본문입니다.</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	at, _ := url.Parse("http://old.example/")
	for _, contentType := range []string{"text/html; charset=euc-kr", "text/html; charset=EUC-KR", "text/html; charset=ks_c_5601-1987"} {
		got, err := readPage(context.Background(), at, body, contentType, 12000)
		if err != nil {
			t.Fatalf("%s: %v", contentType, err)
		}
		if got.Title != "옛 인코딩 글" || got.Structure != "## 첫 소제목\n본문입니다." || got.URL != "http://old.example/" {
			t.Fatalf("%s: page = %+v", contentType, got)
		}
	}
}

// adversarialPage nests depth blockquotes around an image and textBytes of text: before
// the walk was bounded, every one of the ancestors re-read (and copied) the whole text.
func adversarialPage(depth, textBytes int) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	b.WriteString(strings.Repeat("<blockquote>", depth))
	b.WriteString(`<img src="a.jpg">`)
	b.WriteString(strings.Repeat("word ", textBytes/5))
	b.WriteString(strings.Repeat("</blockquote>", depth))
	b.WriteString("</body></html>")
	return b.String()
}

func TestExtractBoundsTheWorkOfADeeplyNestedPage(t *testing.T) {
	doc := adversarialPage(500, 2<<20-16384)
	if len(doc) > 2<<20 {
		t.Fatalf("fixture is %d bytes, over the fetch cap", len(doc))
	}
	// Parsing alone is the floor: the extraction may cost a few times that, not hundreds.
	var before, parsed, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, _, err := Extract(strings.NewReader("<html><body>"+strings.Repeat("word ", (2<<20-16384)/5)+"</body></html>"), 12000); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&parsed)
	floor := parsed.TotalAlloc - before.TotalAlloc

	started := time.Now()
	_, structure, err := Extract(strings.NewReader(doc), 12000)
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	allocated := after.TotalAlloc - parsed.TotalAlloc
	if allocated > 4*floor+(16<<20) {
		t.Fatalf("allocated %d MB for a page whose flat form costs %d MB", allocated>>20, floor>>20)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("took %v", elapsed)
	}
	if !strings.HasPrefix(structure, "[사진]\n> word word") || utf8.RuneCountInString(structure) > 12000 {
		t.Fatalf("structure = %.80q… (%d characters)", structure, utf8.RuneCountInString(structure))
	}
}

func TestExtractStopsAtTheReadsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := extract(ctx, strings.NewReader(adversarialPage(300, 64<<10)), 12000)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the context's error", err)
	}
	at, _ := url.Parse("http://slow.example/")
	_, err = readPage(ctx, at, []byte(adversarialPage(300, 64<<10)), "text/html", 12000)
	if code := refusalCode(t, err); code != RefusalUnreachable {
		t.Fatalf("readPage after the deadline = %s, want %s", code, RefusalUnreachable)
	}
}
