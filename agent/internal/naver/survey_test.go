//go:build survey

// Throwaway live survey harness for T042's four open questions. Build-tagged so it never
// enters a normal build or CI run, and DELETED once T042's `## survey` section records the
// answers. It reads and types into an unsaved draft only: no publish control is ever
// resolved or activated, and nothing is saved.
//
//	cd agent && go test -tags survey -run TestSurveyT042 -v -timeout 10m ./internal/naver/
package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
)

// surveyStructureFn dumps the raw component/paragraph tree so the answers are readable
// whatever shape the editor produces. Unlike the shipped projection it makes no semantic
// decision: it reports every p/li/ul/ol under each top-level component with its own class
// list, which is exactly what "did it convert the paragraph or the component" needs.
const surveyStructureFn = `function () {
  const norm = (v) => String(v == null ? '' : v).replace(/\s+/g, ' ').trim();
  const nodeText = (root) => {
    if (!root) return '';
    const nodes = [...root.querySelectorAll('span.__se-node')];
    const text = nodes.length ? nodes.map((n) => n.textContent).join('') : norm(root.textContent);
    return text.length > 90 ? text.slice(0, 90) + '…' : text;
  };
  const sel = window.getSelection();
  const anchorNode = sel && sel.anchorNode ? sel.anchorNode : null;
  const anchor = anchorNode ? (anchorNode.nodeType === 1 ? anchorNode : anchorNode.parentElement) : null;
  const body = document.querySelector('.se-body.__se-body');
  const tops = body ? [...body.querySelectorAll('.se-component')].filter((c) => !(c.parentElement && c.parentElement.closest('.se-component'))) : [];

  const components = tops.map((component, index) => ({
    index,
    classes: norm(component.className),
    has_caret: Boolean(anchor && component.contains(anchor)),
    units: [...component.querySelectorAll('p, li, ul, ol')].map((unit) => ({
      tag: unit.tagName,
      classes: norm(unit.className),
      text: nodeText(unit),
      has_caret: Boolean(anchor && unit.contains(anchor))
    }))
  }));

  const caretComponent = anchor ? anchor.closest('.se-component') : null;
  const caretUnit = anchor ? anchor.closest('p, li') : null;
  const caret = {
    found: Boolean(anchor),
    component_index: caretComponent ? tops.indexOf(caretComponent) : -1,
    component_classes: caretComponent ? norm(caretComponent.className) : '',
    unit_tag: caretUnit ? caretUnit.tagName : '',
    unit_classes: caretUnit ? norm(caretUnit.className) : '',
    unit_text: nodeText(caretUnit),
    offset: sel ? sel.anchorOffset : -1,
    collapsed: sel ? sel.isCollapsed : false
  };

  // The shipped body_end locator, reported rather than acted on, so question (c) can be
  // answered by reading what it matches after a conversion.
  const paragraphs = body ? [...body.querySelectorAll('.se-component.se-text .se-module-text.__se-unit p.se-text-paragraph')] : [];
  const last = paragraphs[paragraphs.length - 1];
  const bodyEnd = {
    count: paragraphs.length,
    last_classes: last ? norm(last.className) : '',
    last_text: nodeText(last),
    last_component: last ? norm((last.closest('.se-component') || {className: ''}).className) : ''
  };

  const count = (selector) => document.querySelectorAll(selector).length;
  return {
    href: location.href,
    image_count: count('.se-body.__se-body .se-component.se-image'),
    components,
    caret,
    body_end: bodyEnd,
    locators: {
      format_menu: count('button.se-text-format-toolbar-button'),
      format_text: count('button.se-toolbar-option-text-format-text-button'),
      format_heading: count('button.se-toolbar-option-text-format-sectionTitle-button'),
      format_quote: count('button.se-toolbar-option-text-format-quotation-button'),
      list_menu: count('button.se-list-bullet-toolbar-button'),
      list_bullet: count('button.se-toolbar-option-list-bullet-button'),
      list_decimal: count('button.se-toolbar-option-list-decimal-button')
    }
  };
}`

// surveyCaretFn places the caret in one addressed unit by the 98 %-width rule, so a step
// can re-seat the caret after a conversion instead of relying on body_end (whose match set
// is one of the things under survey).
const surveyCaretFn = `function (componentIndex, unitIndex) {
  const body = document.querySelector('.se-body.__se-body');
  if (!body) return {matches: 0, x: 0, y: 0};
  const tops = [...body.querySelectorAll('.se-component')].filter((c) => !(c.parentElement && c.parentElement.closest('.se-component')));
  const component = tops[componentIndex];
  if (!component) return {matches: 0, x: 0, y: 0};
  const units = [...component.querySelectorAll('p.se-text-paragraph, li.se-text-list-item, .se-module-text.se-quote')];
  const unit = unitIndex < 0 ? units[units.length - 1] : units[unitIndex];
  if (!unit) return {matches: 0, x: 0, y: 0};
  const box = unit.getBoundingClientRect();
  if (box.width <= 0 || box.height <= 0) return {matches: 0, x: 0, y: 0};
  return {matches: 1, x: Math.round(box.left + box.width * 0.98), y: Math.round(box.bottom - box.height * 0.25)};
}`

type surveyUnit struct {
	Tag      string `json:"tag"`
	Classes  string `json:"classes"`
	Text     string `json:"text"`
	HasCaret bool   `json:"has_caret"`
}

type surveyComponent struct {
	Index    int          `json:"index"`
	Classes  string       `json:"classes"`
	HasCaret bool         `json:"has_caret"`
	Units    []surveyUnit `json:"units"`
}

type surveyStructure struct {
	Href       string            `json:"href"`
	ImageCount int               `json:"image_count"`
	Components []surveyComponent `json:"components"`
	Caret      struct {
		Found            bool   `json:"found"`
		ComponentIndex   int    `json:"component_index"`
		ComponentClasses string `json:"component_classes"`
		UnitTag          string `json:"unit_tag"`
		UnitClasses      string `json:"unit_classes"`
		UnitText         string `json:"unit_text"`
		Offset           int    `json:"offset"`
		Collapsed        bool   `json:"collapsed"`
	} `json:"caret"`
	BodyEnd struct {
		Count         int    `json:"count"`
		LastClasses   string `json:"last_classes"`
		LastText      string `json:"last_text"`
		LastComponent string `json:"last_component"`
	} `json:"body_end"`
	Locators map[string]int `json:"locators"`
}

type survey struct {
	t    *testing.T
	ctx  context.Context
	port *CDPPort
	last surveyStructure
}

func (s *survey) dump(label string) surveyStructure {
	s.t.Helper()
	time.Sleep(250 * time.Millisecond)
	var structure surveyStructure
	if err := s.port.page.CallFunction(s.ctx, surveyStructureFn, []any{}, &structure); err != nil {
		s.t.Fatalf("%s: structure read failed: %v", label, err)
	}
	s.last = structure
	var out strings.Builder
	fmt.Fprintf(&out, "\n──────── %s\n", label)
	fmt.Fprintf(&out, "  locators   %s\n", locatorLine(structure.Locators))
	fmt.Fprintf(&out, "  caret      found=%t comp=%d <%s.%s> offset=%d text=%q\n",
		structure.Caret.Found, structure.Caret.ComponentIndex, structure.Caret.UnitTag,
		structure.Caret.UnitClasses, structure.Caret.Offset, structure.Caret.UnitText)
	fmt.Fprintf(&out, "  body_end   paragraphs=%d last=%q in=[%s]\n",
		structure.BodyEnd.Count, structure.BodyEnd.LastText, structure.BodyEnd.LastComponent)
	fmt.Fprintf(&out, "  images     %d\n", structure.ImageCount)
	for _, component := range structure.Components {
		marker := ""
		if component.HasCaret {
			marker = "   ⟵ caret"
		}
		fmt.Fprintf(&out, "  [%d] %s%s\n", component.Index, component.Classes, marker)
		for _, unit := range component.Units {
			unitMarker := ""
			if unit.HasCaret {
				unitMarker = "   ⟵ caret"
			}
			fmt.Fprintf(&out, "        %-3s %-40s %q%s\n", unit.Tag, unit.Classes, unit.Text, unitMarker)
		}
	}
	s.t.Log(out.String())
	return structure
}

func locatorLine(locators map[string]int) string {
	order := []string{"format_menu", "format_text", "format_heading", "format_quote", "list_menu", "list_bullet", "list_decimal"}
	parts := make([]string, 0, len(order))
	for _, key := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", key, locators[key]))
	}
	return strings.Join(parts, " ")
}

// act clicks one versioned control through the SHIPPED driver function and reports how many
// elements it matched, which is the fact 260908 lost (format_menu matched 0 with no caret).
func (s *survey) act(control string) {
	s.t.Helper()
	var activation driverActivation
	if err := s.port.page.CallFunction(s.ctx, driverActivateFn, []any{control, ""}, &activation); err != nil {
		s.t.Fatalf("act %s: %v", control, err)
	}
	s.t.Logf("  act %-16s matches=%d activated=%t already=%t", control, activation.Matches, activation.Activated, activation.Already)
	if activation.Matches != 1 || (!activation.Activated && !activation.Already) {
		s.t.Fatalf("act %s refused (matches=%d) — the answer so far is in the dumps above", control, activation.Matches)
	}
	time.Sleep(250 * time.Millisecond)
}

// seatCaret re-seats the caret in the last unit of the last component that has one, so a
// step never depends on body_end while body_end itself is under survey.
func (s *survey) seatCaret() {
	s.t.Helper()
	componentIndex := -1
	for _, component := range s.last.Components {
		if len(component.Units) > 0 {
			componentIndex = component.Index
		}
	}
	if componentIndex < 0 {
		s.t.Fatal("no component holds a unit to seat the caret in")
	}
	var point driverPoint
	if err := s.port.page.CallFunction(s.ctx, surveyCaretFn, []any{componentIndex, -1}, &point); err != nil {
		s.t.Fatalf("seat caret: %v", err)
	}
	if point.Matches != 1 {
		s.t.Fatalf("seat caret: component %d resolved %d units", componentIndex, point.Matches)
	}
	if err := s.port.page.ClickPoint(s.ctx, point.X, point.Y); err != nil {
		s.t.Fatalf("seat caret click: %v", err)
	}
	s.t.Logf("  seat caret       component=%d point=(%.0f,%.0f)", componentIndex, point.X, point.Y)
	time.Sleep(250 * time.Millisecond)
}

