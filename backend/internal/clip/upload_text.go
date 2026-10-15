package clip

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/campaign"
	"github.com/postpilot/backend/internal/llm"
)

// Current upload rules, verified 2026-10-16 from the platforms themselves:
// Naver video (photo/post has a different limit):
// https://help.naver.com/service/30048/contents/24422?osType=COMMONOS
// Instagram caption: https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/media/
// Instagram's app takes five tags, unlike the API's older thirty-tag allowance:
// https://help.instagram.com/351460621611097/
const (
	UploadTextNaverMaxChars     = 300
	UploadTextInstagramMaxChars = 2200
	UploadTextMaxTags           = 5
)

var (
	ErrUploadTextConfirmation = errors.New("edited upload text needs confirmation")
	ErrUploadTextLimit        = errors.New("upload text exceeds the channel limits")
)

type UploadTextLimits struct {
	MaxChars, MaxTags int
	TagsInCaption     bool
}

func UploadLimits(channel string) (UploadTextLimits, error) {
	switch channel {
	case ChannelNaver:
		return UploadTextLimits{MaxChars: UploadTextNaverMaxChars, MaxTags: UploadTextMaxTags}, nil
	case ChannelInstagram:
		return UploadTextLimits{MaxChars: UploadTextInstagramMaxChars, MaxTags: UploadTextMaxTags, TagsInCaption: true}, nil
	}
	return UploadTextLimits{}, ErrInvalid
}

type UploadText struct {
	ProjectID, UserID, Channel, Body string
	LastJobID                        string
	Tags                             []string
	Edited                           bool
	Revision                         int
	UpdatedAt                        time.Time
}

type UploadTextInput struct {
	Language, Channel, Title, Instruction string
	Campaign                              bool
	Facts                                 CompositionInputs
	Storyline                             *Storyline
	Captions                              []string
	AdvertiserGuide                       *AdvertiserGuide `json:",omitempty"`
	Voice                                 string           `json:",omitempty"`
	Guidelines                            VideoGuidelines
	Policy                                llm.CallPolicy
}

type UploadTextWriter interface {
	ValidateUploadText(context.Context, llm.ModelRef, UploadTextInput) error
	WriteUploadText(context.Context, llm.ModelRef, UploadTextInput) (UploadText, llm.Usage, error)
}

// Only the default voice's style projection crosses this port, never its samples.
type UploadTextVoice interface {
	DefaultClipVoice(context.Context, string, string) (string, error)
}

type UploadTextPricing interface {
	FreezeUploadText(context.Context, llm.ModelRef) (GenerationPricing, error)
}

type UploadTextSave struct {
	UserID, ProjectID, Channel, Body, JobID string
	Tags                                    []string
	ExpectedRevision, ExpectedPlanRevision  int
	Edited                                  bool
	Now                                     time.Time
}

type UploadTextQuote struct {
	GenerationQuote
	Channel          string
	ExpectedRevision int
}

type UploadTextSnapshot struct {
	Version, PlanRevision, TextRevision int
	UserID, ProjectID, MaterialDigest   string
	ConfirmEdited                       bool
	Input                               UploadTextInput
	Approval                            *GenerationApproval
}

type UploadTextFinisher interface {
	CompleteUploadText(context.Context, UploadTextSave) (UploadText, error)
}

// Upload-text approvals have no source-batch dependency: finalized clips need no originals.
type UploadTextStore interface {
	ListUploadTexts(context.Context, string, string) ([]UploadText, error)
	SaveUploadText(context.Context, UploadTextSave) (UploadText, error)
	SaveUploadTextQuote(context.Context, UploadTextQuote, time.Time) error
	GetUploadTextQuote(context.Context, string, string) (UploadTextQuote, error)
	LinkUploadTextJob(context.Context, UploadTextQuote, string, UploadTextSnapshot, time.Time) error
}

// Material contains owner text only, with no original-file or observation dependency.
func UploadTextMaterial(p Project, channel string) (UploadTextInput, error) {
	if _, err := UploadLimits(channel); err != nil {
		return UploadTextInput{}, err
	}
	if p.EditPlanRevision <= 0 || p.EditPlan == "" {
		return UploadTextInput{}, ErrPlanConflict
	}
	plan, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return UploadTextInput{}, err
	}
	in := UploadTextInput{Language: p.Language, Channel: channel, Title: p.Title,
		Instruction: p.Instruction, Campaign: p.Campaign, Storyline: p.Storyline,
		AdvertiserGuide: EffectiveAdvertiserGuide(p), Captions: []string{}}
	if p.Composition != nil {
		in.Facts = p.Composition.Inputs
		in.Facts.Associations = nil
	}
	if plan.Portable != nil {
		for _, text := range plan.Portable.Elements {
			if strings.TrimSpace(text.Resolved.Text) != "" {
				in.Captions = append(in.Captions, text.Resolved.Text)
			}
		}
	} else {
		for _, cut := range plan.Cuts {
			for _, text := range cut.Placed() {
				in.Captions = append(in.Captions, text.Text)
			}
		}
	}
	return in, nil
}

