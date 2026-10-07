package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrTestStylePublicationConflict = errors.New("writing test style publication conflicts")

// AcceptedAnalysisRevision identifies only the immutable accepted expression/source snapshot.
// Directory renames and still-pending edits are not part of an accepted profile's identity.
func AcceptedAnalysisRevision(value Analysis) string {
	value.Origin = NormalizedOrigin(value.Origin)
	value.CreatedAt = value.CreatedAt.UTC()
	if len(value.MaterialIDs) == 0 {
		value.MaterialIDs = []string{}
	}
	if len(value.AcceptedSources) == 0 {
		value.AcceptedSources = []AcceptedSource{}
	}
	if len(value.AcceptedMaterials) == 0 {
		value.AcceptedMaterials = []AcceptedMaterial{}
	} else {
		value.AcceptedMaterials = append([]AcceptedMaterial(nil), value.AcceptedMaterials...)
		for i := range value.AcceptedMaterials {
			value.AcceptedMaterials[i].CreatedAt = value.AcceptedMaterials[i].CreatedAt.UTC()
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "accepted:" + hex.EncodeToString(sum[:])
}

// PrepareWritingStyle validates a fictional draft without a provider call or canonical write.
func (a *Authoring) PrepareWritingStyle(value WritingStyleDraft, model string, at time.Time) (Analysis, error) {
	return BuildSyntheticAnalysis(value, model, at.UTC())
}

type TestStyleStore interface {
	ReadTestPublicationReceipt(context.Context, string, string, string, string) (TestStyleReceipt, bool, error)
	PublishTestStyle(context.Context, TestStylePublication, Voice) (TestStyleReceipt, error)
}
type TestStylePublisher struct {
	store TestStyleStore
	now   func() time.Time
	newID func() string
}

func NewTestStylePublisher(store TestStyleStore) *TestStylePublisher {
	if store == nil {
		panic("voice: test style publisher requires its atomic store")
	}
	return &TestStylePublisher{store: store, now: time.Now, newID: newID}
}
func (p *TestStylePublisher) ValidateTestPublicationChoices(_ context.Context, in TestStylePublication) error {
	if in.Action == "save_setting" {
		_, err := normalizeVoiceName(in.Name)
		return err
	}
	if in.Action != "use_setting" {
		return ErrTestStylePublicationConflict
	}
	return nil
}
func ValidTestStylePublication(in TestStylePublication) error {
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.TestID) == "" || strings.TrimSpace(in.WinnerID) == "" || strings.TrimSpace(in.RequestKey) == "" || strings.TrimSpace(in.Fingerprint) == "" || (in.Action != "save_setting" && in.Action != "use_setting") {
		return ErrTestStylePublicationConflict
	}
	origin := NormalizedOrigin(in.Analysis.Origin)
	if origin == OriginPersonal {
		if in.Action != "use_setting" || in.SourceVoiceID == "" || in.AcceptedRevision == "" || AcceptedAnalysisRevision(in.Analysis) != in.AcceptedRevision {
			return ErrTestStylePublicationConflict
		}
		return nil
	}
	if origin != OriginSynthetic || len(in.Analysis.MaterialIDs) != 0 || len(in.Analysis.AcceptedSources) != 0 || len(in.Analysis.AcceptedMaterials) != 0 {
		return ErrTestStylePublicationConflict
	}
	for _, example := range in.Analysis.AI.Examples {
		if example.MaterialID != "" {
			return ErrTestStylePublicationConflict
		}
	}
	if !validKoreanCandidateText(in.Analysis.AI.Impression, 1, CandidateDescriptionMaxChars) || !validKoreanCandidateText(in.Analysis.SyntheticSample, CandidateSampleMinChars, CandidateSampleMaxChars) || !containsKoreanProse(ProseSentences(in.Analysis.SyntheticSample)) {
		return ErrTestStylePublicationConflict
	}
	if in.Action == "save_setting" {
		if strings.TrimSpace(in.Name) == "" || utf8.RuneCountInString(strings.TrimSpace(in.Name)) > VoiceNameMaxChars {
			return ErrTestStylePublicationConflict
		}
	} else if in.SourceVoiceID == "" || in.AcceptedRevision == "" || AcceptedAnalysisRevision(in.Analysis) != in.AcceptedRevision {
		return ErrTestStylePublicationConflict
	}
	return nil
}
func (p *TestStylePublisher) PublishTestWinner(ctx context.Context, in TestStylePublication) (TestStyleReceipt, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := ValidTestStylePublication(in); err != nil {
		return TestStyleReceipt{}, err
	}
	now := p.now().UTC()
	return p.store.PublishTestStyle(ctx, in, Voice{ID: p.newID(), UserID: in.UserID, Name: in.Name, CreatedAt: now, UpdatedAt: now})
}

// The request key is a retry handle. The actual owner/test/winner/action operation is stable
// even when a caller loses that key; changed defaults/names/profile data remain a conflict.
func TestStylePublicationFingerprint(in TestStylePublication) (string, error) {
	in.RequestKey = ""
	in.Name = strings.TrimSpace(in.Name)
	if in.Action == "use_setting" {
		in.Name = ""
	}
	identity := struct {
		Request  TestStylePublication
		Accepted string
	}{Request: in, Accepted: AcceptedAnalysisRevision(in.Analysis)}
	identity.Request.Analysis = Analysis{}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

var _ StyleFactory = (*Authoring)(nil)
var _ TestedStyles = (*TestStylePublisher)(nil)

// ReadTestPublicationReceipt reads proof of a committed action without private payload or live target gates.
func (p *TestStylePublisher) ReadTestPublicationReceipt(ctx context.Context, userID, testID, winnerID string, action string) (TestStyleReceipt, bool, error) {
	return p.store.ReadTestPublicationReceipt(ctx, userID, testID, winnerID, action)
}
