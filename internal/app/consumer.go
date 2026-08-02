package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/eventing"
	"github.com/weeb-vip/news-ingest/internal/store"
)

// ServeConsumer reads the news + fanart topics and upserts into MySQL.
//
// One processor per topic, run concurrently. ep binds a processor to a single topic, and
// each owns its own consumer-group session — so a slow or failing topic cannot stall the
// other, which the previous single-loop implementation allowed.
func ServeConsumer() error {
	cfg := config.Load()
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}

	ctx := signalContext()
	slog.Info("news-ingest consumer starting",
		"topics", []string{cfg.Kafka.NewsTopic, cfg.Kafka.FanartTopic},
		"group", cfg.Kafka.ConsumerGroupName, "brokers", cfg.Kafka.BootstrapServers)

	// errgroup rather than bare goroutines: if either processor dies the whole consumer
	// should exit and let the orchestrator restart it. Silently continuing on one topic
	// looks healthy from outside while half the pipeline is dead.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return eventing.ConsumeNews(gctx, cfg.Kafka, st) })
	g.Go(func() error { return eventing.ConsumeFanart(gctx, cfg.Kafka, st) })
	return g.Wait()
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
