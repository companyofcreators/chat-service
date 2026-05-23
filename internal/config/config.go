package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddress      string `env:"HTTP_ADDRESS" env-default:":8085"`
	DBDSN            string `env:"DB_DSN" env-required:"true"`
	RedisAddr        string `env:"REDIS_ADDR" env-default:"localhost:6379"`
	RedisPassword    string `env:"REDIS_PASSWORD"`
	KafkaBrokers     string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	JWTPublicKeyPath string `env:"JWT_PUBLIC_KEY_PATH" env-required:"true"`
	HeaderHMACKey    string `env:"HEADER_HMAC_KEY" env-default:"diploma-internal-hmac-secret-key-2026"`
	LogLevel         string `env:"LOG_LEVEL" env-default:"info"`
	AllowedOrigin    string `env:"WS_ALLOWED_ORIGIN" env-default:""`
}

func Load() (*Config, error) {
	if err := godotenv.Load(".env"); err != nil {
		slog.Warn(".env file not found, using environment variables", "error", err)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) KafkaBrokersList() []string {
	if c.KafkaBrokers == "" {
		return []string{"localhost:9092"}
	}
	return splitBrokers(c.KafkaBrokers)
}

func splitBrokers(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
