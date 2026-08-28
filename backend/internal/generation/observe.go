package generation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) observe(ctx context.Context, post PostInput, model llm.ModelRef, progress Progress) ([]Observation, error) {
	total := len(post.Images)
	merged := make([]Observation, 0, total)
	for start := 0; start < total; start += s.batchSize {
		end := min(start+s.batchSize, total)
		batch := post.Images[start:end]
		parts := make([]llm.Part, 0, len(batch)+1)
		filenames := make([]string, 0, len(batch))
		for _, image := range batch {
			data, err := s.images.Read(ctx, image.Key)
			if err != nil {
				return nil, fmt.Errorf("read photo %s: %w", image.Filename, err)
			}
			parts = append(parts, llm.ImagePart(data, "image/jpeg"))
			filenames = append(filenames, image.Filename)
		}
		parts = append(parts, llm.TextPart("files: "+strings.Join(filenames, ", ")))
		request := llm.Request{
			System:   ObservePrompt,
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: parts}},
		}
		if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
			request.JSONSchema = ObservationsSchema()
		}
		response, err := s.models.Complete(ctx, model, request)
		if err != nil {
			return nil, providerCallError("사진 관찰", err)
		}
		returned, err := parseObservations(response.Text)
		if err != nil {
			return nil, fmt.Errorf("parse observations: %w", err)
		}
		merged = append(merged, matchObservations(batch, returned)...)
		if err := s.posts.SetObservations(ctx, post.UserID, post.Slug, merged); err != nil {
			return nil, fmt.Errorf("persist observations: %w", err)
		}
		progress("observe", end, total)
	}
	return merged, nil
}

func matchObservations(images []Image, returned []Observation) []Observation {
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
		matched = append(matched, observation)
	}
	return matched
}
