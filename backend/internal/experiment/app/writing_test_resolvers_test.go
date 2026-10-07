package app

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/authoring"
	authoringstore "github.com/postpilot/backend/internal/authoring/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type resolverUnusedPostDeps struct {
	post.ObjectStore
	post.ActiveJobFinder
	post.PendingExperimentFinder
	post.ExperimentContentPurger
	post.GuidelineCandidateDetacher
	post.MemorySourceDetacher
}

func (resolverUnusedPostDeps) DetachPost(context.Context, string, string) error {
	panic("resolver mutated canonical source links")
}

type resolverFields struct{}

func (resolverFields) Known(value string) bool { return value == "restaurant" }

type resolverVoiceDirectory struct{}

func (resolverVoiceDirectory) Voices(context.Context, string) ([]post.VoiceRef, error) {
	return nil, nil
}

type resolverVoiceJobs struct{ voice.Jobs }

func (resolverVoiceJobs) ActiveForVoiceKind(context.Context, string, string) (*voice.ActiveJob, error) {
	return nil, nil
}

type resolverModelCatalog struct{ infos []llm.ModelInfo }

func (c resolverModelCatalog) Models() []llm.ModelInfo { return c.infos }
func (c resolverModelCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	for _, info := range c.infos {
		if info.Ref == ref {
			return info, true
		}
	}
	return llm.ModelInfo{}, false
}

type resolverCredits struct{}

func (resolverCredits) ForCalls([]provider.PlannedCall) int                { return 0 }
func (resolverCredits) Balance(context.Context, string) (int, bool, error) { return 1000, true, nil }

type resolverTemplateDirectory struct{ s *template.Service }

func (d resolverTemplateDirectory) Templates(ctx context.Context, user string) ([]guideline.TemplateRef, error) {
	found, err := d.s.List(ctx, user)
	if err != nil {
		return nil, err
	}
	var result []guideline.TemplateRef
	for _, t := range found {
		result = append(result, guideline.TemplateRef{ID: t.ID, Name: t.Name})
	}
	return result, nil
}

type resolverAuthoringUnused struct {
	authoring.Jobs
	authoring.Budget
	authoring.Estimator
}
type resolverAuthoringTargets struct {
	authoring.Targets
	templates  *template.Authoring
	guidelines *guideline.Authoring
	styles     *voice.Authoring
}

func (t resolverAuthoringTargets) Validate(kind authoring.Kind, a authoring.Artifact) error {
	switch kind {
	case authoring.PostTemplate:
		_, err := t.templates.Validate(template.Draft{Name: a.Name, Description: a.Description, Body: a.Body, TitleArea: a.TitleArea})
		return err
	case authoring.PostGuideline:
		_, err := t.guidelines.Validate(guideline.AuthoringDraft{Name: a.Name, Body: a.Body})
		return err
	case authoring.WritingVoice:
		_, err := t.styles.Validate(voice.WritingStyleDraft{Name: a.Name, Description: a.Description, Sample: a.Body})
		return err
	}
	return authoring.ErrDraftInvalid
}

type resolverPostSpy struct {
	*post.WritingTestMaterials
	sourceReads int
}

func (p *resolverPostSpy) AttachedImages(ctx context.Context, user, slug string) (post.Post, error) {
	p.sourceReads++
	return p.WritingTestMaterials.AttachedImages(ctx, user, slug)
}

type resolverHarness struct {
	resolver           *WritingTestResolvers
	db                 *db.DB
	posts              *poststore.Store
	postSpy            *resolverPostSpy
	voices             *voicestore.Store
	voiceService       *voice.Service
	templates          *template.Service
	guidelines         *guideline.Service
	authoring          *authoring.Service
	authoringStore     *authoringstore.Store
	templateAuthoring  *template.Authoring
	guidelineAuthoring *guideline.Authoring
	at                 time.Time
}

