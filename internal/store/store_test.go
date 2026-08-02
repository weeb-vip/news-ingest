package store

import (
	"reflect"
	"strings"
	"testing"
)

// gormColumns reads the column names off a struct's gorm tags, so the expectation is
// derived from the schema rather than restated by hand — a hand-written list would need
// updating in two places and would drift exactly when it matters.
func gormColumns(v any) []string {
	t := reflect.TypeOf(v)
	var out []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("gorm")
		for _, part := range strings.Split(tag, ";") {
			if name, ok := strings.CutPrefix(part, "column:"); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

func TestUpsertCoversEveryMutableColumn(t *testing.T) {
	// The failure this guards against is quiet and slow: a column present on the struct but
	// absent from DoUpdates is written on INSERT and never again, so new rows look perfect
	// while existing rows keep stale values forever. `language` and `reference_links` were
	// added to this table recently, which is exactly when the list gets forgotten.
	updatable := map[string]bool{}
	for _, c := range newsUpdatableColumns {
		updatable[c] = true
	}

	for _, col := range gormColumns(AnimeNews{}) {
		if col == "id" {
			if updatable[col] {
				t.Error("the primary key must not be in DoUpdates")
			}
			continue
		}
		if !updatable[col] {
			t.Errorf("column %q is on AnimeNews but missing from newsUpdatableColumns — "+
				"it will be written on insert and never updated", col)
		}
	}
}

func TestUpsertDoesNotNameColumnsThatDoNotExist(t *testing.T) {
	// The mirror image: a stale entry here (a renamed or dropped column) makes every upsert
	// fail at the database with "Unknown column", which is how a whole day of messages was
	// lost once already.
	actual := map[string]bool{}
	for _, c := range gormColumns(AnimeNews{}) {
		actual[c] = true
	}
	for _, c := range newsUpdatableColumns {
		if !actual[c] {
			t.Errorf("newsUpdatableColumns names %q, which is not a column on AnimeNews", c)
		}
	}
}

func TestTableNamesMatchTheAnimeApiSchema(t *testing.T) {
	// These tables are owned and migrated by anime-api; we only write to them. A typo here
	// is not caught until runtime, against production.
	if got := (AnimeNews{}).TableName(); got != "anime_news" {
		t.Errorf("news table = %q, want anime_news", got)
	}
	if got := (Fanart{}).TableName(); got != "anime_fanart" {
		t.Errorf("fanart table = %q, want anime_fanart", got)
	}
}

func TestNewsCarriesTheColumnsTheApiExposes(t *testing.T) {
	// Guards the two fields added for the news UI. Losing either silently degrades the site
	// (no language badge, no reference links) without any error.
	cols := map[string]bool{}
	for _, c := range gormColumns(AnimeNews{}) {
		cols[c] = true
	}
	for _, needed := range []string{"language", "reference_links", "published_date", "title_slug"} {
		if !cols[needed] {
			t.Errorf("AnimeNews is missing the %q column", needed)
		}
	}
}
