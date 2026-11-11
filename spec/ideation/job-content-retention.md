# IDEATION job-content-retention
> st:open@261111 | Decide how long each kind of finished AI job keeps its frozen input and output content — deferred from template-from-request; working notes in `JOB-RETENTION-TODO.md`

## vision
- Problem: every AI job stays as a `generation_jobs` row whose `payload` keeps the input frozen at start (and for some kinds the result) after the job finishes, and nothing clears it; a template request's payload can hold a post the owner pasted from someone else's blog, which nothing reads once the job is done.
- Target: every owner whose material passes through AI jobs.
- Core value: job content is kept only as long as something needs it.

## explored
- [o] a template request is a job like every other kind until this is decided ← owner direction (ideation/done/template-from-request)
- [o] existing exceptions: an extraction job's payload is not kept once its candidates are resolved (MEM-16); finished model tests keep private inputs and outputs for thirty days (MODEL-42)
- [?] per-kind retention for the other job kinds (`generate`, `revise`, `analyze_voice`, `check_voice`, `storyline`, `revise_storyline`, clip jobs, template requests)

## shape
- -

## domains
- -

## open
- for each job kind, when a finished job's `payload` is cleared: at once, after N hours, after N days, or kept
- whether usage, charge and failure records stay while only the content is cleared
- how many days content must stay so a failure can be investigated
- whether the privacy-policy wording has to match (the terms step in `MONETIZATION-TODO.md`)
