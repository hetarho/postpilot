package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"github.com/postpilot/backend/internal/platform/config"
)

type legacyRecipeJSON struct {
	Name     string      `json:"name"`
	Fields   []fieldJSON `json:"fields"`
	Guidance string      `json:"guidance"`
	Styles   []string    `json:"styles"`
	Accent   string      `json:"accent"`
	Preset   string      `json:"preset"`
	Pace     string      `json:"pace"`
}
type compositionSnapshotJSON struct {
	LegacyRecipe *legacyRecipeJSON `json:"legacy_recipe,omitempty"`
	Version      int               `json:"version"`
	Body         string            `json:"body"`
	TemplateID   string            `json:"template_id"`
	Legacy       bool              `json:"legacy"`
}
type compositionItemJSON struct {
	ID     string            `json:"id"`
	Values map[string]string `json:"values"`
}
type associationJSON struct {
	GroupID     string `json:"group_id"`
	ItemID      string `json:"item_id"`
	SourceID    string `json:"source_id"`
	Fingerprint string `json:"fingerprint"`
	StartMS     int    `json:"start_ms"`
	EndMS       int    `json:"end_ms"`
}
type compositionInputsJSON struct {
	Version      int                              `json:"version"`
	Values       map[string]string                `json:"values"`
	Items        map[string][]compositionItemJSON `json:"items"`
	Associations []associationJSON                `json:"associations"`
}

func encodeComposition(c *clip.ProjectComposition) (string, string, error) {
	if c == nil {
		return "", "", nil
	}
	s := compositionSnapshotJSON{Version: c.Snapshot.Version, Body: c.Snapshot.Body, TemplateID: c.Snapshot.TemplateID, Legacy: c.Snapshot.Legacy}
	if r := c.Snapshot.LegacyRecipe; r != nil {
		s.LegacyRecipe = &legacyRecipeJSON{Name: r.Name, Guidance: r.CutGuidance, Styles: r.CopyStyles, Accent: r.Accent, Preset: r.Preset, Pace: r.CaptionPace}
		for _, f := range r.InformationFields {
			s.LegacyRecipe.Fields = append(s.LegacyRecipe.Fields, fieldJSON{f.Label, f.Prompt})
		}
	}
	in := compositionInputsJSON{Version: clip.CompositionVersion, Values: c.Inputs.Values, Items: map[string][]compositionItemJSON{}, Associations: []associationJSON{}}
	if in.Values == nil {
		in.Values = map[string]string{}
	}
	for g, items := range c.Inputs.Items {
		in.Items[g] = []compositionItemJSON{}
		for _, item := range items {
			in.Items[g] = append(in.Items[g], compositionItemJSON{item.ID, item.Values})
		}
	}
	for _, a := range c.Inputs.Associations {
		in.Associations = append(in.Associations, associationJSON{a.GroupID, a.ItemID, a.SourceID, a.Fingerprint, a.StartMS, a.EndMS})
	}
	b, e := json.Marshal(s)
	if e != nil {
		return "", "", e
	}
	v, e := json.Marshal(in)
	return string(b), string(v), e
}
func decodeComposition(snapshot, inputs string) (*clip.ProjectComposition, error) {
	if snapshot == "" && inputs == "" {
		return nil, nil
	}
	var s compositionSnapshotJSON
	var in compositionInputsJSON
	if strictJSON(snapshot, &s) != nil || strictJSON(inputs, &in) != nil || s.Version != clip.CompositionVersion || in.Version != clip.CompositionVersion {
		return nil, errors.New("invalid stored clip composition version")
	}
	c := &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: s.Version, Body: s.Body, TemplateID: s.TemplateID, Legacy: s.Legacy}, Inputs: clip.CompositionInputs{Values: in.Values, Items: map[string][]composition.Item{}}}
	if r := s.LegacyRecipe; r != nil {
		recipe := &clip.Recipe{Name: r.Name, CutGuidance: r.Guidance, CopyStyles: r.Styles, Accent: r.Accent, Preset: r.Preset, CaptionPace: r.Pace, CompositionLegacy: true}
		for _, f := range r.Fields {
			recipe.InformationFields = append(recipe.InformationFields, clip.InformationField{Label: f.Label, Prompt: f.Prompt})
		}
		c.Snapshot.LegacyRecipe = recipe
	}
	for g, items := range in.Items {
		c.Inputs.Items[g] = []composition.Item{}
		for _, item := range items {
			c.Inputs.Items[g] = append(c.Inputs.Items[g], composition.Item{ID: item.ID, Values: item.Values})
		}
	}
	for _, a := range in.Associations {
		c.Inputs.Associations = append(c.Inputs.Associations, clip.SourceAssociation{GroupID: a.GroupID, ItemID: a.ItemID, SourceID: a.SourceID, Fingerprint: a.Fingerprint, StartMS: a.StartMS, EndMS: a.EndMS})
	}
	limits := config.ClipCompositionLimits()
	if s.Legacy {
		limits = clip.LegacyCompositionLimits(limits)
	}
	d, e := composition.Parse(s.Body, limits)
	if e != nil {
		return nil, e
	}
	if e := clip.ValidateCompositionInputs(d, c.Inputs, config.ClipCompositionLimits(), false); e != nil {
		return nil, e
	}
	return c, nil
}
func saveComposition(ctx context.Context, q *sqlc.Queries, p clip.Project) error {
	s, in, e := encodeComposition(p.Composition)
	if e != nil {
		return e
	}
	if s == "" {
		return nil
	}
	return affected(q.SaveProjectComposition(ctx, sqlc.SaveProjectCompositionParams{CompositionSnapshotJson: nullable(s), CompositionInputsJson: nullable(in), ID: p.ID, UserID: p.UserID}))
}
func hydrateComposition(ctx context.Context, q *sqlc.Queries, p *clip.Project) error {
	if p.Composition != nil {
		return nil
	}
	r := clip.Recipe{CopyStyles: []string{"clean"}}
	if p.VideoTemplateID != "" {
		t, e := getTemplate(ctx, q, p.UserID, p.VideoTemplateID)
		if e != nil {
			return e
		}
		r = t.Recipe
	}
	if r.CompositionBody != "" && !r.CompositionLegacy {
		p.Composition = &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: r.CompositionBody, TemplateID: p.VideoTemplateID}, Inputs: clip.CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}}
	} else {
		c := clip.LegacyProjectComposition(*p, r)
		p.Composition = &c
	}
	return nil
}

// Materialize pre-migration snapshots before changing their shared read input.
func freezeTemplateProjects(ctx context.Context, q *sqlc.Queries, user, id string) error {
	rows, err := q.ProjectsForTemplate(ctx, sqlc.ProjectsForTemplateParams{VideoTemplateID: nullable(id), UserID: user})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.CompositionSnapshotJson.Valid {
			continue
		}
		p, err := getProject(ctx, q, user, row.ID)
		if err != nil {
			return err
		}
		if err := saveComposition(ctx, q, p); err != nil {
			return err
		}
	}
	return nil
}
