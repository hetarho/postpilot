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
		state, err := s.port.paragraphState(ctx, index)
		if err != nil {
			t.Fatalf("paragraph state %d: %v", index, err)
		}
		t.Logf("paragraphState(%2d) matches=%d total=%d kind=%q in_list=%t in_cite=%t list_items=%d",
			index, state.Matches, state.Total, state.Kind, state.InList, state.InCite, state.ListItems)
	}
	if end.Matches != 1 || first.Matches != 1 || beyond.Matches != 0 {
		t.Fatalf("the new body locators did not resolve as expected on a live writer")
	}
}
