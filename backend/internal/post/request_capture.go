package post

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// RequestCaptureRun freezes the source identity independently of revision changes
// caused by this run's own observations and plan publication.
type RequestCaptureRun struct {
	JobID, UserID, PostSlug        string
	InputRevision, ContentRevision int64
	SourceFingerprint              string
	PlanFingerprint                string
}

type RequestCaptureCall struct {
	ID              string
	Sequence        int
	AttachmentID    string
	AttachmentIDs   []string
	AttachmentKinds []string
}

// RequestCaptureCompletion binds safe call payload to exactly one current result.
// A nil completion leaves issued failed/interrupted calls source-scoped.
type RequestCaptureCompletion struct {
	Result            *OriginResultIdentity
	PlanFingerprint   *string
	UnavailableStages []string
	UnavailableReason string
}

// RequestCaptureSourceFingerprint hashes only owner inputs and immutable media
// incarnations. It deliberately excludes observations, generated plans, content,
// signed links, storage keys and automatic rotation. Call/result fences cover those
// independently; this hash is not a history or a customer-visible identifier.
func RequestCaptureSourceFingerprint(value Post) string {
	var source strings.Builder
	write := func(v string) { fmt.Fprintf(&source, "%d:%s", len(v), v) }
	for _, v := range []string{value.Title, value.Memo, value.VoiceID, value.TemplateID, string(value.TargetLanguage), value.Field} {
		write(v)
	}
	write(fmt.Sprint(value.TagCount))
	write(fmt.Sprint(value.UseMemory))
	if value.TargetLength == nil {
		write("")
	} else {
		write(fmt.Sprint(*value.TargetLength))
	}
	write(fmt.Sprint(len(value.QualityRules)))
	for _, v := range value.QualityRules {
		write(v)
	}
	answers := slices.Clone(value.TemplateAnswers)
	sort.Slice(answers, func(i, j int) bool { return answers[i].Label < answers[j].Label })
	write(fmt.Sprint(len(answers)))
	for _, v := range answers {
		write(v.Label)
		write(v.Text)
		write(fmt.Sprint(v.Enabled))
	}
	type attachment struct{ id, filename, kind, ownerRotation string }
	media := make([]attachment, 0, len(value.Images)+len(value.Videos))
	for _, v := range value.Images {
		rotation := ""
		if v.RotationByOwner {
			rotation = fmt.Sprint(v.Rotation)
		}
		media = append(media, attachment{v.ID, v.Filename, string(AttachmentPhoto), rotation})
	}
	for _, v := range value.Videos {
		media = append(media, attachment{v.ID, v.Filename, string(AttachmentVideo), ""})
	}
	sort.Slice(media, func(i, j int) bool { return media[i].id < media[j].id })
	write(fmt.Sprint(len(media)))
	for _, v := range media {
		write(v.id)
		write(v.filename)
		write(v.kind)
		write(v.ownerRotation)
	}
	sum := sha256.Sum256([]byte(source.String()))
	return hex.EncodeToString(sum[:])
}
