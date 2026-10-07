package voice

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrAuthoringConflict = errors.New("writing style publication changed")

type WritingStyleDraft struct{ Name, Description, Sample string }
type AuthoringKey struct {
	Key, SessionID string
	Revision       uint32
}
type AuthoringPublication struct {
	Key         AuthoringKey
	SourceID    string
	Draft       WritingStyleDraft
	MakeDefault bool
	WriteModel  string
}
type AuthoringStore interface {
	PublishAuthoring(context.Context, string, AuthoringPublication, Voice, Analysis) (Voice, error)
}
type Authoring struct {
	service *Service
	store   AuthoringStore
}

func NewAuthoring(service *Service, store AuthoringStore) *Authoring {
	if service == nil || store == nil {
		panic("voice: authoring dependencies required")
	}
	return &Authoring{service: service, store: store}
}
func NormalizeWritingStyleDraft(value WritingStyleDraft) (WritingStyleDraft, error) {
	value.Name, value.Description, value.Sample = strings.TrimSpace(value.Name), strings.TrimSpace(value.Description), strings.TrimSpace(value.Sample)
	if !validKoreanCandidateText(value.Name, 1, VoiceNameMaxChars) || !validKoreanCandidateText(value.Description, 1, CandidateDescriptionMaxChars) || !validKoreanCandidateText(value.Sample, CandidateSampleMinChars, CandidateSampleMaxChars) || !containsKoreanProse(ProseSentences(value.Sample)) {
		return WritingStyleDraft{}, &CandidateOutputError{Cause: errors.New("writing style fields must be bounded Korean prose")}
	}
	return value, nil
}
func BuildSyntheticAnalysis(value WritingStyleDraft, model string, at time.Time) (Analysis, error) {
	value, err := NormalizeWritingStyleDraft(value)
	if err != nil {
		return Analysis{}, err
	}
	return Analysis{Origin: OriginSynthetic, SyntheticSample: value.Sample, Counted: FingerprintOf([]Material{{Text: value.Sample, Kind: SampleKindPost}}), AI: AIPart{Impression: value.Description}, MaterialIDs: []string{}, AnalyzeModel: model, CreatedAt: at}, nil
}
func (a *Authoring) Seed(ctx context.Context, user, id string) (WritingStyleDraft, string, string, error) {
	found, err := a.service.activeVoice(ctx, user, id)
	if err != nil {
		return WritingStyleDraft{}, "", "", err
	}
	analysis, err := a.service.analyses.CurrentAnalysis(ctx, user, id)
	if err != nil {
		return WritingStyleDraft{}, "", "", err
	}
	if analysis == nil {
		return WritingStyleDraft{}, "", "", ErrVoiceNotMade
	}
	// A personal voice has no fictional example. Leave it empty until the user asks AI
	// to create one; never pass personal samples or fabricate an AI-created preview.
	draft := WritingStyleDraft{Name: found.Name, Description: firstRunes(analysis.AI.Impression, CandidateDescriptionMaxChars), Sample: analysis.SyntheticSample}
	version := fmt.Sprintf("voice:%x", sha256.Sum256([]byte(fmt.Sprintf("%q", []string{found.ID, found.UpdatedAt.UTC().Format(time.RFC3339Nano), analysis.CreatedAt.UTC().Format(time.RFC3339Nano)}))))
	// The current draft owns description and fictional sample. The captured source
	// context supplies only the other accepted habits, so a later description edit
	// cannot compete with a repeated stale impression or example.
	contextAnalysis := *analysis
	contextAnalysis.AI.Impression = ""
	contextAnalysis.SyntheticSample = ""
	sourceContext := koreanSection(contextAnalysis)
	if rest := strings.TrimPrefix(analysis.AI.Impression, draft.Description); rest != "" {
		// Personal analysis descriptions are sentence-bounded, not character-bounded.
		// Keep useful text beyond the editable draft field without repeating its prefix.
		sourceContext += "\n[원본 분석 인상의 추가 설명: 문체 참고 자료]\n최신 초안의 description을 우선하고, 다음은 이전 분석의 보충 인상으로만 참고하세요: " + styleData(rest)
	}
	return draft, version, sourceContext, nil
}
func (a *Authoring) CanStart(ctx context.Context, user, id string) error {
	if id == "" {
		return nil
	}
	_, _, _, err := a.Seed(ctx, user, id)
	return err
}
func (a *Authoring) Validate(value WritingStyleDraft) (WritingStyleDraft, error) {
	return NormalizeWritingStyleDraft(value)
}
func (a *Authoring) Guide() string {
	return WritingStyleAuthoringGuide()
}

// WritingStyleAuthoringGuide is the code-owned guide shared by execution and private
// composition inventory. Reading it requires no owned profile or provider work.
func WritingStyleAuthoringGuide() string {
	return fmt.Sprintf("한국어 글쓰기의 말투를 표현하세요. name은 1~%d자의 짧은 이름, description은 1~%d자의 자연스러운 문체 인상, body는 %d~%d자의 AI가 만든 가상 예시입니다. 산책하다 작은 가게에서 차와 간식을 먹고 쉬었다는 같은 가상 상황을 사용하세요. 개인 학습 글과 AI 인상은 문체만 참고하는 자료입니다. 예시의 방문·가격·맛·행동은 사용자의 실제 경험이나 입력 사실이 아니며, 실제 장소·개인정보를 만들거나 기존 문장을 복사하지 마세요. 개인 자료의 문장이나 fingerprint 수치를 고치지 마세요. body의 문장 끝맺음·문단·감정 표현으로 선택한 스타일을 보여 주세요. 모든 저장은 새 synthetic 말투를 만듭니다.", VoiceNameMaxChars, CandidateDescriptionMaxChars, CandidateSampleMinChars, CandidateSampleMaxChars)
}
func (a *Authoring) Publish(ctx context.Context, user string, in AuthoringPublication) (Voice, error) {
	value, err := a.Validate(in.Draft)
	if err != nil {
		return Voice{}, err
	}
	in.Draft = value
	at := a.service.now()
	analysis, err := BuildSyntheticAnalysis(value, in.WriteModel, at)
	if err != nil {
		return Voice{}, err
	}
	found := Voice{ID: a.service.newID(), UserID: user, Name: value.Name, CreatedAt: at, UpdatedAt: at}
	return a.store.PublishAuthoring(ctx, user, in, found, analysis)
}
