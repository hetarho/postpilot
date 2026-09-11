package template

import (
	"context"
	"errors"
	"testing"
)

func ptr(value int) *int { return &value }

// TEMPLATE-6: the bounds are the POST option's own. A template must not be able to store a
// number the post would refuse, because nobody would be typing anything when the assignment
// tried to seed it.
func TestNumbersAreBoundedByThePostOptionsRules(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newFakeStore(), testLimits())

	cases := []struct {
		name    string
		numbers Numbers
		field   string
	}{
		{"zero length", Numbers{TargetLength: ptr(0)}, "target_length"},
		{"negative length", Numbers{TargetLength: ptr(-1)}, "target_length"},
		{"tag count below the floor", Numbers{TagCount: ptr(0)}, "tag_count"},
		{"tag count above the ceiling", Numbers{TagCount: ptr(11)}, "tag_count"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var outOfRange *NumberOutOfRangeError
			if _, err := svc.Create(ctx, "alice", "리뷰 "+tc.name, "", okBody, tc.numbers); !errors.As(err, &outOfRange) {
				t.Fatalf("create accepted %+v: %v", tc.numbers, err)
			}
			if outOfRange.Field != tc.field {
				t.Fatalf("wrong field named: %s", outOfRange.Field)
			}
		})
	}

	// The edges are accepted, and so is "no opinion" on both.
	created, err := svc.Create(ctx, "alice", "리뷰", "", okBody, Numbers{TargetLength: ptr(1), TagCount: ptr(10)})
	if err != nil {
		t.Fatalf("the edges were refused: %v", err)
	}
	if created.TargetLength == nil || *created.TargetLength != 1 || created.TagCount == nil || *created.TagCount != 10 {
		t.Fatalf("the numbers were not stored: %+v", created)
	}
	quiet, err := svc.Create(ctx, "alice", "의견 없음", "", okBody, Numbers{})
	if err != nil {
		t.Fatal(err)
	}
	if quiet.TargetLength != nil || quiet.TagCount != nil {
		t.Fatalf("a number was invented: %+v", quiet)
	}

	// An update is validated the same way, and a refusal writes nothing.
	var outOfRange *NumberOutOfRangeError
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{Numbers: &Numbers{TagCount: ptr(99)}}); !errors.As(err, &outOfRange) {
		t.Fatalf("update accepted an out-of-range count: %v", err)
	}
	all, err := svc.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	var unchanged Template
	for _, t2 := range all {
		if t2.ID == created.ID {
			unchanged = t2
		}
	}
	if unchanged.TagCount == nil || *unchanged.TagCount != 10 {
		t.Fatalf("a refused update wrote something: %+v", unchanged)
	}
}
