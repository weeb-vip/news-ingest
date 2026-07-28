// Package dedupe normalizes news fields and derives stable ids so the same real-world
// item never lands twice, even across re-runs where the AI paraphrases wording.
package dedupe

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// NormalizeURL lowercases the host, drops www, and strips query/fragment/trailing
// slash so cosmetic variations of the same article collapse to one key.
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
	return host + path
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
// one item). Fallback when there's no URL: anime + date + title slug.
func NewsID(animeID, normURL, isoDate, slug string) string {
	if normURL != "" {
		return sha1hex(animeID + "|" + normURL)
	}
	return sha1hex(animeID + "|" + isoDate + "|" + slug)
}

// FanartID keys on the image URL.
func FanartID(animeID, imageURL string) string {
	return sha1hex(animeID + "|" + strings.TrimSpace(imageURL))
}
