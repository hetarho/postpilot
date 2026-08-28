package storage

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadAtMost(t *testing.T) {
	for _, size := range []int{0, 4} {
		got, err := readAtMost(bytes.NewBuffer(bytes.Repeat([]byte{'x'}, size)), 4)
		if err != nil || len(got) != size {
			t.Fatalf("size %d: got=%d err=%v", size, len(got), err)
		}
	}
	_, err := readAtMost(strings.NewReader("12345"), 4)
	if err == nil {
		t.Fatal("oversized stream was accepted")
	}
}
