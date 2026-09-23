package post

import "context"

// testDeps is the neutral set of collaborators (ARCH-40): no active job, the test voice
// directory, no pending experiment, a purge and a detach that succeed, nothing being
// published. A test that cares about one assigns the field it needs.
func testDeps() Deps {
	return Deps{
		Jobs:           neutralJobs{},
		Voices:         testVoices(),
		Experiments:    fakePendingExperiments{},
		ContentPurger:  &recordingContentPurger{},
		CandidateLinks: neutralDetacher{},
		MemoryLinks:    neutralDetacher{},
		Fields:         knownFields{"restaurant": true, "cafe": true},
	}
}

// knownFields is the 분야 directory: the ids it lists are the product's.
type knownFields map[string]bool

func (f knownFields) Known(id string) bool { return f[id] }

type neutralJobs struct{}

func (neutralJobs) ActiveForPost(context.Context, string) (*ActiveJob, error) { return nil, nil }

type neutralDetacher struct{}

func (neutralDetacher) DetachPost(context.Context, string, string) error { return nil }
