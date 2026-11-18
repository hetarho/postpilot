package webpage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ParagraphExcerptChars bounds each paragraph's text in the structure: enough to say what a
// section is about, not to carry the page's prose into a template.
const ParagraphExcerptChars = 120

// Extract reads an HTML document's title and its body's structure, in document order:
// headings as `## ` / `### ` lines, photos as `[사진]` (a run of them as `[사진 n장 묶음]`),
// paragraphs as excerpts, list items as `- ` and quotes as `> ` lines. Navigation, headers,
// footers, scripts, forms and frames are skipped. The structure is cut at maxChars Unicode
// characters, at a line boundary.
func Extract(r io.Reader, maxChars int) (title, structure string, err error) {
	return extract(context.Background(), r, maxChars)
}

// extract is Extract bounded by ctx: a page built to make the walk slow stops at the read's
// deadline with ctx's error.
func extract(ctx context.Context, r io.Reader, maxChars int) (title, structure string, err error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", err
	}
	w := &walker{ctx: ctx, max: maxChars}
	title = pageTitle(doc, w.tick)
	root := contentRoot(doc)
	if root != nil {
		// Whether an element holds an image or a block is read once for the whole tree,
		// not again at every ancestor of a deeply nested element.
		w.images = holders(root, func(n *html.Node) bool { return n.DataAtom == atom.Img })
		w.blocks = holders(root, func(n *html.Node) bool { return blocks[n.DataAtom] })
		w.walk(root)
	}
	w.flushPhotos()
	if w.err != nil {
		return "", "", w.err
	}
	return title, strings.Join(w.lines, "\n"), nil
}

type walker struct {
	ctx    context.Context
	lines  []string
	chars  int
	photos int
	max    int
	full   bool
	visits int
	err    error
	images map[*html.Node]bool
	blocks map[*html.Node]bool
}

// tick counts one visited node and reports whether the walk must stop: its output budget
// is spent or its deadline has passed.
func (w *walker) tick() bool {
	if w.full {
		return true
	}
	w.visits++
	if w.visits%1024 == 0 && w.ctx.Err() != nil {
		w.err, w.full = w.ctx.Err(), true
	}
	return w.full
}

func (w *walker) add(line string) {
	if w.full {
		return
	}
	w.flushPhotos()
	w.push(line)
}

func (w *walker) push(line string) {
	n := utf8.RuneCountInString(line) + 1
	if w.chars+n > w.max {
		w.full = true
		return
	}
	w.lines = append(w.lines, line)
	w.chars += n
}

func (w *walker) flushPhotos() {
	if w.photos == 0 || w.full {
		w.photos = 0
		return
	}
	if w.photos == 1 {
		w.push("[사진]")
	} else {
		w.push(fmt.Sprintf("[사진 %d장 묶음]", w.photos))
	}
	w.photos = 0
}

var skipped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true, atom.Svg: true,
	atom.Nav: true, atom.Header: true, atom.Footer: true, atom.Aside: true, atom.Form: true,
	atom.Button: true, atom.Iframe: true, atom.Object: true, atom.Select: true, atom.Textarea: true,
	atom.Head: true, atom.Canvas: true, atom.Video: true, atom.Audio: true, atom.Figcaption: true,
}

func (w *walker) walk(n *html.Node) {
	if w.tick() {
		return
	}
	if n.Type == html.ElementNode {
		if skipped[n.DataAtom] || hidden(n) {
			return
		}
		switch {
		case n.DataAtom == atom.H1 || n.DataAtom == atom.H2:
			w.heading("## ", n)
			return
		case n.DataAtom == atom.H3 || n.DataAtom == atom.H4 || n.DataAtom == atom.H5 || n.DataAtom == atom.H6:
			w.heading("### ", n)
			return
		case hasClass(n, "se-sectionTitle"):
			// Naver SmartEditor's 소제목 component is a paragraph inside a titled section.
			w.heading("## ", n)
			return
		case n.DataAtom == atom.Img:
			if attr(n, "src") != "" || attr(n, "data-src") != "" || attr(n, "data-lazy-src") != "" {
				w.photos++
			}
			return
		case n.DataAtom == atom.P:
			w.text("", n)
			return
		case n.DataAtom == atom.Li:
			w.text("- ", n)
			return
		case n.DataAtom == atom.Blockquote:
			w.text("> ", n)
			return
		case (n.DataAtom == atom.Div || n.DataAtom == atom.Td || n.DataAtom == atom.Dd || n.DataAtom == atom.Section) && !w.blocks[n]:
			// Text set straight into a box, with <br> for its lines, is a paragraph too.
			w.text("", n)
			return
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		w.walk(child)
	}
}

func (w *walker) heading(marker string, n *html.Node) {
	if text := excerptOf(n, 80, w.tick); text != "" {
		w.add(marker + text)
	}
}

// text adds a paragraph-like element as an excerpt, and counts the photos inside it in
// their place, since an editor may wrap a photo in a paragraph.
func (w *walker) text(marker string, n *html.Node) {
	if w.images[n] {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			w.walk(child)
		}
	}
	if w.full {
		return
	}
	if text := excerptOf(n, ParagraphExcerptChars, w.tick); text != "" {
		w.add(marker + text)
	}
}

