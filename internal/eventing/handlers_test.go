package eventing

import (
	"encoding/json"
	"testing"

	"github.com/weeb-vip/news-ingest/internal/model"
)

func ptr[T any](v T) *T { return &v }

func TestNewsRowMapsRequiredFields(t *testing.T) {
	got := newsRow(model.NewsMessage{
		ID: "n1", AnimeID: "a1", Title: "Season 2 confirmed", Summary: ptr("It was confirmed."),
		Category: "renewal", SourceURL: ptr("https://ann.com/x"), SourceName: ptr("ANN"),
	})
	if got.ID != "n1" || got.AnimeID != "a1" || *got.SourceURL != "https://ann.com/x" {
		t.Fatalf("identity fields not carried through: %+v", got)
	}
	if got.Category != "renewal" || *got.SourceName != "ANN" {
		t.Fatalf("descriptive fields not carried through: %+v", got)
	}
}

func TestPublishedDateParsesTheDateOnlyFormat(t *testing.T) {
	// The producer sends published_date as YYYY-MM-DD and researched_at as RFC3339. Using
	// one parser for both silently drops whichever does not match, and a news feed sorted
	// newest-first with null dates is indistinguishable from having no news.
	got := newsRow(model.NewsMessage{ID: "n1", PublishedDate: ptr("2026-07-15")})
	if got.PublishedDate == nil {
		t.Fatal("published date was dropped")
	}
	if y, m, d := got.PublishedDate.Date(); y != 2026 || int(m) != 7 || d != 15 {
		t.Fatalf("parsed to the wrong date: %v", got.PublishedDate)
	}
}

func TestResearchedAtParsesRFC3339(t *testing.T) {
	got := newsRow(model.NewsMessage{ID: "n1", ResearchedAt: ptr("2026-08-02T11:30:00Z")})
	if got.ResearchedAt == nil {
		t.Fatal("researched_at was dropped")
	}
	if got.ResearchedAt.Hour() != 11 {
		t.Fatalf("parsed to the wrong time: %v", got.ResearchedAt)
	}
}

func TestUnparseableDatesAreDroppedNotFatal(t *testing.T) {
	// A malformed date must not fail the message: with retry in place, returning an error
	// would send the item round the retry topic three times and then discard it entirely,
	// losing real news over one bad field.
	got := newsRow(model.NewsMessage{
		ID: "n1", Title: "Still useful", PublishedDate: ptr("not-a-date"),
		ResearchedAt: ptr("also-not-a-date"),
	})
	if got.PublishedDate != nil || got.ResearchedAt != nil {
		t.Fatal("a malformed date should be dropped, not stored")
	}
	if got.Title != "Still useful" {
		t.Fatal("the rest of the item should survive a bad date")
	}
}

func TestEmptyTitleSlugStaysNull(t *testing.T) {
	// The column is nullable and unique-ish; writing "" for every item without a slug would
	// collide rather than simply be absent.
	if got := newsRow(model.NewsMessage{ID: "n1"}); got.TitleSlug != nil {
		t.Fatalf("expected nil slug, got %q", *got.TitleSlug)
	}
	got := newsRow(model.NewsMessage{ID: "n1", TitleSlug: "season-2-confirmed"})
	if got.TitleSlug == nil || *got.TitleSlug != "season-2-confirmed" {
		t.Fatal("a present slug should be stored")
	}
}

func TestReferencesAreStoredAsJSON(t *testing.T) {
	got := newsRow(model.NewsMessage{
		ID: "n1",
		References: []model.Reference{
			{Kind: "video", Title: "Main trailer", URL: "https://youtube.com/watch?v=x"},
		},
	})
	if got.References == nil {
		t.Fatal("references were dropped")
	}
	var back []map[string]any
	if err := json.Unmarshal([]byte(*got.References), &back); err != nil {
		t.Fatalf("stored references are not valid JSON: %v", err)
	}
	if len(back) != 1 || back[0]["url"] != "https://youtube.com/watch?v=x" {
		t.Fatalf("references did not round-trip: %s", *got.References)
	}
}

func TestNoReferencesLeavesTheColumnNull(t *testing.T) {
	// "[]" and NULL are different to the API: one says "we looked and found none", the
	// other "this predates the feature". Writing "[]" for every old item would be a lie.
	if got := newsRow(model.NewsMessage{ID: "n1"}); got.References != nil {
		t.Fatalf("expected nil, got %q", *got.References)
	}
}

func TestLanguageIsCarriedThrough(t *testing.T) {
	// This is the column whose absence in production silently destroyed a day of messages,
	// so it is worth an explicit test rather than trusting the struct literal.
	got := newsRow(model.NewsMessage{ID: "n1", Language: ptr("ja")})
	if got.Language == nil || *got.Language != "ja" {
		t.Fatalf("language not carried through: %+v", got.Language)
	}
}
