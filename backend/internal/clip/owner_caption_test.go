package clip_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// placedDraft is the creation fixture's first caption with the owner's own
// placement, size and style on it, ready to save.
func placedDraft(plan clip.EditPlan, owner clip.OwnerCaption) clip.CorrectionPlan {
	draft := creationDraft(plan)
	for i := range draft.Elements {
		if draft.Elements[i].Role == "caption" {
			draft.Elements[i].Owner = owner
			break
		}
	}
	return draft
}

func firstCaption(t *testing.T, plan clip.EditPlan) clip.PortableText {
	t.Helper()
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role == "caption" {
			return text
		}
	}
	t.Fatal("the fixture has no caption")
	return clip.PortableText{}
}

// CDS-82: a save accepts a caption's own position, size and style and hands all
// three back unchanged, so ② reopens on what the owner left.
func TestSaveRoundTripsTheOwnersPlacement(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	p.CaptionStyles = []string{design.DefaultCaptionStyle, "film"}
	at := clip.CaptionPlacement{X: 200, Y: 900}
	next, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Position: &at, Size: 64, Style: "film"}))
	if err != nil {
		t.Fatal(err)
	}
	saved := firstCaption(t, next).Owner
	if saved.Position == nil || *saved.Position != at || saved.Size != 64 || saved.Style != "film" {
		t.Fatalf("the owner's placement did not survive the save: %+v", saved)
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again := firstCaption(t, reopened).Owner; again.Position == nil || *again.Position != at || again.Size != 64 || again.Style != "film" {
		t.Fatalf("the envelope lost the placement: %+v", again)
	}
	// The projection hands it back, so a resave carries it rather than clearing it.
	placed := firstCaption(t, reopened).Resolved.InstanceID
	for _, e := range clip.CorrectionFromPlan(reopened).Elements {
		if e.InstanceID == placed && (e.Owner.Position == nil || *e.Owner.Position != at) {
			t.Fatalf("the projection dropped the placement: %+v", e.Owner)
		}
	}
}

// A plan written before ② could place a caption decodes with all three absent,
// which is automatic placement and the project's default style.
func TestAPlanWrittenBeforePlacementDecodesWithNone(t *testing.T) {
	_, plan := creationFixture(t)
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	// Strip every trace of the field from the stored bytes, which is exactly what
	// a plan saved before ② could place a caption looks like.
	stripped := strings.ReplaceAll(raw, `"Owner":{"Position":null,"Size":0,"Style":""},`, "")
	if strings.Contains(stripped, `"Owner":`) {
		t.Fatalf("the placement is stored in a shape this test does not remove: %s", raw)
	}
	decoded, err := clip.DecodeEditPlan(stripped)
	if err != nil {
		t.Fatal(err)
	}
	if owner := firstCaption(t, decoded).Owner; owner != (clip.OwnerCaption{}) {
		t.Fatalf("an older plan decoded with a placement: %+v", owner)
	}
}

// CDS-3's floor is refused where the size is written, and so is a style the
// product does not carry. The POSITION is clamped instead (CDS-82).
func TestSaveRefusesASizeOutsideTheRoleAndAnUnknownStyle(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	floor := int(design.Caption().Role().Min)
	for _, owner := range []clip.OwnerCaption{
		{Size: floor - 1},
		{Size: int(design.Caption().Role().Size) + 1},
		{Style: "no-such-style"},
	} {
		if _, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, owner)); err == nil {
			t.Fatalf("a save accepted %+v", owner)
		}
	}
	if _, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Size: floor})); err != nil {
		t.Fatalf("the floor itself was refused: %v", err)
	}
}

func TestSaveClampsAPlacementIntoTheSafeArea(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	safe, _ := design.Safe("vertical")
	at := clip.CaptionPlacement{X: -500, Y: 99999}
	next, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Position: &at}))
	if err != nil {
		t.Fatal(err)
	}
	stored := firstCaption(t, next).Owner.Position
	if stored == nil || float64(stored.X) != safe.X || float64(stored.Y) != safe.Y+safe.Height {
		t.Fatalf("the stored position was not clamped into %v: %+v", safe, stored)
	}
}

