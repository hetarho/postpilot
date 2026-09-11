package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/template"
)

func number(value int) *int { return &value }

// TEMPLATE-47: the two numbers round-trip as authored, and NULL stays NULL — a template
// nobody gave a number to must not read back as one asking for zero characters.
func TestNumbersRoundTripAndUnsetStaysUnset(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	quiet := row("quiet", "alice", "의견 없음")
	if err := s.Insert(ctx, quiet, 10); err != nil {
		t.Fatal(err)
	}
	opinionated := row("opinionated", "alice", "정보성 리뷰")
	opinionated.TargetLength = number(1800)
	opinionated.TagCount = number(7)
	if err := s.Insert(ctx, opinionated, 10); err != nil {
		t.Fatal(err)
	}

	read, err := s.Get(ctx, "alice", "opinionated")
	if err != nil {
		t.Fatal(err)
	}
	if read.TargetLength == nil || *read.TargetLength != 1800 || read.TagCount == nil || *read.TagCount != 7 {
		t.Fatalf("numbers did not round-trip: %+v", read)
	}
	blank, err := s.Get(ctx, "alice", "quiet")
	if err != nil {
		t.Fatal(err)
	}
	if blank.TargetLength != nil || blank.TagCount != nil {
		t.Fatalf("a number was invented: %+v", blank)
	}

	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, t2 := range listed {
		if t2.ID == "opinionated" && (t2.TargetLength == nil || *t2.TargetLength != 1800) {
			t.Fatalf("the directory dropped a number: %+v", t2)
		}
	}
}

// TEMPLATE-8: the pair is written together, so unticking a number on the screen writes NULL
// back — while a patch that carries no pair at all leaves both columns alone.
func TestUpdateWritesBothNumbersAndCanUnsetThem(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	seeded := row("seeded", "alice", "정보성 리뷰")
	seeded.TargetLength = number(1800)
	seeded.TagCount = number(7)
	if err := s.Insert(ctx, seeded, 10); err != nil {
		t.Fatal(err)
	}

	// One member cleared, the other replaced: both columns are named by this one save.
	updated, err := s.Update(ctx, "alice", "seeded", template.Patch{
		Numbers: &template.Numbers{TagCount: number(3)},
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TargetLength != nil || updated.TagCount == nil || *updated.TagCount != 3 {
		t.Fatalf("the pair was not written together: %+v", updated)
	}

	// A patch with no pair is an edit of the text alone and leaves the numbers as they are.
	name := "이름만"
	renamed, err := s.Update(ctx, "alice", "seeded", template.Patch{Name: &name}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if renamed.TagCount == nil || *renamed.TagCount != 3 {
		t.Fatalf("a rename disturbed the numbers: %+v", renamed)
	}
}
