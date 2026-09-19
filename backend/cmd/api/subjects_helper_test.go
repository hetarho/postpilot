package main

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

// attach and clipJob build a test job through the same subject mapping the adapters use,
// so a test enqueues exactly what the product would.
func attach(in job.NewJob, slug, voiceID string) job.NewJob {
	in.Subjects, in.Guards = postVoiceWork(in.Kind, in.UserID, slug, voiceID)
	return in
}

// clipJob mirrors the clip adapter: charged clip work defers its hold until its owner
// approves the quote, and a render spends nothing at all.
func clipJob(in job.NewJob, project string) job.NewJob {
	in.DeferHold = in.Kind == "generate_clip" || in.Kind == "revise_clip"
	in.NonMetered = in.Kind == "render_clip"
	subject := job.Subject{Dimension: clip.JobSubject, ID: project}
	in.Subjects = append(in.Subjects, subject)
	in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: in.UserID}})
	return in
}

func jobKindsForTest() jobstore.Kinds { return jobKinds() }

func jobReportingForTest() job.Reporting { return jobReporting{} }
