package purpose

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func testLimits() Limits {
	return Limits{NameMaxChars: 40, DescriptionMaxChars: 200, InstructionsMaxChars: 2000}
}

// memoryStore is a Store that keeps the ownership and presence rules the SQL one enforces,
// so the service tests exercise validation rather than SQLite. The real statements are
// covered by store_test.go against a real database.
type memoryStore struct {
	mu     sync.Mutex
	rows   map[string]Purpose
	counts map[string]int
	seq    int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: map[string]Purpose{}, counts: map[string]int{}}
}

func (m *memoryStore) Insert(_ context.Context, p Purpose) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.rows {
		if existing.UserID == p.UserID && existing.Name == p.Name {
			return ErrDuplicateName
		}
	}
	m.rows[p.ID] = p
	return nil
}

func (m *memoryStore) List(_ context.Context, userID string) ([]Purpose, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Purpose{}
	for _, p := range m.rows {
		if p.UserID == userID {
			p.PostCount = m.counts[p.ID]
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *memoryStore) Get(_ context.Context, userID, id string) (Purpose, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.rows[id]
	if !ok || p.UserID != userID {
		return Purpose{}, ErrNotFound
	}
	p.PostCount = m.counts[id]
	return p, nil
}

func (m *memoryStore) Update(_ context.Context, userID, id string, patch Patch, updatedAt time.Time) (Purpose, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.rows[id]
	if !ok || p.UserID != userID {
		return Purpose{}, ErrNotFound
	}
	if patch.Name != nil {
		for otherID, other := range m.rows {
			if otherID != id && other.UserID == userID && other.Name == *patch.Name {
				return Purpose{}, ErrDuplicateName
			}
		}
		p.Name = *patch.Name
	}
	if patch.Description != nil {
		p.Description = *patch.Description
	}
	if patch.Instructions != nil {
		p.Instructions = *patch.Instructions
	}
	p.UpdatedAt = updatedAt
	m.rows[id] = p
	return p, nil
}

func (m *memoryStore) Delete(_ context.Context, userID, id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.rows[id]
	if !ok || p.UserID != userID {
		return 0, ErrNotFound
	}
	delete(m.rows, id)
	detached := m.counts[id]
	delete(m.counts, id)
	return detached, nil
}

func newTestService(t *testing.T) (*Service, *memoryStore) {
	t.Helper()
	store := newMemoryStore()
	svc := NewService(store, testLimits())
	svc.newID = func() string {
		store.mu.Lock()
		defer store.mu.Unlock()
		store.seq++
		return "purpose-" + string(rune('a'+store.seq-1))
	}
	return svc, store
}

func TestCreateTrimsBoundsAndKeepsNamesUniquePerAccount(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	created, err := svc.Create(ctx, "alice", "  정보성 식당 리뷰  ", "  협찬 방문 리뷰  ", "  사진마다 설명  ")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "정보성 식당 리뷰" || created.Description != "협찬 방문 리뷰" || created.Instructions != "사진마다 설명" {
		t.Fatalf("fields were not trimmed: %+v", created)
	}

	if _, err := svc.Create(ctx, "alice", "정보성 식당 리뷰", "", "지침"); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate name accepted: %v", err)
	}
	// The same name in another account is a different aggregate, not a collision.
	if _, err := svc.Create(ctx, "bob", "정보성 식당 리뷰", "", "지침"); err != nil {
		t.Fatalf("same name in another account refused: %v", err)
	}

	if _, err := svc.Create(ctx, "alice", "   ", "", "지침"); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("blank name accepted: %v", err)
	}
	if _, err := svc.Create(ctx, "alice", "이름", "", "   "); !errors.Is(err, ErrInstructionsRequired) {
		t.Fatalf("blank instructions accepted: %v", err)
	}
	// An empty description is a real value, not a missing one.
	if _, err := svc.Create(ctx, "alice", "설명 없는 용도", "", "지침"); err != nil {
		t.Fatalf("empty description refused: %v", err)
	}

	var tooLong *FieldTooLongError
	limits := testLimits()
	for _, tc := range []struct {
		field                           string
		name, description, instructions string
		max                             int
	}{
		{"name", strings.Repeat("가", limits.NameMaxChars+1), "", "지침", limits.NameMaxChars},
		{"description", "이름1", strings.Repeat("가", limits.DescriptionMaxChars+1), "지침", limits.DescriptionMaxChars},
		{"instructions", "이름2", "", strings.Repeat("가", limits.InstructionsMaxChars+1), limits.InstructionsMaxChars},
	} {
		_, err := svc.Create(ctx, "alice", tc.name, tc.description, tc.instructions)
		if !errors.As(err, &tooLong) || tooLong.Field != tc.field || tooLong.Max != tc.max || tooLong.Chars != tc.max+1 {
			t.Fatalf("%s limit: err=%v", tc.field, err)
		}
	}
	// The ceiling itself is allowed — the limit is inclusive.
	if _, err := svc.Create(ctx, "alice", strings.Repeat("가", limits.NameMaxChars), "", "지침"); err != nil {
		t.Fatalf("name at the ceiling refused: %v", err)
	}
}

