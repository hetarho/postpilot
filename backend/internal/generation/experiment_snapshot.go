package generation

import (
	"encoding/json"
	"fmt"
)

// The experiment snapshots' wire shape lives here and only here (ARCH-7): a write-experiment
// snapshot is hashed as stored, so its keys, their order and their omitempty choices are the
// experiment's identity. The domain types carry no tags; every member below names the key it
// has always had — the Go field name the snapshot used to marshal PostInput by — and a new
// domain member reaches the bytes only by being mapped here
// (TestEveryWriteSnapshotMemberRoundTrips).
//
// Every slice and pointer maps nil to nil and empty to empty: `null` and `[]` are different
// bytes, and so a different hash.

// writeSnapshot is the domain view of a frozen write-experiment input.
type writeSnapshot struct {
	Prepared       bool
	TargetLanguage Language
	ObserveModel   string
	// ObserveFiles is the frozen re-observation set, with the same presence contract as the
	// generate payload's: absent is a snapshot taken before the picker existed and observes
	// every attached photo, present-and-empty observes nothing.
	ObserveFiles *[]string
	Post         PostInput
	Profile      Profile
	// Observations is pre-seeded at snapshot time with what the run carries over, and
	// PrepareWriteInput merges what it observes into it. Both candidates then read one
	// complete set, which is what makes the frozen input the experiment's identity.
	Observations []Observation
	// SnapshotOnly keeps the preparing observation in this snapshot instead of writing it
	// onto the post (MODEL-66, GEN-19). Absent is false, which persists: that is what every
	// snapshot frozen before the field meant, so a job that survives a deploy behaves as it
	// was started.
	SnapshotOnly bool
}

type experimentSnapshot struct {
	Kind           string                `json:"kind"`
	Prepared       bool                  `json:"prepared"`
	TargetLanguage string                `json:"target_language,omitempty"`
	ObserveModel   string                `json:"observe_model,omitempty"`
	ObserveFiles   *[]string             `json:"observe_files,omitempty"`
	Post           snapshotPost          `json:"post"`
	Profile        snapshotProfile       `json:"profile,omitempty"`
	Observations   []snapshotObservation `json:"observations,omitempty"`
	SnapshotOnly   bool                  `json:"snapshot_only,omitempty"`
}

// snapshotPost is PostInput as a snapshot has always carried it. Field, QualityRuleIDs and
// Published are inputs to resolve and never frozen, so they have no member at all.
type snapshotPost struct {
	Slug              string                `json:"Slug"`
	UserID            string                `json:"UserID"`
	Voice             snapshotVoice         `json:"Voice"`
	TemplateID        string                `json:"TemplateID"`
	Template          *snapshotTemplate     `json:"Template"`
	Guidelines        []string              `json:"Guidelines"`
	UseMemory         bool                  `json:"UseMemory"`
	Memories          []string              `json:"Memories"`
	QualityRules      []string              `json:"quality_rules,omitempty"`
	FieldPhrases      []string              `json:"field_phrases,omitempty"`
	TemplateAnswers   []snapshotAnswer      `json:"TemplateAnswers"`
	Title             string                `json:"Title"`
	Memo              string                `json:"Memo"`
	Images            []snapshotImage       `json:"Images"`
	Observations      []snapshotObservation `json:"Observations"`
	Content           *snapshotContent      `json:"Content"`
	TargetLanguage    string                `json:"TargetLanguage"`
	ContentLanguage   *string               `json:"ContentLanguage"`
	TargetLength      *int                  `json:"TargetLength"`
	TagCount          int                   `json:"tag_count,omitempty"`
	WriteNativeEffort bool                  `json:"WriteNativeEffort"`
}

type snapshotVoice struct {
	ID             string `json:"ID"`
	Name           string `json:"Name"`
	Deleted        bool   `json:"Deleted"`
	SourceLanguage string `json:"SourceLanguage"`
}

