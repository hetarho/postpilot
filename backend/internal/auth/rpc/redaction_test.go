package rpc

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
)

func redactedThrough(t *testing.T, acting plan.Plan, response any) any {
	t.Helper()
	ctx := context.Background()
	if acting != "" {
		ctx = auth.WithActor(ctx, auth.Actor{UserID: "someone", Plan: acting})
	}
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		switch msg := response.(type) {
		case *postpilotv1.GetGenerationResponse:
			return connect.NewResponse(msg), nil
		case *postpilotv1.ListExperimentsResponse:
			return connect.NewResponse(msg), nil
		}
		t.Fatalf("unexpected response type %T", response)
		return nil, nil
	}
	res, err := NewSupplierRedaction().WrapUnary(next)(ctx, connect.NewRequest(&postpilotv1.GetGenerationRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Any()
}

// QUOTA-66: provider prose inside any failure, however deep, never reaches a non-master; the
// reason and its params, which the product renders its own copy from, stay.
func TestSupplierRedactionClearsProviderProseForNonMasters(t *testing.T) {
	generation := func() *postpilotv1.GetGenerationResponse {
		return &postpilotv1.GetGenerationResponse{Job: &postpilotv1.GenerationJob{Id: "job", Failure: &postpilotv1.Failure{
			Reason: "MODEL_UNAVAILABLE", Params: map[string]string{"model": "vendor/x"}, TechnicalDetail: "402: can only afford 1200 tokens",
		}}}
	}
	experiments := func() *postpilotv1.ListExperimentsResponse {
		return &postpilotv1.ListExperimentsResponse{Experiments: []*postpilotv1.ModelExperiment{{
			Id: "exp", AdoptionFailure: &postpilotv1.Failure{Reason: "MODEL_UNAVAILABLE", TechnicalDetail: "quota exceeded for key"},
			Candidates: []*postpilotv1.ExperimentCandidate{{Id: "a", Failure: &postpilotv1.Failure{Reason: "MODEL_RATE_LIMITED", TechnicalDetail: "balance 0.02"}}},
		}}}
	}

	for _, acting := range []plan.Plan{plan.Free, plan.Max, ""} {
		job := redactedThrough(t, acting, generation()).(*postpilotv1.GetGenerationResponse).GetJob().GetFailure()
		if job.GetTechnicalDetail() != "" || job.GetReason() != "MODEL_UNAVAILABLE" || job.GetParams()["model"] != "vendor/x" {
			t.Fatalf("%q job failure = %+v", acting, job)
		}
		exp := redactedThrough(t, acting, experiments()).(*postpilotv1.ListExperimentsResponse).GetExperiments()[0]
		if exp.GetAdoptionFailure().GetTechnicalDetail() != "" || exp.GetCandidates()[0].GetFailure().GetTechnicalDetail() != "" ||
			exp.GetCandidates()[0].GetFailure().GetReason() != "MODEL_RATE_LIMITED" {
			t.Fatalf("%q experiment failures = %+v", acting, exp)
		}
	}

	job := redactedThrough(t, plan.Master, generation()).(*postpilotv1.GetGenerationResponse).GetJob().GetFailure()
	if job.GetTechnicalDetail() == "" {
		t.Fatal("the operator lost the provider detail it diagnoses from")
	}
}

func TestSupplierRedactionPassesErrorsThrough(t *testing.T) {
	want := errors.New("boom")
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, want }
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "someone", Plan: plan.Free})
	if _, err := NewSupplierRedaction().WrapUnary(next)(ctx, connect.NewRequest(&postpilotv1.GetGenerationRequest{})); !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

// costPattern names every field that could carry supplier cost or the credit conversion
// (QUOTA-65, QUOTA-66). A match is either not cost (the reason says what it is) or a field
// only the operator's copy populates (the reason names the test that pins it).
var costPattern = regexp.MustCompile(`(?i)(usd|microusd|cost|price|pricing|_e4$|^rate$|fx_rate|technical_detail|temporary)`)

var costFieldAllowlist = map[string]string{
	"postpilot.v1.CreditPack.price_krw":                     "fixed KRW retail pack price (QUOTA-34), not supplier cost",
	"postpilot.v1.ModelInfo.ai_price_unavailable":           "a bool that says a price is missing, carries no value",
	"postpilot.v1.Failure.technical_detail":                 "cleared for non-masters at the response edge: TestSupplierRedactionClearsProviderProseForNonMasters",
	"postpilot.v1.QuoteClipGenerationResponse.priced_calls": "per call: label, model, stage, count and token budgets; its USD price fields are reserved",
	"postpilot.v1.QuoteClipRevisionResponse.priced_calls":   "per call: label, model, stage, count and token budgets; its USD price fields are reserved",
}

// QUOTA-66: every procedure a non-master can call is walked, response message by response
// message. A cost-like field that is neither explained nor proven operator-only is a leak;
// a field that no longer exists is a stale allowlist entry.
func TestNoProcedureOutsideMasterCarriesUnexplainedCostFields(t *testing.T) {
	seen := map[protoreflect.FullName]bool{}
	var found []string
	var walk func(protoreflect.MessageDescriptor)
	walk = func(message protoreflect.MessageDescriptor) {
		if seen[message.FullName()] {
			return
		}
		seen[message.FullName()] = true
		fields := message.Fields()
		for i := 0; i < fields.Len(); i++ {
			field := fields.Get(i)
			if costPattern.MatchString(string(field.Name())) {
				found = append(found, string(message.FullName())+"."+string(field.Name()))
			}
			switch {
			case field.IsMap():
				if value := field.MapValue(); value.Message() != nil {
					walk(value.Message())
				}
			case field.Message() != nil:
				walk(field.Message())
			}
		}
	}
	procedures := 0
	protoregistry.GlobalFiles.RangeFilesByPackage("postpilot.v1", func(file protoreflect.FileDescriptor) bool {
		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			methods := service.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				if masterProcedures["/"+string(service.FullName())+"/"+string(method.Name())] {
					continue
				}
				procedures++
				walk(method.Output())
			}
		}
		return true
	})
	if procedures == 0 {
		t.Fatal("no procedures walked; the generated descriptors were not registered")
	}
	var leaks []string
	matched := map[string]bool{}
	for _, name := range found {
		matched[name] = true
		if _, ok := costFieldAllowlist[name]; !ok {
			leaks = append(leaks, name)
		}
	}
	sort.Strings(leaks)
	if len(leaks) > 0 {
		t.Errorf("cost-like fields reachable by a non-master with no reason on record:\n  %s", strings.Join(leaks, "\n  "))
	}
	var stale []string
	for name := range costFieldAllowlist {
		if !matched[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("allowlist entries no non-master response reaches any more:\n  %s", strings.Join(stale, "\n  "))
	}
}
