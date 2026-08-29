package eventing

import (
	"context"

	"github.com/ThatCatDev/ep/v2/drivers"
	epNats "github.com/ThatCatDev/ep/v2/drivers/nats"
	"github.com/ThatCatDev/ep/v2/middlewares/nats/backoffretry"
	"github.com/ThatCatDev/ep/v2/processor"
	"golang.org/x/sync/errgroup"

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

	// The retry consumer runs in this same process rather than a second deployment. It
	// needs its own driver because the durable consumer name is driver-level
	// configuration, not per-subject: two Consume calls on one driver would call
	// CreateOrUpdateConsumer with the same durable name and different filter subjects,
	// and the second would reconfigure the first.
	retryCfg := cfg
	retryCfg.ConsumerGroupName = cfg.ConsumerGroupName + "-retry"
	retryDriver := newNatsDriver(retryCfg, offset)
	defer retryDriver.Close()

	build := func(d drivers.Driver[*epNats.Message], subj, retryQueue string) processor.Processor[*epNats.Message, P] {
		return processor.NewProcessor[*epNats.Message, P](d, subj,
			func(ctx context.Context, e natsEvent[P]) (natsEvent[P], error) {
				return e, handle(ctx, e.Payload)
			}).
			AddMiddleware(newNatsLoggerMiddleware[P]().Process).
			AddMiddleware(backoffretry.NewBackoffRetry[P](d, backoffretry.Config{
				MaxRetries: maxRetries,
				HeaderKey:  retryHeader,
				RetryQueue: retryQueue,
			}).Process)
	}

	// Exhausted retries go to a dead-letter subject rather than back onto the retry
	// subject. ep acks and drops a message once the counter reaches MaxRetries, so
	// cycling it here would make a permanently failing message disappear with no record.
	//
	// One consumer returning must stop the other: Consume blocks until its iterator is
	// stopped, so without cancelling here a dead main consumer would leave the process
	// alive and apparently healthy.
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return build(driver, subject, subject+"-retry").Run(groupCtx) })
	group.Go(func() error { return build(retryDriver, subject+"-retry", subject+"-dlq").Run(groupCtx) })

	return group.Wait()
}
