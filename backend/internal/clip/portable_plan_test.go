package clip_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// A version-6 plan keeps its portable composition exactly — text, output
// clocks, evidence — and refuses one whose composition contradicts its cuts.
func TestPortablePlanRoundTripsAndRefusesInconsistency(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Cuts[0].Copies[0].Text = "정확한 <한글> & 🥣"
	portable := nativePortable(plan)
	if len(portable.Elements) != 2 || portable.Elements[0].Resolved.Text != "정확한 <한글> & 🥣" {
		t.Fatal("the copies did not become one caption each", portable)
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
