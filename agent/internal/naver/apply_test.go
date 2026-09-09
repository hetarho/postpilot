package naver

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func applyPort(t *testing.T, editor *fakeEditor) *CDPPort {
	t.Helper()
	if editor.observation == nil {
		editor.observation = closedObservation()
	}
	port, err := NewCDPPort(context.Background(), startFakeCDP(t, editor))
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	// The real settle deadline is 30s; a fake never needs it and a refusal must not stall
	// the suite.
	port.settle = 300 * time.Millisecond
	t.Cleanup(func() { _ = port.Close() })
	return port
}

// The title is written by resolving its own control and typing at the caret it gives —
// never by a caller-supplied selector or a remembered point (PUBLISH-36).
func TestApplyTitleResolvesItsOwnControlThenTypes(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationTitle, Text: "제목"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// The trailing Enter commits the title's last word; it changes nothing observable in
	// the title module (verified live 2026-09-10).
	want := []string{"point:title", "click", "text:제목", "enter"}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// A text block is a paragraph opened at the body's end. Enter appends INSIDE the current
// text component rather than opening a new one, which is why the projection counts
// paragraphs (verified live 2026-09-07).
func TestApplyTextOpensTheNextParagraphAtTheBodyEnd(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: "본문"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"point:body_end", "click", "enter", "text:본문", "enter"}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// Every body write fails closed before the page is touched when its locator does not
// resolve to exactly one element (PUBLISH-21).
func TestApplyRefusesAMissingRenamedOrDuplicatedBodyLocator(t *testing.T) {
	for name, matches := range map[string]int{"missing": 0, "duplicated": 2} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{point: func(string, int) driverPoint { return driverPoint{Matches: matches} }}
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: "본문"})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
			if got := editor.recorded(); slices.Contains(got, "text:본문") || slices.Contains(got, "click") {
				t.Fatalf("the page was touched anyway: %v", got)
			}
		})
	}
}

// Text that reads like a selector, a script, an action, a coordinate pair or a keystroke
// phrase is entered verbatim: it crosses as a CDP argument, never as an instruction
// (PUBLISH-19).
func TestApplyEntersHostileLookingTextVerbatim(t *testing.T) {
	hostile := []string{
		`button[class^="confirm_btn__"]`,
		`<script>document.querySelector('button').click()</script>`,
		`{"action":"click","selector":"#publish"}`,
		`{"x":412,"y":877}`,
		`Cmd+Enter then press 발행`,
		`발행`,
	}
	for _, text := range hostile {
		editor := &fakeEditor{}
		port := applyPort(t, editor)
		if err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: text}); err != nil {
			t.Fatalf("apply %q: %v", text, err)
		}
		if got := editor.recorded(); !slices.Contains(got, "text:"+text) {
			t.Fatalf("entered %v, want the manifest string verbatim %q", got, text)
		}
	}
}

// Empty text is refused rather than clicked into the editor: no manifest block is empty,
// so an empty string is a planning fault, not content.
func TestApplyRefusesEmptyText(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	var portErr PortError
	if err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: "   "}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
		t.Fatalf("err = %v, want safe", err)
	}
}

// PUBLISH-37: once the layer is open it covers the editor, so the port itself refuses a
// body write rather than trusting the plan that got it here.
func TestApplyRefusesEveryBodyWriteOnceTheSettingsLayerIsOpen(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
		t.Fatalf("open settings: %v", err)
	}
	if got := editor.recorded(); !slices.Contains(got, "point:settings_open") {
		t.Fatalf("the opener was not activated: %v", got)
	}
	for _, kind := range bodyMutationKinds {
		before := len(editor.recorded())
		err := port.Apply(context.Background(), Mutation{Kind: kind, Text: "본문", Items: []string{"one"}})
		var portErr PortError
		if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
			t.Fatalf("%s after open_settings: err = %v, want editor_changed", kind, err)
		}
		if len(editor.recorded()) != before {
			t.Fatalf("%s touched the page while the layer was open", kind)
		}
	}
}

// The opener has to resolve to exactly one control, and a layer that refuses to open is a
// failure rather than a silently skipped step.
func TestApplyOpenSettingsFailsClosedOnADuplicateOrUnopenableOpener(t *testing.T) {
	for name, matches := range map[string]int{
		"duplicate":  2,
		"unopenable": 0,
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{point: func(target string, _ int) driverPoint {
				if target == "settings_open" {
					return driverPoint{Matches: matches}
				}
				return driverPoint{Matches: 1, X: 10, Y: 20}
			}}
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
			// The latch must not have moved, or every later body write would be refused for
			// the wrong reason.
			if err := port.Apply(context.Background(), Mutation{Kind: MutationTitle, Text: "제목"}); err != nil {
				t.Fatalf("a failed open must not latch the layer: %v", err)
			}
		})
	}
}

func TestApplyOpenSettingsRequiresOneShownLayerAndTransitionsItsLocators(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	before, err := port.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.SettingsLayerOpen || before.LocatorMatches[MutationTags] != 0 || before.LocatorMatches[MutationCategory] != 0 || before.LocatorMatches[MutationVisibility] != 0 {
		t.Fatalf("closed snapshot = %+v", before)
	}
	if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
		t.Fatal(err)
	}
	after, err := port.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !after.SettingsLayerOpen || after.LocatorMatches[MutationTags] != 1 || after.LocatorMatches[MutationCategory] != 1 || after.LocatorMatches[MutationVisibility] != 4 {
		t.Fatalf("open snapshot = %+v", after)
	}
	if got := editor.recorded(); !slices.Equal(got, []string{"point:settings_open", "click"}) {
		t.Fatalf("driver actions = %v", got)
	}
}

