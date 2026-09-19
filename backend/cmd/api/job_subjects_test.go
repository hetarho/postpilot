package main

import (
	"testing"

	"github.com/postpilot/backend/internal/job"
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
			name: "voice-owned work on a post is guarded by both",
			kind: job.KindLearnVoice, slug: "post-a", voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "post", ID: "post-a"}, {Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "post", ID: "post-a"}, {Dimension: "voice", ID: "voice-a"}},
			guardFilters: []job.Filter{{UserID: "alice"}, {Kind: job.KindLearnVoice}},
		},
		{
			name: "voice-only work is guarded per voice and kind",
			kind: job.KindAnalyzeVoice, voice: "voice-a",
			subjects:     []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guards:       []job.Subject{{Dimension: "voice", ID: "voice-a"}},
			guardFilters: []job.Filter{{Kind: job.KindAnalyzeVoice}},
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
				if guards[i].Subject != want || guards[i].Filter != tc.guardFilters[i] {
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