func actualResolverHarness(t *testing.T) *resolverHarness {
	t.Helper()
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "resolver.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	stamp := at.Format(time.RFC3339Nano)
	for _, user := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)", user, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,'Owned voice',1,?,?)", "voice-"+user, user, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	posts := poststore.New(handle.Writer, handle.Reader)
	unused := resolverUnusedPostDeps{}
	postService := post.NewService(posts, unused, post.Limits{MaxPhotos: 20, MaxVideos: 5, AnswerLabelMax: 40, AnswerValueMax: 4000}, post.Deps{Jobs: unused, Voices: resolverVoiceDirectory{}, Experiments: unused, ContentPurger: unused, CandidateLinks: unused, MemoryLinks: unused, Fields: resolverFields{}})
	postSpy := &resolverPostSpy{WritingTestMaterials: post.NewWritingTestMaterials(postService, posts)}
	templatesStore := templatestore.New(handle.Writer, handle.Reader)
	templates := template.NewService(templatesStore, template.NewLimits(template.Ceilings{NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200, MaxPerAccount: 50, PhotoRowMax: 3, AskLabelMaxChars: 40, AskMaxPerBody: 10}, template.NumberBounds{TargetLengthMin: post.TargetLengthMin, TargetLengthMax: post.TargetLengthMax, TagCountMin: post.TagCountRange.Min, TagCountMax: post.TagCountRange.Max}))
	templateAuthoring := template.NewAuthoring(templates, templatesStore)
	guidelineStore := guidelinestore.New(handle.Writer, handle.Reader)
	guidelines := guideline.NewService(guidelineStore, resolverFields{}, guideline.Limits{TitleMaxChars: 40, TextMaxChars: 300, MaxPerAccount: 50}, 50)
	guidelines.SetTemplateDirectory(resolverTemplateDirectory{templates})
	guidelineAuthoring := guideline.NewAuthoring(guidelines, guidelineStore)
	voices := voicestore.New(handle.Writer, handle.Reader)
	voiceService := voice.NewService(voices, nil, resolverVoiceJobs{})
	styleFactory := voice.NewAuthoring(voiceService, voices)
	catalog := resolverModelCatalog{infos: []llm.ModelInfo{{Ref: llm.ModelRef{ProviderID: "approved", ModelID: "writer"}, Label: "Frozen writer", Stages: []string{"write"}, StructuredOutput: true, ContextTokens: 131072}}}
	models := provider.NewService(providerstore.New(handle.Writer, handle.Reader), catalog, resolverCredits{})
	authoringStore := authoringstore.New(handle.Writer, handle.Reader)
	authoringUnused := resolverAuthoringUnused{}
	authoringService := authoring.NewService(authoringStore, &adapterLLM{}, authoringUnused, resolverAuthoringTargets{templates: templateAuthoring, guidelines: guidelineAuthoring, styles: styleFactory}, authoringUnused, authoringUnused)
	resolver := NewWritingTestResolvers(ResolverDependencies{Posts: postSpy, Models: models, Voices: voiceService, Templates: templateAuthoring, Guidelines: guidelines, GuidelineAuthoring: guidelineAuthoring, Candidates: authoringService, Styles: styleFactory})
	return &resolverHarness{resolver: resolver, db: handle, posts: posts, postSpy: postSpy, voices: voices, voiceService: voiceService, templates: templates, guidelines: guidelines, authoring: authoringService, authoringStore: authoringStore, templateAuthoring: templateAuthoring, guidelineAuthoring: guidelineAuthoring, at: at}
}
func (h *resolverHarness) seedSource(t *testing.T, user, slug string) {
	t.Helper()
	stamp := h.at.Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,voice_id,title,memo,target_language,target_length,tag_count,use_memory,field,input_revision,content_revision,created_at,updated_at) VALUES(?,?,?,'Private source title','PRIVATE SOURCE MEMO','ko',1500,4,0,'restaurant',4,7,?,?)", slug, user, "voice-"+user, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES(?,?,?, ?,640,480,1234,?)", "image-"+user, slug, user+".jpg", user+"/owned.jpg", stamp); err != nil {
		t.Fatal(err)
	}
}

type resolverPostGuard struct{}

