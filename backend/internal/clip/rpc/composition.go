package rpc

import (
	"context"
	"maps"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func (h *Handler) GetClipCapabilities(ctx context.Context, _ *connect.Request[v1.GetClipCapabilitiesRequest]) (*connect.Response[v1.GetClipCapabilitiesResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetClipCapabilitiesResponse{CompositionVersion: clip.CompositionVersion, CompositionPlanVersion: int32(h.service.CompositionCapability())}), nil
}

func compositionInputs(v *v1.ClipCompositionInputs) *clip.CompositionInputs {
	if v == nil {
		return nil
	}
	out := &clip.CompositionInputs{Values: maps.Clone(v.Values), Items: map[string][]composition.Item{}}
	for group, items := range v.Items {
		out.Items[group] = []composition.Item{}
		for _, item := range items.GetItems() {
			out.Items[group] = append(out.Items[group], composition.Item{ID: item.GetId(), Values: maps.Clone(item.GetValues())})
		}
	}
	for _, a := range v.Associations {
		out.Associations = append(out.Associations, clip.SourceAssociation{GroupID: a.GetGroupId(), ItemID: a.GetItemId(), SourceID: a.GetSourceId(), Fingerprint: a.GetFingerprint(), StartMS: int(a.GetStartMs()), EndMS: int(a.GetEndMs())})
	}
	return out
}

func compositionProto(c *clip.ProjectComposition) *v1.ClipProjectComposition {
	if c == nil {
		return nil
	}
	in := &v1.ClipCompositionInputs{Values: maps.Clone(c.Inputs.Values), Items: map[string]*v1.ClipCompositionItems{}}
	for group, items := range c.Inputs.Items {
		values := &v1.ClipCompositionItems{}
		for _, item := range items {
			values.Items = append(values.Items, &v1.ClipCompositionItem{Id: item.ID, Values: maps.Clone(item.Values)})
		}
		in.Items[group] = values
	}
	for _, a := range c.Inputs.Associations {
		in.Associations = append(in.Associations, &v1.ClipSourceAssociation{GroupId: a.GroupID, ItemId: a.ItemID, SourceId: a.SourceID, Fingerprint: a.Fingerprint, StartMs: int32(a.StartMS), EndMs: int32(a.EndMS)})
	}
	return &v1.ClipProjectComposition{Snapshot: &v1.ClipCompositionSnapshot{Version: int32(c.Snapshot.Version), Body: c.Snapshot.Body, TemplateId: c.Snapshot.TemplateID, Legacy: c.Snapshot.Legacy}, Inputs: in}
}
