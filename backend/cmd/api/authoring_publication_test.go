package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/authoring"
	authorstore "github.com/postpilot/backend/internal/authoring/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type publicationFields struct{}

func (publicationFields) Known(id string) bool { return id == "food" }

type publicationObjects struct{ clip.ObjectStore }
type publicationFinalizer struct{ clip.ProjectFinalizer }
type publicationJobs struct{ voice.Jobs }

func (publicationJobs) HasActiveForVoice(context.Context, string) (bool, error) { return false, nil }

type publicationHarness struct {
	*requestHarness
	targets     authoringTargets
	templates   *template.Service
	guides      *guideline.Service
	clipService *clipapp.Service
	voices      *voice.Service
	guideStore  *guidelinestore.Store
	voiceStore  *voicestore.Store
}

func newPublicationHarness(t *testing.T) *publicationHarness {
	t.Helper()
	base := newRequestHarness(t)
	templateStore := templatestore.New(base.d.Writer, base.d.Reader)
	guideStore := guidelinestore.New(base.d.Writer, base.d.Reader)
	voiceStore := voicestore.New(base.d.Writer, base.d.Reader)
	clipStore := clipstore.New(base.d.Writer, base.d.Reader)
	sourceService := clipapp.NewSourceService(clipStore, publicationObjects{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: time.Minute}))
	clips := clipapp.NewService(clipStore, clip.DefaultLimits(), sourceService, publicationFinalizer{})
	guides := guideline.NewService(guideStore, publicationFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: 50}, 30)
	guides.SetTemplateDirectory(guidelineTemplates{service: base.service})
	guides.SetVideoTemplateDirectory(guidelineVideoTemplates{service: clips})
	voices := voice.NewService(voiceStore, base.models, publicationJobs{})
	targets := authoringTargets{postTemplates: template.NewAuthoring(base.service, templateStore), videoTemplates: clipapp.NewAuthoring(clips, clipStore), guidelines: guideline.NewAuthoring(guides, guideStore), voices: voice.NewAuthoring(voices, voiceStore)}
	return &publicationHarness{requestHarness: base, targets: targets, templates: base.service, guides: guides, clipService: clips, voices: voices, guideStore: guideStore, voiceStore: voiceStore}
}
func publicationArtifact(kind authoring.Kind) authoring.Artifact {
	switch kind {
	case authoring.PostTemplate:
		return authoring.Artifact{Name: "정리한 이야기", Description: "친구에게 전하는 간단한 기록", Body: "<write>방문 이유</write>\n<write>좋았던 점</write>", TitleArea: "<write>글 제목</write>"}
	case authoring.VideoTemplate:
		return authoring.Artifact{Name: "짧은 방문 기록", Description: "방문 장면의 자연스러운 흐름", Body: `<clip version="1"><stage name="방문">도착과 분위기</stage><text id="hook" kind="ai" role="hook"><row kind="ai">소개 시작</row></text><text id="ending" kind="ai" role="ending"><row kind="ai">마무리</row></text></clip>`}
	case authoring.PostGuideline:
		return authoring.Artifact{Scope: publicationString("global"), Name: "사실과 느낌", Body: "확인한 사실과 자신의 느낌을 구분해서 설명해 주세요."}
	case authoring.VideoGuideline:
		return authoring.Artifact{Scope: publicationString("global"), Name: "간단한 영상 설명", Body: "장면에 보이는 내용 위주로 설명하고 같은 내용을 반복하지 마세요."}
	case authoring.WritingVoice:
		return authoring.Artifact{Name: "편안한 이야기", Description: "조용하고 편안하게 이야기하는 말투예요.", Body: strings.Repeat("산책하다 가상의 작은 가게에 들러 따뜻한 차를 마셨어요. 창가에서 잠깐 쉬니 마음이 편안했어요. ", 5)}
	default:
		panic("invalid test kind")
	}
}
func publicationInput(kind authoring.Kind) authoring.Publication {
	return authoring.Publication{Key: "publish:" + string(kind) + ":1", UserID: "alice", SessionID: "session-" + string(kind), Kind: kind, Revision: 1, Artifact: publicationArtifact(kind), MakeDefault: kind == authoring.WritingVoice, WriteModel: "p/m"}
}
func (h *publicationHarness) rename(t *testing.T, kind authoring.Kind, id, name string) {
	t.Helper()
	var err error
	ctx := context.Background()
	switch kind {
	case authoring.PostTemplate:
		_, err = h.templates.Update(ctx, "alice", id, template.Patch{Name: &name})
	case authoring.VideoTemplate:
		_, err = h.clipService.UpdateTemplate(ctx, "alice", id, clip.TemplatePatch{Name: &name})
	case authoring.PostGuideline, authoring.VideoGuideline:
		_, err = h.guides.Update(ctx, "alice", id, guideline.Patch{Title: &name})
	case authoring.WritingVoice:
		_, err = h.voices.RenameVoice(ctx, "alice", id, name)
	}
	if err != nil {
		t.Fatal(err)
	}
}
func (h *publicationHarness) remove(t *testing.T, kind authoring.Kind, id string) {
	t.Helper()
	var err error
	ctx := context.Background()
	switch kind {
	case authoring.PostTemplate:
		_, err = h.templates.Delete(ctx, "alice", id)
	case authoring.VideoTemplate:
		_, err = h.clipService.DeleteTemplate(ctx, "alice", id)
	case authoring.PostGuideline, authoring.VideoGuideline:
		err = h.guides.Delete(ctx, "alice", id)
	case authoring.WritingVoice:
		_, err = h.voices.DeleteVoice(ctx, "alice", id)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthoringPublicationFiveKindsAreConcurrentIdempotentAndDoNotResurrect(t *testing.T) {
	for _, kind := range []authoring.Kind{authoring.PostTemplate, authoring.VideoTemplate, authoring.PostGuideline, authoring.VideoGuideline, authoring.WritingVoice} {
		t.Run(string(kind), func(t *testing.T) {
			h := newPublicationHarness(t)
			ctx := context.Background()
			input := publicationInput(kind)
			if err := h.targets.CanStart(ctx, "alice", kind, ""); err != nil {
				t.Fatal(err)
			}
			if err := h.targets.Validate(kind, input.Artifact); err != nil {
				t.Fatal(err)
			}
			const workers = 6
			refs := make(chan authoring.SavedRef, workers)
			errs := make(chan error, workers)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); ref, err := h.targets.Publish(ctx, input); refs <- ref; errs <- err }()
			}
			wg.Wait()
			close(refs)
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			id := ""
			for ref := range refs {
				if id == "" {
					id = ref.ID
				}
				if ref.ID != id || ref.Kind != kind {
					t.Fatalf("duplicate/foreign ref=%+v expected=%s", ref, id)
				}
			}
			if id == "" || h.models.calls != 0 || len(h.admit.holds) != 0 {
				t.Fatalf("id=%s providerCalls=%d holds=%d", id, h.models.calls, len(h.admit.holds))
			}
			if _, err := h.targets.Seed(ctx, "bob", kind, id); !errors.Is(err, authoring.ErrNotFound) {
				t.Fatalf("foreign seed=%v", err)
			}
			if err := h.targets.CanStart(ctx, "bob", kind, id); !errors.Is(err, authoring.ErrNotFound) {
				t.Fatalf("foreign preflight=%v", err)
			}
			changed := "수동으로 바꾼 이름"
			h.rename(t, kind, id, changed)
			if kind == authoring.WritingVoice {
				if _, err := h.voices.SetDefaultVoice(ctx, "alice", ""); err != nil {
					t.Fatal(err)
				}
			}
			replay, err := h.targets.Publish(ctx, input)
			if err != nil || replay.ID != id || replay.Name != changed {
				t.Fatalf("replay=%+v err=%v", replay, err)
			}
			h.remove(t, kind, id)
			replay, err = h.targets.Publish(ctx, input)
			if kind == authoring.WritingVoice {
				current, getErr := h.voices.GetVoice(ctx, "alice", id)
				if err != nil || replay.ID != id || getErr != nil || !current.Deleted() || current.IsDefault {
					t.Fatalf("voice resurrection=%+v current=%+v err=%v getErr=%v", replay, current, err, getErr)
				}
			} else if !errors.Is(err, authoring.ErrNotFound) {
				t.Fatalf("deleted target resurrected=%+v err=%v", replay, err)
			}
		})
	}
}

