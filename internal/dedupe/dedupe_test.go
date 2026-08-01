package dedupe

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"https://www.animenewsnetwork.com/news/x/?utm=1#top": "animenewsnetwork.com/news/x",
		"http://animenewsnetwork.com/news/x":                 "animenewsnetwork.com/news/x",
		"https://Natalie.mu/comic/news/123/":                 "natalie.mu/comic/news/123",
	}
	for in, want := range cases {
		if got := NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeDate(t *testing.T) {
	for _, in := range []string{"2026-07-28", "July 28, 2026", "28 Jul 2026", "2026/07/28", "reported on 2026-07-28"} {
		if got := NormalizeDate(in); got != "2026-07-28" {
			t.Errorf("NormalizeDate(%q) = %q, want 2026-07-28", in, got)
		}
	}
	if got := NormalizeDate("sometime last year"); got != "" {
		t.Errorf("NormalizeDate(unparseable) = %q, want empty", got)
	}
}

// The same article, re-researched with paraphrased title + cosmetic URL differences,
// must produce the SAME id (no duplicate row).
func TestNewsID_SameArticleDedupes(t *testing.T) {
	anime := "c7e45e8d-0e13-47c9-81da-e0d6bc5f4ef7"
	a := NewsID(anime, NormalizeURL("https://www.animenewsnetwork.com/news/x?utm=a"),
		NormalizeDate("2026-07-28"), TitleSlug("Season 3 confirmed"))
	b := NewsID(anime, NormalizeURL("http://animenewsnetwork.com/news/x/"),
		NormalizeDate("July 28, 2026"), TitleSlug("Season 3 officially confirmed for 2027"))
	if a != b {
		t.Errorf("same article produced different ids: %s vs %s", a, b)
	}
	if len(a) != 40 {
		t.Errorf("id should be 40 hex chars, got %d", len(a))
	}
}

// Different articles for the same anime must NOT collide.
func TestNewsID_DifferentArticles(t *testing.T) {
	anime := "abc"
	a := NewsID(anime, NormalizeURL("https://ann.com/a"), "2026-07-28", TitleSlug("x"))
	b := NewsID(anime, NormalizeURL("https://ann.com/b"), "2026-07-28", TitleSlug("x"))
	if a == b {
		t.Error("different articles collided into one id")
	}
}

// No URL → fallback key is anime + date + slug (stable, and dedupes same event).
func TestNewsID_FallbackNoURL(t *testing.T) {
	anime := "abc"
	a := NewsID(anime, "", "2026-07-28", TitleSlug("Big Delay Announced"))
	b := NewsID(anime, "", "2026-07-28", TitleSlug("big delay announced!!"))
	if a != b {
		t.Errorf("same undated event produced different ids: %s vs %s", a, b)
	}
}

// ---- article identity in the query string -------------------------------------
// Regression: NormalizeURL used to strip the whole query, so every article on a CMS
// that identifies posts by ?id= collapsed to one key and silently overwrote the last.

func TestNormalizeURLKeepsIdentifyingQuery(t *testing.T) {
	a := NormalizeURL("https://www.animenewsnetwork.com/encyclopedia/anime.php?id=38401")
	b := NormalizeURL("https://www.animenewsnetwork.com/encyclopedia/anime.php?id=99999")
	if a == b {
		t.Fatalf("two different articles collapsed to one key: %q", a)
	}
	if want := "animenewsnetwork.com/encyclopedia/anime.php?id=38401"; a != want {
		t.Fatalf("got %q want %q", a, want)
	}
}

func TestNormalizeURLDropsTrackingQuery(t *testing.T) {
	plain := NormalizeURL("https://natalie.mu/comic/news/12345")
	tracked := NormalizeURL("https://natalie.mu/comic/news/12345?utm_source=twitter&fbclid=abc&ref=nav")
	if plain != tracked {
		t.Fatalf("tracking params changed the key: %q vs %q", plain, tracked)
	}
}

func TestNormalizeURLQueryOrderStable(t *testing.T) {
	a := NormalizeURL("https://example.jp/n.php?id=7&p=2")
	b := NormalizeURL("https://example.jp/n.php?p=2&id=7")
	if a != b {
		t.Fatalf("param order produced different keys: %q vs %q", a, b)
	}
}

func TestNormalizeURLMixedParams(t *testing.T) {
	got := NormalizeURL("https://example.jp/n.php?utm_medium=x&id=7&fbclid=y")
	if want := "example.jp/n.php?id=7"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// ---- bare site roots -----------------------------------------------------------
// Official anime sites publish announcements as homepage sections, so every story
// scraped from one carries the same url. Keyed on that url they would overwrite
// each other; date+slug keeps them apart.

func TestNewsIDBareSiteFallsBackToDateSlug(t *testing.T) {
	norm := NormalizeURL("https://yanineko-anime.com/")
	if norm != "yanineko-anime.com" {
		t.Fatalf("unexpected normalization: %q", norm)
	}
	first := NewsID("anime-1", norm, "2026-07-30", "ending-theme-artist-thanks")
	second := NewsID("anime-1", norm, "2026-07-21", "insert-song-video-released")
	if first == second {
		t.Fatal("two stories from the same official site collapsed onto one id")
	}
}

func TestNewsIDBareSiteStableAcrossReruns(t *testing.T) {
	norm := NormalizeURL("https://chainsmoker-cat.com")
	a := NewsID("anime-1", norm, "2026-07-30", "same-slug")
	b := NewsID("anime-1", norm, "2026-07-30", "same-slug")
	if a != b {
		t.Fatal("same story produced different ids across runs")
	}
}

func TestNewsIDDeepLinkStillKeysOnURL(t *testing.T) {
	norm := NormalizeURL("https://animeanime.jp/article/2026/07/23/101349.html")
	// A reworded title must NOT create a duplicate when a real article url exists.
	a := NewsID("anime-1", norm, "2026-07-23", "episode-5-surprises-viewers")
	b := NewsID("anime-1", norm, "2026-07-23", "ep-5-surprises-fans-with-tone")
	if a != b {
		t.Fatal("paraphrased title created a duplicate despite a stable article url")
	}
}
