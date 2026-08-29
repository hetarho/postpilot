package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const StructuredAnalysisPromptVersion = "voice-profile-v1"
const structuredAnalysisPrompt = `Analyze the supplied Korean authored corpus as writing style, not subject matter.
Return one JSON object with lexical_description, base_register, connective_style, intro_pattern, closing_pattern,
heading_habit, list_habit, emoji_use, and six integer axes (-3..3). Use an empty string for unsupported traits.
Never return topic-specific nouns as preferred vocabulary. Deterministic ending distribution and sentence length are calculated separately and override your estimates.`

type authoredContentJSON struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	Blocks  []struct {
		Type    string   `json:"type"`
		Content string   `json:"content"`
		File    string   `json:"file"`
		Items   []string `json:"items"`
	} `json:"blocks"`
}
type qualitativeAxesJSON struct {
	Involvement         int `json:"involvement"`
	Narrativity         int `json:"narrativity"`
	PersuasionOvertness int `json:"persuasion_overtness"`
	Abstractness        int `json:"abstractness"`
	AddresseeFocus      int `json:"addressee_focus"`
	Humor               int `json:"humor"`
}
type qualitativeJSON struct {
	LexicalDescription string              `json:"lexical_description"`
	BaseRegister       string              `json:"base_register"`
	ConnectiveStyle    string              `json:"connective_style"`
	IntroPattern       string              `json:"intro_pattern"`
	ClosingPattern     string              `json:"closing_pattern"`
	HeadingHabit       string              `json:"heading_habit"`
	ListHabit          string              `json:"list_habit"`
	EmojiUse           string              `json:"emoji_use"`
	Axes               qualitativeAxesJSON `json:"axes"`
}

func parseAuthoredContent(raw string) (authoredContentJSON, string, error) {
	var value authoredContentJSON
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return value, "", err
	}
	var body strings.Builder
	for _, block := range value.Blocks {
		switch block.Type {
		case "TEXT", "HEADING", "QUOTE":
			if strings.TrimSpace(block.Content) != "" {
				body.WriteString(block.Content)
				body.WriteString("\n")
			}
		case "LIST":
			for _, item := range block.Items {
				body.WriteString(item)
				body.WriteString("\n")
			}
		}
	}
	return value, strings.TrimSpace(body.String()), nil
}

func (s *Service) Finalize(ctx context.Context, userID, postSlug string, requested llm.ModelRef) (LearningEvent, string, bool, error) {
	if s.posts == nil {
		return LearningEvent{}, "", false, fmt.Errorf("voice personalization is not configured")
	}
	if requested.ProviderID == "" || requested.ModelID == "" {
		return LearningEvent{}, "", false, ErrAnalyzeModelRequired
	}
	if err := s.retireStaleRules(ctx, userID); err != nil {
		return LearningEvent{}, "", false, err
	}
	model, err := s.resolveAnalyzeModel(ctx, userID, requested)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	snapshot, err := s.posts.LearningSnapshot(ctx, userID, postSlug)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	hash := learningInputHash(snapshot)
	existing, err := s.personalization.FindLearningEvent(ctx, userID, postSlug, snapshot.BaselineRevision, hash)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	if existing != nil {
		jobID, resumeErr := s.resumeLearningEvent(ctx, existing, model)
		return *existing, jobID, true, resumeErr
	}
	event := LearningEvent{ID: s.newID(), UserID: userID, PostSlug: postSlug, BaselineRevision: snapshot.BaselineRevision, InputHash: hash, BaselineJSON: snapshot.BaselineJSON, FinalJSON: snapshot.FinalJSON, ModelRef: model.String(), Status: "queued", CreatedAt: s.now()}
	if err = s.personalization.InsertLearningEvent(ctx, event); err != nil {
		// The database uniqueness constraint is the final arbiter when two tabs
		// finalize the same immutable input concurrently.
		if raced, findErr := s.personalization.FindLearningEvent(ctx, userID, postSlug, snapshot.BaselineRevision, hash); findErr == nil && raced != nil {
			jobID, resumeErr := s.resumeLearningEvent(ctx, raced, model)
			return *raced, jobID, true, resumeErr
		}
		return LearningEvent{}, "", false, err
	}
	jobID, err := s.enqueueLearningEvent(ctx, &event, model)
	if err != nil {
		return event, "", false, err
	}
	return event, jobID, false, nil
}

