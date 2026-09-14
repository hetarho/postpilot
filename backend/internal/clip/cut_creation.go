package clip

import (
	"regexp"
	"slices"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// The two operations an owner may use to create footage (CLIP-98). Neither
// creates meaning: ADD selects more of an observed scene and SPLIT divides a
// cut the server already approved.
const (
	CutAdd   = "add"
	CutSplit = "split"
)

// CutCreation is one-time, request-only provenance. It authorizes an id the
// plan does not yet contain and is never written to the stored plan, so a cut
// cannot claim to have been created twice.
type CutCreation struct {
	Kind     string
	OriginID string
}

// ownerCutID is the ONE shape a cut the owner created may take: the literal
// prefix and a version-4 UUID. A client cannot mint an id that looks like one
// this generation produced, and a canonical shape is what lets the identity
// history tell an owner cut from a forged one.
var ownerCutID = regexp.MustCompile(`^owner-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidOwnerCutID reports whether an id is a canonical owner-created cut id.
func ValidOwnerCutID(id string) bool { return ownerCutID.MatchString(id) }

// CutError names the exact cut a refusal belongs to, so the editor can point at
// the cut the owner touched instead of saying the plan is invalid (CDS-64).
type CutError struct {
	planViolation
	CutID string
}

func (e *CutError) Unwrap() error { return ErrInvalid }
func cutRefusal(id, reason string) error {
	return &CutError{planViolation(reason), id}
}

// observedScene is the retained observation that contains a source range whole.
// A cut may only be created inside ONE observed scene: footage nobody looked at
// is not evidence, and a range spanning two scenes is a claim about neither.
func observedScene(observations []SourceAnalysis, sourceID, fingerprint string, startMS, endMS int) (Segment, bool) {
	for _, a := range observations {
		if a.Source.ID != sourceID || a.Source.Fingerprint != fingerprint {
			continue
		}
		for _, segment := range a.Segments {
			if segment.StartMS <= startMS && endMS <= segment.EndMS {
				return segment, true
			}
		}
	}
	return Segment{}, false
}

// ObservedCutFocal is the canonical initial crop for owner-added footage.
func ObservedCutFocal(scene Segment) Point {
	if !normalized(scene.Focal.X) || !normalized(scene.Focal.Y) || scene.Focal == (Point{}) {
		return Point{X: .5, Y: .5}
	}
	return scene.Focal
}

// ownerItemBinding re-derives the group and item a NEW cut belongs to from the
// owner's own associations, never from anything the client asserted. Exactly one
// answer binds it; none or several leave it unassigned, because an uncertain
// association is not an association (CLIP-62).
func ownerItemBinding(associations []SourceAssociation, sourceID, fingerprint string, startMS, endMS int) (string, string) {
	group, item := "", ""
	for _, a := range associations {
		if a.SourceID != sourceID || a.Fingerprint != fingerprint || a.StartMS >= endMS || a.EndMS <= startMS {
			continue
		}
		if group != "" && (group != a.GroupID || item != a.ItemID) {
			return "", ""
		}
		group, item = a.GroupID, a.ItemID
	}
	return group, item
}

// admitOwnerCut validates one submitted cut the plan does not yet contain and
// returns the cut and its portable binding, with every server-owned value
// normalized. Nothing the client sent about focal, audio, binding, rate or
// content survives: this creates footage, not a new claim (CLIP-97).
func admitOwnerCut(c CorrectionCut, origin Cut, binding composition.Cut, portable *PortablePlan, submitted []CorrectionCut) (Cut, composition.Cut, error) {
	if !ValidOwnerCutID(c.ID) {
		return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_identity")
	}
	out := Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS,
		// A created cut always leads in with a hard cut and plays at 1x: a
		// transition and a transform are edits the owner makes afterwards.
		TransitionMS: 0, PlaybackRatePermille: RateUnitPermille}
	full := 1.0
	out.Volume = &full
	next := binding
	next.ID, next.StartMS, next.EndMS, next.TransitionMS, next.PlaybackRatePermille = c.ID, c.StartMS, c.EndMS, 0, RateUnitPermille
	switch c.Creation.Kind {
	case CutSplit:
		// The right side of a split is the parent's own footage, continued: it
		// keeps every property the parent already had, and carries no text.
		out.SourceID, out.Fingerprint = origin.SourceID, origin.Fingerprint
		out.Focal, out.Volume, out.PlaybackRatePermille = origin.Focal, origin.Volume, origin.Rate()
		next.SourceID, next.PlaybackRatePermille = origin.SourceID, origin.Rate()
		// An interior timestamp of the parent's ORIGINAL range, and the parent's
		// own end: a split divides footage, it never extends or moves it.
		left, ok := submittedCut(submitted, origin.ID)
		if !ok || out.StartMS <= origin.StartMS || out.StartMS >= origin.EndMS || out.EndMS != origin.EndMS || left.EndMS != out.StartMS || left.StartMS != origin.StartMS {
			return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_split_point")
		}
		if c.SourceID != "" && c.SourceID != origin.SourceID || c.Fingerprint != "" && c.Fingerprint != origin.Fingerprint {
			return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_source")
		}
	case CutAdd:
		if c.SourceID == "" || c.Fingerprint == "" {
			return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_source")
		}
		scene, ok := observedScene(portable.Observations, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS)
		if c.StartMS < 0 || c.EndMS <= c.StartMS || !ok {
			return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_observation")
		}
		// The focal point is the one the observation recorded, not one a client
		// chose; an observation that recorded none centres the frame.
		out.Focal = ObservedCutFocal(scene)
		// The new cut takes its place in the template from the origin and its
		// item from the owner's own associations, re-derived for this scene.
		next.SourceID = c.SourceID
		next.GroupID, next.ItemID = ownerItemBinding(portable.Inputs.Associations, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS)
	default:
		return Cut{}, composition.Cut{}, cutRefusal(c.ID, "cut_creation_kind")
	}
	return out, next, nil
}

func submittedCut(cuts []CorrectionCut, id string) (CorrectionCut, bool) {
	for _, c := range cuts {
		if c.ID == id {
			return c, true
		}
	}
	return CorrectionCut{}, false
}

// ValidateOwnerCutContent refuses text on a cut the owner just created. A split
// keeps the parent's cut-bound content on the LEFT, where it was authored, and
// an added cut shows footage until the owner writes something for it.
func ValidateOwnerCutContent(created []string, elements []CorrectionText) error {
	for _, t := range elements {
		if t.Basis == "cut" && slices.Contains(created, t.CutID) {
			return cutRefusal(t.CutID, "cut_created_content")
		}
	}
	return nil
}

// ValidCreatedTransition is the one transition a created cut may carry.
func ValidCreatedTransition(ms int) bool { return ms == 0 || ms == design.Transition.FadeMS }
