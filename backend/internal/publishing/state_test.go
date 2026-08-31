package publishing

import (
	"errors"
	"testing"
	"time"
)

func TestValidateProgressAllowsOnlyNextMonotonicStage(t *testing.T) {
	tests := []struct {
		name                string
		current, next       Stage
		currentSeq, nextSeq int64
		wantErr             error
	}{
		{"next", StageClaimed, StagePreparing, 1, 2, nil},
		{"same stage heartbeat", StagePreparing, StagePreparing, 2, 3, nil},
		{"skip", StageClaimed, StageOpeningEditor, 1, 2, ErrTransition},
		{"regress", StageFillingContent, StageOpeningEditor, 4, 5, ErrTransition},
		{"stale sequence", StagePreparing, StageOpeningEditor, 4, 4, ErrTransition},
		{"agent cannot publish directly", StageVerifying, StagePublished, 7, 8, ErrTransition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProgress(test.current, test.next, test.currentSeq, test.nextSeq)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestCommitFenceClassifiesEveryLaterFailureUnknown(t *testing.T) {
	for _, stage := range []Stage{StageCommitting, StageVerifying} {
		for _, kind := range []FailureKind{FailureSafe, FailureLoginExpired, FailureBrowserLost} {
			if got := FailureStatus(stage, kind); got != StatusOutcomeUnknown {
				t.Fatalf("%s/%s = %s", stage, kind, got)
			}
		}
	}
	if got := FailureStatus(StageOpeningEditor, FailureCaptcha); got != StatusNeedsAttention {
		t.Fatalf("captcha = %s", got)
	}
	if CanCancel(Job{Status: StatusRunning, Stage: StageCommitting}) {
		t.Fatal("commit-fenced job was cancelable")
	}
	if !CanCancel(Job{Status: StatusNeedsAttention, Stage: StageOpeningEditor}) {
		t.Fatal("pre-commit needs-attention job was not cancelable")
	}
	committedAt := time.Now()
	committed := Job{Status: StatusNeedsAttention, Stage: StageOpeningEditor, CommittedAt: &committedAt}
	if CanCancel(committed) {
		t.Fatal("job with a durable commit timestamp was cancelable")
	}
}

func TestNaverURLBelongsToExpectedBlog(t *testing.T) {
	if !validNaverURL("https://blog.naver.com/my-blog/123", "my-blog") {
		t.Fatal("expected blog URL rejected")
	}
	for _, value := range []string{
		"http://blog.naver.com/my-blog/123",
		"https://evil.test/my-blog/123",
		"https://blog.naver.com/other/123",
		"https://blog.naver.com/my-blog",
		"https://blog.naver.com/my-blog/category",
		"https://blog.naver.com/my-blog/123/extra",
	} {
		if validNaverURL(value, "my-blog") {
			t.Fatalf("unsafe URL accepted: %s", value)
		}
	}
}