// resumeLearningEvent makes the same explicit Finalize/Retry action the recovery
// boundary for an event whose durable job was failed by the restart sweep. Merely
// booting or reading the event never enqueues provider work.
func (s *Service) resumeLearningEvent(ctx context.Context, event *LearningEvent, model llm.ModelRef) (string, error) {
	if event.Status == "done" {
		return event.JobID, nil
	}
	if event.Status != "retryable" && event.JobID != "" {
		active, err := s.personalizationJobs.IsPersonalizationJobActive(ctx, event.JobID, event.UserID)
		if err != nil {
			return "", err
		}
		if active {
			return event.JobID, nil
		}
	}
	return s.enqueueLearningEvent(ctx, event, model)
}

func learningInputHash(snapshot FinalizationInput) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\x00%s\x00%s", snapshot.BaselineRevision, snapshot.BaselineJSON, snapshot.FinalJSON)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) enqueueLearningEvent(ctx context.Context, event *LearningEvent, model llm.ModelRef) (string, error) {
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{Kind: LearnJobKind, UserID: event.UserID, PostSlug: event.PostSlug, Model: model.String(), Payload: event.ID})
	if err != nil {
		_ = s.personalization.SetLearningEventStatus(ctx, event.UserID, event.ID, "retryable", err.Error(), nil)
		event.Status = "retryable"
		event.Error = err.Error()
		return "", err
	}
	event.JobID = jobID
	event.ModelRef = model.String()
	event.Status = "queued"
	event.Error = ""
	if err = s.personalization.SetLearningEventJob(ctx, event.UserID, event.ID, jobID); err != nil {
		return jobID, err
	}
	return jobID, nil
}

func (s *Service) RetryLearning(ctx context.Context, userID, eventID string, requested llm.ModelRef) (LearningEvent, string, error) {
	event, err := s.personalization.GetLearningEvent(ctx, userID, eventID)
	if err != nil {
		return LearningEvent{}, "", err
	}
	if event.Status == "done" {
		return *event, event.JobID, nil
	}
	if requested.ProviderID == "" || requested.ModelID == "" {
		return LearningEvent{}, "", ErrAnalyzeModelRequired
	}
	model, err := s.resolveAnalyzeModel(ctx, userID, requested)
	if err != nil {
		return LearningEvent{}, "", err
	}
	id, err := s.resumeLearningEvent(ctx, event, model)
	return *event, id, err
}

func (s *Service) GetLearningEvent(ctx context.Context, userID, eventID string) (LearningEvent, error) {
	found, err := s.personalization.GetLearningEvent(ctx, userID, eventID)
	if err != nil {
		return LearningEvent{}, err
	}
	return *found, nil
}

