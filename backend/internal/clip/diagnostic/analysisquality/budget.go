package analysisquality

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
)

// Approval is an explicitly approved cumulative session, including every
// underlying completion/correction. A nil cap is absent, not a free allowance.
// This primitive does not grant registry/account access; the CLI has no live
// adapter. A future live composition must supply trusted local admission before
// constructing its network-capable Models factory.
type Approval struct {
	SessionID       string         `json:"sessionId"`
	StatePath       string         `json:"statePath"`
	ApprovedBy      string         `json:"approvedBy"`
	EvidenceDigest  string         `json:"evidenceDigest"`
	PlanDigest      string         `json:"planDigest"`
	PolicyDigest    string         `json:"policyDigest"`
	ExpiresAt       time.Time      `json:"expiresAt"`
	MaximumMicrousd *int64         `json:"maximumMicrousd"`
	Calls           []ApprovedCall `json:"calls"`
}
type ApprovedCall struct {
	ID            string `json:"id"`
	RequestDigest string `json:"requestDigest"`
}
type BudgetCall struct {
	ID           string    `json:"id"`
	UpperBound   int64     `json:"upperBound"`
	Usage        llm.Usage `json:"usage"`
	MeasuredCost *int64    `json:"measuredCost"`
	Status       string    `json:"status"`
}
type BudgetState struct {
	Version                int          `json:"version"`
	ApprovalDigest         string       `json:"approvalDigest"`
	ConfirmedMeasuredSpend int64        `json:"confirmedMeasuredSpend"`
	UnresolvedWorstCase    int64        `json:"unresolvedWorstCase"`
	Next                   int          `json:"next"`
	Inflight               *BudgetCall  `json:"inflight,omitempty"`
	Calls                  []BudgetCall `json:"calls"`
	Halted                 bool         `json:"halted"`
}
type BudgetedModels struct {
	mu                  sync.Mutex
	models              ai.Models
	policy              llm.CallPolicy
	approval            Approval
	state               BudgetState
	statePath, lockPath string
	lock                *os.File
	closed              bool
	halted              bool
	now                 func() time.Time
	persist             func(BudgetState) error
}

func validUsage(u llm.Usage) bool {
	return u.PromptTokens >= 0 && u.PromptTokens <= 10_000_000 && u.CompletionTokens >= 0 && u.CompletionTokens <= 10_000_000 && u.ReasoningTokens >= 0 && u.ReasoningTokens <= u.CompletionTokens && u.CostMicrousd >= 0 && (u.CostReported || u.CostMicrousd == 0)
}

// RequestDigest binds the exact production request, including runtime video
// bounds. Source/artifact identity is additionally bound by Approval.PlanDigest.
func RequestDigest(req llm.Request) string {
	type part struct {
		Text            string
		MIME            string
		Bytes, Duration int64
		Sampling        llm.VideoSampling
	}
	type message struct {
		Role  llm.Role
		Parts []part
	}
	var messages []message
	for _, m := range req.Messages {
		x := message{Role: m.Role}
		for _, p := range m.Parts {
			q := part{Text: p.Text}
			if p.InlineVideo != nil {
				q.MIME = p.InlineVideo.MIME
				q.Bytes = p.InlineVideo.Size
				q.Duration = p.InlineVideo.DurationMS
				q.Sampling = p.InlineVideo.Sampling
			}
			x.Parts = append(x.Parts, q)
		}
		messages = append(messages, x)
	}
	return digest(struct {
		System           string
		Messages         []message
		Schema           []byte
		Stage            string
		MaxTokens        int
		Reasoning        llm.ReasoningEffort
		DisableReasoning bool
		Execution        *llm.ExecutionPolicy
	}{req.System, messages, req.JSONSchema, req.Stage, req.MaxTokens, req.Reasoning, req.DisableReasoning, req.Execution})
}

