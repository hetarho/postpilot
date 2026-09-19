package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

// The catalog as the handler's service reads it. Only the reads this edge makes are real;
// the rest answer "nothing", because what is under test is the mapping, not the store.
type fakeStore struct {
	rows     map[string]modelcatalog.Model
	combos   []modelcatalog.ComboAssignment
	patched  modelcatalog.Patch
	purpose  modelcatalog.Purpose
	register bool
	listErr  error
}

func (f *fakeStore) List(context.Context) ([]modelcatalog.Model, error) {
	out := make([]modelcatalog.Model, 0, len(f.rows))
	for _, row := range f.rows {
		out = append(out, row)
	}
	return out, f.listErr
}
func (f *fakeStore) Get(_ context.Context, modelID string) (modelcatalog.Model, error) {
	row, ok := f.rows[modelID]
	if !ok {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	return row, nil
}
func (f *fakeStore) Upsert(_ context.Context, m modelcatalog.Model) error {
	f.rows[m.ModelID] = m
	return nil
}
func (f *fakeStore) Patch(_ context.Context, modelID string, patch modelcatalog.Patch, _ time.Time) (modelcatalog.Model, error) {
	row, ok := f.rows[modelID]
	if !ok {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	f.patched = patch
	if patch.Level != nil && patch.Purpose != "" {
		if row.Levels == nil {
			row.Levels = map[modelcatalog.Purpose]modelcatalog.Level{}
		}
		row.Levels[patch.Purpose] = *patch.Level
	}
	return row, nil
}
func (f *fakeStore) RegisterPurpose(_ context.Context, m modelcatalog.Model, purpose modelcatalog.Purpose) error {
	f.purpose, f.register = purpose, true
	row := f.rows[m.ModelID]
	row.Purposes = append(row.Purposes, purpose)
	f.rows[m.ModelID] = row
	return nil
}
func (f *fakeStore) DeregisterPurpose(_ context.Context, modelID string, purpose modelcatalog.Purpose, _ time.Time) error {
	f.purpose, f.register = purpose, false
	return nil
}
func (f *fakeStore) RefreshAvailability(context.Context, []modelcatalog.Candidate, time.Time) error {
	return nil
}
func (f *fakeStore) SyncPurposes(context.Context, []modelcatalog.PurposeWrite, time.Time) error {
	return nil
}
func (f *fakeStore) ListCombos(context.Context) ([]modelcatalog.ComboAssignment, error) {
	return f.combos, nil
}
func (f *fakeStore) AssignCombo(context.Context, modelcatalog.ComboAssignment, time.Time) error {
	return nil
}

func curated() modelcatalog.Model {
	return modelcatalog.Model{
		ModelID: "openai/gpt-5", ProviderSlug: "openai", Label: "GPT-5",
		Vision: true, StructuredOutput: true, ContextTokens: 400000,
		InputUSDPerMillion: "1.25", OutputUSDPerMillion: "10",
		Listed:   true,
		Purposes: []modelcatalog.Purpose{modelcatalog.PurposeWriting},
		Levels:   map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelPremium},
	}
}

func service(t *testing.T, store *fakeStore) *modelcatalog.Service {
	t.Helper()
	svc := modelcatalog.NewService(store)
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return svc
}

func detail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("not a connect error: %v", err)
	}
	for _, d := range connectErr.Details() {
		value, decodeErr := d.Value()
		if decodeErr != nil {
			continue
		}
		if app, ok := value.(*postpilotv1.AppErrorDetail); ok {
			return app
		}
	}
	t.Fatalf("no app error detail on %v", err)
	return nil
}

