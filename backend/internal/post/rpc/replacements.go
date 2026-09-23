package rpc

import (
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

// replacementSurfaceToProto maps a stored surface onto the wire's closed enum. Anything else
// is no value at all — UNSPECIFIED and false — never a default (ARCH-3). Nothing inbound
// carries a candidate, so there is no from-proto half.
func replacementSurfaceToProto(surface post.ReplacementSurface) (postpilotv1.ReplacementSurface, bool) {
	switch surface {
	case post.ReplacementSurfaceTitle:
		return postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_TITLE, true
	case post.ReplacementSurfaceTag:
		return postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_TAG, true
	case post.ReplacementSurfaceBody:
		return postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_BODY, true
	}
	return postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_UNSPECIFIED, false
}

// protoReplacementCandidates carries the stored candidates in their stored order. A surface the
// wire cannot name is dropped rather than sent as UNSPECIFIED; the service refuses one on
// write, so it cannot be stored.
func protoReplacementCandidates(candidates []post.ReplacementCandidate) []*postpilotv1.ReplacementCandidate {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]*postpilotv1.ReplacementCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		surface, ok := replacementSurfaceToProto(candidate.Surface)
		if !ok {
			continue
		}
		out = append(out, &postpilotv1.ReplacementCandidate{
			Surface: surface, Index: int32(candidate.Index), Source: candidate.Source,
			Phrases: append([]string(nil), candidate.Phrases...),
		})
	}
	return out
}
