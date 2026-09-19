package main

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// attach and clipJob build a test job through the same subject mapping the adapters use,
// so a test enqueues exactly what the product would.
func attach(in job.NewJob, slug, voiceID string) job.NewJob {
	in.Subjects, in.Guards = postVoiceWork(in.Kind, in.UserID, slug, voiceID)
	return in
}

func clipJob(in job.NewJob, project string) job.NewJob {
	subject := job.Subject{Dimension: clip.JobSubject, ID: project}
	in.Subjects = append(in.Subjects, subject)
	in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: in.UserID}})
	return in
}

func deferredKindsForTest() []string { return deferredDispatchKinds() }
