package config

import (
	"fmt"

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
	LogLevel         string `env:"LOG_LEVEL" env-default:"info"`
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

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
	return splitAndTrim(c.KafkaBrokers)
}

func splitAndTrim(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			trimmed := trimSpace(current)
			if trimmed != "" {
				result = append(result, trimmed)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	trimmed := trimSpace(current)
	if trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
