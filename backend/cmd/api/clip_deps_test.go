package main

import (
	"context"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// Neutral collaborators for the clip constructors (ARCH-40): each answers the way the
// context refused before the collaborator existed, so a test that cares about one
// replaces it.
type neutralFinisher struct{}

func (neutralFinisher) Complete(context.Context, clip.AttemptResult) error {
	return clip.ErrCompositionUnavailable
}
func (neutralFinisher) Recover(context.Context) error { return nil }

type neutralPricing struct{}

func (neutralPricing) Freeze(context.Context, llm.ModelRef, llm.ModelRef, int) (clip.GenerationPricing, error) {
	return clip.GenerationPricing{}, clip.ErrPricingUnavailable
}

type neutralAccounting struct{}

func (neutralAccounting) ForJob(context.Context, string, string) (*clip.Accounting, error) {
	return nil, nil
}

type neutralAdmission struct{}

func (neutralAdmission) ObserveModels() []clip.AnalysisCandidate { return nil }
func (neutralAdmission) QualifyObserve(context.Context, llm.ModelRef) error {
	return clip.ErrPricingUnavailable
}

type nullFinalizer struct{}

func (nullFinalizer) Finalize(context.Context, clip.FinalizationRequest) (clip.Project, error) {
	return clip.Project{}, clip.ErrFinalizationInvalid
}

type nullObjects struct{}

func (nullObjects) PresignSource(context.Context, string, string, time.Duration) (clip.SignedSourcePut, error) {
	return clip.SignedSourcePut{}, nil
}
func (nullObjects) PresignSourcePlayback(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (nullObjects) HeadSource(context.Context, string) (clip.SourceObjectInfo, error) {
	return clip.SourceObjectInfo{}, nil
}
func (nullObjects) Delete(context.Context, string) error             { return nil }
func (nullObjects) ListSourceKeys(context.Context) ([]string, error) { return nil, nil }

func neutralGenerationDeps() clipapp.GenerationDeps { return generationDeps(nil, nil, nil) }

// generationDeps fills the collaborators a test leaves nil with the neutral ones.
func generationDeps(f clip.ClipFinisher, p clip.QuotePricing, a clip.AccountingReader) clipapp.GenerationDeps {
	if f == nil {
		f = neutralFinisher{}
	}
	if p == nil {
		p = neutralPricing{}
	}
	if a == nil {
		a = neutralAccounting{}
	}
	return clipapp.GenerationDeps{Finisher: f, Pricing: p, Accounting: a, Admission: neutralAdmission{}}
}
