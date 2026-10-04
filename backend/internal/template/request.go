package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// failureReasonRequestAnswerInvalid is the durable failure of a request whose answer still broke
// the grammar or a field rule after every correction (TMPL-60).
const failureReasonRequestAnswerInvalid = "TEMPLATE_REQUEST_ANSWER_INVALID"

var (
	// ErrRequestEmpty is a request with no text and no sample post: nothing to ask.
	ErrRequestEmpty = errors.New("template: the request has no text and no sample")
	// ErrRequestRunning is a second request while the account already has one queued or
	// running; one draft is locked per request, and the queue keeps one per account.
	ErrRequestRunning = errors.New("template: a template request is already running")
	// ErrSampleUnavailable is a sample post that is missing, another account's, or holds no
	// content to take a shape from (TMPL-64).
	ErrSampleUnavailable = errors.New("template: the sample post cannot be read")
	// ErrWriteModelRequired is a write ref that is missing, unknown, disabled or not a writer —
	// the refusal post generation gives for the same model (TMPL-58).
	ErrWriteModelRequired = errors.New("template: an enabled write model is required")
	// ErrRequestNotReady is a result read on a request that has not finished well.
	ErrRequestNotReady = errors.New("template: the request has no result")
	errRequestsUnwired = errors.New("template: requests are not wired")
)

// RequestLimits bound a request (TEMPLATE_REQUEST_*): the box, the corrections, and the
// separated 지침 material the result lists.
type RequestLimits struct {
	MaxChars       int
	CorrectionsMax int
	WishesMax      int
	WishMaxChars   int
}

func (l RequestLimits) valid() bool {
	return l.MaxChars > 0 && l.CorrectionsMax >= 0 && l.WishesMax > 0 && l.WishMaxChars > 0
}

// Draft is a template's four authored texts as one value: what a request reads from the
// editor and what its answer puts back. The two generation numbers are not in it — a request
// never sets them (TMPL-60).
type Draft struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TitleArea   string `json:"title_area"`
	Body        string `json:"body"`
}

// Sample is a post given as the shape to follow, already written out as plain text by the
// context that owns posts: its title and its blocks, photos as positions only (TMPL-64).
type Sample struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// StartRequest is one press of the request box. WriteModel is the explicit `provider/model`
// ref the client sends (MODEL-23): the server never infers it from a stored selection.
type StartRequest struct {
	WriteModel     string
	Language       Language
	Text           string
	Draft          Draft
	TemplateID     string
	SamplePostSlug string
}

// RequestResult is a finished request's answer, checked by the rules a typed draft is held to.
type RequestResult struct {
	Draft
	Wishes []string `json:"wishes"`
}

// RequestRun is the queued job as its handler receives it.
type RequestRun struct {
	ID         string
	UserID     string
	WriteModel string
	Payload    []byte
}

// RequestJob is what the queue is asked to hold: the frozen input, and the calls the credit hold
// prices — the first one and every correction allowed (QUOTA-67), each at the cap the input
// froze for every call.
type RequestJob struct {
	UserID           string
	WriteModel       string
	Payload          []byte
	Calls            int
	CompletionTokens int
}

// RequestModels is the model registry as a request uses it.
type RequestModels interface {
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error)
}

// RequestSamples reads a post as a sample. A post that is missing, another account's or holds
// no content is ErrSampleUnavailable.
type RequestSamples interface {
	RequestSample(ctx context.Context, userID, slug string) (Sample, error)
}

// RequestJobs is the queue as a request uses it. Enqueue answers ErrRequestRunning when the
// account already has one; Payload answers ErrNotFound for a job that is not the owner's
// template request and ErrRequestNotReady for one that has not finished well; Cancel answers
// ErrNotFound for a job that is not the owner's template request and nothing for a finished one.
type RequestJobs interface {
	EnqueueRequest(ctx context.Context, job RequestJob) (string, error)
	CancelRequest(ctx context.Context, userID, jobID string) error
	SaveRequestResult(ctx context.Context, jobID string, payload []byte) error
	RequestPayload(ctx context.Context, userID, jobID string) ([]byte, error)
}

// RequestBudget is the completion cap policy, received from its owner (cmd/api, from the
// platform completion budget) rather than held here: this context asks for the cap its call
// needs and holds no number of its own (ARCH-21).
type RequestBudget interface {
	// Short is a short structured answer's cap — four fields and the wishes do not grow with a
	// target length — with the reasoning headroom a native-effort model needs (GEN-22).
	Short(nativeEffort bool) int
}

type requests struct {
	models  RequestModels
	samples RequestSamples
	jobs    RequestJobs
	budget  RequestBudget
	limits  RequestLimits
}

// ConfigureRequests wires the template request (TMPL-58). It is the one template surface that
// calls a provider or enqueues a job (TMPL-16), so everything else in this context runs
// without it.
func (s *Service) ConfigureRequests(models RequestModels, samples RequestSamples, jobs RequestJobs, budget RequestBudget, limits RequestLimits) {
	if !limits.valid() {
		panic("template: request limits must be positive")
	}
	if budget == nil {
		panic("template: a completion budget policy is required")
	}
	s.requests = &requests{models: models, samples: samples, jobs: jobs, budget: budget, limits: limits}
}

