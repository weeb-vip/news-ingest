// Package eventing consumes the news and fanart topics through ep, the same event
// processor the other weeb-vip consumers use (see anime-sync).
//
// It replaces a hand-rolled segmentio/kafka-go loop whose failure path was:
//
//	if err := h(...); err != nil { slog.Error("handler failed; committing anyway ...") }
//	r.CommitMessages(ctx, m)   // ran regardless
//
// The offset was committed whether or not the write succeeded, so a failed message was
// gone — not retried, not replayable. That is fine for a poison message and catastrophic
// for anything transient or environmental: when a deploy landed ahead of its migration,
// every message in the gap hit "Unknown column 'language'" and was silently destroyed
// while the consumer reported itself healthy.
//
// ep's backoffretry middleware republishes a failed message to <topic>-retry with a
// `retry` header counting attempts, so the same incident leaves a visible, replayable
// backlog instead of a hole. After MaxRetries the message stops circulating, which keeps
// a genuinely poisonous message from looping forever — the property the old code was
// reaching for, without the data loss.
package eventing

import (
	"context"

	"github.com/ThatCatDev/ep/v2/drivers"
	epKafka "github.com/ThatCatDev/ep/v2/drivers/kafka"
	"github.com/ThatCatDev/ep/v2/middlewares/kafka/backoffretry"
	"github.com/ThatCatDev/ep/v2/processor"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/weeb-vip/news-ingest/config"
)

// maxRetries matches anime-sync. Three attempts covers a restart or a brief database
// blip; beyond that the problem needs a human, and continuing to retry only obscures it.
const maxRetries = 3

// retryHeader is the header ep increments per attempt. Must match anime-sync so the
// retry topics can be inspected the same way.
const retryHeader = "retry"

// newDriver builds the Kafka driver. One per processor: the driver owns the consumer
// group session, and sharing it across topics would serialise them.
func newDriver(cfg config.KafkaConfig) drivers.Driver[*kafka.Message] {
	offset := cfg.Offset
	return epKafka.NewKafkaDriver(&epKafka.KafkaConfig{
		ConsumerGroupName:       cfg.ConsumerGroupName,
		BootstrapServers:        cfg.BootstrapServers,
		ConsumerAutoOffsetReset: &offset,
	})
}

// run wires one topic to one handler, with retry.
//
// There is deliberately no transform middleware here, unlike anime-sync: those topics
// carry Debezium change events (base64-wrapped {schema, payload}) that need unwrapping
// first, whereas these carry our own envelope as plain JSON, which ep unmarshals into the
// payload type directly.
func run[P any](ctx context.Context, cfg config.KafkaConfig, topic string,
	handle func(context.Context, P) error) error {
	driver := newDriver(cfg)
	defer driver.Close()

	proc := processor.NewProcessor[*kafka.Message, P](driver, topic,
		func(ctx context.Context, e epEvent[P]) (epEvent[P], error) {
			return e, handle(ctx, e.Payload)
		})

	retry := backoffretry.NewBackoffRetry[P](driver, backoffretry.Config{
		MaxRetries: maxRetries,
		HeaderKey:  retryHeader,
		RetryQueue: topic + "-retry",
	})

	return proc.
		AddMiddleware(newLoggerMiddleware[P]().Process).
		AddMiddleware(retry.Process).
		Run(ctx)
}