func TestUpdateChangesOnlyThePresentFields(t *testing.T) {
	ctx := context.Background()
	svc, store := newTestService(t)
	created, err := svc.Create(ctx, "alice", "리뷰", "설명", "지침")
	if err != nil {
		t.Fatal(err)
	}

	// A concurrent rename lands between the read and this edit. An instructions-only patch
	// must not carry the old name back with it.
	if _, err := store.Update(ctx, "alice", created.ID, Patch{Name: pointer("새 이름")}, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Update(ctx, "alice", created.ID, Patch{Instructions: pointer("  새 지침  ")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "새 이름" {
		t.Fatalf("instructions-only update reverted the name to %q", updated.Name)
	}
	if updated.Instructions != "새 지침" {
		t.Fatalf("instructions were not trimmed and applied: %q", updated.Instructions)
	}
	if updated.Description != "설명" {
		t.Fatalf("absent description was rewritten to %q", updated.Description)
	}

	// A present empty description clears it; absence would have preserved it.
	cleared, err := svc.Update(ctx, "alice", created.ID, Patch{Description: pointer("")})
	if err != nil || cleared.Description != "" {
		t.Fatalf("present empty description did not clear: %+v err=%v", cleared, err)
	}
	// A present empty instructions is still a refusal — clearing it is not a valid edit.
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{Instructions: pointer("  ")}); !errors.Is(err, ErrInstructionsRequired) {
		t.Fatalf("blank instructions accepted on update: %v", err)
	}
}

func TestForeignAndUnknownIdsAreIndistinguishable(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	mine, err := svc.Create(ctx, "alice", "리뷰", "", "지침")
	if err != nil {
		t.Fatal(err)
	}

	for name, id := range map[string]string{"foreign": mine.ID, "unknown": "does-not-exist"} {
		if _, err := svc.Update(ctx, "bob", id, Patch{Name: pointer("훔치기")}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s update: %v", name, err)
		}
		if _, err := svc.Delete(ctx, "bob", id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s delete: %v", name, err)
		}
		if _, ok, err := svc.BriefFor(ctx, "bob", id); err != nil || ok {
			t.Fatalf("%s brief: ok=%v err=%v", name, ok, err)
		}
	}
	// Nothing bob did may have touched alice's row.
	current, err := svc.Update(ctx, "alice", mine.ID, Patch{})
	if err != nil || current.Name != "리뷰" {
		t.Fatalf("foreign writes changed the owner's row: %+v err=%v", current, err)
	}
}

func TestDeleteReportsHowManyPostsItDetached(t *testing.T) {
	ctx := context.Background()
	svc, store := newTestService(t)
	referenced, err := svc.Create(ctx, "alice", "리뷰", "", "지침")
	if err != nil {
		t.Fatal(err)
	}
	unreferenced, err := svc.Create(ctx, "alice", "일기", "", "지침")
	if err != nil {
		t.Fatal(err)
	}
	store.counts[referenced.ID] = 3

	if detached, err := svc.Delete(ctx, "alice", referenced.ID); err != nil || detached != 3 {
		t.Fatalf("detached=%d err=%v", detached, err)
	}
	if detached, err := svc.Delete(ctx, "alice", unreferenced.ID); err != nil || detached != 0 {
		t.Fatalf("unreferenced detached=%d err=%v", detached, err)
	}
}

func TestBriefForTreatsAMissingPurposeAsNoPurpose(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	created, err := svc.Create(ctx, "alice", "리뷰", "설명", "지침")
	if err != nil {
		t.Fatal(err)
	}

	brief, ok, err := svc.BriefFor(ctx, "alice", created.ID)
	if err != nil || !ok || brief != (Brief{Name: "리뷰", Description: "설명", Instructions: "지침"}) {
		t.Fatalf("brief=%+v ok=%v err=%v", brief, ok, err)
	}
	// Absence is never an error: a post with no purpose, and a purpose deleted between the
	// save and the enqueue, both have to produce a prompt without a brief.
	for _, id := range []string{"", "deleted-since"} {
		if _, ok, err := svc.BriefFor(ctx, "alice", id); err != nil || ok {
			t.Fatalf("id=%q ok=%v err=%v", id, ok, err)
		}
	}
}

func pointer(value string) *string { return &value }
