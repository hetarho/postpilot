package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const StructuredAnalysisPromptVersion = "voice-profile-v1"

// The prompt names every key the decoder expects — including the `axes` object and its six
// keys — because for a model without structured output the prompt is the only channel that
// carries the shape; a key it is not told about comes back missing and must publish as unknown.
const structuredAnalysisPrompt = `Analyze the supplied Korean authored corpus as writing style, not subject matter.
Return one JSON object with these string keys: lexical_description, base_register, connective_style, intro_pattern,
closing_pattern, heading_habit, list_habit, emoji_use. Use an empty string for unsupported traits.
Also return an "axes" object with exactly these six integer keys, each between -3 and 3:
involvement, narrativity, persuasion_overtness, abstractness, addressee_focus, humor.
Omit an axis key only when the corpus gives no evidence for it; never guess 0 as a filler.
Never return topic-specific nouns as preferred vocabulary. Deterministic ending distribution and sentence length are calculated separately and override your estimates.`

const structuredEnglishAnalysisPrompt = `Analyze the supplied English authored corpus as writing style, not subject matter.
Return one JSON object with these string keys: lexical_description, base_register, connective_style, intro_pattern,
closing_pattern, heading_habit, list_habit, emoji_use. Use an empty string for unsupported traits.
Also return an "axes" object with exactly these six integer keys, each between -3 and 3:
involvement, narrativity, persuasion_overtness, abstractness, addressee_focus, humor.
Omit an axis key only when the corpus gives no evidence for it; never guess 0 as a filler.
Never return topic-specific nouns as preferred vocabulary. Deterministic word length, register, contractions, connectives,
passive and nominal style, sentence cadence, structure, lexical habits, and axes are calculated separately and override estimates.`

func structuredAnalysisPromptForLanguage(language Language) string {
	if language == LanguageEnglish {
		return structuredEnglishAnalysisPrompt
	}
	return structuredAnalysisPrompt
}

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
	Involvement         *int `json:"involvement"`
	Narrativity         *int `json:"narrativity"`
	PersuasionOvertness *int `json:"persuasion_overtness"`
	Abstractness        *int `json:"abstractness"`
	AddresseeFocus      *int `json:"addressee_focus"`
	Humor               *int `json:"humor"`
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

