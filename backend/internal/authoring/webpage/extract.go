package webpage

import (
	"fmt"
	"io"
	"strings"
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
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", err
	}
	title = pageTitle(doc)
	root := contentRoot(doc)
	w := &walker{max: maxChars}
	if root != nil {
		w.walk(root)
	}
	w.flushPhotos()
	return title, strings.Join(w.lines, "\n"), nil
}

type walker struct {
	lines  []string
	chars  int
	photos int
	max    int
	full   bool
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
	if w.full {
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
		case (n.DataAtom == atom.Div || n.DataAtom == atom.Td || n.DataAtom == atom.Dd || n.DataAtom == atom.Section) && !hasBlockChild(n):
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
	if text := excerpt(textOf(n), 80); text != "" {
		w.add(marker + text)
	}
}

// text adds a paragraph-like element as an excerpt, and counts the photos inside it in
// their place, since an editor may wrap a photo in a paragraph.
func (w *walker) text(marker string, n *html.Node) {
	if images := countImages(n); images > 0 {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			w.walk(child)
		}
		if text := excerpt(textOf(n), ParagraphExcerptChars); text != "" {
			w.add(marker + text)
		}
		return
	}
	if text := excerpt(textOf(n), ParagraphExcerptChars); text != "" {
		w.add(marker + text)
	}
}

var blocks = map[atom.Atom]bool{
	atom.Div: true, atom.P: true, atom.Ul: true, atom.Ol: true, atom.Li: true, atom.Table: true,
	atom.Section: true, atom.Article: true, atom.Blockquote: true, atom.Figure: true, atom.Main: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Dl: true, atom.Pre: true, atom.Hr: true,
}

func hasBlockChild(n *html.Node) bool {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && (blocks[child.DataAtom] || hasBlockChild(child)) {
			return true
		}
	}
	return false
}

func countImages(n *html.Node) int {
	count := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Img {
			count++
		}
		count += countImages(child)
	}
	return count
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && (skipped[node.DataAtom] || hidden(node)) {
			return
		}
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		if node.Type == html.ElementNode && node.DataAtom == atom.Br {
			b.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
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
func pageTitle(doc *html.Node) string {
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
					title = textOf(n)
				}
			case atom.H1:
				if h1 == "" {
					h1 = textOf(n)
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
