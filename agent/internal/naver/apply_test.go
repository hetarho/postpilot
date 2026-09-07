package naver

import (
	"context"
	"errors"
	"slices"
	"testing"
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
	want := []string{"point:title", "click", "text:제목"}
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
	want := []string{"point:body_end", "click", "enter", "text:본문"}
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
// live editor (PUBLISH-19).
func TestApplyRefusesTheKindsThisReleaseDoesNotImplement(t *testing.T) {
	for _, kind := range []MutationKind{MutationHeading, MutationQuote, MutationList, MutationUploadImage, MutationImageCaption} {
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
// the settings layer opens, and the layer opens exactly once.
func TestPlanOrderRefusesADocumentWriteAfterTheSettingsLayerOpens(t *testing.T) {
	step := func(kind MutationKind) plannedMutation {
		return plannedMutation{mutation: Mutation{Kind: kind}, update: func(*Snapshot) {}}
	}
	valid := []plannedMutation{
		step(MutationTitle), step(MutationText), step(MutationUploadImage), step(MutationImageCaption),
		step(MutationOpenSettings), step(MutationTags), step(MutationCategory), step(MutationVisibility),
	}
	if err := assertPlanOrder(valid); err != nil {
		t.Fatalf("the r4 order was refused: %v", err)
	}
	for name, plan := range map[string][]plannedMutation{
		"text after settings":    {step(MutationTitle), step(MutationOpenSettings), step(MutationText)},
		"image after settings":   {step(MutationTitle), step(MutationOpenSettings), step(MutationUploadImage)},
		"caption after settings": {step(MutationTitle), step(MutationOpenSettings), step(MutationImageCaption)},
		"never opened":           {step(MutationTitle), step(MutationTags)},
		"opened twice":           {step(MutationTitle), step(MutationOpenSettings), step(MutationOpenSettings)},
		"unreviewed kind":        {step("image_placeholder"), step(MutationOpenSettings)},
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
