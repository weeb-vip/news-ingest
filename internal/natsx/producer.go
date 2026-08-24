package natsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Producer publishes one message per item to a JetStream subject.
//
// It mirrors kafkax.Producer so the two are interchangeable behind
// eventbus.Producer. The signature keeps Kafka's key parameter, but the key
// means less here: Kafka used it to pick a partition, which is what kept an
// anime's events ordered. JetStream has no partitions and no equivalent, so the
// key travels as a header for anything downstream that wants it and does not
// affect delivery order.
//
// That is a real behavioural difference, and it is acceptable for this data:
// news is upserted by its dedupe id rather than applied as ordered deltas, so
// two events for the same anime arriving out of order converge on the same row.
type Producer struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

func NewProducer(url string) (*Producer, error) {
	conn, err := nats.Connect(url,
		nats.Name("news-ingest-api"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()

		return nil, fmt.Errorf("jetstream: %w", err)
	}

	return &Producer{conn: conn, js: js}, nil
}

// Write publishes value to subject, creating the backing stream if it is not
// there yet.
//
// The stream is per-subject and created on demand because these subjects are
// produced here rather than by Debezium: nothing else declares a stream over
// them, so there is no existing one to bind to.
func (p *Producer) Write(ctx context.Context, subject string, key, value []byte) error {
	if err := p.ensureStream(ctx, subject); err != nil {
		return err
	}

	msg := &nats.Msg{Subject: subject, Data: value}
	if len(key) > 0 {
		msg.Header = nats.Header{"Nats-Msg-Key": []string{string(key)}}
	}

	if _, err := p.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}

	return nil
}

func (p *Producer) ensureStream(ctx context.Context, subject string) error {
	name := streamName(subject)
	if _, err := p.js.Stream(ctx, name); err == nil {
		return nil
	}

	_, err := p.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: []string{subject},
		Storage:  jetstream.FileStorage,
	})
	// A racing publisher may have created it between the lookup and here.
	if err != nil && !errorsIsAlreadyInUse(err) {
		return fmt.Errorf("create stream %s: %w", name, err)
	}

	return nil
}

func (p *Producer) Close() error {
	p.conn.Close()

	return nil
}

// streamName derives a stream name from a subject. JetStream forbids dots in
// stream names, so anime.news.v1 becomes ANIME_NEWS_V1.
func streamName(subject string) string {
	out := make([]rune, 0, len(subject))
	for _, r := range subject {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r-32)
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}

	return string(out)
}

func errorsIsAlreadyInUse(err error) bool {
	return errors.Is(err, jetstream.ErrStreamNameAlreadyInUse)
}