func (resolverPostGuard) HasOrdinaryWrite(context.Context, *sql.Tx, string, string) (bool, error) {
	return false, nil
}
func TestResolversUseOnlyExplicitNeutralMaterialAndAuthorizedMediaAndFenceEffectiveSourceChoices(t *testing.T) {
	h := actualResolverHarness(t)
	h.seedSource(t, "alice", "source")
	h.seedSource(t, "bob", "foreign")
	ctx := context.Background()
	material := generation.WritingTestMaterialRequest{Material: "Explicit fictional scenario", Fictional: true, TargetLanguage: generation.LanguageEnglish, TagCount: 3, AttachmentIDs: []string{"image-alice"}}
	if _, err := h.db.Writer.Exec("INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('second-alice','source','second.jpg','alice/second.jpg',640,480,1234,?)", h.at.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	ordered := material
	ordered.AttachmentIDs = []string{"second-alice", "image-alice"}
	orderedSource, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "", 0, 0, ordered)
	if err != nil || len(orderedSource.Attachments) != 2 || orderedSource.Attachments[0].ID != "second-alice" || orderedSource.Attachments[1].ID != "image-alice" {
		t.Fatalf("explicit media order changed=%+v err=%v", orderedSource.Attachments, err)
	}
	neutral, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "", 0, 0, material)
	if err != nil || h.postSpy.sourceReads != 0 || neutral.Post.Memo != material.Material || neutral.Post.Title != "" || neutral.AssignmentsHash != "" || len(neutral.Attachments) != 1 || neutral.Post.Images[0].Key != "alice/owned.jpg" {
		t.Fatalf("neutral=%+v source reads=%d err=%v", neutral, h.postSpy.sourceReads, err)
	}
	for _, id := range []string{"image-bob", "missing"} {
		invalid := material
		invalid.AttachmentIDs = []string{id}
		if _, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "", 0, 0, invalid); !errors.Is(err, experiment.ErrTestNotFound) {
			t.Fatalf("foreign/unknown media=%s err=%v", id, err)
		}
	}
	length := 1500
	material.TargetLength = &length
	material.TargetLanguage = generation.LanguageKorean
	material.TagCount = 4
	material.VoiceID = "voice-alice"
	source, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "source", 4, 7, material)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := h.posts.GetPost(ctx, "source")
	if source.AssignmentsHash != post.TestAssignmentsHash(current) || source.Post.Memo != material.Material {
		t.Fatalf("matching source common=%+v", source)
	}
	override := material
	override.TagCount = 2
	override.VoiceID = ""
	different, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "source", 4, 7, override)
	if err != nil || different.AssignmentsHash == source.AssignmentsHash {
		t.Fatalf("overridden choices not fenced=%+v err=%v", different, err)
	}
	content := post.PostContent{Title: "Winner", Tags: []string{"one"}, Blocks: []post.Block{{Type: post.BlockText, Content: "A complete owner-selected output."}}}
	publisher := post.NewTestResultService(poststore.NewTestResultStore(h.db.Writer, h.db.Reader, resolverPostGuard{}))
	application := post.TestOutputPublication{UserID: "alice", TestID: "different", WinnerID: "winner", RequestKey: "apply", PostSlug: "source", InputRevision: 4, ContentRevision: 7, AssignmentsHash: different.AssignmentsHash, Content: content, Baseline: content, ContentLanguage: post.LanguageKorean}
	if _, err := publisher.ApplyTestResult(ctx, application); !errors.Is(err, post.ErrTestPublicationConflict) {
		t.Fatalf("incompatible common settings applied=%v", err)
	}
	application.TestID = "matching"
	application.RequestKey = "matching"
	application.AssignmentsHash = source.AssignmentsHash
	if _, err := publisher.ApplyTestResult(ctx, application); err != nil {
		t.Fatalf("matching choices refused=%v", err)
	}
	if _, err := h.resolver.ResolveWritingTestSource(ctx, "alice", "source", 4, 7, material); !errors.Is(err, experiment.ErrTestRevisionConflict) {
		t.Fatalf("later source content revision reused=%v", err)
	}
}
func TestResolversFreezeRealTemplateInputsNumbersAndOrderedOwnedGuidelineSlotWithoutRescoping(t *testing.T) {
	h := actualResolverHarness(t)
	ctx := context.Background()
	length, tags := 4200, 7
	saved, err := h.templates.Create(ctx, "alice", template.Authored{Name: "Saved skeleton", Body: `<ask label="visit" required="true">What did you experience?</ask><write>Use the named fact</write>`, Numbers: template.Numbers{TargetLength: &length, TagCount: &tags}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, version, _ := h.templateAuthoring.SeedWithNumbers(ctx, "alice", saved.ID)
	ref := generation.WritingTestReference{SourceKind: "setting", SettingKind: "template", SettingID: saved.ID, SettingRevision: version}
	frozen, err := h.resolver.ResolveWritingTestTemplate(ctx, "alice", ref, nil)
	if err != nil || len(frozen.Fields) != 1 || frozen.Fields[0].Label != "visit" || !frozen.Fields[0].Required {
		t.Fatalf("template=%+v err=%v", frozen, err)
	}
	payload, err := template.DecodeTestSnapshot(frozen.Payload)
	if err != nil || *payload.Numbers.TargetLength != length || *payload.Numbers.TagCount != tags {
		t.Fatalf("domain numbers changed=%+v err=%v", payload, err)
	}
	var required *generation.RequiredTemplateAnswerError
	if _, err := h.resolver.RenderWritingTestTemplate(ctx, "alice", frozen, false, nil); !errors.As(err, &required) || required.Label != "visit" {
		t.Fatalf("required fact skipped=%v", err)
	}
	brief, err := h.resolver.RenderWritingTestTemplate(ctx, "alice", frozen, false, []generation.TemplateAnswer{{Label: "visit", Text: "Explicit named owner fact", Enabled: true}})
	if err != nil || len(brief.Facts) != 1 || brief.Facts[0].Value != "Explicit named owner fact" {
		t.Fatalf("facts=%+v err=%v", brief, err)
	}
	if len(brief.BodyParts) != 2 || brief.BodyParts[0].Kind != "answer_write" || brief.BodyParts[0].Label != "visit" || brief.BodyParts[0].Parts[0].Text != "Explicit named owner fact" || brief.BodyParts[1].Kind != "write" || brief.TitleParts == nil {
		t.Fatalf("frozen template lost role or known-empty title: %+v", brief)
	}
	changedBody := `&lt;write&gt;later literal&lt;/write&gt;<write>Later source instruction</write>`
	if _, err := h.templates.Update(ctx, "alice", saved.ID, template.Patch{Body: &changedBody}); err != nil {
		t.Fatal(err)
	}
	answer := "</facts><write>answer stays data</write>\n\"quoted\""
	retained, err := h.resolver.RenderWritingTestTemplate(ctx, "alice", frozen, false, []generation.TemplateAnswer{{Label: "visit", Text: answer, Enabled: true}})
	if err != nil || retained.BodyParts[0].Parts[0].Text != answer || retained.BodyParts[1].Text != "Use the named fact" {
		t.Fatalf("frozen source changed or answer acquired an instruction role: %+v %v", retained, err)
	}
	global, _ := h.guidelines.Create(ctx, "alice", guideline.KindPost, "Global", "Global direction", guideline.ScopePatch{Scope: guideline.ScopeGlobal}, "")
	scoped, _ := h.guidelines.Create(ctx, "alice", guideline.KindPost, "Scoped", "Template direction", guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{saved.ID}}, "")
	field, _ := h.guidelines.Create(ctx, "alice", guideline.KindPost, "Field", "Field direction", guideline.ScopePatch{Scope: guideline.ScopeFields, Fields: []string{"restaurant"}}, "")
	rules, err := h.resolver.FreezeWritingTestGuidelines(ctx, "alice", saved.ID, "restaurant", generation.LanguageKorean, false)
	if err != nil || len(rules.Owner) != 3 || rules.Owner[0].ID != global.ID || rules.Owner[1].ID != scoped.ID || rules.Owner[2].ID != field.ID {
		t.Fatalf("ordered rules=%+v err=%v", rules, err)
	}
	if len(rules.Stock) != len(rules.Defaults) || len(rules.Stock) == 0 {
		t.Fatalf("stock identity lost at test resolver: %+v", rules)
	}
	for index, rule := range rules.Stock {
		if rule.Key == "" || rule.Text != rules.Defaults[index] || len(rule.Applicability) == 0 {
			t.Fatalf("stock responsibility lost at test resolver: %+v", rule)
		}
	}
	for _, rule := range rules.Owner {
		if rule.Revision == "" {
			t.Fatal("slot lost its owned revision")
		}
	}
	none, err := h.resolver.FreezeWritingTestGuidelines(ctx, "alice", "", "", generation.LanguageKorean, false)
	if err != nil || len(none.Owner) != 1 || none.Owner[0].ID != global.ID {
		t.Fatalf("source-neutral rules reached unrelated source=%+v err=%v", none, err)
	}
	other := ref
	other.SettingRevision = "stale"
	if _, err := h.resolver.ResolveWritingTestTemplate(ctx, "alice", other, nil); !errors.Is(err, experiment.ErrTestRevisionConflict) {
		t.Fatalf("stale structure substituted=%v", err)
	}
	if _, err := h.resolver.ResolveWritingTestTemplate(ctx, "bob", ref, nil); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("foreign structure=%v", err)
	}
}
func TestResolversFreezeAcceptedPersonalVoiceAndStableCurrentSyntheticCandidatesWithoutWriting(t *testing.T) {
	h := actualResolverHarness(t)
	ctx := context.Background()
	body := strings.Repeat("직접 겪은 조용한 하루를 적었어요. 골목을 걷다가 잠시 쉬었어요. ", 10)
	sample := voice.Sample{ID: "accepted-material", UserID: "alice", VoiceID: "voice-alice", Kind: voice.SampleKindPost, Body: body, Chars: utf8.RuneCountInString(body), ContentRevision: 1, CreatedAt: h.at}
	if err := h.voices.InsertSample(ctx, sample); err != nil {
		t.Fatal(err)
	}
	source := voice.AcceptedSource{SampleID: sample.ID, ContentRevision: 1}
	analysis := voice.Analysis{Origin: voice.OriginPersonal, SourceVersionsKnown: true, MaterialIDs: []string{sample.ID}, AcceptedSources: []voice.AcceptedSource{source}, AcceptedMaterials: []voice.AcceptedMaterial{{Source: source, Body: body, Kind: sample.Kind, CreatedAt: h.at}}, Counted: voice.FingerprintOf([]voice.Material{{Text: body, Kind: sample.Kind}}), AI: voice.AIPart{Impression: "Accepted calm voice"}, CreatedAt: h.at}
	if err := h.voices.PublishAnalysis(ctx, "alice", "voice-alice", analysis); err != nil {
		t.Fatal(err)
	}
	accepted, _ := h.voices.CurrentAnalysis(ctx, "alice", "voice-alice")
	revision := voice.AcceptedAnalysisRevision(*accepted)
	if _, err := h.db.Writer.Exec("UPDATE voice_samples SET body='NEW UNACCEPTED PRIVATE WORDS',content_revision=2 WHERE id=?", sample.ID); err != nil {
		t.Fatal(err)
	}
	ref := generation.WritingTestReference{SourceKind: "setting", SettingKind: "voice", SettingID: "voice-alice", SettingRevision: revision}
	profile, err := h.resolver.ResolveWritingTestProfile(ctx, "alice", ref, nil, generation.LanguageKorean, "")
	if err != nil || profile.Revision != revision || profile.Synthetic || len(profile.Profile.Excerpts) != 1 || strings.Contains(profile.Profile.Excerpts[0], "UNACCEPTED") {
		t.Fatalf("accepted personal profile=%+v err=%v", profile, err)
	}
	if _, err := h.resolver.ResolveWritingTestProfile(ctx, "bob", ref, nil, generation.LanguageKorean, ""); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign accepted profile=%v", err)
	}
	if _, err := h.db.Writer.Exec("DELETE FROM voice_samples WHERE id=?", sample.ID); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := h.resolver.ResolveWritingTestProfile(ctx, "alice", ref, nil, generation.LanguageKorean, "")
	if err != nil || len(withdrawn.Profile.Excerpts) != 0 || len(withdrawn.Profile.Sources) != 0 || strings.Contains(withdrawn.Profile.Text, body) {
		t.Fatalf("withdrawn material reached future projection=%+v err=%v", withdrawn.Profile, err)
	}
	artifact := authoring.Artifact{ID: "direct-style", Revision: 2, Name: "차분한 말투", Description: "짧고 담백하게 작은 경험을 설명해요.", Body: strings.Repeat("가상의 작은 가게에서 차를 마셨어요. 창가에서 쉬니 마음이 편안했어요. 조용한 시간을 천천히 즐겼어요. ", 5)}
	session := authoring.Session{ID: "prepared", UserID: "alice", Kind: authoring.WritingVoice, Revision: 2, Phase: "draft", WorkingSource: &artifact, DraftState: authoring.DraftValid, HasUnpublishedChanges: true, CreatedAt: h.at, UpdatedAt: h.at}
	if _, err := h.authoringStore.Create(ctx, session, "create-prepared"); err != nil {
		t.Fatal(err)
	}
	candidateRef := generation.WritingTestReference{SourceKind: "authoring_candidate", AuthoringSessionID: session.ID, AuthoringCandidateID: artifact.ID, AuthoringRevision: artifact.Revision}
	prepared, err := h.resolver.ResolveWritingTestCandidate(ctx, "alice", candidateRef)
	if err != nil {
		t.Fatal(err)
	}
	first, err := h.resolver.ResolveWritingTestProfile(ctx, "alice", candidateRef, &prepared, generation.LanguageKorean, "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := h.resolver.ResolveWritingTestProfile(ctx, "alice", candidateRef, &prepared, generation.LanguageKorean, "")
	if err != nil || !reflect.DeepEqual(first, again) || !first.Synthetic || first.Voice.ID != "" || len(first.Profile.Sources) != 0 || !strings.Contains(first.Profile.Text, "가상") {
		t.Fatalf("synthetic snapshot drift=%+v/%+v err=%v", first, again, err)
	}
	stored, _ := h.authoringStore.Get(ctx, "alice", session.ID)
	if stored.Revision != session.Revision || stored.Saved != nil {
		t.Fatal("test contender saved or modified source")
	}
}