// OpenBudgetedModels checks approval, durable state and an exclusive session
// lock before trusted local admission and before factory construction. Missing
// admission, stale/in-flight state or persistence failure constructs nothing.
func OpenBudgetedModels(ctx context.Context, statePath string, a Approval, p llm.CallPolicy, now time.Time, localAdmission func(context.Context, Approval) error, factory func() (ai.Models, error)) (*BudgetedModels, error) {
	// Neither caller-owned slices/pointers nor a callback may mutate the frozen
	// request matrix/cap after admission or its durable digest was recorded.
	a = copyApproval(a)
	upper, ok := p.QuoteMicrousd()
	if ctx.Err() != nil || !ok || !p.Pricing.Valid() || !labelID.MatchString(a.SessionID) || !text(a.ApprovedBy, 1, 128) || !clip.ValidSHA256(a.EvidenceDigest) || !clip.ValidSHA256(a.PlanDigest) || a.PolicyDigest != digest(p) || a.MaximumMicrousd == nil || *a.MaximumMicrousd < 0 || !a.ExpiresAt.After(now) || len(a.Calls) < 1 || len(a.Calls) > MaxInputs*MaxReplicates || localAdmission == nil || factory == nil {
		return nil, ErrBudget
	}
	seen := map[string]bool{}
	for _, call := range a.Calls {
		if !labelID.MatchString(call.ID) || seen[call.ID] || !clip.ValidSHA256(call.RequestDigest) {
			return nil, ErrBudget
		}
		seen[call.ID] = true
	}
	// The whole predefined matrix must fit; planned repetitions are not free
	// retries. Checked arithmetic includes an explicitly approved zero-price cap.
	if upper > 0 && int64(len(a.Calls)) > *a.MaximumMicrousd/upper {
		return nil, ErrBudget
	}
	absolute, e := filepath.Abs(statePath)
	if e != nil || absolute != a.StatePath {
		return nil, ErrBudget
	}
	parent := filepath.Dir(absolute)
	real, e := filepath.EvalSymlinks(parent)
	stat, e2 := os.Stat(parent)
	if e != nil || e2 != nil || real != parent || !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
		return nil, ErrBudget
	}
	b := &BudgetedModels{policy: p, approval: a, statePath: absolute, lockPath: absolute + ".lock", now: time.Now}
	b.lock, e = os.OpenFile(b.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, ErrBudget
	}
	okay := false
	defer func() {
		if !okay {
			b.Close()
		}
	}()
	b.persist = b.writeState
	b.state = BudgetState{Version: Version, ApprovalDigest: digest(a)}
	if existing, e := os.Lstat(absolute); e == nil {
		if !existing.Mode().IsRegular() || existing.Mode().Perm()&0077 != 0 || existing.Size() > MaxDocumentBytes {
			return nil, ErrBudget
		}
		data, e := os.ReadFile(absolute)
		if e != nil || strict(data, &b.state, MaxDocumentBytes) != nil || b.state.Version != Version || b.state.ApprovalDigest != digest(a) || b.state.Inflight != nil || b.state.Halted || b.state.Next < 0 || b.state.Next > len(a.Calls) || b.state.Next != len(b.state.Calls) || b.state.ConfirmedMeasuredSpend < 0 || b.state.UnresolvedWorstCase != 0 || b.state.ConfirmedMeasuredSpend > *a.MaximumMicrousd {
			return nil, ErrUncertain
		}
		var confirmed int64
		for i, call := range b.state.Calls {
			if call.ID != a.Calls[i].ID || !validUsage(call.Usage) || !call.Usage.CostReported || call.MeasuredCost == nil || *call.MeasuredCost != call.Usage.CostMicrousd || call.Status != "completed" || call.UpperBound != upper || call.Usage.CostMicrousd > upper || confirmed > math.MaxInt64-call.Usage.CostMicrousd {
				return nil, ErrUncertain
			}
			confirmed += call.Usage.CostMicrousd
		}
		if confirmed != b.state.ConfirmedMeasuredSpend {
			return nil, ErrUncertain
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, ErrBudget
	}
	if b.state.Next >= len(a.Calls) || upper > *a.MaximumMicrousd-b.state.ConfirmedMeasuredSpend {
		return nil, ErrBudget
	}
	if b.persist(b.state) != nil {
		return nil, ErrOutput
	}
	if e = localAdmission(ctx, copyApproval(a)); e != nil {
		return nil, safeError(e)
	}
	if ctx.Err() != nil {
		return nil, ErrBudget
	}
	b.models, e = factory()
	if e != nil || b.models == nil {
		return nil, ErrLive
	}
	okay = true
	return b, nil
}

func copyApproval(a Approval) Approval {
	a.Calls = append([]ApprovedCall(nil), a.Calls...)
	if a.MaximumMicrousd != nil {
		maximum := *a.MaximumMicrousd
		a.MaximumMicrousd = &maximum
	}
	return a
}

