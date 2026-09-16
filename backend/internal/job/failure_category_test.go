package job

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"testing"
)

// CLIP-88 asks for a typed category on every failed clip stage. An error nothing
// classified must still say WHAT it was — without carrying the path, the body or
// the prompt its message may hold.
func TestAnUnclassifiedFailureIsNamedByCategoryAndNeverByContent(t *testing.T) {
	err := fmt.Errorf("overlay step: %w", &fs.PathError{Op: "open", Path: "/work/secret-source.mp4", Err: syscall.ENOSPC})
	got := errorCategories(err)
	for _, want := range []string{"*fmt.wrapError", "*fs.PathError(open)", "no space left on device"} {
		if !strings.Contains(got, want) {
			t.Fatalf("category %q is missing from %q", want, got)
		}
	}
	if strings.Contains(got, "secret-source") || strings.Contains(got, "overlay step") {
		t.Fatalf("the categories carried content: %q", got)
	}
	// A joined error names every branch, and nothing at all stays nothing.
	joined := errorCategories(errors.Join(errors.New("a"), &fs.PathError{Op: "stat"}))
	if !strings.Contains(joined, "*fs.PathError(stat)") || errorCategories(nil) != "" {
		t.Fatal("a joined chain lost a branch", joined)
	}
	// Depth is bounded: a chain nothing limits cannot grow the log line.
	deep := error(errors.New("root"))
	for i := 0; i < 30; i++ {
		deep = fmt.Errorf("wrap: %w", deep)
	}
	if n := strings.Count(errorCategories(deep), "<-"); n > 8 {
		t.Fatal("an unbounded chain reached the log", n)
	}
}
