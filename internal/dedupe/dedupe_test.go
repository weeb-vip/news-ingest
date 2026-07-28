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