// RequestLimits are the configured request ceilings, or the zero value before wiring.
func (s *Service) RequestLimits() RequestLimits {
	if s.requests == nil {
		return RequestLimits{}
	}
	return s.requests.limits
}

// requestInput is what freezes on the job row at start: everything the call will read, so a
// draft edited or a post changed after the press cannot change the run.
type requestInput struct {
	Language Language `json:"language"`
	Text     string   `json:"text"`
	Draft    Draft    `json:"draft"`
	Sample   *Sample  `json:"sample,omitempty"`
	// CompletionTokens is every call's cap, sized at start from the model's native effort and
	// priced by the hold, so a catalog flag flipped before the run cannot make the two differ.
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

// StartRequest refuses everything it can before any credit is held, then enqueues the request
// on the write model the client named.
func (s *Service) StartRequest(ctx context.Context, userID string, start StartRequest) (string, error) {
	r := s.requests
	if r == nil {
		return "", errRequestsUnwired
	}
	if start.Language != LanguageKorean && start.Language != LanguageEnglish {
		return "", ErrUnsupportedLanguage
	}
	text := strings.TrimSpace(start.Text)
	if chars := utf8.RuneCountInString(text); chars > r.limits.MaxChars {
		return "", &FieldTooLongError{Field: "request", Chars: chars, Max: r.limits.MaxChars}
	}
	if err := s.boundDraft(start.Draft); err != nil {
		return "", err
	}
	if start.TemplateID != "" {
		if _, err := s.store.Get(ctx, userID, start.TemplateID); err != nil {
			return "", err
		}
	} else {
		// A new template at the cap would be refused at 저장, so the request that would make
		// it is refused before it spends anything (TMPL-62).
		held, err := s.store.List(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("count templates: %w", err)
		}
		if len(held) >= s.limits.MaxPerAccount {
			return "", ErrTooMany
		}
	}
	ref, ok := parseModelRef(start.WriteModel)
	info, found := r.models.Resolve(ref)
	if !ok || !found || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return "", ErrWriteModelRequired
	}
	input := requestInput{Language: start.Language, Text: text, Draft: start.Draft, CompletionTokens: r.budget.Short(info.ReasoningNativeEffort)}
	if slug := strings.TrimSpace(start.SamplePostSlug); slug != "" {
		sample, err := r.samples.RequestSample(ctx, userID, slug)
		if err != nil {
			return "", err
		}
		input.Sample = &sample
	}
	if text == "" && input.Sample == nil {
		return "", ErrRequestEmpty
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode template request: %w", err)
	}
	return r.jobs.EnqueueRequest(ctx, RequestJob{
		UserID: userID, WriteModel: start.WriteModel, Payload: payload,
		Calls: 1 + r.limits.CorrectionsMax, CompletionTokens: input.CompletionTokens,
	})
}

// boundDraft holds the draft a request carries to its fields' own ceilings. It does not require
// a name or a body, and it does not parse: a request may well be "fix this", and the draft it
// sends is whatever the editor holds.
func (s *Service) boundDraft(draft Draft) error {
	for _, field := range []struct {
		name, value string
		max         int
	}{
		{"name", draft.Name, s.limits.NameMaxChars},
		{"description", draft.Description, s.limits.DescriptionMaxChars},
		{"title_area", draft.TitleArea, s.limits.TitleAreaMaxChars},
		{"body", draft.Body, s.limits.BodyMaxChars},
	} {
		if chars := utf8.RuneCountInString(strings.TrimSpace(field.value)); chars > field.max {
			return &FieldTooLongError{Field: field.name, Chars: chars, Max: field.max}
		}
	}
	return nil
}

