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
	"github.com/postpilot/backend/internal/quality"
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
			counts[devseed.StatusFinalized] != fixture.Finalized ||
			counts[devseed.StatusPublished] != fixture.Published {
			t.Errorf("account %q spread is draft=%d review=%d finalized=%d published=%d, want %d/%d/%d/%d",
				fixture.LoginID, counts[devseed.StatusDraft], counts[devseed.StatusReview], counts[devseed.StatusFinalized],
				counts[devseed.StatusPublished], fixture.Drafts, fixture.Reviews, fixture.Finalized, fixture.Published)
		}
	}
}

// The spread itself is the fixture, and the published half is what the quality surfaces read.
// The totals stay what they were before publishing existed, so a list screen's counts do not
// move under anyone.
func TestTheFixtureSpreadIsTheDeclaredOne(t *testing.T) {
	want := map[string][4]int{
		"free": {0, 0, 0, 0}, "base": {2, 1, 0, 0}, "pro": {3, 2, 1, 2},
		"max": {4, 3, 7, 0}, "master": {5, 4, 3, 11},
	}
	for _, fixture := range devseed.Fixtures {
		got := [4]int{fixture.Drafts, fixture.Reviews, fixture.Finalized, fixture.Published}
		if got != want[fixture.LoginID] {
			t.Errorf("account %q spread %v, want %v", fixture.LoginID, got, want[fixture.LoginID])
		}
	}
}

func TestRunReportsThePublishedCount(t *testing.T) {
	h := newHarness()
	report, err := devseed.Run(context.Background(), h.deps())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i, fixture := range devseed.Fixtures {
		if got := report.Accounts[i]; got.Published != fixture.Published || got.Posts() != fixture.Posts() {
			t.Errorf("report for %q says published=%d posts=%d, want %d and %d",
				fixture.LoginID, got.Published, got.Posts(), fixture.Published, fixture.Posts())
		}
	}
}

// POST-73, POST-77: every published post carries an address the post context's own rule
// accepts as it stands, published after it was written, before now, and at a moment no other
// post of its account shares.
func TestPublishedArticlesCarryAValidURLAndASpreadPublishedAt(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	published, want := 0, 0
	addresses := map[string]bool{}
	for _, fixture := range devseed.Fixtures {
		want += fixture.Published
		moments := map[time.Time]bool{}
		for _, article := range h.posts.byUser[fixture.LoginID] {
			if article.Status != devseed.StatusPublished {
				if article.PublishedURL != "" || !article.PublishedAt.IsZero() {
					t.Errorf("%s post %q carries a publication", article.Status, article.Title)
				}
				continue
			}
			published++
			normalized, err := post.ParseNaverBlogURL(article.PublishedURL)
			if err != nil {
				t.Fatalf("post %q address %q is refused: %v", article.Title, article.PublishedURL, err)
			}
			if normalized != article.PublishedURL || !strings.HasPrefix(normalized, "https://blog.naver.com/") {
				t.Errorf("post %q address %q is stored as %q", article.Title, article.PublishedURL, normalized)
			}
			if addresses[normalized] {
				t.Errorf("address %q is used twice", normalized)
			}
			addresses[normalized] = true
			if !article.PublishedAt.After(article.CreatedAt) || !article.PublishedAt.Before(fixed) {
				t.Errorf("post %q published at %s, created %s: want after creation and before now",
					article.Title, article.PublishedAt, article.CreatedAt)
			}
			if moments[article.PublishedAt] {
				t.Errorf("account %q publishes two posts at %s", fixture.LoginID, article.PublishedAt)
			}
			moments[article.PublishedAt] = true
		}
	}
	if published != want || published == 0 {
		t.Fatalf("%d published posts written, want %d", published, want)
	}
}