func TestApplyOpenSettingsFailsClosedWhenTheShownLayerIsMissingOrDuplicated(t *testing.T) {
	for name, layers := range map[string]int{"missing": 0, "duplicated": 2} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{state: func() driverSettingsState { return driverSettingsState{LayerMatches: layers} }}
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
			// The opener was activated, so ambiguity poisons subsequent body writes even if
			// the malformed layer cannot be represented as a valid snapshot.
			if err := port.Apply(context.Background(), Mutation{Kind: MutationTitle, Text: "제목"}); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("body write after ambiguous open = %v", err)
			}
		})
	}
}

func TestApplySettingsFailBeforeTheLayerOpens(t *testing.T) {
	for _, mutation := range []Mutation{
		{Kind: MutationTags, Values: []string{"tag"}},
		{Kind: MutationCategory, ID: "7", Name: "Travel"},
		{Kind: MutationVisibility, ID: "public"},
	} {
		editor := &fakeEditor{}
		port := applyPort(t, editor)
		var portErr PortError
		if err := port.Apply(context.Background(), mutation); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
			t.Fatalf("%s before open: %v", mutation.Kind, err)
		}
		if got := editor.recorded(); len(got) != 0 {
			t.Fatalf("%s touched the page: %v", mutation.Kind, got)
		}
	}
}

func TestApplyTagsUsesOnlyTheRealInputAndKeepsExactOrder(t *testing.T) {
	t.Run("decoy is refused", func(t *testing.T) {
		editor := &fakeEditor{point: func(target string, _ int) driverPoint {
			if target == "tag_input" {
				return driverPoint{Matches: 0}
			}
			return driverPoint{Matches: 1, X: 10, Y: 20}
		}}
		port := applyPort(t, editor)
		if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
			t.Fatal(err)
		}
		var portErr PortError
		if err := port.Apply(context.Background(), Mutation{Kind: MutationTags, Values: []string{"one"}}); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
			t.Fatalf("err = %v", err)
		}
		if slices.Contains(editor.recorded(), "text:one") {
			t.Fatalf("tag reached an unreviewed input: %v", editor.recorded())
		}
	})

	t.Run("three in order", func(t *testing.T) {
		editor := &fakeEditor{}
		port := applyPort(t, editor)
		if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
			t.Fatal(err)
		}
		want := []string{"one", "둘", "three words"}
		if err := port.Apply(context.Background(), Mutation{Kind: MutationTags, Values: want}); err != nil {
			t.Fatal(err)
		}
		snapshot, err := port.Observe(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(snapshot.Tags, want) {
			t.Fatalf("tags = %v, want %v", snapshot.Tags, want)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		editor := &fakeEditor{}
		port := applyPort(t, editor)
		if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
			t.Fatal(err)
		}
		before := len(editor.recorded())
		var portErr PortError
		if err := port.Apply(context.Background(), Mutation{Kind: MutationTags, Values: []string{"same", "same"}}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
			t.Fatalf("err = %v", err)
		}
		if len(editor.recorded()) != before {
			t.Fatalf("duplicate tag touched the page: %v", editor.recorded()[before:])
		}
	})
}

func TestApplyCategoryRequiresTheFrozenIDAndName(t *testing.T) {
	for name, choice := range map[string]driverSetting{
		"missing id":            {Matches: 0, GroupMatches: 11, Expanded: true, NameMatches: false},
		"name mismatch":         {Matches: 1, GroupMatches: 11, Actionable: true, Expanded: true, NameMatches: false},
		"radio stays unchecked": {Matches: 1, GroupMatches: 11, Actionable: true, Expanded: true, NameMatches: true},
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{setting: func(control, _, _ string) driverSetting {
				if control == "category_open" {
					return driverSetting{Matches: 1, GroupMatches: 1, Actionable: true, Expanded: true, NameMatches: true}
				}
				return choice
			}}
			port := applyPort(t, editor)
			if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
				t.Fatal(err)
			}
			var portErr PortError
			if err := port.Apply(context.Background(), Mutation{Kind: MutationCategory, ID: "7", Name: "Travel"}); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestApplyCategoryRefusesASelectboxThatDoesNotExpand(t *testing.T) {
	editor := &fakeEditor{setting: func(control, _, _ string) driverSetting {
		if control == "category_open" {
			return driverSetting{Matches: 1, GroupMatches: 1, Actionable: true, Expanded: false, NameMatches: true}
		}
		return driverSetting{Matches: 1, GroupMatches: 11, Actionable: true, Expanded: true, Checked: true, NameMatches: true}
	}}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
		t.Fatal(err)
	}
	var portErr PortError
	if err := port.Apply(context.Background(), Mutation{Kind: MutationCategory, ID: "7", Name: "Travel"}); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyCategoryOpensTheSelectboxAndVerifiesTheCheckedRadio(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
		t.Fatal(err)
	}
	if err := port.Apply(context.Background(), Mutation{Kind: MutationCategory, ID: "7", Name: "Travel"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := port.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Category != (SelectedSetting{ID: "7", Name: "Travel", Selected: true}) {
		t.Fatalf("category = %+v", snapshot.Category)
	}
}

func TestApplyEachVisibilityUsesItsFixedRadioAndVerifiesChecked(t *testing.T) {
	for _, id := range []string{"public", "neighbor", "both_neighbor", "private"} {
		t.Run(id, func(t *testing.T) {
			editor := &fakeEditor{}
			port := applyPort(t, editor)
			if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
				t.Fatal(err)
			}
			if err := port.Apply(context.Background(), Mutation{Kind: MutationVisibility, ID: id}); err != nil {
				t.Fatal(err)
			}
			snapshot, err := port.Observe(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Visibility != (SelectedSetting{ID: id, Name: visibilityName(id), Selected: true}) {
				t.Fatalf("visibility = %+v", snapshot.Visibility)
			}
		})
	}
}

func TestApplyFailsWhenTheLayerClosesMidSettingsSequence(t *testing.T) {
	calls := 0
	editor := &fakeEditor{state: func() driverSettingsState {
		calls++
		if calls >= 3 {
			return driverSettingsState{}
		}
		return driverSettingsState{LayerMatches: 1, Tags: 1, Category: 1, Visibility: 4}
	}}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationOpenSettings}); err != nil {
		t.Fatal(err)
	}
	var portErr PortError
	if err := port.Apply(context.Background(), Mutation{Kind: MutationTags, Values: []string{"first", "second"}}); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(editor.recorded(), "text:second") {
		t.Fatalf("second tag was typed after the layer closed: %v", editor.recorded())
	}
}

