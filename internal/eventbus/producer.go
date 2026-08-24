package eventbus

import (
	"context"
	"fmt"
	"strings"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/kafkax"
	"github.com/weeb-vip/news-ingest/internal/natsx"
)

// Producer is the write side of the event bus, in whichever transport is
// configured.
//
// The signature is Kafka's, because that is what the call sites already speak
// and the NATS implementation can honour it -- topic becomes a subject, and the
// key becomes a header rather than a partition selector.
type Producer interface {
	Write(ctx context.Context, topic string, key, value []byte) error
	Close() error
}

// NewProducer builds the producer named by PRODUCER_TYPE.
//
// A runtime switch rather than a separate command: this producer lives inside
// the API server, so there is no command name to select a transport with the
// way the consumer has. Staging can be moved by setting PRODUCER_TYPE=nats on
// the deployment while production stays on the default.
//
// Unknown values are an error rather than a fallback to Kafka. A producer
// writing to a broker nobody reads fails silently -- no errors, events simply
// never arrive -- so a typo must not be allowed to look like a healthy service.
func NewProducer(cfg config.Config) (Producer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.ProducerType)) {
	case "", "kafka":
		return kafkax.NewProducer(cfg.Kafka.BootstrapServers), nil
	case "nats":
		return natsx.NewProducer(cfg.Nats.URL)
	default:
		return nil, fmt.Errorf("unknown PRODUCER_TYPE %q: expected kafka or nats", cfg.ProducerType)
	}
}
