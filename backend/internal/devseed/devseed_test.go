package devseed_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/post"
)

// fixed is the clock every test runs against, so the dates the fixture derives are
// assertable rather than whatever time the suite happened to run at.
var fixed = time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

func TestRunEmptiesTheInstallationBeforeWritingAnything(t *testing.T) {
	h := newHarness()
	// Something is already here, as it would be on a machine that has been used.
	h.accounts.existing = 7
	h.media.existing = 3

	report, err := devseed.Run(context.Background(), h.deps())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.DeletedAccounts != 7 || report.DeletedBatches != 3 {
		t.Fatalf("report deleted accounts=%d batches=%d, want 7 and 3", report.DeletedAccounts, report.DeletedBatches)
	}
	// The wipe has to precede every write, or a rerun would collide with the ids it is
	// about to create instead of replacing them.
	if h.order[0] != "delete-accounts" || h.order[1] != "delete-batches" {
		t.Fatalf("seed started with %v, want the wipe first", h.order[:2])
	}
	for _, step := range h.order[2:] {
		if strings.HasPrefix(step, "delete") {
			t.Fatalf("a delete ran after the first write: %v", h.order)
		}
	}
}

func TestRunWritesEveryFixtureAccount(t *testing.T) {
	h := newHarness()
	report, err := devseed.Run(context.Background(), h.deps())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Accounts) != len(devseed.Fixtures) {
		t.Fatalf("seeded %d accounts, want %d", len(report.Accounts), len(devseed.Fixtures))
	}
	for i, fixture := range devseed.Fixtures {
		created, ok := h.accounts.created[fixture.LoginID]
		if !ok {
			t.Fatalf("account %q was never created", fixture.LoginID)
		}
		if created.tier != fixture.Plan {
			t.Errorf("account %q on plan %q, want %q", fixture.LoginID, created.tier, fixture.Plan)
		}
		if created.password != devseed.Password {
			t.Errorf("account %q password %q, want the shared fixture password", fixture.LoginID, created.password)
		}
		// Unverified accounts cannot reach the screens the seed exists to fill.
		if got := h.accounts.verified[fixture.LoginID]; got != fixture.Email() {
			t.Errorf("account %q verified as %q, want %q", fixture.LoginID, got, fixture.Email())
		}
		if h.credits.opened[fixture.LoginID] != fixture.Plan {
			t.Errorf("account %q got no monthly grant for its plan", fixture.LoginID)
		}
		if h.voices.ensured[fixture.LoginID] != 1 {
			t.Errorf("account %q had its default voice established %d times, want once", fixture.LoginID, h.voices.ensured[fixture.LoginID])
		}
		if report.Accounts[i].LoginID != fixture.LoginID {
			t.Errorf("report row %d is %q, want %q in fixture order", i, report.Accounts[i].LoginID, fixture.LoginID)
		}
	}
}

func TestRunWritesThePostSpreadEachAccountDeclares(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, fixture := range devseed.Fixtures {
		written := h.posts.byUser[fixture.LoginID]
		if len(written) != fixture.Posts() {
			t.Fatalf("account %q holds %d posts, want %d", fixture.LoginID, len(written), fixture.Posts())
		}
		counts := map[string]int{}
		for _, article := range written {
			counts[article.Status]++
		}
		if counts[devseed.StatusDraft] != fixture.Drafts ||
			counts[devseed.StatusReview] != fixture.Reviews ||
			counts[devseed.StatusFinalized] != fixture.Finalized {
			t.Errorf("account %q spread is draft=%d review=%d finalized=%d, want %d/%d/%d",
				fixture.LoginID, counts[devseed.StatusDraft], counts[devseed.StatusReview], counts[devseed.StatusFinalized],
				fixture.Drafts, fixture.Reviews, fixture.Finalized)
		}
	}
}

// The empty account is the state every new signup opens in, and the one a change that
// assumes at least one row breaks first. It is pinned so nobody "helpfully" gives it posts.
func TestOneFixtureAccountIsDeliberatelyEmpty(t *testing.T) {
	empty := 0
	for _, fixture := range devseed.Fixtures {
		if fixture.Posts() == 0 {
			empty++
		}
	}
	if empty != 1 {
		t.Fatalf("%d fixture accounts hold no posts, want exactly 1", empty)
	}
}

func TestADraftCarriesNoContentAndTheOthersDo(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, article := range h.posts.written {
		switch article.Status {
		case devseed.StatusDraft:
			// NULL content is what distinguishes "never generated" from "generated empty".
			if article.Content != nil {
				t.Fatalf("draft %q carries content", article.Title)
			}
		default:
			if article.Content == nil {
				t.Fatalf("%s post %q carries no content", article.Status, article.Title)
			}
		}
	}
}

