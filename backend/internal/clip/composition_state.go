package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const CompositionVersion = 1
const CompositionPlanVersion = 5

var ErrCompositionUnavailable = errors.New("clip composition execution unavailable")

// XML escaping can expand an already valid legacy field fivefold. Read
// conversion has a bounded allowance without enlarging native authoring limits.
func LegacyCompositionLimits(l composition.Limits) composition.Limits {
	l.SourceChars *= len("&amp;")
	for _, preset := range design.Presets {
		l.CopyChars = max(l.CopyChars, l.AnswerChars+1+len([]rune(preset.PriceNote)))
	}
	return l
}

type SourceAssociation struct {
	GroupID, ItemID, SourceID, Fingerprint string
	StartMS, EndMS                         int
}
type CompositionInputs struct {
	Values       map[string]string
	Items        map[string][]composition.Item
	Associations []SourceAssociation
}
type CompositionSnapshot struct {
	LegacyRecipe     *Recipe
	Version          int
	Body, TemplateID string
	Legacy           bool
}
type ProjectComposition struct {
	Snapshot CompositionSnapshot
	Inputs   CompositionInputs
}
type CopyFallback struct{ ElementID, CutID, Reason string }
type SourceEvidence struct {
	SourceID, Fingerprint string
	StartMS, EndMS        int
}
type PortableText struct {
	Phrases               []EditablePhrase
	StaleEvidence         bool
	OwnerEdited           bool
	Placement             *CompositionPlacement
	Accent, Keyword, Pace string
	Resolved              composition.ResolvedElement
	Scope                 string
	Evidence              []SourceEvidence
	FallbackReason        string
	// Only grounded alternatives returned by the original writer call. The
	// renderer may select one for readability without asking a model again.
	Alternatives []CopyAlternative
}
type EditablePhrase struct {
	Text           string
	StartMS, EndMS int
}

// Effective automatic choices are frozen with the rendered draft. The original
// element still says whether a value was authored or automatically selected.
type CompositionPlacement struct {
	Style, Position string
	StartMS, EndMS  int // Relative to the owned cut, or output time without a cut.
}
type CopyAlternative struct {
	Text string
	Rows []composition.ResolvedRow
}
type PortablePlan struct {
	NativeEditing bool
	// Retained identities permit session undo after an accepted deletion save.
	// They are private to this generation; corrections cannot invent identities.
	RetiredCuts      []Cut
	RetiredBindings  []composition.Cut
	RetiredElements  []PortableText
	TargetDurationMS int
	Snapshot         CompositionSnapshot
	Inputs           CompositionInputs
	Cuts             []composition.Cut
	Elements         []PortableText
	Fallbacks        []CopyFallback
	// Frozen observations let automatic placement project subject geometry
	// after ratio/crop changes without observing the source again.
	Observations []SourceAnalysis
}

func LegacyFieldID(label string) string {
	h := sha256.Sum256([]byte(label))
	return "field-" + hex.EncodeToString(h[:8])
}
func node(name string, attrs map[string]string, children ...*composition.Node) *composition.Node {
	return &composition.Node{Name: name, Attributes: attrs, Children: children}
}
func literal(text string) *composition.Node { return &composition.Node{Name: "#text", Text: text} }
func legacyCompositionRoot(recipe Recipe) *composition.Node {
	styles := recipe.CopyStyles
	if len(styles) == 0 {
		styles = []string{"clean"}
	}
	pace := recipe.CaptionPace
	if pace == "" {
		pace = "steady"
	}
	root := node("clip", map[string]string{"version": "1", "styles": strings.Join(styles, " "), "accent": recipe.Accent, "pace": pace})
	for _, f := range recipe.InformationFields {
		root.Children = append(root.Children, node("field", map[string]string{"id": LegacyFieldID(f.Label), "label": f.Label, "required": "true"}, literal(f.Prompt)))
	}
	if recipe.CutGuidance != "" {
		root.Children = append(root.Children, node("guide", nil, literal(recipe.CutGuidance)))
	}
	copy := node("text", map[string]string{"id": "narrative", "kind": "ai", "role": "caption", "basis": "cut"}, literal("Describe only the observed scene, following the template narrative."))
	scene := node("scene", map[string]string{"id": "footage", "scope": "scene"}, copy)
	for _, f := range recipe.InformationFields {
		if slices.Contains(design.ChipPriority(recipe.Preset), f.Label) {
			scene.Children = append(scene.Children, node("text", map[string]string{"id": "info-" + strings.TrimPrefix(LegacyFieldID(f.Label), "field-"), "kind": "fixed", "role": "info", "position": "header", "basis": "cut"}, literal(f.Label+" "), node("value", map[string]string{"field": LegacyFieldID(f.Label)})))
		}
	}
	root.Children = append(root.Children, node("repeat", map[string]string{"for": "scenes"}, scene))
	return root
}

