package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type ruleComparisonSnapshot struct {
	Rule                 ContrastRule      `json:"rule"`
	Source               AuthoredSource    `json:"source"`
	Profile              StructuredProfile `json:"profile"`
	TargetLength         *int              `json:"target_length,omitempty"`
	EndingMaxConsecutive int               `json:"ending_max_consecutive"`
	SourceLanguage       Language          `json:"source_language"`
}

// StartRuleComparison derives the voice from the owned rule; the source must belong to that
// same voice, so a same-account source from another voice reads as not found.
func (s *Service) StartRuleComparison(ctx context.Context, userID, ruleID, sourceID string, targetLength *int, model llm.ModelRef) (string, string, error) {
	rule, err := s.personalization.GetRule(ctx, userID, ruleID)
	if err != nil {
		return "", "", err
	}
	voiceID := rule.VoiceID
	active, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return "", "", err
	}
	source, err := s.personalization.GetAuthoredSource(ctx, userID, voiceID, sourceID)
	if err != nil {
		return "", "", err
	}
	if err = requireSourceLanguageMatch(source, active); err != nil {
		return "", "", err
	}
	// The rule comparison runs the WRITE model (the wire field is write_model): membership
	// is checked against the write purpose, whatever the historical error name says.
	if !s.personalizationModels.ModelEnabled(model, llm.StageNameWrite) {
		return "", "", ErrAnalyzeModelRequired
	}
	if err := s.retireStaleRules(ctx, userID, voiceID); err != nil {
		return "", "", err
	}
	if rule, err = s.personalization.GetRule(ctx, userID, ruleID); err != nil {
		return "", "", err
	}
	if rule.Status != RuleCandidate || rule.EvidenceCount < 1 || rule.EvidenceCount > 2 {
		return "", "", ErrInvalidLifecycle
	}
	profile, err := s.Get(ctx, userID, voiceID)
	if err != nil {
		return "", "", err
	}
	if profile.Structured.Version == 0 {
		return "", "", ErrInvalidLifecycle
	}
	if targetLength != nil && *targetLength <= 0 {
		return "", "", ErrInvalidLifecycle
	}
	snapshot, err := json.Marshal(ruleComparisonSnapshot{Rule: rule, Source: source, Profile: profile.Structured, TargetLength: targetLength, EndingMaxConsecutive: s.config.EndingMaxConsecutive, SourceLanguage: active.SourceLanguage})
	if err != nil {
		return "", "", err
	}
	id := s.newID()
	side := "left"
	if id[0] >= '8' {
		side = "right"
	}
	comparison := RuleComparison{ID: id, UserID: userID, VoiceID: voiceID, RuleID: ruleID, SourceID: sourceID, ProfileVersion: profile.Structured.Version, ModelRef: model.String(), TargetLength: targetLength, InputSnapshot: string(snapshot), RuleOnSide: side, Status: "queued", CreatedAt: s.now(), SourceLanguage: active.SourceLanguage, Candidates: []ComparisonCandidate{{ID: s.newID(), ComparisonID: id, DisplaySide: "left", Status: "pending"}, {ID: s.newID(), ComparisonID: id, DisplaySide: "right", Status: "pending"}}}
	if err = s.personalization.InsertRuleComparison(ctx, comparison); err != nil {
		return "", "", err
	}
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{Kind: CompareRuleJobKind, UserID: userID, VoiceID: voiceID, PostSlug: source.PostSlug, Model: model.String(), Payload: id})
	if err != nil {
		comparison.Status = "failed"
		if updateErr := s.personalization.UpdateRuleComparison(ctx, comparison); updateErr != nil {
			return id, "", fmt.Errorf("enqueue comparison: %w; mark comparison failed: %v", err, updateErr)
		}
		return id, "", err
	}
	if err = s.personalization.SetRuleComparisonJob(ctx, userID, id, jobID); err != nil {
		cancelled, cancelErr := s.personalizationJobs.FailQueuedPersonalization(ctx, jobID, userID, Failure{Reason: FailureReasonUnknown})
		if cancelErr != nil {
			return id, jobID, fmt.Errorf("link comparison job: %w; cancel queued job: %v", err, cancelErr)
		}
		if !cancelled {
			// The worker already owns the durable job. Returning its IDs lets the
			// caller follow the aggregate while the handler finishes it safely.
			return id, jobID, nil
		}
		comparison.Status = "failed"
		if updateErr := s.personalization.UpdateRuleComparison(ctx, comparison); updateErr != nil {
			return id, jobID, fmt.Errorf("link comparison job: %w; mark comparison failed: %v", err, updateErr)
		}
		return id, jobID, err
	}
	return id, jobID, nil
}

