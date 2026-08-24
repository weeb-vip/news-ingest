package eventing

import (
	"context"
	"log/slog"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/model"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// ConsumeNewsNats runs the news subject until ctx is cancelled. Twin of ConsumeNews.
func ConsumeNewsNats(ctx context.Context, cfg config.NatsConfig, st *store.Store) error {
	slog.Info("consuming", "subject", cfg.NewsSubject, "retry_subject", cfg.NewsSubject+"-retry")

	return runNats(ctx, cfg, cfg.Offset, cfg.NewsSubject,
		func(_ context.Context, env model.Envelope[model.NewsMessage]) error {
			return upsertNews(st, env.Data)
		})
}

// ConsumeFanartNats runs the fanart subject until ctx is cancelled. Twin of ConsumeFanart.
func ConsumeFanartNats(ctx context.Context, cfg config.NatsConfig, st *store.Store) error {
	slog.Info("consuming", "subject", cfg.FanartSubject, "retry_subject", cfg.FanartSubject+"-retry")

	return runNats(ctx, cfg, cfg.Offset, cfg.FanartSubject,
		func(_ context.Context, env model.Envelope[model.FanartMessage]) error {
			m := env.Data

			return st.UpsertFanart(&store.Fanart{
				ID: m.ID, AnimeID: m.AnimeID, ImageURL: m.ImageURL, SourceURL: m.SourceURL,
			})
		})
}