func LegacyCompositionBody(recipe Recipe) string {
	root := legacyCompositionRoot(recipe)
	fields := map[string]bool{}
	for _, f := range recipe.InformationFields {
		fields[f.Label] = true
	}
	if fields["상호"] {
		root.Children = append(root.Children, node("text", map[string]string{"id": "legacy-template-hook", "kind": "ai", "role": "hook", "basis": "output-start", "start": "0", "end": seconds(int(design.Timing.HookCardS * 1000))}, literal("Write a grounded opening sentence for "), node("value", map[string]string{"field": LegacyFieldID("상호")})))
		root.Children = append(root.Children, node("text", map[string]string{"id": "legacy-template-ending", "kind": "fixed", "role": "ending", "basis": "output-end", "start": seconds(-int(design.Timing.EndCardS * 1000)), "end": "0"}, node("row", map[string]string{"role": "body"}, node("value", map[string]string{"field": LegacyFieldID("상호")})), node("row", map[string]string{"role": "label"}, literal(design.CTA[design.DefaultCTA(recipe.Preset, "")]))))
	}
	return composition.SerializeNode(root)
}

// Legacy project furniture is frozen independently from its reusable template.
func LegacyProjectComposition(p Project, recipe Recipe) ProjectComposition {
	if retained, styles, err := DecodeEditPlan(p.EditPlan); err == nil {
		if len(styles) > 0 {
			recipe.CopyStyles = slices.Clone(styles)
		}
		foundAccent := false
		for _, cut := range retained.Cuts {
			for _, copy := range cut.Copies {
				if !foundAccent && copy.Accent != "" {
					recipe.Accent = copy.Accent
					foundAccent = true
				}
			}
		}
	}
	body := composition.SerializeNode(legacyCompositionRoot(recipe))
	body = legacyProjectFurniture(body, p, recipe, nil)
	in := CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}
	for _, f := range recipe.InformationFields {
		for _, a := range p.Answers {
			if a.Label == f.Label {
				in.Values[LegacyFieldID(f.Label)] = a.Text
			}
		}
	}
	recipe.CompositionBody = ""
	recipe.CompositionLegacy = true
	recipe.InformationFields = slices.Clone(recipe.InformationFields)
	recipe.CopyStyles = slices.Clone(recipe.CopyStyles)
	return ProjectComposition{Snapshot: CompositionSnapshot{Version: CompositionVersion, Body: body, TemplateID: p.VideoTemplateID, Legacy: true, LegacyRecipe: &recipe}, Inputs: in}
}

var compositionIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func ValidateCompositionInputs(d *composition.Document, in CompositionInputs, l composition.Limits, required bool) error {
	fields := map[string]composition.Field{}
	for _, f := range d.Fields {
		k := f.ID
		if f.Group != "" {
			k = f.Group + "." + k
		}
		fields[k] = f
	}
	check := func(values map[string]string, group string) error {
		for _, key := range slices.Sorted(maps.Keys(values)) {
			v := values[key]
			full := key
			if group != "" {
				full = group + "." + key
			}
			f, ok := fields[full]
			if !ok || f.Group != group {
				return &composition.Problem{ElementID: full, Line: 1, Reason: "unknown_field"}
			}
			if !bounded(v, 0, l.AnswerChars) {
				return &composition.Problem{ElementID: full, Line: f.Span.Line, Reason: "answer_limit"}
			}
		}
		if required {
			for _, f := range d.Fields {
				if f.Group == group && f.Required && strings.TrimSpace(values[f.ID]) == "" {
					return &composition.Problem{ElementID: f.ID, Line: f.Span.Line, Reason: "required_binding"}
				}
			}
		}
		return nil
	}
	if e := check(in.Values, ""); e != nil {
		return e
	}
	items := map[string]bool{}
	for _, group := range slices.Sorted(maps.Keys(in.Items)) {
		values := in.Items[group]
		if !slices.Contains(d.Groups, group) {
			return &composition.Problem{ElementID: group, Line: 1, Reason: "unknown_group"}
		}
		if len(values) > l.Items {
			return &composition.Problem{ElementID: group, Line: 1, Reason: "item_limit"}
		}
		for _, item := range values {
			k := group + "/" + item.ID
			if !compositionIdentity.MatchString(item.ID) || items[k] {
				return &composition.Problem{ElementID: item.ID, Line: 1, Reason: "duplicate_item"}
			}
			items[k] = true
			if e := check(item.Values, group); e != nil {
				return e
			}
		}
	}
	if len(in.Associations) > l.Cuts {
		return &composition.Problem{ElementID: "clip", Line: 1, Reason: "cut_limit"}
	}
	seen := map[SourceAssociation]bool{}
	for _, a := range in.Associations {
		if !items[a.GroupID+"/"+a.ItemID] || !compositionIdentity.MatchString(a.SourceID) || a.Fingerprint == "" || len(a.Fingerprint) > 128 || a.StartMS < 0 || a.EndMS <= a.StartMS || seen[a] {
			return &composition.Problem{ElementID: a.ItemID, Line: 1, Reason: "binding_scope"}
		}
		seen[a] = true
	}
	return nil
}

