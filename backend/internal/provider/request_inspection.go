package provider

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

// SelectionForInspection reads the saved active identity and its current
// configured eligibility. It is neither execution admission nor live endpoint
// qualification: inspecting a free model must not fetch a provider document,
// initialize an absent choice, clear an unavailable choice or read credit balance.
// The boolean means a saved active choice exists; Missing/UnavailableReason
// independently tell the caller whether that identity can prepare a preview.
func (s *Service) SelectionForInspection(ctx context.Context, userID string, stage Stage) (Selection, bool, error) {
	if err := ctx.Err(); err != nil {
		return Selection{}, false, err
	}
	if _, err := ParseStage(string(stage)); err != nil {
		return Selection{}, false, err
	}
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return Selection{}, false, err
	}
	selections, err := s.store.ListSelections(ctx, userID)
	if err != nil {
		return Selection{}, false, fmt.Errorf("read inspection selection: %w", err)
	}
	for _, selection := range selections {
		if selection.Stage != stage || selection.Slot != "" && selection.Slot != SlotActive {
			continue
		}
		selection.Slot = SlotActive
		selection.Missing, selection.UnavailableReason, selection.RequiredPlan = false, "", ""
		info, found := s.catalog.Lookup(selection.Ref)
		if !found || !Suitable(stage, info) {
			selection.Missing = true
			return selection, true, nil
		}
		if s.modelGrades {
			access := modelAccess(tier, stage, info)
			selection.RequiredPlan, selection.UnavailableReason = access.RequiredPlan, access.UnavailableReason
		} else if info.Disabled {
			selection.UnavailableReason = "MODEL_PROVIDER_UNAVAILABLE"
		}
		return selection, true, nil
	}
	return Selection{}, false, nil
}

// ModelForInspection evaluates an explicit frozen authoring model without
// changing the active selection or checking a live endpoint. A preview may name
// the admitted operation's model even when the owner has since selected another.
func (s *Service) ModelForInspection(ctx context.Context, userID string, stage Stage, ref llm.ModelRef) (llm.ModelInfo, bool, error) {
	if err := ctx.Err(); err != nil {
		return llm.ModelInfo{}, false, err
	}
	if _, err := ParseStage(string(stage)); err != nil {
		return llm.ModelInfo{}, false, err
	}
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return llm.ModelInfo{}, false, err
	}
	info, found := s.catalog.Lookup(ref)
	if !found || !Suitable(stage, info) || info.Disabled {
		return info, false, nil
	}
	if s.modelGrades && modelAccess(tier, stage, info).UnavailableReason != "" {
		return info, false, nil
	}
	return info, true, nil
}
