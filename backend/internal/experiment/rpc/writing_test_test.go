package rpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/template"
)

type testActionsFake struct {
	calls    int
	user     string
	start    experiment.TestStart
	mutation experiment.TestMutation
	found    experiment.WritingTest
}

func TestWritingPublicationRefusalsRetainDomainLimitsAndSafeReasons(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{template.ErrTooMany, connect.CodeFailedPrecondition, "TEMPLATE_LIMIT_REACHED"},
		{template.ErrDuplicateName, connect.CodeAlreadyExists, "TEMPLATE_NAME_TAKEN"},
		{&guideline.AccountCapError{Max: 5}, connect.CodeFailedPrecondition, "GUIDELINE_LIMIT_REACHED"},
		{guideline.ErrScopeShape, connect.CodeInvalidArgument, "GUIDELINE_SCOPE_INVALID"},
		{guideline.ErrTemplateNotFound, connect.CodeNotFound, "GUIDELINE_TEMPLATE_NOT_FOUND"},
		{provider.ErrModelDisabled, connect.CodeFailedPrecondition, "MODEL_DISABLED"},
		{post.ErrPostBusy, connect.CodeFailedPrecondition, "POST_BUSY"},
		{post.ErrForbidden, connect.CodeNotFound, "POST_NOT_FOUND"},
		{experiment.ErrTestPublicationConflict, connect.CodeAborted, "WRITING_TEST_PUBLICATION_CONFLICT"},
	}
	for _, c := range cases {
		mapped := writingError(fmt.Errorf("private frozen profile: %w", c.err))
		if connect.CodeOf(mapped) != c.code || experimentAppErrorDetail(t, mapped).GetReason() != c.reason || strings.Contains(mapped.Error(), "private frozen") {
			t.Fatalf("error=%v detail=%+v", mapped, experimentAppErrorDetail(t, mapped))
		}
	}
}

