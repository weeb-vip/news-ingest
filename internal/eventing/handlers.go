package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/model"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// ConsumeNews runs the news topic until ctx is cancelled.
func ConsumeNews(ctx context.Context, cfg config.KafkaConfig, st *store.Store) error {
	slog.Info("consuming", "topic", cfg.NewsTopic, "retry_topic", cfg.NewsTopic+"-retry")
	return run(ctx, cfg, cfg.NewsTopic,
		func(_ context.Context, env model.Envelope[model.NewsMessage]) error {
			return upsertNews(st, env.Data)
		})
}

// ConsumeFanart runs the fanart topic until ctx is cancelled.
func ConsumeFanart(ctx context.Context, cfg config.KafkaConfig, st *store.Store) error {
	slog.Info("consuming", "topic", cfg.FanartTopic, "retry_topic", cfg.FanartTopic+"-retry")
	return run(ctx, cfg, cfg.FanartTopic,
		func(_ context.Context, env model.Envelope[model.FanartMessage]) error {
			m := env.Data
			return st.UpsertFanart(&store.Fanart{
				ID: m.ID, AnimeID: m.AnimeID, ImageURL: m.ImageURL, SourceURL: m.SourceURL,
			})
		})
}

func upsertNews(st *store.Store, m model.NewsMessage) error {
	return st.UpsertNews(newsRow(m))
}

// newsRow maps a message onto a row. Split from the write so the mapping — which is where
// the fiddly parts live (two date formats, four nullable fields, a JSON sub-document) — can
// be tested without a database.
func newsRow(m model.NewsMessage) *store.AnimeNews {
	n := &store.AnimeNews{
		ID: m.ID, AnimeID: m.AnimeID, MalID: m.MalID, Title: m.Title, Summary: m.Summary,
		Category: m.Category, SourceURL: m.SourceURL, SourceName: m.SourceName,
		EpisodeNumber: m.EpisodeNumber, Language: m.Language,
	}
	if m.TitleSlug != "" {
		n.TitleSlug = &m.TitleSlug
	}
	if m.PublishedDate != nil {
		if t, err := time.Parse("2006-01-02", *m.PublishedDate); err == nil {
			n.PublishedDate = &t
		}
	}
	if m.ResearchedAt != nil {
		if t, err := time.Parse(time.RFC3339, *m.ResearchedAt); err == nil {
			n.ResearchedAt = &t
		}
	}
	if len(m.References) > 0 {
		// Stored as a JSON document. A marshal failure must not poison the whole item —
		// the news itself is worth more than its attachments, so drop them and continue.
		// This is deliberately NOT an error return: with retry in place, returning one
		// would send an item to the retry topic over a cosmetic field, and it would fail
		// there identically three more times.
		if b, err := json.Marshal(m.References); err == nil {
			s := string(b)
			n.References = &s
		} else {
			slog.Warn("dropping references; marshal failed", "id", m.ID, "err", err)
		}
	}
	return n
}
