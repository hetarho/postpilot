package voice

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

const CandidateJobKind = "writing_voice_candidates"
const CandidateCount = 8
const CandidateDescriptionMaxChars = 200
const CandidateSampleMinChars = 200
const CandidateSampleMaxChars = 700
const candidatePromptTokensPerStyle = 750
const candidateResponseBytesPerStyle = 8 * 1024

var (
	ErrCandidateNotFound      = errors.New("writing voice candidate not found")
	ErrCandidatesNotReady     = errors.New("writing voice candidates are not ready")
	ErrCandidateModelRequired = errors.New("an enabled writing model is required for writing styles")
)

type CandidatesRunningError struct{ ActiveID string }

func (e *CandidatesRunningError) Error() string {
	return "writing voice candidates are already running"
}

type CandidateOutputError struct{ Cause error }

func (e *CandidateOutputError) Error() string {
	return "writing voice candidate output is incomplete or invalid"
}
func (e *CandidateOutputError) Unwrap() error { return e.Cause }
func (e *CandidateOutputError) Failure() llm.Failure {
	return llm.Failure{Reason: "WRITING_VOICE_CANDIDATE_OUTPUT_INVALID"}
}

type WritingCandidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sample      string `json:"sample"`
}

type CandidateBatch struct {
	JobID      string
	Count      int
	Candidates []WritingCandidate
}

type LatestCandidates struct {
	JobID       string
	ResultJobID string
	Candidates  []WritingCandidate
}

type CandidateRun struct {
	ID, UserID, WriteModel string
	Payload                []byte
}
type CandidateJobRequest struct {
	UserID, WriteModel             string
	Payload                        []byte
	CompletionTokens, PromptTokens int
}
type CandidateJob struct {
	ID, Status, WriteModel string
	Payload                []byte
}

type CandidateJobs interface {
	EnqueueCandidates(context.Context, CandidateJobRequest) (string, error)
	CandidateResult(context.Context, string, string) (CandidateJob, error)
	LatestCandidates(context.Context, string, string) (*CandidateJob, error)
	SaveCandidateResult(context.Context, string, []byte) error
	CancelCandidates(context.Context, string, string) error
}

type CandidateAdoption struct {
	Voice              Voice
	JobID, CandidateID string
	Analysis           Analysis
	MakeDefault        bool
}

type CandidateStore interface {
	AdoptCandidate(context.Context, CandidateAdoption) (Voice, error)
}
type CandidateBudget interface{ Short(nativeEffort bool) int }
type CandidateEstimator interface {
	CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool)
}
type CandidateEstimate struct {
	Credits         int
	Free, Available bool
}

// CandidateService is an independent account-owned use case. Its required collaborators
// are constructor ports; it never creates a personal sample or starts ordinary analysis.
type CandidateService struct {
	models    Models
	jobs      CandidateJobs
	store     CandidateStore
	budget    CandidateBudget
	estimates CandidateEstimator
	now       func() time.Time
	newID     func() string
	shuffle   func([]string) error
}

func NewCandidateService(models Models, jobs CandidateJobs, store CandidateStore, budget CandidateBudget, estimates CandidateEstimator) *CandidateService {
	if models == nil || jobs == nil || store == nil || budget == nil || estimates == nil {
		panic("voice: candidate service needs all constructor ports")
	}
	return &CandidateService{models: models, jobs: jobs, store: store, budget: budget, estimates: estimates, now: time.Now, newID: newID, shuffle: shuffleDirections}
}

// One fictional scene makes every chosen format directly comparable.
const candidateScene = "가상의 상황: 주말 오후, 산책하다 작은 가게에 들렀어요. 따뜻한 차와 간식을 먹으며 창가에서 잠깐 쉬었어요. 실제 장소나 사용자 경험이 아닌 같은 짧은 장면을 모든 후보가 각자의 말투로 이야기해 주세요."
const candidateSystem = `사용자가 고를 한국어 글쓰기 말투 %d가지를 만드세요. 이는 사용자의 실제 말투를 추정한 분석이 아니라 AI가 만든 가상 스타일입니다. 주어진 %d개 방향을 각각 한 번씩 사용하고, 모든 후보는 동일한 가상 장면을 이야기하세요. 문장 길이, 끝맺음, 감정 표현, 문단 모양이 다른 %d개 스타일이어야 합니다. 이름은 서로 다른 자연스러운 한국어 1~50자, 설명은 쉬운 한국어 1~200자, 예시는 각각 한국어 200~700자로 쓰세요. 예시에 브랜드, 실제 장소, 인물, 사용자 개인정보나 사실을 넣지 마세요. 보통 8~12문장으로 시작과 마무리를 넣고 글쓴이의 느낌이 드러나게 써 주세요. directions와 같은 순서의 candidates 배열만 있는 JSON 객체 하나로 답하세요. 정확히 %d개여야 합니다. 각 원소는 name, description, sample 필드만 갖습니다.`

