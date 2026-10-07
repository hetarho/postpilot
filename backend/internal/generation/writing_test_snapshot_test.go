package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/template"
)

type plannerPorts struct {
	source                                                                 WritingTestSource
	models                                                                 map[llm.ModelRef]llm.ModelInfo
	voices                                                                 map[string]WritingTestVoice
	templates                                                              map[string]template.TestSnapshot
	rules                                                                  WritingTestRules
	variants                                                               map[string]WritingTestRule
	candidates                                                             map[string]WritingTestPreparedSetting
	sourceReads, voiceReads, templateReads, candidateReads, guidelineReads int
	scopes                                                                 []string
	modelChecks                                                            []string
	blocked                                                                map[llm.ModelRef]error
	templateAuthoring                                                      *template.Authoring
}

func (p *plannerPorts) ResolveWritingTestSource(_ context.Context, user, slug string, input, content int64, material WritingTestMaterialRequest) (WritingTestSource, error) {
	p.sourceReads++
	if user != "alice" {
		return WritingTestSource{}, ErrWritingTestMaterial
	}
	if utf8.RuneCountInString(material.Material) > 4000 || material.TagCount < 1 || material.TagCount > 10 || material.TargetLength != nil && (*material.TargetLength < 100 || *material.TargetLength > 10000) {
		return WritingTestSource{}, ErrWritingTestMaterial
	}
	value := p.source
	if slug == "" {
		value.Post.Slug = ""
		value.Post.Memo = material.Material
		value.InputRevision, value.ContentRevision = 0, 0
		value.Revision = "approved-neutral:v1"
		value.AssignmentsHash = ""
	}
	return value, nil
}
func (p *plannerPorts) PrepareWritingTestModel(_ context.Context, user, stage string, ref llm.ModelRef) (WritingTestModel, error) {
	p.modelChecks = append(p.modelChecks, user+":"+stage+":"+ref.String())
	if err := p.blocked[ref]; err != nil {
		return WritingTestModel{}, err
	}
	value, ok := p.models[ref]
	if user != "alice" || !ok {
		return WritingTestModel{}, ErrWritingTestReference
	}
	return WritingTestModel{Info: value, PromptTokens: 30000}, nil
}
func (p *plannerPorts) ResolveWritingTestProfile(_ context.Context, user string, ref WritingTestReference, prepared *WritingTestPreparedSetting, target Language, topic string) (WritingTestVoice, error) {
	p.voiceReads++
	if user != "alice" {
		return WritingTestVoice{}, ErrWritingTestReference
	}
	if prepared != nil {
		return WritingTestVoice{Voice: VoiceRef{Name: prepared.Name, Made: true}, Profile: Profile{Text: "profile:" + prepared.Body}, Revision: prepared.Revision, Synthetic: true}, nil
	}
	value, ok := p.voices[ref.SettingID]
	if !ok {
		return WritingTestVoice{}, ErrWritingTestReference
	}
	return value, nil
}
func (p *plannerPorts) ResolveWritingTestCandidate(_ context.Context, user string, ref WritingTestReference) (WritingTestPreparedSetting, error) {
	p.candidateReads++
	value, ok := p.candidates[ref.AuthoringCandidateID]
	if user != "alice" || !ok || value.Revision != fmt.Sprint(ref.AuthoringRevision) {
		return WritingTestPreparedSetting{}, ErrWritingTestReference
	}
	return value, nil
}
func (p *plannerPorts) ResolveWritingTestTemplate(_ context.Context, user string, ref WritingTestReference, prepared *WritingTestPreparedSetting) (WritingTestTemplate, error) {
	p.templateReads++
	value, ok := p.templates[ref.SettingID]
	if prepared != nil {
		value = template.TestSnapshot{TargetVersion: prepared.Revision, Draft: template.Draft{Name: prepared.Name, Description: prepared.Description, Body: prepared.Body, TitleArea: prepared.TitleArea}}
		ok = true
	}
	if user != "alice" || !ok {
		return WritingTestTemplate{}, ErrWritingTestReference
	}
	if _, err := p.templateAuthoring.Validate(value.Draft); err != nil {
		return WritingTestTemplate{}, err
	}
	if err := p.templateAuthoring.ValidateNumbers(value.Numbers); err != nil {
		return WritingTestTemplate{}, err
	}
	title, body, err := template.ParseTemplate(value.Draft.TitleArea, value.Draft.Body, template.ParseOptions{PhotoRowMax: 3, AskMaxPerBody: 10})
	if err != nil {
		return WritingTestTemplate{}, err
	}
	fields := []WritingTestTemplateField{}
	for _, nodes := range [][]template.Node{title, body} {
		for _, ask := range template.Asks(nodes) {
			flavor := "verbatim"
			if ask.Text != "" {
				flavor = "write"
			}
			fields = append(fields, WritingTestTemplateField{Label: template.Decode(ask.Label), Flavor: flavor, Required: ask.Required})
		}
	}
	raw, _ := template.EncodeTestSnapshot(value)
	return WritingTestTemplate{ID: ref.SettingID, Name: value.Draft.Name, Revision: value.TargetVersion, Fields: fields, Payload: raw}, nil
}
func (p *plannerPorts) RenderWritingTestTemplate(_ context.Context, user string, value WritingTestTemplate, photos bool, answers []TemplateAnswer) (TemplateBrief, error) {
	frozen, err := template.DecodeTestSnapshot(value.Payload)
	if err != nil {
		return TemplateBrief{}, err
	}
	domainAnswers := make([]template.Answer, len(answers))
	for index, value := range answers {
		domainAnswers[index] = template.Answer{Label: value.Label, Text: value.Text, Enabled: value.Enabled}
	}
	rendered, err := p.templateAuthoring.RenderedForNewWrite(frozen.Draft, photos, domainAnswers)
	if err != nil {
		var missing *template.RequiredAnswerError
		if errors.As(err, &missing) {
			return TemplateBrief{}, &RequiredTemplateAnswerError{Label: missing.Label}
		}
		return TemplateBrief{}, err
	}
	brief := TemplateBrief{Name: rendered.Name, Body: rendered.Body, TitleArea: rendered.TitleArea}
	for _, fact := range rendered.Facts {
		brief.Facts = append(brief.Facts, TemplateFact{Label: fact.Label, Value: fact.Value})
	}
	return brief, nil
}
func (p *plannerPorts) FreezeWritingTestGuidelines(_ context.Context, user, templateID, field string, target Language, memory bool) (WritingTestRules, error) {
	p.guidelineReads++
	p.scopes = append(p.scopes, templateID+":"+field)
	if user != "alice" {
		return WritingTestRules{}, ErrWritingTestReference
	}
	return p.rules, nil
}
func (p *plannerPorts) ResolveWritingTestGuideline(_ context.Context, user string, ref WritingTestReference, prepared *WritingTestPreparedSetting) (WritingTestRule, error) {
	if user != "alice" {
		return WritingTestRule{}, ErrWritingTestReference
	}
	if prepared != nil {
		return WritingTestRule{Name: prepared.Name, Text: prepared.Body, Revision: prepared.Revision}, nil
	}
	value, ok := p.variants[ref.SettingID]
	if !ok {
		return WritingTestRule{}, ErrWritingTestReference
	}
	return value, nil
}