type snapshotTemplate struct {
	Name      string         `json:"Name"`
	Body      string         `json:"Body"`
	Slots     []snapshotSlot `json:"Slots"`
	Rows      []snapshotRow  `json:"Rows"`
	Facts     []snapshotFact `json:"Facts"`
	TitleArea string         `json:"TitleArea,omitempty"`
}

// snapshotSlot carries both a template's slot and a block's slot: the two have always been
// the same two keys.
type snapshotSlot struct {
	Kind  string `json:"Kind"`
	Label string `json:"Label"`
}

type snapshotRow struct {
	Count     int      `json:"Count"`
	Filenames []string `json:"Filenames"`
}

type snapshotFact struct {
	Label string `json:"Label"`
	Value string `json:"Value"`
}

type snapshotAnswer struct {
	Label   string `json:"Label"`
	Text    string `json:"Text"`
	Enabled bool   `json:"Enabled"`
}

type snapshotImage struct {
	Filename    string `json:"Filename"`
	Key         string `json:"Key"`
	Kind        string `json:"Kind"`
	ContentType string `json:"ContentType"`
	DurationMs  int64  `json:"DurationMs"`
}

type snapshotObservation struct {
	File          string   `json:"File"`
	Scene         string   `json:"Scene"`
	Mood          string   `json:"Mood"`
	VisibleText   string   `json:"VisibleText"`
	Objects       []string `json:"Objects"`
	PeoplePresent bool     `json:"PeoplePresent"`
	Model         string   `json:"Model"`
	Events        []string `json:"Events"`
	Speech        string   `json:"Speech"`
}

type snapshotContent struct {
	Title   string          `json:"Title"`
	Summary string          `json:"Summary"`
	Tags    []string        `json:"Tags"`
	Blocks  []snapshotBlock `json:"Blocks"`
}

type snapshotBlock struct {
	Type    string        `json:"Type"`
	Content string        `json:"Content"`
	Level   int32         `json:"Level"`
	File    string        `json:"File"`
	Alt     string        `json:"Alt"`
	Caption string        `json:"Caption"`
	Items   []string      `json:"Items"`
	Slot    *snapshotSlot `json:"Slot"`
}

type snapshotProfile struct {
	Styleguide           string   `json:"Styleguide"`
	ActiveRules          string   `json:"ActiveRules"`
	Excerpts             []string `json:"Excerpts"`
	Rules                string   `json:"Rules"`
	EndingMaxConsecutive int      `json:"EndingMaxConsecutive"`
	SourceLanguage       string   `json:"SourceLanguage"`
	TargetLanguage       string   `json:"TargetLanguage"`
	Portable             bool     `json:"Portable"`
}

// observeExperimentSnapshot is intentionally narrower than PostInput. Target/content
// language, voice, prose, template and write options cannot affect photo facts, so they
// cannot enter an observation experiment's candidate input or observation-only hash.
type observeExperimentSnapshot struct {
	Kind string                `json:"kind"`
	Post observeExperimentPost `json:"post"`
}

type observeExperimentPost struct {
	Slug   string          `json:"slug"`
	UserID string          `json:"user_id"`
	Images []snapshotImage `json:"images"`
}

// encodeWriteSnapshot is the one way a write snapshot becomes bytes. Plain json.Marshal: its
// HTML escaping is in every stored snapshot's bytes.
func encodeWriteSnapshot(snapshot writeSnapshot) ([]byte, error) {
	return json.Marshal(experimentSnapshot{
		Kind: "write", Prepared: snapshot.Prepared, TargetLanguage: string(snapshot.TargetLanguage),
		ObserveModel: snapshot.ObserveModel, ObserveFiles: copyOptionalTexts(snapshot.ObserveFiles),
		Post: toSnapshotPost(snapshot.Post), Profile: toSnapshotProfile(snapshot.Profile),
		Observations: mapSlice(snapshot.Observations, toSnapshotObservation), SnapshotOnly: snapshot.SnapshotOnly,
	})
}