func (s *Service) GetRuleComparison(ctx context.Context, userID, id string) (RuleComparison, error) {
	return s.personalization.GetRuleComparison(ctx, userID, id)
}

func (s *Service) comparisonSource(ctx context.Context, comparison RuleComparison, active Voice) (AuthoredSource, error) {
	if !comparison.SourceLanguage.Valid() || comparison.SourceLanguage != active.SourceLanguage {
		return AuthoredSource{}, contentLanguageMismatch(comparison.SourceLanguage, active.SourceLanguage)
	}
	source, err := s.personalization.GetAuthoredSource(ctx, comparison.UserID, comparison.VoiceID, comparison.SourceID)
	if err != nil {
		return AuthoredSource{}, err
	}
	if err = requireSourceLanguageMatch(source, active); err != nil {
		return AuthoredSource{}, err
	}
	if source.SourceLanguage != comparison.SourceLanguage {
		return AuthoredSource{}, contentLanguageMismatch(source.SourceLanguage, comparison.SourceLanguage)
	}
	return source, nil
}

func (s *Service) RetryRuleComparison(ctx context.Context, userID, id string) (string, error) {
	comparison, err := s.personalization.GetRuleComparison(ctx, userID, id)
	if err != nil {
		return "", err
	}
	if comparison.ChosenSide != "" || comparison.Status == "review" {
		return "", ErrInvalidLifecycle
	}
	active, err := s.activeVoice(ctx, userID, comparison.VoiceID)
	if err != nil {
		return "", err
	}
	source, err := s.comparisonSource(ctx, comparison, active)
	if err != nil {
		return "", err
	}
	for i := range comparison.Candidates {
		if comparison.Candidates[i].Status == "failed" {
			comparison.Candidates[i].Status = "pending"
			comparison.Candidates[i].Error = ""
			comparison.Candidates[i].Failure = nil
		}
	}
	comparison.Status = "queued"
	if err = s.personalization.UpdateRuleComparison(ctx, comparison); err != nil {
		return "", err
	}
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{Kind: CompareRuleJobKind, UserID: userID, VoiceID: comparison.VoiceID, PostSlug: source.PostSlug, Model: comparison.ModelRef, Payload: id})
	if err != nil {
		comparison.Status = "failed"
		_ = s.personalization.UpdateRuleComparison(ctx, comparison)
		return "", err
	}
	if err = s.personalization.SetRuleComparisonJob(ctx, userID, id, jobID); err != nil {
		cancelled, cancelErr := s.personalizationJobs.FailQueuedPersonalization(ctx, jobID, userID, Failure{Reason: FailureReasonUnknown})
		if cancelErr == nil && cancelled {
			comparison.Status = "failed"
			_ = s.personalization.UpdateRuleComparison(ctx, comparison)
			return jobID, err
		}
		if cancelErr != nil {
			return jobID, fmt.Errorf("link comparison retry: %w; cancel queued job: %v", err, cancelErr)
		}
		return jobID, nil
	}
	return jobID, nil
}

func BuildRuleComparisonPrompts(snapshot ruleComparisonSnapshot) (string, string) {
	if snapshot.SourceLanguage == LanguageEnglish {
		return buildEnglishRuleComparisonPrompts(snapshot)
	}
	endingMax := snapshot.EndingMaxConsecutive
	if endingMax <= 0 {
		endingMax = 2
	}
	base := fmt.Sprintf("한국어 블로그 글을 작성하세요. 같은 종결어미를 %d문장보다 많이 연속 쓰지 마세요.\n%s\n예시의 고유 표현이나 내용을 복사하지 마세요.\n", endingMax, renderStructuredProfile(snapshot.Profile))
	if snapshot.TargetLength != nil {
		base += fmt.Sprintf("목표 길이 약 %d자.\n", *snapshot.TargetLength)
	}
	base += "[Candidate rule]\n"
	topic := "[주제]\n" + snapshot.Source.Title + "\n태그: " + strings.Join(snapshot.Source.Tags, ", ")
	off := base + topic
	on := base + snapshot.Rule.Statement + "\n" + topic
	return off, on
}