func TestAuthoringTemplateAndGuidelineUpdatesKeepProtectedFieldsAndDetectDrift(t *testing.T) {
	h := newPublicationHarness(t)
	ctx := context.Background()
	length, tags := 1234, 6
	post, err := h.templates.Create(ctx, "alice", template.Authored{Name: "기존 구조", Body: "<write>예전 본문</write>", Numbers: template.Numbers{TargetLength: &length, TagCount: &tags}})
	if err != nil {
		t.Fatal(err)
	}
	video, err := h.clipService.CreateTemplate(ctx, "alice", clip.Recipe{Name: "기존 영상", CompositionBody: `<clip version="1"/>`}, clip.TemplateDesign{IntroPreset: "b", OutroPreset: "e", CaptionStyles: []string{"bold"}})
	if err != nil {
		t.Fatal(err)
	}
	postGuide, err := h.guides.Create(ctx, "alice", guideline.KindPost, "기존 지침", "방문 경험을 구체적으로 써 주세요.", guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{post.ID}}, "")
	if err != nil {
		t.Fatal(err)
	}
	videoGuide, err := h.guides.Create(ctx, "alice", guideline.KindClip, "기존 영상 지침", "영상 장면을 짧게 설명해 주세요.", guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{video.ID}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		kind authoring.Kind
		id   string
	}{{authoring.PostTemplate, post.ID}, {authoring.VideoTemplate, video.ID}, {authoring.PostGuideline, postGuide.ID}, {authoring.VideoGuideline, videoGuide.ID}} {
		seed, err := h.targets.Seed(ctx, "alice", entry.kind, entry.id)
		if err != nil {
			t.Fatal(err)
		}
		input := publicationInput(entry.kind)
		input.TargetID, input.TargetVersion = entry.id, seed.TargetVersion
		// Historical AI publications omit protected owner fields and retain scope.
		input.Artifact.Scope = nil
		if _, err := h.targets.Publish(ctx, input); err != nil {
			t.Fatal(err)
		}
		switch entry.kind {
		case authoring.PostTemplate:
			current, err := templatestore.New(h.d.Writer, h.d.Reader).Get(ctx, "alice", entry.id)
			if err != nil || current.TargetLength == nil || *current.TargetLength != length || current.TagCount == nil || *current.TagCount != tags {
				t.Fatalf("numbers changed=%+v err=%v", current, err)
			}
		case authoring.VideoTemplate:
			current, err := clipstore.New(h.d.Writer, h.d.Reader).GetTemplate(ctx, "alice", entry.id)
			if err != nil || !reflect.DeepEqual(current.Design, video.Design) {
				t.Fatalf("design changed=%+v err=%v", current, err)
			}
		default:
			current, err := h.guideStore.Get(ctx, "alice", entry.id)
			expected := post.ID
			if entry.kind == authoring.VideoGuideline {
				expected = video.ID
			}
			if err != nil || current.Scope != guideline.ScopeTemplates || len(current.TemplateIDs) != 1 || current.TemplateIDs[0] != expected {
				t.Fatalf("scope changed=%+v err=%v", current, err)
			}
		}
		h.rename(t, entry.kind, entry.id, "변경된 원본")
		input.Revision++
		input.Key = fmt.Sprintf("second:%s", entry.kind)
		if _, err := h.targets.Publish(ctx, input); !errors.Is(err, authoring.ErrTargetConflict) {
			t.Fatalf("stale %s applied: %v", entry.kind, err)
		}
	}
}