// decodeWriteSnapshot reads a stored write snapshot back, with the legacy normalizations every
// reader has always applied — they are part of a prepared snapshot's bytes, since
// PrepareWriteInput re-encodes what this returns.
func decodeWriteSnapshot(raw []byte) (writeSnapshot, error) {
	var wire experimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &wire) != nil || wire.Kind != "write" {
		return writeSnapshot{}, fmt.Errorf("saved write input is unavailable")
	}
	snapshot := writeSnapshot{
		Prepared: wire.Prepared, TargetLanguage: Language(wire.TargetLanguage),
		ObserveModel: wire.ObserveModel, ObserveFiles: copyOptionalTexts(wire.ObserveFiles),
		Post: fromSnapshotPost(wire.Post), Profile: fromSnapshotProfile(wire.Profile),
		Observations: mapSlice(wire.Observations, fromSnapshotObservation), SnapshotOnly: wire.SnapshotOnly,
	}
	// Write snapshots persisted before the language field existed retain Korean on retry. New
	// snapshots cannot omit it because SnapshotWriteInput validates first.
	if snapshot.TargetLanguage == "" {
		snapshot.TargetLanguage = snapshot.Post.TargetLanguage
	}
	if snapshot.TargetLanguage == "" {
		snapshot.TargetLanguage = LanguageKorean
	}
	if !snapshot.TargetLanguage.Valid() {
		return writeSnapshot{}, ErrLanguageRequired
	}
	if snapshot.Post.TargetLanguage != "" && snapshot.Post.TargetLanguage != snapshot.TargetLanguage {
		return writeSnapshot{}, ErrLanguageRequired
	}
	snapshot.Post.TargetLanguage = snapshot.TargetLanguage
	// A snapshot frozen before the tag count existed prompts for the default (GEN-46).
	snapshot.Post.TagCount = resolveTagCount(snapshot.Post.TagCount)
	return snapshot, nil
}

func encodeObserveSnapshot(slug, userID string, images []Image) ([]byte, error) {
	return json.Marshal(observeExperimentSnapshot{Kind: "observe", Post: observeExperimentPost{
		Slug: slug, UserID: userID, Images: mapSlice(images, toSnapshotImage),
	}})
}

// decodeObserveSnapshot returns the frozen post an observation experiment compares models on.
func decodeObserveSnapshot(raw []byte) (PostInput, error) {
	var wire observeExperimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &wire) != nil || wire.Kind != "observe" {
		return PostInput{}, fmt.Errorf("saved observe input is unavailable")
	}
	return PostInput{Slug: wire.Post.Slug, UserID: wire.Post.UserID, Images: mapSlice(wire.Post.Images, fromSnapshotImage)}, nil
}

// mapSlice maps each element, keeping nil nil and empty empty.
func mapSlice[T, U any](in []T, f func(T) U) []U {
	if in == nil {
		return nil
	}
	out := make([]U, len(in))
	for i, value := range in {
		out[i] = f(value)
	}
	return out
}

// copyTexts copies a string slice, keeping nil nil and empty empty — unlike cloneTexts, which
// turns an empty slice into nil and so `[]` into `null`.
func copyTexts(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}

func copyOptionalTexts(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	copied := copyTexts(*in)
	return &copied
}