var blocks = map[atom.Atom]bool{
	atom.Div: true, atom.P: true, atom.Ul: true, atom.Ol: true, atom.Li: true, atom.Table: true,
	atom.Section: true, atom.Article: true, atom.Blockquote: true, atom.Figure: true, atom.Main: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Dl: true, atom.Pre: true, atom.Hr: true,
}

// holders marks every node with an element below it that is one of what: one pass over
// the tree, where asking each element in turn would walk its subtree again at every level.
func holders(root *html.Node, is func(*html.Node) bool) map[*html.Node]bool {
	held := map[*html.Node]bool{}
	var visit func(*html.Node) bool
	visit = func(n *html.Node) bool {
		holds := false
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if visit(child) {
				holds = true
			}
		}
		if holds {
			held[n] = true
		}
		return holds || (n.Type == html.ElementNode && is(n))
	}
	visit(root)
	return held
}

// excerptOf is n's visible text, its whitespace collapsed, cut by excerpt — gathered
// without reading more of n than the excerpt needs: at most max+1 characters, so an
// element's cost does not grow with the text below it. stop, when it reports true, ends the
// gathering early.
func excerptOf(n *html.Node, max int, stop func() bool) string {
	var b strings.Builder
	count, gap := 0, false
	var visit func(*html.Node) bool
	visit = func(node *html.Node) bool {
		if stop != nil && stop() {
			return false
		}
		if node.Type == html.ElementNode && (skipped[node.DataAtom] || hidden(node)) {
			return true
		}
		switch {
		case node.Type == html.TextNode:
			for _, r := range node.Data {
				if r == '\u200b' {
					continue
				}
				if unicode.IsSpace(r) {
					gap = true
					continue
				}
				if gap && count > 0 {
					b.WriteByte(' ')
					count++
				}
				gap = false
				b.WriteRune(r)
				if count++; count > max {
					return false
				}
			}
			gap = true
		case node.Type == html.ElementNode && node.DataAtom == atom.Br:
			gap = true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if !visit(child) {
				return false
			}
		}
		return true
	}
	visit(n)
	return excerpt(b.String(), max)
}

func excerpt(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "​", ""))
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:max])) + "…"
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, name := range strings.Fields(attr(n, "class")) {
		if name == class {
			return true
		}
	}
	return false
}

func hidden(n *html.Node) bool {
	if _, ok := attrOK(n, "hidden"); ok {
		return true
	}
	if attr(n, "aria-hidden") == "true" {
		return true
	}
	style := strings.ReplaceAll(strings.ToLower(attr(n, "style")), " ", "")
	return strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden")
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// pageTitle prefers the page's own og:title, then <title>, then its first h1.
func pageTitle(doc *html.Node, stop func() bool) string {
	var og, title, h1 string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Meta:
				if og == "" && (attr(n, "property") == "og:title" || attr(n, "name") == "og:title") {
					og = attr(n, "content")
				}
			case atom.Title:
				if title == "" {
					title = excerptOf(n, 200, stop)
				}
			case atom.H1:
				if h1 == "" {
					h1 = excerptOf(n, 200, stop)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	for _, candidate := range []string{og, title, h1} {
		if text := excerpt(strings.Join(strings.Fields(candidate), " "), 200); text != "" {
			return text
		}
	}
	return ""
}

// contentRoot is where a page keeps its body: Naver SmartEditor's main container, an older
// Naver post's view area, the page's article, its main, and otherwise its body.
func contentRoot(doc *html.Node) *html.Node {
	var editor, legacy, article, main, body *html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case editor == nil && hasClass(n, "se-main-container"):
				editor = n
			case legacy == nil && (attr(n, "id") == "postViewArea" || hasClass(n, "post_ct")):
				legacy = n
			case article == nil && n.DataAtom == atom.Article:
				article = n
			case main == nil && n.DataAtom == atom.Main:
				main = n
			case body == nil && n.DataAtom == atom.Body:
				body = n
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	for _, candidate := range []*html.Node{editor, legacy, article, main, body} {
		if candidate != nil {
			return candidate
		}
	}
	return doc
}