func (b *BudgetedModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.halted || b.state.Halted || ref != b.policy.Ref {
		return llm.ModelInfo{}, false
	}
	return b.models.Resolve(ref)
}

func (b *BudgetedModels) Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	upper, ok := b.policy.QuoteMicrousd()
	if ctx.Err() != nil || b.closed || b.halted || b.state.Halted || b.state.Inflight != nil || !ok || !b.approval.ExpiresAt.After(b.now()) || b.state.Next >= len(b.approval.Calls) || ref != b.policy.Ref || req.Execution == nil || req.Execution.Call != b.policy || !req.Execution.Matches(ref, req) || RequestDigest(req) != b.approval.Calls[b.state.Next].RequestDigest || b.state.UnresolvedWorstCase < 0 || b.state.ConfirmedMeasuredSpend > *b.approval.MaximumMicrousd-b.state.UnresolvedWorstCase || upper > *b.approval.MaximumMicrousd-b.state.ConfirmedMeasuredSpend-b.state.UnresolvedWorstCase {
		return llm.Response{}, ErrBudget
	}
	// A removed/replaced lock cannot authorize another metadata GET or POST.
	lockInfo, e := b.lock.Stat()
	pathInfo, e2 := os.Lstat(b.lockPath)
	if e != nil || e2 != nil || !os.SameFile(lockInfo, pathInfo) {
		b.halted = true
		return llm.Response{}, ErrBudget
	}
	call := BudgetCall{ID: b.approval.Calls[b.state.Next].ID, UpperBound: upper, Status: "inflight"}
	b.state.Inflight = &call
	b.state.UnresolvedWorstCase = upper
	if b.persist(b.state) != nil {
		b.halted = true
		return llm.Response{}, ErrOutput
	}
	response, callErr := b.models.Complete(ctx, ref, req)
	call.Usage = response.Usage
	call.Status = "completed"
	if !validUsage(response.Usage) || !response.Usage.CostReported || response.Usage.CostMicrousd > math.MaxInt64-b.state.ConfirmedMeasuredSpend {
		call.Status = "usage_uncertain"
		b.state.Halted = true
		b.halted = true
	} else {
		cost := response.Usage.CostMicrousd
		call.MeasuredCost = &cost
		b.state.ConfirmedMeasuredSpend += cost
		b.state.UnresolvedWorstCase = 0
		if cost > upper || b.state.ConfirmedMeasuredSpend > *b.approval.MaximumMicrousd || response.Usage.PromptTokens > b.policy.InputTokenLimit() || response.Usage.CompletionTokens > b.policy.CompletionTokens {
			call.Status = "ceiling_exceeded"
			b.state.Halted = true
			b.halted = true
		}
	}
	if callErr != nil {
		if call.Status == "completed" {
			call.Status = "provider_failed"
		}
		b.state.Halted = true
		b.halted = true
	}
	b.state.Calls = append(b.state.Calls, call)
	b.state.Next++
	if call.Status == "usage_uncertain" {
		b.state.Inflight = &call
	} else {
		b.state.Inflight = nil
	}
	if b.persist(b.state) != nil {
		b.halted = true
		return response, ErrOutput
	}
	if callErr != nil {
		return response, callErr
	}
	if b.halted {
		return response, ErrUncertain
	}
	return response, nil
}

func (b *BudgetedModels) writeState(s BudgetState) error {
	data, e := json.Marshal(s)
	if e != nil || len(data) > MaxDocumentBytes {
		return ErrOutput
	}
	f, e := os.CreateTemp(filepath.Dir(b.statePath), ".analysis-budget-")
	if e != nil {
		return ErrOutput
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil || os.Rename(f.Name(), b.statePath) != nil {
		return ErrOutput
	}
	dir, e := os.Open(filepath.Dir(b.statePath))
	if e != nil {
		return ErrOutput
	}
	defer dir.Close()
	if dir.Sync() != nil {
		return ErrOutput
	}
	return nil
}

func (b *BudgetedModels) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	if b.lock == nil {
		return nil
	}
	opened, e := b.lock.Stat()
	current, e2 := os.Lstat(b.lockPath)
	closeErr := b.lock.Close()
	if e == nil && e2 == nil && os.SameFile(opened, current) {
		return errors.Join(closeErr, os.Remove(b.lockPath))
	}
	return closeErr
}
