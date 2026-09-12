package clip

// SameAttemptResult ignores temporary read URLs and the time value's location
// and monotonic clock, while requiring every durable candidate value to match.
func SameAttemptResult(a, b AttemptResult) bool {
	return a.JobID == b.JobID && a.UserID == b.UserID && a.ProjectID == b.ProjectID &&
		a.ExpectedRevision == b.ExpectedRevision && a.Analysis == b.Analysis && a.EditPlan == b.EditPlan &&
		a.Result.Key == b.Result.Key && a.Result.ContentType == b.Result.ContentType &&
		a.Result.Bytes == b.Result.Bytes && a.Result.DurationMS == b.Result.DurationMS &&
		a.Result.CreatedAt.Equal(b.Result.CreatedAt)
}
