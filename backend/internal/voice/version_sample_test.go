package voice_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/voice"
)

// publishHead gives a voice a head version to hang a snapshot on. A snapshot describes a
// PUBLISHED version, so a voice with none has nowhere to put one.
func publishHead(t *testing.T, h *voiceHarness, user, voiceID, description string) int64 {
	t.Helper()
	version, err := h.store.PublishProfileVersion(context.Background(), user, voiceID, voice.StructuredProfile{
		Empty:   false,
		Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: description, Source: voice.SourceAnalyzed}},
	}, "analysis", 0, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return version.Version
}

func TestVersionSampleReplacesRatherThanAccumulates(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	head := publishHead(t, h, "alice", alice, "첫 분석")

	if err := h.svc.RecordVersionSample(ctx, "alice", alice, `{"title":"첫 글"}`); err != nil {
		t.Fatal(err)
	}
	// A second generation under the SAME head replaces the snapshot: a version carries at most
	// one, and it is the most recent thing that version produced.
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, `{"title":"둘째 글"}`); err != nil {
		t.Fatal(err)
	}
	sample, err := h.svc.VersionSample(ctx, "alice", alice, head)
	if err != nil || sample.Content != `{"title":"둘째 글"}` || sample.Version != head {
		t.Fatalf("snapshot after second generation = %+v err=%v", sample, err)
	}

	// A NEW head is a different version and keeps its own snapshot; the old one survives, which
	// is what makes an older version previewable at all.
	next, err := h.store.PublishProfileVersion(ctx, "alice", alice, voice.StructuredProfile{
		Empty: false, Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "둘째 분석"}},
	}, "analysis", 0, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, `{"title":"셋째 글"}`); err != nil {
		t.Fatal(err)
	}
	if old, err := h.svc.VersionSample(ctx, "alice", alice, head); err != nil || old.Content != `{"title":"둘째 글"}` {
		t.Fatalf("older version lost its snapshot: %+v err=%v", old, err)
	}
	if fresh, err := h.svc.VersionSample(ctx, "alice", alice, next.Version); err != nil || fresh.Content != `{"title":"셋째 글"}` {
		t.Fatalf("new head snapshot = %+v err=%v", fresh, err)
	}
}