// The kinds this signed release does not implement yet refuse rather than half-writing a
// live editor (PUBLISH-19). Both of them land with T043.
func TestApplyRefusesTheKindsThisReleaseDoesNotImplement(t *testing.T) {
	for _, kind := range []MutationKind{MutationUploadImage, MutationImageCaption} {
		editor := &fakeEditor{}
		port := applyPort(t, editor)
		var portErr PortError
		if err := port.Apply(context.Background(), Mutation{Kind: kind}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
			t.Fatalf("%s: err = %v, want safe", kind, err)
		}
		if got := editor.recorded(); len(got) != 0 {
			t.Fatalf("%s touched the page: %v", kind, got)
		}
	}
}

// PUBLISH-37 as a property of the plan: Prepare may not schedule a document write after
// the settings layer opens, and the layer opens exactly once. The 260910 survey adds two
// more plan-level rules — no paragraph may be written once a quote or list conversion has
// landed, and the conversions run backwards — because a converted quotation traps the caret
// in its 출처 module and a converted list turns the next Enter into another list item.
func TestPlanOrderRefusesADocumentWriteAfterTheSettingsLayerOpens(t *testing.T) {
	step := func(kind MutationKind) plannedMutation {
		return plannedMutation{mutation: Mutation{Kind: kind}, update: func(*Snapshot) {}}
	}
	convert := func(kind MutationKind, ordinal int) plannedMutation {
		return plannedMutation{mutation: Mutation{Kind: kind, Ordinal: ordinal}, update: func(*Snapshot) {}}
	}
	valid := []plannedMutation{
		step(MutationTitle), step(MutationText), step(MutationHeading),
		step(MutationUploadImage), step(MutationImageCaption), step(MutationText), step(MutationText),
		convert(MutationList, 3), convert(MutationQuote, 2),
		step(MutationOpenSettings), step(MutationTags), step(MutationCategory), step(MutationVisibility),
	}
	if err := assertPlanOrder(valid); err != nil {
		t.Fatalf("the r4 order with the two body passes was refused: %v", err)
	}
	for name, plan := range map[string][]plannedMutation{
		"text after settings":             {step(MutationTitle), step(MutationOpenSettings), step(MutationText)},
		"image after settings":            {step(MutationTitle), step(MutationOpenSettings), step(MutationUploadImage)},
		"caption after settings":          {step(MutationTitle), step(MutationOpenSettings), step(MutationImageCaption)},
		"never opened":                    {step(MutationTitle), step(MutationTags)},
		"opened twice":                    {step(MutationTitle), step(MutationOpenSettings), step(MutationOpenSettings)},
		"unreviewed kind":                 {step("image_placeholder"), step(MutationOpenSettings)},
		"text after a quote conversion":   {step(MutationTitle), step(MutationText), convert(MutationQuote, 0), step(MutationText), step(MutationOpenSettings)},
		"heading after a list conversion": {step(MutationTitle), step(MutationText), convert(MutationList, 0), step(MutationHeading), step(MutationOpenSettings)},
		"conversions running forwards":    {step(MutationTitle), step(MutationText), step(MutationText), convert(MutationQuote, 0), convert(MutationList, 1), step(MutationOpenSettings)},
		"an upload after a conversion":    {step(MutationTitle), step(MutationText), convert(MutationQuote, 0), step(MutationUploadImage), step(MutationOpenSettings)},
		"a caption after a conversion":    {step(MutationTitle), step(MutationText), convert(MutationQuote, 0), step(MutationImageCaption), step(MutationOpenSettings)},
		"one paragraph converted twice":   {step(MutationTitle), step(MutationText), convert(MutationQuote, 0), convert(MutationList, 0), step(MutationOpenSettings)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := assertPlanOrder(plan); err == nil {
				t.Fatal("the plan was accepted")
			}
		})
	}
}

// The retired kind must be unreachable, not merely unused: a plan naming it is refused and
// the reviewed vocabulary does not contain it.
func TestImagePlaceholderIsGoneFromTheReviewedVocabulary(t *testing.T) {
	if slices.Contains(reviewedMutationKinds, MutationKind("image_placeholder")) {
		t.Fatal("image_placeholder is still in the reviewed vocabulary")
	}
	if len(reviewedMutationKinds) != 11 {
		t.Fatalf("reviewed kinds = %d, want the eleven r4 kinds", len(reviewedMutationKinds))
	}
	manifest, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := manifest.SemanticLocators["image_placeholder"]; exists {
		t.Fatal("the manifest still carries an image_placeholder locator")
	}
	if manifest.SemanticLocators["open_settings"].CSS == "" {
		t.Fatal("the manifest carries no open_settings locator")
	}
}