type plannerUnusedTemplateStore struct{}

func (plannerUnusedTemplateStore) CountAuthoringTargets(context.Context, string) (int, error) {
	panic("unexpected canonical read")
}
func (plannerUnusedTemplateStore) PublishAuthoring(context.Context, string, template.AuthoringPublication, string, time.Time, int) (template.Template, error) {
	panic("unexpected canonical save")
}

type plannerMemory struct{ calls int }

func (m *plannerMemory) ForPost(context.Context, string, []string) ([]string, error) {
	m.calls++
	return []string{"approved common memory"}, nil
}

type plannerQuality struct{ calls int }

func (q *plannerQuality) RulesFor(context.Context, string, string, []string, Language) ([]string, error) {
	q.calls++
	return []string{"approved common quality rule"}, nil
}

func plannerFixture(t *testing.T) (*WritingTestFactory, *plannerPorts, *fakePosts, *fakeModels, *plannerMemory, *plannerQuality) {
	t.Helper()
	ports := &plannerPorts{models: map[llm.ModelRef]llm.ModelInfo{}, voices: map[string]WritingTestVoice{}, templates: map[string]template.TestSnapshot{}, variants: map[string]WritingTestRule{}, candidates: map[string]WritingTestPreparedSetting{}, blocked: map[llm.ModelRef]error{}}
	ports.source = WritingTestSource{Post: PostInput{UserID: "alice", Slug: "source", Title: "Shared title", Memo: "Shared owner memo", Field: "food", Content: &PostContent{Title: "OLD CANONICAL CONTENT"}, Storyline: &Storyline{Paragraphs: []StorylineParagraph{{Text: "OLD STORYLINE"}}}}, InputRevision: 7, ContentRevision: 9, Revision: "source:v7", AssignmentsHash: "assignments:v1"}
	ports.rules = WritingTestRules{Defaults: []string{"stock first", "stock second"}, Owner: []WritingTestRule{{ID: "prefix", Revision: "r1", Text: "fixed first rule"}, {ID: "slot", Revision: "r1", Text: "original designated rule"}, {ID: "suffix", Revision: "r1", Text: "fixed last rule"}}}
	ports.voices["base-voice"] = WritingTestVoice{Voice: VoiceRef{ID: "base-voice", Name: "Accepted base voice", Made: true}, Profile: Profile{Text: "accepted base profile", Excerpts: []string{"accepted example"}}, Revision: "r1"}
	ports.templates["base-template"] = template.TestSnapshot{TargetID: "base-template", TargetVersion: "r1", Draft: template.Draft{Name: "Common template", Body: "<write>Common structure</write>"}}
	ports.templateAuthoring = template.NewAuthoring(template.NewService(nil, template.NewLimits(template.Ceilings{NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200, MaxPerAccount: 50, PhotoRowMax: 3, AskLabelMaxChars: 40, AskMaxPerBody: 10}, template.NumberBounds{TargetLengthMin: 100, TargetLengthMax: 10000, TagCountMin: 1, TagCountMax: 10})), plannerUnusedTemplateStore{})
	models := newFakeModels()
	posts := &fakePosts{input: ports.source.Post}
	memory, quality := &plannerMemory{}, &plannerQuality{}
	deps := testDeps()
	deps.Memories = memory
	deps.QualityRules = quality
	service := NewService(posts, fakeProfiles{}, models, fakeImages{}, nil, 4, DefaultReasoningPolicy(), fakeBudget{observe: 2048, floor: 8192, perChar: 4, ceiling: 32768}, deps)
	factory := NewWritingTestFactory(service, WritingTestFactoryDeps{Sources: ports, Models: ports, Profiles: ports, Templates: ports, Guidelines: ports, Candidates: ports})
	for _, id := range []string{"fixed-write", "fixed-observe"} {
		ref := llm.ModelRef{ProviderID: "p", ModelID: id}
		ports.models[ref] = llm.ModelInfo{Ref: ref, Stages: []string{"observe", "write"}, Vision: true, VideoInput: true, VideoDelivery: llm.VideoDelivery{SignedVideoURL: true}, StructuredOutput: true, ContextTokens: 131072}
	}
	return factory, ports, posts, models, memory, quality
}
func plannerRequest(t *testing.T, p *plannerPorts, factor, stage string, count int) (WritingTestSnapshotRequest, WritingTestMaterialRequest) {
	t.Helper()
	length := 2200
	material := WritingTestMaterialRequest{ObserveModel: llm.ModelRef{ProviderID: "p", ModelID: "fixed-observe"}, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "fixed-write"}, VoiceID: "base-voice", TemplateID: "base-template", GuidelineSlotID: "slot", TargetLanguage: LanguageKorean, TargetLength: &length, TagCount: 3, UseMemory: true, QualityRules: []string{"length"}}
	raw, _ := EncodeWritingTestMaterialRequest(material)
	request := WritingTestSnapshotRequest{UserID: "alice", SourcePostSlug: "source", InputRevision: 7, ContentRevision: 9, Factor: factor, ModelStage: stage, Count: count, CommonMaterial: raw, Variants: make([][]byte, count)}
	for index := range count {
		id := fmt.Sprintf("entrant-%d", index)
		ref := WritingTestReference{SourceKind: "setting", SettingKind: factor, SettingID: id, SettingRevision: "r1"}
		switch factor {
		case "model":
			ref = WritingTestReference{SourceKind: "model", Model: llm.ModelRef{ProviderID: "p", ModelID: id}}
			p.models[ref.Model] = llm.ModelInfo{Ref: ref.Model, Stages: []string{"observe", "write"}, Vision: true, VideoInput: true, VideoDelivery: llm.VideoDelivery{SignedVideoURL: true}, StructuredOutput: true, ContextTokens: 131072}
		case "voice":
			p.voices[id] = WritingTestVoice{Voice: VoiceRef{ID: id, Name: id, Made: true}, Profile: Profile{Text: "accepted profile:" + id}, Revision: "r1"}
		case "template":
			number := 1000 + index*100
			p.templates[id] = template.TestSnapshot{TargetID: id, TargetVersion: "r1", Draft: template.Draft{Name: id, Body: "<write>" + id + " structure</write>"}, Numbers: template.Numbers{TargetLength: &number}}
		case "guideline":
			p.variants[id] = WritingTestRule{ID: id, Name: id, Revision: "r1", Text: "variant direction:" + id}
		}
		request.Variants[index], _ = EncodeWritingTestReference(ref)
	}
	return request, material
}
func plannerAttachments(p *plannerPorts, photos int, video bool) {
	images, observations := storedSnapshot(photos, "old/observer")
	if video {
		images = append(images, Image{Filename: "clip.mp4", Key: "owned/clip", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 30000})
	}
	p.source.Post.Images, p.source.Post.Observations = images, observations
	for index, image := range images {
		p.source.Attachments = append(p.source.Attachments, WritingTestAttachment{ID: fmt.Sprint(index), Fingerprint: "bytes:" + image.Key, Image: image})
	}
}

