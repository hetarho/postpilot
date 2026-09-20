package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

// fakeStore records what the service asked for. The field rules are the service's business
// and nothing here re-checks them, which is the point: a rule that is not in the service is
// a rule the hand-authoring path and the approval path could disagree about.
type fakeStore struct {
	inserted    []Memory
	sourceSlugs []string
	caps        []int
	dedupe      bool
	insertErr   error
	patches     []Patch
	dropped     []string
	stored      Memory
}

func (f *fakeStore) Insert(_ context.Context, m Memory, sourcePostSlug string, maxPerAccount int) (Memory, bool, error) {
	f.inserted = append(f.inserted, m)
	f.sourceSlugs = append(f.sourceSlugs, sourcePostSlug)
	f.caps = append(f.caps, maxPerAccount)
	if f.insertErr != nil {
		return Memory{}, false, f.insertErr
	}
	return m, f.dedupe, nil
}

func (f *fakeStore) List(context.Context, string) ([]Memory, error) { return []Memory{f.stored}, nil }

func (f *fakeStore) Get(context.Context, string, string) (Memory, error) { return f.stored, nil }

func (f *fakeStore) Update(_ context.Context, _, _ string, patch Patch, _ time.Time) (Memory, error) {
	f.patches = append(f.patches, patch)
	return f.stored, nil
}

func (f *fakeStore) Delete(context.Context, string, string) error { return nil }

func (f *fakeStore) DropPostSources(_ context.Context, _, postSlug string) error {
	f.dropped = append(f.dropped, postSlug)
	return nil
}

func newTestService(store Store) *Service {
	s := NewService(store, Limits{TextMaxChars: 20, TagsMax: 3, MaxPerAccount: 5})
	s.now = func() time.Time { return at }
	s.newID = func() string { return "generated" }
	return s
}

// MEM-12, in one table: every field rule, stated once, on the one path that creates.
func TestCreateAppliesEveryFieldRule(t *testing.T) {
	for name, test := range map[string]struct {
		text string
		kind Kind
		tags []string
		want error
	}{
		"empty text":       {text: "   ", kind: KindPreference, want: ErrInvalidText},
		"absent kind":      {text: "사실", kind: "", want: ErrInvalidKind},
		"unknown kind":     {text: "사실", kind: Kind("mood"), want: ErrInvalidKind},
		"empty tag":        {text: "사실", kind: KindPlace, tags: []string{"카페", " "}, want: ErrInvalidTag},
		"text over bound":  {text: strings.Repeat("가", 21), kind: KindPersona, want: nil},
		"too many tags":    {text: "사실", kind: KindPlace, tags: []string{"1", "2", "3", "4"}, want: nil},
		"duplicate tags":   {text: "사실", kind: KindPlace, tags: []string{"카페", "카페", "성수"}, want: nil},
		"trimmable enough": {text: "  사실  ", kind: KindHistory, want: nil},
	} {
		store := &fakeStore{}
		created, _, err := newTestService(store).Create(context.Background(), "alice", test.text, test.kind, test.tags, "")
		switch name {
		case "text over bound":
			var tooLong *TextTooLongError
			if !errors.As(err, &tooLong) || tooLong.Max != 20 || tooLong.Chars != 21 {
				t.Errorf("%s = %v, want a TextTooLongError carrying both counts", name, err)
			}
		case "too many tags":
			var tooMany *TooManyTagsError
			if !errors.As(err, &tooMany) || tooMany.Max != 3 || tooMany.Count != 4 {
				t.Errorf("%s = %v, want a TooManyTagsError carrying both counts", name, err)
			}
		case "duplicate tags":
			// Collapsed BEFORE counting, so repeating one tag is never "too many".
			if err != nil {
				t.Errorf("%s = %v", name, err)
			} else if got := created.Tags; len(got) != 2 || got[0] != "카페" || got[1] != "성수" {
				t.Errorf("%s tags = %v, want the duplicates collapsed in order", name, got)
			}
		case "trimmable enough":
			if err != nil || created.Text != "사실" {
				t.Errorf("%s = %q, %v; want the trimmed text", name, created.Text, err)
			}
		default:
			if !errors.Is(err, test.want) {
				t.Errorf("%s = %v, want %v", name, err, test.want)
			}
		}
	}
}

