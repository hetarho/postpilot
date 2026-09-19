package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// RevisionQuoteStore is what a revision needs beyond the shared quote store: a
// quote whose digest binds the saved plan, and a link that consumes it.
// RevisionPlanStore saves what a revision wrote.
type RevisionPlanStore interface {
	SaveRevisedPlan(ctx context.Context, user, project, job string, revision int, raw string) (Project, error)
}

type RevisionQuoteStore interface {
	SaveRevisionQuote(ctx context.Context, quote GenerationQuote, now time.Time) error
	LinkRevisionJob(ctx context.Context, quote GenerationQuote, job string, now time.Time) error
}

// RevisionInputDigest binds a quote to the exact plan the owner was looking at
// and the exact words they wrote. A plan saved after the quote — their own edit,
// or another revision — invalidates it rather than rewriting something they
// never approved (CLIP-131).
func RevisionInputDigest(p Project, request, target, write string, pricing GenerationPricing) string {
	input := struct {
		User, Project, Write   string
		Revision               int
		Request, Target        string
		PlanJSON               string
		Pricing                GenerationPricing
		Composition            *ProjectComposition
		Instruction            string
		CaptionPace, Accent    string
		Disclosure, CTA, Ratio string
		HideDisclosure         bool
		Target_                int
		Answers                []Answer
	}{p.UserID, p.ID, write, p.EditPlanRevision, request, target, p.EditPlan, pricing, p.Composition,
		p.Instruction, p.CaptionPace, p.Accent, p.Disclosure, p.CTA, p.Ratio, p.HideDisclosure, p.TargetDurationMS, p.Answers}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