func buildEnglishRuleComparisonPrompts(snapshot ruleComparisonSnapshot) (string, string) {
	base := "Write an English blog post. Preserve the measured register, contraction pattern, connective style, and sentence cadence.\n" + renderStructuredProfileForLanguage(snapshot.Profile, LanguageEnglish) + "\nDo not copy distinctive wording or subject matter from the example.\n"
	if snapshot.TargetLength != nil {
		base += fmt.Sprintf("Target approximately %d characters.\n", *snapshot.TargetLength)
	}
	base += "[Candidate rule]\n"
	topic := "[Topic]\n" + snapshot.Source.Title + "\nTags: " + strings.Join(snapshot.Source.Tags, ", ")
	return base + topic, base + snapshot.Rule.Statement + "\n" + topic
}

func (s *Service) CompareRule(ctx context.Context, userID, comparisonID, modelRef string, progress Progress) error {
	comparison, err := s.personalization.GetRuleComparison(ctx, userID, comparisonID)
	if err != nil {
		return err
	}
	active, err := s.activeVoice(ctx, userID, comparison.VoiceID)
	if err != nil {
		return voiceUnavailableError(err)
	}
	if _, err = s.comparisonSource(ctx, comparison, active); err != nil {
		return err
	}
	var snapshot ruleComparisonSnapshot
	if err = json.Unmarshal([]byte(comparison.InputSnapshot), &snapshot); err != nil {
		return err
	}
	if snapshot.SourceLanguage != comparison.SourceLanguage || snapshot.Source.SourceLanguage != comparison.SourceLanguage {
		return contentLanguageMismatch(snapshot.SourceLanguage, comparison.SourceLanguage)
	}
	model, err := parseModelRef(modelRef)
	if err != nil {
		return err
	}
	off, on := BuildRuleComparisonPrompts(snapshot)
	comparison.Status = "running"
	_ = s.personalization.UpdateRuleComparison(ctx, comparison)
	type result struct {
		i      int
		output string
		err    error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	done := 0
	for i := range comparison.Candidates {
		if comparison.Candidates[i].Status == "succeeded" {
			done++
			continue
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			prompt := off
			if comparison.Candidates[index].DisplaySide == comparison.RuleOnSide {
				prompt = on
			}
			userInstruction := "글 본문만 반환하세요."
			if comparison.SourceLanguage == LanguageEnglish {
				userInstruction = "Return only the article body."
			}
			// A rule comparison writes two post bodies, and the job carries the WRITE model
			// (main.go passes found.WriteModel), so this is the writing stage.
			response, e := s.models.Complete(ctx, model, llm.Request{System: prompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(userInstruction)}}}, Stage: llm.StageNameWrite})
			results <- result{i: index, output: response.Text, err: e}
		}(i)
	}
	go func() { wg.Wait(); close(results) }()
	for result := range results {
		candidate := &comparison.Candidates[result.i]
		if result.err != nil {
			candidate.Status = "failed"
			candidate.Error = ""
			failure := normalizeFailure(result.err)
			candidate.Failure = &failure
		} else {
			candidate.Status = "succeeded"
			candidate.Output = result.output
			candidate.Error = ""
			candidate.Failure = nil
			done++
		}
		progress("compare_rule", done, 2)
	}
	switch done {
	case 2:
		comparison.Status = "review"
	case 1:
		comparison.Status = "partial"
	default:
		comparison.Status = "failed"
	}
	return s.personalization.UpdateRuleComparison(ctx, comparison)
}