// The cap and the source link are the store's to apply, so the service's job is to hand
// both over unchanged — including the empty slug that marks a fact written by hand.
func TestCreateHandsTheStoreTheCapAndTheSource(t *testing.T) {
	store := &fakeStore{}
	service := newTestService(store)
	if _, _, err := service.Create(context.Background(), "alice", "사실", KindPlace, nil, "  post-1 "); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Create(context.Background(), "alice", "손으로 쓴 사실", KindPersona, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := store.sourceSlugs; len(got) != 2 || got[0] != "post-1" || got[1] != "" {
		t.Fatalf("source slugs = %q, want the trimmed slug then none", got)
	}
	if got := store.caps; got[0] != 5 || got[1] != 5 {
		t.Fatalf("caps = %v, want the configured 5 both times", got)
	}
	created := store.inserted[0]
	if created.ID != "generated" || !created.CreatedAt.Equal(at) || !created.LastSeenAt.Equal(at) {
		t.Fatalf("a create did not stamp its own id and times: %+v", created)
	}
}

// A create answers whatever the store decided about deduplication, because only the store
// can know: the lookup and the insert are one transaction there (MEM-9).
func TestCreateReportsTheStoresDeduplication(t *testing.T) {
	store := &fakeStore{dedupe: true}
	_, deduplicated, err := newTestService(store).Create(context.Background(), "alice", "사실", KindPreference, nil, "post-1")
	if err != nil || !deduplicated {
		t.Fatalf("create = %v, deduplicated %v", err, deduplicated)
	}
}

// An edit validates only what it carried, and an empty patch is a read rather than a write.
func TestUpdateValidatesOnlyThePresentParts(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	service := newTestService(store)

	blank := "   "
	if _, err := service.Update(ctx, "alice", "m1", Patch{Text: &blank}); !errors.Is(err, ErrInvalidText) {
		t.Fatalf("blank text edit = %v", err)
	}
	unknown := Kind("mood")
	if _, err := service.Update(ctx, "alice", "m1", Patch{Kind: &unknown}); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("unknown kind edit = %v", err)
	}
	if len(store.patches) != 0 {
		t.Fatal("a refused edit reached the store")
	}

	tags := []string{" 카페 ", "카페", "성수"}
	if _, err := service.Update(ctx, "alice", "m1", Patch{Tags: &tags}); err != nil {
		t.Fatal(err)
	}
	patch := store.patches[0]
	if patch.Text != nil || patch.Kind != nil {
		t.Fatalf("a tag-only edit carried other parts: %+v", patch)
	}
	if got := *patch.Tags; len(got) != 2 || got[0] != "카페" || got[1] != "성수" {
		t.Fatalf("tags = %v, want them trimmed and collapsed", got)
	}

	if _, err := service.Update(ctx, "alice", "  ", Patch{Text: &blank}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty id = %v, want ErrNotFound", err)
	}
	if _, err := service.Update(ctx, "alice", "m1", Patch{}); err != nil {
		t.Fatal(err)
	}
	if len(store.patches) != 1 {
		t.Fatal("an empty patch was written rather than read")
	}
}

// The post-delete hook is the only thing another context calls, and a blank slug is not a
// post: forwarding it would ask the store to drop every link that names nothing.
func TestDetachPostForwardsOnlyARealSlug(t *testing.T) {
	store := &fakeStore{}
	service := newTestService(store)
	if err := service.DetachPost(context.Background(), "alice", "   "); err != nil {
		t.Fatal(err)
	}
	if err := service.DetachPost(context.Background(), "alice", "post-1"); err != nil {
		t.Fatal(err)
	}
	if len(store.dropped) != 1 || store.dropped[0] != "post-1" {
		t.Fatalf("dropped = %v, want the one real slug", store.dropped)
	}
}

// MEM-6 is asked of the kind itself, so retrieval (T290) cannot answer it from a list that
// drifted from this one.
func TestTheKindsSplitIntoTheTwoRetrievalHalves(t *testing.T) {
	always := map[Kind]bool{KindPreference: true, KindPersona: true}
	for _, kind := range Kinds {
		if kind.AlwaysCandidate() != always[kind] {
			t.Errorf("%s.AlwaysCandidate() = %v", kind, kind.AlwaysCandidate())
		}
		if !kind.Valid() {
			t.Errorf("%s is not valid", kind)
		}
	}
	if len(Kinds) != 5 {
		t.Fatalf("kinds = %d, want the closed five", len(Kinds))
	}
	if _, err := ParseKind("mood"); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("ParseKind of an unknown string = %v", err)
	}
	if _, err := ParseKind(""); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("ParseKind of the zero value = %v", err)
	}
	kind, err := ParseKind("history")
	if err != nil || kind != KindHistory {
		t.Fatalf("ParseKind(history) = %q, %v", kind, err)
	}
}
