package main

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
)

// The rule this pins used to live inside the queue: which subject serializes which kind.
// It is the composition root's answer now, and these are the four shapes it produces.
func TestPostVoiceWorkStatesTheGuardsTheQueueUsedToInfer(t *testing.T) {
	for _, tc := range []struct {
		name             string
		kind             string
		slug, voice      string
		subjects, guards []job.Subject
		guardFilters     []job.Filter
	}{
		{
			name: "a post generation is one job per post for its owner",
			kind: job.KindGenerate, slug: "post-a", voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "post", ID: "post-a"}, {Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "post", ID: "post-a"}},
			guardFilters: []job.Filter{{UserID: "alice"}},
		},
		{
			name: "voice-only work is guarded per voice and kind",
			kind: job.KindAnalyzeVoice, voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guardFilters: []job.Filter{{Kind: job.KindAnalyzeVoice}},
		},
		{
			name: "a 검증 is guarded per voice and kind",
			kind: job.KindCheckVoice, voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guardFilters: []job.Filter{{Kind: job.KindCheckVoice}},
		},
		{
			name: "an experiment on a post and a voice is guarded by the post alone",
			kind: job.KindModelExperiment, slug: "post-a", voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "post", ID: "post-a"}, {Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "post", ID: "post-a"}},
			guardFilters: []job.Filter{{UserID: "alice"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subjects, guards := postVoiceWork(tc.kind, "alice", tc.slug, tc.voice)
			if len(subjects) != len(tc.subjects) {
				t.Fatalf("subjects = %v, want %v", subjects, tc.subjects)
			}
			for i, want := range tc.subjects {
				if subjects[i] != want {
					t.Fatalf("subject %d = %v, want %v", i, subjects[i], want)
				}
			}
			if len(guards) != len(tc.guards) {
				t.Fatalf("guards = %v, want %v", guards, tc.guards)
			}
			for i, want := range tc.guards {
				if guards[i].Subject != want || !reflect.DeepEqual(guards[i].Filter, tc.guardFilters[i]) {
					t.Fatalf("guard %d = %v, want %v %v", i, guards[i], want, tc.guardFilters[i])
				}
			}
		})
	}
}

// Work with no subject falls back to the queue's own default: one per user and kind.
func TestPostVoiceWorkLeavesUnattachedWorkToTheQueueDefault(t *testing.T) {
	subjects, guards := postVoiceWork(job.KindGenerate, "alice", "", "")
	if subjects != nil || guards != nil {
		t.Fatalf("subjects/guards = %v/%v, want none", subjects, guards)
	}
}

// Saving a published address waits only for work that writes the post (POST-73): a job that
// learns from it or reads it must not hold a URL paste hostage.
func TestOnlyGenerationRevisionAndComparisonsWritePostContent(t *testing.T) {
	for kind, want := range map[string]bool{
		job.KindGenerate:        true,
		job.KindRevise:          true,
		job.KindModelExperiment: true,
		job.KindExtractMemory:   false,
		job.KindAnalyzeVoice:    false,
		job.KindCheckVoice:      false,
		// A storyline job writes the storyline, never the content (GEN-68, GEN-69).
		job.KindStoryline:       false,
		job.KindReviseStoryline: false,
	} {
		if got := postContentWork(kind); got != want {
			t.Errorf("postContentWork(%q) = %v, want %v", kind, got, want)
		}
	}
	for _, kind := range []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise, ""} {
		if postContentWork(kind) {
			t.Errorf("postContentWork(%q) = true", kind)
		}
	}
}

// GEN-68, GEN-69: a storyline job is post-targeted — one active job per post — and belongs to no
// voice.
func TestStorylineWorkIsPostTargetedAndOwnsNoVoice(t *testing.T) {
	for _, kind := range []string{job.KindStoryline, job.KindReviseStoryline} {
		if voiceOwnedKind(kind) {
			t.Errorf("%s is voice-owned", kind)
		}
		subjects, guards := postVoiceWork(kind, "alice", "post", "")
		want := job.Subject{Dimension: post.JobSubject, ID: "post"}
		if len(subjects) != 1 || subjects[0] != want || len(guards) != 1 || guards[0].Subject != want || !reflect.DeepEqual(guards[0].Filter, job.Filter{UserID: "alice"}) {
			t.Errorf("%s: subjects %v guards %v", kind, subjects, guards)
		}
	}
}

