package app

import (
	"slices"
	"strings"

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
	if c != nil && !c.Snapshot.Legacy && s.CompositionCapability() < clip.CompositionPlanVersion {
		return clip.ErrCompositionUnavailable
	}
	return nil
}

// TemplateProjection is what the owner reads and edits: a body still written
// under the old grammar comes back converted, flagged so the editor can say so,
// while the stored body waits for the owner's own save (CLIP-140). Generation
// and project freezing read the stored template directly and never this.
func (s *Service) TemplateProjection(t clip.VideoTemplate) clip.VideoTemplate {
	if t.CompositionBody == "" {
		return t
	}
	// The converted body is what the owner will save, so it is read and BoundedText
	// by the template's own limits; a body that cannot be read under them is left
	// exactly as it is stored.
	converted, changed, problem := composition.ConvertLegacyTemplate(t.CompositionBody, s.limits.Composition)
	if problem != nil || !changed {
		return t
	}
	t.CompositionBody, t.CompositionConverted = converted, true
	return t
}

func (s *Service) authoredRecipe(r clip.Recipe) (clip.Recipe, error) {
	d, e := composition.ParseTemplate(r.CompositionBody, s.limits.Composition)
	if e != nil {
		return r, e
	}
	r.CompositionLegacy = false
	r.Accent = d.Accent
	r.CaptionPace = d.Pace
	r.Preset = ""
	r.InformationFields = nil
	r.CutGuidance = strings.Join(d.Guidance, "\n")
	// Compatibility projections are presentation only; field IDs remain authoritative.
	for _, f := range d.Fields {
		if f.Group == "" {
			r.InformationFields = append(r.InformationFields, clip.InformationField{Label: f.Label, Prompt: f.Prompt})
		}
	}
	return r, nil
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
	if t.CompositionBody == "" || t.CompositionLegacy {
		c := clip.LegacyProjectComposition(p, t.Recipe)
		if in != nil {
			d, problem := composition.ReadStored(c.Snapshot.Body, clip.LegacyCompositionLimits(s.limits.Composition))
			if problem != nil {
				return nil, problem
			}
			if err := clip.ValidateCompositionInputs(d, *in, s.limits.Composition, false); err != nil {
				return nil, err
			}
			if err := clip.ValidateSourceAssociations(p, in.Associations); err != nil {
				return nil, err
			}
			p.Answers = legacyAnswers(p.Answers, t.InformationFields, in.Values)
			c = clip.LegacyProjectComposition(p, t.Recipe)
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

// Typed patches replace all declared values; unrelated historical answers stay
// available to the compatibility renderer and are never turned into new fields.
func legacyAnswers(previous []clip.Answer, fields []clip.InformationField, values map[string]string) []clip.Answer {
	out := slices.Clone(previous)
	for _, field := range fields {
		answer := clip.Answer{Label: field.Label, Text: values[clip.LegacyFieldID(field.Label)]}
		found := false
		for i := range out {
			if out[i].Label == field.Label {
				out[i] = answer
				found = true
				break
			}
		}
		if !found {
			out = append(out, answer)
		}
	}
	return out
}