var candidateDirections = []string{
	"담백하게 짧은 문장으로 핵심만 말하는 말투",
	"친구에게 수다 떨듯 편하고 따뜻한 말투",
	"차분하게 자세히 설명하는 정중한 말투",
	"작은 즐거움에 감탄하는 밝고 경쾌한 말투",
	"분위기와 감각을 잔잔하게 기록하는 말투",
	"가벼운 농담을 곁들이는 친근한 말투",
	"좋은 점과 아쉬운 점을 솔직하게 비교하는 말투",
	"다녀온 순간을 시간 순서대로 생생하게 전하는 말투",
	"여운을 남기는 부드럽고 서정적인 말투",
	"또렷한 문장으로 정보를 먼저 전하는 말투",
	"소소한 감정을 자연스럽게 드러내는 일기 같은 말투",
	"질문을 곁들여 독자에게 말을 거는 말투",
	"꾸밈없이 조용하게 자신의 느낌을 적는 말투",
	"간단한 소제목과 짧은 문단을 사용하는 정돈된 말투",
	"가끔 이모지를 넣어 밝게 이야기하는 말투",
	"편안한 해요체로 천천히 추천하는 말투",
}

type candidateInput struct {
	Count            int      `json:"count,omitempty"`
	Directions       []string `json:"directions"`
	Scene            string   `json:"scene"`
	CompletionTokens int      `json:"completion_tokens"`
}

type candidateResult struct {
	Count      int                `json:"count,omitempty"`
	Candidates []WritingCandidate `json:"candidates"`
}

func shuffleDirections(values []string) error {
	for i := len(values) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		values[i], values[j.Int64()] = values[j.Int64()], values[i]
	}
	return nil
}

func (s *CandidateService) eligible(ref llm.ModelRef) (llm.ModelInfo, error) {
	info, ok := s.models.Resolve(ref)
	if ref.ProviderID == "" || ref.ModelID == "" || !ok || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return llm.ModelInfo{}, ErrCandidateModelRequired
	}
	return info, nil
}

func candidateRequest(input candidateInput) llm.Request {
	count, _ := NormalizeCandidateCount(input.Count)
	raw, _ := json.Marshal(struct {
		Directions []string `json:"directions"`
		Scene      string   `json:"scene"`
	}{input.Directions, input.Scene})
	return llm.Request{System: fmt.Sprintf(candidateSystem, count, count, count, count), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(string(raw))}}}, Stage: llm.StageNameWrite, Reasoning: llm.ReasoningLow, MaxTokens: input.CompletionTokens}
}

func requestedCandidateCount(counts []int) (int, error) {
	if len(counts) > 1 {
		return 0, ErrCandidateCount
	}
	if len(counts) == 0 {
		return CandidateCount, nil
	}
	return NormalizeCandidateCount(counts[0])
}
func candidateCompletionTokens(base, count int) (int, error) {
	if base <= 0 || base > (int(^uint(0)>>1)-CandidateCount)/count {
		return 0, errors.New("writing voice candidate completion budget is invalid")
	}
	return (base*count + CandidateCount - 1) / CandidateCount, nil
}

func (s *CandidateService) Estimate(ctx context.Context, ref llm.ModelRef, counts ...int) (CandidateEstimate, error) {
	count, err := requestedCandidateCount(counts)
	if err != nil {
		return CandidateEstimate{}, err
	}
	info, err := s.eligible(ref)
	if err != nil {
		return CandidateEstimate{}, err
	}
	completion, err := candidateCompletionTokens(s.budget.Short(info.ReasoningNativeEffort), count)
	if err != nil {
		return CandidateEstimate{}, err
	}
	if info.Levels[llm.StageNameWrite] == "free" {
		return CandidateEstimate{Free: true, Available: true}, nil
	}
	// Every possible direction subset fits this input allowance; the estimate adapter
	// applies the shared admission prompt reservation bound.
	credits, ok := s.estimates.CallCredits(ctx, info, int64(candidatePromptTokensPerStyle*count), int64(completion))
	return CandidateEstimate{Credits: credits, Available: ok}, nil
}