func toSnapshotPost(post PostInput) snapshotPost {
	var content *snapshotContent
	if post.Content != nil {
		mapped := toSnapshotContent(*post.Content)
		content = &mapped
	}
	var contentLanguage *string
	if post.ContentLanguage != nil {
		value := string(*post.ContentLanguage)
		contentLanguage = &value
	}
	return snapshotPost{
		Slug: post.Slug, UserID: post.UserID,
		Voice:      snapshotVoice{ID: post.Voice.ID, Name: post.Voice.Name, Deleted: post.Voice.Deleted, SourceLanguage: string(post.Voice.SourceLanguage)},
		TemplateID: post.TemplateID, Template: toSnapshotTemplate(post.Template),
		Guidelines: copyTexts(post.Guidelines), UseMemory: post.UseMemory, Memories: copyTexts(post.Memories),
		QualityRules: copyTexts(post.QualityRules), FieldPhrases: copyTexts(post.FieldPhrases),
		TemplateAnswers: mapSlice(post.TemplateAnswers, func(a TemplateAnswer) snapshotAnswer {
			return snapshotAnswer{Label: a.Label, Text: a.Text, Enabled: a.Enabled}
		}),
		Title: post.Title, Memo: post.Memo,
		Images:       mapSlice(post.Images, toSnapshotImage),
		Observations: mapSlice(post.Observations, toSnapshotObservation),
		Content:      content, TargetLanguage: string(post.TargetLanguage), ContentLanguage: contentLanguage,
		TargetLength: copyOptionalInt(post.TargetLength), TagCount: post.TagCount, WriteNativeEffort: post.WriteNativeEffort,
	}
}

