package model

// ---- HTTP ingest request (from the research tool /publish) ----

type IngestRequest struct {
	AnimeID      string   `json:"anime_id"`
	MalID        *int     `json:"mal_id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	News         []NewsIn `json:"news"`
	Fanart       []string `json:"fanart"`
	ResearchedAt string   `json:"researched_at"`
}

type NewsIn struct {
	Title    string  `json:"title"`
	Summary  string  `json:"summary"`
	Category string  `json:"category"`
	Date     string  `json:"date"`
	URL      *string `json:"url"`
	Source   string  `json:"source"`
	Episode  *int    `json:"episode"`
	// ISO 639-1 code of the SOURCE article, not of the summary (which the research
	// tool always writes in English). Lets the site say "this link is Japanese".
	Language *string `json:"language"`
	// Media the article points at — a PV, an insert-song video, the official site.
	// URLs come from the page's real hrefs upstream, never authored by a model.
	References []Reference `json:"references"`
}

// Reference is one piece of media an article links to. Kind is a coarse bucket
// (video / post / site) the scraper derives from the link's host.
type Reference struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ---- Kafka messages (one per item, wrapped in a {"data": …} envelope) ----
// Field names match the anime-api MySQL columns so the consumer maps 1:1.

type Envelope[T any] struct {
	Data T `json:"data"`
}

type NewsMessage struct {
	ID            string  `json:"id"` // sha1(anime_id + normalized_url) — dedupe key + Kafka key
	AnimeID       string  `json:"anime_id"`
	MalID         *int    `json:"mal_id"`
	Title         string  `json:"title"`
	Summary       *string `json:"summary"`
	Category      string  `json:"category"`
	SourceURL     *string `json:"source_url"`
	SourceName    *string `json:"source_name"`
	PublishedDate *string `json:"published_date"` // YYYY-MM-DD
	EpisodeNumber *int    `json:"episode_number"`
	TitleSlug     string  `json:"title_slug"`
	ResearchedAt  *string `json:"researched_at"`
	Language      *string `json:"language"`
	// Carried as a slice on the wire; the consumer marshals it into the JSON column.
	References []Reference `json:"references,omitempty"`
}

type FanartMessage struct {
	ID        string  `json:"id"` // sha1(anime_id + image_url)
	AnimeID   string  `json:"anime_id"`
	ImageURL  string  `json:"image_url"`
	SourceURL *string `json:"source_url"`
}
