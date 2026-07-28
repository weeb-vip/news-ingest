package config

import "github.com/jinzhu/configor"

// Config is loaded from env vars (with struct defaults) — same convention as the
// other weeb-vip services. Kafka connection matches the internal plaintext cluster.
type Config struct {
	App    AppConfig
	DB     DBConfig
	Kafka  KafkaConfig
	Ingest IngestConfig
}

type AppConfig struct {
	Port string `default:"3000" env:"PORT"`
}

type DBConfig struct {
	Host     string `default:"localhost" env:"DBHOST"`
	DataBase string `default:"weeb" env:"DBNAME"`
	User     string `default:"weeb" env:"DBUSERNAME"`
	Password string `default:"mysecretpassword" env:"DBPASSWORD"`
	Port     uint   `default:"3306" env:"DBPORT"`
	SSLMode  string `default:"false" env:"DBSSL"`
}

type KafkaConfig struct {
	BootstrapServers  string `default:"localhost:9092" env:"KAFKA_BOOTSTRAP_SERVERS"`
	ConsumerGroupName string `default:"news-ingest" env:"KAFKA_CONSUMER_GROUP_NAME"`
	NewsTopic         string `default:"anime.news.v1" env:"KAFKA_NEWS_TOPIC"`
	FanartTopic       string `default:"anime.fanart.v1" env:"KAFKA_FANART_TOPIC"`
	Offset            string `default:"earliest" env:"KAFKA_OFFSET"`
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
