package generation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// observe runs the observation stage over `targets` — the photos this run was frozen to
// observe, which is not necessarily every attached photo — over a `seed` of what was already
// known, and returns the merged snapshot.
func (s *Service) observe(ctx context.Context, post PostInput, targets []Image, seed []Observation, model llm.ModelRef, progress Progress) ([]Observation, error) {
	observations, _, err := s.observeCandidate(ctx, post, targets, seed, model, progress, true)
	return observations, err
}

func (s *Service) observeCandidate(ctx context.Context, post PostInput, targets []Image, seed []Observation, model llm.ModelRef, progress Progress, persist bool) ([]Observation, llm.Usage, error) {
	total := len(targets)
	fresh := make([]Observation, 0, total)
	var usage llm.Usage
	for start := 0; start < total; start += s.batchSize {
		end := min(start+s.batchSize, total)
		batch := targets[start:end]
		parts := make([]llm.Part, 0, len(batch)+1)
		filenames := make([]string, 0, len(batch))
		for _, image := range batch {
			data, err := s.images.Read(ctx, image.Key)
			if err != nil {
				return nil, usage, fmt.Errorf("read photo %s: %w", image.Filename, err)
			}
			parts = append(parts, llm.ImagePart(data, "image/jpeg"))
			filenames = append(filenames, image.Filename)
		}
		parts = append(parts, llm.TextPart("files: "+strings.Join(filenames, ", ")))
		request := llm.Request{
			System:    ObservePrompt,
			Messages:  []llm.Message{{Role: llm.RoleUser, Parts: parts}},
			Reasoning: s.reasoning.Observe,
		}
		if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
			request.JSONSchema = ObservationsSchema()
		}
		response, err := s.models.Complete(ctx, model, request)
		usage.PromptTokens += response.Usage.PromptTokens
		usage.CompletionTokens += response.Usage.CompletionTokens
		if response.Usage.CostReported {
			usage.CostMicrousd += response.Usage.CostMicrousd
			usage.CostReported = true
		}
		if err != nil {
			return nil, usage, providerCallError("사진 관찰", err)
		}
		returned, err := parseObservations(response.Text)
		if err != nil {
			return nil, usage, fmt.Errorf("parse observations: %w", responseParseError(response, err))
		}
		fresh = append(fresh, matchObservations(batch, returned, model.String())...)
		if persist {
			// The MERGED snapshot, not just what this run has observed so far: a partial
			// re-observation has to leave one entry per attached photo already after the
			// FIRST batch, so the contact sheet only ever grows.
			if err := s.posts.SetObservations(ctx, post.UserID, post.Slug, mergeObservations(post.Images, seed, fresh)); err != nil {
				return nil, usage, fmt.Errorf("persist observations: %w", err)
			}
		}
		progress("observe", end, total)
	}
	return mergeObservations(post.Images, seed, fresh), usage, nil
}

func matchObservations(images []Image, returned []Observation, model string) []Observation {
	attached := make(map[string]struct{}, len(images))
	for _, image := range images {
		attached[image.Filename] = struct{}{}
	}
	byFile := make(map[string]Observation, len(returned))
	for _, observation := range returned {
		if _, ok := attached[observation.File]; !ok {
			slog.Warn("dropping observation for unattached file", "file", observation.File)
			continue
		}
		if _, duplicate := byFile[observation.File]; duplicate {
			slog.Warn("dropping duplicate observation", "file", observation.File)
			continue
		}
		byFile[observation.File] = observation
	}
	matched := make([]Observation, 0, len(images))
	for _, image := range images {
		observation, ok := byFile[image.Filename]
		if !ok {
			observation = Observation{File: image.Filename}
		}
		// Stamped here, where the batch's model is the known fact — on the empty entry a
		// missing filename produces too, so "this model looked and said nothing" stays
		// distinguishable from "nothing has ever looked at this photo".
		observation.Model = model
		matched = append(matched, observation)
	}
	return matched
}

// mergeObservations is what every persist writes: the seed of what was already known, in post
// order, with this run's fresh entries laid over it. A photo neither half covers is left OUT
// rather than given an empty entry — an empty entry reads as "a model looked and saw nothing",
// which is not the same as a photo still waiting its turn, and the contact sheet says so.
//
// Two properties follow, and both are the point. The snapshot can only grow or stay put during
// a run: a photo awaiting its batch keeps the entry it had, so a partial re-observation is
// complete from the first persist. And an unseeded run over every photo reproduces the
// pre-change writes exactly, batch by batch.
func mergeObservations(attached []Image, seed, fresh []Observation) []Observation {
	byFile := make(map[string]Observation, len(seed)+len(fresh))
	for _, observation := range seed {
		byFile[observation.File] = observation
	}
	// fresh last: a photo this run observed replaces whatever was known about it.
	for _, observation := range fresh {
		byFile[observation.File] = observation
	}
	merged := make([]Observation, 0, len(attached))
	for _, image := range attached {
		if observation, ok := byFile[image.Filename]; ok {
			merged = append(merged, observation)
		}
	}
	return merged
}

