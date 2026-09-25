package design

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// A region preset is an anchored stack (CDS-87): slots, rules and groups in
// order with fixed ink-to-ink gaps between them, held at the preset's anchor.
// It fixes each slot's role, size, floor, face, paint and neutral decoration and
// nothing else (CDS-88): every word it draws is a template entry's.
type RegionPreset struct {
	Anchor RegionAnchor `json:"anchor"`
	// The measure every slot fits; 0 is the ratio's copy measure (CDS-9).
	Width float64 `json:"width"`
	// The scrim a bright ground gives the block (CDS-32): radial, top or
	// bottom; empty is radial for a centred block and the edge's own otherwise.
	Scrim string `json:"scrim"`
	// Degrees the whole block turns about its centre.
	Rotate  float64        `json:"rotate"`
	SideBar *RegionSideBar `json:"side_bar"`
	Stamp   *RegionStamp   `json:"stamp"`
	Items   []RegionItem   `json:"items"`
}

// RegionAnchor holds a block in place (CDS-79): its centre or its top edge at a
// 9:16 y that moves to the same fraction of canvas height on every ratio, or
// its bottom edge on the ratio's BOTTOM anchor; centred on the canvas or set
// left at the LEFT anchor plus an inset.
type RegionAnchor struct {
	Kind  string  `json:"kind"`
	Y     float64 `json:"y"`
	X     string  `json:"x"`
	Inset float64 `json:"inset"`
}

// RegionSideBar is a white bar standing at the LEFT anchor over the block's
// full height, the slots set to its right by the anchor's inset (CDS-96).
type RegionSideBar struct {
	W float64 `json:"w"`
}

// RegionStamp lays the preset's four slots out as a stamp (CDS-99): two rings,
// one slot on each arc between them, one inside, one below.
type RegionStamp struct {
	ROuter      float64 `json:"r_outer"`
	RInner      float64 `json:"r_inner"`
	StrokeOuter float64 `json:"stroke_outer"`
	StrokeInner float64 `json:"stroke_inner"`
	FillAlpha   float64 `json:"fill_alpha"`
	DotR        float64 `json:"dot_r"`
	ArcLen      float64 `json:"arc_len"`
	BelowGap    float64 `json:"below_gap"`
	Rotate      float64 `json:"rotate"`
}

type RegionSlotSpec struct {
	Role string `json:"role"`
	// 0 takes the role's own size (a hook: the ratio's hook size) and floor.
	Size     float64  `json:"size"`
	Floor    float64  `json:"floor"`
	Face     string   `json:"face"`
	Weight   int      `json:"weight"`
	Tracking *float64 `json:"tracking"`
	Fill     string   `json:"fill"`
	Alpha    *float64 `json:"alpha"`
	Stroke   string   `json:"stroke"`
	// A stroke width other than the two tokens (CDS-94's 14 px).
	StrokeWidth float64 `json:"stroke_width"`
	// Draw the text as a white outline of this width, with no fill (CDS-92).
	Outline float64 `json:"outline"`
	Shadow  string  `json:"shadow"`
	Lines   int     `json:"lines"`
	// The slot's own measure, narrower than the preset's.
	Width float64 `json:"width"`
	// Where a stamp slot sits: upper, centre, lower or below.
	Place string     `json:"place"`
	Decor *SlotDecor `json:"decor"`
}

// SlotDecor is the neutral decoration bound to one slot (CDS-88): it is drawn
// with the slot and dropped with it (CDS-73).
type SlotDecor struct {
	Flank *struct {
		W     float64 `json:"w"`
		Gap   float64 `json:"gap"`
		Alpha float64 `json:"alpha"`
	} `json:"flank"`
	Dot *struct {
		R   float64 `json:"r"`
		Gap float64 `json:"gap"`
	} `json:"dot"`
	Frame *struct {
		PadV   float64 `json:"pad_v"`
		PadH   float64 `json:"pad_h"`
		Stroke float64 `json:"stroke"`
		Alpha  float64 `json:"alpha"`
		MinW   float64 `json:"min_w"`
	} `json:"frame"`
	Plate *struct {
		PadV   float64 `json:"pad_v"`
		PadH   float64 `json:"pad_h"`
		Radius float64 `json:"radius"`
	} `json:"plate"`
}

type RegionRuleSpec struct {
	Kind  string   `json:"kind"`
	W     float64  `json:"w"`
	Alpha *float64 `json:"alpha"`
}

// RegionChips lays its slots as pills in rows that wrap inside the measure
// (CDS-97).
type RegionChips struct {
	Gap    float64 `json:"gap"`
	RowGap float64 `json:"row_gap"`
	Pill   struct {
		PadV      float64 `json:"pad_v"`
		PadH      float64 `json:"pad_h"`
		Stroke    float64 `json:"stroke"`
		Alpha     float64 `json:"alpha"`
		FillAlpha float64 `json:"fill_alpha"`
	} `json:"pill"`
	Slots []RegionSlotSpec `json:"slots"`
}

