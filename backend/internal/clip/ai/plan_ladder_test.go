package ai

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestEveryGeneratedCheckDeclaresItsLadderTier(t *testing.T) {
	// Check literal validators too: adding a new check without classifying it
	// fails this test, so no new rule silently becomes fatal.
	for _, folder := range []string{".", ".."} {
		files, err := os.ReadDir(folder)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") || file.Name() == "attempt_diagnostics.go" {
				continue
			}
			tree, err := parser.ParseFile(token.NewFileSet(), filepath.Join(folder, file.Name()), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(tree, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				fn, ok := call.Fun.(*ast.Ident)
				if !ok || (fn.Name != "outputError" && fn.Name != "planViolation") {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok {
					return true
				}
				code, _ := strconv.Unquote(lit.Value)
				if strings.HasPrefix(code, "observe_") {
					return true
				}
				if _, ok := planCheckTiers[code]; !ok {
					t.Errorf("%s: %s has no ladder tier", file.Name(), code)
				}
				return true
			})
		}
	}
	for code, tier := range planCheckTiers {
		t.Run(code, func(t *testing.T) {
			if tierForPlan(code) != tier || clip.SafeAttemptCheck(code) != code {
				t.Fatal("unclassified or unsafe check")
			}
		})
	}
	if tierForPlan("future_readable_rule") == failPlan {
		t.Fatal("unclassified readable rules must not default to failure")
	}
}

func TestNarrowingOutcomeTable(t *testing.T) {
	for _, tc := range []struct {
		code    string
		mutate  func(*clip.EditPlan, *clip.PlanningInput)
		removed bool
	}{
		{"plan_focal", func(p *clip.EditPlan, _ *clip.PlanningInput) { p.Cuts[0].Focal.X = 2 }, false},
		{"plan_volume", func(p *clip.EditPlan, _ *clip.PlanningInput) { v := 2.0; p.Cuts[0].Volume = &v }, false},
		{"plan_cut_rate", func(p *clip.EditPlan, _ *clip.PlanningInput) { p.Cuts[0].PlaybackRatePermille = 1234 }, false},
		{"plan_cut_scene", func(p *clip.EditPlan, in *clip.PlanningInput) {
			p.Cuts[0].EndMS = 16000
			in.Analyses[0].Segments[0].EndMS = 10000
		}, false},
		{"plan_cut_usability", func(p *clip.EditPlan, in *clip.PlanningInput) {
			p.Cuts[0].PlaybackRatePermille = 2000
			in.Analyses[0].Segments[0].Certainty = clip.CertaintyUncertain
		}, false},
		{"plan_cut_usability", func(_ *clip.EditPlan, in *clip.PlanningInput) {
			in.Analyses[0].Segments[0].Certainty = clip.CertaintyUnknown
		}, true},
		{"plan_source", func(p *clip.EditPlan, _ *clip.PlanningInput) { p.Cuts[0].SourceID = "missing" }, true},
		{"plan_caption_time", func(p *clip.EditPlan, _ *clip.PlanningInput) {
			p.Cuts[0].Copies = []clip.Copy{{Text: "exact words", StartMS: -10, EndMS: 30000}}
		}, false},
		{"composition_observation_gap", func(p *clip.EditPlan, in *clip.PlanningInput) { in.Analyses[0].Segments = nil }, true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			in := clip.PlanningInput{Ratio: "vertical", Analyses: []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000}}}, Segments: []clip.Segment{{EndMS: 30000, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}}}}
			plan := clip.EditPlan{Ratio: "vertical", Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "fp", EndMS: 15000}}}
			tc.mutate(&plan, &in)
			narrowGeneratedCuts(Config{Render: clip.RenderConfig{MaxCuts: 100}}, in, &plan)
			if (len(plan.Cuts) == 0) != tc.removed || len(plan.Notices) != 1 || plan.Notices[0].Reason != tc.code {
				t.Fatalf("wrong ladder outcome: %+v", plan)
			}
			if len(plan.Cuts) > 0 && len(plan.Cuts[0].Copies) > 0 && plan.Cuts[0].Copies[0].Text != "exact words" {
				t.Fatal("repair rewrote text")
			}
		})
	}
}