func fromSnapshotPost(wire snapshotPost) PostInput {
	var content *PostContent
	if wire.Content != nil {
		mapped := fromSnapshotContent(*wire.Content)
		content = &mapped
	}
	var contentLanguage *Language
	if wire.ContentLanguage != nil {
		value := Language(*wire.ContentLanguage)
		contentLanguage = &value
	}
	return PostInput{
		Slug: wire.Slug, UserID: wire.UserID,
		Voice:      VoiceRef{ID: wire.Voice.ID, Name: wire.Voice.Name, Deleted: wire.Voice.Deleted, SourceLanguage: Language(wire.Voice.SourceLanguage)},
		TemplateID: wire.TemplateID, Template: fromSnapshotTemplate(wire.Template),
		Guidelines: copyTexts(wire.Guidelines), UseMemory: wire.UseMemory, Memories: copyTexts(wire.Memories),
		QualityRules: copyTexts(wire.QualityRules), FieldPhrases: copyTexts(wire.FieldPhrases),
		TemplateAnswers: mapSlice(wire.TemplateAnswers, func(a snapshotAnswer) TemplateAnswer {
			return TemplateAnswer{Label: a.Label, Text: a.Text, Enabled: a.Enabled}
		}),
		Title: wire.Title, Memo: wire.Memo,
		Images:       mapSlice(wire.Images, fromSnapshotImage),
		Observations: mapSlice(wire.Observations, fromSnapshotObservation),
		Content:      content, TargetLanguage: Language(wire.TargetLanguage), ContentLanguage: contentLanguage,
		TargetLength: copyOptionalInt(wire.TargetLength), TagCount: wire.TagCount, WriteNativeEffort: wire.WriteNativeEffort,
	}
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func toSnapshotTemplate(brief *TemplateBrief) *snapshotTemplate {
	if brief == nil {
		return nil
	}
	return &snapshotTemplate{
		Name: brief.Name, Body: brief.Body,
		Slots: mapSlice(brief.Slots, func(s TemplateSlot) snapshotSlot { return snapshotSlot{Kind: s.Kind, Label: s.Label} }),
		Rows: mapSlice(brief.Rows, func(r TemplatePhotoRow) snapshotRow {
			return snapshotRow{Count: r.Count, Filenames: copyTexts(r.Filenames)}
		}),
		Facts:     mapSlice(brief.Facts, func(f TemplateFact) snapshotFact { return snapshotFact{Label: f.Label, Value: f.Value} }),
		TitleArea: brief.TitleArea,
	}
}

func fromSnapshotTemplate(wire *snapshotTemplate) *TemplateBrief {
	if wire == nil {
		return nil
	}
	return &TemplateBrief{
		Name: wire.Name, Body: wire.Body,
		Slots: mapSlice(wire.Slots, func(s snapshotSlot) TemplateSlot { return TemplateSlot{Kind: s.Kind, Label: s.Label} }),
		Rows: mapSlice(wire.Rows, func(r snapshotRow) TemplatePhotoRow {
			return TemplatePhotoRow{Count: r.Count, Filenames: copyTexts(r.Filenames)}
		}),
		Facts:     mapSlice(wire.Facts, func(f snapshotFact) TemplateFact { return TemplateFact{Label: f.Label, Value: f.Value} }),
		TitleArea: wire.TitleArea,
	}
}

func toSnapshotImage(image Image) snapshotImage {
	return snapshotImage{Filename: image.Filename, Key: image.Key, Kind: string(image.Kind), ContentType: image.ContentType, DurationMs: image.DurationMs}
}

func fromSnapshotImage(wire snapshotImage) Image {
	return Image{Filename: wire.Filename, Key: wire.Key, Kind: AttachmentKind(wire.Kind), ContentType: wire.ContentType, DurationMs: wire.DurationMs}
}

func toSnapshotObservation(o Observation) snapshotObservation {
	return snapshotObservation{
		File: o.File, Scene: o.Scene, Mood: o.Mood, VisibleText: o.VisibleText, Objects: copyTexts(o.Objects),
		PeoplePresent: o.PeoplePresent, Model: o.Model, Events: copyTexts(o.Events), Speech: o.Speech,
	}
}

func fromSnapshotObservation(wire snapshotObservation) Observation {
	return Observation{
		File: wire.File, Scene: wire.Scene, Mood: wire.Mood, VisibleText: wire.VisibleText, Objects: copyTexts(wire.Objects),
		PeoplePresent: wire.PeoplePresent, Model: wire.Model, Events: copyTexts(wire.Events), Speech: wire.Speech,
	}
}

func toSnapshotContent(content PostContent) snapshotContent {
	return snapshotContent{
		Title: content.Title, Summary: content.Summary, Tags: copyTexts(content.Tags),
		Blocks: mapSlice(content.Blocks, func(b Block) snapshotBlock {
			var slot *snapshotSlot
			if b.Slot != nil {
				slot = &snapshotSlot{Kind: b.Slot.Kind, Label: b.Slot.Label}
			}
			return snapshotBlock{Type: string(b.Type), Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt, Caption: b.Caption, Items: copyTexts(b.Items), Slot: slot}
		}),
	}
}

func fromSnapshotContent(wire snapshotContent) PostContent {
	return PostContent{
		Title: wire.Title, Summary: wire.Summary, Tags: copyTexts(wire.Tags),
		Blocks: mapSlice(wire.Blocks, func(b snapshotBlock) Block {
			var slot *BlockSlot
			if b.Slot != nil {
				slot = &BlockSlot{Kind: b.Slot.Kind, Label: b.Slot.Label}
			}
			return Block{Type: BlockType(b.Type), Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt, Caption: b.Caption, Items: copyTexts(b.Items), Slot: slot}
		}),
	}
}

func toSnapshotProfile(profile Profile) snapshotProfile {
	return snapshotProfile{
		Styleguide: profile.Styleguide, ActiveRules: profile.ActiveRules, Excerpts: copyTexts(profile.Excerpts), Rules: profile.Rules,
		EndingMaxConsecutive: profile.EndingMaxConsecutive, SourceLanguage: string(profile.SourceLanguage),
		TargetLanguage: string(profile.TargetLanguage), Portable: profile.Portable,
	}
}

func fromSnapshotProfile(wire snapshotProfile) Profile {
	return Profile{
		Styleguide: wire.Styleguide, ActiveRules: wire.ActiveRules, Excerpts: copyTexts(wire.Excerpts), Rules: wire.Rules,
		EndingMaxConsecutive: wire.EndingMaxConsecutive, SourceLanguage: Language(wire.SourceLanguage),
		TargetLanguage: Language(wire.TargetLanguage), Portable: wire.Portable,
	}
}