// A second body block is a paragraph appended INSIDE the current text component: both
// writes resolve the same body_end target, and the projection counts paragraphs rather
// than components (verified live 2026-09-07).
func TestApplyAppendsASecondParagraphThroughTheSameBodyEndTarget(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	for _, text := range []string{"첫 문단", "둘째 문단"} {
		if err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: text}); err != nil {
			t.Fatalf("apply %q: %v", text, err)
		}
	}
	// Each write ends with the Enter that commits its last word, and the next write types
	// into the paragraph that Enter opened rather than adding one of its own.
	want := []string{
		"point:body_end", "click", "enter", "text:첫 문단", "enter",
		"point:body_end", "click", "text:둘째 문단", "enter",
	}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// A heading writes its paragraph and converts it in the SAME step. The conversion alone is
// invisible to the projection — Naver exports a section title as plain text — so a separate
// step could never move the snapshot token Publisher.mutate requires to move.
func TestApplyHeadingWritesTheParagraphThenConvertsThatParagraph(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationHeading, Text: "소제목", Level: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{
		"point:body_end", "click", "enter", "text:소제목", "enter",
		"point:body_paragraph", "click", "activate:format_menu", "activate:format_heading",
	}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// A quote converts the paragraph the write pass already entered and types NOTHING of its
// own: 인용구 is born with an empty 출처 module whose paragraph is the component's last, so
// text entered after the conversion would land in the citation (verified live 2026-09-10).
func TestApplyQuoteConvertsTheAddressedParagraphAndEntersNoText(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationQuote, Text: "인용", Ordinal: 3}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"point:body_paragraph", "click", "activate:format_menu", "activate:format_quote"}
	got := editor.recorded()
	if !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
	for _, action := range got {
		if strings.HasPrefix(action, "text:") || action == "enter" {
			t.Fatalf("the quote conversion wrote into the editor: %v", got)
		}
	}
}

// One manifest LIST block with N items is built by converting the paragraph the write pass
// entered as item one and then pressing Enter per remaining item: Enter from a list item
// appends another LI to the SAME UL (verified live 2026-09-10).
func TestApplyListConvertsThenAppendsTheRemainingItems(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	mutation := Mutation{Kind: MutationList, Ordinal: 2, Items: []string{"하나", "둘", "셋"}}
	if err := port.Apply(context.Background(), mutation); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// The two Enters after the last item commit its trailing word and drop the empty item
	// that committing Enter opened.
	want := []string{
		"point:body_paragraph", "click", "activate:list_menu", "activate:list_bullet",
		"enter", "text:둘", "enter", "text:셋", "enter", "enter",
	}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
	if editor.listItems != 3 {
		t.Fatalf("the list holds %d items, want 3", editor.listItems)
	}
}

// The property toolbar carries a format or list control only while the caret sits in a
// plain, unconverted body paragraph: with the caret inside a quotation the list control
// resolved 0 live. Every conversion therefore asserts the target's component kind before
// resolving any control, and refuses before the page is touched (PUBLISH-21).
func TestApplyRefusesAConversionWhoseTargetIsNotAPlainParagraph(t *testing.T) {
	targets := map[string]driverParagraphState{
		"already a section title": {Matches: 1, Total: 4, Kind: "se-sectionTitle"},
		"already a quotation":     {Matches: 1, Total: 4, Kind: "se-quotation"},
		"already a list item":     {Matches: 1, Total: 4, Kind: "se-text", InList: true, ListItems: 1},
		"the quotation's 출처":      {Matches: 1, Total: 4, Kind: "se-quotation", InCite: true},
		"out of range":            {Matches: 0, Total: 4},
	}
	for name, state := range targets {
		for _, mutation := range []Mutation{
			{Kind: MutationQuote, Text: "인용", Ordinal: 1},
			{Kind: MutationList, Ordinal: 1, Items: []string{"하나", "둘"}},
		} {
			t.Run(name+"/"+string(mutation.Kind), func(t *testing.T) {
				editor := &fakeEditor{paragraph: func(int) driverParagraphState { return state }}
				port := applyPort(t, editor)
				err := port.Apply(context.Background(), mutation)
				var portErr PortError
				if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
					t.Fatalf("err = %v, want editor_changed", err)
				}
				if got := editor.recorded(); len(got) != 0 {
					t.Fatalf("the page was touched anyway: %v", got)
				}
			})
		}
	}
}

// A missing, duplicated or renamed paragraph control fails closed. The option controls do
// not exist until the menu is open — they resolve 0 while it is closed — so the option is
// counted after the menu opens, exactly as the settings layer's controls are; nothing is
// typed on this path, so a refusal leaves the document as the write pass left it.
func TestApplyRefusesAMissingDuplicatedOrRenamedParagraphControl(t *testing.T) {
	for name, refused := range map[string]string{
		"menu":           "format_menu",
		"heading option": "format_heading",
		"quote option":   "format_quote",
		"list menu":      "list_menu",
		"bullet option":  "list_bullet",
	} {
		for _, matches := range []int{0, 2} {
			t.Run(fmt.Sprintf("%s/%d", name, matches), func(t *testing.T) {
				editor := &fakeEditor{activate: func(control, _ string) driverActivation {
					if control == refused {
						return driverActivation{Matches: matches}
					}
					return driverActivation{Matches: 1, Activated: true}
				}}
				port := applyPort(t, editor)
				mutation := Mutation{Kind: MutationQuote, Text: "인용", Ordinal: 0}
				if strings.HasPrefix(refused, "list") {
					mutation = Mutation{Kind: MutationList, Ordinal: 0, Items: []string{"하나", "둘"}}
				} else if refused == "format_heading" {
					mutation = Mutation{Kind: MutationHeading, Text: "소제목", Level: 1}
				}
				err := port.Apply(context.Background(), mutation)
				var portErr PortError
				if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
					t.Fatalf("err = %v, want editor_changed", err)
				}
				for _, action := range editor.recorded() {
					if action == "text:인용" || action == "text:둘" {
						t.Fatalf("content was entered through a refused control: %v", editor.recorded())
					}
				}
			})
		}
	}
}

