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
	return draft, version, koreanSection(*analysis), nil
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
	return fmt.Sprintf("한국어글쓰기의말투를표현하세요. name은1~%d자의짧은이름,description은1~%d자의자연스러운인상,body는%d~%d자의AI가만든가상예시입니다. 예시는산책하다작은가게에서차와간식을먹고쉬었다는같은가상상황을사용하세요. 실제사용자경험이나장소·개인정보를만들거나기존문장을복사하지마세요. 개인자료의문장이나fingerprint수치를고치지마세요. body의문장끝맺음·문단·감정표현으로선택한스타일을보여주세요. 모든저장은새synthetic말투를만듭니다.", VoiceNameMaxChars, CandidateDescriptionMaxChars, CandidateSampleMinChars, CandidateSampleMaxChars)
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
