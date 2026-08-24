package config

import (
	"strings"

	"github.com/jinzhu/configor"
)

// Config is loaded from env vars (with struct defaults) — same convention as the
// other weeb-vip services. Kafka connection matches the internal plaintext cluster.
type Config struct {
	App    AppConfig
	DB     DBConfig
	Kafka  KafkaConfig
	Nats   NatsConfig
	Ingest IngestConfig

	// Which broker the API publishes to. The consumer picks its transport by
	// command; this producer lives inside the API server, so it needs a value.
	ProducerType string `default:"kafka" env:"PRODUCER_TYPE"`
}

type AppConfig struct {
	Port string `default:"3000" env:"PORT"`
}

type DBConfig struct {
	Host     string `default:"localhost" env:"DBHOST"`
	DataBase string `default:"weeb" env:"DBNAME"`
	User     string `default:"weeb" env:"DBUSERNAME"`
	Password string `default:"mysecretpassword" env:"DBPASSWORD"`
	Port     uint   `default:"5432" env:"DBPORT"`
	SSLMode  string `default:"require" env:"DBSSL"`
	// Own migration table, matching the convention in list-service/user-service. The
	// default `schema_migrations` is NOT safe here: news-ingest currently shares a database
	// with anime-api, which also migrates, and a shared table would have each service
	// overwrite the other's version number.
	MigrationTableName string `default:"__migrations_news-ingest" env:"DBMIGRATIONTABLE"`
}

type KafkaConfig struct {
	BootstrapServers  string `default:"localhost:9092" env:"KAFKA_BOOTSTRAP_SERVERS"`
	ConsumerGroupName string `default:"news-ingest" env:"KAFKA_CONSUMER_GROUP_NAME"`
	NewsTopic         string `default:"anime.news.v1" env:"KAFKA_NEWS_TOPIC"`
	FanartTopic       string `default:"anime.fanart.v1" env:"KAFKA_FANART_TOPIC"`
	Offset            string `default:"earliest" env:"KAFKA_OFFSET"`
}

// NatsConfig is the producer half of KafkaConfig, for PRODUCER_TYPE=nats.
//
// The subjects deliberately reuse the Kafka topic names: they are already
// dot-separated, which is exactly NATS's subject hierarchy, so nothing about
// the naming has to change.
type NatsConfig struct {
	URL string `default:"nats://localhost:4222" env:"NATSURL"`

	// The durable consumer name, the closest equivalent to a Kafka consumer
	// group. Left empty the consumer is ephemeral and loses its position across
	// restarts, which here means reprocessing whatever the stream still retains.
	ConsumerGroupName string `default:"news-ingest" env:"NATSCONSUMERGROUPNAME"`

	Offset string `default:"earliest" env:"NATSOFFSET"`

	NewsSubject   string `default:"anime.news.v1" env:"NATS_NEWS_SUBJECT"`
	FanartSubject string `default:"anime.fanart.v1" env:"NATS_FANART_SUBJECT"`
}

type IngestConfig struct {
	// Optional bearer token the research tool must present on POST /v1/news.
	Token string `env:"INGEST_TOKEN"`
}

func Load() Config {
	var c Config
	// No committed config file — env vars + defaults drive everything.
	_ = configor.New(&configor.Config{}).Load(&c)
	return c
}

// NewsDestination and FanartDestination return the topic or subject to publish
// to, depending on PRODUCER_TYPE.
//
// They exist so the call sites do not each have to branch on the transport.
// The two happen to hold the same strings today -- Kafka topic names are
// already dot-separated, which is exactly NATS's subject hierarchy -- but they
// are separate settings so one can move without the other.
func (c Config) NewsDestination() string {
	if c.isNats() {
		return c.Nats.NewsSubject
	}

	return c.Kafka.NewsTopic
}

func (c Config) FanartDestination() string {
	if c.isNats() {
		return c.Nats.FanartSubject
	}

	return c.Kafka.FanartTopic
}

func (c Config) isNats() bool {
	return strings.EqualFold(strings.TrimSpace(c.ProducerType), "nats")
}
