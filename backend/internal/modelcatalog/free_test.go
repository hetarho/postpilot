package modelcatalog_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

type freeQualifier struct {
	allowed bool
	path    llm.FreePath
}

func (q *freeQualifier) QualifyFree(_ context.Context, _ string, path llm.FreePath) (bool, error) {
	q.path = path
	return q.allowed, nil
}

func TestFreeCurationRequiresVerifiedPath(t *testing.T) {
	row := priced("vendor/free", "0", "0", modelcatalog.PurposePhotoAnalysis)
	store := newFakeStore(row)
	svc := newService(t, store, nil)
	free := modelcatalog.LevelFree
	patch := modelcatalog.Patch{Purpose: modelcatalog.PurposePhotoAnalysis, Level: &free}
	if _, err := svc.Update(context.Background(), row.ModelID, patch); !errors.Is(err, modelcatalog.ErrFreeIneligible) {
		t.Fatalf("unverified free path: %v", err)
	}
	qualifier := &freeQualifier{allowed: true}
	svc.SetFreeQualifier(qualifier)
	if _, err := svc.Update(context.Background(), row.ModelID, patch); err != nil {
		t.Fatal(err)
	}
	if qualifier.path != llm.FreeImageInput {
		t.Fatalf("qualified %s, want image input", qualifier.path)
	}
	if got := store.rows[row.ModelID].Levels[patch.Purpose]; got != free {
		t.Fatalf("stored level %q", got)
	}
}

func TestFreeBulkPreviewRejectsPaidPathWithoutWrites(t *testing.T) {
	store := newFakeStore(priced("vendor/paid", "1", "0", modelcatalog.PurposeWriting))
	candidate := candidate("vendor/paid", 1)
	candidate.InputUSDPerMillion, candidate.OutputUSDPerMillion = "1", "0"
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{candidate}})
	svc.SetFreeQualifier(&freeQualifier{allowed: true})
	text := modelcatalog.DocumentVersionLine + "\n[writing]\nvendor/paid free\n"
	plan, err := svc.PreviewDocument(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) != 1 || plan.Issues[0].Cause != modelcatalog.IssueFreeIneligible {
		t.Fatalf("issues = %+v", plan.Issues)
	}
	if store.syncs != 0 || strings.Contains(string(store.rows["vendor/paid"].Levels[modelcatalog.PurposeWriting]), "free") {
		t.Fatal("preview changed curation")
	}
}

func TestFreeBulkRoundTripPreservesClassification(t *testing.T) {
	row := priced("vendor/free", "0", "0", modelcatalog.PurposeWriting)
	row.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelFree}
	store := newFakeStore(row)
	candidate := candidate(row.ModelID, 1)
	candidate.InputUSDPerMillion, candidate.OutputUSDPerMillion = "0", "0"
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{candidate}})
	svc.SetFreeQualifier(&freeQualifier{allowed: true})
	exported, err := svc.ExportDocument(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exported, "vendor/free free") {
		t.Fatalf("free missing from export: %s", exported)
	}
	preview, err := svc.PreviewDocument(context.Background(), exported)
	if err != nil || len(preview.Issues) != 0 {
		t.Fatalf("free roundtrip issues=%+v err=%v", preview.Issues, err)
	}
	if store.syncs != 0 {
		t.Fatal("preview wrote curation")
	}
}