// A control that activated without converting anything is a driver/editor mismatch, not a
// success: the port re-observes the addressed paragraph and refuses when its component kind
// did not change. This is the whole reason the conversions carry their own assertion — a
// heading conversion is invisible to the projection, so Publisher.mutate cannot catch it.
func TestApplyRefusesAConversionThatDidNotLand(t *testing.T) {
	cases := map[string]Mutation{
		"heading": {Kind: MutationHeading, Text: "소제목", Level: 1},
		"quote":   {Kind: MutationQuote, Text: "인용", Ordinal: 0},
	}
	for name, mutation := range cases {
		t.Run(name, func(t *testing.T) {
			// The activation is accepted but the paragraph stays a plain se-text.
			editor := &fakeEditor{paragraph: func(int) driverParagraphState {
				return driverParagraphState{Matches: 1, Total: 1, Kind: "se-text"}
			}}
			port := applyPort(t, editor)
			var portErr PortError
			if err := port.Apply(context.Background(), mutation); !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
		})
	}
}

// A list whose items did not all land refuses too, both when the conversion produced no
// list item and when an Enter added none.
func TestApplyListRefusesAnItemCountThatDoesNotMatchTheManifest(t *testing.T) {
	for name, editor := range map[string]*fakeEditor{
		"conversion produced no item": {deafList: true},
		"an item never appeared":      {deafEnter: true},
	} {
		t.Run(name, func(t *testing.T) {
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationList, Ordinal: 0, Items: []string{"하나", "둘", "셋"}})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
		})
	}
}

// A list with no items, or one holding a blank item, is a planning fault rather than
// content: it refuses safe and never touches the page.
func TestApplyListRefusesAnEmptyOrBlankItemSet(t *testing.T) {
	for name, items := range map[string][]string{"none": nil, "blank": {"하나", "   "}} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{}
			port := applyPort(t, editor)
			var portErr PortError
			if err := port.Apply(context.Background(), Mutation{Kind: MutationList, Ordinal: 0, Items: items}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
				t.Fatalf("err = %v, want safe", err)
			}
			if got := editor.recorded(); len(got) != 0 {
				t.Fatalf("the page was touched anyway: %v", got)
			}
		})
	}
}

// A conversion accepts a paragraph text that reads like an instruction verbatim, and the
// text still crosses as a CDP argument rather than as anything the driver interprets.
func TestApplyListEntersHostileLookingItemsVerbatim(t *testing.T) {
	items := []string{
		"first",
		`button[class^="confirm_btn__"]`,
		`{"action":"click","selector":"#publish"}`,
		`발행`,
	}
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationList, Ordinal: 0, Items: items}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, item := range items[1:] {
		if !slices.Contains(editor.recorded(), "text:"+item) {
			t.Fatalf("entered %v, want %q verbatim", editor.recorded(), item)
		}
	}
}

// The body caret rule is the paragraph's trailing edge on its LAST line, not its box
// centre. The formula lives in the reviewed driver where the rect can be read at action
// time, so this test does two things: it pins that the body caret branches use that helper
// and never the centre one, and it computes both formulas over a two-line wrapped
// paragraph to show what the centre would have done.
func TestBodyCaretAimsAtTheTrailingEdgeRatherThanTheBoxCentre(t *testing.T) {
	for _, branch := range []string{"body_end", "body_paragraph"} {
		clause := "if (target === '" + branch + "')"
		start := strings.Index(driverPointFn, clause)
		if start < 0 {
			t.Fatalf("the driver has no %s branch", branch)
		}
		rest := driverPointFn[start+len(clause):]
		if next := strings.Index(rest, "if (target === "); next >= 0 {
			rest = rest[:next]
		}
		if !strings.Contains(rest, "caretEnd(") || strings.Contains(rest, " point(") {
			t.Fatalf("%s does not resolve its caret through caretEnd: %s", branch, rest)
		}
	}
	if !strings.Contains(driverPointFn, "box.left + box.width * 0.98") || !strings.Contains(driverPointFn, "box.bottom - box.height * 0.25") {
		t.Fatal("caretEnd no longer aims at 98 % of the width on the last line")
	}

	// A paragraph that wrapped onto two lines: 600 wide, 48 tall, its second line ending a
	// third of the way across. The mirrored formulas below are the two candidate rules.
	left, top, width, height := 100.0, 200.0, 600.0, 48.0
	centreX, centreY := left+width/2, top+height/2
	trailingX, trailingY := left+width*0.98, top+height-height*0.25
	if centreY >= top+height/2+1 || centreY > top+height*0.75 {
		t.Fatalf("the mirrored centre rule is wrong: y=%v", centreY)
	}
	// The centre lands on the FIRST line of a two-line paragraph, mid-text; the trailing
	// edge lands on the last line past the end of its text, which is where the caret has to
	// go for the next insertion not to split a paragraph.
	if centreY > top+height/2 {
		t.Fatalf("centre y=%v is not on the first line", centreY)
	}
	if trailingY <= top+height/2 {
		t.Fatalf("trailing y=%v is not on the last line", trailingY)
	}
	if centreX >= trailingX {
		t.Fatalf("centre x=%v is not left of the trailing edge x=%v", centreX, trailingX)
	}
}

