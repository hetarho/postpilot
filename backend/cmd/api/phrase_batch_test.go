package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/naversearch"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/quality"
)

// The batch reads disabled as a nil search, so the adapter must answer a true nil interface: a
// typed nil would pass the batch's check and call through to nothing (QUAL-42).
func TestPhraseSearchIsAbsentWithoutBothKeys(t *testing.T) {
	for name, cfg := range map[string]*config.Config{
		"no keys":         {},
		"only the id":     {NaverSearchClientID: "id"},
		"only the secret": {NaverSearchClientSecret: "secret"},
	} {
		if search := phraseSearch(cfg); search != nil {
			t.Errorf("%s: phraseSearch = %#v, want a nil interface", name, search)
		}
	}
	search := phraseSearch(&config.Config{NaverSearchClientID: "id", NaverSearchClientSecret: "secret", NaverSearchEnabled: true})
	adapter, ok := search.(qualityBlogSearch)
	if !ok {
		t.Fatalf("phraseSearch with both keys = %#v, want the Naver adapter", search)
	}
	if _, ok := adapter.client.(*naversearch.Client); !ok {
		t.Fatalf("the adapter wraps %#v, want the Naver client", adapter.client)
	}
}

type fakeNaverBlog struct {
	query string
	start int
	items []naversearch.Item
	err   error
}

func (f *fakeNaverBlog) SearchBlog(_ context.Context, query string, start int) ([]naversearch.Item, error) {
	f.query, f.start = query, start
	return f.items, f.err
}

func TestQualityBlogSearchMapsItemsInOrder(t *testing.T) {
	blog := &fakeNaverBlog{items: []naversearch.Item{
		{Title: "성수 카페 투어", Description: "분위기 좋은 성수 카페"},
		{Title: "을지로 노포", Description: ""},
		{Title: "", Description: "설명만 있는 글"},
	}}
	got, err := qualityBlogSearch{client: blog}.SearchBlog(context.Background(), "카페 추천", 101)
	if err != nil {
		t.Fatal(err)
	}
	want := []quality.SearchItem{
		{Title: "성수 카페 투어", Description: "분위기 좋은 성수 카페"},
		{Title: "을지로 노포", Description: ""},
		{Title: "", Description: "설명만 있는 글"},
	}
	if !reflect.DeepEqual(got, want) || blog.query != "카페 추천" || blog.start != 101 {
		t.Fatalf("got %+v for (%q, %d), want %+v for (카페 추천, 101)", got, blog.query, blog.start, want)
	}

	boom := &naversearch.StatusError{Status: 429, Code: "012"}
	if _, err := (qualityBlogSearch{client: &fakeNaverBlog{err: boom}}).SearchBlog(context.Background(), "q", 1); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the client's error", err)
	}
}