// A snapshot is a COPY, so the post it came from is irrelevant to it afterwards. There is no
// join to break: deleting the post, editing it, or reassigning it cannot reach this row.
func TestVersionSampleSurvivesItsSourcePost(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	head := publishHead(t, h, "alice", alice, "분석")
	insertPost(t, h, "gone", "alice", alice, "지워질 글", time.Now().UTC().Format(time.RFC3339Nano))
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, `{"title":"지워질 글"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("DELETE FROM posts WHERE slug='gone'"); err != nil {
		t.Fatal(err)
	}
	sample, err := h.svc.VersionSample(ctx, "alice", alice, head)
	if err != nil || sample.Content != `{"title":"지워질 글"}` {
		t.Fatalf("snapshot followed its post: %+v err=%v", sample, err)
	}
}

func TestVersionSampleIsPrivateToOneVoiceAndOneAccount(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	other, _, err := h.svc.CreateVoice(ctx, "alice", "다른 말투", voice.LanguageKorean, nil)
	if err != nil {
		t.Fatal(err)
	}
	aliceHead := publishHead(t, h, "alice", alice, "앨리스 분석")
	otherHead := publishHead(t, h, "alice", other.ID, "다른 분석")
	bob := h.voice("bob")
	bobHead := publishHead(t, h, "bob", bob, "밥 분석")
	for _, row := range []struct{ user, voiceID, content string }{
		{"alice", alice, "ALICE_ONE"}, {"alice", other.ID, "ALICE_TWO"}, {"bob", bob, "BOB"},
	} {
		if err := h.svc.RecordVersionSample(ctx, row.user, row.voiceID, row.content); err != nil {
			t.Fatal(err)
		}
	}
	// Two voices in one account do not see each other's snapshots.
	if got, err := h.svc.VersionSample(ctx, "alice", alice, aliceHead); err != nil || got.Content != "ALICE_ONE" {
		t.Fatalf("alice default snapshot = %+v err=%v", got, err)
	}
	if got, err := h.svc.VersionSample(ctx, "alice", other.ID, otherHead); err != nil || got.Content != "ALICE_TWO" {
		t.Fatalf("alice second voice snapshot = %+v err=%v", got, err)
	}
	// A crafted voice id from another account is a missing VOICE, never that account's data.
	if _, err := h.svc.VersionSample(ctx, "bob", alice, aliceHead); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("cross-account snapshot read = %v", err)
	}
	if err := h.svc.RecordVersionSample(ctx, "bob", alice, "HIJACK"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("cross-account snapshot write = %v", err)
	}
	if got, err := h.svc.VersionSample(ctx, "bob", bob, bobHead); err != nil || got.Content != "BOB" {
		t.Fatalf("bob snapshot was overwritten: %+v err=%v", got, err)
	}
	// A version that never produced a post is an ordinary state, reported as such.
	if _, err := h.svc.VersionSample(ctx, "alice", alice, aliceHead+99); !errors.Is(err, voice.ErrVersionSampleNotFound) {
		t.Fatalf("absent snapshot = %v", err)
	}
}

func TestVersionSampleNeedsAPublishedVersionAndAnActiveVoice(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	// No head yet: recording invents no version 0.
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, "TOO_EARLY"); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := h.db.Reader.QueryRow("SELECT count(*) FROM voice_version_samples").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("recorded a snapshot with no head: rows=%d err=%v", rows, err)
	}
	head := publishHead(t, h, "alice", alice, "분석")
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, "KEEP"); err != nil {
		t.Fatal(err)
	}
	// A deleted voice takes no new writing, but its record stays READABLE like the rest of its
	// profile.
	gone, _, err := h.svc.CreateVoice(ctx, "alice", "사라질 말투", voice.LanguageKorean, nil)
	if err != nil {
		t.Fatal(err)
	}
	goneHead := publishHead(t, h, "alice", gone.ID, "분석")
	if err := h.svc.RecordVersionSample(ctx, "alice", gone.ID, "BEFORE_DELETE"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", gone.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RecordVersionSample(ctx, "alice", gone.ID, "AFTER_DELETE"); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("recorded into a deleted voice = %v", err)
	}
	if got, err := h.svc.VersionSample(ctx, "alice", gone.ID, goneHead); err != nil || got.Content != "BEFORE_DELETE" {
		t.Fatalf("tombstone snapshot = %+v err=%v", got, err)
	}
	if got, err := h.svc.VersionSample(ctx, "alice", alice, head); err != nil || got.Content != "KEEP" {
		t.Fatalf("live voice snapshot = %+v err=%v", got, err)
	}
	// Empty content is not a snapshot: a run that produced nothing records nothing.
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := h.svc.VersionSample(ctx, "alice", alice, head); err != nil || got.Content != "KEEP" {
		t.Fatalf("empty content overwrote a snapshot: %+v err=%v", got, err)
	}
}

// The version list has to say whether a row can be previewed without carrying every body.
func TestProfileVersionListReportsSnapshotPresenceOnly(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	first := publishHead(t, h, "alice", alice, "첫 분석")
	if err := h.svc.RecordVersionSample(ctx, "alice", alice, `{"title":"본문"}`); err != nil {
		t.Fatal(err)
	}
	second, err := h.store.PublishProfileVersion(ctx, "alice", alice, voice.StructuredProfile{Empty: false}, "manual", 0, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	versions, err := h.store.ListProfileVersions(ctx, "alice", alice)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions = %+v err=%v", versions, err)
	}
	byVersion := map[int64]voice.ProfileVersion{}
	for _, version := range versions {
		byVersion[version.Version] = version
	}
	if !byVersion[first].HasSample {
		t.Fatalf("version %d lost its snapshot flag: %+v", first, byVersion[first])
	}
	if byVersion[second.Version].HasSample {
		t.Fatalf("version %d claims a snapshot it never produced", second.Version)
	}
}

// A10: the analysis text reaches the model ONCE, and the legacy header is gone for good.
func TestAnalysisTextReachesThePromptExactlyOnce(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	analysis := "## 1. 종결어미 분포\nANALYSIS_MARKER 해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
	h.addSample(t, "alice", alice, "sample", "글", longSample("글"), time.Now())
	h.models.response = analysis
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	projection, err := h.svc.PromptProfileForLanguage(ctx, "alice", alice, voice.LanguageKorean)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(projection.Styleguide, "ANALYSIS_MARKER"); got != 1 {
		t.Fatalf("analysis text appears %d times in the projection:\n%s", got, projection.Styleguide)
	}
	if strings.Contains(projection.Styleguide, "[Legacy manual guidance]") {
		t.Fatalf("legacy manual guidance header survived:\n%s", projection.Styleguide)
	}
}
