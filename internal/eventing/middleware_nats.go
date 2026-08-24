package eventing

import (
	"context"
	"log/slog"

	epNats "github.com/ThatCatDev/ep/v2/drivers/nats"
	"github.com/ThatCatDev/ep/v2/event"
	"github.com/ThatCatDev/ep/v2/middleware"
)

// natsEvent is ep's event type for a NATS message carrying payload P, the twin of
// epEvent.
type natsEvent[P any] = event.Event[*epNats.Message, P]

// natsLoggerMiddleware is loggerMiddleware for the NATS driver.
//
// It is a separate type only because the middleware interface is typed on the driver
// message. The placement rule is the same and matters just as much: it sits BEFORE the
// retry middleware so it sees the original failure rather than the retry middleware's
// verdict.
type natsLoggerMiddleware[P any] struct{}

func newNatsLoggerMiddleware[P any]() *natsLoggerMiddleware[P] { return &natsLoggerMiddleware[P]{} }

func (l *natsLoggerMiddleware[P]) Process(ctx context.Context, data natsEvent[P],
	next middleware.Handler[*epNats.Message, P]) (*natsEvent[P], error) {
	result, err := next(ctx, data)
	if err != nil {
		slog.Error("handler failed; message will be retried",
			"subject", subjectOf(data), "attempt", data.Headers[retryHeader], "err", err)

		return result, err
	}
	slog.Debug("message processed", "subject", subjectOf(data))

	return result, nil
}

// subjectOf is topicOf's counterpart. A NATS message carries its destination as a plain
// Subject field; there is no partition, so there is nothing else to read.
func subjectOf[P any](data natsEvent[P]) string {
	if data.DriverMessage == nil {
		return ""
	}

	return data.DriverMessage.Subject
}