func mergeQualitativeProfile(profile *StructuredProfile, qualitative qualitativeJSON, language Language, unknown func(string) VoiceValue) {
	if language != LanguageEnglish {
		// Keep the Korean merge order byte-for-byte equivalent to the original analyzer.
		profile.Lexical.Description = unknown(qualitative.LexicalDescription)
		if profile.Endings.BaseRegister.Unknown || qualitative.BaseRegister == "" {
			profile.Endings.BaseRegister = unknown(qualitative.BaseRegister)
		}
		profile.Syntax.ConnectiveStyle = unknown(qualitative.ConnectiveStyle)
		profile.Structure.IntroPattern = unknown(qualitative.IntroPattern)
		profile.Structure.ClosingPattern = unknown(qualitative.ClosingPattern)
		profile.Structure.HeadingHabit = unknown(qualitative.HeadingHabit)
		profile.Structure.ListHabit = unknown(qualitative.ListHabit)
		profile.Structure.EmojiUse = unknown(qualitative.EmojiUse)
		profile.Axes = AxesProfile{Involvement: qualitative.Axes.Involvement, Narrativity: qualitative.Axes.Narrativity, PersuasionOvertness: qualitative.Axes.PersuasionOvertness, Abstractness: qualitative.Axes.Abstractness, AddresseeFocus: qualitative.Axes.AddresseeFocus, Humor: qualitative.Axes.Humor}
		return
	}
	// English deterministic measurements are authoritative. Qualitative output fills only
	// genuinely unsupported fields and axes; it never turns English cadence into Korean
	// ending categories.
	fill := func(target *VoiceValue, candidate string) {
		if target.Unknown || strings.TrimSpace(target.Value) == "" {
			*target = unknown(candidate)
		}
	}
	fill(&profile.Lexical.Description, qualitative.LexicalDescription)
	fill(&profile.Endings.BaseRegister, qualitative.BaseRegister)
	fill(&profile.Syntax.ConnectiveStyle, qualitative.ConnectiveStyle)
	fill(&profile.Structure.IntroPattern, qualitative.IntroPattern)
	fill(&profile.Structure.ClosingPattern, qualitative.ClosingPattern)
	fill(&profile.Structure.HeadingHabit, qualitative.HeadingHabit)
	fill(&profile.Structure.ListHabit, qualitative.ListHabit)
	fill(&profile.Structure.EmojiUse, qualitative.EmojiUse)
	fillAxis := func(target **int, candidate *int) {
		if *target == nil {
			*target = candidate
		}
	}
	fillAxis(&profile.Axes.Involvement, qualitative.Axes.Involvement)
	fillAxis(&profile.Axes.Narrativity, qualitative.Axes.Narrativity)
	fillAxis(&profile.Axes.PersuasionOvertness, qualitative.Axes.PersuasionOvertness)
	fillAxis(&profile.Axes.Abstractness, qualitative.Axes.Abstractness)
	fillAxis(&profile.Axes.AddresseeFocus, qualitative.Axes.AddresseeFocus)
	fillAxis(&profile.Axes.Humor, qualitative.Axes.Humor)
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

// LearnFromFinalizedPost derives the voice from the owned post — the caller never
// nominates one — and requires the machine baseline to have been written under that same
// voice, so a reassigned post cannot turn voice A's generated text into evidence about B.
func (s *Service) LearnFromFinalizedPost(ctx context.Context, userID, postSlug string, requested llm.ModelRef) (LearningEvent, string, bool, error) {
	if s.posts == nil {
		return LearningEvent{}, "", false, fmt.Errorf("voice personalization is not configured")
	}
	if requested.ProviderID == "" || requested.ModelID == "" {
		return LearningEvent{}, "", false, ErrAnalyzeModelRequired
	}
	snapshot, err := s.posts.LearningSnapshot(ctx, userID, postSlug)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	active, err := s.learnableVoice(ctx, snapshot)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	model, err := s.resolveAnalyzeModel(ctx, userID, requested)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	voiceID := active.ID
	if err := s.retireStaleRules(ctx, userID, voiceID); err != nil {
		return LearningEvent{}, "", false, err
	}
	hash := learningInputHash(snapshot)
	existing, err := s.personalization.FindLearningEvent(ctx, userID, voiceID, postSlug, snapshot.BaselineRevision, hash)
	if err != nil {
		return LearningEvent{}, "", false, err
	}
	if existing != nil {
		jobID, resumeErr := s.resumeLearningEvent(ctx, existing, model)
		return *existing, jobID, true, resumeErr
	}
	event := LearningEvent{ID: s.newID(), UserID: userID, VoiceID: voiceID, PostSlug: postSlug, BaselineRevision: snapshot.BaselineRevision, InputHash: hash, BaselineJSON: snapshot.BaselineJSON, FinalJSON: snapshot.FinalJSON, ModelRef: model.String(), Status: "queued", CreatedAt: s.now(), ContentLanguage: snapshot.ContentLanguage, SourceLanguage: active.SourceLanguage}
	if err = s.personalization.InsertLearningEvent(ctx, event); err != nil {
		// The database uniqueness constraint is the final arbiter when two tabs
		// finalize the same immutable input concurrently.
		if raced, findErr := s.personalization.FindLearningEvent(ctx, userID, voiceID, postSlug, snapshot.BaselineRevision, hash); findErr == nil && raced != nil {
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

// learnableVoice is the finalization gate: post.voice_id == machine_baseline_voice_id and
// that voice is still active. It is authoritative even when a stale client still shows
// an eligible state.
func (s *Service) learnableVoice(ctx context.Context, snapshot FinalizationInput) (Voice, error) {
	if snapshot.VoiceID == "" {
		return Voice{}, ErrVoiceRequired
	}
	if snapshot.BaselineVoiceID != snapshot.VoiceID {
		return Voice{}, ErrBaselineVoiceMismatch
	}
	active, err := s.activeVoice(ctx, snapshot.UserID, snapshot.VoiceID)
	if err != nil {
		return Voice{}, err
	}
	if err = requireContentLanguageMatch(snapshot.ContentLanguage, snapshot.VoiceSourceLanguage, active.SourceLanguage); err != nil {
		return Voice{}, err
	}
	return active, nil
}

// resumeLearningEvent makes the same explicit learn/retry action the recovery
// boundary for an event whose durable job was failed by the restart sweep. Merely
// booting or reading the event never enqueues provider work.
func (s *Service) resumeLearningEvent(ctx context.Context, event *LearningEvent, model llm.ModelRef) (string, error) {
	if event.Status == "done" {
		return event.JobID, nil
	}
	// The event's frozen voice and both frozen language tags, never the post's current
	// assignment, are authoritative for every retry.
	if _, err := s.learningVoice(ctx, *event); err != nil {
		return "", err
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
	fmt.Fprintf(h, "%d\x00%s\x00%s\x00%s\x00%s", snapshot.BaselineRevision, snapshot.BaselineJSON, snapshot.FinalJSON, snapshot.ContentLanguage, snapshot.VoiceSourceLanguage)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) enqueueLearningEvent(ctx context.Context, event *LearningEvent, model llm.ModelRef) (string, error) {
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{Kind: LearnJobKind, UserID: event.UserID, VoiceID: event.VoiceID, PostSlug: event.PostSlug, Model: model.String(), Payload: event.ID})
	if err != nil {
		failure := normalizeFailure(err)
		_ = s.personalization.SetLearningEventStatus(ctx, event.UserID, event.ID, "retryable", &failure, nil)
		event.Status = "retryable"
		event.Error = ""
		event.Failure = &failure
		return "", err
	}
	event.JobID = jobID
	event.ModelRef = model.String()
	event.Status = "queued"
	event.Error = ""
	event.Failure = nil
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
	if _, err = s.learningVoice(ctx, *event); err != nil {
		return *event, "", err
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
	if _, err := s.learningVoice(ctx, *event); err != nil {
		if errors.Is(err, ErrContentLanguageMismatch) {
			return err
		}
		return s.failLearning(ctx, *event, voiceUnavailableError(err))
	}
	_ = s.personalization.SetLearningEventStatus(ctx, job.UserID, event.ID, "running", nil, nil)
	final, body, err := parseAuthoredContent(event.FinalJSON)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	_, baselineBody, err := parseAuthoredContent(event.BaselineJSON)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	sources, err := s.personalization.ListAuthoredSources(ctx, job.UserID, event.VoiceID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	sources = authoredSourcesForLanguage(sources, event.SourceLanguage)
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
	request := llm.Request{System: structuredAnalysisPromptForLanguage(event.SourceLanguage), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(corpus.String())}}}, Stage: llm.StageNameAnalyze}
	if info, ok := s.models.Resolve(ref); ok && info.StructuredOutput {
		request.JSONSchema = VoiceAnalysisSchemaForLanguage(event.SourceLanguage)
	}
	response, err := s.models.Complete(ctx, ref, request)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	var qualitative qualitativeJSON
	if err = json.Unmarshal([]byte(strings.TrimSpace(response.Text)), &qualitative); err != nil {
		return s.failLearning(ctx, *event, fmt.Errorf("typed voice analysis returned invalid JSON: %w", err))
	}
	profile := MeasuredProfileForLanguage(corpus.String(), event.SourceLanguage, s.now)
	unknown := func(v string) VoiceValue {
		if strings.TrimSpace(v) == "" {
			return VoiceValue{Unknown: true, Source: SourceUnknown}
		}
		return VoiceValue{Value: strings.TrimSpace(v), Source: SourceAnalyzed}
	}
	mergeQualitativeProfile(&profile, qualitative, event.SourceLanguage, unknown)
	if err = validateAxes(profile.Axes); err != nil {
		return s.failLearning(ctx, *event, err)
	}
	overrides, err := s.personalization.ListManualOverrides(ctx, job.UserID, event.VoiceID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	for _, override := range overrides {
		if err = applyOverride(&profile, override.Layer, override.Field, override.Value); err != nil {
			return s.failLearning(ctx, *event, err)
		}
	}
	excerpt := excerptAroundTarget(body, s.config.FewShotExcerptTargetChars, s.config.FewShotExcerptMaxChars)
	source := AuthoredSource{ID: s.newID(), UserID: job.UserID, VoiceID: event.VoiceID, PostSlug: event.PostSlug, LearningEventID: event.ID, Title: final.Title, Tags: final.Tags, Body: body, Excerpt: excerpt, CreatedAt: s.now(), SourceLanguage: event.SourceLanguage}
	profile.Rules, err = s.personalization.ListRules(ctx, job.UserID, event.VoiceID)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	rules, err := s.extractStyleRulesForLanguage(ctx, ref, AlignSentences(baselineBody, body), event.SourceLanguage)
	if err != nil {
		return s.failLearning(ctx, *event, err)
	}
	rules, err = s.classifyRuleRelationsForLanguage(ctx, ref, rules, profile.Rules, event.SourceLanguage)
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
	for _, axis := range a.AxisValues() {
		if axis.Value != nil && (*axis.Value < -3 || *axis.Value > 3) {
			return fmt.Errorf("voice axis %s is outside -3..3", axis.Key)
		}
	}
	return nil
}
func (s *Service) failLearning(ctx context.Context, event LearningEvent, cause error) error {
	failure := normalizeFailure(cause)
	_ = s.personalization.SetLearningEventStatus(ctx, event.UserID, event.ID, "retryable", &failure, nil)
	return cause
}