func UploadMaterialDigest(in UploadTextInput) string {
	in.Voice, in.Guidelines, in.Policy = "", VideoGuidelines{}, llm.CallPolicy{}
	raw, _ := json.Marshal(in)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func UploadQuoteDigest(in UploadTextInput, revision int, confirmed bool, pricing GenerationPricing) string {
	raw, _ := json.Marshal(struct {
		Input     UploadTextInput
		Revision  int
		Confirmed bool
		Pricing   GenerationPricing
	}{in, revision, confirmed, pricing})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func UploadTextDisclosure(language string) string {
	if language == "en" {
		return "disclosure_banner_or_text_disclosure"
	}
	return "대가표시_배너_또는_문구_대가표시"
}

func normalizedUploadTags(values []string) ([]string, error) {
	tags := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		tag := strings.Join(strings.Fields(strings.TrimLeft(strings.TrimSpace(raw), "#")), "")
		if tag == "" || utf8.RuneCountInString(tag) > campaign.ItemMaxChars {
			return nil, ErrUploadTextLimit
		}
		key := strings.ToLower(tag)
		if !seen[key] {
			seen[key] = true
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func UploadTextBodyLimit(channel, language string, on bool, tags []string) (int, error) {
	limits, err := UploadLimits(channel)
	if err != nil {
		return 0, err
	}
	room := limits.MaxChars
	if on {
		room -= utf8.RuneCountInString(UploadTextDisclosure(language)) + 2
	}
	if limits.TagsInCaption && len(tags) > 0 {
		// The body and separately copyable tags must also fit together in a caption.
		room -= 2 + utf8.RuneCountInString("#"+strings.Join(tags, " #"))
	}
	if room < 1 || len(tags) > limits.MaxTags {
		return 0, ErrUploadTextLimit
	}
	return room, nil
}

func (in UploadTextInput) EffectiveGuide() *AdvertiserGuide {
	if !in.Campaign || in.AdvertiserGuide == nil || in.AdvertiserGuide.Empty() {
		return nil
	}
	return in.AdvertiserGuide
}

func RequiredUploadTags(guide *AdvertiserGuide) ([]string, error) {
	if guide == nil {
		return []string{}, nil
	}
	tags, err := normalizedUploadTags(guide.Hashtags)
	if err != nil || len(tags) > UploadTextMaxTags {
		return nil, ErrUploadTextLimit
	}
	return tags, nil
}

// Model text is bounded mechanically. Required tags are kept before optional ones;
// an impossible required set is refused before work, rather than silently discarded.
func BoundGeneratedUploadText(in UploadTextInput, body string, suggested []string) (UploadText, error) {
	tags, err := RequiredUploadTags(in.EffectiveGuide())
	if err != nil {
		return UploadText{}, err
	}
	for _, raw := range suggested {
		if len(tags) >= UploadTextMaxTags {
			break
		}
		candidate, e := normalizedUploadTags(append(append([]string{}, tags...), raw))
		if e == nil {
			tags = candidate
		}
	}
	room, err := UploadTextBodyLimit(in.Channel, in.Language, in.Campaign, tags)
	if err != nil {
		return UploadText{}, err
	}
	text := []rune(strings.TrimSpace(body))
	if len(text) > room {
		text = text[:room]
	}
	if strings.TrimSpace(string(text)) == "" {
		return UploadText{}, llm.ErrBadOutput
	}
	return UploadText{Channel: in.Channel, Body: strings.TrimSpace(string(text)), Tags: tags}, nil
}

// Owner edits are refused at the bound; only generated output is trimmed.
func ValidateUploadTextEdit(channel, language string, on bool, body string, rawTags []string) (UploadText, error) {
	tags, err := normalizedUploadTags(rawTags)
	if err != nil {
		return UploadText{}, err
	}
	room, err := UploadTextBodyLimit(channel, language, on, tags)
	body = strings.TrimSpace(body)
	if err != nil || body == "" || utf8.RuneCountInString(body) > room {
		return UploadText{}, ErrUploadTextLimit
	}
	return UploadText{Channel: channel, Body: body, Tags: tags, Edited: true}, nil
}
