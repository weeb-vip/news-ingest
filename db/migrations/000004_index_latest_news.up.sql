-- Index for the site-wide news feed: newest first, across all anime.
--
-- id is the tiebreaker so pagination is stable — published_date is only a DATE, so a day
-- with several items would otherwise order arbitrarily and a page boundary could repeat or
-- skip rows between requests.
ALTER TABLE anime_news ADD INDEX idx_news_latest (published_date DESC, id);
