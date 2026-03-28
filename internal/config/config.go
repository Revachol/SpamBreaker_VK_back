package config

import (
	"os"
	"time"
)

type Config struct {
	Server   ServerConfig
	ML       MLConfig
	Postgres PostgresConfig
}

type ServerConfig struct {
	Port string
}

type MLConfig struct {
	BaseURL string
}

type PostgresConfig struct {
	Host                string        `yaml:"host"`
	Port                string        `yaml:"port"`
	Base                string        `yaml:"base"`
	User                string        `yaml:"user"`
	Password            string        `yaml:"password"`
	MinConns            int32         `yaml:"min_conns"`
	MaxConns            int32         `yaml:"max_conns"`
	MaxLife             time.Duration `yaml:"max_life"`
	MaxIdle             time.Duration `yaml:"max_idle"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`

	Migrated   bool   `yaml:"migrated"`
	Migrations string `yaml:"migrations"`
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		ML: MLConfig{
			BaseURL: getEnv("ML_SERVICE_URL", "http://localhost:8000"),
		},
		Postgres: PostgresConfig{
			Host:                getEnv("PG_HOST", "localhost"),
			Port:                getEnv("PG_PORT", "5432"),
			Base:                getEnv("PG_BASE", "spambreaker"),
			User:                getEnv("PG_USER", "uservice"),
			Password:            getEnv("PG_PSWD", "password"),
			MinConns:            1,
			MaxConns:            10,
			MaxLife:             time.Hour,
			MaxIdle:             30 * time.Minute,
			HealthCheckInterval: 1 * time.Minute,

			Migrated:   false,
			Migrations: "./configs/migrations/",
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