func (s *survey) write(text string) {
	s.t.Helper()
	if err := s.port.typeText(s.ctx, text); err != nil {
		s.t.Fatalf("write %q: %v", text, err)
	}
	time.Sleep(200 * time.Millisecond)
}

func (s *survey) enter() {
	s.t.Helper()
	if err := s.port.page.PressEnter(s.ctx); err != nil {
		s.t.Fatalf("enter: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

// openSurvey binds the dedicated page and reports the draft's state. It only reads: no
// typing, no click, no publish control. Both entry points below share it.
func openSurvey(t *testing.T, ctx context.Context, requireClean bool) *survey {
	t.Helper()
	paths, err := config.DefaultPaths()
	if err != nil {
		t.Fatalf("agent paths: %v", err)
	}
	cfg, err := config.Load(paths)
	if err != nil {
		t.Fatalf("load agent config: %v", err)
	}
	wanted := os.Getenv("POSTPILOT_SURVEY_CONNECTION")
	var chosen config.Connection
	for _, connection := range cfg.Connections {
		if !connection.Armed {
			continue
		}
		if wanted != "" && connection.ID != wanted && connection.Label != wanted {
			continue
		}
		if chosen.ID != "" {
			t.Fatalf("more than one armed connection; set POSTPILOT_SURVEY_CONNECTION to one of %q / %q", chosen.Label, connection.Label)
		}
		chosen = connection
	}
	if chosen.ID == "" {
		t.Fatal("no armed connection in the agent config — run `postpilot-agent setup` and pair first")
	}
	t.Logf("connection %s (%s) blog=%s profile=%s", chosen.Label, chosen.ID, chosen.PlatformAccountID, chosen.ProfileDir)

	// Connect, never Start: launching or navigating a dirty SmartEditor raises the
	// beforeunload dialog the reviewed driver surface cannot dismiss.
	session, err := browser.Connect(chosen.ProfileDir)
	if err != nil {
		t.Fatalf("dedicated browser CDP unavailable (%v)\n"+
			"the dedicated browser must already be open on the writer: run `postpilot-agent diagnostics` once, then open 내 블로그 글쓰기 in THAT browser window", err)
	}
	port, err := NewCDPPort(ctx, session.CDPURL)
	if err != nil {
		t.Fatalf("bind dedicated page: %v", err)
	}

	// ── S0: the clean-draft requirement, machine-checked.
	snapshot, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe failed: %v\nis the dedicated page on blog.naver.com/PostWriteForm.naver, signed in, with no native dialog open?", err)
	}
	t.Logf("S0 snapshot url=%s account=%s auth=%s signature=%s title=%q blocks=%d images=%d settings_layer=%t",
		snapshot.URL, snapshot.AccountID, snapshot.Auth, snapshot.SignatureID, snapshot.Title, len(snapshot.Body), snapshot.ImageCount, snapshot.SettingsLayerOpen)
	if snapshot.AccountID != chosen.PlatformAccountID {
		t.Fatalf("bound writer is blog %q but the connection is paired to %q", snapshot.AccountID, chosen.PlatformAccountID)
	}
	if snapshot.Auth != AuthReady {
		t.Fatalf("auth is %q — sign in to Naver in the dedicated browser first", snapshot.Auth)
	}
	if snapshot.SettingsLayerOpen {
		t.Fatal("the publish settings layer is open and occludes the editor — close it (발행 설정 닫기) and re-run")
	}
	if requireClean && (snapshot.ImageCount != 0 || len(snapshot.Body) != 0) {
		encoded, _ := json.Marshal(snapshot.Body)
		t.Fatalf("the draft is NOT clean: %d images, %d body blocks %s\n"+
			"discard the leftover draft in the dedicated browser (모두 지우기 / 취소, dismissing 작성 중인 글 restore), leave an empty writer open, and re-run",
			snapshot.ImageCount, len(snapshot.Body), encoded)
	}
	s := &survey{t: t, ctx: ctx, port: port}
	s.dump("S0 current draft")
	return s
}

// TestSurveyPreflight is the read-only half: it reports whether the draft is clean enough
// for TestSurveyT042 and writes nothing at all.
func TestSurveyPreflight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	if s.last.ImageCount != 0 {
		t.Logf("\nPREFLIGHT: DIRTY — %d image(s) in the body. Discard this draft in the dedicated browser, leave an empty writer open, then re-run this preflight.", s.last.ImageCount)
		return
	}
	for _, component := range s.last.Components {
		for _, unit := range component.Units {
			if strings.TrimSpace(unit.Text) != "" {
				t.Log("\nPREFLIGHT: DIRTY — the body already holds text. Discard this draft in the dedicated browser, leave an empty writer open, then re-run this preflight.")
				return
			}
		}
	}
	t.Log("\nPREFLIGHT: CLEAN — run TestSurveyT042 next.")
}

// TestSurveyT042 answers T042's four open questions in one pass on a clean writer draft.
func TestSurveyT042(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, true)
	port := s.port
	defer port.Close()

	// ── S1..S3 build the two-paragraph component every question needs.
	if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
		t.Fatalf("seat caret at body_end: %v", err)
	}
	s.dump("S1 caret seated at body_end")
	s.write("SURVEY P1 첫 문단")
	s.dump("S2 first paragraph")
	s.enter()
	s.write("SURVEY P2 둘째 문단")
	s.dump("S3 two paragraphs — one component or two?")

	// ── (a) 문단 서식 변경 → 소제목 with the caret in the SECOND paragraph of a
	// two-paragraph component: does it convert that paragraph or the whole component?
	s.act("format_menu")
	s.dump("S4 format menu open (caret in paragraph 2 of 2)")
	s.act("format_heading")
	s.dump("S5 ANSWER (a) — 소제목 applied to paragraph 2 of 2")

	// ── (c) what Enter from the converted block opens, and what body_end then matches.
	s.seatCaret()
	s.enter()
	s.write("SURVEY P3 변환 후 엔터")
	s.dump("S6 ANSWER (c) — Enter out of the converted block, and body_end's match")

	// ── (b) the same question for 인용구, on a fresh multi-paragraph text component.
	s.seatCaret()
	s.enter()
	s.write("SURVEY P4 인용구 대상")
	s.dump("S7 setup for (b)")
	s.act("format_menu")
	s.act("format_quote")
	s.dump("S8 ANSWER (b) — 인용구 applied to the caret's paragraph")

	// ── (d) the list toolbar on a multi-paragraph component.
	s.seatCaret()
	s.enter()
	s.write("SURVEY P5 리스트 대상 1")
	s.enter()
	s.write("SURVEY P6 리스트 대상 2")
	s.dump("S9 setup for (d) — list_menu's match count with the caret here")
	s.act("list_menu")
	s.dump("S10 list menu open")
	s.act("list_bullet")
	s.dump("S11 ANSWER (d) — 글머리 기호 applied on a multi-paragraph component")

	t.Log("\nsurvey complete. Nothing was saved and no publish control was resolved.\n" +
		"Discard this draft in the dedicated browser before doing anything else.")
}

// seatComponent seats the caret in the last unit of one addressed component, so a pass can
// target a plain text component by index instead of relying on body_end (which matches only
// `.se-component.se-text` paragraphs and therefore skips converted components entirely).
func (s *survey) seatComponent(index int) {
	s.t.Helper()
	var point driverPoint
	if err := s.port.page.CallFunction(s.ctx, surveyCaretFn, []any{index, -1}, &point); err != nil {
		s.t.Fatalf("seat component %d: %v", index, err)
	}
	if point.Matches != 1 {
		s.t.Fatalf("seat component %d resolved %d units", index, point.Matches)
	}
	if err := s.port.page.ClickPoint(s.ctx, point.X, point.Y); err != nil {
		s.t.Fatalf("seat component %d click: %v", index, err)
	}
	s.t.Logf("  seat component=%d point=(%.0f,%.0f)", index, point.X, point.Y)
	time.Sleep(250 * time.Millisecond)
}

// lastPlainText finds the last top-level component that is a plain `se-text` — the only kind
// the list control exists for, as pass 1 proved by matching 0 inside a quotation.
func (s *survey) lastPlainText() int {
	s.t.Helper()
	index := -1
	for _, component := range s.last.Components {
		classes := strings.Fields(component.Classes)
		plain := false
		for _, class := range classes {
			switch class {
			case "se-text":
				plain = true
			case "se-documentTitle", "se-sectionTitle", "se-quotation", "se-image", "se-imageStrip":
				plain = false
			}
		}
		if plain {
			index = component.Index
		}
	}
	if index < 0 {
		s.t.Fatal("no plain se-text component in the body")
	}
	return index
}

// TestSurveyT042List answers question (d) on a multi-paragraph PLAIN TEXT component, and the
// follow-up a three-item list needs: what Enter from a converted list item produces. It does
// not require a clean draft — it addresses its target component explicitly.
func TestSurveyT042List(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	target := s.lastPlainText()
	t.Logf("targeting plain se-text component [%d]", target)
	s.seatComponent(target)
	for _, text := range []string{"SURVEY L1 리스트 대상 1", "SURVEY L2 리스트 대상 2", "SURVEY L3 리스트 대상 3"} {
		s.enter()
		s.write(text)
	}
	s.dump("L1 three paragraphs in one plain text component")

	s.seatComponent(target)
	s.dump("L2 caret re-seated — list_menu's match count in a multi-paragraph text component")
	s.act("list_menu")
	s.dump("L3 list menu open")
	s.act("list_bullet")
	s.dump("L4 ANSWER (d) — 글머리 기호 on the caret's paragraph of a 3-paragraph component")

	s.enter()
	s.write("SURVEY L4 엔터 후")
	s.dump("L5 what Enter from a converted list item produces")
}

