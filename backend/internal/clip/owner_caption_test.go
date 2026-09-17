package clip_test

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
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
	cfg := config.ClipRender(&config.Config{})
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
// project does not allow (CLIP-142). The POSITION is clamped instead (CDS-82).
func TestSaveRefusesASizeBelowTheFloorAndAnUnallowedStyle(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	p, plan := creationFixture(t)
	floor := int(design.Caption().Role().Min)
	for _, owner := range []clip.OwnerCaption{
		{Size: floor - 1},
		{Size: int(design.Caption().Role().Size) + 1},
		{Style: "film"},
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
	cfg := config.ClipRender(&config.Config{})
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
	allowed := []string{design.DefaultCaptionStyle}
	for _, role := range []string{"info", "hook", "ending", "badge"} {
		if _, err := clip.ValidateOwnerCaption(clip.OwnerCaption{Size: 64}, role, "vertical", allowed); err == nil {
			t.Fatalf("%s was allowed its own placement", role)
		}
		if got, err := clip.ValidateOwnerCaption(clip.OwnerCaption{}, role, "vertical", allowed); err != nil || got != (clip.OwnerCaption{}) {
			t.Fatalf("%s was refused for carrying nothing: %v %+v", role, err, got)
		}
	}
}
