package kafkax

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Producer writes one message per item. Keyed by anime_id (Hash balancer) so all
// of an anime's events land on the same partition and stay ordered.
type Producer struct {
	w *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		w: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Balancer:               &kafka.Hash{},
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: false,
		},
	}
}

func (p *Producer) Write(ctx context.Context, topic string, key, value []byte) error {
	return p.w.WriteMessages(ctx, kafka.Message{Topic: topic, Key: key, Value: value})
}

func (p *Producer) Close() error { return p.w.Close() }