// A caption is entered into the caption module of ONE addressed image ordinal. The module
// is created with its image, so the ordinal is the whole address: no selector and no
// remembered point reach the driver from the caller.
func TestApplyImageCaptionResolvesTheAddressedOrdinalThenTypes(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationImageCaption, Ordinal: 3, Text: "캡션"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"point:image_select", "click", "point:image_caption", "click", "text:캡션", "enter"}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// A caption applied to an ordinal with no image, or to an image whose caption module is
// missing or duplicated, fails closed before any text is typed (PUB-21). The same holds when
// the image itself cannot be selected, since an unselected image keeps its caption boxless.
func TestApplyImageCaptionRefusesAWrongOrdinalOrAmbiguousModule(t *testing.T) {
	for name, resolved := range map[string]driverPoint{
		"no image at that ordinal": {Matches: 0},
		"two caption modules":      {Matches: 2},
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{point: func(string, int) driverPoint { return resolved }}
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationImageCaption, Ordinal: 9, Text: "캡션"})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
			if got := editor.recorded(); slices.Contains(got, "text:캡션") {
				t.Fatalf("a caption was typed anyway: %v", got)
			}
		})
	}
}

// An empty caption is not written at all: the placeholder the module carries while empty is
// the editor's own chrome, and typing a blank string into it would be a planning fault.
func TestApplyImageCaptionRefusesAnEmptyCaption(t *testing.T) {
	for name, text := range map[string]string{"empty": "", "blank": "   "} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{}
			port := applyPort(t, editor)
			var portErr PortError
			if err := port.Apply(context.Background(), Mutation{Kind: MutationImageCaption, Ordinal: 0, Text: text}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
				t.Fatalf("err = %v, want safe", err)
			}
			if got := editor.recorded(); len(got) != 0 {
				t.Fatalf("the page was touched anyway: %v", got)
			}
		})
	}
}

// A caption module reads back its placeholder while empty, so the projection must read it
// only when the module is not `se-is-empty`. An image with no caption contributes an empty
// caption to the body, never the placeholder string.
func TestProjectionNeverReadsTheCaptionPlaceholderAsContent(t *testing.T) {
	observation := closedObservation()
	blocks := observation["blocks"].([]map[string]any)
	blocks[3] = map[string]any{
		"kind": "image", "text": "", "ordinal": 0,
		// What the script yields for an `se-is-empty` module: the empty string, never
		// 사진 설명을 입력하세요.
		"caption": "", "uploaded": true, "source": "a.jpg",
	}
	editor := &fakeEditor{observation: observation}
	port := applyPort(t, editor)
	snapshot, err := port.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	for _, block := range snapshot.Body {
		if strings.Contains(block.Caption, "사진 설명") {
			t.Fatalf("the placeholder entered the projection: %+v", block)
		}
	}
	if snapshot.Body[3].Kind != SemanticImage || snapshot.Body[3].Caption != "" {
		t.Fatalf("image block = %+v", snapshot.Body[3])
	}
	if !strings.Contains(editorObservationScript, "se-is-empty") {
		t.Fatal("the observation script no longer guards the caption's empty state")
	}
}

// The upload locator is the image BUTTON, not the hidden file input: the input does not
// exist until that button is clicked, so counting the input would make Publisher.mutate's
// pre-check unsatisfiable for every upload.
func TestUploadLocatorCountsTheImageButtonRatherThanTheHiddenInput(t *testing.T) {
	manifest, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.SemanticLocators["upload_image"].CSS; got != "button.se-image-toolbar-button" {
		t.Fatalf("upload_image locator = %q", got)
	}
	if strings.Contains(editorObservationScript, "upload_image: count('input[type=file]#hidden-file')") {
		t.Fatal("the observation still counts the hidden input as the upload locator")
	}
	if !strings.Contains(editorObservationScript, "upload_image: count('button.se-image-toolbar-button')") {
		t.Fatal("the observation does not count the image button as the upload locator")
	}
	if expectedLocatorMatches(MutationUploadImage) != 1 {
		t.Fatal("the upload pre-check no longer expects exactly one match")
	}
}

// PUB-19: only enumerated current-job JPEG paths may reach the restricted upload operation.
// Prepare builds the plan from that set, so this guard is structural — it refuses a plan
// that named anything else, planned the wrong number of uploads, or hung an asset path on a
// kind that has no business carrying one.
func TestPlanRefusesAnyAssetPathOutsideTheJobsEnumeratedSet(t *testing.T) {
	paths := []string{"/jobs/x/0000.jpg", "/jobs/x/0001.jpg"}
	upload := func(ordinal int, path string) plannedMutation {
		return plannedMutation{mutation: Mutation{Kind: MutationUploadImage, Ordinal: ordinal, AssetPath: path}, update: func(*Snapshot) {}}
	}
	step := func(kind MutationKind) plannedMutation {
		return plannedMutation{mutation: Mutation{Kind: kind}, update: func(*Snapshot) {}}
	}
	valid := []plannedMutation{step(MutationTitle), upload(0, paths[0]), upload(1, paths[1]), step(MutationOpenSettings)}
	if err := assertEnumeratedAssets(valid, paths); err != nil {
		t.Fatalf("the enumerated plan was refused: %v", err)
	}
	for name, plan := range map[string][]plannedMutation{
		"a path outside the set":     {upload(0, paths[0]), upload(1, "/etc/passwd")},
		"another job's asset":        {upload(0, paths[0]), upload(1, "/jobs/y/0000.jpg")},
		"a relative path":            {upload(0, paths[0]), upload(1, "0001.jpg")},
		"too few uploads":            {upload(0, paths[0])},
		"the same asset twice":       {upload(0, paths[0]), upload(1, paths[0])},
		"an asset path on a caption": {upload(0, paths[0]), upload(1, paths[1]), {mutation: Mutation{Kind: MutationImageCaption, AssetPath: paths[0]}, update: func(*Snapshot) {}}},
		// PUB-21's ordinal order, refused three ways.
		"ordinals reversed":   {upload(1, paths[1]), upload(0, paths[0])},
		"an ordinal skipped":  {upload(0, paths[0]), upload(2, paths[1])},
		"an ordinal repeated": {upload(0, paths[0]), upload(0, paths[1])},
	} {
		t.Run(name, func(t *testing.T) {
			if err := assertEnumeratedAssets(plan, paths); err == nil {
				t.Fatal("the plan was accepted")
			}
		})
	}
}

