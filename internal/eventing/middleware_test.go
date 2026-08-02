package eventing

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ThatCatDev/ep/v2/middleware"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func testEvent(topic string, headers map[string]string) epEvent[string] {
	return epEvent[string]{
		Headers:       headers,
		DriverMessage: &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic}},
	}
}

func TestLoggerReportsTheFailureAndPropagatesIt(t *testing.T) {
	// The error must be returned unchanged: the retry middleware sits after this one and
	// decides what to do based on it. Swallowing it here would restore exactly the old
	// behaviour — a logged failure that the pipeline then treats as success.
	buf := captureLogs(t)
	boom := errors.New("Unknown column 'language' in 'field list'")
	next := middleware.Handler[*kafka.Message, string](
		func(ctx context.Context, d epEvent[string]) (*epEvent[string], error) {
			return &d, boom
		})

	_, err := newLoggerMiddleware[string]().Process(context.Background(),
		testEvent("anime.news.v1", map[string]string{retryHeader: "1"}), next)

	if !errors.Is(err, boom) {
		t.Fatalf("error must propagate to the retry middleware, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "will be retried") || !strings.Contains(out, "anime.news.v1") {
		t.Fatalf("failure log should name the outcome and topic: %s", out)
	}
	if !strings.Contains(out, "attempt=1") {
		t.Fatalf("failure log should carry the attempt count: %s", out)
	}
}

func TestLoggerStaysQuietOnSuccess(t *testing.T) {
	buf := captureLogs(t)
	next := middleware.Handler[*kafka.Message, string](
		func(ctx context.Context, d epEvent[string]) (*epEvent[string], error) { return &d, nil })

	if _, err := newLoggerMiddleware[string]().Process(context.Background(),
		testEvent("anime.news.v1", nil), next); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "level=ERROR") {
		t.Fatalf("a successful message should not log an error: %s", buf.String())
	}
}

func TestTopicOfIsSafeOnIncompleteMessages(t *testing.T) {
	// Defensive: a nil DriverMessage or nil Topic must not panic inside logging, or a
	// logging detail takes down the consumer.
	if got := topicOf(epEvent[string]{}); got != "" {
		t.Fatalf("expected empty topic, got %q", got)
	}
	if got := topicOf(epEvent[string]{DriverMessage: &kafka.Message{}}); got != "" {
		t.Fatalf("expected empty topic, got %q", got)
	}
	if got := topicOf(testEvent("anime.fanart.v1", nil)); got != "anime.fanart.v1" {
		t.Fatalf("expected the topic name, got %q", got)
	}
}