// Only a caption carries one: a fixed region, the badge and the two cards have
// no placement of their own, and a client that sends one is refused.
func TestOnlyACaptionMayCarryAPlacement(t *testing.T) {
	for _, role := range []string{"info", "hook", "ending", "badge"} {
		e := composition.Element{ID: role, Role: role}
		if _, err := clip.ValidateOwnerCaption(clip.OwnerCaption{Size: 64}, e, "vertical", nil); err == nil {
			t.Fatalf("%s was allowed its own placement", role)
		}
		if got, err := clip.ValidateOwnerCaption(clip.OwnerCaption{}, e, "vertical", nil); err != nil || got != (clip.OwnerCaption{}) {
			t.Fatalf("%s was refused for carrying nothing: %v %+v", role, err, got)
		}
	}
}

// CLIP-142, CDS-66: the owner may give a caption any approved style, inside the
// project's AI set or not. The set limits what a writer picks; changing it later
// leaves the owner's choice standing through a further correction, and it is
// the style the caption renders in.
func TestSaveAcceptsAnApprovedStyleOutsideTheAISet(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	p.CaptionStyles = []string{design.DefaultCaptionStyle}
	next, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Style: "neon"}))
	if err != nil {
		t.Fatal("an approved style outside the AI set was refused:", err)
	}
	if got := firstCaption(t, next).Owner.Style; got != "neon" {
		t.Fatalf("the owner's style was not stored: %q", got)
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan, p.CaptionStyles = raw, []string{"film"}
	again, err := clip.ApplyCorrection(cfg, p, creationDraft(reopened))
	if err != nil {
		t.Fatal("a further correction refused the kept style:", err)
	}
	caption := firstCaption(t, again)
	style, fallback := clip.CaptionStyleOf(caption, p.DesignSelection().AllowedCaptionStyles())
	if caption.Owner.Style != "neon" || style != "neon" || fallback {
		t.Fatalf("the owner's style did not survive the set change: %q renders %q (fallback %v)", caption.Owner.Style, style, fallback)
	}
}

// CDS-100: a size the newly chosen style can take is kept; one it cannot is
// refused by name, with the range that style admits and the size asked for, so
// the editor says what to correct instead of the size being reset.
func TestSaveRefusesASizeTheChosenStyleCannotTake(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	size := int(design.Caption().Role().Min)
	kept, err := clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Size: size, Style: "film"}))
	if err != nil || firstCaption(t, kept).Owner.Size != size {
		t.Fatalf("a size the new style takes was not kept: %v", err)
	}
	keynote, _ := design.CaptionRule("keynote")
	if float64(size) >= keynote.Role().Min {
		t.Fatal("the fixture needs a style whose floor is above the default's")
	}
	_, err = clip.ApplyCorrection(cfg, p, placedDraft(plan, clip.OwnerCaption{Size: size, Style: "keynote"}))
	var problem *composition.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("a size below the new style's floor was not refused by name: %v", err)
	}
	want := composition.Problem{ElementID: firstCaption(t, plan).Resolved.Element.ID, Line: firstCaption(t, plan).Resolved.Element.Span.Line,
		Reason: "caption_size", Min: int(keynote.Role().Min), Max: int(keynote.Role().Size), Actual: size}
	if *problem != want {
		t.Fatalf("the refusal does not say what to correct: %+v, want %+v", *problem, want)
	}
	if params := problem.FailureParams(); params["min"] != "72" || params["max"] != "84" || params["actual"] != "64" {
		t.Fatalf("the transport does not carry the range: %v", params)
	}
}

// CDS-82, CDS-100: the size is bounded by the style the caption renders in,
// which for a caption the owner gave no style is the one its plan names or, for
// one naming none, the AI set's first — never the default when it draws another.
func TestTheSizeIsBoundedByTheStyleTheCaptionRendersIn(t *testing.T) {
	keynote, _ := design.CaptionRule("keynote")
	low := int(design.Caption().Role().Min)
	for _, c := range []struct {
		name, style string
		allowed     []string
	}{
		{"named by the plan", "keynote", []string{design.DefaultCaptionStyle}},
		{"naming none", "auto", []string{"keynote"}},
	} {
		e := composition.Element{ID: "narration-1", Role: "caption", Style: c.style}
		_, err := clip.ValidateOwnerCaption(clip.OwnerCaption{Size: low}, e, "vertical", c.allowed)
		var problem *composition.Problem
		if !errors.As(err, &problem) || problem.Min != int(keynote.Role().Min) || problem.Max != int(keynote.Role().Size) {
			t.Fatalf("%s: a size below the drawn style's floor was admitted: %v", c.name, err)
		}
		if _, err := clip.ValidateOwnerCaption(clip.OwnerCaption{Size: int(keynote.Role().Min)}, e, "vertical", c.allowed); err != nil {
			t.Fatalf("%s: the drawn style's own floor was refused: %v", c.name, err)
		}
	}
}