func (s *Service) DecideRuleComparison(ctx context.Context, userID, id, side string) (RuleComparison, error) {
	if side != "left" && side != "right" {
		return RuleComparison{}, fmt.Errorf("chosen side must be left or right")
	}
	comparison, err := s.personalization.GetRuleComparison(ctx, userID, id)
	if err != nil {
		return RuleComparison{}, err
	}
	if !comparisonReadyForDecision(comparison) {
		return RuleComparison{}, ErrInvalidLifecycle
	}
	active, err := s.activeVoice(ctx, userID, comparison.VoiceID)
	if err != nil {
		return RuleComparison{}, err
	}
	if _, err = s.comparisonSource(ctx, comparison, active); err != nil {
		return RuleComparison{}, err
	}
	now := s.now()
	comparison.ChosenSide = side
	comparison.DecidedAt = &now
	comparison.ActivationEvidence = s.config.RuleActivationEvidence
	profile, err := s.Get(ctx, userID, comparison.VoiceID)
	if err != nil {
		return RuleComparison{}, err
	}
	for i := range profile.Structured.Rules {
		if profile.Structured.Rules[i].ID != comparison.RuleID {
			continue
		}
		if side == comparison.RuleOnSide {
			profile.Structured.Rules[i].EvidenceCount++
			profile.Structured.Rules[i].LastEvidenceAt = now
			if profile.Structured.Rules[i].EvidenceCount >= s.config.RuleActivationEvidence {
				profile.Structured.Rules[i].Status = RuleActive
			}
		} else {
			profile.Structured.Rules[i].Status = RuleRejected
		}
	}
	comparison.ProfileAfterDecision = &profile.Structured
	if err = s.personalization.UpdateRuleComparison(ctx, comparison); err != nil {
		return RuleComparison{}, err
	}
	return s.personalization.GetRuleComparison(ctx, userID, id)
}

func comparisonReadyForDecision(comparison RuleComparison) bool {
	if comparison.Status != "review" || len(comparison.Candidates) != 2 {
		return false
	}
	for _, candidate := range comparison.Candidates {
		if candidate.Status != "succeeded" || strings.TrimSpace(candidate.Output) == "" {
			return false
		}
	}
	return true
}

// StartValidation takes an explicit active voice and counts only that voice's finalized
// sources toward the three it needs.
func (s *Service) StartValidation(ctx context.Context, userID, voiceID string, analyze, write llm.ModelRef, judge bool) (string, string, error) {
	if analyze.ProviderID == "" || analyze.ModelID == "" || write.ProviderID == "" || write.ModelID == "" {
		return "", "", ErrAnalyzeModelRequired
	}
	active, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return "", "", err
	}
	sources, err := s.personalization.ListAuthoredSources(ctx, userID, voiceID)
	if err != nil {
		return "", "", err
	}
	sources = authoredSourcesForLanguage(sources, active.SourceLanguage)
	if len(sources) < s.config.ValidationPostCount {
		return "", "", &InsufficientSourcesError{Minimum: s.config.ValidationPostCount}
	}
	if err := s.retireStaleRules(ctx, userID, voiceID); err != nil {
		return "", "", err
	}
	selected, err := s.resolveAnalyzeModel(ctx, userID, analyze)
	if err != nil {
		return "", "", err
	}
	if !s.personalizationModels.ModelEnabled(write, llm.StageNameWrite) {
		return "", "", ErrAnalyzeModelRequired
	}
	profile, err := s.Get(ctx, userID, voiceID)
	if err != nil {
		return "", "", err
	}
	id := s.newID()
	selectedSources := pickValidationSources(id, sources, s.config.ValidationPostCount)
	validation := ProfileValidation{ID: id, UserID: userID, VoiceID: voiceID, ProfileVersion: profile.Structured.Version, AnalyzeModelRef: selected.String(), WriteModelRef: write.String(), JudgeEnabled: judge, Status: "queued", CreatedAt: s.now(), SourceLanguage: active.SourceLanguage}
	for i, source := range selectedSources {
		validation.Items = append(validation.Items, ValidationItem{ID: s.newID(), ValidationID: id, SourceID: source.ID, Position: i, Original: source.Body, Status: "pending"})
	}
	if err = s.personalization.InsertProfileValidation(ctx, validation); err != nil {
		return "", "", err
	}
	// Each sampled post runs the write model once and the analyze model twice (a neutral
	// summary, then a judgement), so the hold must price the whole fan-out rather than one
	// call of each.
	items := len(validation.Items)
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{
		Kind: ValidateProfileJobKind, UserID: userID, VoiceID: voiceID, Model: write.String(),
		ExtraModels: []string{selected.String()}, Payload: id,
		CallCounts: map[string]int{
			write.String():    items,
			selected.String(): items * 2,
		},
	})
	if err != nil {
		validation.Status = "failed"
		now := s.now()
		validation.FinishedAt = &now
		if updateErr := s.personalization.UpdateProfileValidation(ctx, validation); updateErr != nil {
			return id, "", fmt.Errorf("enqueue validation: %w; mark validation failed: %v", err, updateErr)
		}
		return id, "", err
	}
	if err = s.personalization.SetProfileValidationJob(ctx, userID, id, jobID); err != nil {
		cancelled, cancelErr := s.personalizationJobs.FailQueuedPersonalization(ctx, jobID, userID, Failure{Reason: FailureReasonUnknown})
		if cancelErr != nil {
			return id, jobID, fmt.Errorf("link validation job: %w; cancel queued job: %v", err, cancelErr)
		}
		if !cancelled {
			return id, jobID, nil
		}
		validation.Status = "failed"
		now := s.now()
		validation.FinishedAt = &now
		if updateErr := s.personalization.UpdateProfileValidation(ctx, validation); updateErr != nil {
			return id, jobID, fmt.Errorf("link validation job: %w; mark validation failed: %v", err, updateErr)
		}
		return id, jobID, err
	}
	return id, jobID, nil
}

