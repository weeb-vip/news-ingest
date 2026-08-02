package config

import "testing"

func TestDefaultsMatchTheDeployedTopics(t *testing.T) {
	// There is no committed config file: env vars and these struct defaults are the whole
	// story. A default that drifts from the topic names in weeb-argocd produces a consumer
	// that starts cleanly, joins a group nobody produces to, and processes nothing — which
	// looks identical to "there is no news yet".
	c := Load()
	if c.Kafka.NewsTopic != "anime.news.v1" {
		t.Errorf("news topic default = %q", c.Kafka.NewsTopic)
	}
	if c.Kafka.FanartTopic != "anime.fanart.v1" {
		t.Errorf("fanart topic default = %q", c.Kafka.FanartTopic)
	}
	if c.Kafka.ConsumerGroupName != "news-ingest" {
		t.Errorf("consumer group default = %q", c.Kafka.ConsumerGroupName)
	}
}

func TestOffsetDefaultsToEarliest(t *testing.T) {
	// "latest" would silently skip everything already in the topic on a fresh group — so a
	// redeploy after an incident would abandon exactly the backlog you are trying to drain.
	if c := Load(); c.Kafka.Offset != "earliest" {
		t.Errorf("offset default = %q, want earliest", c.Kafka.Offset)
	}
}

func TestEnvironmentOverridesDefaults(t *testing.T) {
	t.Setenv("KAFKA_BOOTSTRAP_SERVERS", "broker-1:9092,broker-2:9092")
	t.Setenv("KAFKA_NEWS_TOPIC", "anime.news.v2")
	t.Setenv("DBNAME", "weeb_staging")

	c := Load()
	if c.Kafka.BootstrapServers != "broker-1:9092,broker-2:9092" {
		t.Errorf("brokers not overridden: %q", c.Kafka.BootstrapServers)
	}
	if c.Kafka.NewsTopic != "anime.news.v2" {
		t.Errorf("news topic not overridden: %q", c.Kafka.NewsTopic)
	}
	if c.DB.DataBase != "weeb_staging" {
		t.Errorf("database not overridden: %q", c.DB.DataBase)
	}
}

func TestIngestTokenHasNoDefault(t *testing.T) {
	// An empty token means the API rejects everything rather than accepting anything: a
	// baked-in default would be a public write endpoint the day someone forgot to set it.
	if c := Load(); c.Ingest.Token != "" {
		t.Error("INGEST_TOKEN must not have a compiled-in default")
	}
}