// RegionList sets its slots left-aligned as one group, each after a small
// square, all at the smallest size any of them needs (CDS-98).
type RegionList struct {
	Gap    float64 `json:"gap"`
	Square struct {
		Size float64 `json:"size"`
		Gap  float64 `json:"gap"`
	} `json:"square"`
	Slots []RegionSlotSpec `json:"slots"`
}

// RegionItem is one step of the stack: exactly one of a slot, a rule, a gap, a
// chip row or a list.
type RegionItem struct {
	Slot  *RegionSlotSpec
	Rule  *RegionRuleSpec
	Gap   float64
	Chips *RegionChips
	List  *RegionList
}

func (it *RegionItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 1 {
		return errors.New("a region item is exactly one of slot, rule, gap, chips or list")
	}
	for key, value := range raw {
		switch key {
		case "slot":
			it.Slot = &RegionSlotSpec{}
			return strictJSON(value, it.Slot)
		case "rule":
			it.Rule = &RegionRuleSpec{}
			return strictJSON(value, it.Rule)
		case "gap":
			return strictJSON(value, &it.Gap)
		case "chips":
			it.Chips = &RegionChips{}
			return strictJSON(value, it.Chips)
		case "list":
			it.List = &RegionList{}
			return strictJSON(value, it.List)
		default:
			return fmt.Errorf("unknown region item %q", key)
		}
	}
	return nil
}

