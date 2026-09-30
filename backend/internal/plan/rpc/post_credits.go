package rpc

import (
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
)

// postCreditsBasis is the one mirror of the wire's PostCreditsBasis (ARCH-3): every basis the
// product computes has a wire value, pinned by a test that walks the generated enum.
var postCreditsBasis = map[plan.PostCreditsBasis]postpilotv1.PostCreditsBasis{
	plan.PostCreditsRecentUsage: postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_RECENT_USAGE,
	plan.PostCreditsEstimate:    postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_ESTIMATE,
}

// PostCreditsBasisToProto names where a per-post figure came from on the wire (QUOTA-64). An
// unknown basis is UNSPECIFIED, which a client shows no figure for.
func PostCreditsBasisToProto(basis plan.PostCreditsBasis) postpilotv1.PostCreditsBasis {
	return postCreditsBasis[basis]
}
