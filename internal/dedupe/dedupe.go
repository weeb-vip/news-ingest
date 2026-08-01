// Package dedupe normalizes news fields and derives stable ids so the same real-world
// item never lands twice, even across re-runs where the AI paraphrases wording.
package dedupe

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// identifyingParams are query parameters that carry the article's IDENTITY rather than
// tracking or presentation. Plenty of CMSs put the article id in the query string —
// animenewsnetwork.com/encyclopedia/anime.php?id=38401 or
// animatetimes.com/news/details.php?id=1769944248 — where the path is shared by every
// article on the site. Dropping the whole query collapses them onto one key, so the
// second article silently overwrites the first.
var identifyingParams = map[string]bool{
	"id": true, "p": true, "sid": true, "aid": true, "nid": true,
	"article_id": true, "articleid": true, "story_id": true, "storyid": true,
	"news_id": true, "newsid": true, "post": true, "post_id": true,
	"entry_id": true, "entryid": true, "page_id": true, "pageid": true,
	"item_id": true, "itemid": true, "v": true,
}

// NormalizeURL lowercases the host, drops www, strips the fragment and trailing slash,
// and keeps only query parameters that identify the article — so cosmetic variations
// (utm_*, fbclid, ref) still collapse while genuinely different articles stay distinct.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	path := strings.TrimRight(u.Path, "/")

	// Sorted, so parameter order can never yield two keys for one article.
	q := u.Query()
	var keys []string
	for k := range q {
		if identifyingParams[strings.ToLower(k)] {
			keys = append(keys, strings.ToLower(k))
		}
	}
	if len(keys) == 0 {
		return host + path
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+q.Get(k))
	}
	return host + path + "?" + strings.Join(parts, "&")
}

// isBareSite reports whether a normalized url points at a site root rather than a
// specific article — e.g. "yanineko-anime.com" with no path and no identifying query.
func isBareSite(normURL string) bool {
	if normURL == "" {
		return false
	}
	return !strings.Contains(normURL, "/") && !strings.Contains(normURL, "?")
}

var dateLayouts = []string{
	"2006-01-02", time.RFC3339, "2006/01/02", "January 2, 2006", "Jan 2, 2006",
	"2 January 2006", "02 Jan 2006", "01/02/2006", "2006.01.02", "Jan 2 2006",
}

// NormalizeDate parses a variety of reported-date formats into YYYY-MM-DD.
// Returns "" if it can't be parsed (those items are dropped upstream anyway).
func NormalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	// last resort: a leading ISO date embedded in a longer string
	if m := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).FindString(raw); m != "" {
		return m
	}
	return ""
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// TitleSlug lowercases and collapses a title to a stable slug for the fallback key.
func TitleSlug(title string) string {
	s := nonAlnum.ReplaceAllString(strings.ToLower(strings.TrimSpace(title)), "-")
	s = strings.Trim(s, "-")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func sha1hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:]) // 40 chars
}

// NewsID is the dedupe key. Primary: anime + normalized article URL (one article =
// one item). Falls back to anime + date + title slug when there is no URL, or when the
// URL is a bare site root.
//
// The bare-root case matters: an official anime site publishes announcements as
// sections of its homepage rather than separate pages, so every story scraped from
// it carries the SAME url. Keyed on that url they would all collapse onto one row,
// each overwriting the last — and an official site is exactly the source that
// produces many stories over time. Date + slug keeps them distinct, at the cost of a
// reworded title creating a duplicate, which is much the better failure.
func NewsID(animeID, normURL, isoDate, slug string) string {
	if normURL != "" && !isBareSite(normURL) {
		return sha1hex(animeID + "|" + normURL)
	}
	return sha1hex(animeID + "|" + isoDate + "|" + slug)
}

// FanartID keys on the image URL.
func FanartID(animeID, imageURL string) string {
	return sha1hex(animeID + "|" + strings.TrimSpace(imageURL))
}
