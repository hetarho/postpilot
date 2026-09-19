package clip_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestLegacyPortablePlanPreservesTextAndCardAbsence(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Cuts[0].Copies[0].Text = "정확한 <한글> & 🥣"
	r := clip.Recipe{Accent: "teal"}
	portable, err := clip.FreezeLegacyPlan(p, plan, r, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(portable.Elements) != 2 || portable.Elements[0].Resolved.Text != "정확한 <한글> & 🥣" {
		t.Fatal("empty legacy furniture invented", portable)
	}
	if portable.Elements[0].Resolved.StartMS != 120 || portable.Elements[0].Resolved.EndMS != 9880 || portable.Elements[1].Resolved.StartMS != 9920 {
		t.Fatal("cut/output clocks changed", portable.Elements)
	}
	if portable.Elements[0].Resolved.AuthoredTiming {
		t.Fatal("automatic timing became authored")
	}
	plan.Portable = portable
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil || !strings.Contains(raw, `"Version":6`) {
		t.Fatal(raw, err)
	}
	again, err := clip.DecodeEditPlan(raw)
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatal("version 6 lost snapshot/evidence", err, again.Portable)
	}
	for _, mutate := range []func(*clip.EditPlan){
		func(p *clip.EditPlan) { p.Portable.Elements[0].Resolved.EndMS = p.DurationMS + 1 },
		func(p *clip.EditPlan) { p.Portable.Cuts[0].SourceID = "foreign" },
		func(p *clip.EditPlan) { p.Portable.Elements[0].Evidence[0].Fingerprint = "different" },
		func(p *clip.EditPlan) {
			p.Portable.Elements[1].Resolved.InstanceID = p.Portable.Elements[0].Resolved.InstanceID
		},
	} {
		v, e := clip.DecodeEditPlan(raw)
		if e != nil {
			t.Fatal(e)
		}
		mutate(&v)
		if _, e := clip.EncodeEditPlan(v); e == nil {
			t.Fatal("inconsistent portable plan accepted")
		}
	}
}

func TestLegacyCardsBecomeExplicitWithoutSharedDisclosure(t *testing.T) {
	p, _ := correctionFixture(t)
	p.Disclosure = "sponsored"
	p.Answers = []clip.Answer{{Label: "상호", Text: "카페"}, {Label: "위치", Text: "서울"}, {Label: "가격", Text: "7,000원"}}
	plan, _ := clip.DecodeEditPlan(p.EditPlan)
	plan.Hook = "서울 카페"
	plan.Cuts[0].Chips = []string{"위치"}
	r := clip.Recipe{Preset: "cafe", Accent: "teal", InformationFields: []clip.InformationField{{Label: "상호", Prompt: "이름"}}}
	body := clip.LegacyCompositionBody(r)
	if strings.Contains(body, "협찬") || strings.Contains(body, "legacy-disclosure") {
		t.Fatal("shared template copied campaign", body)
	}
	if _, e := composition.Parse(body, clip.DefaultCompositionLimits()); e != nil {
		t.Fatal("invalid legacy template source", e, body)
	}
	portable, err := clip.FreezeLegacyPlan(p, plan, r, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]int{}
	for _, e := range portable.Elements {
		roles[e.Resolved.Element.Role]++
	}
	if roles["hook"] != 1 || roles["ending"] != 1 || roles["badge"] != 1 || roles["info"] != 1 {
		t.Fatal(roles, portable)
	}
	p.HideDisclosure = true
	hidden, err := clip.FreezeLegacyPlan(p, plan, r, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range hidden.Elements {
		if e.Resolved.Element.Role == "badge" {
			t.Fatal("hidden disclosure restored")
		}
	}
}

func TestDetachedLegacyProjectUsesDefaultDesignSelection(t *testing.T) {
	p, _ := correctionFixture(t)
	c := clip.LegacyProjectComposition(p, clip.Recipe{})
	if strings.Contains(c.Snapshot.Body, `styles=`) || !strings.Contains(c.Snapshot.Body, `intro="b"`) || !strings.Contains(c.Snapshot.Body, `outro="e"`) {
		t.Fatal(c.Snapshot.Body)
	}
}

func TestVersionSixIgnoresRetiredStylePermissions(t *testing.T) {
	project, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(project.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(raw, `{`, `{"Styles":["simple"],"CopyStyles":["clean"],`, 1)
	restored, err := clip.DecodeEditPlan(legacy)
	if err != nil || !reflect.DeepEqual(plan, restored) {
		t.Fatal("retired permissions changed saved content", err)
	}
}