func (s *CandidateService) Start(ctx context.Context, userID string, ref llm.ModelRef, counts ...int) (string, error) {
	count, err := requestedCandidateCount(counts)
	if err != nil {
		return "", err
	}
	info, err := s.eligible(ref)
	if err != nil {
		return "", err
	}
	directions := append([]string(nil), candidateDirections...)
	if err := s.shuffle(directions); err != nil {
		return "", fmt.Errorf("randomize candidate directions: %w", err)
	}
	completion, err := candidateCompletionTokens(s.budget.Short(info.ReasoningNativeEffort), count)
	if err != nil {
		return "", err
	}
	input := candidateInput{Count: count, Directions: directions[:count], Scene: candidateScene, CompletionTokens: completion}
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return s.jobs.EnqueueCandidates(ctx, CandidateJobRequest{UserID: userID, WriteModel: ref.String(), Payload: payload, CompletionTokens: input.CompletionTokens, PromptTokens: promptTokens(candidateRequest(input))})
}

func (s *CandidateService) Run(ctx context.Context, run CandidateRun, progress Progress) error {
	var input candidateInput
	if err := json.Unmarshal(run.Payload, &input); err != nil {
		return fmt.Errorf("decode candidate input: %w", err)
	}
	count, err := NormalizeCandidateCount(input.Count)
	if err != nil || len(input.Directions) != count || input.Scene != candidateScene || input.CompletionTokens <= 0 {
		return errors.New("writing voice candidate input is invalid")
	}
	input.Count = count
	seen := map[string]bool{}
	for _, direction := range input.Directions {
		allowed := false
		for _, known := range candidateDirections {
			if direction == known {
				allowed = true
				break
			}
		}
		if !allowed || seen[direction] {
			return errors.New("writing voice candidate directions are invalid")
		}
		seen[direction] = true
	}
	ref, err := parseModelRef(run.WriteModel)
	if err != nil {
		return ErrCandidateModelRequired
	}
	request := candidateRequest(input)
	if info, ok := s.models.Resolve(ref); ok && info.StructuredOutput {
		request.JSONSchema = WritingCandidateSchema(count)
		if len(request.JSONSchema) == 0 {
			return errors.New("writing voice candidate schema is invalid")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	progress("write", 0, 1)
	response, err := s.models.Complete(ctx, ref, request)
	if err != nil {
		return err
	}
	candidates, err := parseCandidates(response.Text, count)
	if err != nil {
		if response.FinishReason == "length" {
			return llm.ResponseParseError(response, err)
		}
		return &CandidateOutputError{Cause: err}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := json.Marshal(candidateResult{Count: count, Candidates: candidates})
	if err != nil {
		return err
	}
	if err := s.jobs.SaveCandidateResult(ctx, run.ID, payload); err != nil {
		return err
	}
	progress("write", 1, 1)
	return nil
}

func parseCandidates(text string, counts ...int) ([]WritingCandidate, error) {
	count, err := requestedCandidateCount(counts)
	if err != nil {
		return nil, err
	}
	// Fully Unicode-escaped maximum-size fields fit under this count-specific bound.
	if len(text) > candidateResponseBytesPerStyle*count {
		return nil, errors.New("candidate output exceeds the response bound")
	}
	candidate, ok := llm.JSONCandidate(text)
	if !ok {
		return nil, errors.New("candidate JSON required")
	}
	var result candidateResult
	if err := json.Unmarshal([]byte(candidate), &result); err != nil {
		return nil, err
	}
	if len(result.Candidates) != count {
		return nil, fmt.Errorf("exactly %d candidates are required", count)
	}
	names, samples := map[string]bool{}, map[string]bool{}
	for i := range result.Candidates {
		value := &result.Candidates[i]
		value.Name, value.Description, value.Sample = strings.TrimSpace(value.Name), strings.TrimSpace(value.Description), strings.TrimSpace(value.Sample)
		if !validKoreanCandidateText(value.Name, 1, VoiceNameMaxChars) || !validKoreanCandidateText(value.Description, 1, CandidateDescriptionMaxChars) || !validKoreanCandidateText(value.Sample, CandidateSampleMinChars, CandidateSampleMaxChars) || !containsKoreanProse(ProseSentences(value.Sample)) || names[collapseSpace(value.Name)] || samples[collapseSpace(value.Sample)] {
			return nil, errors.New("candidate fields must be bounded, Korean and distinct")
		}
		names[collapseSpace(value.Name)], samples[collapseSpace(value.Sample)] = true, true
		value.ID = fmt.Sprintf("style-%d", i+1)
	}
	return result.Candidates, nil
}

func validKoreanCandidateText(value string, minChars, maxChars int) bool {
	n := utf8.RuneCountInString(value)
	return n >= minChars && n <= maxChars && containsKoreanProse([]string{value})
}

func (s *CandidateService) Get(ctx context.Context, userID, jobID string) (CandidateBatch, error) {
	found, err := s.jobs.CandidateResult(ctx, userID, jobID)
	if err != nil {
		return CandidateBatch{}, err
	}
	if found.Status != "done" {
		return CandidateBatch{}, ErrCandidatesNotReady
	}
	var result candidateResult
	if err := json.Unmarshal(found.Payload, &result); err != nil {
		return CandidateBatch{}, fmt.Errorf("decode saved writing candidates: %w", err)
	}
	count, err := NormalizeCandidateCount(result.Count)
	if err != nil || len(result.Candidates) != count {
		return CandidateBatch{}, ErrCandidatesNotReady
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return CandidateBatch{}, ErrCandidatesNotReady
	}
	validated, err := parseCandidates(string(raw), count)
	if err != nil {
		return CandidateBatch{}, ErrCandidatesNotReady
	}
	for i := range validated {
		if result.Candidates[i].ID != validated[i].ID {
			return CandidateBatch{}, ErrCandidatesNotReady
		}
	}
	return CandidateBatch{JobID: found.ID, Count: count, Candidates: validated}, nil
}

func (s *CandidateService) Latest(ctx context.Context, userID string) (LatestCandidates, error) {
	// Finished results stay immutable. Read the successful batch first so a newer
	// attempt created between these reads can never appear older than its result.
	result, err := s.jobs.LatestCandidates(ctx, userID, "done")
	if err != nil {
		return LatestCandidates{}, err
	}
	latest, err := s.jobs.LatestCandidates(ctx, userID, "")
	if err != nil {
		return LatestCandidates{}, err
	}
	out := LatestCandidates{Candidates: []WritingCandidate{}}
	if latest != nil {
		out.JobID = latest.ID
	}
	if result != nil {
		batch, err := s.Get(ctx, userID, result.ID)
		if err != nil {
			return LatestCandidates{}, err
		}
		out.ResultJobID, out.Candidates = batch.JobID, batch.Candidates
	}
	return out, nil
}

func (s *CandidateService) Cancel(ctx context.Context, userID, jobID string) error {
	return s.jobs.CancelCandidates(ctx, userID, jobID)
}

func (s *CandidateService) Adopt(ctx context.Context, userID, jobID, candidateID string, makeDefault bool) (Voice, error) {
	batch, err := s.Get(ctx, userID, jobID)
	if err != nil {
		return Voice{}, err
	}
	var chosen *WritingCandidate
	for i := range batch.Candidates {
		if batch.Candidates[i].ID == candidateID {
			chosen = &batch.Candidates[i]
			break
		}
	}
	if chosen == nil {
		return Voice{}, ErrCandidateNotFound
	}
	found, err := s.jobs.CandidateResult(ctx, userID, jobID)
	if err != nil {
		return Voice{}, err
	}
	now := s.now()
	analysis, err := BuildSyntheticAnalysis(WritingStyleDraft{Name: chosen.Name, Description: chosen.Description, Sample: chosen.Sample}, found.WriteModel, now)
	if err != nil {
		return Voice{}, err
	}
	return s.store.AdoptCandidate(ctx, CandidateAdoption{Voice: Voice{ID: s.newID(), UserID: userID, Name: chosen.Name, CreatedAt: now, UpdatedAt: now}, JobID: jobID, CandidateID: candidateID, Analysis: analysis, MakeDefault: makeDefault})
}
