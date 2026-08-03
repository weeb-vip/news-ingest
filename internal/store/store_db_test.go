package store_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/db"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// These run against a real MySQL. The reflection tests in store_test.go can prove the
// upsert NAMES every column; only a database can prove it actually writes them — which is
// the half that broke in production, where a column existed on the struct, was written on
// insert, and then never updated again on any row that already existed.
//
// Skipped when TEST_DB_HOST is unset so `go test ./...` still works on a laptop with no
// database. CI sets it (see .github/workflows/build.yaml).
func testConfig(t *testing.T) config.DBConfig {
	t.Helper()
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST not set; skipping database tests")
	}
	port, _ := strconv.Atoi(orDefault(os.Getenv("TEST_DB_PORT"), "3306"))
	return config.DBConfig{
		Host:               host,
		Port:               uint(port),
		DataBase:           orDefault(os.Getenv("TEST_DB_NAME"), "weeb_test"),
		User:               orDefault(os.Getenv("TEST_DB_USER"), "root"),
		Password:           os.Getenv("TEST_DB_PASSWORD"),
		SSLMode:            "false",
		MigrationTableName: "__migrations_news-ingest",
	}
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// shared is opened and migrated once; each test runs inside its own transaction.
var shared *store.Store

// newStore hands back a Store bound to a transaction that is rolled back when the test
// finishes. Nothing a test writes survives it, so tests cannot leak into each other and
// nothing needs deleting — which also means pointing this at a populated database cannot
// destroy anything.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := testConfig(t) // skips the test when no database is configured

	if shared == nil {
		// Migrating here exercises the migrations on every CI run, not only at deploy.
		if err := db.MigrateUp(cfg); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		st, err := store.Open(cfg)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		shared = st
	}

	tx, rollback := shared.Begin()
	t.Cleanup(rollback)
	return tx
}

func date(t *testing.T, s string) *time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}

func ptr[T any](v T) *T { return &v }

func TestUpsertActuallyUpdatesEveryMutableColumn(t *testing.T) {
	// The failure this reproduces: a column written on INSERT but missing from DoUpdates
	// looks perfect on new rows and stays frozen forever on existing ones. Only a second
	// write against a row that already exists can catch it.
	st := newStore(t)
	ctx := context.Background()

	first := &store.AnimeNews{
		ID: "n1", AnimeID: "a1", Title: "original", Category: "other",
		Summary: ptr("first summary"), SourceName: ptr("ANN"), Language: ptr("en"),
		PublishedDate: date(t, "2026-07-01"), MalID: ptr(1), EpisodeNumber: ptr(1),
		TitleSlug: ptr("original"), References: ptr(`[{"kind":"site","title":"a","url":"https://a"}]`),
	}
	if err := st.UpsertNews(first); err != nil {
		t.Fatalf("insert: %v", err)
	}

	updated := &store.AnimeNews{
		ID: "n1", AnimeID: "a2", Title: "revised", Category: "renewal",
		Summary: ptr("second summary"), SourceName: ptr("Natalie"), Language: ptr("ja"),
		PublishedDate: date(t, "2026-08-02"), MalID: ptr(2), EpisodeNumber: ptr(9),
		TitleSlug: ptr("revised"), References: ptr(`[{"kind":"video","title":"b","url":"https://b"}]`),
	}
	if err := st.UpsertNews(updated); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := st.NewsByID(ctx, "n1")
	if err != nil || got == nil {
		t.Fatalf("read back: %v (row=%v)", err, got)
	}
	for _, c := range []struct {
		field string
		got   any
		want  any
	}{
		{"anime_id", got.AnimeID, "a2"},
		{"title", got.Title, "revised"},
		{"category", got.Category, "renewal"},
		{"summary", *got.Summary, "second summary"},
		{"source_name", *got.SourceName, "Natalie"},
		{"language", *got.Language, "ja"},
		{"mal_id", *got.MalID, 2},
		{"episode_number", *got.EpisodeNumber, 9},
		{"title_slug", *got.TitleSlug, "revised"},
		{"published_date", got.PublishedDate.Format("2006-01-02"), "2026-08-02"},
	} {
		if c.got != c.want {
			t.Errorf("%s was not updated: got %v, want %v", c.field, c.got, c.want)
		}
	}
	if got.References == nil || *got.References == `[{"kind":"site","title":"a","url":"https://a"}]` {
		t.Error("reference_links was not updated — it is in the struct but stale in the row")
	}
}