// GEN-55, QUAL-7: a generated post stores the nouns its write returned, and each stands in its
// text by the containment rule the measurements count with; at least one stands in the title.
func TestGeneratedArticlesCarryNounsTheirTextContains(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	checked := 0
	for _, article := range h.posts.written {
		if article.Content == nil {
			continue
		}
		content := article.Content
		if len(content.Nouns) < 4 || len(content.Nouns) > 6 {
			t.Errorf("post %q carries %d nouns, want 4 to 6", article.Title, len(content.Nouns))
		}
		texts := []string{content.Title}
		for _, block := range content.Blocks {
			texts = append(texts, block.Content)
			texts = append(texts, block.Items...)
		}
		text := quality.Tokens(strings.Join(texts, "\n"))
		title := quality.Tokens(content.Title)
		inTitle := false
		for _, noun := range content.Nouns {
			if !quality.Contains(text, quality.Tokens(noun), quality.LanguageKorean) {
				t.Errorf("post %q carries noun %q its text does not contain", article.Title, noun)
			}
			inTitle = inTitle || quality.Contains(title, quality.Tokens(noun), quality.LanguageKorean)
		}
		if !inTitle {
			t.Errorf("no noun of post %q stands in its title %q", article.Title, content.Title)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no article carried content, so no noun was checked")
	}
}

// GEN-53: a review post of an account with the list's 분야 offers one candidate per surface,
// each pointing at text that is there and offering phrases of that list; every other post
// offers none.
func TestReviewCandidatesPointAtTextThatIsThere(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	listed := map[string]bool{}
	for _, phrase := range devseed.PhraseList.Phrases {
		listed[phrase] = true
	}
	offering := 0
	for _, fixture := range devseed.Fixtures {
		for _, article := range h.posts.byUser[fixture.LoginID] {
			var candidates []devseed.Replacement
			if article.Content != nil {
				candidates = article.Content.Replacements
			}
			if article.Status != devseed.StatusReview || fixture.Field != devseed.PhraseList.Field {
				if len(candidates) != 0 {
					t.Errorf("%s post %q of %q offers %d candidates, want none",
						article.Status, article.Title, fixture.LoginID, len(candidates))
				}
				continue
			}
			offering++
			content := article.Content
			var surfaces []string
			for _, candidate := range candidates {
				surfaces = append(surfaces, candidate.Surface)
				source := quality.Tokens(candidate.Source)
				var there bool
				switch candidate.Surface {
				case devseed.SurfaceTitle:
					there = candidate.Index == 0 && quality.Contains(quality.Tokens(content.Title), source, quality.LanguageKorean)
				case devseed.SurfaceTag:
					there = candidate.Index < len(content.Tags) &&
						quality.Contains(quality.Tokens(content.Tags[candidate.Index]), source, quality.LanguageKorean)
				case devseed.SurfaceBody:
					there = candidate.Index < len(content.Blocks) && content.Blocks[candidate.Index].Type == devseed.BlockText &&
						quality.Contains(quality.Tokens(content.Blocks[candidate.Index].Content), source, quality.LanguageKorean)
				}
				if !there {
					t.Errorf("post %q candidate %+v points at text that is not there", article.Title, candidate)
				}
				if len(candidate.Phrases) == 0 || len(candidate.Phrases) > 3 {
					t.Errorf("post %q candidate %+v offers %d phrases, want 1 to 3", article.Title, candidate, len(candidate.Phrases))
				}
				for _, phrase := range candidate.Phrases {
					if !listed[phrase] || phrase == candidate.Source {
						t.Errorf("post %q candidate %+v offers %q", article.Title, candidate, phrase)
					}
				}
			}
			if strings.Join(surfaces, ",") != "title,tag,body" {
				t.Errorf("post %q offers candidates on %v, want title, tag and body", article.Title, surfaces)
			}
		}
	}
	if offering == 0 {
		t.Fatal("no review post offered candidates")
	}
}

func TestEveryPostOfAFieldAccountCarriesItsField(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	withField := map[string]bool{"pro": true, "master": true}
	for _, fixture := range devseed.Fixtures {
		want := ""
		if withField[fixture.LoginID] {
			want = devseed.FieldDailyLife
		}
		if fixture.Field != want {
			t.Errorf("account %q has 분야 %q, want %q", fixture.LoginID, fixture.Field, want)
		}
		for _, article := range h.posts.byUser[fixture.LoginID] {
			if article.Field != want {
				t.Errorf("post %q of %q carries 분야 %q, want %q", article.Title, fixture.LoginID, article.Field, want)
			}
		}
	}
}

// QUAL-38: the fixture has the shape the batch collects, so a screen shows what a real list
// would, and its 분야 is one the product knows.
func TestThePhraseListFixtureIsAValidList(t *testing.T) {
	list := devseed.PhraseList
	if !quality.Known(list.Field) {
		t.Fatalf("phrase list field %q is not on the product's list", list.Field)
	}
	if len(list.Phrases) == 0 || len(list.Phrases) > quality.PhraseListMax {
		t.Fatalf("phrase list holds %d phrases, want 1 to %d", len(list.Phrases), quality.PhraseListMax)
	}
	if list.CorpusSize <= 0 {
		t.Errorf("phrase list corpus size %d", list.CorpusSize)
	}
	seen := map[string]bool{}
	for _, phrase := range list.Phrases {
		if tokens := len(strings.Fields(phrase)); tokens < quality.PhraseMinTokens || tokens > quality.PhraseMaxTokens {
			t.Errorf("phrase %q has %d tokens, want %d to %d", phrase, tokens, quality.PhraseMinTokens, quality.PhraseMaxTokens)
		}
		if seen[phrase] {
			t.Errorf("phrase %q is listed twice", phrase)
		}
		seen[phrase] = true
	}
}

// QUAL-12: one account publishes enough for every metric's minimum, so every row can be judged,
// and another publishes some but fewer, so the minimum line shows. The bar is the quality
// context's own, so a raised minimum fails here instead of silently hiding a row state.
func TestOneAccountMeetsEveryPublishedMinimum(t *testing.T) {
	largest := 0
	for _, metric := range quality.Metrics() {
		largest = max(largest, quality.Minimum(metric))
	}
	meets, below := 0, 0
	for _, fixture := range devseed.Fixtures {
		switch {
		case fixture.Published >= largest:
			meets++
		case fixture.Published > 0:
			below++
		}
	}
	if meets != 1 || below == 0 {
		t.Fatalf("%d accounts meet the largest minimum %d and %d publish fewer, want exactly 1 and at least 1",
			meets, largest, below)
	}
}

// ARCH-44: the list is installation-wide, written once after every account, and after the wipe
// — which never reaches it. The report says what the port answered.
func TestRunEnsuresThePhraseListAfterTheWipe(t *testing.T) {
	for _, written := range []bool{true, false} {
		h := newHarness()
		h.phrases.written = written
		report, err := devseed.Run(context.Background(), h.deps())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if report.PhraseListWritten != written {
			t.Errorf("report says written=%v, the port answered %v", report.PhraseListWritten, written)
		}
		if len(h.phrases.ensured) != 1 || h.phrases.ensured[0].Field != devseed.PhraseList.Field || !h.phrases.at[0].Equal(fixed) {
			t.Fatalf("phrase lists ensured %+v at %v, want the fixture's once at the seed's clock", h.phrases.ensured, h.phrases.at)
		}
		if last := h.order[len(h.order)-1]; last != "phrases:"+devseed.PhraseList.Field {
			t.Errorf("the seed ended with %q, want the phrase list after every account", last)
		}
	}
}

// TMPL-50: the one account with the title-area template has it on its first draft, and no other
// post names a template.
func TestRunAssignsTheTitleAreaTemplateToAFirstDraft(t *testing.T) {
	h := newHarness()
	if _, err := devseed.Run(context.Background(), h.deps()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(h.templates.created) != 1 {
		t.Fatalf("%d templates created, want 1", len(h.templates.created))
	}
	created := h.templates.created[0]
	if created.UserID != "master" || created.Name != devseed.TitleAreaTemplate.Name ||
		strings.TrimSpace(created.TitleArea) == "" || created.Body != devseed.TitleAreaTemplate.Body {
		t.Fatalf("template created as %+v", created)
	}
	templateAt, firstPostAt := -1, -1
	for i, step := range h.order {
		if step == "template:master" && templateAt < 0 {
			templateAt = i
		}
		if step == "post:master" && firstPostAt < 0 {
			firstPostAt = i
		}
	}
	if templateAt < 0 || templateAt > firstPostAt {
		t.Errorf("the template was created at step %d and master's first post at %d", templateAt, firstPostAt)
	}
	for _, fixture := range devseed.Fixtures {
		for i, article := range h.posts.byUser[fixture.LoginID] {
			want := ""
			if fixture.Template && i == 0 {
				want = templateFor(fixture.LoginID)
				if article.Status != devseed.StatusDraft {
					t.Errorf("the template's post %q is %s, want a draft", article.Title, article.Status)
				}
			}
			if article.TemplateID != want {
				t.Errorf("post %q of %q names template %q, want %q", article.Title, fixture.LoginID, article.TemplateID, want)
			}
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
	h.posts.failOn = "pro"

	_, err := devseed.Run(context.Background(), h.deps())
	if err == nil {
		t.Fatal("Run succeeded although a post write failed")
	}
	if !strings.Contains(err.Error(), "pro") {
		t.Fatalf("error %q does not name the account it stopped on", err)
	}
}

func TestRunRefusesAMissingCollaborator(t *testing.T) {
	for name, drop := range map[string]func(*devseed.Deps){
		"voices":       func(d *devseed.Deps) { d.Voices = nil },
		"templates":    func(d *devseed.Deps) { d.Templates = nil },
		"phrase lists": func(d *devseed.Deps) { d.PhraseLists = nil },
	} {
		h := newHarness()
		deps := h.deps()
		drop(&deps)

		if _, err := devseed.Run(context.Background(), deps); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("Run with no %s collaborator answered %v", name, err)
		}
		if len(h.accounts.created) != 0 {
			t.Fatalf("accounts were created although the %s collaborator was missing", name)
		}
		if h.accounts.deleted {
			t.Fatalf("the installation was wiped although the %s collaborator was missing", name)
		}
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

func templateFor(userID string) string { return "template-" + userID }

type harness struct {
	accounts  *fakeAccounts
	voices    *fakeVoices
	credits   *fakeCredits
	posts     *fakePosts
	media     *fakeMedia
	templates *fakeTemplates
	phrases   *fakePhraseLists
	order     []string
}

func newHarness() *harness {
	h := &harness{}
	record := func(step string) { h.order = append(h.order, step) }
	h.accounts = &fakeAccounts{record: record, created: map[string]createdAccount{}}
	h.voices = &fakeVoices{record: record, ensured: map[string]int{}}
	h.credits = &fakeCredits{record: record, opened: map[string]plan.Plan{}}
	h.posts = &fakePosts{record: record, byUser: map[string][]devseed.Article{}}
	h.media = &fakeMedia{record: record}
	h.templates = &fakeTemplates{record: record}
	h.phrases = &fakePhraseLists{record: record, written: true}
	return h
}

func (h *harness) deps() devseed.Deps {
	return devseed.Deps{
		Accounts:    h.accounts,
		Voices:      h.voices,
		Credits:     h.credits,
		Posts:       h.posts,
		Media:       h.media,
		Templates:   h.templates,
		PhraseLists: h.phrases,
		Now:         func() time.Time { return fixed },
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

type fakeTemplates struct {
	record  func(string)
	created []devseed.Template
}

func (f *fakeTemplates) Create(_ context.Context, template devseed.Template) (string, error) {
	f.record("template:" + template.UserID)
	f.created = append(f.created, template)
	return templateFor(template.UserID), nil
}

type fakePhraseLists struct {
	record func(string)
	// written is what EnsureList answers: false models a list already standing.
	written bool
	ensured []devseed.PhraseListFixture
	at      []time.Time
}

func (f *fakePhraseLists) EnsureList(_ context.Context, list devseed.PhraseListFixture, at time.Time) (bool, error) {
	f.record("phrases:" + list.Field)
	f.ensured = append(f.ensured, list)
	f.at = append(f.at, at)
	return f.written, nil
}
