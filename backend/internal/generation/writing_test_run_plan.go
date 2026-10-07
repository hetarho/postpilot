package generation

// PlanWritingTestRetry describes only remaining shared preparation and the explicitly selected
// failed entrant pipelines. The caller derives failedIndices from its owner-scoped test record;
// a completed output or an issued-unconfirmed checkpoint can never become retry work.
func (f *WritingTestFactory) PlanWritingTestRetry(snapshot WritingTestSnapshot, shared *WritingTestCheckpoint, checkpoints map[int]WritingTestCheckpoint, failedIndices []int) ([]WritingTestCall, error) {
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	if err := checkWritingTestVersions(common); err != nil {
		return nil, err
	}
	if len(failedIndices) == 0 {
		return nil, ErrWritingTestRetryRequired
	}
	var calls []WritingTestCall
	sharedObservations := common.Observations
	if !observerWritingTest(common) && !common.Prepared {
		targets, seed := frozenObserveSelection(common.Post.Images, common.ObserveFiles, common.Observations)
		batches := writingTestBatches(targets, common.BatchSize)
		checkpoint := writingTestInitialCheckpoint(snapshot, common, -1, nil, seed)
		if shared != nil {
			checkpoint, err = acceptWritingTestCheckpoint(checkpoint, *shared, batches)
			if err != nil {
				return nil, err
			}
		}
		if err := writingTestResumeAllowed(checkpoint, true); err != nil {
			return nil, err
		}
		if checkpoint.Prepared {
			sharedObservations = checkpoint.Observations
		}
		calls = appendWritingTestCall(calls, WritingTestCall{Ref: common.ObserveModel, Stage: "observe", Count: len(batches) - checkpoint.CompletedObserveCalls, PromptTokens: common.ObservePromptTokens, CompletionTokens: common.ObserveCompletionTokens})
	}
	seen := map[int]bool{}
	for _, index := range failedIndices {
		if index < 0 || index >= len(variants) || seen[index] {
			return nil, ErrWritingTestReference
		}
		seen[index] = true
		variant := variants[index]
		var batches [][]Image
		if observerWritingTest(common) {
			batches = writingTestBatches(variant.Snapshot.Post.Images, common.BatchSize)
		}
		seed := sharedObservations
		if observerWritingTest(common) {
			seed = nil
		}
		checkpoint := writingTestInitialCheckpoint(snapshot, common, index, &variant, seed)
		if stored, ok := checkpoints[index]; ok {
			checkpoint, err = acceptWritingTestCheckpoint(checkpoint, stored, batches)
			if err != nil {
				return nil, err
			}
		}
		if checkpoint.Answer != nil {
			return nil, ErrWritingTestRetryRequired
		}
		if err := writingTestResumeAllowed(checkpoint, true); err != nil {
			return nil, err
		}
		calls = appendWritingTestCall(calls, WritingTestCall{Ref: variant.ObserveModel, Stage: "observe", Count: len(batches) - checkpoint.CompletedObserveCalls, PromptTokens: variant.ObservePromptTokens, CompletionTokens: variant.ObserveCompletionTokens})
		calls = appendWritingTestCall(calls, WritingTestCall{Ref: variant.WriteModel, Stage: "write", Count: 1, PromptTokens: variant.WritePromptTokens, CompletionTokens: variant.WriteCompletionTokens})
	}
	return calls, nil
}

func appendWritingTestCall(calls []WritingTestCall, next WritingTestCall) []WritingTestCall {
	if next.Count <= 0 {
		return calls
	}
	for index, call := range calls {
		if call.Ref == next.Ref && call.Stage == next.Stage && call.PromptTokens == next.PromptTokens && call.CompletionTokens == next.CompletionTokens {
			calls[index].Count += next.Count
			return calls
		}
	}
	return append(calls, next)
}
