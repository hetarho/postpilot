package voice

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

var validFeedbackReasons = map[string]RuleLayer{"vocabulary": LayerLexical, "ending": LayerEndings, "length": LayerSyntax, "structure": LayerStructure}

func canonicalRule(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, value)
}

func (s *Service) GiveFeedback(ctx context.Context, userID, postSlug, sentenceRef, reason, authoredText string, satisfaction bool) (string, error) {
	if s.posts == nil {
		return "", fmt.Errorf("voice personalization is not configured")
	}
	snapshot, err := s.posts.LearningSnapshot(ctx, userID, postSlug)
	if err != nil {
		return "", err
	}
	event, err := s.personalization.FindLearningEvent(ctx, userID, postSlug, snapshot.BaselineRevision, learningInputHash(snapshot))
	if err != nil {
		return "", err
	}
	if event == nil || event.Status != "done" {
		return "", fmt.Errorf("%w: feedback requires a completed finalization", ErrInvalidLifecycle)
	}
	_, finalBody, err := parseAuthoredContent(snapshot.FinalJSON)
	if err != nil {
		return "", err
	}
	kind := "thumbs"
	state := "ignored"
	if satisfaction {
		if snapshot.BaselineJSON != snapshot.FinalJSON {
			return "", fmt.Errorf("satisfaction is available only when the finalized text was unchanged")
		}
		kind = "satisfaction"
		reason = ""
	} else {
		if _, ok := validFeedbackReasons[reason]; !ok {
			return "", fmt.Errorf("feedback reason must be vocabulary, ending, length, or structure")
		}
		if strings.TrimSpace(sentenceRef) == "" || strings.TrimSpace(authoredText) == "" {
			return "", fmt.Errorf("sentence feedback requires an authored sentence")
		}
		if !strings.Contains(finalBody, strings.TrimSpace(authoredText)) {
			return "", fmt.Errorf("sentence feedback must reference the owned finalized text")
		}
	}
	feedback := Feedback{ID: s.newID(), UserID: userID, PostSlug: postSlug, SentenceRef: sentenceRef, Kind: kind, Reason: reason, PayloadRef: authoredText, ProcessingState: state, CreatedAt: s.now()}
	if existing, lookupErr := s.findFeedback(ctx, feedback); lookupErr != nil {
		return "", lookupErr
	} else if existing != nil {
		return existing.ID, nil
	}
	if err = s.personalization.InsertFeedback(ctx, feedback); err == nil {
		return feedback.ID, nil
	}
	// The unique constraint arbitrates simultaneous retries from multiple tabs.
	if existing, lookupErr := s.findFeedback(ctx, feedback); lookupErr == nil && existing != nil {
		return existing.ID, nil
	}
	return feedback.ID, err
}

func (s *Service) findFeedback(ctx context.Context, want Feedback) (*Feedback, error) {
	items, err := s.personalization.ListFeedback(ctx, want.UserID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		item := &items[i]
		if item.PostSlug == want.PostSlug && item.SentenceRef == want.SentenceRef && item.Kind == want.Kind && item.PayloadRef == want.PayloadRef {
			return item, nil
		}
	}
	return nil, nil
}

func (s *Service) ChangeRuleStatus(ctx context.Context, userID, ruleID string, status RuleStatus) (Profile, error) {
	if status != RuleCandidate && status != RuleActive && status != RuleRetired && status != RuleRejected {
		return Profile{}, ErrInvalidLifecycle
	}
	if err := s.retireStaleRules(ctx, userID); err != nil {
		return Profile{}, err
	}
	if _, err := s.personalization.GetRule(ctx, userID, ruleID); err != nil {
		return Profile{}, err
	}
	profile, err := s.Get(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	now := s.now()
	for i := range profile.Structured.Rules {
		if profile.Structured.Rules[i].ID == ruleID {
			profile.Structured.Rules[i].Status = status
			profile.Structured.Rules[i].LastEvidenceAt = now
		}
	}
	if err = s.personalization.ApplyRuleStatusAndPublish(ctx, userID, ruleID, status, profile.Structured, now); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}
func (s *Service) Confirmations(ctx context.Context, userID string) ([]RuleConfirmation, error) {
	return s.personalization.ListConfirmations(ctx, userID)
}
func (s *Service) ResolveConfirmation(ctx context.Context, userID, confirmationID string, replace bool) (Profile, error) {
	if err := s.retireStaleRules(ctx, userID); err != nil {
		return Profile{}, err
	}
	if err := s.personalization.ResolveConfirmationAndPublish(ctx, userID, confirmationID, replace, s.now()); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}
