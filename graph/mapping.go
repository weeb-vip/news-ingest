package graph

import (
	"encoding/json"
	"log/slog"

	"github.com/weeb-vip/news-ingest/graph/model"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// toGraph converts a row to the API type.
//
// Dates are rendered as YYYY-MM-DD strings rather than a scalar because that is the shape
// anime-api served and the frontend already parses. Changing it here would be a breaking
// change disguised as a migration.
func toGraph(r store.AnimeNews) *model.AnimeNews {
	out := &model.AnimeNews{
		ID:            r.ID,
		AnimeID:       r.AnimeID,
		Title:         r.Title,
		Summary:       r.Summary,
		Category:      r.Category,
		SourceURL:     r.SourceURL,
		SourceName:    r.SourceName,
		EpisodeNumber: r.EpisodeNumber,
		MalID:         r.MalID,
		Language:      r.Language,
	}
	if r.PublishedDate != nil {
		s := r.PublishedDate.Format("2006-01-02")
		out.PublishedDate = &s
	}
	out.References = decodeReferences(r.ID, r.References)
	return out
}

// decodeReferences unpacks the JSON column.
//
// Malformed JSON yields no references rather than an error: the news item itself is worth
// far more than its attachments, and failing the whole query — or the whole feed — because
// one row has a bad sub-document would turn a cosmetic problem into an outage.
func decodeReferences(id string, raw *string) []*model.NewsReference {
	if raw == nil || *raw == "" {
		return nil
	}
	var refs []struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal([]byte(*raw), &refs); err != nil {
		slog.Warn("dropping unreadable references", "id", id, "err", err)
		return nil
	}
	out := make([]*model.NewsReference, 0, len(refs))
	for _, r := range refs {
		if r.URL == "" {
			continue // a reference with no link is not a reference
		}
		out = append(out, &model.NewsReference{Kind: r.Kind, Title: r.Title, URL: r.URL})
	}
	return out
}

func toGraphAll(rows []store.AnimeNews) []*model.AnimeNews {
	out := make([]*model.AnimeNews, 0, len(rows))
	for _, r := range rows {
		out = append(out, toGraph(r))
	}
	return out
}