// Every seeded article has to satisfy the same content rule the editor and the generation
// pipeline apply, or the seed would produce posts no screen can render. A seeded post has
// no attachments, so this also pins that no fixture block names a file.
func TestEveryFixtureArticlePassesTheDraftingContextsOwnValidation(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	checked := 0
	for _, article := range h.posts.written {
		if article.Content == nil {
			continue
		}
		blocks := make([]post.Block, 0, len(article.Content.Blocks))
		for _, block := range article.Content.Blocks {
			blocks = append(blocks, post.Block{
				Type:    post.BlockType(block.Type),
				Content: block.Content,
				Level:   block.Level,
				Items:   block.Items,
			})
		}
		content := post.PostContent{
			Title:   article.Content.Title,
			Summary: article.Content.Summary,
			Tags:    article.Content.Tags,
			Blocks:  blocks,
		}
		if err := post.ValidateContent(content, nil, nil); err != nil {
			t.Fatalf("article %q is not valid content: %v", article.Title, err)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no article carried content, so nothing was validated")
	}
}

func TestEveryArticleNamesItsAccountsOwnVoiceAndATargetLanguage(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, article := range h.posts.written {
		// [I4]: voices are isolated per account and every post selects exactly one.
		if want := voiceFor(article.UserID); article.VoiceID != want {
			t.Errorf("post %q names voice %q, want %q", article.Title, article.VoiceID, want)
		}
		if article.Language == "" {
			t.Errorf("post %q names no target language", article.Title)
		}
	}
}

// The list screen sorts and groups by date, so two seeded posts must never share a
// timestamp and all of them must be in the past.
func TestSeededPostsAreSpreadBackwardsInTime(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, fixture := range devseed.Fixtures {
		seen := map[time.Time]string{}
		for _, article := range h.posts.byUser[fixture.LoginID] {
			if !article.CreatedAt.Before(fixed) {
				t.Errorf("post %q is dated %s, which is not in the past", article.Title, article.CreatedAt)
			}
			if other, clash := seen[article.CreatedAt]; clash {
				t.Errorf("posts %q and %q share a timestamp", other, article.Title)
			}
			seen[article.CreatedAt] = article.Title
		}
	}
}

// A repeat suffix means "the second post with this title in this account". An account
// whose posts all read "(2)" with no "(1)" anywhere reads as a bug in the list, which is
// exactly what counting anything but the account's own repeats produces.
func TestATitleIsSuffixedOnlyOnceTheAccountActuallyRepeatsOne(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, fixture := range devseed.Fixtures {
		bases := map[string]int{}
		for _, article := range h.posts.byUser[fixture.LoginID] {
			base, suffix := splitRepeat(article.Title)
			bases[base]++
			if suffix == 0 {
				continue
			}
			// The (n)th of a title can only exist once n-1 of them already do.
			if bases[base] != suffix {
				t.Errorf("account %q has %q as repeat %d but holds %d of that title so far",
					fixture.LoginID, article.Title, suffix, bases[base])
			}
		}
	}
}

// Two posts in one account never carry the same title: a list where every row reads the
// same proves nothing about ordering, selection or navigation.
func TestTitlesAreDistinctWithinAnAccount(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, fixture := range devseed.Fixtures {
		seen := map[string]bool{}
		for _, article := range h.posts.byUser[fixture.LoginID] {
			if seen[article.Title] {
				t.Errorf("account %q holds two posts titled %q", fixture.LoginID, article.Title)
			}
			seen[article.Title] = true
		}
	}
}

// splitRepeat reads a fixture title back into its topic and its repeat number, where an
// unsuffixed title is the first of its kind and reports 1.
func splitRepeat(title string) (string, int) {
	open := strings.LastIndex(title, " (")
	if open < 0 || !strings.HasSuffix(title, ")") {
		return title, 1
	}
	number, err := strconv.Atoi(title[open+2 : len(title)-1])
	if err != nil {
		return title, 1
	}
	return title[:open], number
}

func TestRunReportsWhichAccountItFailedOn(t *testing.T) {
	h := newHarness()
	h.posts.failOn = "seed-pro"

	_, err := devseed.Run(context.Background(), h.deps())
	if err == nil {
		t.Fatal("Run succeeded although a post write failed")
	}
	if !strings.Contains(err.Error(), "seed-pro") {
		t.Fatalf("error %q does not name the account it stopped on", err)
	}
}

func TestRunRefusesAMissingCollaborator(t *testing.T) {
	h := newHarness()
	deps := h.deps()
	deps.Voices = nil

	if _, err := devseed.Run(context.Background(), deps); err == nil {
		t.Fatal("Run accepted deps with no voice collaborator")
	}
	if len(h.accounts.created) != 0 {
		t.Fatal("accounts were created although the deps were incomplete")
	}
	if h.accounts.deleted {
		t.Fatal("the installation was wiped although the deps were incomplete")
	}
}

// Fixture properties that are the point of the fixture rather than incidental to it.
func TestFixturesCoverTheWholePlanLadderWithDistinctLogins(t *testing.T) {
	logins := map[string]bool{}
	tiers := map[plan.Plan]bool{}
	for _, fixture := range devseed.Fixtures {
		if logins[fixture.LoginID] {
			t.Fatalf("login %q appears twice", fixture.LoginID)
		}
		logins[fixture.LoginID] = true
		tiers[fixture.Plan] = true
	}
	for _, tier := range []plan.Plan{plan.Free, plan.Basic, plan.Pro, plan.Max, plan.Master} {
		if !tiers[tier] {
			t.Errorf("no fixture account is on the %s plan", tier)
		}
	}
}

func TestEachAccountHoldsADifferentNumberOfPosts(t *testing.T) {
	counts := map[int]string{}
	for _, fixture := range devseed.Fixtures {
		if other, clash := counts[fixture.Posts()]; clash {
			t.Errorf("%q and %q both hold %d posts, so the spread proves nothing", other, fixture.LoginID, fixture.Posts())
		}
		counts[fixture.Posts()] = fixture.LoginID
	}
}

// --- harness ---------------------------------------------------------------------------

func voiceFor(userID string) string { return "voice-" + userID }

type harness struct {
	accounts *fakeAccounts
	voices   *fakeVoices
	credits  *fakeCredits
	posts    *fakePosts
	media    *fakeMedia
	order    []string
}

func newHarness() *harness {
	h := &harness{}
	record := func(step string) { h.order = append(h.order, step) }
	h.accounts = &fakeAccounts{record: record, created: map[string]createdAccount{}, verified: map[string]string{}}
	h.voices = &fakeVoices{record: record, ensured: map[string]int{}}
	h.credits = &fakeCredits{record: record, opened: map[string]plan.Plan{}}
	h.posts = &fakePosts{record: record, byUser: map[string][]devseed.Article{}}
	h.media = &fakeMedia{record: record}
	return h
}

func (h *harness) deps() devseed.Deps {
	return devseed.Deps{
		Accounts: h.accounts,
		Voices:   h.voices,
		Credits:  h.credits,
		Posts:    h.posts,
		Media:    h.media,
		Now:      func() time.Time { return fixed },
	}
}

type createdAccount struct {
	password string
	tier     plan.Plan
}

type fakeAccounts struct {
	record   func(string)
	existing int64
	deleted  bool
	created  map[string]createdAccount
	verified map[string]string
}

func (f *fakeAccounts) DeleteAll(context.Context) (int64, error) {
	f.record("delete-accounts")
	f.deleted = true
	return f.existing, nil
}

func (f *fakeAccounts) Create(_ context.Context, loginID, password string, tier plan.Plan) error {
	f.record("create:" + loginID)
	if _, exists := f.created[loginID]; exists {
		return fmt.Errorf("duplicate account %q", loginID)
	}
	f.created[loginID] = createdAccount{password: password, tier: tier}
	return nil
}

func (f *fakeAccounts) VerifyEmail(_ context.Context, loginID, email string, _ time.Time) error {
	f.record("verify:" + loginID)
	f.verified[loginID] = email
	return nil
}

type fakeVoices struct {
	record  func(string)
	ensured map[string]int
}

func (f *fakeVoices) EnsureDefault(_ context.Context, userID string) (string, error) {
	f.record("voice:" + userID)
	f.ensured[userID]++
	return voiceFor(userID), nil
}

type fakeCredits struct {
	record func(string)
	opened map[string]plan.Plan
}

func (f *fakeCredits) OpenMonthlyLot(_ context.Context, userID string, tier plan.Plan) error {
	f.record("credits:" + userID)
	f.opened[userID] = tier
	return nil
}

type fakePosts struct {
	record  func(string)
	failOn  string
	written []devseed.Article
	byUser  map[string][]devseed.Article
}

func (f *fakePosts) Write(_ context.Context, article devseed.Article) error {
	f.record("post:" + article.UserID)
	if article.UserID == f.failOn {
		return errors.New("store is unavailable")
	}
	f.written = append(f.written, article)
	f.byUser[article.UserID] = append(f.byUser[article.UserID], article)
	return nil
}

type fakeMedia struct {
	record   func(string)
	existing int64
}

func (f *fakeMedia) DeleteAllSourceBatches(context.Context) (int64, error) {
	f.record("delete-batches")
	return f.existing, nil
}
