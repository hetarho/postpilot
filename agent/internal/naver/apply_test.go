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
		editor.observation = healthyObservation()
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
	if got := editor.recorded(); !slices.Contains(got, "activate:settings_open") {
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
	for name, activation := range map[string]driverActivation{
		"duplicate":  {Matches: 2},
		"unopenable": {Matches: 1, Activated: false},
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{activate: func(string, string) driverActivation { return activation }}
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

// The kinds this signed release does not implement yet refuse rather than half-writing a
// live editor (PUBLISH-19).
func TestApplyRefusesTheKindsThisReleaseDoesNotImplement(t *testing.T) {
	for _, kind := range []MutationKind{MutationHeading, MutationQuote, MutationList, MutationUploadImage, MutationImageCaption, MutationTags, MutationCategory, MutationVisibility} {
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
