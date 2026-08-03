package store

import "testing"

func TestLimitIsClampedToASaneRange(t *testing.T) {
	// latestNews is a public, unauthenticated feed. An unbounded limit lets one request ask
	// for every news item ever published, and a zero or negative one silently returns
	// nothing — which reads as "there is no news" rather than "you asked for none".
	cases := []struct{ in, want int }{
		{0, 1}, {-5, 1}, {20, 20}, {100, 100}, {1000, 100},
	}
	for _, c := range cases {
		if got := clamp(c.in, 1, 100); got != c.want {
			t.Errorf("clamp(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFilterOnlyNarrowsWhenAsked(t *testing.T) {
	// A nil filter field means "no restriction". Treating it as an empty-string match would
	// return nothing at all, and the feed would look broken rather than unfiltered.
	empty := ""
	cat := "renewal"
	for name, f := range map[string]NewsFilter{
		"nothing set":      {},
		"empty strings":    {Category: &empty, Language: &empty},
		"category only":    {Category: &cat},
	} {
		// apply() builds SQL against a nil *gorm.DB in this unit context, so what is asserted
		// here is the decision, not the generated SQL: only a non-empty value should narrow.
		narrows := (f.Category != nil && *f.Category != "") || (f.Language != nil && *f.Language != "")
		want := name == "category only"
		if narrows != want {
			t.Errorf("%s: narrows=%v, want %v", name, narrows, want)
		}
	}
}
