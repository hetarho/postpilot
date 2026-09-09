package rpc

import (
	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"reflect"
	"testing"
)

func TestCorrectionWireRoundtripAndConflict(t *testing.T) {
	s := &clip.CorrectionState{Plan: clip.CorrectionPlan{DurationMS: 15000, Cuts: []clip.CorrectionCut{{ID: "cut", SourceID: "source", Fingerprint: "fingerprint", StartMS: 500, EndMS: 15500, VolumePermille: 123, Copy: clip.Caption{Text: "정확한 <한글>", Position: "center", Style: "diary", Accent: "teal", StartMS: 200, EndMS: 3000}}}}, Sources: []clip.AnalysisSource{{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fingerprint", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}, Filename: "travel.mp4"}}, CopyStyles: []string{"diary"}, FadeMS: 200, MaxCuts: 100, MaxCopyRunes: 500, MinDurationMS: 15000, MaxDurationMS: 90000}
	wire := editingProto(s)
	raw, err := protojson.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded v1.ClipEditingState
	if err = protojson.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(correctionPlan(decoded.Plan), s.Plan) || decoded.Sources[0].Filename != "travel.mp4" || decoded.MaxDurationMs != 90000 {
		t.Fatal(decoded.String())
	}
	if correctionPlan(nil).DurationMS != 0 || editingProto(nil) != nil {
		t.Fatal("nil presence")
	}
	err = toConnectError(clip.ErrPlanConflict)
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal(err)
	}
	ce := err.(*connect.Error)
	detail, e := ce.Details()[0].Value()
	if e != nil || detail.(*v1.AppErrorDetail).Reason != "CLIP_PLAN_CONFLICT" {
		t.Fatal(detail, e)
	}
	if (&v1.SaveClipEditPlanRequest{}).ProtoReflect().Descriptor().Fields().ByName("user_id") != nil || (&v1.ClipEditPlan{}).ProtoReflect().Descriptor().Fields().ByName("ratio") != nil {
		t.Fatal("owner or ratio became editable")
	}
}