func TestUpsertIsIdempotent(t *testing.T) {
	// Re-publishing a season must update rows, not duplicate them. The whole retry story
	// downstream depends on this.
	st := newStore(t)
	n := &store.AnimeNews{ID: "n1", AnimeID: "a1", Title: "t", Category: "other"}
	for i := 0; i < 3; i++ {
		if err := st.UpsertNews(n); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	_, total, err := st.LatestNews(context.Background(), 10, 0, store.NewsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected 1 row after 3 upserts, got %d", total)
	}
}

func TestFanartIsInsertOnly(t *testing.T) {
	// Image URLs are immutable, so the conflict clause is DoNothing. A second write must
	// not error — the pipeline re-publishes the same fanart on every run.
	st := newStore(t)
	f := &store.Fanart{ID: "f1", AnimeID: "a1", ImageURL: "https://img/1.jpg"}
	if err := st.UpsertFanart(f); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.UpsertFanart(&store.Fanart{ID: "f1", AnimeID: "a1", ImageURL: "https://img/CHANGED.jpg"}); err != nil {
		t.Fatalf("re-insert should be a no-op, not an error: %v", err)
	}
}

func seedFeed(t *testing.T, st *store.Store) {
	t.Helper()
	rows := []*store.AnimeNews{
		{ID: "n1", AnimeID: "a1", Title: "oldest", Category: "renewal",
			Language: ptr("en"), PublishedDate: date(t, "2026-07-15")},
		{ID: "n2", AnimeID: "a1", Title: "newest", Category: "delay",
			Language: ptr("ja"), PublishedDate: date(t, "2026-07-20")},
		{ID: "n3", AnimeID: "a2", Title: "middle", Category: "announcement",
			Language: ptr("en"), PublishedDate: date(t, "2026-07-18")},
		{ID: "n4", AnimeID: "a2", Title: "undated", Category: "other", Language: ptr("en")},
	}
	for _, r := range rows {
		if err := st.UpsertNews(r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLatestNewsOrdersNewestFirstWithUndatedLast(t *testing.T) {
	// An item we could not date is not news from the beginning of time. Sorted naively,
	// NULL leads the feed — the most visible position on the page.
	st := newStore(t)
	seedFeed(t, st)
	rows, total, err := st.LatestNews(context.Background(), 10, 0, store.NewsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	var order []string
	for _, r := range rows {
		order = append(order, r.ID)
	}
	want := []string{"n2", "n3", "n1", "n4"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestLatestNewsFilters(t *testing.T) {
	st := newStore(t)
	seedFeed(t, st)
	ctx := context.Background()

	rows, total, err := st.LatestNews(ctx, 10, 0, store.NewsFilter{Language: ptr("ja")})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != "n2" {
		t.Fatalf("language filter: total=%d rows=%d", total, len(rows))
	}

	rows, total, err = st.LatestNews(ctx, 10, 0, store.NewsFilter{Category: ptr("renewal")})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].ID != "n1" {
		t.Fatalf("category filter: total=%d", total)
	}

	// An empty string must mean "no filter", not "match empty" — otherwise the feed
	// silently returns nothing whenever the caller passes a blank argument.
	_, total, err = st.LatestNews(ctx, 10, 0, store.NewsFilter{Category: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("empty filter should not narrow: total=%d, want 4", total)
	}
}

func TestPaginationDoesNotRepeatOrSkip(t *testing.T) {
	// published_date is a DATE, so items share a day routinely. Without the id tiebreaker
	// the database may order them differently between requests, and a reader paging through
	// sees one item twice and misses another. Same-day rows make that visible.
	st := newStore(t)
	for i := 0; i < 10; i++ {
		id := "same" + strconv.Itoa(i)
		if err := st.UpsertNews(&store.AnimeNews{
			ID: id, AnimeID: "a1", Title: id, Category: "other",
			PublishedDate: date(t, "2026-07-15"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	seen := map[string]bool{}
	for offset := 0; offset < 10; offset += 3 {
		rows, _, err := st.LatestNews(ctx, 3, offset, store.NewsFilter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if seen[r.ID] {
				t.Fatalf("item %s appeared on two pages", r.ID)
			}
			seen[r.ID] = true
		}
	}
	if len(seen) != 10 {
		t.Fatalf("paged through %d of 10 items; the rest were skipped", len(seen))
	}
}

func TestNewsForAnimeIsScopedAndOrdered(t *testing.T) {
	st := newStore(t)
	seedFeed(t, st)
	rows, err := st.NewsForAnime(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "n2" || rows[1].ID != "n1" {
		t.Fatalf("expected a1's two items newest-first, got %+v", rows)
	}
}

func TestNewsByIDReturnsNilWhenMissing(t *testing.T) {
	// The router asks about references that may have been deleted since. A missing row is
	// a null field, not a query error that fails the whole request.
	st := newStore(t)
	got, err := st.NewsByID(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("a missing row must not be an error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	// Deployments re-run this on every rollout. Outside a transaction deliberately: DDL is
	// not transactional in MySQL, so wrapping it would prove nothing.
	cfg := testConfig(t)
	for i := 0; i < 2; i++ {
		if err := db.MigrateUp(cfg); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}
}