func TestWritingTestFrozenPlansAndNonfactorInputsAcrossEveryAxisAndCount(t *testing.T) {
	for _, axis := range []struct{ factor, stage string }{{"model", "write"}, {"model", "observe"}, {"voice", ""}, {"template", ""}, {"guideline", ""}} {
		for _, count := range []int{2, 4, 8, 16} {
			t.Run(fmt.Sprintf("%s-%s-%d", axis.factor, axis.stage, count), func(t *testing.T) {
				factory, ports, posts, models, memory, quality := plannerFixture(t)
				plannerAttachments(ports, 5, true)
				request, material := plannerRequest(t, ports, axis.factor, axis.stage, count)
				snapshot, err := factory.FreezeWritingTest(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				common, variants, err := decodeWritingTestSnapshot(snapshot)
				if err != nil || len(variants) != count {
					t.Fatalf("decode=%d %v", len(variants), err)
				}
				calls, err := factory.PlanWritingTest(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				observeCalls, writeCalls := 0, 0
				for _, call := range calls {
					if call.PromptTokens <= 0 || call.CompletionTokens <= 0 {
						t.Fatal("unbounded plan", call)
					}
					if call.Stage == "observe" {
						observeCalls += call.Count
					} else {
						writeCalls += call.Count
					}
				}
				wantObserve := 3
				if axis.stage == "observe" {
					wantObserve *= count
				}
				if observeCalls != wantObserve || writeCalls != count || snapshot.ObserveCalls != wantObserve {
					t.Fatalf("calls=%+v", calls)
				}
				if common.Post.Content != nil || common.Post.ContentLanguage != nil || common.Post.Storyline != nil || len(common.Post.FollowStoryline) != 0 {
					t.Fatal("test reused canonical output/storyline")
				}
				if common.Post.TagCount != material.TagCount || *common.Post.TargetLength != *material.TargetLength {
					t.Fatal("template defaults changed test options")
				}
				var first []byte
				for _, variant := range variants {
					input := variant.Snapshot
					switch axis.factor {
					case "model":
						if axis.stage == "observe" {
							input.ObserveModel = "common"
						}
					case "voice":
						input.Profile = Profile{NoVoice: true}
						input.Post.Voice = VoiceRef{}
					case "template":
						input.Post.Template = nil
					case "guideline":
						input.Post.Guidelines = copyTexts(input.Post.Guidelines)
						input.Post.Guidelines[1] = "one varied slot"
					}
					bytes, err := encodeWriteSnapshot(input)
					if err != nil {
						t.Fatal(err)
					}
					if first == nil {
						first = bytes
					} else if string(first) != string(bytes) {
						t.Fatal("a nonfactor input changed")
					}
					if !reflect.DeepEqual(input.Post.DefaultGuidelines, common.Post.DefaultGuidelines) || *input.Post.TargetLength != 2200 || input.Post.TagCount != 3 {
						t.Fatal("stock rules/default numbers changed")
					}
				}
				if ports.sourceReads != 1 || ports.guidelineReads != 1 || !reflect.DeepEqual(ports.scopes, []string{"base-template:food"}) || memory.calls != 1 || quality.calls != 1 {
					t.Fatal("common scope/material was resolved per entrant")
				}
				if len(models.calls) != 0 || posts.reads != 0 || len(posts.contents) != 0 || len(posts.observationWrites) != 0 {
					t.Fatal("planning called AI or mutated/read a canonical source through ordinary generation")
				}
			})
		}
	}
}

func TestWritingTestTemplateRequiredFieldUnionUsesOneExplicitFactMap(t *testing.T) {
	factory, ports, _, _, _, _ := plannerFixture(t)
	request, material := plannerRequest(t, ports, "template", "", 2)
	first := ports.templates["entrant-0"]
	first.Draft.TitleArea = `<ask label="name" required="true"/>`
	ports.templates["entrant-0"] = first
	second := ports.templates["entrant-1"]
	second.Draft.Body = `<ask label="name" required="true">Describe the name</ask><ask label="detail" required="true"/>`
	ports.templates["entrant-1"] = second
	material.TemplateAnswers = []TemplateAnswer{{Label: "name", Text: "Explicit shared name", Enabled: true}}
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	_, err := factory.FreezeWritingTest(context.Background(), request)
	var missing *RequiredTemplateAnswerError
	if !errors.As(err, &missing) || missing.Label != "detail" {
		t.Fatalf("missing union input=%v", err)
	}
	material.TemplateAnswers = append(material.TemplateAnswers, TemplateAnswer{Label: "detail", Text: "Explicit shared detail", Enabled: true})
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	common, variants, _ := decodeWritingTestSnapshot(snapshot)
	if !reflect.DeepEqual(common.RequiredFields, []WritingTestTemplateField{{Label: "name", Flavor: "verbatim", Required: true}, {Label: "name", Flavor: "write", Required: true}, {Label: "detail", Flavor: "verbatim", Required: true}}) {
		t.Fatalf("field union=%+v", common.RequiredFields)
	}
	for _, variant := range variants {
		if !strings.Contains(variant.Snapshot.Post.Template.TitleArea+variant.Snapshot.Post.Template.Body, "Explicit shared name") {
			t.Fatal("fact was not identically supplied")
		}
	}
	material.TemplateAnswers[1].Enabled = false
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.As(err, &missing) {
		t.Fatal("disabled required fact accepted", err)
	}
}

func TestWritingTestSourceOwnershipVersionBoundsAndRawPayloadBypassAreRefused(t *testing.T) {
	for _, change := range []func(*WritingTestSnapshotRequest, *WritingTestMaterialRequest, *plannerPorts){
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) { r.UserID = "bob" },
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) { r.InputRevision++ },
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) {
			r.ContentRevision++
		},
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) { m.TagCount = 11 },
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) {
			n := 10001
			m.TargetLength = &n
		},
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) {
			m.TargetLanguage = "ja"
		},
		func(r *WritingTestSnapshotRequest, m *WritingTestMaterialRequest, p *plannerPorts) {
			r.SourcePostSlug = ""
			m.Material = strings.Repeat("가", 4001)
		},
	} {
		factory, ports, _, models, _, _ := plannerFixture(t)
		request, material := plannerRequest(t, ports, "model", "write", 2)
		change(&request, &material, ports)
		request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
		if _, err := factory.FreezeWritingTest(context.Background(), request); err == nil {
			t.Fatal("invalid source input accepted")
		}
		if len(models.calls) != 0 {
			t.Fatal("invalid admission called provider")
		}
	}
	factory, ports, _, _, _, _ := plannerFixture(t)
	request, _ := plannerRequest(t, ports, "model", "write", 2)
	request.CommonMaterial = []byte(`{"Profile":{"Text":"raw private bypass"}}`)
	if _, err := factory.FreezeWritingTest(context.Background(), request); err == nil || ports.sourceReads != 0 {
		t.Fatal("raw profile bypass was read")
	}
	request, _ = plannerRequest(t, ports, "template", "", 2)
	request.Variants[0] = []byte(`{"Body":"raw XML bypass"}`)
	if _, err := factory.FreezeWritingTest(context.Background(), request); err == nil {
		t.Fatal("raw template contender accepted")
	}
}

