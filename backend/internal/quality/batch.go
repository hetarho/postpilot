package quality

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SearchItem is one search result as the phrase batch reads it.
type SearchItem struct {
	Title, Description string
}

// PhraseBatch keeps each 분야's phrase list fresh (QUAL-17, QUAL-38). It is an in-process loop
// rather than a job: a phrase list is installation-wide product data with no owner, while every
// job belongs to an account. Each field carries its own next refresh time, so a restart
// catches up on exactly what fell due.
type PhraseBatch struct {
	store    PhraseLists
	search   BlogSearch
	interval time.Duration
	now      func() time.Time
}

// NewPhraseBatch builds the batch. A nil search is the legal disabled mode (QUAL-42): the box has
// no Naver keys, no pass runs and every list stays empty. A non-positive interval is the
// product default.
func NewPhraseBatch(store PhraseLists, search BlogSearch, interval time.Duration, now func() time.Time) *PhraseBatch {
	if store == nil {
		panic("quality: phrase store collaborator is required")
	}
	if now == nil {
		panic("quality: phrase clock collaborator is required")
	}
	if interval <= 0 {
		interval = PhraseRefreshInterval
	}
	return &PhraseBatch{store: store, search: search, interval: interval, now: now}
}

// Run catches up once and then checks every min(interval, PhraseRefreshCheck) until the context
// ends, all inside the caller's goroutine, so a slow search never holds the listener shut. A
// failed pass is logged and costs its own tick and nothing else.
func (b *PhraseBatch) Run(ctx context.Context) {
	if b.search == nil {
		slog.Info("quality phrase refresh disabled: no Naver search credentials")
		return
	}
	ticker := time.NewTicker(min(b.interval, PhraseRefreshCheck))
	defer ticker.Stop()
	if err := b.RunOnce(ctx); err != nil && ctx.Err() == nil {
		slog.Error("quality phrase refresh failed", "err", err, "boot", true)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.RunOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Error("quality phrase refresh failed", "err", err)
			}
		}
	}
}

// RunOnce refreshes every due field in the catalogue's order. A field is due with no row or a
// next refresh at or before now. Its pages are all fetched before its one write, so no
// transaction is open while a request is in flight. A failed field keeps its last list and is
// tried again after the retry delay; it never stops the others, and the pass answers every
// field's error joined. A cancelled context stops the pass at once, writing nothing for the
// field in hand, which stays due for the next catch-up.
func (b *PhraseBatch) RunOnce(ctx context.Context) error {
	if b.search == nil {
		return nil
	}
	var errs []error
	for _, field := range Fields() {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		stored, found, err := b.store.PhraseList(ctx, field.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("refresh %s: %w", field.ID, err))
			continue
		}
		if found && stored.NextRefreshAt.After(b.now()) {
			continue
		}
		items, fetchErr := b.fetch(ctx, field.Query)
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		now := b.now()
		if fetchErr != nil {
			row := PhraseList{Field: field.ID, Phrases: []string{}}
			if found {
				row = stored
			}
			row.NextRefreshAt = now.Add(min(b.interval, PhraseRetryDelay))
			if err := b.store.ReplacePhraseList(ctx, row); err != nil {
				errs = append(errs, fmt.Errorf("refresh %s: %w", field.ID, err))
			}
			errs = append(errs, fmt.Errorf("refresh %s: %w", field.ID, fetchErr))
			continue
		}
		phrases := ExtractPhrases(unitsOf(items))
		texts := make([]string, len(phrases))
		for i, phrase := range phrases {
			texts[i] = phrase.Text
		}
		if err := b.store.ReplacePhraseList(ctx, PhraseList{
			Field: field.ID, Phrases: texts, CorpusSize: len(items), RefreshedAt: &now, NextRefreshAt: now.Add(b.interval),
		}); err != nil {
			errs = append(errs, fmt.Errorf("refresh %s: %w", field.ID, err))
		}
	}
	return errors.Join(errs...)
}

// fetch reads up to PhrasePages pages, stopping after a short one: fewer results than a page
// means the search has no more.
func (b *PhraseBatch) fetch(ctx context.Context, query string) ([]SearchItem, error) {
	var items []SearchItem
	for page := range PhrasePages {
		got, err := b.search.SearchBlog(ctx, query, 1+page*PhrasePageSize)
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
		if len(got) < PhrasePageSize {
			break
		}
	}
	return items, nil
}

// unitsOf flattens the results into the units a phrase is counted once per: each non-blank
// title and description.
func unitsOf(items []SearchItem) []string {
	units := make([]string, 0, 2*len(items))
	for _, item := range items {
		for _, text := range []string{item.Title, item.Description} {
			if strings.TrimSpace(text) != "" {
				units = append(units, text)
			}
		}
	}
	return units
}