// An association is owner input against recorded evidence, never a new observation.
func ValidateSourceAssociations(p Project, associations []SourceAssociation) error {
	if len(associations) == 0 {
		return nil
	}
	analyses, err := RetainedObservations(p)
	if err != nil {
		return err
	}
	for _, a := range associations {
		valid := false
		for _, observed := range analyses {
			if observed.Source.ID != a.SourceID || observed.Source.Fingerprint != a.Fingerprint || a.EndMS > observed.Source.Info.DurationMS {
				continue
			}
			for _, segment := range observed.Segments {
				if a.StartMS >= segment.StartMS && a.EndMS <= segment.EndMS {
					valid = true
				}
			}
		}
		if !valid {
			return &composition.Problem{ElementID: a.ItemID, Line: 1, Reason: "source_association"}
		}
	}
	return nil
}

func GenerationComposition(t VideoTemplate, p Project, limits composition.Limits) (*ProjectComposition, error) {
	if t.CompositionBody == "" || t.CompositionLegacy {
		p.EditPlan = ""
		c := LegacyProjectComposition(p, t.Recipe)
		return &c, nil
	}
	d, err := composition.Parse(t.CompositionBody, limits)
	if err != nil {
		return nil, err
	}
	inputs := CompositionInputs{}
	if p.Composition != nil {
		inputs = p.Composition.Inputs
	}
	if err := ValidateCompositionInputs(d, inputs, limits, true); err != nil {
		return nil, err
	}
	if err := ValidateSourceAssociations(p, inputs.Associations); err != nil {
		return nil, err
	}
	return &ProjectComposition{Snapshot: CompositionSnapshot{Version: CompositionVersion, Body: d.Source, TemplateID: t.ID}, Inputs: inputs}, nil
}

// Consumers advertise support only when both planning and rendering understand
// the same stored plan. Merely accepting source drafts is not execution support.
type CompositionExecutor interface{ CompositionPlanVersion() int }

func (s *Service) CompositionCapability() int {
	if s.generation == nil {
		return 0
	}
	return s.generation.CompositionCapability()
}

func (s *GenerationService) CompositionCapability() int {
	p, pok := s.planner.(CompositionExecutor)
	r, rok := s.renderer.(CompositionExecutor)
	if !pok || !rok {
		return 0
	}
	return min(p.CompositionPlanVersion(), r.CompositionPlanVersion())
}

func (s *GenerationService) checkComposition(c *ProjectComposition) error {
	if c != nil && !c.Snapshot.Legacy && s.CompositionCapability() < CompositionPlanVersion {
		return ErrCompositionUnavailable
	}
	return nil
}

func (s *Service) authoredRecipe(r Recipe) (Recipe, error) {
	d, e := composition.Parse(r.CompositionBody, s.limits.Composition)
	if e != nil {
		return r, e
	}
	r.CompositionLegacy = false
	r.CopyStyles = slices.Clone(d.Styles)
	r.Accent = d.Accent
	r.CaptionPace = d.Pace
	r.Preset = ""
	r.InformationFields = nil
	r.CutGuidance = strings.Join(d.Guidance, "\n")
	// Compatibility projections are presentation only; field IDs remain authoritative.
	for _, f := range d.Fields {
		if f.Group == "" {
			r.InformationFields = append(r.InformationFields, InformationField{f.Label, f.Prompt})
		}
	}
	return r, nil
}