func TestWritingTestExplicitNeutralScenarioMemoryAndReuseMakeNoHiddenCalls(t *testing.T) {
	factory, ports, posts, models, memory, quality := plannerFixture(t)
	request, material := plannerRequest(t, ports, "model", "write", 2)
	request.SourcePostSlug = ""
	request.InputRevision, request.ContentRevision = 0, 0
	material.Material = "A fictional walk and cup of tea"
	material.Fictional = true
	material.VoiceID, material.TemplateID = "", ""
	material.UseMemory = false
	material.QualityRules = nil
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	common, _, _ := decodeWritingTestSnapshot(snapshot)
	if !common.Fictional || !strings.Contains(common.Post.Memo, "사용자의 실제 경험이 아님") || common.Post.Slug != "" || common.Post.TargetLength == nil || !common.Profile.NoVoice {
		t.Fatal("neutral scenario impersonated a canonical post/experience")
	}
	if posts.reads != 0 || memory.calls != 0 || quality.calls != 0 || ports.voiceReads != 0 || ports.templateReads != 0 || len(models.calls) != 0 {
		t.Fatal("neutral scenario read unrelated material")
	}
	plannerAttachments(ports, 5, false)
	request, material = plannerRequest(t, ports, "model", "write", 2)
	empty := []string{}
	material.ObserveFiles = &empty
	material.ObserveModel = llm.ModelRef{ProviderID: "unavailable", ModelID: "old"}
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	snapshot, err = factory.FreezeWritingTest(context.Background(), request)
	if err != nil || snapshot.ObserveCalls != 0 {
		t.Fatal("reuse-all needed an unavailable observer", err)
	}
	plan, _ := factory.PlanWritingTest(snapshot)
	for _, call := range plan {
		if call.Stage == "observe" {
			t.Fatal("reuse-all invented observation calls")
		}
	}
}

