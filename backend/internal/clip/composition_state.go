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

// The envelope version 5 wrote. It is a portable reader in its own right, not a
// legacy one, which is why decoding dispatches on the exact stored version and
// never compares it numerically against CompositionPlanVersion.
const portablePlanVersion = 5

// Version 6 adds each cut's fixed playback rate and the complete owner-owned
// source-audio snapshot (CLIP-98, CLIP-18). New plans are written in it.
const CompositionPlanVersion = 6

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
	Phrases       []EditablePhrase
	StaleEvidence bool
	OwnerEdited   bool
	Placement     *CompositionPlacement
	// What the owner set for this caption in ② — its own position, size and
	// style (CDS-82). The zero value is a caption nobody has placed, which is
	// automatic placement and the project's default style, exactly as before.
	Owner OwnerCaption
	// The template's own words rather than the writer's: a caption entry of the
	// outline whose text is fixed (CLIP-65). It is placed by the narration call
	// like any other caption, and the grounding a WRITTEN claim answers to does
	// not apply to it (CLIP-122, CDS-42).
	Authored              bool
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

// NarrationScope marks a caption the narration call wrote (CLIP-134). Such a
// caption belongs to no cut and to no template declaration: it owns an absolute
// interval on the transformed output timeline, may lie inside one cut or cross
// several, and never overlaps another narration caption (CLIP-66).
const NarrationScope = "narration"

// narrationInstanceID is the ONE shape a narration identity takes. The server
// mints it, the way it mints an owner cut id: a client that sent one would be
// creating an identity, and only the plan may hold identities (CLIP-97).
var narrationInstanceID = regexp.MustCompile(`^narration-([1-9][0-9]*)$`)

func NarrationID(n int) string { return "narration-" + strconv.Itoa(n) }

// NextNarrationID is the next free narration identity, one past the highest
// number the plan has ever used. `existing` is every identity the plan holds,
// retired captions included, so a removed caption's id is never handed to a
// different sentence — the rule cut ids already follow.
func NextNarrationID(existing []string) string {
	highest := 0
	for _, id := range existing {
		if m := narrationInstanceID.FindStringSubmatch(id); m != nil {
			n, _ := strconv.Atoi(m[1])
			highest = max(highest, n)
		}
	}
	return NarrationID(highest + 1)
}

// NarrationCaption starts with automatic placement and the default treatment.
// The narration can name a style on Element; the owner can override it in ②.
// Its interval is absolute on the output timeline.
func NarrationCaption(id, text string, startMS, endMS int) PortableText {
	a, b := startMS, endMS
	e := composition.Element{ID: id, Kind: "ai", Role: "caption", Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &a, EndMS: &b}
	return PortableText{Scope: NarrationScope, Resolved: composition.ResolvedElement{InstanceID: id, Element: e, Text: text, StartMS: a, EndMS: b, AuthoredTiming: true}}
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
	pace := recipe.CaptionPace
	if pace == "" {
		pace = "steady"
	}
	root := node("clip", map[string]string{"version": "1", "intro": "b", "caption": "bold", "outro": "e", "accent": recipe.Accent, "pace": pace})
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

// EmptyCompositionBody is the document a project with NO template generates
// from (CLIP-5): an outline with no entry at all — no slot text, no field, no
// group, no badge and no guide — so the clip is its footage and its narration
// alone. It names no design either: what a region renders in is the project's
// and is read from there (CLIP-14, CLIP-139).
func EmptyCompositionBody() string {
	return composition.SerializeNode(node("clip", map[string]string{"version": "1"}))
}

// NoTemplate reports a frozen document no template stands behind: the empty one
// above, frozen by a project that never had a template or has lost the one it
// had (CLIP-5, CLIP-25). A detached LEGACY project is not one of these — its
// recipe is still frozen with it.
func (c *ProjectComposition) NoTemplate() bool {
	return c != nil && !c.Snapshot.Legacy && c.Snapshot.TemplateID == ""
}

// NoTemplateComposition is what a project with no template freezes: the empty
// document above and no inputs, because nothing declared any.
func NoTemplateComposition() ProjectComposition {
	return ProjectComposition{
		Snapshot: CompositionSnapshot{Version: CompositionVersion, Body: EmptyCompositionBody()},
		Inputs:   CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}},
	}
}

func LegacyCompositionBody(recipe Recipe) string {
	root := legacyCompositionRoot(recipe)
	hook := node("text", map[string]string{"id": "legacy-template-hook", "kind": "ai", "role": "hook", "basis": "output-start"})
	ending := node("text", map[string]string{"id": "legacy-template-ending", "kind": "fixed", "role": "ending", "basis": "output-end"})
	for _, field := range recipe.InformationFields {
		if field.Label == "상호" {
			hook.Children = append(hook.Children, node("row", nil, literal("Write a grounded opening phrase for "), node("value", map[string]string{"field": LegacyFieldID(field.Label)})))
			ending.Children = append(ending.Children, node("row", nil, node("value", map[string]string{"field": LegacyFieldID(field.Label)})))
		}
	}
	root.Children = append(root.Children, hook, ending)
	return composition.SerializeNode(root)
}