func (s *Service) Learn(ctx context.Context, job LearningJob, progress Progress) error {
	event, err := s.personalization.GetLearningEvent(ctx, job.UserID, job.EventID)
	if err != nil {
		return err
	}
	if event.Status == "done" {
		progress("learn", 1, 1)
		return nil
	}
	_ = s.personalization.SetLearningEventStatus(ctx, job.UserID, event.ID, "running", "", nil)
	final, body, err := parseAuthoredContent(event.FinalJSON)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	_, baselineBody, err := parseAuthoredContent(event.BaselineJSON)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	sources, err := s.personalization.ListAuthoredSources(ctx, job.UserID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	var corpus strings.Builder
	for _, source := range sources {
		corpus.WriteString(source.Body)
		corpus.WriteString("\n\n")
	}
	corpus.WriteString(body)
	ref, err := parseModelRef(job.WriteModel)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	progress("learn", 0, 1)
	response, err := s.models.Complete(ctx, ref, llm.Request{System: structuredAnalysisPrompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(corpus.String())}}}})
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	var qualitative qualitativeJSON
	if err = json.Unmarshal([]byte(strings.TrimSpace(response.Text)), &qualitative); err != nil {
		return s.failLearning(ctx, *event, fmt.Errorf("typed voice analysis returned invalid JSON: %w", err))
	}
	profile := MeasuredProfile(corpus.String(), s.now)
	unknown := func(v string) VoiceValue {
		if strings.TrimSpace(v) == "" {
			return VoiceValue{Unknown: true, Source: SourceUnknown}
		}
		return VoiceValue{Value: strings.TrimSpace(v), Source: SourceAnalyzed}
	}
	profile.Lexical.Description = unknown(qualitative.LexicalDescription)
	if !profile.Endings.BaseRegister.Unknown && qualitative.BaseRegister != "" {
	} else {
		profile.Endings.BaseRegister = unknown(qualitative.BaseRegister)
	}
	profile.Syntax.ConnectiveStyle = unknown(qualitative.ConnectiveStyle)
	profile.Structure.IntroPattern = unknown(qualitative.IntroPattern)
	profile.Structure.ClosingPattern = unknown(qualitative.ClosingPattern)
	profile.Structure.HeadingHabit = unknown(qualitative.HeadingHabit)
	profile.Structure.ListHabit = unknown(qualitative.ListHabit)
	profile.Structure.EmojiUse = unknown(qualitative.EmojiUse)
	profile.Axes = AxesProfile{Involvement: qualitative.Axes.Involvement, Narrativity: qualitative.Axes.Narrativity, PersuasionOvertness: qualitative.Axes.PersuasionOvertness, Abstractness: qualitative.Axes.Abstractness, AddresseeFocus: qualitative.Axes.AddresseeFocus, Humor: qualitative.Axes.Humor}
	if err = validateAxes(profile.Axes); err != nil {
		return s.failLearning(ctx, *event, err)
	}
	overrides, err := s.personalization.ListManualOverrides(ctx, job.UserID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	for _, override := range overrides {
		if err = applyOverride(&profile, override.Layer, override.Field, override.Value); err != nil {
			return s.failLearning(ctx, *event, err)
		}
	}
	excerpt := excerptAroundTarget(body, s.config.FewShotExcerptTargetChars, s.config.FewShotExcerptMaxChars)
	source := AuthoredSource{ID: s.newID(), UserID: job.UserID, PostSlug: event.PostSlug, LearningEventID: event.ID, Title: final.Title, Tags: final.Tags, Body: body, Excerpt: excerpt, CreatedAt: s.now()}
	profile.Rules, err = s.personalization.ListRules(ctx, job.UserID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	rules, err := s.extractStyleRules(ctx, ref, AlignSentences(baselineBody, body))
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	rules, err = s.classifyRuleRelations(ctx, ref, rules, profile.Rules)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	profile.SourceCount = len(sources) + 1
	profile.Sources = append([]AuthoredSource{source}, sources...)
	if err = s.personalization.ApplyLearningResult(ctx, *event, LearningResult{Source: source, Profile: profile, Rules: rules}, s.config, s.now()); err != nil {
		return s.failLearning(ctx, *event, err)
	}
	progress("learn", 1, 1)
	return nil
}

func validateAxes(a AxesProfile) error {
	for _, v := range []int{a.Involvement, a.Narrativity, a.PersuasionOvertness, a.Abstractness, a.AddresseeFocus, a.Humor} {
		if v < -3 || v > 3 {
			return fmt.Errorf("voice axis is outside -3..3")
		}
	}
	return nil
}
func (s *Service) failLearning(ctx context.Context, event LearningEvent, cause error) error {
	_ = s.personalization.SetLearningEventStatus(ctx, event.UserID, event.ID, "retryable", cause.Error(), nil)
	return cause
}