type publicationIdleAuthoringJobs struct{ authoring.Jobs }
type publicationIdleAuthoringBudget struct{ authoring.Budget }
type publicationIdleAuthoringEstimator struct{ authoring.Estimator }

func (h *publicationHarness) authoringService() *authoring.Service {
	return authoring.NewService(authorstore.New(h.d.Writer, h.d.Reader), h.models, publicationIdleAuthoringJobs{}, h.targets, publicationIdleAuthoringBudget{}, publicationIdleAuthoringEstimator{})
}

func TestAuthoringCanonicalMetadataKeepsSavedVersionAndAllowsTheNextEdit(t *testing.T) {
	for _, kind := range []authoring.Kind{authoring.PostTemplate, authoring.PostGuideline} {
		t.Run(string(kind), func(t *testing.T) {
			h := newPublicationHarness(t)
			ctx := context.Background()
			target, err := h.templates.Create(ctx, "alice", template.Authored{Name: "Number baseline", Body: "<write>Baseline</write>"})
			if err != nil {
				t.Fatal(err)
			}
			id := target.ID
			var scopeIDs []string
			if kind == authoring.PostGuideline {
				second, err := h.templates.Create(ctx, "alice", template.Authored{Name: "Second scope", Body: "<write>Second</write>"})
				if err != nil {
					t.Fatal(err)
				}
				guide, err := h.guides.Create(ctx, "alice", guideline.KindPost, "Scope baseline", "Preserve this direction", guideline.ScopePatch{Scope: guideline.ScopeGlobal}, "")
				if err != nil {
					t.Fatal(err)
				}
				id = guide.ID
				scopeIDs = []string{" " + second.ID + " ", target.ID, second.ID}
			}
			svc := h.authoringService()
			state, err := svc.Create(ctx, "alice", kind, id, "metadata-session")
			if err != nil || state.WorkingSource == nil {
				t.Fatalf("create=%+v %v", state, err)
			}
			source := *state.WorkingSource
			if kind == authoring.PostTemplate {
				source.TargetLength, source.TagCount = publicationString("+001200"), publicationString("007")
			} else {
				source.Scope = publicationString("templates")
				source.TemplateIDs = scopeIDs
				source.Fields = []string{}
			}
			state, err = svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "metadata-patch", WorkingSource: source})
			if err != nil || state.DraftState != authoring.DraftValid {
				t.Fatalf("patch=%+v %v", state, err)
			}
			state, err = svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "metadata-save"}, false)
			seed, seedErr := h.targets.Seed(ctx, "alice", kind, id)
			if err != nil || seedErr != nil || state.TargetVersion == "" || state.TargetVersion != seed.TargetVersion || state.HasUnpublishedChanges {
				t.Fatalf("canonical metadata lost saved version: state=%+v seed=%+v err=%v seedErr=%v", state, seed, err, seedErr)
			}
			// The saved version must authorize the owner's next edit. Empty and nil
			// scope links are the same shape once the chosen scope becomes global.
			source = *state.WorkingSource
			if kind == authoring.PostGuideline {
				source.Scope = publicationString("global")
				source.TemplateIDs, source.Fields = []string{}, []string{}
			} else {
				source.Body = "<write>Next owner edit</write>"
			}
			state, err = svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "next-patch", WorkingSource: source})
			if err != nil {
				t.Fatal(err)
			}
			state, err = svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "next-save"}, false)
			seed, seedErr = h.targets.Seed(ctx, "alice", kind, id)
			if err != nil || seedErr != nil || state.TargetVersion == "" || state.TargetVersion != seed.TargetVersion {
				t.Fatalf("next edit lost saved version: state=%+v err=%v seedErr=%v", state, err, seedErr)
			}
			if h.models.calls != 0 || len(h.admit.holds) != 0 {
				t.Fatal("manual save called or charged AI")
			}
		})
	}
}

