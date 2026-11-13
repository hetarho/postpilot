package post

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// The observation fields the owner may correct (POST-126), as OwnerFields names them. People
// presence, speech, the model and the rotation are not offered for editing.
const (
	ObservationFieldScene       = "scene"
	ObservationFieldMood        = "mood"
	ObservationFieldVisibleText = "visible_text"
	ObservationFieldObjects     = "objects"
	ObservationFieldEvents      = "events"
)

// ErrObservationEditInvalid: an edit naming an attachment the post holds no observation for,
// one naming the same attachment twice, or `events` on a photo. The editor only offers the
// observed attachments' own fields, so each is a client bug.
var ErrObservationEditInvalid = errors.New("the observation edit is invalid")

// ObservationEdit is the owner's correction of one attachment's observation (POST-126). Each
// field is presence-aware: nil keeps the stored value and a present one replaces it, where an
// empty list clears the field.
type ObservationEdit struct {
	File                     string
	Scene, Mood, VisibleText *string
	Objects, Events          *[]string
}

// observationWriterKinds are the job kinds whose persist writes the observation snapshot: the
// observation job, and the storyline and generation jobs that observe what is missing first
// (GEN-89). An owner edit saved under one of them would be overwritten by its merged snapshot.
var observationWriterKinds = []string{"observe", "generate", "storyline"}

func writesObservations(active *ActiveJob) bool {
	return active != nil && slices.Contains(observationWriterKinds, active.Kind)
}

// validObservationEdits is what the edits can be refused for without reading the post.
func validObservationEdits(edits []ObservationEdit) error {
	seen := make(map[string]bool, len(edits))
	for _, edit := range edits {
		if edit.File == "" || seen[edit.File] {
			return ErrObservationEditInvalid
		}
		seen[edit.File] = true
	}
	return nil
}

// checkObservationEdits refuses edits the post cannot take — while a job that writes the
// observations targets the post (its persist would overwrite the edit), naming an attachment
// with no observation entry, or giving a photo `events` — and returns the snapshot to store, nil
// when nothing changes: an identical save is no edit and sets no mark.
func (s *Service) checkObservationEdits(ctx context.Context, found Post, edits []ObservationEdit) ([]Observation, error) {
	if len(edits) == 0 {
		return nil, nil
	}
	if s.jobs != nil {
		active, err := s.ordinaryForPost(ctx, found.UserID, found.Slug)
		if err != nil {
			return nil, fmt.Errorf("check active job before an observation edit: %w", err)
		}
		if writesObservations(active) {
			return nil, ErrPostBusy
		}
	}
	// Only a video's entry has events (VIDEO-9), and the entry alone cannot tell a video that
	// showed none from a photo.
	var videos []string
	if slices.ContainsFunc(edits, func(edit ObservationEdit) bool { return edit.Events != nil }) {
		listed, err := s.videos.ListVideos(ctx, found.Slug)
		if err != nil {
			return nil, fmt.Errorf("list videos for an observation edit: %w", err)
		}
		for _, video := range listed {
			videos = append(videos, video.Filename)
		}
	}
	next := slices.Clone(found.Observations)
	changed := false
	for _, edit := range edits {
		index := slices.IndexFunc(next, func(observation Observation) bool { return observation.File == edit.File })
		if index < 0 {
			return nil, ErrObservationEditInvalid
		}
		if edit.Events != nil && !slices.Contains(videos, edit.File) {
			return nil, ErrObservationEditInvalid
		}
		edited, fields := applyObservationEdit(next[index], edit)
		if len(fields) == 0 {
			continue
		}
		edited.OwnerEdited = true
		for _, field := range fields {
			if !slices.Contains(edited.OwnerFields, field) {
				edited.OwnerFields = append(slices.Clone(edited.OwnerFields), field)
			}
		}
		next[index] = edited
		changed = true
	}
	if !changed {
		return nil, nil
	}
	return next, nil
}

// applyObservationEdit lays the present fields over the stored entry and names each one whose
// value actually changed.
func applyObservationEdit(stored Observation, edit ObservationEdit) (Observation, []string) {
	var fields []string
	text := func(field string, value *string, target *string) {
		if value != nil && *value != *target {
			*target = *value
			fields = append(fields, field)
		}
	}
	list := func(field string, value *[]string, target *[]string) {
		if value != nil && !slices.Equal(*value, *target) {
			*target = slices.Clone(*value)
			fields = append(fields, field)
		}
	}
	text(ObservationFieldScene, edit.Scene, &stored.Scene)
	text(ObservationFieldMood, edit.Mood, &stored.Mood)
	text(ObservationFieldVisibleText, edit.VisibleText, &stored.VisibleText)
	list(ObservationFieldObjects, edit.Objects, &stored.Objects)
	list(ObservationFieldEvents, edit.Events, &stored.Events)
	return stored, fields
}
