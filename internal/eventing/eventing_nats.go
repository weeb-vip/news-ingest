package eventing

import (
	"context"

	"github.com/ThatCatDev/ep/v2/drivers"
	epNats "github.com/ThatCatDev/ep/v2/drivers/nats"
	"github.com/ThatCatDev/ep/v2/middlewares/nats/backoffretry"
	"github.com/ThatCatDev/ep/v2/processor"

	"github.com/weeb-vip/news-ingest/config"
)

// newNatsDriver builds the NATS driver. One per processor, for the same reason as the
// Kafka one: the driver owns the durable consumer, and sharing it across subjects would
// serialise them.
//
// StreamName is left unset. These subjects are produced by this service's own API rather
// than by Debezium, so no other stream declares them and the driver creates one per
// subject. Naming Debezium's CDC stream here would try to graft them onto retention
// sized for change events.
func newNatsDriver(cfg config.NatsConfig, offset string) drivers.Driver[*epNats.Message] {
	return epNats.NewNatsDriver(&epNats.Config{
		URL:                     cfg.URL,
		ConsumerGroupName:       cfg.ConsumerGroupName,
		ConsumerAutoOffsetReset: &offset,
	})
}

// runNats is run's twin: one subject, one handler, with retry.
//
// No transform middleware here either. These carry this service's own envelope as plain
// JSON, which ep unmarshals into the payload type directly -- only the Debezium CDC
// subjects need unwrapping.
func runNats[P any](ctx context.Context, cfg config.NatsConfig, offset, subject string,
	handle func(context.Context, P) error) error {
	driver := newNatsDriver(cfg, offset)
	defer driver.Close()

	proc := processor.NewProcessor[*epNats.Message, P](driver, subject,
		func(ctx context.Context, e natsEvent[P]) (natsEvent[P], error) {
			return e, handle(ctx, e.Payload)
		})

	retry := backoffretry.NewBackoffRetry[P](driver, backoffretry.Config{
		MaxRetries: maxRetries,
		HeaderKey:  retryHeader,
		RetryQueue: subject + "-retry",
	})

	return proc.
		AddMiddleware(newNatsLoggerMiddleware[P]().Process).
		AddMiddleware(retry.Process).
		Run(ctx)
}