func TestAuthoringVideoTemplateUpdatePreservesExistingProjectCompositionAndDesign(t *testing.T) {
	h := newPublicationHarness(t)
	ctx := context.Background()
	artifact := publicationArtifact(authoring.VideoTemplate)
	saved, err := h.clipService.CreateTemplate(ctx, "alice", clip.Recipe{Name: artifact.Name, CompositionBody: artifact.Body}, clip.TemplateDesign{IntroPreset: "b", OutroPreset: "e", CaptionStyles: []string{"bold"}})
	if err != nil {
		t.Fatal(err)
	}
	project, err := h.clipService.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "Existing project", VideoTemplateID: saved.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil || project.Composition == nil {
		t.Fatalf("create project=%+v err=%v", project, err)
	}
	seed, err := h.targets.Seed(ctx, "alice", authoring.VideoTemplate, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	input := publicationInput(authoring.VideoTemplate)
	input.TargetID, input.TargetVersion = saved.ID, seed.TargetVersion
	input.Artifact = *seed.Artifact
	input.Artifact.Body = strings.Replace(input.Artifact.Body, "도착과 분위기", "요청한 새 구성", 1)
	if _, err := h.targets.Publish(ctx, input); err != nil {
		t.Fatal(err)
	}
	current, err := h.clipService.GetProject(ctx, "alice", project.ID)
	if err != nil || !reflect.DeepEqual(current.Composition, project.Composition) || current.IntroPreset != project.IntroPreset || current.OutroPreset != project.OutroPreset || !reflect.DeepEqual(current.CaptionStyles, project.CaptionStyles) {
		t.Fatalf("authoring changed an existing project=%+v err=%v", current, err)
	}
	stored, err := clipstore.New(h.d.Writer, h.d.Reader).GetTemplate(ctx, "alice", saved.ID)
	if err != nil || stored.CompositionBody != input.Artifact.Body || !reflect.DeepEqual(stored.Design, saved.Design) {
		t.Fatalf("template structure/design=%+v err=%v", stored, err)
	}
	if h.models.calls != 0 || len(h.admit.holds) != 0 {
		t.Fatal("manual publication started or charged AI")
	}
}

func TestAuthoringWritingStylesAlwaysForkAndNeverUsePersonalMaterials(t *testing.T) {
	h := newPublicationHarness(t)
	ctx := context.Background()
	source, err := h.voices.CreateVoice(ctx, "alice", "내가 직접 쓴 말투")
	if err != nil {
		t.Fatal(err)
	}
	personal := strings.Repeat("이것은 다른 곳에 보내지 않는 비밀 학습 내용이에요. ", 10)
	if _, err := h.voices.AddSample(ctx, "alice", source.ID, "비밀", personal); err != nil {
		t.Fatal(err)
	}
	original := voice.Analysis{Origin: voice.OriginPersonal, AI: voice.AIPart{Impression: "편안하게 이야기하는 말투예요."}, AnalyzeModel: "p/analyze", CreatedAt: time.Now()}
	if err := h.voiceStore.PublishAnalysis(ctx, "alice", source.ID, original); err != nil {
		t.Fatal(err)
	}
	seed, err := h.targets.Seed(ctx, "alice", authoring.WritingVoice, source.ID)
	if err != nil || seed.Artifact.Body != "" || !seed.ForkVoice || strings.Contains(seed.SourceContext, "비밀 학습") {
		t.Fatalf("personal seed=%+v err=%v", seed, err)
	}
	if err := h.targets.Validate(authoring.WritingVoice, *seed.Artifact); err == nil {
		t.Fatal("unrefined personal seed is publishable")
	}
	input := publicationInput(authoring.WritingVoice)
	input.TargetID, input.TargetVersion = source.ID, seed.TargetVersion
	first, err := h.targets.Publish(ctx, input)
	if err != nil || first.ID == source.ID {
		t.Fatalf("personal source overwritten=%+v err=%v", first, err)
	}
	saved, err := h.voiceStore.CurrentAnalysis(ctx, "alice", first.ID)
	if err != nil || saved.Origin != voice.OriginSynthetic || saved.SyntheticSample != strings.TrimSpace(input.Artifact.Body) || len(saved.MaterialIDs) != 0 {
		t.Fatalf("synthetic saved=%+v err=%v", saved, err)
	}
	samples, err := h.voiceStore.ListSampleBodies(ctx, "alice", first.ID)
	if err != nil || len(samples) != 0 {
		t.Fatalf("AI samples leaked=%+v err=%v", samples, err)
	}
	unchanged, err := h.voiceStore.CurrentAnalysis(ctx, "alice", source.ID)
	if err != nil || unchanged.Origin != voice.OriginPersonal || unchanged.AI.Impression != original.AI.Impression {
		t.Fatalf("personal source changed=%+v err=%v", unchanged, err)
	}
	synthSeed, err := h.targets.Seed(ctx, "alice", authoring.WritingVoice, first.ID)
	if err != nil || synthSeed.Artifact.Body != strings.TrimSpace(input.Artifact.Body) {
		t.Fatalf("synthetic seed=%+v err=%v", synthSeed, err)
	}
	input.SessionID, input.Key, input.Revision = "refine-synthetic", "refine-synthetic:1", 1
	input.TargetID, input.TargetVersion = first.ID, synthSeed.TargetVersion
	second, err := h.targets.Publish(ctx, input)
	if err != nil || second.ID == first.ID || second.ID == source.ID {
		t.Fatalf("synthetic source overwritten=%+v err=%v", second, err)
	}
	if second.Name == first.Name {
		t.Fatal("collision suffix not used")
	}
	old, err := h.voiceStore.CurrentAnalysis(ctx, "alice", first.ID)
	if err != nil || old.SyntheticSample != saved.SyntheticSample {
		t.Fatalf("synthetic source changed=%+v err=%v", old, err)
	}
}

func TestAuthoringPublicationReceiptFailureRollsBackAllTargetWrites(t *testing.T) {
	for _, entry := range []struct {
		kind  authoring.Kind
		table string
	}{{authoring.PostTemplate, "template_authoring_publications"}, {authoring.VideoTemplate, "video_template_authoring_publications"}, {authoring.PostGuideline, "guideline_authoring_publications"}, {authoring.WritingVoice, "voice_authoring_publications"}} {
		t.Run(string(entry.kind), func(t *testing.T) {
			h := newPublicationHarness(t)
			ctx := context.Background()
			if _, err := h.d.Writer.Exec("CREATE TRIGGER force_receipt_failure BEFORE INSERT ON " + entry.table + " BEGIN SELECT RAISE(ABORT,'test receipt failure'); END"); err != nil {
				t.Fatal(err)
			}
			if _, err := h.targets.Publish(ctx, publicationInput(entry.kind)); err == nil {
				t.Fatal("failed receipt committed")
			}
			var count int
			entity := map[authoring.Kind]string{authoring.PostTemplate: "templates", authoring.VideoTemplate: "video_templates", authoring.PostGuideline: "guidelines", authoring.WritingVoice: "voices"}[entry.kind]
			if err := h.d.Reader.QueryRow("SELECT count(*) FROM " + entity + " WHERE user_id='alice'").Scan(&count); err != nil || count != 0 {
				t.Fatalf("partial entity count=%d err=%v", count, err)
			}
			if entry.kind == authoring.WritingVoice {
				if err := h.d.Reader.QueryRow("SELECT count(*) FROM voice_analyses WHERE user_id='alice'").Scan(&count); err != nil || count != 0 {
					t.Fatalf("partial voice snapshot count=%d err=%v", count, err)
				}
			}
		})
	}
}

func TestAuthoringPreflightCapsAndValidationAreDomainOwned(t *testing.T) {
	h := newPublicationHarness(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 50; i++ {
		if _, err := h.d.Writer.Exec("INSERT INTO templates(id,user_id,name,description,body,title_area,created_at,updated_at) VALUES (?,'alice',?,'','<write>본문</write>','',?,?)", fmt.Sprintf("limit-%d", i), fmt.Sprintf("구조%d", i), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := h.d.Writer.Exec("INSERT INTO guidelines(id,user_id,kind,title,text,scope,created_at,updated_at) VALUES (?,'alice','post','',?,'global',?,?)", fmt.Sprintf("rule-%d", i), fmt.Sprintf("기존지침 %d", i), now, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.targets.CanStart(ctx, "alice", authoring.PostTemplate, ""); !errors.Is(err, template.ErrTooMany) {
		t.Fatalf("template cap=%v", err)
	}
	var capError *guideline.AccountCapError
	if err := h.targets.CanStart(ctx, "alice", authoring.PostGuideline, ""); !errors.As(err, &capError) {
		t.Fatalf("guideline cap=%v", err)
	}
	if err := h.targets.CanStart(ctx, "alice", authoring.VideoGuideline, ""); err != nil {
		t.Fatalf("separate kind cap=%v", err)
	}
	for _, kind := range []authoring.Kind{authoring.PostTemplate, authoring.VideoTemplate, authoring.PostGuideline, authoring.VideoGuideline, authoring.WritingVoice} {
		value := publicationArtifact(kind)
		value.Body = ""
		if err := h.targets.Validate(kind, value); err == nil {
			t.Fatalf("empty body valid for %s", kind)
		}
	}
	malformed := publicationArtifact(authoring.PostTemplate)
	malformed.Body = "<repeat>broken"
	if err := h.targets.Validate(authoring.PostTemplate, malformed); err == nil {
		t.Fatal("invalid post grammar accepted")
	}
	malformed = publicationArtifact(authoring.VideoTemplate)
	malformed.Body = `<clip version="1"><scene id="bad"/></clip>`
	if err := h.targets.Validate(authoring.VideoTemplate, malformed); err == nil {
		t.Fatal("invalid saved video grammar accepted")
	}
	if h.models.calls != 0 {
		t.Fatal("preflight/validation called provider")
	}
}

func publicationString(v string) *string { return &v }
func TestAuthoringDirectNumbersAndScopeAreDomainValidatedAndCASPublished(t *testing.T) {
	h := newPublicationHarness(t)
	ctx := context.Background()
	post, e := h.templates.Create(ctx, "alice", template.Authored{Name: "Number baseline", Body: "<write>Baseline</write>"})
	if e != nil {
		t.Fatal(e)
	}
	seed, e := h.targets.Seed(ctx, "alice", authoring.PostTemplate, post.ID)
	if e != nil {
		t.Fatal(e)
	}
	source := *seed.Artifact
	source.TargetLength, source.TagCount = publicationString("1200"), publicationString("7")
	if e = h.targets.Validate(authoring.PostTemplate, source); e != nil {
		t.Fatal(e)
	}
	p := authoring.Publication{UserID: "alice", Key: "numbers-once", SessionID: "numbers", Kind: authoring.PostTemplate, Revision: 1, TargetID: post.ID, TargetVersion: seed.TargetVersion, Artifact: source}
	if _, e = h.targets.Publish(ctx, p); e != nil {
		t.Fatal(e)
	}
	_, current, _, e := h.targets.postTemplates.SeedWithNumbers(ctx, "alice", post.ID)
	if e != nil || current.TargetLength == nil || *current.TargetLength != 1200 || current.TagCount == nil || *current.TagCount != 7 {
		t.Fatal("owner numbers lost", e)
	}
	for _, bad := range []string{" ", "-1", "1200.5", "12000000000"} {
		source.TargetLength = publicationString(bad)
		if e = h.targets.Validate(authoring.PostTemplate, source); e == nil {
			t.Fatal("invalid owner number accepted", bad)
		}
	}
	guide, e := h.guides.Create(ctx, "alice", guideline.KindPost, "Scope baseline", "Keep it clear", guideline.ScopePatch{Scope: guideline.ScopeGlobal}, "")
	if e != nil {
		t.Fatal(e)
	}
	seed, e = h.targets.Seed(ctx, "alice", authoring.PostGuideline, guide.ID)
	if e != nil {
		t.Fatal(e)
	}
	source = *seed.Artifact
	source.Scope = publicationString("templates")
	source.TemplateIDs = []string{post.ID}
	p = authoring.Publication{UserID: "alice", Key: "scope-once", SessionID: "scope", Kind: authoring.PostGuideline, Revision: 1, TargetID: guide.ID, TargetVersion: seed.TargetVersion, Artifact: source}
	if _, e = h.targets.Publish(ctx, p); e != nil {
		t.Fatal(e)
	}
	rows, e := h.guideStore.List(ctx, "alice", guideline.KindPost)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[0].Scope != guideline.ScopeTemplates || !reflect.DeepEqual(rows[0].TemplateIDs, []string{post.ID}) {
		t.Fatal("explicit scope lost", rows)
	}
	source.Scope = publicationString("global")
	if e = h.targets.Validate(authoring.PostGuideline, source); e == nil {
		t.Fatal("contradictory scope shape accepted")
	}
	source.Scope = publicationString("templates")
	source.TemplateIDs = []string{"foreign"}
	p.Key = "scope-foreign"
	p.Artifact = source
	if _, e = h.targets.Publish(ctx, p); e == nil {
		t.Fatal("foreign scope accepted")
	}
}