// RunRequest is the job: the 형식 안내 and the request rules, then the request, the draft and the
// sample; an answer that breaks a rule goes back with what it broke, at most CorrectionsMax
// times (TMPL-60). A provider failure, a truncation or a filtered answer is not corrected — none
// of them is something the model can fix by being told.
func (s *Service) RunRequest(ctx context.Context, run RequestRun, progress func(stage string, done, total int)) error {
	r := s.requests
	if r == nil {
		return errRequestsUnwired
	}
	var input requestInput
	if err := json.Unmarshal(run.Payload, &input); err != nil {
		return fmt.Errorf("decode template request: %w", err)
	}
	ref, ok := parseModelRef(run.WriteModel)
	if !ok {
		return ErrWriteModelRequired
	}
	system, err := s.requestSystem(input.Language)
	if err != nil {
		return err
	}
	// The cap the start froze and the hold priced; a payload without one is an ordinary model's.
	maxTokens := input.CompletionTokens
	if maxTokens <= 0 {
		maxTokens = r.budget.Short(false)
	}
	ask := llm.Message{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(requestMessage(input))}}
	messages := []llm.Message{ask}
	calls := 1 + r.limits.CorrectionsMax
	for attempt := 0; ; attempt++ {
		progress("write", attempt, calls)
		request := llm.Request{
			System: system, Messages: messages,
			Stage: llm.StageNameWrite, Reasoning: llm.ReasoningLow, MaxTokens: maxTokens,
		}
		if info, found := r.models.Resolve(ref); found && info.StructuredOutput {
			request.JSONSchema = RequestAnswerSchema()
		}
		response, err := r.models.Complete(ctx, ref, request)
		if err != nil {
			return err
		}
		result, checkErr := s.checkAnswer(response.Text)
		if checkErr == nil {
			payload, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("encode template request result: %w", err)
			}
			if err := r.jobs.SaveRequestResult(ctx, run.ID, payload); err != nil {
				return fmt.Errorf("save template request result: %w", err)
			}
			progress("write", calls, calls)
			return nil
		}
		if response.FinishReason == "length" {
			return llm.ResponseParseError(response, checkErr)
		}
		if response.FinishReason == "content_filter" {
			return fmt.Errorf("%w: the answer was filtered", llm.ErrBadOutput)
		}
		if attempt >= r.limits.CorrectionsMax {
			return &RequestAnswerInvalidError{Cause: checkErr}
		}
		// The owner may have stopped the request while the last answer was being written; a
		// correction is another paid call, so none is sent once they have (TMPL-63).
		if err := ctx.Err(); err != nil {
			return err
		}
		// A correction carries the request, the last answer and what it broke — never the earlier
		// wrong answers, which the model has no use for and every later call would pay for again.
		messages = []llm.Message{
			ask,
			{Role: llm.RoleAssistant, Parts: []llm.Part{llm.TextPart(response.Text)}},
			{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(correctionMessage(input.Language, checkErr))}},
		}
	}
}

// CancelRequest stops the owner's queued or running request (TMPL-63). Only confirmed usage is
// charged (QUOTA-49); a request that has already finished is left as it is.
func (s *Service) CancelRequest(ctx context.Context, userID, jobID string) error {
	if s.requests == nil {
		return errRequestsUnwired
	}
	return s.requests.jobs.CancelRequest(ctx, userID, jobID)
}

// RequestResult reads a finished request's answer for its owner.
func (s *Service) RequestResult(ctx context.Context, userID, jobID string) (RequestResult, error) {
	r := s.requests
	if r == nil {
		return RequestResult{}, errRequestsUnwired
	}
	payload, err := r.jobs.RequestPayload(ctx, userID, jobID)
	if err != nil {
		return RequestResult{}, err
	}
	var result RequestResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return RequestResult{}, fmt.Errorf("decode template request result: %w", err)
	}
	if result.Wishes == nil {
		result.Wishes = []string{}
	}
	return result, nil
}

// answerJSONError is an answer that is not the JSON object the rules ask for.
type answerJSONError struct{ cause error }

func (e *answerJSONError) Error() string { return "the answer is not the requested JSON object" }
func (e *answerJSONError) Unwrap() error { return e.cause }

// checkAnswer reads an answer and holds it to exactly the rules a typed draft is held to
// (validDraft), then trims and caps the separated wishes (TMPL-61).
func (s *Service) checkAnswer(text string) (RequestResult, error) {
	candidate, ok := llm.JSONCandidate(text)
	if !ok {
		return RequestResult{}, &answerJSONError{}
	}
	var answer struct {
		Draft
		Wishes []string `json:"wishes"`
	}
	if err := json.Unmarshal([]byte(candidate), &answer); err != nil {
		return RequestResult{}, &answerJSONError{cause: err}
	}
	draft, err := s.validDraft(answer.Draft)
	if err != nil {
		return RequestResult{}, err
	}
	limits := s.requests.limits
	wishes := make([]string, 0, limits.WishesMax)
	for _, wish := range answer.Wishes {
		wish = strings.TrimSpace(wish)
		if wish == "" {
			continue
		}
		if runes := []rune(wish); len(runes) > limits.WishMaxChars {
			wish = strings.TrimSpace(string(runes[:limits.WishMaxChars]))
		}
		wishes = append(wishes, wish)
		if len(wishes) == limits.WishesMax {
			break
		}
	}
	return RequestResult{Draft: draft, Wishes: wishes}, nil
}

// RequestAnswerInvalidError is a request whose last allowed answer still broke a rule. Its
// failure is the template context's own stable reason; the cause stays technical.
type RequestAnswerInvalidError struct{ Cause error }

func (e *RequestAnswerInvalidError) Error() string {
	return fmt.Sprintf("template request answer still invalid after corrections: %v", e.Cause)
}

func (e *RequestAnswerInvalidError) Unwrap() error { return e.Cause }

func (e *RequestAnswerInvalidError) Failure() llm.Failure {
	detail := ""
	if e.Cause != nil {
		detail = e.Cause.Error()
	}
	return llm.Failure{Reason: failureReasonRequestAnswerInvalid, TechnicalDetail: detail}
}

func parseModelRef(value string) (llm.ModelRef, bool) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, false
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, true
}