// surveyModuleFn dumps the module layer the paragraph dump skipped: which
// `.se-module-*` wrapper each paragraph sits in. The quotation's rendered output shows a
// large centred quote line and a smaller line under it, so the two are different modules
// and the projection must not read them as one.
const surveyModuleFn = `function () {
  const norm = (v) => String(v == null ? '' : v).replace(/\s+/g, ' ').trim();
  const nodeText = (root) => {
    if (!root) return '';
    const nodes = [...root.querySelectorAll('span.__se-node')];
    const text = nodes.length ? nodes.map((n) => n.textContent).join('') : norm(root.textContent);
    return text.length > 60 ? text.slice(0, 60) + '…' : text;
  };
  const body = document.querySelector('.se-body.__se-body');
  const tops = body ? [...body.querySelectorAll('.se-component')].filter((c) => !(c.parentElement && c.parentElement.closest('.se-component'))) : [];
  return {
    components: tops.map((component, index) => ({
      index,
      classes: norm(component.className),
      modules: [...component.querySelectorAll('[class*="se-module"]')].map((module) => ({
        classes: norm(module.className),
        empty_flag: module.classList.contains('se-is-empty'),
        paragraphs: [...module.querySelectorAll('p.se-text-paragraph, li.se-text-list-item')].map((unit) => ({
          tag: unit.tagName, classes: norm(unit.className), text: nodeText(unit)
        })),
        text: nodeText(module)
      }))
    }))
  };
}`

type surveyModuleDump struct {
	Components []struct {
		Index   int    `json:"index"`
		Classes string `json:"classes"`
		Modules []struct {
			Classes    string `json:"classes"`
			EmptyFlag  bool   `json:"empty_flag"`
			Text       string `json:"text"`
			Paragraphs []struct {
				Tag     string `json:"tag"`
				Classes string `json:"classes"`
				Text    string `json:"text"`
			} `json:"paragraphs"`
		} `json:"modules"`
	} `json:"components"`
}

// TestSurveyModules is read-only. It answers what the quotation's inner modules are and what
// the SHIPPED projection makes of the whole survey document.
func TestSurveyModules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	s.dumpModules("module layer")
	s.dumpProjection()
}

func (s *survey) dumpModules(label string) {
	s.t.Helper()
	time.Sleep(250 * time.Millisecond)
	var dump surveyModuleDump
	if err := s.port.page.CallFunction(s.ctx, surveyModuleFn, []any{}, &dump); err != nil {
		s.t.Fatalf("module dump: %v", err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\n──────── %s\n", label)
	for _, component := range dump.Components {
		fmt.Fprintf(&out, "  [%d] %s\n", component.Index, component.Classes)
		for _, module := range component.Modules {
			fmt.Fprintf(&out, "      MODULE %-46s empty=%t text=%q\n", module.Classes, module.EmptyFlag, module.Text)
			for _, paragraph := range module.Paragraphs {
				fmt.Fprintf(&out, "             %-3s %-44s %q\n", paragraph.Tag, paragraph.Classes, paragraph.Text)
			}
		}
	}
	s.t.Log(out.String())
}

func (s *survey) dumpProjection() {
	s.t.Helper()
	snapshot, err := s.port.Observe(s.ctx)
	if err != nil {
		s.t.Fatalf("observe: %v", err)
	}
	var out strings.Builder
	out.WriteString("\n──────── what the SHIPPED projection reports\n")
	for i, block := range snapshot.Body {
		fmt.Fprintf(&out, "  %2d %-6s %q\n", i, block.Kind, block.Text)
	}
	s.t.Log(out.String())
}

// TestSurveyQuoteModules reproduces just the quotation case and reads its MODULE layer, the
// one thing the paragraph dump could not see: the rendered output shows a large centred
// quote line with a smaller line beneath it, so Enter from a quote body may be typing into a
// source/citation module rather than into the quote itself.
func TestSurveyQuoteModules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	if err := s.port.resolveAndClick(ctx, "body_end", 0); err != nil {
		t.Fatalf("seat caret at body_end: %v", err)
	}
	s.enter()
	s.write("QUOTE 본문 문단")
	s.enter()
	s.write("QUOTE 변환 대상")
	s.dump("Q1 two plain paragraphs, caret in the second")
	s.act("format_menu")
	s.act("format_quote")
	s.dump("Q2 converted to 인용구")
	s.dumpModules("Q2 module layer of the fresh quotation")
	s.enter()
	s.write("QUOTE 엔터 후 1")
	s.dump("Q3 after one Enter + text inside the quotation")
	s.dumpModules("Q3 module layer — which module took the new text")
	s.dumpProjection()
}

// TestSurveyDriverProbe is read-only. It calls the SHIPPED reviewed functions this task
// added against the live editor, so a wrong class name or a JS syntax error in them shows up
// here instead of during T008's live smoke.
func TestSurveyDriverProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	var end driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_end", 0}, &end); err != nil {
		t.Fatalf("body_end: %v", err)
	}
	t.Logf("body_end        matches=%d point=(%.0f,%.0f)", end.Matches, end.X, end.Y)

	var first driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_paragraph", 0}, &first); err != nil {
		t.Fatalf("body_paragraph 0: %v", err)
	}
	t.Logf("body_paragraph0 matches=%d point=(%.0f,%.0f)", first.Matches, first.X, first.Y)

	var beyond driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_paragraph", 99}, &beyond); err != nil {
		t.Fatalf("body_paragraph 99: %v", err)
	}
	t.Logf("body_paragraph99 matches=%d (must be 0: an out-of-range index fails closed)", beyond.Matches)

	for _, index := range []int{0, -1} {
		state, err := s.port.paragraphState(ctx, "body_paragraph", index)
		if err != nil {
			t.Fatalf("paragraph state %d: %v", index, err)
		}
		t.Logf("paragraphState(%2d) matches=%d total=%d kind=%q in_list=%t in_cite=%t list_items=%d empty=%t",
			index, state.Matches, state.Total, state.Kind, state.InList, state.InCite, state.ListItems, state.Empty)
	}
	if end.Matches != 1 || first.Matches != 1 || beyond.Matches != 0 {
		t.Fatalf("the new body locators did not resolve as expected on a live writer")
	}
}

// uploadImage performs the sequence T019 verified on 260907, so a survey pass can ask WHERE
// the editor puts the result: intercept the chooser, resolve and click the image button,
// then hand the hidden input exactly one file.
func (s *survey) uploadImage(path string) {
	s.t.Helper()
	if err := s.port.page.InterceptFileChooser(s.ctx); err != nil {
		s.t.Fatalf("intercept chooser: %v", err)
	}
	s.act("image_add")
	if err := s.port.page.SetFileInputFiles(s.ctx, "input[type=file]#hidden-file", []string{path}); err != nil {
		s.t.Fatalf("set files %s: %v", path, err)
	}
	s.t.Logf("  upload           %s", path)
	time.Sleep(2500 * time.Millisecond)
	// The upload opens Naver's photo-library sidebar, which overlays the editor's right
	// edge and swallows every caret click taken there.
	s.act("close_library")
	time.Sleep(400 * time.Millisecond)
}

// surveyPhotos returns the throwaway JPEGs this survey uploads. They are scratch files, not
// a job's enumerated assets: the harness is not the shipped upload path.
func surveyPhotos(t *testing.T) []string {
	t.Helper()
	dir := os.Getenv("POSTPILOT_SURVEY_PHOTOS")
	if dir == "" {
		t.Skip("set POSTPILOT_SURVEY_PHOTOS to a directory of small JPEGs")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	photos := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			photos = append(photos, filepath.Join(dir, entry.Name()))
		}
	}
	if len(photos) < 3 {
		t.Fatalf("need at least three JPEGs in %s, found %d", dir, len(photos))
	}
	sort.Strings(photos)
	return photos
}

