package store

import (
	"context"

	"gorm.io/gorm"
)

// NewsFilter narrows the site-wide feed. A nil field means "no restriction" rather than
// "match empty", which is why these are pointers.
type NewsFilter struct {
	Category *string
	Language *string
}

func (f NewsFilter) apply(q *gorm.DB) *gorm.DB {
	if f.Category != nil && *f.Category != "" {
		q = q.Where("category = ?", *f.Category)
	}
	if f.Language != nil && *f.Language != "" {
		q = q.Where("language = ?", *f.Language)
	}
	return q
}

// LatestNews returns a page of news across every anime, newest first, plus the total
// matching the filter.
//
// Ordered by (published_date DESC, id) to match idx_news_latest. The id tiebreaker is not
// cosmetic: published_date is a DATE, so several items share one day routinely, and without
// a stable second key the database may order them differently between requests — a reader
// paging through would see an item twice or miss one entirely.
//
// NULL published_date sorts last rather than first: an item we could not date is not news
// from the beginning of time, and putting it at the head of a "latest" feed would be wrong
// in the most visible position on the page.
func (s *Store) LatestNews(ctx context.Context, limit, offset int, f NewsFilter) ([]AnimeNews, int64, error) {
	limit = clamp(limit, 1, 100)
	if offset < 0 {
		offset = 0
	}

	var total int64
	if err := f.apply(s.db.WithContext(ctx).Model(&AnimeNews{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AnimeNews
	err := f.apply(s.db.WithContext(ctx).Model(&AnimeNews{})).
		Order("published_date IS NULL, published_date DESC, id").
		Limit(limit).Offset(offset).
		Find(&rows).Error
	return rows, total, err
}

// NewsForAnime returns one anime's news, newest first. This is the federated Anime.news
// field, so it is called once per anime in a query — see NewsForAnimeIDs for the batched
// form the resolver actually uses.
func (s *Store) NewsForAnime(ctx context.Context, animeID string) ([]AnimeNews, error) {
	var rows []AnimeNews
	err := s.db.WithContext(ctx).
		Where("anime_id = ?", animeID).
		Order("published_date IS NULL, published_date DESC, id").
		Find(&rows).Error
	return rows, err
}

// NewsForAnimeIDs fetches news for several anime in one query, grouped by anime id.
//
// The router resolves Anime.news once per anime in the result set, so a page listing 20
// shows would otherwise issue 20 queries. This exists so the resolver can batch them.
func (s *Store) NewsForAnimeIDs(ctx context.Context, ids []string) (map[string][]AnimeNews, error) {
	out := make(map[string][]AnimeNews, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []AnimeNews
	err := s.db.WithContext(ctx).
		Where("anime_id IN ?", ids).
		Order("published_date IS NULL, published_date DESC, id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AnimeID] = append(out[r.AnimeID], r)
	}
	return out, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// NewsByID resolves a single item, for the federated entity lookup. A missing row is not an
// error — the router asks about references that may have been deleted since.
func (s *Store) NewsByID(ctx context.Context, id string) (*AnimeNews, error) {
	var row AnimeNews
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
