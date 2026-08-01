package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/kafkax"
	"github.com/weeb-vip/news-ingest/internal/model"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// ServeConsumer reads the news + fanart topics and upserts into MySQL.
func ServeConsumer() error {
	cfg := config.Load()
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}

	topics := []string{cfg.Kafka.NewsTopic, cfg.Kafka.FanartTopic}
	ctx := signalContext()
	slog.Info("news-ingest consumer starting",
		"topics", topics, "group", cfg.Kafka.ConsumerGroupName, "brokers", cfg.Kafka.BootstrapServers)

	return kafkax.Consume(ctx, cfg.Kafka.BootstrapServers, cfg.Kafka.ConsumerGroupName, cfg.Kafka.Offset, topics,
		func(_ context.Context, topic string, value []byte) error {
			switch topic {
			case cfg.Kafka.NewsTopic:
				return handleNews(st, value)
			case cfg.Kafka.FanartTopic:
				return handleFanart(st, value)
			default:
				return nil
			}
		})
}

func handleNews(st *store.Store, value []byte) error {
	var env model.Envelope[model.NewsMessage]
	if err := json.Unmarshal(value, &env); err != nil {
		return err
	}
	m := env.Data
	n := &store.AnimeNews{
		ID: m.ID, AnimeID: m.AnimeID, MalID: m.MalID, Title: m.Title, Summary: m.Summary,
		Category: m.Category, SourceURL: m.SourceURL, SourceName: m.SourceName, EpisodeNumber: m.EpisodeNumber,
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
	n.Language = m.Language
	if len(m.References) > 0 {
		// Stored as a JSON document. A marshal failure must not poison the whole item —
		// the news itself is worth more than its attachments, so drop them and continue.
		if b, err := json.Marshal(m.References); err == nil {
			s := string(b)
			n.References = &s
		} else {
			slog.Warn("dropping references; marshal failed", "id", m.ID, "err", err)
		}
	}
	return st.UpsertNews(n)
}

func handleFanart(st *store.Store, value []byte) error {
	var env model.Envelope[model.FanartMessage]
	if err := json.Unmarshal(value, &env); err != nil {
		return err
	}
	m := env.Data
	return st.UpsertFanart(&store.Fanart{ID: m.ID, AnimeID: m.AnimeID, ImageURL: m.ImageURL, SourceURL: m.SourceURL})
}

func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		slog.Info("shutdown signal received")
		cancel()
	}()
	return ctx
}