// TestSurveyT043Images answers the four placement questions T043 cannot be built without.
// A clean draft is required: every answer is about where an image lands relative to the
// blocks around it.
func TestSurveyT043Images(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, true)
	defer s.port.Close()

	// ── (I1) a LEADING image: the caret sits in the editor's own empty leading paragraph
	// and no block precedes the image.
	if err := s.port.resolveAndClick(ctx, "body_end", 0); err != nil {
		t.Fatalf("seat caret in the leading paragraph: %v", err)
	}
	s.dump("I0 empty draft, caret in the leading paragraph")
	s.uploadImage(photos[0])
	s.dump("I1 ANSWER — where a leading image landed, and what is left before it")
	s.dumpModules("I1 module layer of the image component")
	s.dumpProjection()

	// ── (I2) can a paragraph be appended AFTER an image, and what does body_end resolve to?
	var end driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_end", 0}, &end); err != nil {
		t.Fatalf("body_end after an image: %v", err)
	}
	t.Logf("  body_end after the image: matches=%d point=(%.0f,%.0f)", end.Matches, end.X, end.Y)
	if err := s.port.appendParagraph(ctx, "IMG P1 이미지 다음 문단"); err != nil {
		t.Logf("  appendParagraph after an image FAILED: %v", err)
	}
	s.dump("I2 ANSWER — a paragraph appended after an image")

	// ── (I3) the caret in the LAST paragraph of a multi-paragraph component.
	s.enter()
	s.write("IMG P2 둘째 문단")
	s.enter()
	s.write("IMG P3 셋째 문단")
	s.dump("I3 setup — one component holding three paragraphs")
	s.seatCaret()
	s.uploadImage(photos[1])
	s.dump("I3 ANSWER — the caret was in the LAST paragraph")

	// ── (I4) the caret in a MIDDLE paragraph: does the component split at the caret, or
	// does the image land after the whole component? This is the one that decides whether
	// images can be interleaved between text blocks at all.
	middle := -1
	for _, component := range s.last.Components {
		if strings.Contains(component.Classes, "se-text") && !strings.Contains(component.Classes, "se-documentTitle") && len(component.Units) >= 2 {
			middle = component.Index
		}
	}
	if middle < 0 {
		t.Fatal("no multi-paragraph text component to aim at")
	}
	var point driverPoint
	if err := s.port.page.CallFunction(ctx, surveyCaretFn, []any{middle, 0}, &point); err != nil {
		t.Fatalf("seat caret in the first paragraph of component %d: %v", middle, err)
	}
	if point.Matches != 1 {
		t.Fatalf("component %d resolved %d units", middle, point.Matches)
	}
	if err := s.port.page.ClickPoint(ctx, point.X, point.Y); err != nil {
		t.Fatalf("click: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	s.dump("I4 setup — caret in the FIRST of several paragraphs of one component")
	s.uploadImage(photos[2])
	s.dump("I4 ANSWER — did the component split at the caret, or did the image follow it whole?")
	s.dumpProjection()

	t.Log("\nsurvey complete. Nothing was saved and no publish control was resolved.\n" +
		"Discard this draft in the dedicated browser.")
}

// surveyHitTestFn reports what actually sits at one resolved point, and the addressed
// paragraph's own box. A click that resolves cleanly but leaves the caret outside the body
// means something is over the point, and this is the only way to see what.
const surveyHitTestFn = `function (x, y) {
` + bodyParagraphsJS + `  const describe = (node) => {
    if (!node) return {tag: '', classes: '', component: ''};
    const component = node.closest ? node.closest('.se-component') : null;
    return {
      tag: node.tagName || '',
      classes: String(node.className || '').replace(/\s+/g, ' ').trim(),
      component: component ? String(component.className).replace(/\s+/g, ' ').trim() : ''
    };
  };
  const paragraphs = bodyParagraphs();
  const last = paragraphs[paragraphs.length - 1];
  const box = last ? last.getBoundingClientRect() : null;
  const stack = (document.elementsFromPoint ? [...document.elementsFromPoint(x, y)] : []).slice(0, 5);
  return {
    hit: describe(document.elementFromPoint(x, y)),
    stack: stack.map(describe),
    last_box: box ? {left: Math.round(box.left), top: Math.round(box.top), width: Math.round(box.width), height: Math.round(box.height)} : null,
    scroll_y: Math.round(window.scrollY),
    inner_height: window.innerHeight,
    active: describe(document.activeElement)
  };
}`

type surveyHitTest struct {
	Hit struct {
		Tag       string `json:"tag"`
		Classes   string `json:"classes"`
		Component string `json:"component"`
	} `json:"hit"`
	Stack []struct {
		Tag       string `json:"tag"`
		Classes   string `json:"classes"`
		Component string `json:"component"`
	} `json:"stack"`
	LastBox *struct {
		Left   int `json:"left"`
		Top    int `json:"top"`
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"last_box"`
	ScrollY     int `json:"scroll_y"`
	InnerHeight int `json:"inner_height"`
	Active      struct {
		Tag       string `json:"tag"`
		Classes   string `json:"classes"`
		Component string `json:"component"`
	} `json:"active"`
}

// TestSurveyHitTest is read-only. It resolves body_end and reports what is at that point.
func TestSurveyHitTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	var end driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_end", 0}, &end); err != nil {
		t.Fatalf("body_end: %v", err)
	}
	var hit surveyHitTest
	if err := s.port.page.CallFunction(ctx, surveyHitTestFn, []any{end.X, end.Y}, &hit); err != nil {
		t.Fatalf("hit test: %v", err)
	}
	t.Logf("body_end matches=%d point=(%.0f,%.0f)", end.Matches, end.X, end.Y)
	if hit.LastBox != nil {
		t.Logf("last paragraph box left=%d top=%d width=%d height=%d", hit.LastBox.Left, hit.LastBox.Top, hit.LastBox.Width, hit.LastBox.Height)
	}
	t.Logf("scrollY=%d innerHeight=%d", hit.ScrollY, hit.InnerHeight)
	t.Logf("activeElement  <%s.%s> in [%s]", hit.Active.Tag, hit.Active.Classes, hit.Active.Component)
	t.Logf("elementFromPoint <%s.%s> in [%s]", hit.Hit.Tag, hit.Hit.Classes, hit.Hit.Component)
	for index, node := range hit.Stack {
		t.Logf("  stack[%d] <%s.%s> in [%s]", index, node.Tag, node.Classes, node.Component)
	}
}

// surveySidebarFn measures the sidebar the image upload opens and enumerates every control
// inside it that looks like a close affordance, so the driver can either avoid the occluded
// region or close the panel through a versioned control.
const surveySidebarFn = `function () {
  const norm = (v) => String(v == null ? '' : v).replace(/\s+/g, ' ').trim();
  const rect = (node) => {
    if (!node) return null;
    const box = node.getBoundingClientRect();
    return {left: Math.round(box.left), top: Math.round(box.top), width: Math.round(box.width), height: Math.round(box.height)};
  };
  const sidebars = [...document.querySelectorAll('aside.se-sidebar, .se-sidebar')];
  const shown = sidebars.filter((node) => {
    const box = node.getBoundingClientRect();
    return box.width > 0 && box.height > 0 && getComputedStyle(node).visibility !== 'hidden';
  });
  const buttons = [];
  for (const sidebar of shown) {
    for (const button of sidebar.querySelectorAll('button')) {
      const label = norm(button.getAttribute('aria-label') || button.textContent);
      const classes = norm(button.className);
      if (/close|닫기|접기/i.test(label + ' ' + classes)) {
        buttons.push({label, classes, rect: rect(button)});
      }
    }
  }
  const body = document.querySelector('.se-body.__se-body');
  return {
    sidebar_total: sidebars.length,
    sidebar_shown: shown.length,
    sidebar_classes: shown.map((node) => norm(node.className)),
    sidebar_rects: shown.map(rect),
    body_rect: rect(body),
    close_candidates: buttons
  };
}`

// TestSurveySidebar is read-only.
func TestSurveySidebar(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	var dump map[string]any
	if err := s.port.page.CallFunction(ctx, surveySidebarFn, []any{}, &dump); err != nil {
		t.Fatalf("sidebar: %v", err)
	}
	encoded, _ := json.MarshalIndent(dump, "", "  ")
	t.Logf("\n%s", encoded)
}

// TestSurveyT043Placement answers the placement questions that do NOT need a clean draft,
// so a run aborted by the sidebar occlusion can be continued without asking the owner to
// clear the editor again. It closes the sidebar first, since a previous upload leaves it
// over the editor's right edge where every caret point is taken.
func TestSurveyT043Placement(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()

	s.act("close_library")
	time.Sleep(400 * time.Millisecond)
	s.dump("P0 sidebar closed")

	// ── (I2) a paragraph appended AFTER an image, now that the point is reachable.
	if err := s.port.appendParagraph(ctx, "IMG P1 이미지 다음 문단"); err != nil {
		t.Fatalf("appendParagraph after an image: %v", err)
	}
	s.dump("P1 ANSWER (I2) — a paragraph appended after an image")

	// ── (I3) the caret in the LAST paragraph of a multi-paragraph component.
	s.enter()
	s.write("IMG P2 둘째 문단")
	s.enter()
	s.write("IMG P3 셋째 문단")
	s.dump("P2 setup — one component holding three paragraphs")
	tail := s.lastPlainText()
	s.seatComponent(tail)
	s.uploadImage(photos[3])
	s.dump("P3 ANSWER (I3) — the caret was in the LAST paragraph of that component")
	s.dumpProjection()

	// ── (I4) the caret in the FIRST of several paragraphs of one component. This decides
	// whether an image can be interleaved BETWEEN two text blocks at all.
	if err := s.port.appendParagraph(ctx, "IMG P4 중간 대상"); err != nil {
		t.Fatalf("append P4: %v", err)
	}
	s.enter()
	s.write("IMG P5 뒤따르는 문단")
	s.dump("P4 setup — a fresh component with two paragraphs")
	middle := s.lastPlainText()
	var point driverPoint
	if err := s.port.page.CallFunction(ctx, surveyCaretFn, []any{middle, 0}, &point); err != nil {
		t.Fatalf("seat caret in the first paragraph of component %d: %v", middle, err)
	}
	if point.Matches != 1 {
		t.Fatalf("component %d resolved %d units", middle, point.Matches)
	}
	if err := s.port.page.ClickPoint(ctx, point.X, point.Y); err != nil {
		t.Fatalf("click: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	s.dump("P5 setup — caret in the FIRST of two paragraphs")
	s.uploadImage(photos[4])
	s.dump("P6 ANSWER (I4) — did the component split at the caret, or did the image follow it whole?")
	s.dumpProjection()

	t.Log("\nsurvey complete. Nothing was saved and no publish control was resolved.")
}

// TestSurveyT043MiddleParagraph is the one case P6 could not answer: the caret at the END of
// a NON-EMPTY paragraph that has a following paragraph in the same component. That is
// exactly the shape a TEXT, IMAGE, TEXT manifest reaches after the write pass, so whether
// SmartEditor splits the component here decides whether images can be interleaved at all.
func TestSurveyT043MiddleParagraph(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	s.act("close_library")
	time.Sleep(400 * time.Millisecond)
	s.dump("M0 current draft")

	// Find a component whose FIRST unit is non-empty and which has at least two units.
	target, unit := -1, -1
	for _, component := range s.last.Components {
		if !strings.Contains(component.Classes, "se-text") || strings.Contains(component.Classes, "se-documentTitle") {
			continue
		}
		for index, candidate := range component.Units {
			if strings.TrimSpace(candidate.Text) != "" && index+1 < len(component.Units) && strings.TrimSpace(component.Units[index+1].Text) != "" {
				target, unit = component.Index, index
				break
			}
		}
		if target >= 0 {
			break
		}
	}
	if target < 0 {
		t.Fatal("no component holds a non-empty paragraph followed by another non-empty one")
	}
	// Address it through the SHIPPED body_paragraph target, which reveals the element and
	// hit-tests the point. The harness's own caret helper does neither, and on a document
	// this long it clicked the sticky toolbar instead of the paragraph.
	global := 0
	for _, component := range s.last.Components {
		if !strings.Contains(component.Classes, "se-text") && !strings.Contains(component.Classes, "se-sectionTitle") && !strings.Contains(component.Classes, "se-quotation") {
			continue
		}
		if strings.Contains(component.Classes, "se-documentTitle") {
			continue
		}
		if component.Index == target {
			global += unit
			break
		}
		global += len(component.Units)
	}
	t.Logf("aiming at component[%d] unit[%d] = body_paragraph %d", target, unit, global)
	var point driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"body_paragraph", global}, &point); err != nil {
		t.Fatalf("seat caret: %v", err)
	}
	if point.Matches != 1 {
		t.Fatalf("resolved %d units", point.Matches)
	}
	if err := s.port.page.ClickPoint(ctx, point.X, point.Y); err != nil {
		t.Fatalf("click: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	s.dump("M1 setup — caret at the end of a non-empty paragraph with another after it")
	s.uploadImage(photos[5])
	s.dump("M2 ANSWER — where the image landed relative to that paragraph")
	s.dumpProjection()
}

// TestSurveyLiveBodyPath drives the SHIPPED Apply path against the live editor for a mini
// manifest that exercises every body kind plus one interleaved photo, then compares the
// projection. It is the live counterpart of the fake-CDP tests: the fakes prove the driver's
// shape, this proves the shape matches SmartEditor.
//
// It REQUIRES a clean draft. An earlier attempt ran against a draft polluted by previous
// survey passes, whose leftover list swallowed every later paragraph as another list item —
// the trap T042 documented — and the failure looked like a driver defect when it was the
// draft. In the real flow no write ever follows a conversion, so a clean start is also the
// only faithful one.
func TestSurveyLiveBodyPath(t *testing.T) {
	photo := os.Getenv("POSTPILOT_SURVEY_PHOTOS")
	if photo == "" {
		t.Skip("set POSTPILOT_SURVEY_PHOTOS")
	}
	photo = filepath.Join(photo, "live.jpg")
	if _, err := os.Stat(photo); err != nil {
		t.Skipf("no live.jpg in the survey photo directory: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, true)
	defer s.port.Close()
	port := s.port

	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close the library first: %v", err)
	}
	base, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	written, err := port.paragraphState(ctx, "body_paragraph", -1)
	if err != nil {
		t.Fatalf("paragraph count: %v", err)
	}
	offset := written.Total
	if written.Matches == 0 {
		offset = 0
	}
	t.Logf("starting from %d body blocks and %d written paragraphs; %d images",
		len(base.Body), offset, base.ImageCount)

	// The write pass, in manifest order, with the photo where the manifest puts it.
	apply := func(mutation Mutation) {
		t.Helper()
		if err := port.Apply(ctx, mutation); err != nil {
			t.Fatalf("apply %s: %v", mutation.Kind, err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	const firstBlock = "LIVE T1 첫 문단"
	apply(Mutation{Kind: MutationText, Text: firstBlock})
	// The very first block goes into SmartEditor's own opening paragraph, the one carrying
	// the writer's placeholder. A clean run on 260910 read that paragraph back SHORT
	// ("LIVE T1 첫 "), while typing into a post-image empty paragraph and into an existing
	// one both read back exact — so this is asserted on its own, before anything else can
	// confuse the diagnosis.
	afterFirst, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe after the first write: %v", err)
	}
	if len(afterFirst.Body) != len(base.Body)+1 {
		t.Fatalf("the first write produced %d blocks, want one: %+v", len(afterFirst.Body)-len(base.Body), afterFirst.Body)
	}
	if got := afterFirst.Body[len(base.Body)].Text; got != firstBlock {
		var dump map[string]any
		_ = port.page.CallFunction(ctx, surveyParagraphTextFn, []any{}, &dump)
		encoded, _ := json.Marshal(dump)
		t.Fatalf("the first write into the editor's own opening paragraph read back %q, want %q\nparagraph: %s", got, firstBlock, encoded)
	}
	t.Logf("the first write read back exact: %q", firstBlock)
	apply(Mutation{Kind: MutationUploadImage, Ordinal: base.ImageCount, AssetPath: photo})
	// A caption addresses its image by DOCUMENT ordinal. In a real run that equals the
	// upload ordinal, because photos are uploaded in manifest order into a document built
	// in the same order — but this test appends to a draft whose existing images sit after
	// the insertion point, so the ordinal is read back from the projection instead.
	afterUpload, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe after the upload: %v", err)
	}
	newOrdinal := -1
	for index, block := range afterUpload.Body {
		if block.Kind == SemanticText && block.Text == "LIVE T1 첫 문단" && index+1 < len(afterUpload.Body) && afterUpload.Body[index+1].Kind == SemanticImage {
			newOrdinal = afterUpload.Body[index+1].Ordinal
		}
	}
	if newOrdinal < 0 {
		t.Fatalf("the photo did not land immediately after the paragraph it follows: %+v", afterUpload.Body)
	}
	t.Logf("the photo landed at document ordinal %d, immediately after its paragraph", newOrdinal)
	apply(Mutation{Kind: MutationImageCaption, Ordinal: newOrdinal, Text: "LIVE 캡션"})
	apply(Mutation{Kind: MutationText, Text: "LIVE T2 사진 다음"})
	apply(Mutation{Kind: MutationHeading, Text: "LIVE H1 소제목", Level: 2})
	apply(Mutation{Kind: MutationText, Text: "LIVE Q1 인용 대상"})
	apply(Mutation{Kind: MutationText, Text: "LIVE L1 항목 하나"})
	s.dump("LIVE write pass complete")

	// The conversion pass, backwards.
	apply(Mutation{Kind: MutationList, Ordinal: offset + 4, Items: []string{"LIVE L1 항목 하나", "LIVE L2 항목 둘", "LIVE L3 항목 셋"}})
	apply(Mutation{Kind: MutationQuote, Text: "LIVE Q1 인용 대상", Ordinal: offset + 3})
	s.dump("LIVE conversion pass complete")

	final, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("final observe: %v", err)
	}
	s.dumpProjection()
	added := final.Body[len(base.Body):]
	if len(added) > 0 && added[0].Kind != SemanticText {
		t.Fatalf("the first block this test added is %+v", added[0])
	}
	want := []SemanticBlock{
		{Kind: SemanticText, Text: "LIVE T1 첫 문단"},
		{Kind: SemanticImage, Ordinal: newOrdinal, Caption: "LIVE 캡션", Uploaded: true},
		{Kind: SemanticText, Text: "LIVE T2 사진 다음"},
		{Kind: SemanticText, Text: "LIVE H1 소제목"},
		{Kind: SemanticText, Text: "“LIVE Q1 인용 대상”"},
		{Kind: SemanticText, Text: "- LIVE L1 항목 하나\n- LIVE L2 항목 둘\n- LIVE L3 항목 셋"},
	}
	if len(added) != len(want) {
		t.Fatalf("the body gained %d blocks, want %d: %+v", len(added), len(want), added)
	}
	for index := range want {
		if added[index].Kind != want[index].Kind || added[index].Text != want[index].Text ||
			added[index].Caption != want[index].Caption || added[index].Uploaded != want[index].Uploaded {
			t.Fatalf("added[%d] = %+v, want %+v", index, added[index], want[index])
		}
	}
	if final.ImageCount != base.ImageCount+1 {
		t.Fatalf("image count went %d → %d", base.ImageCount, final.ImageCount)
	}
	t.Log("\nLIVE: the shipped body path reproduced the manifest exactly, photo interleaved.")
}

// surveyCaptionProbeFn reports why a caption module's point does or does not resolve.
const surveyCaptionProbeFn = `function (ordinal) {
  const describe = (node) => {
    if (!node) return {tag: '', classes: '', component: ''};
    const component = node.closest ? node.closest('.se-component') : null;
    return {
      tag: node.tagName || '',
      classes: String(node.className || '').replace(/\s+/g, ' ').trim(),
      component: component ? String(component.className).replace(/\s+/g, ' ').trim() : ''
    };
  };
  const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
  const image = images[ordinal];
  if (!image) return {images: images.length, found: false};
  const captions = image.querySelectorAll('.se-module-text.se-caption');
  if (captions.length !== 1) return {images: images.length, found: true, captions: captions.length};
  const caption = captions[0];
  const box = caption.getBoundingClientRect();
  const x = Math.round(box.left + box.width / 2);
  const y = Math.round(box.top + box.height / 2);
  return {
    images: images.length, found: true, captions: 1,
    rect: {left: Math.round(box.left), top: Math.round(box.top), width: Math.round(box.width), height: Math.round(box.height)},
    inner_height: window.innerHeight,
    hit: describe(document.elementFromPoint(x, y)),
    same_component: Boolean(document.elementFromPoint(x, y) && document.elementFromPoint(x, y).closest('.se-component') === image)
  };
}`

// TestSurveyCaptionProbe is read-only.
func TestSurveyCaptionProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	ordinal := 0
	if value := os.Getenv("POSTPILOT_SURVEY_ORDINAL"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("bad ordinal: %v", err)
		}
		ordinal = parsed
	}
	var probe map[string]any
	if err := s.port.page.CallFunction(ctx, surveyCaptionProbeFn, []any{ordinal}, &probe); err != nil {
		t.Fatalf("caption probe: %v", err)
	}
	encoded, _ := json.MarshalIndent(probe, "", "  ")
	t.Logf("caption %d:\n%s", ordinal, encoded)
	var point driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"image_caption", ordinal}, &point); err != nil {
		t.Fatalf("driver point: %v", err)
	}
	t.Logf("driverPointFn(image_caption, %d) = matches %d at (%.0f,%.0f)", ordinal, point.Matches, point.X, point.Y)
}

// surveyImagePointFn returns the point of one image's own module, so a survey can select the
// image and see whether that is what gives its empty caption module a box.
const surveyImagePointFn = `function (ordinal) {
  const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
  const image = images[ordinal];
  if (!image) return {matches: 0, x: 0, y: 0};
  const module = image.querySelector('.se-module-image') || image;
  module.scrollIntoView({block: 'center', inline: 'nearest'});
  const box = module.getBoundingClientRect();
  if (box.width <= 0 || box.height <= 0) return {matches: 0, x: 0, y: 0};
  const x = Math.round(box.left + box.width / 2);
  const y = Math.round(box.top + box.height / 2);
  const at = document.elementFromPoint(x, y);
  if (!at || at.closest('.se-component') !== image) return {matches: 0, x: 0, y: 0};
  return {matches: 1, x, y};
}`

// TestSurveyCaptionAfterSelectingTheImage asks what gives an empty caption module a box.
func TestSurveyCaptionAfterSelectingTheImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	if err := s.port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close library: %v", err)
	}
	ordinal := 0
	if value := os.Getenv("POSTPILOT_SURVEY_ORDINAL"); value != "" {
		ordinal, _ = strconv.Atoi(value)
	}
	probe := func(label string) {
		var dump map[string]any
		if err := s.port.page.CallFunction(ctx, surveyCaptionProbeFn, []any{ordinal}, &dump); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		encoded, _ := json.Marshal(dump)
		t.Logf("%-22s %s", label, encoded)
		var point driverPoint
		if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"image_caption", ordinal}, &point); err != nil {
			t.Fatalf("%s point: %v", label, err)
		}
		t.Logf("%-22s driverPointFn(image_caption) matches=%d", label, point.Matches)
	}
	probe("before selecting")

	var image driverPoint
	if err := s.port.page.CallFunction(ctx, surveyImagePointFn, []any{ordinal}, &image); err != nil {
		t.Fatalf("image point: %v", err)
	}
	t.Logf("image %d point matches=%d (%.0f,%.0f)", ordinal, image.Matches, image.X, image.Y)
	if image.Matches != 1 {
		t.Fatal("the image module did not resolve")
	}
	if err := s.port.page.ClickPoint(ctx, image.X, image.Y); err != nil {
		t.Fatalf("click the image: %v", err)
	}
	time.Sleep(600 * time.Millisecond)
	probe("after selecting")

	// If the caption is now reachable, prove it takes text.
	var point driverPoint
	if err := s.port.page.CallFunction(ctx, driverPointFn, []any{"image_caption", ordinal}, &point); err != nil {
		t.Fatalf("point: %v", err)
	}
	if point.Matches == 1 {
		if err := s.port.Apply(ctx, Mutation{Kind: MutationImageCaption, Ordinal: ordinal, Text: "선택 후 캡션"}); err != nil {
			t.Fatalf("caption after selecting: %v", err)
		}
		time.Sleep(400 * time.Millisecond)
		s.dumpProjection()
	}
}

// TestSurveyCaptionRoundTrip selects one image, captions it and reads the module back.
func TestSurveyCaptionRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	if err := s.port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close library: %v", err)
	}
	ordinal := 0
	if value := os.Getenv("POSTPILOT_SURVEY_ORDINAL"); value != "" {
		ordinal, _ = strconv.Atoi(value)
	}
	var image driverPoint
	if err := s.port.page.CallFunction(ctx, surveyImagePointFn, []any{ordinal}, &image); err != nil || image.Matches != 1 {
		t.Fatalf("image point: %v matches=%d", err, image.Matches)
	}
	if err := s.port.page.ClickPoint(ctx, image.X, image.Y); err != nil {
		t.Fatalf("select the image: %v", err)
	}
	time.Sleep(600 * time.Millisecond)
	if err := s.port.Apply(ctx, Mutation{Kind: MutationImageCaption, Ordinal: ordinal, Text: "라운드트립 캡션"}); err != nil {
		t.Fatalf("caption: %v", err)
	}
	time.Sleep(600 * time.Millisecond)
	s.dumpModules("caption round trip — the image component's modules")
	s.dumpProjection()
}

// surveySelectionFn reports exactly where the caret sits inside its paragraph, so a survey
// can tell which step of a sequence moved it.
const surveySelectionFn = `function () {
  const sel = window.getSelection();
  if (!sel || !sel.anchorNode) return {found: false};
  const node = sel.anchorNode.nodeType === 1 ? sel.anchorNode : sel.anchorNode.parentElement;
  const paragraph = node.closest ? node.closest('p.se-text-paragraph, li.se-text-list-item') : null;
  const text = paragraph ? [...paragraph.querySelectorAll('span.__se-node')].map((n) => n.textContent).join('') : '';
  // Where the caret is in the paragraph's own text, counted across its text nodes.
  let before = 0;
  if (paragraph && sel.anchorNode.nodeType === 3) {
    const walker = document.createTreeWalker(paragraph, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) {
      if (walker.currentNode === sel.anchorNode) { before += sel.anchorOffset; break; }
      before += walker.currentNode.textContent.length;
    }
  }
  const box = paragraph ? paragraph.getBoundingClientRect() : null;
  let textRight = 0;
  if (paragraph) {
    const range = document.createRange();
    range.selectNodeContents(paragraph);
    const rects = [...range.getClientRects()];
    textRight = rects.length ? Math.round(rects[rects.length - 1].right) : 0;
  }
  return {
    found: true,
    node_type: sel.anchorNode.nodeType,
    anchor_offset: sel.anchorOffset,
    caret_in_text: before,
    text,
    text_length: text.length,
    paragraph_box: box ? {left: Math.round(box.left), width: Math.round(box.width), top: Math.round(box.top), height: Math.round(box.height)} : null,
    text_right: textRight,
    lines: paragraph ? [...(() => { const r = document.createRange(); r.selectNodeContents(paragraph); return r.getClientRects(); })()].length : 0
  };
}`

// TestSurveyCaretDrift finds which step of the upload sequence moves the caret mid-text.
func TestSurveyCaretDrift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close library: %v", err)
	}
	report := func(label string) {
		var dump map[string]any
		if err := port.page.CallFunction(ctx, surveySelectionFn, []any{}, &dump); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		encoded, _ := json.Marshal(dump)
		t.Logf("%-28s %s", label, encoded)
	}
	if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: "DRIFT 문단 하나 둘 셋"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	report("after the write")

	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	report("after close_library")

	var end driverPoint
	if err := port.page.CallFunction(ctx, driverPointFn, []any{"body_end", 0}, &end); err != nil {
		t.Fatalf("body_end: %v", err)
	}
	t.Logf("body_end resolved matches=%d at (%.0f,%.0f)", end.Matches, end.X, end.Y)
	if err := port.page.ClickPoint(ctx, end.X, end.Y); err != nil {
		t.Fatalf("click: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	report("after the body_end click")

	var button driverPoint
	if err := port.page.CallFunction(ctx, driverPointFn, []any{"image_add", 0}, &button); err != nil {
		t.Fatalf("image_add: %v", err)
	}
	t.Logf("image_add resolved matches=%d at (%.0f,%.0f)", button.Matches, button.X, button.Y)
}

// TestSurveyUploadSplit reproduces the exact upload sequence step by step and reports the
// selection before the file is handed over, which is the only moment left that could put the
// caret mid-text.
func TestSurveyUploadSplit(t *testing.T) {
	dir := os.Getenv("POSTPILOT_SURVEY_PHOTOS")
	if dir == "" {
		t.Skip("set POSTPILOT_SURVEY_PHOTOS")
	}
	photo := filepath.Join(dir, "live.jpg")
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	report := func(label string) {
		var dump map[string]any
		if err := port.page.CallFunction(ctx, surveySelectionFn, []any{}, &dump); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		encoded, _ := json.Marshal(dump)
		t.Logf("%-30s %s", label, encoded)
	}
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: "SPLIT 하나 둘 셋 넷"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	report("1 after the write")

	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
		t.Fatalf("body_end: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	report("2 after the body_end click")

	if err := port.page.InterceptFileChooser(ctx); err != nil {
		t.Fatalf("intercept: %v", err)
	}
	report("3 after enabling interception")

	if err := port.resolveAndClick(ctx, "image_add", 0); err != nil {
		t.Fatalf("image_add: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	report("4 after the image button click")

	if err := port.page.SetFileInputFiles(ctx, hiddenFileInput, []string{photo}); err != nil {
		t.Fatalf("set files: %v", err)
	}
	time.Sleep(2500 * time.Millisecond)
	report("5 after the upload")
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	s.dump("6 the result")
	s.dumpProjection()
}

// surveyParagraphTextFn shows how the last body paragraph is actually built: which of its
// children SmartEditor has wrapped in a `__se-node` span and which are still raw. The
// projection reads only `__se-node` spans, so anything unwrapped is invisible to it.
const surveyParagraphTextFn = `function () {
` + bodyParagraphsJS + `  const paragraphs = bodyParagraphs();
  const paragraph = paragraphs[paragraphs.length - 1];
  if (!paragraph) return {found: false};
  const children = [...paragraph.childNodes].map((node) => ({
    type: node.nodeType,
    tag: node.tagName || '',
    classes: String(node.className || '').replace(/\s+/g, ' ').trim(),
    se_node: node.nodeType === 1 && node.classList.contains('__se-node'),
    text: node.textContent
  }));
  const nodeText = [...paragraph.querySelectorAll('span.__se-node')].map((n) => n.textContent).join('');
  return {
    found: true,
    node_text: nodeText,
    text_content: paragraph.textContent,
    equal: nodeText === paragraph.textContent,
    children
  };
}`

// TestSurveyProjectionLag reads the last paragraph immediately after a write and again after
// a pause, to see whether the projection can read a freshly typed paragraph short.
func TestSurveyProjectionLag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	report := func(label string) {
		var dump map[string]any
		if err := port.page.CallFunction(ctx, surveyParagraphTextFn, []any{}, &dump); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		encoded, _ := json.Marshal(dump)
		t.Logf("%-24s %s", label, encoded)
	}
	if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: "LAG 하나 둘 셋 문단"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	report("immediately")
	time.Sleep(200 * time.Millisecond)
	report("after 200ms")
	time.Sleep(400 * time.Millisecond)
	report("after 600ms")
	time.Sleep(1200 * time.Millisecond)
	report("after 1.8s")
	snapshot, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if len(snapshot.Body) > 0 {
		t.Logf("projection's last block: %q", snapshot.Body[len(snapshot.Body)-1].Text)
	}
}

// TestSurveyEmptyTargetWrite exercises the branch that types straight into an already-empty
// paragraph instead of pressing Enter — the branch a clean editor always takes on its first
// block, and the one under suspicion for losing the tail of the inserted text.
func TestSurveyEmptyTargetWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	before, err := port.paragraphState(ctx, "body_end", -1)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	t.Logf("append target: matches=%d empty=%t kind=%q", before.Matches, before.Empty, before.Kind)
	if !before.Empty {
		t.Skip("the draft's last paragraph is not empty; run this right after an image upload")
	}
	const text = "EMPTY 하나 둘 셋 문단"
	if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: text}); err != nil {
		t.Fatalf("write: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	var dump map[string]any
	if err := port.page.CallFunction(ctx, surveyParagraphTextFn, []any{}, &dump); err != nil {
		t.Fatalf("read back: %v", err)
	}
	encoded, _ := json.Marshal(dump)
	t.Logf("after the empty-target write: %s", encoded)
	snapshot, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	last := snapshot.Body[len(snapshot.Body)-1]
	t.Logf("projection's last block: %q (want %q)", last.Text, text)
	if last.Text != text {
		t.Fatalf("the empty-target write lost text: got %q, want %q", last.Text, text)
	}
}

// surveyCaretMapFn asks the browser which caret position the driver's own trailing-edge
// point maps to, and compares it with the paragraph's text length. If they differ, the
// 98 %-width rule is landing mid-text rather than at the end.
const surveyCaretMapFn = `function (index) {
` + bodyParagraphsJS + `  const paragraphs = bodyParagraphs();
  const paragraph = paragraphs[index < 0 ? paragraphs.length + index : index];
  if (!paragraph) return {found: false, total: paragraphs.length};
  paragraph.scrollIntoView({block: 'center', inline: 'nearest'});
  const box = paragraph.getBoundingClientRect();
  const text = [...paragraph.querySelectorAll('span.__se-node')].map((n) => n.textContent).join('');
  const probe = (x, y) => {
    const at = document.elementFromPoint(x, y);
    let offset = -1;
    let container = '';
    if (document.caretRangeFromPoint) {
      const range = document.caretRangeFromPoint(x, y);
      if (range) {
        offset = range.startOffset;
        container = range.startContainer.textContent || '';
      }
    }
    return {x: Math.round(x), y: Math.round(y), hit: at ? String(at.className || at.tagName) : '', offset, container};
  };
  const range = document.createRange();
  range.selectNodeContents(paragraph);
  const rects = [...range.getClientRects()].map((r) => ({left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), bottom: Math.round(r.bottom)}));
  return {
    found: true,
    text,
    text_length: text.length,
    box: {left: Math.round(box.left), right: Math.round(box.right), width: Math.round(box.width), top: Math.round(box.top), bottom: Math.round(box.bottom), height: Math.round(box.height)},
    text_rects: rects,
    at_98: probe(box.left + box.width * 0.98, box.bottom - box.height * 0.25),
    at_centre: probe(box.left + box.width / 2, box.top + box.height / 2),
    at_text_end: rects.length ? probe(rects[rects.length - 1].right + 4, (rects[rects.length - 1].top + rects[rects.length - 1].bottom) / 2) : null
  };
}`

// TestSurveyCaretMap is read-only.
func TestSurveyCaretMap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	if err := s.port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	var dump map[string]any
	index := -1
	if value := os.Getenv("POSTPILOT_SURVEY_ORDINAL"); value != "" {
		index, _ = strconv.Atoi(value)
	}
	if err := s.port.page.CallFunction(ctx, surveyCaretMapFn, []any{index}, &dump); err != nil {
		t.Fatalf("caret map: %v", err)
	}
	encoded, _ := json.MarshalIndent(dump, "", "  ")
	t.Logf("\n%s", encoded)
}

// TestSurveyTailLoss isolates which step of the upload sequence drops the tail of a freshly
// written paragraph. It never uploads anything: it writes, reads back, clicks the image
// button, and reads back again.
func TestSurveyTailLoss(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	read := func(label string) string {
		var dump struct {
			NodeText string `json:"node_text"`
		}
		if err := port.page.CallFunction(ctx, surveyParagraphTextFn, []any{}, &dump); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		t.Logf("%-34s %q", label, dump.NodeText)
		return dump.NodeText
	}
	for _, text := range []string{"TRUNC 하나 둘 문단", "TRUNC ASCII TAIL"} {
		t.Run(text, func(t *testing.T) {
			if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: text}); err != nil {
				t.Fatalf("write: %v", err)
			}
			time.Sleep(400 * time.Millisecond)
			if got := read("1 after the write"); got != text {
				t.Fatalf("the write itself lost text: %q", got)
			}
			if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
				t.Fatalf("body_end: %v", err)
			}
			time.Sleep(300 * time.Millisecond)
			read("2 after the body_end click")
			if err := port.page.InterceptFileChooser(ctx); err != nil {
				t.Fatalf("intercept: %v", err)
			}
			read("3 after enabling interception")
			if err := port.resolveAndClick(ctx, "image_add", 0); err != nil {
				t.Fatalf("image_add: %v", err)
			}
			time.Sleep(600 * time.Millisecond)
			got := read("4 after the image button click")
			if got != text {
				t.Errorf("the image button click dropped the tail: %q, want %q", got, text)
			}
		})
	}
}

// TestSurveyEmptyThenUpload is the reproduction that needs no clean draft: an upload always
// leaves a trailing EMPTY paragraph, so every iteration takes the same no-Enter write branch
// the editor's own opening paragraph takes, and then uploads. It reports how much of each
// string survives.
func TestSurveyEmptyThenUpload(t *testing.T) {
	dir := os.Getenv("POSTPILOT_SURVEY_PHOTOS")
	if dir == "" {
		t.Skip("set POSTPILOT_SURVEY_PHOTOS")
	}
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	read := func() string {
		var dump struct {
			NodeText string `json:"node_text"`
		}
		if err := port.page.CallFunction(ctx, surveyParagraphTextFn, []any{}, &dump); err != nil {
			t.Fatalf("read: %v", err)
		}
		return dump.NodeText
	}
	texts := []string{"ABCDEFGHIJKL", "가나다라마바사아", "MIX 하나 둘 문단"}
	for index, text := range texts {
		state, err := port.paragraphState(ctx, "body_end", -1)
		if err != nil {
			t.Fatalf("state: %v", err)
		}
		t.Logf("── %q: append target empty=%t", text, state.Empty)
		if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: text}); err != nil {
			t.Fatalf("write %q: %v", text, err)
		}
		time.Sleep(400 * time.Millisecond)
		wrote := read()
		if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photos[index%len(photos)]}); err != nil {
			t.Fatalf("upload after %q: %v", text, err)
		}
		time.Sleep(600 * time.Millisecond)
		after, err := port.Observe(ctx)
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		t.Logf("   wrote %q; the body after the upload:", wrote)
		for position, block := range after.Body {
			t.Logf("      [%d] %-6s %q", position, block.Kind, block.Text)
		}
	}
}

// surveyAllParagraphsFn compares, for every body paragraph, what the projection reads
// (`span.__se-node` only) against the paragraph's real textContent, and lists its children.
// If they disagree, the projection is losing text the editor still holds.
const surveyAllParagraphsFn = `function () {
` + bodyParagraphsJS + `  return bodyParagraphs().map((paragraph, index) => {
    const nodeText = [...paragraph.querySelectorAll('span.__se-node')].map((n) => n.textContent).join('');
    return {
      index,
      node_text: nodeText,
      text_content: paragraph.textContent,
      agree: nodeText === paragraph.textContent,
      children: [...paragraph.childNodes].map((node) => ({
        type: node.nodeType,
        tag: node.tagName || '',
        classes: String(node.className || '').replace(/\s+/g, ' ').trim(),
        se_node: node.nodeType === 1 && node.classList.contains('__se-node'),
        text: node.textContent
      }))
    };
  });
}`

// TestSurveyProjectionVsDOM is read-only.
func TestSurveyProjectionVsDOM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	if err := s.port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	var dump []struct {
		Index       int    `json:"index"`
		NodeText    string `json:"node_text"`
		TextContent string `json:"text_content"`
		Agree       bool   `json:"agree"`
		Children    []struct {
			Type    int    `json:"type"`
			Tag     string `json:"tag"`
			Classes string `json:"classes"`
			SeNode  bool   `json:"se_node"`
			Text    string `json:"text"`
		} `json:"children"`
	}
	if err := s.port.page.CallFunction(ctx, surveyAllParagraphsFn, []any{}, &dump); err != nil {
		t.Fatalf("paragraphs: %v", err)
	}
	for _, paragraph := range dump {
		mark := "  "
		if !paragraph.Agree {
			mark = "!!"
		}
		t.Logf("%s [%d] projection %q  vs  DOM %q", mark, paragraph.Index, paragraph.NodeText, paragraph.TextContent)
		if paragraph.Agree {
			continue
		}
		for _, child := range paragraph.Children {
			t.Logf("        child type=%d <%s.%s> se_node=%t %q", child.Type, child.Tag, child.Classes, child.SeNode, child.Text)
		}
	}
}

// TestSurveyEnterCommitsTheWord tests the two write variants against the same empty target:
// insertText alone, and Enter-then-insertText. The upload after each shows which one leaves
// the trailing word committed.
func TestSurveyEnterCommitsTheWord(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	survivor := func(text string) string {
		snapshot, err := port.Observe(ctx)
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		for _, block := range snapshot.Body {
			if block.Kind == SemanticText && block.Text == text {
				return block.Text
			}
		}
		best := ""
		for _, block := range snapshot.Body {
			if block.Kind == SemanticText && strings.HasPrefix(text, block.Text) && len(block.Text) > len(best) {
				best = block.Text
			}
		}
		return best
	}
	run := func(label, text string, pressEnter bool, photo string) {
		state, err := port.paragraphState(ctx, "body_end", -1)
		if err != nil {
			t.Fatalf("state: %v", err)
		}
		if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
			t.Fatalf("click: %v", err)
		}
		if pressEnter {
			if err := port.page.PressEnter(ctx); err != nil {
				t.Fatalf("enter: %v", err)
			}
		}
		if err := port.typeText(ctx, text); err != nil {
			t.Fatalf("type: %v", err)
		}
		time.Sleep(400 * time.Millisecond)
		if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photo}); err != nil {
			t.Fatalf("upload: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		got := survivor(text)
		verdict := "INTACT"
		if got != text {
			verdict = "LOST THE TAIL"
		}
		t.Logf("%-28s target empty=%t enter=%t → %q [%s]", label, state.Empty, pressEnter, got, verdict)
	}
	run("insertText alone", "AAA BBB CCCWORD", false, photos[0])
	run("Enter then insertText", "DDD EEE FFFWORD", true, photos[1])

	// Enter AFTER the text: it commits the pending word and leaves the caret in a fresh
	// empty paragraph, which the image then follows — so the photo still lands immediately
	// after its block, with one empty paragraph between them that the projection drops.
	trailing := func(label, text, photo string) {
		if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
			t.Fatalf("click: %v", err)
		}
		if err := port.page.PressEnter(ctx); err != nil {
			t.Fatalf("enter: %v", err)
		}
		if err := port.typeText(ctx, text); err != nil {
			t.Fatalf("type: %v", err)
		}
		if err := port.page.PressEnter(ctx); err != nil {
			t.Fatalf("trailing enter: %v", err)
		}
		time.Sleep(400 * time.Millisecond)
		if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photo}); err != nil {
			t.Fatalf("upload: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		got := survivor(text)
		verdict := "INTACT"
		if got != text {
			verdict = "LOST THE TAIL"
		}
		t.Logf("%-28s → %q [%s]", label, got, verdict)
	}
	trailing("text then trailing Enter", "GGG HHH IIIWORD", photos[2])

	// Two paragraphs in one component, uploading at the end of the second: the shape that
	// read back intact on 260910 before the empty-target write was introduced.
	if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
		t.Fatalf("click: %v", err)
	}
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := port.typeText(ctx, "JJJ KKK FIRSTWORD"); err != nil {
		t.Fatalf("type: %v", err)
	}
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := port.typeText(ctx, "LLL MMM SECONDWORD"); err != nil {
		t.Fatalf("type: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photos[3]}); err != nil {
		t.Fatalf("upload: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	t.Logf("%-28s first=%q second=%q", "two paragraphs, upload last",
		survivor("JJJ KKK FIRSTWORD"), survivor("LLL MMM SECONDWORD"))
}

// TestSurveyListTailCommit asks how a list's LAST item can have its pending word committed.
// A trailing Enter commits it but opens another list item, so the question is what the
// editor does with an empty one and whether a second Enter leaves the list.
func TestSurveyListTailCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	show := func(label string) {
		snapshot, err := port.Observe(ctx)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		last := ""
		if len(snapshot.Body) > 0 {
			last = snapshot.Body[len(snapshot.Body)-1].Text
		}
		t.Logf("%-26s blocks=%d last=%q", label, len(snapshot.Body), last)
	}
	// Build a list the shipped way: write item one as a paragraph, convert, then Enter+type.
	if err := port.Apply(ctx, Mutation{Kind: MutationText, Text: "LIST ONE ALPHA"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	written, err := port.paragraphState(ctx, "body_paragraph", -1)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	index := written.Total - 1
	if err := port.Apply(ctx, Mutation{Kind: MutationList, Ordinal: index, Items: []string{"LIST ONE ALPHA", "LIST TWO BETA", "LIST THREE GAMMA"}}); err != nil {
		t.Fatalf("list: %v", err)
	}
	show("after the list")

	// One Enter: commits the last item's word and opens another item.
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("enter: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	show("after one Enter")

	// A second Enter on the empty item: does the editor drop it and leave the list?
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("enter: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	show("after two Enters")
	s.dumpProjection()
}

// TestSurveyCommitIsAKeyNotAWait decides the fix. If waiting commits the pending word, the
// answer is a settle wait after every write. If only a key commits it, the answer is a
// trailing key — which then has to be arranged separately for the title, a caption and a
// list's last item.
func TestSurveyCommitIsAKeyNotAWait(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	survivor := func(text string) string {
		snapshot, err := port.Observe(ctx)
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		best := ""
		for _, block := range snapshot.Body {
			if block.Kind == SemanticText && strings.HasPrefix(text, block.Text) && len(block.Text) > len(best) {
				best = block.Text
			}
		}
		return best
	}
	try := func(label, text string, wait time.Duration, photo string) {
		if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
			t.Fatalf("click: %v", err)
		}
		if err := port.page.PressEnter(ctx); err != nil {
			t.Fatalf("enter: %v", err)
		}
		if err := port.typeText(ctx, text); err != nil {
			t.Fatalf("type: %v", err)
		}
		time.Sleep(wait)
		if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photo}); err != nil {
			t.Fatalf("upload: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		got := survivor(text)
		verdict := "INTACT"
		if got != text {
			verdict = "LOST THE TAIL"
		}
		t.Logf("%-34s waited %-6s → %q [%s]", label, wait, got, verdict)
	}
	try("no wait", "WAIT AAA ZEROWORD", 0, photos[0])
	try("waited 3s", "WAIT BBB THREEWORD", 3*time.Second, photos[1])
	try("waited 8s", "WAIT CCC EIGHTWORD", 8*time.Second, photos[2])
}

// TestSurveyCommitRace measures it instead of theorising: the same write-then-upload shape,
// repeated, with and without a trailing Enter. A pending trailing word is lost only when the
// image insertion beats SmartEditor's own commit tick, so the question is not whether the
// loss happens but how reliably a trailing Enter prevents it.
func TestSurveyCommitRace(t *testing.T) {
	photos := surveyPhotos(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	survivor := func(text string) string {
		snapshot, err := port.Observe(ctx)
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		best := ""
		for _, block := range snapshot.Body {
			if block.Kind == SemanticText && strings.HasPrefix(text, block.Text) && len(block.Text) > len(best) {
				best = block.Text
			}
		}
		return best
	}
	round := func(label string, trailingEnter bool, rounds int) {
		lost := 0
		for index := 0; index < rounds; index++ {
			text := fmt.Sprintf("RACE %s %02d TAILWORD", label, index)
			if err := port.resolveAndClick(ctx, "body_end", 0); err != nil {
				t.Fatalf("click: %v", err)
			}
			if err := port.page.PressEnter(ctx); err != nil {
				t.Fatalf("enter: %v", err)
			}
			if err := port.typeText(ctx, text); err != nil {
				t.Fatalf("type: %v", err)
			}
			if trailingEnter {
				if err := port.page.PressEnter(ctx); err != nil {
					t.Fatalf("trailing enter: %v", err)
				}
			}
			if err := port.Apply(ctx, Mutation{Kind: MutationUploadImage, AssetPath: photos[index%len(photos)]}); err != nil {
				t.Fatalf("upload: %v", err)
			}
			if got := survivor(text); got != text {
				lost++
				t.Logf("   %s round %d LOST: %q", label, index, got)
			}
		}
		t.Logf("%-22s trailing Enter=%-5t → %d of %d lost the tail", label, trailingEnter, lost, rounds)
	}
	round("bare", false, 5)
	round("committed", true, 5)
}

// TestSurveyTitleAndCaptionCommit checks whether the same trailing Enter is safe in the two
// other places text is entered: the document title and an image's caption module.
func TestSurveyTitleAndCaptionCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s := openSurvey(t, ctx, false)
	defer s.port.Close()
	port := s.port
	if err := port.activate(ctx, "close_library", ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	base, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	t.Logf("title before %q; %d blocks, %d images", base.Title, len(base.Body), base.ImageCount)

	// The title, then Enter.
	if err := port.Apply(ctx, Mutation{Kind: MutationTitle, Text: "TITLE 하나 TAILWORD"}); err != nil {
		t.Fatalf("title: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	mid, _ := port.Observe(ctx)
	t.Logf("title after the write  %q (blocks=%d)", mid.Title, len(mid.Body))
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("title enter: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	after, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	t.Logf("title after Enter      %q (blocks=%d → %d)", after.Title, len(mid.Body), len(after.Body))

	// A caption, then Enter, on the last image.
	if base.ImageCount == 0 {
		t.Skip("no image in the draft to caption")
	}
	ordinal := base.ImageCount - 1
	if err := port.Apply(ctx, Mutation{Kind: MutationImageCaption, Ordinal: ordinal, Text: "CAP 하나 TAILWORD"}); err != nil {
		t.Fatalf("caption: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	beforeEnter, _ := port.Observe(ctx)
	if err := port.page.PressEnter(ctx); err != nil {
		t.Fatalf("caption enter: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	afterEnter, err := port.Observe(ctx)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	find := func(snapshot Snapshot) string {
		for _, block := range snapshot.Body {
			if block.Kind == SemanticImage && block.Ordinal == ordinal {
				return block.Caption
			}
		}
		return "<none>"
	}
	t.Logf("caption after the write %q (blocks=%d, images=%d)", find(beforeEnter), len(beforeEnter.Body), beforeEnter.ImageCount)
	t.Logf("caption after Enter     %q (blocks=%d, images=%d)", find(afterEnter), len(afterEnter.Body), afterEnter.ImageCount)
}