func TestListCatalogProjectsEveryCuratedFieldAndItsComboAssignments(t *testing.T) {
	store := &fakeStore{
		rows: map[string]modelcatalog.Model{"openai/gpt-5": curated()},
		combos: []modelcatalog.ComboAssignment{{
			Combo: modelcatalog.Combo("balanced"), ObserveModelID: "openai/gpt-5", WriteModelID: "openai/gpt-5",
		}},
	}
	res, err := NewHandler(service(t, store)).ListCatalog(context.Background(),
		connect.NewRequest(&postpilotv1.ListCatalogRequest{Purpose: string(modelcatalog.PurposeWriting)}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	entries := res.Msg.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	entry := entries[0]
	if entry.GetModelId() != "openai/gpt-5" || entry.GetProviderSlug() != "openai" || entry.GetLabel() != "GPT-5" {
		t.Fatalf("identity = %+v", entry)
	}
	if !entry.GetVision() || !entry.GetStructuredOutput() || entry.GetContextTokens() != 400000 {
		t.Fatalf("capabilities = %+v", entry)
	}
	if entry.GetInputUsdPerMillion() != "1.25" || entry.GetOutputUsdPerMillion() != "10" {
		t.Fatalf("pricing = %+v", entry)
	}
	if !entry.GetCurated() || entry.GetLevel() != string(modelcatalog.LevelPremium) ||
		len(entry.GetPurposes()) != 1 {
		t.Fatalf("curation = %+v", entry)
	}
	// The estimator assignments ride this read so the operator's screen needs no second
	// procedure (QUOTA-25): every combo is named, assigned or not.
	assigned := map[string]string{}
	for _, combo := range res.Msg.GetEstimatorCombos() {
		assigned[combo.GetCombo()] = combo.GetWriteModelId()
	}
	if assigned["balanced"] != "openai/gpt-5" {
		t.Fatalf("combos = %+v", res.Msg.GetEstimatorCombos())
	}
	if _, named := assigned["value"]; !named {
		t.Fatalf("an unassigned combo went unnamed: %+v", assigned)
	}
}

func TestListCatalogRefusesAPurposeNobodyDeclared(t *testing.T) {
	store := &fakeStore{rows: map[string]modelcatalog.Model{"openai/gpt-5": curated()}}
	_, err := NewHandler(service(t, store)).ListCatalog(context.Background(),
		connect.NewRequest(&postpilotv1.ListCatalogRequest{Purpose: "sorcery"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || detail(t, err).GetReason() != "MODEL_PURPOSE_INVALID" {
		t.Fatalf("unknown purpose = %v", err)
	}
}

func TestPurposeAndCurationWritesAnswerWithTheRowTheyWrote(t *testing.T) {
	store := &fakeStore{rows: map[string]modelcatalog.Model{"openai/gpt-5": curated()}}
	h := NewHandler(service(t, store))

	res, err := h.SetModelPurpose(context.Background(), connect.NewRequest(&postpilotv1.SetModelPurposeRequest{
		ModelId: "openai/gpt-5", Purpose: string(modelcatalog.PurposeStyleAnalysis), Registered: true,
	}))
	if err != nil {
		t.Fatalf("set purpose: %v", err)
	}
	if store.purpose != modelcatalog.PurposeStyleAnalysis || !store.register {
		t.Fatalf("store saw %q registered=%v", store.purpose, store.register)
	}
	// The answer is projected for the purpose that was written, so the row the client
	// re-renders carries that tab's effort and not another's.
	if res.Msg.GetEntry().GetModelId() != "openai/gpt-5" {
		t.Fatalf("entry = %+v", res.Msg.GetEntry())
	}

	level := "value"
	effort := "low"
	if _, err := h.UpdateModel(context.Background(), connect.NewRequest(&postpilotv1.UpdateModelRequest{
		ModelId: "openai/gpt-5", Purpose: string(modelcatalog.PurposeWriting), Level: &level, ReasoningEffort: &effort,
	})); err != nil {
		t.Fatalf("update: %v", err)
	}
	if store.patched.Level == nil || *store.patched.Level != modelcatalog.LevelValue {
		t.Fatalf("level patch = %+v", store.patched.Level)
	}
	if store.patched.Reasoning == nil || *store.patched.Reasoning != llm.ReasoningEffort("low") {
		t.Fatalf("reasoning patch = %+v", store.patched.Reasoning)
	}
	if store.patched.Purpose != modelcatalog.PurposeWriting {
		t.Fatalf("patch purpose = %q", store.patched.Purpose)
	}
}

func TestAModelIdIsRequiredBeforeAnythingIsRead(t *testing.T) {
	h := NewHandler(nil)
	for name, err := range map[string]error{
		"set purpose": func() error {
			_, err := h.SetModelPurpose(context.Background(), connect.NewRequest(&postpilotv1.SetModelPurposeRequest{}))
			return err
		}(),
		"update": func() error {
			_, err := h.UpdateModel(context.Background(), connect.NewRequest(&postpilotv1.UpdateModelRequest{}))
			return err
		}(),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument || detail(t, err).GetReason() != "MODEL_ID_REQUIRED" {
			t.Errorf("%s without an id = %v", name, err)
		}
	}
}

func TestEveryDomainRefusalHasItsOwnCodeAndReason(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{modelcatalog.ErrNotFound, connect.CodeNotFound, "MODEL_NOT_FOUND"},
		{modelcatalog.ErrInvalidReasoning, connect.CodeInvalidArgument, "MODEL_REASONING_INVALID"},
		{modelcatalog.ErrUnknownPurpose, connect.CodeInvalidArgument, "MODEL_PURPOSE_INVALID"},
		{modelcatalog.ErrPurposeIneligible, connect.CodeFailedPrecondition, "MODEL_PURPOSE_INELIGIBLE"},
		{modelcatalog.ErrPurposeNotRegistered, connect.CodeFailedPrecondition, "MODEL_PURPOSE_NOT_REGISTERED"},
		{errors.New("catalog on fire"), connect.CodeInternal, "UNKNOWN_FAILURE"},
	}
	for _, test := range cases {
		err := toConnectError("list catalog", test.err)
		if connect.CodeOf(err) != test.code || detail(t, err).GetReason() != test.reason {
			t.Errorf("%v → code=%s reason=%s", test.err, connect.CodeOf(err), detail(t, err).GetReason())
		}
	}
}

func TestDocumentPlansAndIssuesCrossTheEdgeLineByLine(t *testing.T) {
	plan := toProtoDocumentPlan([]modelcatalog.DocumentPurposePlan{{
		Purpose: modelcatalog.PurposeWriting, Register: []string{"a"}, Deregister: []string{"b"},
		Unchanged: []string{"c"},
		Relevel: []modelcatalog.LevelChange{{
			ModelID: "a", From: modelcatalog.LevelValue, To: modelcatalog.LevelPremium,
		}},
	}})
	if len(plan) != 1 || plan[0].GetPurpose() != string(modelcatalog.PurposeWriting) {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan[0].GetRelevel()) != 1 || plan[0].GetRelevel()[0].GetTo() != "premium" {
		t.Fatalf("relevel = %+v", plan[0].GetRelevel())
	}
	issues := toProtoDocumentIssues([]modelcatalog.DocumentIssue{{Line: 7, Text: "openai/ghost", Cause: "unknown_model"}})
	if len(issues) != 1 || issues[0].GetLine() != 7 || issues[0].GetCause() != "unknown_model" {
		t.Fatalf("issues = %+v", issues)
	}
}
