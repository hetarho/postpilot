package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type speechRPCFixture struct {
	calls int
	owner string
	err   error
}

func (f *speechRPCFixture) QualificationSpeechChoices(ctx context.Context, owner string, tier plan.Plan, session string) ([]modelcatalog.SpeechChoice, error) {
	f.owner = owner
	return f.SpeechChoices(ctx, tier)
}

func (f *speechRPCFixture) SpeechChoices(context.Context, plan.Plan) ([]modelcatalog.SpeechChoice, error) {
	f.calls++
	return []modelcatalog.SpeechChoice{{ID: "profile", Revision: 1, Label: "Korean", Design: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, DesignLabel: "Design", Synthesis: llm.ModelRef{ProviderID: "speech", ModelID: "synth"}, SynthesisLabel: "Synth", Level: modelcatalog.LevelValue, RequiredPlan: plan.Light, Entitled: true, Available: true, VoiceReady: true, DescriptionMax: 1000, PreviewMin: 100, PreviewMax: 1000, SpeechMax: 1000}}, f.err
}
func (f *speechRPCFixture) BrowseSpeech(context.Context, bool) (modelcatalog.SpeechAdminBrowse, error) {
	f.calls++
	return modelcatalog.SpeechAdminBrowse{}, f.err
}
func (f *speechRPCFixture) SaveSpeechProfile(_ context.Context, p modelcatalog.SpeechProfile, _ int64) (modelcatalog.SpeechProfile, error) {
	f.calls++
	return p, f.err
}
func (f *speechRPCFixture) StartSpeechQualification(_ context.Context, owner, id string, rev int64, max string) (modelcatalog.SpeechQualificationSession, error) {
	f.calls++
	f.owner = owner
	return modelcatalog.SpeechQualificationSession{ID: "session", OwnerID: owner, ProfileID: id, Revision: rev, MaximumUSD: max}, f.err
}

func TestSpeechCustomerProjectionHasNoSupplierOrAccountFieldsForAnyPlan(t *testing.T) {
	for _, tier := range []plan.Plan{plan.Light, plan.Master} {
		ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "owner", Plan: tier})
		fixture := &speechRPCFixture{}
		h := NewSpeechHandler(fixture)
		response, err := h.ListSpeechProfiles(ctx, connect.NewRequest(&v1.ListSpeechProfilesRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := protojson.Marshal(response.Msg)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), "Design") || !strings.Contains(string(encoded), "speechMax") {
			t.Fatalf("projection lost model or limits: %s", encoded)
		}
		assertSpeechPublicDescriptor(t, response.Msg.ProtoReflect().Descriptor())
		fixture.err = errors.New("private xi-api-key secret, supplier balance $99, USDPerUnit=4")
		_, err = h.ListSpeechProfiles(ctx, connect.NewRequest(&v1.ListSpeechProfilesRequest{}))
		if err == nil {
			t.Fatal("expected sanitized failure")
		}
		for _, forbidden := range []string{"xi-api-key", "secret", "balance", "$99", "USD"} {
			if strings.Contains(err.Error(), forbidden) {
				t.Fatalf("customer error leak (%s): %v", tier, err)
			}
		}
	}
}

func assertSpeechPublicDescriptor(t *testing.T, d protoreflect.MessageDescriptor) {
	t.Helper()
	for i := 0; i < d.Fields().Len(); i++ {
		f := d.Fields().Get(i)
		name := strings.ToLower(string(f.Name()))
		for _, forbidden := range []string{"price", "cost", "usd", "account", "key", "source", "token", "evidence"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("customer schema leaks field %s", f.FullName())
			}
		}
		if f.Message() != nil {
			assertSpeechPublicDescriptor(t, f.Message())
		}
	}
}

func TestSpeechAdminProceduresRefuseNonMasterBeforeReadingOrWriting(t *testing.T) {
	fixture := &speechRPCFixture{}
	h := NewSpeechHandler(fixture)
	for _, tier := range []plan.Plan{plan.Free, plan.Light, plan.Pro, plan.Max} {
		ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "ordinary", Plan: tier})
		_, err := h.AdminListSpeechProfiles(ctx, connect.NewRequest(&v1.AdminListSpeechProfilesRequest{}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatal(err)
		}
		_, err = h.SaveSpeechProfile(ctx, connect.NewRequest(&v1.SaveSpeechProfileRequest{}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatal(err)
		}
		_, err = h.StartSpeechQualification(ctx, connect.NewRequest(&v1.StartSpeechQualificationRequest{}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatal(err)
		}
	}
	if fixture.calls != 0 {
		t.Fatal("unprivileged call reached catalog")
	}
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "authenticated-owner", Plan: plan.Master})
	_, err := h.StartSpeechQualification(ctx, connect.NewRequest(&v1.StartSpeechQualificationRequest{ProfileId: "profile", Revision: 7, MaximumUsd: "1.25"}))
	if err != nil || fixture.owner != "authenticated-owner" {
		t.Fatalf("owner spoof: %s %v", fixture.owner, err)
	}
}

func (f *speechRPCFixture) RegisterSpeechCombination(_ context.Context, r modelcatalog.SpeechRegistration) (modelcatalog.SpeechProfile, error) {
	f.calls++
	return modelcatalog.SpeechProfile{}, f.err
}
func (f *speechRPCFixture) SaveSpeechAccountTariff(_ context.Context, t modelcatalog.SpeechAccountTariff, _ int64) (modelcatalog.SpeechAccountTariff, error) {
	f.calls++
	return t, f.err
}

func TestSpeechCatalogMutationsAreMasterOnly(t *testing.T) {
	f := &speechRPCFixture{}
	h := NewSpeechHandler(f)
	for _, tier := range []plan.Plan{plan.Free, plan.Light, plan.Pro, plan.Max} {
		ctx := auth.WithActor(t.Context(), auth.Actor{UserID: "ordinary", Plan: tier})
		_, err := h.RegisterSpeechCombination(ctx, connect.NewRequest(&v1.RegisterSpeechCombinationRequest{}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatal(err)
		}
		_, err = h.SaveSpeechTariff(ctx, connect.NewRequest(&v1.SaveSpeechTariffRequest{}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatal(err)
		}
	}
	if f.calls != 0 {
		t.Fatal("unprivileged mutation reached service")
	}
}