func TestWritingTestFrozenInputsAndHashesRetainAllSourceVersions(t *testing.T) {
	factory, ports, _, _, _, _ := plannerFixture(t)
	request, _ := plannerRequest(t, ports, "voice", "", 2)
	first, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	common, variants, _ := decodeWritingTestSnapshot(first)
	ports.source.Post.Memo = "Later owner memo"
	ports.source.ContentRevision++
	ports.source.AssignmentsHash = "later assignments"
	voice := ports.voices["entrant-0"]
	voice.Profile.Text = "Later accepted profile"
	voice.Revision = "r2"
	ports.voices["entrant-0"] = voice
	again, decoded, _ := decodeWritingTestSnapshot(first)
	if again.Post.Memo != common.Post.Memo || decoded[0].Snapshot.Profile.Text != variants[0].Snapshot.Profile.Text || again.AssignmentsHash != "assignments:v1" || again.InputRevision != 7 || again.ContentRevision != 9 {
		t.Fatal("live mutations changed frozen private input")
	}
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.Is(err, ErrWritingTestRevision) {
		t.Fatal("changed source revision accepted", err)
	}
	request.ContentRevision = ports.source.ContentRevision
	ref := WritingTestReference{SourceKind: "setting", SettingKind: "voice", SettingID: "entrant-0", SettingRevision: "r2"}
	request.Variants[0], _ = EncodeWritingTestReference(ref)
	second, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == second.Hash {
		t.Fatal("source/accepted-profile revision missing from hash")
	}
	var raw map[string]any
	_ = json.Unmarshal(first.Common, &raw)
	raw["source_revision"] = "tampered"
	tampered := first
	tampered.Common, _ = json.Marshal(raw)
	if _, _, err := decodeWritingTestSnapshot(tampered); err == nil {
		t.Fatal("snapshot hash did not fence tampering")
	}
}

