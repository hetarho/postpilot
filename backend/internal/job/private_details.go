package job

// ProtectDetails lets a work owner keep model identity and diagnostics private
// across generic public job reads and logs. Internal owner-checked Result reads
// retain the original record for accounting and terminal reconciliation.
func (q *Queue) ProtectDetails(kind string) {
	if kind == "" {
		panic("job: protected work kind is required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.protectedDetails == nil {
		q.protectedDetails = make(map[string]bool)
	}
	q.protectedDetails[kind] = true
}

func (q *Queue) detailsProtected(kind string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.protectedDetails[kind]
}
