package main

import (
	"log/slog"
	"os"

	"github.com/weeb-vip/news-ingest/internal/app"
)

// news-ingest has two run modes (choose via the first arg, like the other
// weeb-vip services set in their Helm `args`):
//
//	news-ingest serve-api        # HTTP ingest → Kafka
//	news-ingest serve-consumer   # Kafka → MySQL
func main() {
	if len(os.Args) < 2 {
		slog.Error("usage: news-ingest <serve-api|serve-consumer>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve-api":
		err = app.ServeAPI()
	case "serve-consumer":
		err = app.ServeConsumer()
	default:
		slog.Error("unknown command", "cmd", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		slog.Error("exited with error", "err", err)
		os.Exit(1)
	}
}
