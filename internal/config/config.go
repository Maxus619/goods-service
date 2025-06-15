package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"log"
)

type Config struct {
	HttpPort string `env:"HTTP_PORT" envDefault:"8080"`

	DbHost     string `env:"DB_HOST" envDefault:"postgres"`
	DbPort     string `env:"DB_PORT" envDefault:"5432"`
	DbUser     string `env:"DB_USER" envDefault:"user"`
	DbPassword string `env:"DB_PASSWORD" envDefault:"password"`
	DbName     string `env:"DB_NAME" envDefault:"goods"`

	RedisHost string `env:"REDIS_HOST" envDefault:"redis"`
	RedisPort string `env:"REDIS_PORT" envDefault:"6379"`

	ChHost string `env:"CLICKHOUSE_HOST" envDefault:"clickhouse"`
	ChPort string `env:"CLICKHOUSE_PORT" envDefault:"9000"`

	NatsURL string `env:"NATS_URL" envDefault:"nats://nats:4222"`
}

func MustLoad() *Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("error loading .env file: %v", err)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	return &cfg
}
