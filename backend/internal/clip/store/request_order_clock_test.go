package store_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestProjectRequestsKeepAcceptanceOrder(t *testing.T) {
	base := time.Date(2026, time.November, 11, 10, 45, 5, 123456789, time.UTC)
	for _, tc := range []struct {
		name  string
		times []time.Time
	}{
		{"clock rollback", []time.Time{base, base.Add(-12 * time.Second), base.Add(-20 * time.Second)}},
		{"equal timestamps", []time.Time{base, base, base}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, st, _ := setup(t)
			_, project := create(t, service)
			ctx := t.Context()
			accepted := []clip.ProjectRequest{
				{Kind: clip.RequestInstruction, Body: "  first request  ", CreatedAt: tc.times[0]},
				{Kind: clip.RequestStoryline, Body: "second request", CreatedAt: tc.times[1]},
				{Kind: clip.RevisionRequestKind(clip.RevisionNarration), Body: "third request\n", CreatedAt: tc.times[2]},
			}
			// IDs sort opposite to acceptance; neither IDs nor wall time establish order.
			for i, id := range []string{"z-first", "m-second", "a-third"} {
				if err := st.RecordProjectRequest(ctx, id, "alice", project.ID, accepted[i]); err != nil {
					t.Fatal(err)
				}
			}
			for _, user := range []string{"alice", "bob"} {
				other, err := service.CreateProject(ctx, user, clip.ProjectInput{
					Language: "ko", Title: "another project", Ratio: "vertical", TargetDurationMS: 15000,
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := st.RecordProjectRequest(ctx, "other-"+user, user, other.ID, clip.ProjectRequest{
					Kind: clip.RequestInstruction, Body: "another project's words", CreatedAt: base.Add(time.Hour),
				}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := st.ListProjectRequests(ctx, "alice", project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(accepted) {
				t.Fatalf("project-scoped history: %+v", got)
			}
			for i, request := range got {
				want := accepted[len(accepted)-1-i]
				if request.Kind != want.Kind || request.Body != want.Body || !request.CreatedAt.Equal(want.CreatedAt) {
					t.Fatalf("entry %d: got %+v, want %+v in reverse acceptance order", i, request, want)
				}
			}
			if foreign, err := st.ListProjectRequests(ctx, "bob", project.ID); err != nil || len(foreign) != 0 {
				t.Fatalf("foreign owner's history: %+v, %v", foreign, err)
			}
			read, err := service.GetProject(ctx, "alice", project.ID)
			if err != nil || len(read.Requests) != len(accepted) || read.Requests[0].Body != accepted[2].Body {
				t.Fatalf("project read lost acceptance order: %+v, %v", read.Requests, err)
			}
		})
	}
}