// strictJSON decodes one value refusing a field the schema does not name, so a
// misspelt preset key fails at startup instead of drawing a default.
func strictJSON(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

// validate refuses a preset naming a role, colour, face, stroke, shadow, rule
// or stamp place the system does not define.
func (p RegionPreset) validate(s system) error {
	switch p.Anchor.Kind {
	case "centre", "top", "bottom":
	default:
		return fmt.Errorf("anchor kind %q", p.Anchor.Kind)
	}
	if p.Anchor.X != "" && p.Anchor.X != "centre" && p.Anchor.X != "left" {
		return fmt.Errorf("anchor x %q", p.Anchor.X)
	}
	if p.Scrim != "" && p.Scrim != "radial" && p.Scrim != "top" && p.Scrim != "bottom" {
		return fmt.Errorf("scrim %q", p.Scrim)
	}
	slots := p.Slots()
	if len(slots) == 0 {
		return errors.New("a preset draws at least one slot")
	}
	if p.Stamp != nil && len(slots) != 4 {
		return errors.New("a stamp has four slots")
	}
	for i, slot := range slots {
		if _, ok := s.Type[slot.Role]; !ok {
			return fmt.Errorf("slot %d role %q", i+1, slot.Role)
		}
		if _, ok := s.Color[slot.Fill]; !ok {
			return fmt.Errorf("slot %d fill %q", i+1, slot.Fill)
		}
		if _, ok := s.Faces[slot.Face]; slot.Face != "" && !ok {
			return fmt.Errorf("slot %d face %q", i+1, slot.Face)
		}
		if _, ok := s.Shadow[slot.Shadow]; slot.Shadow != "" && !ok {
			return fmt.Errorf("slot %d shadow %q", i+1, slot.Shadow)
		}
		switch slot.Stroke {
		case "", "none", "text", "small":
		default:
			return fmt.Errorf("slot %d stroke %q", i+1, slot.Stroke)
		}
		want := ""
		if p.Stamp != nil {
			want = []string{"upper", "centre", "lower", "below"}[i]
		}
		if slot.Place != want {
			return fmt.Errorf("slot %d place %q", i+1, slot.Place)
		}
	}
	for _, it := range p.Items {
		if it.Rule != nil {
			if _, ok := s.Rules[it.Rule.Kind]; !ok {
				return fmt.Errorf("rule kind %q", it.Rule.Kind)
			}
		}
	}
	return nil
}

// Slots lists the preset's slots in outline order, a group's slots in place.
func (p RegionPreset) Slots() []RegionSlotSpec {
	var out []RegionSlotSpec
	for _, it := range p.Items {
		switch {
		case it.Slot != nil:
			out = append(out, *it.Slot)
		case it.Chips != nil:
			out = append(out, it.Chips.Slots...)
		case it.List != nil:
			out = append(out, it.List.Slots...)
		}
	}
	return out
}

// Type is the slot's effective type on a ratio: the role's scale with the
// preset's own size, floor, face and tracking laid over it (CDS-19, CDS-46).
func (s RegionSlotSpec) Type(ratio string) TypeRole {
	role := Type[s.Role]
	if s.Role == "hook" {
		if layout, ok := Layout(ratio); ok {
			role.Size = layout.HookSize
		}
	}
	if s.Size > 0 {
		role.Size = s.Size
	}
	if s.Floor > 0 {
		role.Floor = s.Floor
	}
	if role.Floor <= 0 || role.Floor > role.Size {
		role.Floor = role.Size
	}
	if s.Face != "" {
		role.Face = s.Face
	}
	if s.Weight > 0 {
		role.Weight = s.Weight
	}
	if s.Tracking != nil {
		role.Tracking = *s.Tracking
	}
	return role
}

// Spec is what the slot's fit needs (CDS-86). An arc or chip slot is one line.
func (s RegionSlotSpec) Spec(ratio string) SlotSpec {
	t := s.Type(ratio)
	lines := s.Lines
	if s.Place == "upper" || s.Place == "lower" {
		lines = 1
	}
	return SlotSpec{Role: s.Role, Size: t.Size, Floor: t.Floor, Face: t.Face, Weight: t.Weight, Tracking: t.Tracking, Lines: lines}
}

// Paint is the slot's fill colour and opacity.
func (s RegionSlotSpec) Paint() (string, float64) {
	colour := Color[s.Fill]
	if s.Alpha != nil {
		colour.Alpha *= *s.Alpha
	}
	return colour.Hex, colour.Alpha
}

// Box is the rule's rectangle size and opacity.
func (r RegionRuleSpec) Box() (float64, float64, float64) {
	token := Rules[r.Kind]
	w, alpha := token.Width, token.Alpha
	if r.W > 0 {
		w = r.W
	}
	if r.Alpha != nil {
		alpha = *r.Alpha
	}
	return w, token.Height, alpha
}

// measure is the preset's own measure on a ratio; a left-set block's stops at
// the safe area's right edge (CDS-9, CDS-79).
func (p RegionPreset) measure(ratio string) float64 {
	layout, _ := Layout(ratio)
	w := layout.CopyMaxWidth
	if p.Width > 0 {
		w = p.Width
	}
	if p.Anchor.X == "left" {
		w = min(w, layout.Safe.X+layout.Safe.Width-layout.Anchor.Left-p.Anchor.Inset)
	}
	return w
}

// SlotWidth is the width slot i's text fits (CDS-86): the slot's own measure or
// the preset's, less whatever its decoration or group takes beside the text.
func (p RegionPreset) SlotWidth(ratio string, index int) float64 {
	measure := p.measure(ratio)
	i := 0
	for _, it := range p.Items {
		switch {
		case it.Slot != nil:
			if i == index {
				return it.Slot.textWidth(measure, p.Stamp)
			}
			i++
		case it.Chips != nil:
			if index < i+len(it.Chips.Slots) {
				return measure - 2*it.Chips.Pill.PadH
			}
			i += len(it.Chips.Slots)
		case it.List != nil:
			if index < i+len(it.List.Slots) {
				return measure - it.List.Square.Size - it.List.Square.Gap
			}
			i += len(it.List.Slots)
		}
	}
	return measure
}

func (s RegionSlotSpec) textWidth(measure float64, stamp *RegionStamp) float64 {
	if stamp != nil && (s.Place == "upper" || s.Place == "lower") {
		return stamp.ArcLen
	}
	w := measure
	if s.Width > 0 {
		w = s.Width
	}
	if d := s.Decor; d != nil {
		switch {
		case d.Flank != nil:
			w -= 2 * (d.Flank.W + d.Flank.Gap)
		case d.Dot != nil:
			w -= 2*d.Dot.R + d.Dot.Gap
		case d.Frame != nil:
			w -= 2 * d.Frame.PadH
		case d.Plate != nil:
			w -= 2 * d.Plate.PadH
		}
	}
	return w
}

// RegionArc is the circle an arc line runs along: the upper half left to right,
// or the lower half left to right read upright (CDS-99).
type RegionArc struct {
	CX, CY, R float64
	Lower     bool
}

// RegionLine is one drawn line: its text, fitted size, baseline, advance width
// from the metrics table, and where it stands — X is its centre for a centred
// line and its left edge for a left one.
type RegionLine struct {
	Text     string
	Size     float64
	Baseline float64
	Width    float64
	X        float64
	Align    string
	Arc      *RegionArc
	Rotated  bool
}

type PlacedRegionSlot struct {
	Index int
	Spec  RegionSlotSpec
	// The fitted type: Size is the size the lines are set at.
	Type  TypeRole
	Lines []RegionLine
	Over  bool
	// The slot's ink box: the table's Hangul ink over every line.
	Box Bounds
}

type PlacedRegionRule struct {
	Kind  string
	Box   Bounds
	Alpha float64
}

// RegionShape is one piece of neutral decoration (CDS-88) — a rectangle, a
// rounded one or a circle — bound to a slot or to the block itself (Slot -1).
type RegionShape struct {
	Kind        string
	Slot        int
	Box         Bounds
	Radius      float64
	Circle      bool
	Fill        string
	FillAlpha   float64
	Stroke      string
	StrokeAlpha float64
	StrokeWidth float64
	Shadow      bool
	Rotated     bool
}

// RegionRotation turns the parts marked Rotated about one point.
type RegionRotation struct {
	Deg, CX, CY float64
}

// RegionLayout is one region block laid out for its rows (CDS-86, CDS-87).
type RegionLayout struct {
	// Where lines stand: their centre for a centred block, their left edge for
	// a left-set one.
	AnchorX float64
	Align   string
	Width   float64
	Slots   []PlacedRegionSlot
	Rules   []PlacedRegionRule
	Shapes  []RegionShape
	Rotate  RegionRotation
	// radial, top or bottom (CDS-32).
	Scrim  string
	Bounds Bounds
	Over   bool
}

// Slot returns the placed slot for a preset slot index.
func (l RegionLayout) Slot(index int) (PlacedRegionSlot, bool) {
	for _, s := range l.Slots {
		if s.Index == index {
			return s, true
		}
	}
	return PlacedRegionSlot{}, false
}

// LayoutRegion lays one region block out for its rows, indexed by slot: every
// non-empty row fitted to its width (CDS-86), slots, rules and groups stacked by
// the preset's ink-to-ink gaps (CDS-87), an empty slot dropped together with the
// gap before it and its own decoration while the block's decoration stays as
// long as any slot draws (CDS-73), and the block held at its anchor on the
// ratio's canvas (CDS-79).
func LayoutRegion(kind, id, ratio string, rows []string) (RegionLayout, error) {
	preset, ok := Region(kind, id)
	if !ok {
		return RegionLayout{}, ViolationRegion
	}
	layout, ok := Layout(ratio)
	if !ok {
		return RegionLayout{}, ViolationRegion
	}
	out := RegionLayout{Width: preset.measure(ratio), Align: "centre", AnchorX: layout.Anchor.Center, Scrim: preset.Scrim}
	if preset.Anchor.X == "left" {
		out.Align, out.AnchorX = "left", layout.Anchor.Left+preset.Anchor.Inset
	}
	if out.Scrim == "" {
		out.Scrim = "radial"
		if preset.Anchor.Kind == "top" || preset.Anchor.Kind == "bottom" {
			out.Scrim = preset.Anchor.Kind
		}
	}
	row := func(i int) string {
		if i < len(rows) {
			return rows[i]
		}
		return ""
	}
	if preset.Stamp != nil {
		return layoutStamp(out, preset, layout, ratio, row), nil
	}
	type step struct {
		slot   *PlacedRegionSlot
		shapes []RegionShape
		rule   *PlacedRegionRule
		group  []PlacedRegionSlot
		place  func(top float64) ([]PlacedRegionSlot, []RegionShape)
		height float64
		gap    float64
		pad    float64
	}
	var steps []step
	pending, index, drawn := 0.0, 0, false
	for _, it := range preset.Items {
		switch {
		case it.Gap > 0:
			pending = it.Gap
		case it.Slot != nil:
			i := index
			index++
			text := row(i)
			if strings.TrimSpace(text) == "" {
				pending = 0
				continue
			}
			slot, height, pad := fitStackSlot(*it.Slot, ratio, i, text, preset.SlotWidth(ratio, i))
			steps = append(steps, step{slot: slot, height: height, gap: pending, pad: pad})
			pending, drawn = 0, true
			out.Over = out.Over || slot.Over
		case it.Rule != nil:
			w, h, alpha := it.Rule.Box()
			steps = append(steps, step{rule: &PlacedRegionRule{Kind: it.Rule.Kind, Box: Bounds{Width: w, Height: h}, Alpha: alpha}, height: h, gap: pending})
			pending = 0
		case it.Chips != nil:
			first := index
			index += len(it.Chips.Slots)
			height, place, any, over := layoutChips(*it.Chips, ratio, first, row, out, preset.SlotWidth(ratio, first))
			if !any {
				pending = 0
				continue
			}
			steps = append(steps, step{place: place, height: height, gap: pending})
			pending, drawn = 0, true
			out.Over = out.Over || over
		case it.List != nil:
			first := index
			index += len(it.List.Slots)
			height, place, any, over := layoutList(*it.List, ratio, first, row, out, preset.SlotWidth(ratio, first))
			if !any {
				pending = 0
				continue
			}
			steps = append(steps, step{place: place, height: height, gap: pending})
			pending, drawn = 0, true
			out.Over = out.Over || over
		}
	}
	if !drawn {
		return out, nil
	}
	total, first := 0.0, true
	for _, s := range steps {
		if !first {
			total += s.gap
		}
		total += s.height
		first = false
	}
	base, _ := Layout("vertical")
	scale := float64(layout.Canvas.Height) / float64(base.Canvas.Height)
	var top float64
	switch preset.Anchor.Kind {
	case "top":
		top = preset.Anchor.Y * scale
	case "bottom":
		top = layout.Anchor.Bottom - total
	default:
		top = preset.Anchor.Y*scale - total/2
	}
	left := func(w float64) float64 {
		if out.Align == "left" {
			return out.AnchorX
		}
		return out.AnchorX - w/2
	}
	y, first := top, true
	for _, s := range steps {
		if !first {
			y += s.gap
		}
		first = false
		switch {
		case s.slot != nil:
			t := s.slot.Type
			ink := InkOf(t.Face, t.Weight)
			widest := 0.0
			for k := range s.slot.Lines {
				s.slot.Lines[k].Baseline = y + s.pad + ink.Top*t.Size + float64(k)*t.Size*t.LineHeight
				widest = math.Max(widest, s.slot.Lines[k].Width)
			}
			s.slot.Box = Bounds{X: left(widest), Y: y + s.pad, Width: widest, Height: s.height - 2*s.pad}
			shapes := decorate(s.slot, out, y, s.height)
			out.Slots = append(out.Slots, *s.slot)
			out.Shapes = append(out.Shapes, shapes...)
			out.Bounds = unionBounds(out.Bounds, s.slot.Box)
			for _, shape := range shapes {
				out.Bounds = unionBounds(out.Bounds, shape.Box)
			}
		case s.place != nil:
			slots, shapes := s.place(y)
			for _, slot := range slots {
				out.Slots = append(out.Slots, slot)
				out.Bounds = unionBounds(out.Bounds, slot.Box)
			}
			for _, shape := range shapes {
				out.Shapes = append(out.Shapes, shape)
				out.Bounds = unionBounds(out.Bounds, shape.Box)
			}
		default:
			s.rule.Box.X, s.rule.Box.Y = left(s.rule.Box.Width), y
			out.Rules = append(out.Rules, *s.rule)
			out.Bounds = unionBounds(out.Bounds, s.rule.Box)
		}
		y += s.height
	}
	if preset.SideBar != nil {
		bar := RegionShape{Kind: "side_bar", Slot: -1, Box: Bounds{X: layout.Anchor.Left, Y: top, Width: preset.SideBar.W, Height: total}, Fill: Color["text_white"].Hex, FillAlpha: 1}
		out.Shapes = append(out.Shapes, bar)
		out.Bounds = unionBounds(out.Bounds, bar.Box)
	}
	if preset.Rotate != 0 {
		// About the block's centre: its anchor line and the middle of its stack.
		cx := out.AnchorX
		if out.Align == "left" {
			cx = out.Bounds.X + out.Bounds.Width/2
		}
		out.Rotate = RegionRotation{Deg: preset.Rotate, CX: cx, CY: top + total/2}
		for i := range out.Slots {
			for k := range out.Slots[i].Lines {
				out.Slots[i].Lines[k].Rotated = true
			}
		}
		for i := range out.Shapes {
			out.Shapes[i].Rotated = true
		}
	}
	return out, nil
}

// fitStackSlot fits one stacked slot and reports its height, including the
// padding a frame or plate adds above and below the text.
func fitStackSlot(spec RegionSlotSpec, ratio string, index int, text string, width float64) (*PlacedRegionSlot, float64, float64) {
	t := spec.Type(ratio)
	fit := FitRegionSlot(spec.Spec(ratio), text, width)
	t.Size = fit.Size
	slot := &PlacedRegionSlot{Index: index, Spec: spec, Type: t, Over: fit.Over}
	for _, line := range fit.Lines {
		slot.Lines = append(slot.Lines, RegionLine{Text: line, Size: fit.Size, Width: TextWidth(t.Face, t.Weight, t.Tracking, fit.Size, line)})
	}
	ink := InkOf(t.Face, t.Weight)
	height := ink.Top*fit.Size + float64(len(fit.Lines)-1)*fit.Size*t.LineHeight + ink.Bottom*fit.Size
	pad := 0.0
	if d := spec.Decor; d != nil {
		switch {
		case d.Frame != nil:
			pad = d.Frame.PadV
		case d.Plate != nil:
			pad = d.Plate.PadV
		}
	}
	return slot, height + 2*pad, pad
}

// decorate sets a stacked slot's line positions and draws its own decoration.
func decorate(slot *PlacedRegionSlot, out RegionLayout, top, height float64) []RegionShape {
	for k := range slot.Lines {
		slot.Lines[k].X, slot.Lines[k].Align = out.AnchorX, out.Align
	}
	d := slot.Spec.Decor
	if d == nil {
		return nil
	}
	widest := slot.Box.Width
	mid := slot.Box.Y + slot.Box.Height*0.52
	white := Color["text_white"].Hex
	switch {
	case d.Flank != nil:
		w := d.Flank
		centre := out.AnchorX
		return []RegionShape{
			{Kind: "flank", Slot: slot.Index, Box: Bounds{X: centre - widest/2 - w.Gap - w.W, Y: mid - 1, Width: w.W, Height: 2}, Fill: white, FillAlpha: w.Alpha},
			{Kind: "flank", Slot: slot.Index, Box: Bounds{X: centre + widest/2 + w.Gap, Y: mid - 1, Width: w.W, Height: 2}, Fill: white, FillAlpha: w.Alpha},
		}
	case d.Dot != nil:
		for k := range slot.Lines {
			slot.Lines[k].X = out.AnchorX + 2*d.Dot.R + d.Dot.Gap
		}
		slot.Box.X = out.AnchorX
		slot.Box.Width = 2*d.Dot.R + d.Dot.Gap + widest
		centre := slot.Box.Y + slot.Box.Height/2
		return []RegionShape{{Kind: "dot", Slot: slot.Index, Circle: true, Radius: d.Dot.R, Box: Bounds{X: out.AnchorX, Y: centre - d.Dot.R, Width: 2 * d.Dot.R, Height: 2 * d.Dot.R}, Fill: white, FillAlpha: 1, Shadow: true}}
	case d.Frame != nil:
		f := d.Frame
		w := math.Max(widest+2*f.PadH, f.MinW)
		return []RegionShape{{Kind: "frame", Slot: slot.Index, Box: Bounds{X: out.AnchorX - w/2, Y: top, Width: w, Height: height}, Stroke: white, StrokeAlpha: f.Alpha, StrokeWidth: f.Stroke, Shadow: true}}
	case d.Plate != nil:
		p := d.Plate
		w := widest + 2*p.PadH
		plate := Color["badge_ad"]
		return []RegionShape{{Kind: "plate", Slot: slot.Index, Box: Bounds{X: out.AnchorX - w/2, Y: top, Width: w, Height: height}, Radius: p.Radius, Fill: plate.Hex, FillAlpha: plate.Alpha}}
	}
	return nil
}

// layoutChips fits every filled chip slot on one line and flows the pills into
// centred rows that start anew when the next chip does not fit (CDS-97).
func layoutChips(c RegionChips, ratio string, first int, row func(int) string, out RegionLayout, width float64) (float64, func(float64) ([]PlacedRegionSlot, []RegionShape), bool, bool) {
	type chip struct {
		slot *PlacedRegionSlot
		w, h float64
	}
	var chips []chip
	over := false
	for j, spec := range c.Slots {
		text := row(first + j)
		if strings.TrimSpace(text) == "" {
			continue
		}
		one := spec
		one.Lines = 1
		slot, height, _ := fitStackSlot(one, ratio, first+j, text, width)
		over = over || slot.Over
		chips = append(chips, chip{slot: slot, w: slot.Lines[0].Width + 2*c.Pill.PadH, h: height + 2*c.Pill.PadV})
	}
	if len(chips) == 0 {
		return 0, nil, false, false
	}
	var rows [][]chip
	var cur []chip
	used := 0.0
	for _, ch := range chips {
		if len(cur) > 0 && used+c.Gap+ch.w > out.Width {
			rows = append(rows, cur)
			cur, used = nil, 0
		}
		if len(cur) > 0 {
			used += c.Gap
		}
		cur = append(cur, ch)
		used += ch.w
	}
	rows = append(rows, cur)
	rowHeight := 0.0
	for _, ch := range chips {
		rowHeight = math.Max(rowHeight, ch.h)
	}
	height := float64(len(rows))*rowHeight + float64(len(rows)-1)*c.RowGap
	place := func(top float64) ([]PlacedRegionSlot, []RegionShape) {
		var slots []PlacedRegionSlot
		var shapes []RegionShape
		for r, line := range rows {
			w := 0.0
			for i, ch := range line {
				if i > 0 {
					w += c.Gap
				}
				w += ch.w
			}
			x := out.AnchorX - w/2
			y := top + float64(r)*(rowHeight+c.RowGap)
			for _, ch := range line {
				t := ch.slot.Type
				ink := InkOf(t.Face, t.Weight)
				ch.slot.Lines[0].Baseline = y + c.Pill.PadV + ink.Top*t.Size
				ch.slot.Lines[0].X, ch.slot.Lines[0].Align = x+ch.w/2, "centre"
				ch.slot.Box = Bounds{X: x + c.Pill.PadH, Y: y + c.Pill.PadV, Width: ch.slot.Lines[0].Width, Height: rowHeight - 2*c.Pill.PadV}
				shapes = append(shapes, RegionShape{Kind: "pill", Slot: ch.slot.Index, Box: Bounds{X: x, Y: y, Width: ch.w, Height: rowHeight}, Radius: rowHeight / 2, Fill: "#000000", FillAlpha: c.Pill.FillAlpha, Stroke: Color["text_white"].Hex, StrokeAlpha: c.Pill.Alpha, StrokeWidth: c.Pill.Stroke})
				slots = append(slots, *ch.slot)
				x += ch.w + c.Gap
			}
		}
		return slots, shapes
	}
	return height, place, true, over
}

// layoutList sets the filled list slots left-aligned as one centred group, each
// after a small square, all at the smallest size any of them needs (CDS-98).
func layoutList(l RegionList, ratio string, first int, row func(int) string, out RegionLayout, width float64) (float64, func(float64) ([]PlacedRegionSlot, []RegionShape), bool, bool) {
	var items []*PlacedRegionSlot
	size, over := math.Inf(1), false
	for j, spec := range l.Slots {
		text := row(first + j)
		if strings.TrimSpace(text) == "" {
			continue
		}
		one := spec
		one.Lines = 1
		slot, _, _ := fitStackSlot(one, ratio, first+j, text, width)
		over = over || slot.Over
		size = math.Min(size, slot.Type.Size)
		items = append(items, slot)
	}
	if len(items) == 0 {
		return 0, nil, false, false
	}
	widest, rowHeight := 0.0, 0.0
	for _, slot := range items {
		t := &slot.Type
		t.Size = size
		slot.Lines[0].Size = size
		slot.Lines[0].Width = TextWidth(t.Face, t.Weight, t.Tracking, size, slot.Lines[0].Text)
		widest = math.Max(widest, slot.Lines[0].Width)
		ink := InkOf(t.Face, t.Weight)
		rowHeight = ink.Top*size + ink.Bottom*size
	}
	groupW := l.Square.Size + l.Square.Gap + widest
	height := float64(len(items))*rowHeight + float64(len(items)-1)*l.Gap
	place := func(top float64) ([]PlacedRegionSlot, []RegionShape) {
		x := out.AnchorX - groupW/2
		var slots []PlacedRegionSlot
		var shapes []RegionShape
		for r, slot := range items {
			y := top + float64(r)*(rowHeight+l.Gap)
			ink := InkOf(slot.Type.Face, slot.Type.Weight)
			slot.Lines[0].Baseline = y + ink.Top*size
			slot.Lines[0].X, slot.Lines[0].Align = x+l.Square.Size+l.Square.Gap, "left"
			slot.Box = Bounds{X: slot.Lines[0].X, Y: y, Width: slot.Lines[0].Width, Height: rowHeight}
			mid := y + rowHeight/2
			shapes = append(shapes, RegionShape{Kind: "square", Slot: slot.Index, Box: Bounds{X: x, Y: mid - l.Square.Size/2, Width: l.Square.Size, Height: l.Square.Size}, Radius: 2, Fill: Color["text_white"].Hex, FillAlpha: 1, Shadow: true})
			slots = append(slots, *slot)
		}
		return slots, shapes
	}
	return height, place, true, over
}

// layoutStamp lays the four stamp slots out around two rings centred at the
// anchor (CDS-99): the rings, the arcs and the inside turn together; the slot
// below the stamp does not.
func layoutStamp(out RegionLayout, preset RegionPreset, layout RatioLayout, ratio string, row func(int) string) RegionLayout {
	st := preset.Stamp
	base, _ := Layout("vertical")
	cy := preset.Anchor.Y * float64(layout.Canvas.Height) / float64(base.Canvas.Height)
	cx := out.AnchorX
	drawn := false
	for i, spec := range preset.Slots() {
		text := row(i)
		if strings.TrimSpace(text) == "" {
			continue
		}
		drawn = true
		t := spec.Type(ratio)
		fit := FitRegionSlot(spec.Spec(ratio), text, preset.SlotWidth(ratio, i))
		t.Size = fit.Size
		slot := PlacedRegionSlot{Index: i, Spec: spec, Type: t, Over: fit.Over}
		ink := InkOf(t.Face, t.Weight)
		for _, line := range fit.Lines {
			slot.Lines = append(slot.Lines, RegionLine{Text: line, Size: fit.Size, Width: TextWidth(t.Face, t.Weight, t.Tracking, fit.Size, line), X: cx, Align: "centre", Rotated: spec.Place != "below"})
		}
		switch spec.Place {
		case "upper", "lower":
			r := st.RInner + 14
			if spec.Place == "lower" {
				r = st.ROuter - 14
			}
			line := &slot.Lines[0]
			line.Arc = &RegionArc{CX: cx, CY: cy, R: r, Lower: spec.Place == "lower"}
			// The band the arc's glyphs occupy, for the manifest and V1.
			if spec.Place == "upper" {
				line.Baseline = cy - r
				slot.Box = Bounds{X: cx - r - ink.Top*t.Size, Y: cy - r - ink.Top*t.Size, Width: 2 * (r + ink.Top*t.Size), Height: r + ink.Top*t.Size}
			} else {
				line.Baseline = cy + r
				slot.Box = Bounds{X: cx - r, Y: cy, Width: 2 * r, Height: r}
			}
		case "centre":
			h := ink.Top*t.Size + float64(len(fit.Lines)-1)*t.Size*t.LineHeight + ink.Bottom*t.Size
			top := cy - h/2
			widest := 0.0
			for k := range slot.Lines {
				slot.Lines[k].Baseline = top + ink.Top*t.Size + float64(k)*t.Size*t.LineHeight
				widest = math.Max(widest, slot.Lines[k].Width)
			}
			slot.Box = Bounds{X: cx - widest/2, Y: top, Width: widest, Height: h}
		default:
			top := cy + st.ROuter + st.BelowGap
			h := ink.Top*t.Size + float64(len(fit.Lines)-1)*t.Size*t.LineHeight + ink.Bottom*t.Size
			widest := 0.0
			for k := range slot.Lines {
				slot.Lines[k].Baseline = top + ink.Top*t.Size + float64(k)*t.Size*t.LineHeight
				widest = math.Max(widest, slot.Lines[k].Width)
			}
			slot.Box = Bounds{X: cx - widest/2, Y: top, Width: widest, Height: h}
		}
		out.Slots = append(out.Slots, slot)
		out.Bounds = unionBounds(out.Bounds, slot.Box)
		out.Over = out.Over || fit.Over
	}
	if !drawn {
		return out
	}
	white := Color["text_white"].Hex
	ring := func(kind string, r, stroke, fill float64) RegionShape {
		s := RegionShape{Kind: kind, Slot: -1, Circle: true, Radius: r, Box: Bounds{X: cx - r, Y: cy - r, Width: 2 * r, Height: 2 * r}, Stroke: white, StrokeAlpha: 1, StrokeWidth: stroke, Rotated: true, Shadow: true}
		if fill > 0 {
			s.Fill, s.FillAlpha = "#000000", fill
		}
		return s
	}
	out.Shapes = append(out.Shapes, ring("ring", st.ROuter, st.StrokeOuter, st.FillAlpha), ring("ring", st.RInner, st.StrokeInner, 0))
	mid := (st.ROuter + st.RInner) / 2
	for _, x := range []float64{cx - mid, cx + mid} {
		out.Shapes = append(out.Shapes, RegionShape{Kind: "ring_dot", Slot: -1, Circle: true, Radius: st.DotR, Box: Bounds{X: x - st.DotR, Y: cy - st.DotR, Width: 2 * st.DotR, Height: 2 * st.DotR}, Fill: white, FillAlpha: 1, Rotated: true})
	}
	for _, shape := range out.Shapes {
		out.Bounds = unionBounds(out.Bounds, shape.Box)
	}
	out.Rotate = RegionRotation{Deg: st.Rotate, CX: cx, CY: cy}
	return out
}

func unionBounds(a, b Bounds) Bounds {
	if a.Width == 0 && a.Height == 0 {
		return b
	}
	x0, y0 := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	x1, y1 := math.Max(a.X+a.Width, b.X+b.Width), math.Max(a.Y+a.Height, b.Y+b.Height)
	return Bounds{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// RegionSlotAt is one preset slot's fit spec and the width it fits on a ratio
// (CDS-86): what the writer is told and what its repair judges a row by.
func RegionSlotAt(kind, id, ratio string, index int) (SlotSpec, float64, bool) {
	preset, ok := Region(kind, id)
	_, known := Layout(ratio)
	slots := preset.Slots()
	if !ok || !known || index < 0 || index >= len(slots) {
		return SlotSpec{}, 0, false
	}
	spec := slots[index].Spec(ratio)
	// A chip or list slot is one line whatever its role.
	if isGroupSlot(preset, index) {
		spec.Lines = 1
	}
	return spec, preset.SlotWidth(ratio, index), true
}

func isGroupSlot(p RegionPreset, index int) bool {
	i := 0
	for _, it := range p.Items {
		switch {
		case it.Slot != nil:
			if i == index {
				return false
			}
			i++
		case it.Chips != nil:
			if index < i+len(it.Chips.Slots) {
				return true
			}
			i += len(it.Chips.Slots)
		case it.List != nil:
			if index < i+len(it.List.Slots) {
				return true
			}
			i += len(it.List.Slots)
		}
	}
	return false
}