func pickValidationSources(seed string, sources []AuthoredSource, count int) []AuthoredSource {
	out := append([]AuthoredSource(nil), sources...)
	sort.Slice(out, func(i, j int) bool {
		a := sha256.Sum256([]byte(seed + out[i].ID))
		b := sha256.Sum256([]byte(seed + out[j].ID))
		return hex.EncodeToString(a[:]) < hex.EncodeToString(b[:])
	})
	return out[:min(count, len(out))]
}

func (s *Service) requireValidationSources(ctx context.Context, validation ProfileValidation, active Voice) error {
	if !validation.SourceLanguage.Valid() || validation.SourceLanguage != active.SourceLanguage {
		return contentLanguageMismatch(validation.SourceLanguage, active.SourceLanguage)
	}
	for _, item := range validation.Items {
		source, err := s.personalization.GetAuthoredSource(ctx, validation.UserID, validation.VoiceID, item.SourceID)
		if err != nil {
			return err
		}
		if err = requireSourceLanguageMatch(source, active); err != nil {
			return err
		}
		if source.SourceLanguage != validation.SourceLanguage {
			return contentLanguageMismatch(source.SourceLanguage, validation.SourceLanguage)
		}
	}
	return nil
}

func validationSummaryPrompt(language Language) string {
	if language == LanguageEnglish {
		return "Summarize only the topic and facts of the following article in one or two English sentences. Do not reuse or describe its wording, style, contractions, or sentence cadence."
	}
	return "다음 글의 주제와 사실만 한국어 1–2문장으로 요약하세요. 원문의 문구, 문체, 종결어미를 재사용하거나 설명하지 마세요."
}

func validationWritePrompt(language Language, profile StructuredProfile, endingMax int) string {
	if language == LanguageEnglish {
		return fmt.Sprintf("%s\nUsing only the neutral topic summary below, write a new English article that follows the profile's register, contractions, connectives, passive/nominal tendency, and sentence cadence.", renderStructuredProfileForLanguage(profile, LanguageEnglish))
	}
	return fmt.Sprintf("%s\n같은 종결어미를 %d문장보다 많이 연속 쓰지 말고 아래의 중립 주제 요약만으로 새 글을 쓰세요.", renderStructuredProfile(profile), endingMax)
}

