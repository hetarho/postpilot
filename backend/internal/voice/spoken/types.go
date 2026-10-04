// Package spoken owns private sound identities separately from writing profiles.
package spoken

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

const NameMax = 60
const PlaybackTTL = time.Minute

// Publication expires before the cleanup grace period, including storage I/O.
const AudioPublicationTimeout = 2 * time.Minute
const AudioPrefix = "private/spoken/"

var (
	ErrNotFound         = errors.New("spoken voice not found")
	ErrInvalid          = errors.New("invalid spoken voice input")
	ErrConflict         = errors.New("spoken voice revision changed")
	ErrImmutable        = errors.New("confirmed sound identity is immutable")
	ErrAuditionRequired = errors.New("selected candidate must be auditioned")
	ErrMediaUnavailable = errors.New("spoken sample unavailable")
)

// Profile is an immutable, price-free snapshot acquired through a catalog port.
// Supplier sound handles never enter the customer projection.
type Profile struct {
	ConnectionScope                       string
	ID                                    string
	Revision                              int64
	Design, Synthesis                     llm.ModelRef
	DesignLabel, SpeechLabel, Grade       string
	Settings                              llm.SpeechSettings
	DescriptionMax, PreviewMax, SpeechMax int
	OutputFormat                          string
}

type DraftInput struct {
	Name, Description, PreviewText string
	ProfileID                      string
	ProfileRevision                int64
	QualificationSessionID         string
}

type Draft struct {
	ID, OwnerID                                         string
	Revision                                            int64
	Name, Description, PreviewText                      string
	Profile                                             Profile
	QualificationSessionID                              string
	GenerationID, SelectedCandidateID, ConfirmedVoiceID string
	CreatedAt, UpdatedAt                                time.Time
	Candidates                                          []Candidate
}

// Phase derives from persisted identities, never a separately mutable UI step.
func (d Draft) Phase() string {
	if d.ConfirmedVoiceID != "" {
		return "confirmed"
	}
	if d.SelectedCandidateID != "" {
		return "selected"
	}
	if len(d.Candidates) > 0 {
		return "candidates"
	}
	return "editing"
}

type Candidate struct {
	ID, OwnerID, DraftID, GenerationID string
	Handle                             llm.CandidateHandle
	AssetID                            string
	DurationMS                         int64
	Ordinal                            int
	AuditionedAt                       *time.Time
}

type Voice struct {
	ID, OwnerID                    string
	Revision                       int64
	Name, Description, PreviewText string
	Profile                        Profile
	Handle                         llm.VoiceHandle
	SampleAssetID                  string
	SampleDurationMS               int64
	CreatedAt                      time.Time
	RemovedAt                      *time.Time
}

type Asset struct {
	ID, OwnerID, ObjectKey, SHA256, Format, ProvenanceDigest string
	Bytes, Samples                                           int64
	SampleRate, Channels                                     int
	CreatedAt                                                time.Time
	RevokedAt                                                *time.Time
}

func (a Asset) DurationMS() int64 {
	if a.SampleRate <= 0 {
		return 0
	}
	return a.Samples * 1000 / int64(a.SampleRate)
}

type Playback struct {
	ID, OwnerID, AssetID string
	ExpiresAt            time.Time
	ServedAt             *time.Time
}

type Cleanup struct {
	ID, ObjectKey string
	CreatedAt     time.Time
}

type RequestIdentity struct{ OwnerID, Operation, Key, Digest string }
type MutationResult struct {
	ID       string
	Revision int64
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > NameMax {
		return "", ErrInvalid
	}
	return name, nil
}

func validateDraft(in DraftInput, p Profile) error {
	if _, err := validateName(in.Name); err != nil {
		return err
	}
	if p.ID != in.ProfileID || p.Revision != in.ProfileRevision || p.Revision <= 0 || p.Design.ProviderID == "" || p.Design.ProviderID != p.Synthesis.ProviderID || p.Design.ModelID == "" || p.Synthesis.ModelID == "" ||
		p.OutputFormat != llm.SpeechOutputFormat || p.DescriptionMax < llm.SpeechDescriptionMin || p.DescriptionMax > llm.SpeechDescriptionMax || p.PreviewMax < llm.SpeechPreviewMin || p.PreviewMax > llm.SpeechPreviewMax || p.SpeechMax < 1 || p.SpeechMax > llm.SpeechMaxText || p.Settings.Validate() != nil {
		return ErrInvalid
	}
	if !utf8.ValidString(in.Description) || strings.TrimSpace(in.Description) == "" || utf8.RuneCountInString(in.Description) < llm.SpeechDescriptionMin || utf8.RuneCountInString(in.Description) > p.DescriptionMax ||
		!utf8.ValidString(in.PreviewText) || strings.TrimSpace(in.PreviewText) == "" || utf8.RuneCountInString(in.PreviewText) < llm.SpeechPreviewMin || utf8.RuneCountInString(in.PreviewText) > p.PreviewMax {
		return ErrInvalid
	}
	return nil
}
