package kafkax

import (
	"context"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Handler processes one message value for a topic. If it returns an error the
// offset is still committed (and the error logged) so a poison message can't
// crash-loop the consumer — the upsert is idempotent, so at-most-once here is safe.
type Handler func(ctx context.Context, topic string, value []byte) error

// Consume reads the given topics under one consumer group until ctx is cancelled.
func Consume(ctx context.Context, brokers, group, offset string, topics []string, h Handler) error {
	startOffset := kafka.LastOffset
	if offset == "earliest" {
		startOffset = kafka.FirstOffset
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(brokers, ","),
		GroupID:     group,
		GroupTopics: topics,
		StartOffset: startOffset,
	})
	defer r.Close()

	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			return err // ctx cancelled or unrecoverable
		}
		if err := h(ctx, m.Topic, m.Value); err != nil {
			slog.Error("handler failed; committing anyway to avoid crash-loop",
				"topic", m.Topic, "err", err)
		}
		if err := r.CommitMessages(ctx, m); err != nil {
			slog.Error("commit failed", "topic", m.Topic, "err", err)
		}
	}
}