// migratedJobs is the queue's store as the root wires it, over a migrated temp database with
// one account.
func migratedJobs(t *testing.T) (*jobstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(t.Context(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	return jobstore.New(handle.Writer, handle.Reader, jobKinds()), handle
}

// pickOne releases a deferred job the way its owner's approval would and dispatches it.
func pickOne(t *testing.T, store *jobstore.Store, id, kind string) job.Job {
	t.Helper()
	if slices.Contains(jobKinds().Deferred, kind) {
		if released, err := store.Activate(t.Context(), "alice", id); err != nil || !released {
			t.Fatalf("activate %s: %v %v", id, released, err)
		}
	}
	picked, err := store.PickNextQueued(t.Context(), time.Now().UTC())
	if err != nil || picked.ID != id || picked.Status != job.StatusRunning {
		t.Fatalf("pick = %+v, %v; want %s running", picked, err, id)
	}
	return picked
}

// F13: which kinds an owner may stop lives in Go alone — jobKinds' Cancellable and
// jobCancellation — and the schema no longer repeats it, so nothing there may refuse one of
// them. Every cancellable kind is stopped through the store against a migrated database:
// queued, where the request is the terminal write, and running, where the worker finishes it.
func TestEveryCancellableKindIsStoppedThroughTheStore(t *testing.T) {
	store, handle := migratedJobs(t)
	rule := jobCancellation{}
	for _, kind := range jobKinds().Cancellable {
		t.Run(kind, func(t *testing.T) {
			ctx, at := t.Context(), time.Now().UTC()
			version := slices.IndexFunc([]int{0, 1}, func(v int) bool { return rule.Allowed(kind, v) })
			if !rule.Kind(kind) || version < 0 {
				t.Fatalf("%s is cancellable but the root's rule never lets it be stopped", kind)
			}
			var subjects []job.Subject
			project := ""
			if clip.IsJobKind(kind) {
				project = "clip-" + kind
				if _, err := handle.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES(?,'alice','clip','square',15000,?,?)`, project, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
				subjects = []job.Subject{{Dimension: clip.JobSubject, ID: project}}
			}
			request := func(id string) {
				t.Helper()
				err := store.RequestOwnedCancellation(ctx, "alice", id, at)
				if project != "" {
					err = store.RequestCancellation(ctx, "alice", project, id, at)
				}
				if err != nil {
					t.Fatalf("request to stop %s: %v", id, err)
				}
			}
			read := func(id string) job.Job {
				t.Helper()
				found, err := store.GetByID(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				return found
			}
			for _, id := range []string{kind, kind + "-running"} {
				if err := store.Insert(ctx, job.Job{ID: id, Kind: kind, UserID: "alice", Subjects: subjects, CancellationPolicyVersion: version, CreatedAt: at, UpdatedAt: at}); err != nil {
					t.Fatalf("insert %s: %v", id, err)
				}
				if id == kind {
					request(id)
					if stopped := read(id); stopped.Status != job.StatusCancelled || stopped.CancelRequestedAt == nil {
						t.Fatalf("queued %s after the request = %s, requested %v", kind, stopped.Status, stopped.CancelRequestedAt)
					}
					continue
				}
				pickOne(t, store, id, kind)
				request(id)
				if asked := read(id); asked.Status != job.StatusRunning || asked.CancelRequestedAt == nil {
					t.Fatalf("running %s after the request = %s, requested %v", kind, asked.Status, asked.CancelRequestedAt)
				}
				if err := store.Finish(ctx, id, job.StatusCancelled, nil, at); err != nil {
					t.Fatalf("finish %s: %v", id, err)
				}
				if stopped := read(id); stopped.Status != job.StatusCancelled {
					t.Fatalf("running %s finished as %s", kind, stopped.Status)
				}
			}
		})
	}
}

// No schema CHECK backs this rule up, so the root's answer is the whole of it: charged clip
// work and a template request may be stopped only under the policy their owner approved, a
// render and a browser render's sampling always, and nothing else at all.
func TestOnlyTheApprovedPolicyLetsChargedWorkBeStopped(t *testing.T) {
	rule := jobCancellation{}
	for _, tc := range []struct {
		kind             string
		legacy, approved bool
	}{
		{clip.JobKindGenerate, false, true}, {clip.JobKindRevise, false, true},
		{clip.JobKindStoryline, false, true}, {clip.JobKindReviseStoryline, false, true},
		{job.KindTemplateRequest, false, true},
		{clip.JobKindRender, true, true}, {clip.JobKindSampleBrowserRender, true, true},
		{job.KindGenerate, false, false},
	} {
		if rule.Kind(tc.kind) != (tc.approved || tc.legacy) || rule.Allowed(tc.kind, 0) != tc.legacy || rule.Allowed(tc.kind, 1) != tc.approved {
			t.Errorf("%s: kind %v, legacy %v, approved %v", tc.kind, rule.Kind(tc.kind), rule.Allowed(tc.kind, 0), rule.Allowed(tc.kind, 1))
		}
	}
}

// F13: the dispatcher writes the stage the root names for each kind as it picks the job up,
// and observe for any other. Charged clip work must start in prepare: it is the only stage
// clip/app's Reserve accepts a hold in.
func TestDispatchStartsEveryKindInItsFirstStage(t *testing.T) {
	store, _ := migratedJobs(t)
	stages := jobKinds().FirstStages
	for _, kind := range []string{clip.JobKindGenerate, clip.JobKindRevise, clip.JobKindStoryline, clip.JobKindReviseStoryline} {
		if !clip.ChargedJobKind(kind) || stages[kind] != "prepare" {
			t.Errorf("charged clip kind %s starts in %q, want prepare", kind, stages[kind])
		}
	}
	want := map[string]string{job.KindGenerate: "observe", clip.JobKindRender: "observe"}
	for kind, stage := range stages {
		want[kind] = stage
	}
	for _, kind := range slices.Sorted(maps.Keys(want)) {
		at := time.Now().UTC()
		id := kind + "-picked"
		if err := store.Insert(t.Context(), job.Job{ID: id, Kind: kind, UserID: "alice", CreatedAt: at, UpdatedAt: at}); err != nil {
			t.Fatalf("insert %s: %v", kind, err)
		}
		if picked := pickOne(t, store, id, kind); picked.Stage != want[kind] {
			t.Errorf("%s was dispatched in %q, want %q", kind, picked.Stage, want[kind])
		}
		if err := store.Finish(t.Context(), id, job.StatusDone, nil, at); err != nil {
			t.Fatal(err)
		}
	}
}
