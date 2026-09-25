package post

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// listPageSizeMax bounds one page of the list. The browser asks for far fewer (POST-90); the
// bound only keeps a hand-built request from turning a page back into the whole account.
const listPageSizeMax = 100

// ListCursor is a row's position in the list order: the stored updated_at string and the slug,
// compared exactly the way the store's ORDER BY compares them. The stored string and not the
// parsed time, because a row written before the timestamp width was pinned re-formats to a
// different string, and a cursor that disagrees with the ORDER BY repeats or skips rows.
type ListCursor struct {
	UpdatedAt string
	Slug      string
}

// ListFilter bounds one store read of the list. Limit -1 reads every row the filter keeps.
type ListFilter struct {
	Status string
	After  *ListCursor
	Limit  int
}

// ListQuery is one list request. PageSize 0 is the whole narrowed list, which is what a
// client that predates paging asks for by sending nothing.
type ListQuery struct {
	PageSize  int
	PageToken string
	Query     string
	Status    string
}

// ListPage is one page of the narrowed list, newest first. NextPageToken is empty on the
// page holding the last narrowed post.
type ListPage struct {
	Summaries     []Summary
	NextPageToken string
}

// List answers one page of the caller's posts, narrowed by the search and the status over
// every post the account owns rather than over any page (POST-91).
func (s *Service) List(ctx context.Context, userID string, q ListQuery) (ListPage, error) {
	if q.PageSize < 0 {
		return ListPage{}, fmt.Errorf("%w: page size %d", ErrInvalidListRequest, q.PageSize)
	}
	if q.Status != "" && !isStatus(q.Status) {
		return ListPage{}, fmt.Errorf("%w: status %q", ErrInvalidListRequest, q.Status)
	}
	after, err := decodeListToken(q.PageToken)
	if err != nil {
		return ListPage{}, err
	}
	size := min(q.PageSize, listPageSizeMax)
	query := normalizeListText(q.Query)

	// Without a search the store stops at the page; with one it cannot, because the match
	// runs here: SQLite folds ASCII case only and cannot collapse the whitespace inside the
	// stored text, so a LIKE would disagree with POST-65 on exactly the cases it names. One
	// row past the page says whether there is a next one.
	limit := -1
	if size > 0 && query == "" {
		limit = size + 1
	}
	rows, err := s.posts.ListPosts(ctx, userID, ListFilter{Status: q.Status, After: after, Limit: limit})
	if err != nil {
		return ListPage{}, fmt.Errorf("list posts: %w", err)
	}
	if query != "" {
		kept := rows[:0]
		for _, row := range rows {
			if summaryMatches(row.Title, row.Tags, query) {
				kept = append(kept, row)
			}
		}
		rows = kept
	}
	var page ListPage
	if size > 0 && len(rows) > size {
		rows = rows[:size]
		page.NextPageToken = encodeListToken(rows[size-1].Cursor)
	}
	if err := s.decorateSummaries(ctx, userID, rows); err != nil {
		return ListPage{}, err
	}
	page.Summaries = rows
	return page, nil
}

// decorateSummaries resolves what a row shows beyond its own columns — the running job, the
// undecided comparison, the voice and the template — for the rows being answered only.
func (s *Service) decorateSummaries(ctx context.Context, userID string, summaries []Summary) error {
	var err error
	if s.jobs != nil {
		for i := range summaries {
			summaries[i].ActiveJob, err = s.jobs.ActiveForPost(ctx, summaries[i].Slug)
			if err != nil {
				return fmt.Errorf("load active job for %s: %w", summaries[i].Slug, err)
			}
		}
	}
	if s.experiments != nil {
		for i := range summaries {
			summaries[i].PendingExperimentID, err = s.experiments.PendingForPost(ctx, userID, summaries[i].Slug)
			if err != nil {
				return fmt.Errorf("load pending experiment for %s: %w", summaries[i].Slug, err)
			}
		}
	}
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return err
	}
	templateRefs, err := s.templateRefs(ctx, userID)
	if err != nil {
		return err
	}
	for i := range summaries {
		summaries[i].Voice = projectVoice(refs, summaries[i].VoiceID)
		summaries[i].Template = projectTemplate(templateRefs, summaries[i].TemplateID)
	}
	return nil
}

func isStatus(status string) bool {
	switch status {
	case StatusDraft, StatusReview, StatusFinalized, StatusPublished:
		return true
	}
	return false
}

// normalizeListText is POST-65's normalization, step for step the browser's `normalize`
// (features/filter-posts/model/narrow.ts), which names the matched tags on a row with it: the
// two must agree on what matched. Like the browser it applies to the query, the title and each
// tag alike: trimmed, a leading `#` dropped (someone typing `#제주` is naming a tag), every
// whitespace run collapsed to one space, and case-folded.
func normalizeListText(value string) string {
	value = strings.TrimFunc(value, isListSpace)
	value = strings.TrimLeft(value, "#")
	return strings.ToLower(collapseListSpace(value))
}

// collapseListSpace replaces every whitespace run with one space, keeping a run at either end
// as one space, as the browser's `/\s+/g` replace does.
func collapseListSpace(value string) string {
	var b strings.Builder
	space := false
	for _, r := range value {
		if isListSpace(r) {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

// isListSpace is JavaScript's `\s`, which is what the browser's normalization collapses.
func isListSpace(r rune) bool {
	switch r {
	case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// summaryMatches is POST-65's match of an already normalized, non-empty query: a substring of
// the title as listed, or of any current tag.
func summaryMatches(title string, tags []string, query string) bool {
	if strings.Contains(normalizeListText(title), query) {
		return true
	}
	for _, tag := range tags {
		if strings.Contains(normalizeListText(tag), query) {
			return true
		}
	}
	return false
}

// listTokenSeparator cannot occur in a stored timestamp, so the slug after it is taken whole.
const listTokenSeparator = "\x00"

func encodeListToken(cursor ListCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursor.UpdatedAt + listTokenSeparator + cursor.Slug))
}

func decodeListToken(token string) (*ListCursor, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("%w: page token: %v", ErrInvalidListRequest, err)
	}
	updatedAt, slug, found := strings.Cut(string(raw), listTokenSeparator)
	if !found || updatedAt == "" || slug == "" {
		return nil, fmt.Errorf("%w: page token", ErrInvalidListRequest)
	}
	return &ListCursor{UpdatedAt: updatedAt, Slug: slug}, nil
}