// PUB-36: geometry is read at action time and never recorded, stored, replayed or carried
// between mutations or runs. This walks every type the image path crosses and proves none of
// them has a field that could hold a resolved point, and that the port itself keeps no such
// field between calls.
func TestNoTypeOnTheImagePathCanHoldResolvedGeometry(t *testing.T) {
	// Exact names for the ones that are whole words on their own, substrings for the rest.
	// "Body" is not geometry just because it ends in a y.
	exact := []string{"x", "y", "left", "top", "right", "bottom", "width", "height", "offset", "position"}
	substring := []string{"point", "rect", "coord", "viewport", "screenx", "geometry", "clientx", "clienty"}
	walked := 0
	var walk func(t *testing.T, typ reflect.Type, path string)
	walk = func(t *testing.T, typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return
		}
		walked++
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			lowered := strings.ToLower(field.Name)
			if slices.Contains(exact, lowered) {
				t.Fatalf("%s.%s could hold resolved geometry", path, field.Name)
			}
			for _, banned := range substring {
				if strings.Contains(lowered, banned) {
					t.Fatalf("%s.%s could hold resolved geometry", path, field.Name)
				}
			}
			if field.Type == reflect.TypeOf(driverPoint{}) {
				t.Fatalf("%s.%s retains a resolved point", path, field.Name)
			}
			if field.Type.Kind() == reflect.Struct && field.Type.PkgPath() == typ.PkgPath() {
				walk(t, field.Type, path+"."+field.Name)
			}
		}
	}
	for _, subject := range []any{Mutation{}, SemanticBlock{}, Snapshot{}, Prepared{}, PreparationResult{}, Readback{}, FinalControl{}} {
		typ := reflect.TypeOf(subject)
		walk(t, typ, typ.Name())
	}
	if walked < 7 {
		t.Fatalf("walked only %d types", walked)
	}
	// The port holds the settings latch and the manifest, and nothing shaped like a point.
	portType := reflect.TypeOf(CDPPort{})
	for index := 0; index < portType.NumField(); index++ {
		if portType.Field(index).Type == reflect.TypeOf(driverPoint{}) {
			t.Fatalf("CDPPort.%s caches a resolved point between calls", portType.Field(index).Name)
		}
	}
}

// The upload is one atomic sequence, and its order matters: the photo-library sidebar that a
// previous upload left over the editor is closed first, the caret is placed from the body's
// own last paragraph, interception is enabled BEFORE the click that would otherwise open the
// native chooser, and the hidden input — which that click is what creates — receives exactly
// one enumerated path. Verified live 2026-09-07 and 2026-09-10.
func TestApplyUploadImageRunsTheWholeSequenceInOrder(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, Ordinal: 0, AssetPath: "/jobs/x/0000.jpg"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{
		"activate:close_library",
		"point:body_end", "click",
		"intercept",
		"point:image_add", "click",
		"query:input[type=file]#hidden-file", "upload:/jobs/x/0000.jpg",
		"activate:close_library",
	}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
	if !slices.Equal(editor.uploaded, []string{"/jobs/x/0000.jpg"}) {
		t.Fatalf("uploaded %v, want exactly one file", editor.uploaded)
	}
}

// The upload locator is satisfiable before the click and the input only exists after it: the
// pre-check counts the image button, and the file input is absent until the button is
// clicked. This pins both halves of that timing.
func TestTheHiddenFileInputIsAbsentAtPreCheckTimeAndPresentAtUploadTime(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	snapshot, err := port.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if snapshot.LocatorMatches[MutationUploadImage] != 1 {
		t.Fatalf("the upload pre-check saw %d matches before any click", snapshot.LocatorMatches[MutationUploadImage])
	}
	if err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "/jobs/x/0000.jpg"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := editor.recorded()
	query := slices.Index(got, "query:input[type=file]#hidden-file")
	click := slices.Index(got, "point:image_add")
	if query < 0 || click < 0 || query < click {
		t.Fatalf("the hidden input was resolved before the button that creates it: %v", got)
	}
	if !slices.Equal(editor.uploaded, []string{"/jobs/x/0000.jpg"}) {
		t.Fatalf("uploaded %v", editor.uploaded)
	}
}

// The hidden input is created on demand, so it can also fail to appear. That is an editor
// mismatch, not a retryable hiccup: no file is handed anywhere and the upload fails closed.
func TestApplyUploadImageRefusesWhenTheFileInputNeverAppears(t *testing.T) {
	editor := &fakeEditor{fileInputMissing: true}
	port := applyPort(t, editor)
	err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "/jobs/x/0000.jpg"})
	var portErr PortError
	if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
		t.Fatalf("err = %v, want editor_changed", err)
	}
	if len(editor.uploaded) != 0 {
		t.Fatalf("a file was handed over anyway: %v", editor.uploaded)
	}
}

// An upload with no path is a planning fault and never touches the page.
func TestApplyUploadImageRefusesAnEmptyPath(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	var portErr PortError
	if err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "  "}); !errors.As(err, &portErr) || portErr.Kind != FailureSafe {
		t.Fatalf("err = %v, want safe", err)
	}
	if got := editor.recorded(); len(got) != 0 {
		t.Fatalf("the page was touched anyway: %v", got)
	}
}