// CLIP-142, CDS-66: a caption renders in the owner's choice, then the style its
// plan names — an approved one kept whatever the AI set says — and only a caption
// naming none takes the set's first entry; one the product does not carry falls
// back to it and says so.
func TestACaptionRendersInTheStyleItsPlanHolds(t *testing.T) {
	allowed := []string{"film"}
	for _, c := range []struct {
		style, owner, want string
		fallback           bool
	}{
		{"neon", "", "neon", false},
		{"auto", "", "film", false},
		{"", "", "film", false},
		{"retired-style", "", "film", true},
		{"neon", "keynote", "keynote", false},
	} {
		text := clip.PortableText{Owner: clip.OwnerCaption{Style: c.owner}}
		text.Resolved.Element = composition.Element{ID: "narration-1", Role: "caption", Style: c.style}
		if got, fallback := clip.CaptionStyleOf(text, allowed); got != c.want || fallback != c.fallback {
			t.Errorf("%+v renders %q (fallback %v)", c, got, fallback)
		}
	}
}

// CDS-66: a plan kept from an earlier attempt is held to the AI set of the
// generation that uses it, with the notice a narration's own fallback records;
// the owner's choices are not the writer's and are left alone.
func TestAGenerationHoldsItsWritersStylesToTheSetAllowedNow(t *testing.T) {
	caption := func(id, style, owner string) clip.PortableText {
		text := clip.PortableText{Owner: clip.OwnerCaption{Style: owner}}
		text.Resolved.Element = composition.Element{ID: id, Role: "caption", Style: style}
		return text
	}
	plan := clip.EditPlan{Portable: &clip.PortablePlan{Elements: []clip.PortableText{
		caption("narration-1", "neon", ""),
		caption("narration-2", "film", ""),
		caption("narration-3", "neon", "keynote"),
		caption("narration-4", "auto", ""),
	}}}
	clip.RestrictGeneratedCaptionStyles(&plan, []string{"film"})
	got := []string{}
	for _, text := range plan.Portable.Elements {
		got = append(got, text.Resolved.Element.Style)
	}
	if strings.Join(got, ",") != "film,film,neon,auto" {
		t.Fatalf("the writer's styles were not held to the set: %v", got)
	}
	if len(plan.Notices) != 1 || plan.Notices[0].ElementID != "narration-1" || plan.Notices[0].Reason != "composition_caption_style" {
		t.Fatalf("the fallback was not stated: %+v", plan.Notices)
	}
}

// CLIP-191: changing one caption's style keeps its exact words, its output
// interval and its phrases' intervals, and leaves every other caption and every
// cut as they were.
func TestAStyleChangeKeepsTheCaptionsWordsTimingAndEverythingElse(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, saved := creationFixture(t)
	// The same save without the style is what the style change is measured
	// against, so only what the style itself changes can differ.
	plan, err := clip.ApplyCorrection(cfg, p, creationDraft(saved))
	if err != nil {
		t.Fatal(err)
	}
	next, err := clip.ApplyCorrection(cfg, p, placedDraft(saved, clip.OwnerCaption{Style: "word-pop"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Portable.Elements) != len(plan.Portable.Elements) {
		t.Fatalf("the style change added or removed a text: %d → %d", len(plan.Portable.Elements), len(next.Portable.Elements))
	}
	styled := firstCaption(t, plan).Resolved.InstanceID
	for i, was := range plan.Portable.Elements {
		now := next.Portable.Elements[i]
		if now.Resolved.InstanceID != was.Resolved.InstanceID || now.Resolved.Text != was.Resolved.Text ||
			now.Resolved.StartMS != was.Resolved.StartMS || now.Resolved.EndMS != was.Resolved.EndMS ||
			!reflect.DeepEqual(now.Phrases, was.Phrases) {
			t.Fatalf("%s moved: %+v → %+v", was.Resolved.InstanceID, was.Resolved, now.Resolved)
		}
		want := clip.OwnerCaption{}
		if was.Resolved.InstanceID == styled {
			want.Style = "word-pop"
		}
		if now.Owner != want {
			t.Fatalf("%s carries %+v, want %+v", was.Resolved.InstanceID, now.Owner, want)
		}
	}
	if !reflect.DeepEqual(next.Cuts, plan.Cuts) {
		t.Fatal("the style change moved the footage")
	}
}
