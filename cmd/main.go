package main

import (
	"log/slog"
	"os"

	"github.com/weeb-vip/news-ingest/internal/app"
)

// news-ingest has two run modes (choose via the first arg, like the other
// weeb-vip services set in their Helm `args`):
//
//	news-ingest serve-api             # HTTP ingest → Kafka or NATS
//	news-ingest serve-consumer        # Kafka → Postgres
//	news-ingest serve-consumer-nats   # NATS  → Postgres
//
// serve-api has no -nats twin: its transport is chosen by PRODUCER_TYPE rather
// than by command, because the producer lives inside the HTTP server and there
// is no separate process to name. The consumers are standalone, so the command
// name is the clearer switch.
//
// There is no migrate mode. anime_news and anime_fanart belong to anime-api,
// which creates them in its own migrations; this service only upserts rows.
func main() {
	if len(os.Args) < 2 {
		slog.Error("usage: news-ingest <serve-api|serve-consumer|serve-consumer-nats>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve-api":
		err = app.ServeAPI()
	case "serve-consumer":
		err = app.ServeConsumer()
	case "serve-consumer-nats":
		err = app.ServeConsumerNats()
	default:
		slog.Error("unknown command", "cmd", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		slog.Error("exited with error", "err", err)
		os.Exit(1)
	}
}