// Legacy project furniture is frozen independently from its reusable template.
func LegacyProjectComposition(p Project, recipe Recipe) ProjectComposition {
	if retained, err := DecodeEditPlan(p.EditPlan); err == nil {
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
	if required {
		for _, group := range d.Groups {
			// The effective minimum the grammar derived, never the declared
			// one — a group holding a required field admits an item even when
			// the template saved no minimum (CLIP-119).
			minimum, count := d.Minima[group.ID], len(in.Items[group.ID])
			if count < minimum {
				name := group.Label
				if strings.TrimSpace(name) == "" {
					name = group.ID
				}
				return &composition.Problem{ElementID: name, Line: group.Span.Line, Reason: "items_required", Label: name, Min: minimum, Actual: count}
			}
		}
	}
	for _, group := range slices.Sorted(maps.Keys(in.Items)) {
		values := in.Items[group]
		index := slices.IndexFunc(d.Groups, func(g composition.Group) bool { return g.ID == group })
		if index < 0 {
			return &composition.Problem{ElementID: group, Line: 1, Reason: "unknown_group"}
		}
		if len(values) > min(l.Items, d.Groups[index].Max) {
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
// ValidateSourceAssociations refuses a range the project's own observations
// contradict. A binding that matches no retained observation is ignored rather
// than refused (CLIP-123): before generation there are no observations at all,
// and a source the owner replaced simply stops matching — nothing downstream
// reads a binding whose source is gone, because every cut carries the source id
// and fingerprint the binding must match.
func ValidateSourceAssociations(p Project, associations []SourceAssociation) error {
	if len(associations) == 0 {
		return nil
	}
	analyses, err := RetainedObservations(p)
	if err != nil {
		return err
	}
	for _, a := range associations {
		for _, observed := range analyses {
			if observed.Source.ID != a.SourceID || observed.Source.Fingerprint != a.Fingerprint {
				continue
			}
			// The source was observed, so the range has to be footage that was
			// actually looked at. A whole-source binding spans every segment by
			// design and is checked against the source's own duration instead.
			if a.StartMS == 0 && a.EndMS == observed.Source.Info.DurationMS {
				break
			}
			inside := false
			for _, segment := range observed.Segments {
				inside = inside || a.StartMS >= segment.StartMS && a.EndMS <= segment.EndMS
			}
			if !inside || a.EndMS > observed.Source.Info.DurationMS {
				return &composition.Problem{ElementID: a.ItemID, Line: 1, Reason: "source_association"}
			}
			break
		}
	}
	return nil
}

func GenerationComposition(t VideoTemplate, p Project, limits composition.Limits) (*ProjectComposition, error) {
	// No template is attached — never selected, or deleted since. The clip is
	// generated from the project's own settings and there is no declared
	// structure to satisfy, so CLIP-102's check has nothing to check rather
	// than something to refuse (CLIP-5, CLIP-25).
	if t.ID == "" {
		c := NoTemplateComposition()
		return &c, nil
	}
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

// TemplateProjection is what the owner reads and edits: a body still written
// under the old grammar comes back converted, flagged so the editor can say so,
// while the stored body waits for the owner's own save (CLIP-140). Generation
// and project freezing read the stored template directly and never this.
func (s *Service) TemplateProjection(t VideoTemplate) VideoTemplate {
	if t.CompositionBody == "" {
		return t
	}
	// The converted body is what the owner will save, so it is read and bounded
	// by the template's own limits; a body that cannot be read under them is left
	// exactly as it is stored.
	converted, changed, problem := composition.ConvertLegacyTemplate(t.CompositionBody, s.limits.Composition)
	if problem != nil || !changed {
		return t
	}
	t.CompositionBody, t.CompositionConverted = converted, true
	return t
}

func (s *Service) authoredRecipe(r Recipe) (Recipe, error) {
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
			r.InformationFields = append(r.InformationFields, InformationField{f.Label, f.Prompt})
		}
	}
	return r, nil
}

func (s *Service) projectComposition(t VideoTemplate, in *CompositionInputs, p Project) (*ProjectComposition, error) {
	if t.ID == "" {
		c := NoTemplateComposition()
		if in != nil {
			d, problem := composition.ReadStored(c.Snapshot.Body, s.limits.Composition)
			if problem != nil {
				return nil, problem
			}
			// Nothing is declared, so any value or item supplied here names a
			// field that does not exist and is refused rather than stored.
			if err := ValidateCompositionInputs(d, *in, s.limits.Composition, false); err != nil {
				return nil, err
			}
			if err := ValidateSourceAssociations(p, in.Associations); err != nil {
				return nil, err
			}
		}
		return &c, nil
	}
	if t.CompositionBody == "" || t.CompositionLegacy {
		c := LegacyProjectComposition(p, t.Recipe)
		if in != nil {
			d, problem := composition.ReadStored(c.Snapshot.Body, LegacyCompositionLimits(s.limits.Composition))
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
		if v, err := DecodeEditPlan(p.EditPlan); err == nil {
			plan = &v
		}
	}
	values := map[string]string{}
	if plan != nil {
		for _, answer := range p.Answers {
			values[answer.Label] = strings.TrimSpace(answer.Text)
		}
	}
	row := func(text string) *composition.Node { return node("row", nil, literal(text)) }
	hook := ""
	if plan != nil {
		hook = strings.TrimSpace(plan.Hook)
	}
	add("legacy-hook", "hook", "output-start", 0, int(design.Timing.IntroDefaultS*1000), row(hook), row(func() string {
		if hook != "" {
			return values["상호"]
		}
		return ""
	}()))
	detail := values["가격"]
	if detail == "" {
		detail = values["메뉴"]
	}
	add("legacy-ending", "ending", "output-end", -int(design.Timing.OutroDefaultS*1000), 0, row(values["상호"]), row(values["위치"]), row(detail))
	return body[:rootEnd] + extra.String() + body[rootEnd:]
}
func seconds(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', -1, 64) }