func validationJudgePrompt(language Language) string {
	if language == LanguageEnglish {
		return "Ignore topic overlap and evaluate exactly five dimensions as true/false JSON only: endings (English register, contractions, and cadence), sentence_rhythm, opening_closing, vocabulary, addressee."
	}
	return "주제 일치는 무시하고 endings, sentence_rhythm, opening_closing, vocabulary, addressee 다섯 항목을 true/false JSON으로만 평가하세요."
}

func (s *Service) GetValidation(ctx context.Context, userID, id string) (ProfileValidation, error) {
	return s.personalization.GetProfileValidation(ctx, userID, id)
}
func (s *Service) ListValidations(ctx context.Context, userID, voiceID string) ([]ProfileValidation, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return nil, err
	}
	return s.personalization.ListProfileValidations(ctx, userID, voiceID)
}

func (s *Service) RetryValidation(ctx context.Context, userID, id string) (string, error) {
	validation, err := s.personalization.GetProfileValidation(ctx, userID, id)
	if err != nil {
		return "", err
	}
	if validation.Status == "done" {
		return "", ErrInvalidLifecycle
	}
	active, err := s.activeVoice(ctx, userID, validation.VoiceID)
	if err != nil {
		return "", err
	}
	if err = s.requireValidationSources(ctx, validation, active); err != nil {
		return "", err
	}
	for i := range validation.Items {
		if validation.Items[i].Status == "failed" {
			validation.Items[i].Status = "pending"
			validation.Items[i].Error = ""
			validation.Items[i].Failure = nil
		}
	}
	validation.Status = "queued"
	validation.FinishedAt = nil
	if err = s.personalization.UpdateProfileValidation(ctx, validation); err != nil {
		return "", err
	}
	// The retry runs the SAME two models the validation froze, so both go through the gate
	// again: a downgrade between the first run and the retry must refuse the locked one.
	jobID, err := s.personalizationJobs.EnqueuePersonalization(ctx, PersonalizationJobRequest{
		Kind: ValidateProfileJobKind, UserID: userID, VoiceID: validation.VoiceID, Model: validation.WriteModelRef,
		ExtraModels: []string{validation.AnalyzeModelRef}, Payload: id,
	})
	if err != nil {
		validation.Status = "failed"
		now := s.now()
		validation.FinishedAt = &now
		_ = s.personalization.UpdateProfileValidation(ctx, validation)
		return "", err
	}
	if err = s.personalization.SetProfileValidationJob(ctx, userID, id, jobID); err != nil {
		cancelled, cancelErr := s.personalizationJobs.FailQueuedPersonalization(ctx, jobID, userID, Failure{Reason: FailureReasonUnknown})
		if cancelErr == nil && cancelled {
			validation.Status = "failed"
			now := s.now()
			validation.FinishedAt = &now
			_ = s.personalization.UpdateProfileValidation(ctx, validation)
			return jobID, err
		}
		if cancelErr != nil {
			return jobID, fmt.Errorf("link validation retry: %w; cancel queued job: %v", err, cancelErr)
		}
		return jobID, nil
	}
	return jobID, nil
}