// The photo-library sidebar overlays the editor's right edge, which is exactly where a caret
// point is taken. A sidebar that will not close is a refusal, because the caret click after
// it would land on the panel rather than in the document.
func TestApplyUploadImageRefusesASidebarThatWillNotClose(t *testing.T) {
	for name, resolved := range map[string]driverActivation{
		"the close control is gone":  {Matches: 0},
		"two panels are shown":       {Matches: 2},
		"the control does not click": {Matches: 1},
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{activate: func(control, _ string) driverActivation {
				if control == "close_library" {
					return resolved
				}
				return driverActivation{Matches: 1, Activated: true}
			}}
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "/jobs/x/0000.jpg"})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
			if len(editor.uploaded) != 0 {
				t.Fatalf("a file was handed over anyway: %v", editor.uploaded)
			}
		})
	}
}

// A caret point that resolves but does not actually hit the element the locator resolved is
// refused rather than clicked. That is not defensive: an image upload opens a 300 px sidebar
// over the editor's right edge, and every caret click taken there landed on the panel and
// left the caret outside the document (observed live 2026-09-10).
func TestTheDriverVerifiesACaretPointActuallyHitsItsParagraph(t *testing.T) {
	if !strings.Contains(driverPointFn, "elementFromPoint") {
		t.Fatal("the driver no longer hit-tests the point it is about to click")
	}
	if !strings.Contains(driverPointFn, "scrollIntoView") {
		t.Fatal("the driver no longer brings the resolved element into view before reading its rect")
	}
	// Both body caret branches and both control branches go through the verifying helper.
	for _, target := range []string{"body_end", "body_paragraph", "image_add", "image_select", "image_caption"} {
		clause := "if (target === '" + target + "')"
		start := strings.Index(driverPointFn, clause)
		if start < 0 {
			t.Fatalf("the driver has no %s branch", target)
		}
		rest := driverPointFn[start+len(clause):]
		if next := strings.Index(rest, "if (target === "); next >= 0 {
			rest = rest[:next]
		}
		if !strings.Contains(rest, "caretEnd(") && !strings.Contains(rest, "point(") && !strings.Contains(rest, "at(") {
			t.Fatalf("%s resolves a point without the verifying helper: %s", target, rest)
		}
	}
}

// An empty append target is typed into rather than stepped over with Enter: SmartEditor's
// own opening paragraph and the slot it leaves after an image are both empty, and pressing
// Enter there strands a blank paragraph the published post would render as a blank line.
func TestApplyTextTypesIntoAnAlreadyEmptyParagraphWithoutALeadingEnter(t *testing.T) {
	editor := &fakeEditor{paragraph: func(int) driverParagraphState {
		return driverParagraphState{Matches: 1, Total: 1, Kind: "se-text", Empty: true}
	}}
	port := applyPort(t, editor)
	if err := port.Apply(context.Background(), Mutation{Kind: MutationText, Text: "첫 문단"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"point:body_end", "click", "text:첫 문단", "enter"}
	if got := editor.recorded(); !slices.Equal(got, want) {
		t.Fatalf("driver actions = %v, want %v", got, want)
	}
}

// A photo counts only once a fresh observation shows exactly one more SETTLED image. Until
// then SmartEditor is still splitting the caret's component around the new image and the
// projection reads a paragraph whose tail has been detached — which is how a clean-editor
// run read back "LIVE T1 첫 " for a paragraph that had just read whole. An upload that adds
// none, adds two, or never finishes processing all fail closed (PUB-21).
func TestApplyUploadImageWaitsForExactlyOneSettledImage(t *testing.T) {
	for name, editor := range map[string]*fakeEditor{
		"no image appeared": {imageAddsNone: true},
		"two images added":  {imageAddsTwo: true},
		"never finishes":    {imageNeverSettle: true},
	} {
		t.Run(name, func(t *testing.T) {
			port := applyPort(t, editor)
			err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "/jobs/x/0000.jpg"})
			var portErr PortError
			if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
				t.Fatalf("err = %v, want editor_changed", err)
			}
		})
	}
}

// An upload refuses to start while an earlier photo is still processing: "exactly one more"
// has to be an observation, not a guess about which of two pending images is ours.
func TestApplyUploadImageRefusesWhileAnEarlierPhotoIsStillProcessing(t *testing.T) {
	editor := &fakeEditor{baseImages: 2, imageNeverSettle: true}
	// One upload already happened, so the fake reports an unsettled image before this one.
	editor.uploaded = []string{"/jobs/x/0000.jpg"}
	port := applyPort(t, editor)
	err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, AssetPath: "/jobs/x/0001.jpg"})
	var portErr PortError
	if !errors.As(err, &portErr) || portErr.Kind != FailureEditorChanged {
		t.Fatalf("err = %v, want editor_changed", err)
	}
	if slices.Contains(editor.recorded(), "upload:/jobs/x/0001.jpg") {
		t.Fatalf("the second file was handed over anyway: %v", editor.recorded())
	}
}

// Eight photos in a row each settle before the next begins, so the count the driver requires
// is always "one more than what this upload started from".
func TestApplyUploadImageCountsUpAcrossEightPhotos(t *testing.T) {
	editor := &fakeEditor{}
	port := applyPort(t, editor)
	for ordinal := 0; ordinal < 8; ordinal++ {
		path := fmt.Sprintf("/jobs/x/%04d.jpg", ordinal)
		if err := port.Apply(context.Background(), Mutation{Kind: MutationUploadImage, Ordinal: ordinal, AssetPath: path}); err != nil {
			t.Fatalf("upload %d: %v", ordinal, err)
		}
	}
	if len(editor.uploaded) != 8 {
		t.Fatalf("uploaded %v", editor.uploaded)
	}
	for ordinal, path := range editor.uploaded {
		if path != fmt.Sprintf("/jobs/x/%04d.jpg", ordinal) {
			t.Fatalf("upload %d handed over %q", ordinal, path)
		}
	}
}
