package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// Service is the admission gate and the ledger writer.
type Service struct {
	store  Store
	models Models

	// now is a seam for tests in this package, not configuration: every axis is measured
	// over a calendar window, so the only way to exercise a boundary is to move the clock.
	now func() time.Time
}

func NewService(store Store, models Models) *Service {
	return &Service{store: store, models: models, now: time.Now}
}

// Admit decides whether one more piece of LLM work may start, and records the start when
// it may.
//
// The order is deliberate: authority first (a model the tier may not run is refused
// whatever the budget says), then the start count, then today's spend, then the month's.
// A refusal writes nothing at all, which is what makes the count axis honest.
//
// Enforcement is at admission because cost is only known after a call completes. The
// accepted consequence is bounded overshoot: a window may exceed its budget by the cost
// of the jobs already in flight when it filled. Killing those buys nothing — the tokens
// are already paid for — and would break the job queue's contract ([I5]).
func (s *Service) Admit(ctx context.Context, start Start) error {
	if start.UserID == "" || start.Kind == "" || start.JobID == "" {
		return fmt.Errorf("admit: user, kind and job id are required")
	}
	if !start.Plan.Valid() {
		return fmt.Errorf("admit: acting plan is unknown")
	}
	if err := plan.EnsureAllowed(start.Plan, start.Models); err != nil {
		return err
	}

	now := s.now()
	limits := plan.LimitsFor(start.Plan)
	// One transaction, because the count and the row that changes it are one decision: two
	// requests that each read "9 of 10" must not both become the tenth and the eleventh.
	return s.store.InWriteTx(ctx, func(tx Store) error {
		// Master is exempt from the numeric axes but still recorded below: unlimited spend is
		// exactly the account whose spend the operator most wants to be able to read.
		if !limits.Unlimited() {
			if err := checkAxes(ctx, tx, start.UserID, limits, now); err != nil {
				return err
			}
		}
		return tx.InsertAdmission(ctx, Admission{
			UserID: start.UserID, Kind: start.Kind, JobID: start.JobID, CreatedAt: now,
		})
	})
}

func checkAxes(ctx context.Context, store Store, userID string, limits plan.Limits, now time.Time) error {
	dayStart, dayEnd := plan.DayWindow(now)
	if limits.DailyJobStarts > 0 {
		started, err := store.CountAdmissions(ctx, userID, dayStart, dayEnd)
		if err != nil {
			return fmt.Errorf("count admissions: %w", err)
		}
		if started >= int64(limits.DailyJobStarts) {
			return &plan.QuotaError{
				Axis: plan.AxisDailyCount, Limit: int64(limits.DailyJobStarts), Used: started, ResetsAt: dayEnd,
			}
		}
	}

	if limits.DailyBudgetMicrousd > 0 {
		spent, err := store.SumCost(ctx, userID, dayStart, dayEnd)
		if err != nil {
			return fmt.Errorf("sum daily cost: %w", err)
		}
		if spent >= limits.DailyBudgetMicrousd {
			return &plan.QuotaError{
				Axis: plan.AxisDailyCost, Limit: limits.DailyBudgetMicrousd, Used: spent, ResetsAt: dayEnd,
			}
		}
	}

	if limits.MonthlyBudgetMicrousd > 0 {
		monthStart, monthEnd := plan.MonthWindow(now)
		spent, err := store.SumCost(ctx, userID, monthStart, monthEnd)
		if err != nil {
			return fmt.Errorf("sum monthly cost: %w", err)
		}
		if spent >= limits.MonthlyBudgetMicrousd {
			return &plan.QuotaError{
				Axis: plan.AxisMonthlyCost, Limit: limits.MonthlyBudgetMicrousd, Used: spent, ResetsAt: monthEnd,
			}
		}
	}

	return nil
}

// Release drops the admission for a job that was admitted but never created. Enqueue
// admits before it inserts — a refusal must leave no job row — so the rare failure
// between the two would otherwise leave the account charged for a start it never got.
func (s *Service) Release(ctx context.Context, jobID string) error {
	if jobID == "" {
		return nil
	}
	if err := s.store.DeleteAdmissionForJob(ctx, jobID); err != nil {
		return fmt.Errorf("release admission: %w", err)
	}
	return nil
}

// Record persists one completed provider call, pricing it by the shared reported →
// estimated → unavailable precedence. A model that has since left the registry still
// records its tokens; only its estimate is lost.
func (s *Service) Record(ctx context.Context, call Call) error {
	info, _ := s.models.Lookup(call.Model)
	cost := llm.ResolveCost(llm.CostInput{
		PromptTokens:        int64(call.Usage.PromptTokens),
		CompletionTokens:    int64(call.Usage.CompletionTokens),
		ReportedMicrousd:    call.Usage.CostMicrousd,
		Reported:            call.Usage.CostReported,
		InputUSDPerMillion:  info.InputUSDPerMillion,
		OutputUSDPerMillion: info.OutputUSDPerMillion,
	})

	return s.store.InsertEvent(ctx, Event{
		UserID:           call.UserID,
		Kind:             call.Kind,
		JobID:            call.JobID,
		Stage:            call.Stage,
		Model:            call.Model.String(),
		PromptTokens:     int64(call.Usage.PromptTokens),
		CompletionTokens: int64(call.Usage.CompletionTokens),
		CostMicrousd:     cost.Microusd,
		CostSource:       cost.Source,
		CreatedAt:        s.now(),
	})
}

// RecordCall writes the ledger row for one completed provider call, attributed to the
// work in context.
//
// A successful call is always recorded, even when the provider reported nothing: the row
// is the evidence the call happened. A FAILED call is recorded only when it reported
// usage — job 23 preserves that, and those tokens were bought — because a call that never
// reached a model has nothing to account for.
func (s *Service) RecordCall(ctx context.Context, ref llm.ModelRef, u llm.Usage, failed bool) error {
	work, ok := WorkFromContext(ctx)
	if !ok {
		return nil
	}
	if failed && u.PromptTokens == 0 && u.CompletionTokens == 0 && !u.CostReported {
		return nil
	}
	return s.Record(ctx, Call{
		UserID: work.UserID, Kind: work.Kind, JobID: work.JobID,
		Stage: work.StageFor(ref), Model: ref, Usage: u,
	})
}

// Summary reports the account's live position in both windows.
func (s *Service) Summary(ctx context.Context, userID string) (Summary, error) {
	now := s.now()
	dayStart, dayEnd := plan.DayWindow(now)
	monthStart, monthEnd := plan.MonthWindow(now)

	started, err := s.store.CountAdmissions(ctx, userID, dayStart, dayEnd)
	if err != nil {
		return Summary{}, fmt.Errorf("count admissions: %w", err)
	}
	today, err := s.store.SumCost(ctx, userID, dayStart, dayEnd)
	if err != nil {
		return Summary{}, fmt.Errorf("sum daily cost: %w", err)
	}
	month, err := s.store.SumCost(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return Summary{}, fmt.Errorf("sum monthly cost: %w", err)
	}

	return Summary{
		JobsStartedToday:  int(started),
		CostTodayMicrousd: today,
		CostMonthMicrousd: month,
		DayResetsAt:       dayEnd,
		MonthResetsAt:     monthEnd,
	}, nil
}
