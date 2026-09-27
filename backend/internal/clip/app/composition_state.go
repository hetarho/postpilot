package app

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func (s *Service) CompositionCapability() int {
	if s.generation == nil {
		return 0
	}
	return s.generation.CompositionCapability()
}

func (s *GenerationService) CompositionCapability() int {
	p, pok := s.planner.(clip.CompositionExecutor)
	r, rok := s.renderer.(clip.CompositionExecutor)
	if !pok || !rok {
		return 0
	}
	return min(p.CompositionPlanVersion(), r.CompositionPlanVersion())
}

func (s *GenerationService) checkComposition(c *clip.ProjectComposition) error {
	if c != nil && s.CompositionCapability() < clip.CompositionPlanVersion {
		return clip.ErrCompositionUnavailable
	}
	return nil
}

// authoredBody reads a template body under the template's own limits and
// returns it exactly as the owner wrote it; a template is its outline and
// nothing is derived from it to be stored beside it (CLIP-4, CLIP-14).
func (s *Service) authoredBody(body string) error {
	if _, e := composition.ParseTemplate(body, s.limits.Composition); e != nil {
		return e
	}
	return nil
}

func (s *Service) projectComposition(t clip.VideoTemplate, in *clip.CompositionInputs, p clip.Project) (*clip.ProjectComposition, error) {
	if t.ID == "" {
		c := clip.NoTemplateComposition()
		if in != nil {
			d, problem := composition.ReadStored(c.Snapshot.Body, s.limits.Composition)
			if problem != nil {
				return nil, problem
			}
			// Nothing is declared, so any value or item supplied here names a
			// field that does not exist and is refused rather than stored.
			if err := clip.ValidateCompositionInputs(d, *in, s.limits.Composition, false); err != nil {
				return nil, err
			}
			if err := clip.ValidateSourceAssociations(p, in.Associations); err != nil {
				return nil, err
			}
		}
		return &c, nil
	}
	d, e := composition.Parse(t.CompositionBody, s.limits.Composition)
	if e != nil {
		return nil, e
	}
	values := clip.CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}
	if in != nil {
		values = *in
	}
	if e := clip.ValidateCompositionInputs(d, values, s.limits.Composition, false); e != nil {
		return nil, e
	}
	if err := clip.ValidateSourceAssociations(p, values.Associations); err != nil {
		return nil, err
	}
	return &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: d.Source, TemplateID: t.ID}, Inputs: values}, nil
}
