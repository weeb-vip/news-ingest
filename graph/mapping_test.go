package graph

import (
	"testing"
	"time"

	"github.com/weeb-vip/news-ingest/internal/store"
)

func ptr[T any](v T) *T { return &v }

func TestPublishedDateRendersAsTheContractSaysItShould(t *testing.T) {
	// anime-api served this as a YYYY-MM-DD string and the frontend parses it that way.
	// Emitting a different shape here would be a breaking change disguised as a migration —
	// and one the router would not catch, because the type still says String.
	when := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	got := toGraph(store.AnimeNews{ID: "n1", PublishedDate: &when})
	if got.PublishedDate == nil || *got.PublishedDate != "2026-07-15" {
		t.Fatalf("published date = %v, want 2026-07-15", got.PublishedDate)
	}
}

func TestMissingPublishedDateStaysNull(t *testing.T) {
	// Not the zero time. "0001-01-01" would sort and display as a real date.
	if got := toGraph(store.AnimeNews{ID: "n1"}); got.PublishedDate != nil {
		t.Fatalf("expected nil, got %q", *got.PublishedDate)
	}
}

func TestReferencesDecodeFromTheJSONColumn(t *testing.T) {
	raw := `[{"kind":"video","title":"Main trailer","url":"https://youtube.com/watch?v=x"}]`
	got := toGraph(store.AnimeNews{ID: "n1", References: &raw})
	if len(got.References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(got.References))
	}
	r := got.References[0]
	if r.Kind != "video" || r.Title != "Main trailer" || r.URL != "https://youtube.com/watch?v=x" {
		t.Fatalf("reference did not round-trip: %+v", r)
	}
}

func TestMalformedReferencesDropRatherThanFailTheQuery(t *testing.T) {
	// One bad sub-document must not take out a whole feed. This is a site-wide page: a
	// single unparseable row would otherwise blank the front page for everyone.
	bad := `{not json at all`
	got := toGraph(store.AnimeNews{ID: "n1", Title: "still useful", References: &bad})
	if got.References != nil {
		t.Fatal("malformed references should be dropped")
	}
	if got.Title != "still useful" {
		t.Fatal("the item itself must survive its bad attachments")
	}
}

func TestReferencesWithoutAUrlAreSkipped(t *testing.T) {
	// A reference is a link. Without one there is nothing to click, and rendering it
	// produces a dead chip in the UI.
	raw := `[{"kind":"site","title":"no link"},{"kind":"video","title":"ok","url":"https://y.t/1"}]`
	got := toGraph(store.AnimeNews{ID: "n1", References: &raw})
	if len(got.References) != 1 || got.References[0].URL != "https://y.t/1" {
		t.Fatalf("expected only the linked reference, got %+v", got.References)
	}
}

func TestEmptyAndNullReferenceColumnsBothYieldNothing(t *testing.T) {
	for name, raw := range map[string]*string{"null": nil, "empty": ptr("")} {
		if got := toGraph(store.AnimeNews{ID: "n1", References: raw}); got.References != nil {
			t.Errorf("%s column should yield no references, got %+v", name, got.References)
		}
	}
}

func TestNullableFieldsPassThroughUntouched(t *testing.T) {
	// These are nullable end to end — column, model and schema. Substituting "" anywhere
	// would turn "we don't know" into "it is blank", which the UI renders differently.
	got := toGraph(store.AnimeNews{ID: "n1", AnimeID: "a1", Title: "t", Category: "other"})
	if got.Summary != nil || got.SourceURL != nil || got.SourceName != nil ||
		got.Language != nil || got.EpisodeNumber != nil || got.MalID != nil {
		t.Fatalf("absent fields should stay nil: %+v", got)
	}
}

func TestDerefAppliesTheSchemaDefault(t *testing.T) {
	// gqlgen hands a nil pointer when an optional argument is omitted, not the declared
	// default — so latestNews() with no arguments would page zero items without this.
	if got := deref(nil, 20); got != 20 {
		t.Fatalf("omitted argument should fall back to the default, got %d", got)
	}
	if got := deref(ptr(5), 20); got != 5 {
		t.Fatalf("provided argument should win, got %d", got)
	}
	if got := deref(ptr(0), 20); got != 0 {
		t.Fatalf("an explicit 0 is a real value, not an omission; got %d", got)
	}
}