// observationEmpty reports an entry that carries no eyesight — what a photo has after a model
// looked and returned nothing for it. Provenance is deliberately not a field here: File names
// the photo and Model names who looked, and neither is something the writer can write from.
func observationEmpty(observation Observation) bool {
	return observation.Scene == "" && observation.Mood == "" && observation.VisibleText == "" &&
		len(observation.Objects) == 0 && !observation.PeoplePresent
}

// reusableObservations indexes the stored snapshot by filename, keeping only the entries a run
// may actually carry over. An empty entry is deliberately excluded: reusing it would let the
// write stage build from a photo nothing has ever described.
func reusableObservations(observations []Observation) map[string]Observation {
	byFile := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		if observation.File == "" || observationEmpty(observation) {
			continue
		}
		byFile[observation.File] = observation
	}
	return byFile
}

// resolveObserveSelection turns a requested filename set into the photos this run will observe.
// `requested` nil means no picker was involved, and every attached photo is observed.
//
// The request is read as a SET of filenames, so duplicates in it collapse and names that are
// not attached are dropped — the same discipline matchObservations applies to model output.
// Beyond that, every attached photo with nothing to reuse is FORCED into the selection
// server-side: the picker states that rule, but the run must not be able to write from a photo
// it has never looked at even when a client says otherwise.
func resolveObserveSelection(images []Image, stored []Observation, requested *[]string) []Image {
	reusable := reusableObservations(stored)
	selected := make(map[string]struct{}, len(images))
	if requested == nil {
		for _, image := range images {
			selected[image.Filename] = struct{}{}
		}
	} else {
		asked := make(map[string]struct{}, len(*requested))
		for _, filename := range *requested {
			asked[filename] = struct{}{}
		}
		for _, image := range images {
			_, reuse := reusable[image.Filename]
			if _, ok := asked[image.Filename]; ok || !reuse {
				selected[image.Filename] = struct{}{}
			}
		}
	}
	targets := make([]Image, 0, len(images))
	for _, image := range images {
		if _, ok := selected[image.Filename]; ok {
			targets = append(targets, image)
		}
	}
	return targets
}

// frozenObserveSelection reads a run's observation decision out of what was frozen for it and
// nothing else: the filename set, and the snapshot as it stood at the freeze. An absent set is
// a run enqueued before the picker existed, and it observes every attached photo with no seed
// — the behavior it was queued with, byte for byte.
//
// A photo attached after the freeze appears in neither half, so this run does not observe it
// and writes no entry for it; it belongs to the next generation. A photo deleted after the
// freeze simply drops out, because the seed is projected over the photos attached NOW.
func frozenObserveSelection(images []Image, observeFiles *[]string, frozen []Observation) (targets []Image, seed []Observation) {
	if observeFiles == nil {
		return images, nil
	}
	selected := make(map[string]struct{}, len(*observeFiles))
	for _, filename := range *observeFiles {
		selected[filename] = struct{}{}
	}
	targets = make([]Image, 0, len(images))
	for _, image := range images {
		if _, ok := selected[image.Filename]; ok {
			targets = append(targets, image)
		}
	}
	return targets, attachedObservations(images, frozen)
}

// freezeObserveSelection resolves a picker answer into the two things that get frozen: the
// filenames this run will observe, and the reusable snapshot as it stands right now. Both the
// ordinary enqueue and the A/B snapshot go through it, so the two entry points cannot drift
// apart on the forcing rule.
//
// The WHOLE reusable snapshot is frozen, not only the part being carried over, so that a photo
// waiting for its batch can keep the entry it already had. Which entries survive the run is
// then decided by ObserveFiles alone, in one place.
func freezeObserveSelection(images []Image, stored []Observation, requested *[]string) (files []string, snapshot []Observation) {
	targets := resolveObserveSelection(images, stored, requested)
	files = make([]string, 0, len(targets))
	for _, image := range targets {
		files = append(files, image.Filename)
	}
	return files, attachedObservations(images, stored)
}

// attachedObservations projects the reusable entries of a snapshot onto the photos actually
// attached, in post order. An entry for a photo that is no longer attached is dropped, which
// is the same discipline matchObservations applies to model output.
func attachedObservations(images []Image, observations []Observation) []Observation {
	reusable := reusableObservations(observations)
	out := make([]Observation, 0, len(images))
	for _, image := range images {
		if observation, ok := reusable[image.Filename]; ok {
			out = append(out, observation)
		}
	}
	return out
}

// observedImages narrows a live attachment list to the photos a run actually holds an
// observation for, in post order. It is what keeps the write stage's filename list and its
// observation list describing the same photos.
func observedImages(images []Image, observations []Observation) []Image {
	byFile := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		byFile[observation.File] = struct{}{}
	}
	out := make([]Image, 0, len(images))
	for _, image := range images {
		if _, ok := byFile[image.Filename]; ok {
			out = append(out, image)
		}
	}
	return out
}
