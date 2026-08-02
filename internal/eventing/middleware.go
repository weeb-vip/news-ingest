package eventing

import (
	"context"
	"log/slog"

	"github.com/ThatCatDev/ep/v2/event"
	"github.com/ThatCatDev/ep/v2/middleware"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// epEvent is ep's event type for a Kafka message carrying payload P. Aliased because the
// full generic spelling appears in every handler signature and obscures them.
type epEvent[P any] = event.Event[*kafka.Message, P]

// loggerMiddleware records the outcome of each message.
//
// It sits BEFORE the retry middleware in the chain so it sees the original failure. Placed
// after, it would only ever see the retry middleware's verdict and every failure would look
// identical — which is how the previous consumer managed to drop messages while appearing
// healthy.
type loggerMiddleware[P any] struct{}

func newLoggerMiddleware[P any]() *loggerMiddleware[P] { return &loggerMiddleware[P]{} }

func (l *loggerMiddleware[P]) Process(ctx context.Context, data epEvent[P],
	next middleware.Handler[*kafka.Message, P]) (*epEvent[P], error) {
	result, err := next(ctx, data)
	if err != nil {
		// Retry count comes from the header ep maintains, so the log distinguishes a first
		// failure from one that is about to exhaust its attempts.
		slog.Error("handler failed; message will be retried",
			"topic", topicOf(data), "attempt", data.Headers[retryHeader], "err", err)
		return result, err
	}
	slog.Debug("message processed", "topic", topicOf(data))
	return result, nil
}

func topicOf[P any](data epEvent[P]) string {
	if data.DriverMessage != nil && data.DriverMessage.TopicPartition.Topic != nil {
		return *data.DriverMessage.TopicPartition.Topic
	}
	return ""
}
