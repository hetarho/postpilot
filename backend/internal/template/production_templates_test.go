package template

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The checked-in candidate manifest is the exact shape applied to the six production rows.
// Parse it through the service on every backend test run before operators use the CAS script.
func TestValidateProductionTemplateCandidates(t *testing.T) {
	raw, err := os.ReadFile("../../../scripts/data/post-templates-20261001.json")
	if err != nil {
		t.Fatal(err)
	}
	var candidates map[string]struct {
		Description    string   `json:"description"`
		Body           string   `json:"body"`
		RequiredLabels []string `json:"required_labels"`
	}
	if err := json.Unmarshal(raw, &candidates); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 6 {
		t.Fatalf("template count = %d, want 6", len(candidates))
	}
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		candidate := candidates[name]
		t.Run(name, func(t *testing.T) {
			limits := testLimits()
			limits.AskMaxPerBody = 10
			svc := NewService(newFakeStore(), limits)
			created, err := svc.Create(context.Background(), "alice", Authored{Name: name, Description: candidate.Description, Body: candidate.Body})
			if err != nil {
				t.Fatalf("service save refused candidate: %v", err)
			}
			_, nodes, err := ParseTemplate(created.TitleArea, created.Body, ParseOptions{PhotoRowMax: 4, AskMaxPerBody: 10})
			if err != nil {
				t.Fatalf("parser refused saved candidate: %v", err)
			}
			asks := Asks(nodes)
			if len(asks) > 10 {
				t.Fatalf("question count = %d, limit 10", len(asks))
			}
			var required []string
			for _, ask := range asks {
				if ask.Required {
					required = append(required, Decode(ask.Label))
				}
			}
			if !reflect.DeepEqual(required, candidate.RequiredLabels) {
				t.Fatalf("required labels = %q, declaration = %q", required, candidate.RequiredLabels)
			}
			if len(asks) == 0 || !strings.HasSuffix(strings.TrimSpace(candidate.Body), asks[len(asks)-1].Source) {
				t.Fatal("the final section is not the last question")
			}
			if len(required) == 0 {
				t.Fatal("candidate has no required experience field")
			}
			_, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", created.ID, false, nil)
			var missing *RequiredAnswerError
			if ok || !errors.As(err, &missing) || missing.Label != required[0] {
				t.Fatalf("missing-answer gate = ok:%v err:%v, want %q", ok, err, required[0])
			}
			answers := make([]Answer, 0, len(required))
			for _, label := range required {
				answers = append(answers, Answer{Label: label, Text: "직접 확인한 내용", Enabled: true})
			}
			if _, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", created.ID, false, answers); err != nil || !ok {
				t.Fatalf("fully answered render = ok:%v err:%v", ok, err)
			}
			t.Logf("body=%d chars, questions=%d, required=%d, last=%q", len([]rune(candidate.Body)), len(asks), len(required), Decode(asks[len(asks)-1].Label))
		})
	}
}