func (s *Service) ValidateProfile(ctx context.Context, userID, validationID string, progress Progress) error {
	validation, err := s.personalization.GetProfileValidation(ctx, userID, validationID)
	if err != nil {
		return err
	}
	active, err := s.activeVoice(ctx, userID, validation.VoiceID)
	if err != nil {
		return voiceUnavailableError(err)
	}
	if err = s.requireValidationSources(ctx, validation, active); err != nil {
		return err
	}
	version, err := s.personalization.GetProfileVersion(ctx, userID, validation.VoiceID, validation.ProfileVersion)
	if err != nil {
		return err
	}
	analyze, err := parseModelRef(validation.AnalyzeModelRef)
	if err != nil {
		return err
	}
	write, err := parseModelRef(validation.WriteModelRef)
	if err != nil {
		return err
	}
	validation.Status = "running"
	validation.FinishedAt = nil
	total := len(validation.Items)
	for i := range validation.Items {
		item := &validation.Items[i]
		if item.Status == "generated" || item.Status == "scored" {
			progress("validate_profile", i+1, total)
			continue
		}
		if item.NeutralSummary == "" {
			summaryResponse, e := s.models.Complete(ctx, analyze, llm.Request{System: validationSummaryPrompt(validation.SourceLanguage), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(item.Original)}}}, Stage: llm.StageNameAnalyze})
			if e != nil {
				setValidationItemFailure(item, e)
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			summary := strings.TrimSpace(summaryResponse.Text)
			summarySentences := SegmentSentences(summary)
			if len(summarySentences) < 1 || len(summarySentences) > 2 {
				setValidationItemFailure(item, fmt.Errorf("neutral summary must contain one or two sentences"))
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			if reusesWording(item.Original, summary) {
				setValidationItemFailure(item, fmt.Errorf("neutral summary reused source wording"))
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			item.NeutralSummary = summary
			item.Status = "summarized"
			clearValidationItemFailure(item)
			if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
				return persistErr
			}
		}
		if item.Regenerated == "" {
			writeResponse, e := s.models.Complete(ctx, write, llm.Request{System: validationWritePrompt(validation.SourceLanguage, version.Profile, s.config.EndingMaxConsecutive), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(item.NeutralSummary)}}}, Stage: llm.StageNameWrite})
			if e != nil {
				setValidationItemFailure(item, e)
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			item.Regenerated = strings.TrimSpace(writeResponse.Text)
			item.Status = "generated"
			clearValidationItemFailure(item)
			if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
				return persistErr
			}
		}
		if validation.JudgeEnabled && item.ScoresJSON == "" {
			judgeResponse, e := s.models.Complete(ctx, analyze, llm.Request{System: validationJudgePrompt(validation.SourceLanguage), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("original:\n" + item.Original + "\nregenerated:\n" + item.Regenerated)}}}, Stage: llm.StageNameAnalyze})
			if e != nil {
				setValidationItemFailure(item, e)
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			scores := map[string]bool{}
			if e = json.Unmarshal([]byte(judgeResponse.Text), &scores); e != nil || !validJudgeScores(scores) {
				setValidationItemFailure(item, fmt.Errorf("judge returned invalid five-dimension JSON"))
				if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
					return persistErr
				}
				continue
			}
			encoded, _ := json.Marshal(scores)
			item.ScoresJSON = string(encoded)
			for _, key := range []string{"endings", "sentence_rhythm", "opening_closing", "vocabulary", "addressee"} {
				validation.TotalCount++
				if scores[key] {
					validation.YCount++
				}
			}
			item.Status = "scored"
			clearValidationItemFailure(item)
			if persistErr := s.personalization.UpdateProfileValidation(ctx, validation); persistErr != nil {
				return persistErr
			}
		}
		progress("validate_profile", i+1, total)
	}
	succeeded := 0
	for _, item := range validation.Items {
		if item.Status == "generated" || item.Status == "scored" {
			succeeded++
		}
	}
	now := s.now()
	validation.FinishedAt = &now
	if succeeded == total {
		validation.Status = "done"
	} else if succeeded > 0 {
		validation.Status = "partial"
	} else {
		validation.Status = "failed"
	}
	return s.personalization.UpdateProfileValidation(ctx, validation)
}

func setValidationItemFailure(item *ValidationItem, cause error) {
	failure := normalizeFailure(cause)
	item.Status = "failed"
	item.Error = ""
	item.Failure = &failure
}

func clearValidationItemFailure(item *ValidationItem) {
	item.Error = ""
	item.Failure = nil
}

func reusesWording(source, summary string) bool {
	runes := []rune(strings.TrimSpace(summary))
	if len(runes) < 8 {
		return false
	}
	for i := 0; i+8 <= len(runes); i++ {
		fragment := string(runes[i : i+8])
		if utf8.RuneCountInString(strings.TrimSpace(fragment)) == 8 && strings.Contains(source, fragment) {
			return true
		}
	}
	return false
}

func validJudgeScores(scores map[string]bool) bool {
	if len(scores) != 5 {
		return false
	}
	for _, key := range []string{"endings", "sentence_rhythm", "opening_closing", "vocabulary", "addressee"} {
		if _, ok := scores[key]; !ok {
			return false
		}
	}
	return true
}
