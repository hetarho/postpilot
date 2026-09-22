package experiment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func pairWithBadges(t *testing.T) (*Service, *memoryStore, Experiment) {
	t.Helper()
	svc, store, _, _, _ := newTestService()
	pair := ready(t, svc, store, writeRequest(OriginLab))
	return svc, store, pair
}

// A verdict may explain itself for both candidates at once, positive and negative mixed, and
// the explanation is written with the verdict rather than after it (MODEL-61, MODEL-63).
func TestAVerdictCarriesBadgesForBothCandidates(t *testing.T) {
	svc, store, pair := pairWithBadges(t)
	chosen, unchosen := pair.Candidates[0].ID, pair.Candidates[1].ID
	decided, err := svc.Choose(context.Background(), "alice", pair.ID, chosen, false, []CandidateBadges{
		{CandidateID: chosen, Badges: []Badge{BadgeFast, BadgeInVoice}},
		{CandidateID: unchosen, Badges: []Badge{BadgeAILike, BadgeOther}, OtherNote: "제목이 비슷해요"},
	})
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if decided.Status != StatusDecided || decided.WinnerCandidateID != chosen {
		t.Fatalf("verdict = %+v", decided)
	}
	reloaded, err := store.Get(context.Background(), pair.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range reloaded.Candidates {
		switch candidate.ID {
		case chosen:
			if len(candidate.Badges) != 2 || candidate.Badges[0] != BadgeFast || candidate.Badges[1] != BadgeInVoice {
				t.Fatalf("chosen badges = %v", candidate.Badges)
			}
			if candidate.OtherNote != "" {
				t.Fatalf("chosen carried a note it was not given: %q", candidate.OtherNote)
			}
		case unchosen:
			if candidate.OtherNote != "제목이 비슷해요" {
				t.Fatalf("unchosen note = %q", candidate.OtherNote)
			}
		}
	}
}

// Zero badges confirm as readily as ten: a badge is evidence offered, never a toll on the
// verdict (MODEL-61).
func TestAVerdictWithNoBadgesIsRecordedJustTheSame(t *testing.T) {
	for _, badges := range [][]CandidateBadges{nil, {}, {{CandidateID: "ignored-empty"}}} {
		svc, _, pair := pairWithBadges(t)
		offered := badges
		if len(offered) == 1 {
			offered = []CandidateBadges{{CandidateID: pair.Candidates[0].ID}}
		}
		decided, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, offered)
		if err != nil || decided.Status != StatusDecided {
			t.Fatalf("verdict without badges = %+v, %v", decided, err)
		}
	}
}

// What a comparison cannot carry is refused before the verdict is written, so a rejected
// explanation never leaves a verdict recorded without it.
func TestBadgesOutsideTheComparisonAreRefusedBeforeTheVerdict(t *testing.T) {
	cases := []struct {
		name  string
		stage Stage
		build func(pair Experiment) []CandidateBadges
	}{
		{"a candidate of another comparison", StageWrite, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{{CandidateID: "someone-elses", Badges: []Badge{BadgeFast}}}
		}},
		{"the same candidate twice", StageWrite, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{
				{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeFast}},
				{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeSlow}},
			}
		}},
		{"a badge outside the catalog", StageWrite, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{{CandidateID: pair.Candidates[0].ID, Badges: []Badge{Badge("vibes")}}}
		}},
		{"a note with no badge that explains it", StageWrite, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeFast}, OtherNote: "설명"}}
		}},
		{"a note past its bound", StageWrite, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{{
				CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeOther},
				OtherNote: strings.Repeat("가", BadgeNoteMaxLength+1),
			}}
		}},
		{"a voice judgement of a comparison that wrote no prose", StageObserve, func(pair Experiment) []CandidateBadges {
			return []CandidateBadges{{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeInVoice}}}
		}},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			svc, store, _, _, _ := newTestService()
			request := writeRequest(OriginLab)
			if sample.stage == StageObserve {
				request.Stage = StageObserve
			}
			pair := ready(t, svc, store, request)
			if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, sample.build(pair)); !errors.Is(err, ErrBadgesInvalid) {
				t.Fatalf("choose = %v, want ErrBadgesInvalid", err)
			}
			after, _ := store.Get(context.Background(), pair.ID)
			if after.Status != StatusReview || after.WinnerCandidateID != "" {
				t.Fatalf("a refused explanation still recorded a verdict: %+v", after)
			}
		})
	}
}

// A note bounded exactly at the limit is accepted, and the limit counts characters rather
// than bytes so Korean prose is not cut to a third of what the field promises.
func TestABadgeNoteIsBoundedInCharacters(t *testing.T) {
	svc, store, pair := pairWithBadges(t)
	note := strings.Repeat("가", BadgeNoteMaxLength)
	if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, []CandidateBadges{
		{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeOther}, OtherNote: note},
	}); err != nil {
		t.Fatalf("a note at the bound = %v", err)
	}
	reloaded, _ := store.Get(context.Background(), pair.ID)
	if reloaded.Candidates[0].OtherNote != note && reloaded.Candidates[1].OtherNote != note {
		t.Fatal("the note at the bound was not stored")
	}
}

// Badges explain a verdict; they do not amplify it. A board built from the same verdicts
// ranks identically whether or not they carried any (MODEL-63).
func TestBadgesNeverEnterARating(t *testing.T) {
	ratings := map[string]int{}
	for _, name := range []string{"bare", "badged"} {
		svc, store, pair := pairWithBadges(t)
		var badges []CandidateBadges
		if name == "badged" {
			badges = []CandidateBadges{
				{CandidateID: pair.Candidates[0].ID, Badges: []Badge{BadgeFast, BadgeAccurate, BadgeConcise}},
				{CandidateID: pair.Candidates[1].ID, Badges: []Badge{BadgeSlow, BadgeVerbose}},
			}
		}
		decided, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, badges)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, ScopeMe)
		if err != nil {
			t.Fatal(err)
		}
		winner := decided.Winner()
		ratings[name] = ratingOf(entries, winner.Model)
		_ = store
	}
	if ratings["bare"] != ratings["badged"] || ratings["bare"] == 0 {
		t.Fatalf("badges moved a rating: %v", ratings)
	}
}

// Dismissal chooses nothing, so there is nothing to explain and no sheet opens for it.
func TestDismissalCarriesNoBadges(t *testing.T) {
	svc, store, pair := pairWithBadges(t)
	if _, err := svc.Dismiss(context.Background(), "alice", pair.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Get(context.Background(), pair.ID)
	for _, candidate := range reloaded.Candidates {
		if len(candidate.Badges) != 0 || candidate.OtherNote != "" {
			t.Fatalf("dismissal recorded an explanation: %+v", candidate)
		}
	}
}