func (f *testActionsFake) record(user string) { f.calls++; f.user = user }
func (f *testActionsFake) Estimate(_ context.Context, in experiment.TestStart) (experiment.TestQuote, error) {
	f.record(in.UserID)
	f.start = in
	return experiment.TestQuote{Key: "quote", Credits: 7, ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (f *testActionsFake) EstimateFailed(_ context.Context, in experiment.TestRetryQuoteRequest) (experiment.TestQuote, error) {
	f.record(in.UserID)
	return experiment.TestQuote{Key: "retry", Credits: 3, ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (f *testActionsFake) Start(_ context.Context, in experiment.TestStart) (experiment.WritingTest, error) {
	f.record(in.UserID)
	f.start = in
	return f.found, nil
}
func (f *testActionsFake) Get(_ context.Context, user, id string) (experiment.WritingTest, error) {
	f.record(user)
	if id != f.found.ID {
		return experiment.WritingTest{}, experiment.ErrTestNotFound
	}
	return f.found, nil
}
func (f *testActionsFake) List(_ context.Context, user string, _ int, _ string) ([]experiment.WritingTest, string, error) {
	f.record(user)
	return []experiment.WritingTest{f.found}, "next", nil
}
func (f *testActionsFake) Retry(_ context.Context, in experiment.TestRetry) (experiment.WritingTest, error) {
	f.record(in.UserID)
	f.mutation = in.TestMutation
	return f.found, nil
}
func (f *testActionsFake) Decide(_ context.Context, in experiment.MatchDecision) (experiment.WritingTest, error) {
	f.record(in.UserID)
	f.mutation = in.TestMutation
	return f.found, nil
}
func (f *testActionsFake) Cancel(_ context.Context, in experiment.TestMutation) (experiment.WritingTest, error) {
	f.record(in.UserID)
	f.mutation = in
	return f.found, nil
}
func (f *testActionsFake) SaveWinner(_ context.Context, in experiment.WinnerPublication) (experiment.TestPublication, experiment.WritingTest, error) {
	f.record(in.UserID)
	f.mutation = in.TestMutation
	return experiment.TestPublication{ID: "pub", TestID: in.TestID, Action: in.Action, Status: string(experiment.TestPublicationConfirmed)}, f.found, nil
}
func (f *testActionsFake) ApplyOutput(_ context.Context, in experiment.OutputApplication) (experiment.TestPublication, experiment.WritingTest, error) {
	f.record(in.UserID)
	f.mutation = in.TestMutation
	return experiment.TestPublication{ID: "pub", TestID: in.TestID, Action: string(experiment.TestApplyOutput), Status: string(experiment.TestPublicationConfirmed)}, f.found, nil
}
func rpcWritingTest() experiment.WritingTest {
	return experiment.WritingTest{ID: "owned", UserID: "alice", Factor: experiment.FactorModel, ModelStage: experiment.StageWrite, Count: 2, Status: experiment.TestQueued, Input: experiment.TestInput{TargetLanguage: "en"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}
func rpcTestPlan(factor v1.WritingTestFactor, count int) *v1.WritingTestPlan {
	p := &v1.WritingTestPlan{Factor: factor, Count: int32(count), Context: &v1.WritingTestContext{TargetLanguage: v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN, TagCount: 5, Material: &v1.WritingTestMaterial{Text: "explicit owner material", Fictional: true, TemplateAnswers: []*v1.TemplateAnswer{{Label: "experience", Text: "named fact", Enabled: true}}}}}
	for i := 0; i < count; i++ {
		e := &v1.WritingTestEntrant{}
		switch factor {
		case v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL:
			p.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_WRITE
			e.Source = &v1.WritingTestEntrant_Model{Model: &v1.ModelRef{ProviderId: "p", ModelId: fmt.Sprint(i)}}
		default:
			k := v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE
			if factor == v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE {
				k = v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE
			}
			if factor == v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE {
				k = v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE
			}
			e.Source = &v1.WritingTestEntrant_Setting{Setting: &v1.WritingTestSettingRef{Kind: k, Id: fmt.Sprint(i), Revision: "saved"}}
		}
		p.Entrants = append(p.Entrants, e)
	}
	return p
}
func TestWritingTestTransportAcceptsAllExactFormatsAndSavedAxesWithoutAdmittingAtEstimate(t *testing.T) {
	for _, factor := range []v1.WritingTestFactor{v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL, v1.WritingTestFactor_WRITING_TEST_FACTOR_VOICE, v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE, v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE} {
		for _, count := range []int{2, 4, 8, 16} {
			t.Run(fmt.Sprintf("%s/%d", factor, count), func(t *testing.T) {
				f := &testActionsFake{found: rpcWritingTest()}
				h := NewWritingTestHandler(f, f)
				ctx := auth.WithUser(context.Background(), "alice")
				plan := rpcTestPlan(factor, count)
				quote, err := h.EstimateWritingTest(ctx, connect.NewRequest(&v1.EstimateWritingTestRequest{Plan: plan}))
				if err != nil || quote.Msg.GetCredits() != 7 || f.start.RequestKey != "" || f.start.Count != count {
					t.Fatalf("estimate=%+v start=%+v err=%v", quote, f.start, err)
				}
				_, err = h.StartWritingTest(ctx, connect.NewRequest(&v1.StartWritingTestRequest{Plan: plan, RequestKey: "start", QuoteKey: "quote"}))
				if err != nil || f.start.RequestKey != "start" || f.start.QuoteKey != "quote" || f.user != "alice" {
					t.Fatalf("start=%+v err=%v", f.start, err)
				}
				for _, ref := range f.start.Entrants {
					if ref.SourceKind == "setting" {
						if ref.SettingKind != string(f.start.Factor) {
							t.Fatalf("setting kind=%s", ref.SettingKind)
						}
						wire, err := writingEntrantToProto(ref)
						if err != nil || !proto.Equal(wire, plan.Entrants[0]) && wire.GetSetting().GetKind() != plan.Entrants[0].GetSetting().GetKind() {
							t.Fatalf("round trip=%+v %v", wire, err)
						}
					}
				}
			})
		}
	}
}
func TestWritingTestTransportRejectsUnknownAndMismatchedWireBeforeService(t *testing.T) {
	valid := rpcTestPlan(v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE, 2)
	cases := map[string]func(*v1.WritingTestPlan){"factor": func(p *v1.WritingTestPlan) { p.Factor = 99 }, "stage": func(p *v1.WritingTestPlan) { p.ModelStage = 99 }, "setting kind": func(p *v1.WritingTestPlan) {
		p.Entrants[0].GetSetting().Kind = v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_TEMPLATE
	}, "language": func(p *v1.WritingTestPlan) { p.Context.TargetLanguage = 99 }, "quality": func(p *v1.WritingTestPlan) { p.Context.QualityRules = []v1.QualityMetric{99} }, "count3": func(p *v1.WritingTestPlan) { p.Count = 3 }, "duplicate": func(p *v1.WritingTestPlan) { p.Entrants[1] = proto.Clone(p.Entrants[0]).(*v1.WritingTestEntrant) }, "missing context": func(p *v1.WritingTestPlan) { p.Context = nil }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p := proto.Clone(valid).(*v1.WritingTestPlan)
			change(p)
			f := &testActionsFake{}
			h := NewWritingTestHandler(f, f)
			_, err := h.EstimateWritingTest(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.EstimateWritingTestRequest{Plan: p}))
			if err == nil || f.calls != 0 {
				t.Fatalf("invalid accepted %v calls=%d", err, f.calls)
			}
		})
	}
}
func TestWritingTestTransportPreservesObserverAxisAndMixedPreparedReferences(t *testing.T) {
	observer := rpcTestPlan(v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL, 2)
	observer.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE
	observer.Context.Material.AttachmentIds = []string{"owner-photo"}
	decoded, err := writingStartFromProto("alice", observer)
	if err != nil || decoded.ModelStage != experiment.StageObserve {
		t.Fatalf("observer decoded=%+v err=%v", decoded, err)
	}
	mixed := rpcTestPlan(v1.WritingTestFactor_WRITING_TEST_FACTOR_VOICE, 4)
	mixed.Entrants[1] = &v1.WritingTestEntrant{Source: &v1.WritingTestEntrant_AuthoringCandidate{AuthoringCandidate: &v1.WritingTestAuthoringRef{SessionId: "owned-session", CandidateId: "prepared-one", Revision: 7}}}
	mixed.Entrants[3] = &v1.WritingTestEntrant{Source: &v1.WritingTestEntrant_AuthoringCandidate{AuthoringCandidate: &v1.WritingTestAuthoringRef{SessionId: "owned-session", CandidateId: "prepared-two", Revision: 9}}}
	decoded, err = writingStartFromProto("alice", mixed)
	if err != nil || len(decoded.Entrants) != 4 || decoded.Entrants[1].AuthoringRevision != 7 || decoded.Entrants[3].AuthoringRevision != 9 {
		t.Fatalf("mixed decoded=%+v err=%v", decoded, err)
	}
	for i, ref := range decoded.Entrants {
		wire, err := writingEntrantToProto(ref)
		if err != nil || !proto.Equal(wire, mixed.Entrants[i]) {
			t.Fatalf("mixed ref%d wire=%+v err=%v", i, wire, err)
		}
	}
}
func TestWritingTestEveryEndpointAuthenticatesBeforePrivateAccess(t *testing.T) {
	f := &testActionsFake{}
	h := NewWritingTestHandler(f, f)
	calls := []func() error{func() error {
		_, e := h.EstimateWritingTest(context.Background(), connect.NewRequest(&v1.EstimateWritingTestRequest{}))
		return e
	}, func() error {
		_, e := h.EstimateFailedTestCandidates(context.Background(), connect.NewRequest(&v1.EstimateFailedTestCandidatesRequest{}))
		return e
	}, func() error {
		_, e := h.StartWritingTest(context.Background(), connect.NewRequest(&v1.StartWritingTestRequest{}))
		return e
	}, func() error {
		_, e := h.GetWritingTest(context.Background(), connect.NewRequest(&v1.GetWritingTestRequest{}))
		return e
	}, func() error {
		_, e := h.ListWritingTests(context.Background(), connect.NewRequest(&v1.ListWritingTestsRequest{}))
		return e
	}, func() error {
		_, e := h.RetryFailedTestCandidates(context.Background(), connect.NewRequest(&v1.RetryFailedTestCandidatesRequest{}))
		return e
	}, func() error {
		_, e := h.DecideTestMatch(context.Background(), connect.NewRequest(&v1.DecideTestMatchRequest{}))
		return e
	}, func() error {
		_, e := h.CancelWritingTest(context.Background(), connect.NewRequest(&v1.CancelWritingTestRequest{}))
		return e
	}, func() error {
		_, e := h.SaveWritingTestWinner(context.Background(), connect.NewRequest(&v1.SaveWritingTestWinnerRequest{}))
		return e
	}, func() error {
		_, e := h.ApplyWritingTestOutput(context.Background(), connect.NewRequest(&v1.ApplyWritingTestOutputRequest{}))
		return e
	}}
	for i, call := range calls {
		if err := call(); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("endpoint %d error=%v", i, err)
		}
	}
	if f.calls != 0 {
		t.Fatal("unauthenticated operation reached private service")
	}
}
func TestWritingTestBlindWireRetainsCompleteOutputAndOnlyRevealsIdentityAndTokensAtBoundary(t *testing.T) {
	output := experiment.TestOutput{ContentLanguage: "en", Content: experiment.TestOutputContent{Title: "Complete title", Summary: "Summary", Tags: []string{"tag"}, Blocks: []experiment.TestOutputBlock{{Type: "TEXT", Content: "full paragraph"}, {Type: "HEADING", Content: "heading", Level: 2}, {Type: "IMAGE", File: "a.jpg", Alt: "alt", Caption: "caption"}, {Type: "QUOTE", Content: "quote"}, {Type: "LIST", Items: []string{"one", "two"}}, {Type: "VIDEO", File: "clip.mp4"}, {Type: "GALLERY", Files: []string{"a.jpg", "b.jpg"}, Layout: "COLLAGE", Alt: "group"}, {Type: "GALLERY", Files: []string{"c.jpg", "d.jpg"}, Layout: "SLIDE"}}}, Storyline: &experiment.TestOutputStoryline{Paragraphs: []experiment.TestOutputParagraph{{Text: "complete plan", Files: []string{"a.jpg"}}}, MadeWith: []string{"a.jpg"}}}
	raw, err := experiment.EncodeTestOutput(output)
	if err != nil {
		t.Fatal(err)
	}
	found := rpcWritingTest()
	found.Status = experiment.TestReview
	found.CommonSnapshot = []byte("private common prompt")
	found.Candidates = []experiment.TestCandidate{{ID: "opaque", Status: "succeeded", Output: raw, FrozenVariant: []byte("private variant"), Accounting: []byte("suppliercost"), Identity: &experiment.TestCandidateIdentity{Label: "private model identity", Ref: experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "private-provider", ModelID: "private-model"}}}, Usage: &experiment.Usage{PromptTokens: 10, CompletionTokens: 20, LatencyMS: 30, CostMicrousd: 999}, Failure: &experiment.Failure{Reason: "UNKNOWN_FAILURE", Params: map[string]string{"model": "private-model"}, TechnicalDetail: "private diagnostic"}}}
	for _, status := range []experiment.TestStatus{experiment.TestReview, experiment.TestPartial, experiment.TestFailed, experiment.TestCompleted, experiment.TestCancelled} {
		found.Status = status
		wire, err := writingTestToProto(found)
		if err != nil {
			t.Fatal(err)
		}
		revealed := status == experiment.TestCompleted || status == experiment.TestCancelled
		if wire.GetRevealed() != revealed || (wire.Candidates[0].Identity != nil) != revealed || (wire.Candidates[0].Usage != nil) != revealed {
			t.Fatalf("status=%s reveal=%+v", status, wire)
		}
		candidate := wire.Candidates[0]
		if len(candidate.Output.Blocks) != 8 || candidate.Output.Blocks[6].Layout != v1.GalleryLayout_GALLERY_LAYOUT_COLLAGE || candidate.Output.Blocks[7].Layout != v1.GalleryLayout_GALLERY_LAYOUT_SLIDE || candidate.Storyline.Paragraphs[0].Text != "complete plan" {
			t.Fatalf("complete output lost %s", candidate)
		}
		encoded, _ := protojson.Marshal(wire)
		for _, secret := range []string{"private common", "private variant", "suppliercost", "999", "private diagnostic", "private-model"} {
			if secret == "private-model" && revealed {
				continue
			}
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("secret leaked status=%s secret=%s wire=%s", status, secret, encoded)
			}
		}
	}
}