func TestWritingTestEveryModelNeedsOwnerRightsAndActualVideoDelivery(t *testing.T) {
	factory, ports, _, models, _, _ := plannerFixture(t)
	plannerAttachments(ports, 0, true)
	request, _ := plannerRequest(t, ports, "model", "observe", 2)
	ref := llm.ModelRef{ProviderID: "p", ModelID: "entrant-1"}
	info := ports.models[ref]
	info.VideoDelivery.SignedVideoURL = false
	ports.models[ref] = info
	var unsupported *VideoUnsupportedError
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.As(err, &unsupported) {
		t.Fatal("modality alone authorized video URL", err)
	}
	info.VideoDelivery.SignedVideoURL = true
	ports.models[ref] = info
	locked := errors.New("owner plan cannot execute this model")
	ports.blocked[ref] = locked
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.Is(err, locked) {
		t.Fatal("catalog resolution bypassed owner rights", err)
	}
	if len(models.calls) != 0 {
		t.Fatal("ineligible video comparison called a provider")
	}
}

func TestWritingTestPreparedCandidatesResolvePrivatelyAndStaySynthetic(t *testing.T) {
	for _, factor := range []string{"voice", "template", "guideline"} {
		t.Run(factor, func(t *testing.T) {
			factory, ports, posts, models, _, _ := plannerFixture(t)
			request, _ := plannerRequest(t, ports, factor, "", 2)
			prepared := WritingTestPreparedSetting{Kind: factor, Revision: "3", Name: "Prepared private setting", Body: "Prepared direction", Synthetic: true}
			if factor == "template" {
				prepared.Body = "<write>Prepared candidate structure</write>"
			}
			ports.candidates["private-candidate"] = prepared
			ref := WritingTestReference{SourceKind: "authoring_candidate", AuthoringSessionID: "private-session", AuthoringCandidateID: "private-candidate", AuthoringRevision: 3}
			request.Variants[1], _ = EncodeWritingTestReference(ref)
			snapshot, err := factory.FreezeWritingTest(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			_, variants, _ := decodeWritingTestSnapshot(snapshot)
			if ports.candidateReads != 1 || !variants[1].Synthetic || variants[1].Reference != ref {
				t.Fatal("prepared contender became an owned/personal setting")
			}
			if factor == "voice" && (variants[1].Snapshot.Post.Voice.ID != "" || len(variants[1].Snapshot.Profile.Sources) != 0) {
				t.Fatal("synthetic style copied personal evidence")
			}
			if len(models.calls) != 0 || len(posts.contents) != 0 || len(posts.observationWrites) != 0 {
				t.Fatal("candidate resolution generated or published")
			}
			ref.AuthoringRevision = 2
			request.Variants[1], _ = EncodeWritingTestReference(ref)
			if _, err := factory.FreezeWritingTest(context.Background(), request); err == nil {
				t.Fatal("stale candidate revision accepted")
			}
		})
	}
}

func TestWritingTestSemanticDuplicatesRefuseEvenWhenOwnedIdentitiesDiffer(t *testing.T) {
	for _, factor := range []string{"model", "voice", "template", "guideline"} {
		t.Run(factor, func(t *testing.T) {
			factory, ports, _, models, _, _ := plannerFixture(t)
			stage := ""
			if factor == "model" {
				stage = "write"
			}
			request, _ := plannerRequest(t, ports, factor, stage, 2)
			switch factor {
			case "model":
				request.Variants[1] = request.Variants[0]
			case "voice":
				value := ports.voices["entrant-1"]
				value.Profile = ports.voices["entrant-0"].Profile
				ports.voices["entrant-1"] = value
			case "template":
				value := ports.templates["entrant-1"]
				value.Draft.Body = ports.templates["entrant-0"].Draft.Body
				ports.templates["entrant-1"] = value
			case "guideline":
				value := ports.variants["entrant-1"]
				value.Text = ports.variants["entrant-0"].Text
				ports.variants["entrant-1"] = value
			}
			if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.Is(err, ErrWritingTestDuplicate) {
				t.Fatal("same semantic contender admitted", err)
			}
			if len(models.calls) != 0 {
				t.Fatal("duplicates were checked after paid work")
			}
		})
	}
}

func TestWritingTestSourceRevisionAndAssignmentHashesChangeWithoutChangingPromptMaterial(t *testing.T) {
	for _, metadata := range []string{"input", "content", "assignments"} {
		t.Run(metadata, func(t *testing.T) {
			factory, ports, _, _, _, _ := plannerFixture(t)
			request, _ := plannerRequest(t, ports, "model", "write", 2)
			original, err := factory.FreezeWritingTest(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			switch metadata {
			case "input":
				ports.source.InputRevision++
				request.InputRevision = ports.source.InputRevision
			case "content":
				ports.source.ContentRevision++
				request.ContentRevision = ports.source.ContentRevision
			case "assignments":
				ports.source.AssignmentsHash = "new assignments hash"
			}
			changed, err := factory.FreezeWritingTest(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			_, before, _ := decodeWritingTestSnapshot(original)
			_, after, _ := decodeWritingTestSnapshot(changed)
			oldBytes, _ := encodeWriteSnapshot(before[0].Snapshot)
			newBytes, _ := encodeWriteSnapshot(after[0].Snapshot)
			if string(oldBytes) != string(newBytes) || original.Hash == changed.Hash {
				t.Fatal("source metadata identity was conflated with writer material")
			}
		})
	}
}

func TestWritingTestUnusedFactAnswersDoNotAuthorizeMaterialFreeGeneration(t *testing.T) {
	factory, ports, _, models, _, _ := plannerFixture(t)
	request, material := plannerRequest(t, ports, "model", "write", 2)
	request.SourcePostSlug = ""
	request.InputRevision, request.ContentRevision = 0, 0
	material.VoiceID, material.TemplateID, material.Material = "", "", ""
	material.TemplateAnswers = []TemplateAnswer{{Label: "unused answer", Text: "Not consumed by a selected template", Enabled: true}}
	material.Fictional = true
	request.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.Is(err, ErrWritingTestMaterial) {
		t.Fatal("unconsumed facts/fictional marker authorized an empty test", err)
	}
	if len(models.calls) != 0 {
		t.Fatal("missing material refusal spent credits")
	}
}
