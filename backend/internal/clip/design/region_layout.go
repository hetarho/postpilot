package design

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// A region preset is an anchored stack (CDS-87): slots and rules in order with
// fixed ink-to-ink gaps between them, held at the preset's anchor. It fixes each
// slot's role, size, floor, face and paint and nothing else (CDS-88): every word
// it draws is a template entry's.
type RegionPreset struct {
	Anchor RegionAnchor `json:"anchor"`
	// The measure every slot fits; 0 is the ratio's copy measure (CDS-9).
	Width float64      `json:"width"`
	Items []RegionItem `json:"items"`
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
	Shadow   string   `json:"shadow"`
	Lines    int      `json:"lines"`
}

type RegionRuleSpec struct {
	Kind  string   `json:"kind"`
	W     float64  `json:"w"`
	Alpha *float64 `json:"alpha"`
}

// RegionItem is one step of the stack: exactly one of a slot, a rule or a gap.
type RegionItem struct {
	Slot *RegionSlotSpec
	Rule *RegionRuleSpec
	Gap  float64
}

func (it *RegionItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 1 {
		return errors.New("a region item is exactly one of slot, rule or gap")
	}
	for key, value := range raw {
		switch key {
		case "slot":
			it.Slot = &RegionSlotSpec{}
			return json.Unmarshal(value, it.Slot)
		case "rule":
			it.Rule = &RegionRuleSpec{}
			return json.Unmarshal(value, it.Rule)
		case "gap":
			return json.NewDecoder(bytes.NewReader(value)).Decode(&it.Gap)
		default:
			return fmt.Errorf("unknown region item %q", key)
		}
	}
	return nil
}

// Slots lists the preset's slots in outline order.
func (p RegionPreset) Slots() []RegionSlotSpec {
	var out []RegionSlotSpec
	for _, it := range p.Items {
		if it.Slot != nil {
			out = append(out, *it.Slot)
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

// Spec is what the slot's fit needs (CDS-86).
func (s RegionSlotSpec) Spec(ratio string) SlotSpec {
	t := s.Type(ratio)
	return SlotSpec{Role: s.Role, Size: t.Size, Floor: t.Floor, Face: t.Face, Weight: t.Weight, Tracking: t.Tracking, Lines: s.Lines}
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

// RegionLine is one drawn line: its text, fitted size, baseline and advance
// width from the metrics table.
type RegionLine struct {
	Text     string
	Size     float64
	Baseline float64
	Width    float64
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

// RegionLayout is one region block laid out for its rows (CDS-86, CDS-87).
type RegionLayout struct {
	// Where lines stand: their centre for a centred block, their left edge for
	// a left-set one.
	AnchorX float64
	Align   string
	Width   float64
	Slots   []PlacedRegionSlot
	Rules   []PlacedRegionRule
	Bounds  Bounds
	Over    bool
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
// non-empty row fitted to the measure (CDS-86), slots and rules stacked by the
// preset's ink-to-ink gaps (CDS-87), an empty slot dropped together with the
// gap before it and the block's rules kept while any slot draws (CDS-73), and
// the block held at its anchor on the ratio's canvas (CDS-79).
func LayoutRegion(kind, id, ratio string, rows []string) (RegionLayout, error) {
	preset, ok := Region(kind, id)
	if !ok {
		return RegionLayout{}, ViolationRegion
	}
	layout, ok := Layout(ratio)
	if !ok {
		return RegionLayout{}, ViolationRegion
	}
	out := RegionLayout{Width: layout.CopyMaxWidth, Align: "centre", AnchorX: layout.Anchor.Center}
	if preset.Width > 0 {
		out.Width = preset.Width
	}
	if preset.Anchor.X == "left" {
		out.Align, out.AnchorX = "left", layout.Anchor.Left+preset.Anchor.Inset
	}
	type step struct {
		slot   *PlacedRegionSlot
		rule   *PlacedRegionRule
		height float64
		gap    float64
	}
	var steps []step
	pending, index, drawn := 0.0, 0, false
	for _, it := range preset.Items {
		switch {
		case it.Gap > 0:
			pending = it.Gap
		case it.Slot != nil:
			text := ""
			if index < len(rows) {
				text = rows[index]
			}
			i := index
			index++
			if strings.TrimSpace(text) == "" {
				pending = 0
				continue
			}
			t := it.Slot.Type(ratio)
			fit := FitRegionSlot(it.Slot.Spec(ratio), text, out.Width)
			t.Size = fit.Size
			slot := &PlacedRegionSlot{Index: i, Spec: *it.Slot, Type: t, Over: fit.Over}
			for _, line := range fit.Lines {
				slot.Lines = append(slot.Lines, RegionLine{Text: line, Size: fit.Size, Width: TextWidth(t.Face, t.Weight, t.Tracking, fit.Size, line)})
			}
			ink := InkOf(t.Face, t.Weight)
			height := ink.Top*fit.Size + float64(len(fit.Lines)-1)*fit.Size*t.LineHeight + ink.Bottom*fit.Size
			steps = append(steps, step{slot: slot, height: height, gap: pending})
			pending, drawn = 0, true
			out.Over = out.Over || fit.Over
		case it.Rule != nil:
			w, h, alpha := it.Rule.Box()
			steps = append(steps, step{rule: &PlacedRegionRule{Kind: it.Rule.Kind, Box: Bounds{Width: w, Height: h}, Alpha: alpha}, height: h, gap: pending})
			pending = 0
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
		if s.slot != nil {
			t := s.slot.Type
			ink := InkOf(t.Face, t.Weight)
			widest := 0.0
			for k := range s.slot.Lines {
				s.slot.Lines[k].Baseline = y + ink.Top*t.Size + float64(k)*t.Size*t.LineHeight
				widest = math.Max(widest, s.slot.Lines[k].Width)
			}
			s.slot.Box = Bounds{X: left(widest), Y: y, Width: widest, Height: s.height}
			out.Slots = append(out.Slots, *s.slot)
			out.Bounds = unionBounds(out.Bounds, s.slot.Box)
		} else {
			s.rule.Box.X, s.rule.Box.Y = left(s.rule.Box.Width), y
			out.Rules = append(out.Rules, *s.rule)
			out.Bounds = unionBounds(out.Bounds, s.rule.Box)
		}
		y += s.height
	}
	return out, nil
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
	layout, known := Layout(ratio)
	slots := preset.Slots()
	if !ok || !known || index < 0 || index >= len(slots) {
		return SlotSpec{}, 0, false
	}
	width := layout.CopyMaxWidth
	if preset.Width > 0 {
		width = preset.Width
	}
	return slots[index].Spec(ratio), width, true
}