func (s *Service) projectComposition(t VideoTemplate, in *CompositionInputs, p Project) (*ProjectComposition, error) {
	if t.CompositionBody == "" || t.CompositionLegacy {
		c := LegacyProjectComposition(p, t.Recipe)
		if in != nil {
			d, problem := composition.Parse(c.Snapshot.Body, LegacyCompositionLimits(s.limits.Composition))
			if problem != nil {
				return nil, problem
			}
			if err := ValidateCompositionInputs(d, *in, s.limits.Composition, false); err != nil {
				return nil, err
			}
			if err := ValidateSourceAssociations(p, in.Associations); err != nil {
				return nil, err
			}
			p.Answers = legacyAnswers(p.Answers, t.InformationFields, in.Values)
			c = LegacyProjectComposition(p, t.Recipe)
		}
		return &c, nil
	}
	d, e := composition.Parse(t.CompositionBody, s.limits.Composition)
	if e != nil {
		return nil, e
	}
	values := CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}
	if in != nil {
		values = *in
	}
	if e := ValidateCompositionInputs(d, values, s.limits.Composition, false); e != nil {
		return nil, e
	}
	if err := ValidateSourceAssociations(p, values.Associations); err != nil {
		return nil, err
	}
	return &ProjectComposition{Snapshot: CompositionSnapshot{Version: CompositionVersion, Body: d.Source, TemplateID: t.ID}, Inputs: values}, nil
}

// Typed patches replace all declared values; unrelated historical answers stay
// available to the compatibility renderer and are never turned into new fields.
func legacyAnswers(previous []Answer, fields []InformationField, values map[string]string) []Answer {
	out := slices.Clone(previous)
	for _, field := range fields {
		answer := Answer{Label: field.Label, Text: values[LegacyFieldID(field.Label)]}
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

func MatchCompositionSources(c *ProjectComposition, b SourceBatch) error {
	if c == nil {
		return nil
	}
	for _, a := range c.Inputs.Associations {
		valid := false
		for _, source := range b.Sources {
			if source.ID == a.SourceID && source.Fingerprint == a.Fingerprint && a.StartMS >= 0 && a.EndMS <= source.DurationMS {
				valid = true
			}
		}
		if !valid {
			return &composition.Problem{ElementID: a.ItemID, Line: 1, Reason: "source_association"}
		}
	}
	return nil
}

func legacyProjectFurniture(body string, p Project, recipe Recipe, planOverride *EditPlan) string {
	rootEnd := strings.LastIndex(body, "</clip>")
	if rootEnd < 0 {
		return body
	}
	var extra strings.Builder
	add := func(id, role, basis string, start, end int, children ...*composition.Node) {
		a := map[string]string{"id": id, "kind": "fixed", "role": role, "basis": basis}
		if basis != "whole" {
			a["start"] = seconds(start)
			a["end"] = seconds(end)
		}
		if role == "badge" || role == "info" {
			a["position"] = "header"
		}
		extra.WriteString(composition.SerializeNode(node("text", a, children...)))
	}
	if !p.HideDisclosure && design.Disclosure[p.Disclosure] != "" {
		add("legacy-disclosure", "badge", "whole", 0, 0, literal(design.Disclosure[p.Disclosure]))
	}
	plan := planOverride
	if plan == nil && p.EditPlan != "" {
		if v, _, err := DecodeEditPlan(p.EditPlan); err == nil {
			plan = &v
		}
	}
	if plan != nil {
		values := map[string]string{}
		for _, a := range p.Answers {
			values[a.Label] = strings.TrimSpace(a.Text)
		}
		accent := recipe.Accent
		for _, cut := range plan.Cuts {
			for _, copy := range cut.Copies {
				if copy.Accent != "" && accent == "" {
					accent = copy.Accent
				}
			}
		}
		row := func(role, text string) *composition.Node {
			return node("row", map[string]string{"role": role}, literal(text))
		}
		if values["상호"] != "" {
			if strings.TrimSpace(plan.Hook) != "" {
				rows := []*composition.Node{}
				if label := design.Presets[recipe.Preset].Label; label != "" && accent != "" {
					rows = append(rows, row("label", label))
				}
				rows = append(rows, row("hook", strings.TrimSpace(plan.Hook)), row("body", values["상호"]))
				add("legacy-hook", "hook", "output-start", 0, min(int(design.Timing.HookCardS*1000), plan.DurationMS), rows...)
			}
			rows := []*composition.Node{row("body", values["상호"])}
			if values["위치"] != "" {
				rows = append(rows, row("label", values["위치"]))
			}
			for _, label := range []string{"가격", "메뉴"} {
				if value := values[label]; value != "" {
					if label == "가격" && design.Presets[recipe.Preset].PriceNote != "" {
						value += " " + design.Presets[recipe.Preset].PriceNote
					}
					rows = append(rows, row("caption", value))
					break
				}
			}
			if accent != "" {
				rows = append(rows, row("label", design.CTA[design.DefaultCTA(recipe.Preset, p.CTA)]))
			}
			add("legacy-ending", "ending", "output-end", -min(int(design.Timing.EndCardS*1000), plan.DurationMS), 0, rows...)
		}
	}
	return body[:rootEnd] + extra.String() + body[rootEnd:]
}
func seconds(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', -1, 64) }
