package config

import (
	"os"
)

type Config struct {
	Server ServerConfig
	ML     MLConfig
}

type ServerConfig struct {
	Port string
}

type MLConfig struct {
	BaseURL string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		ML: MLConfig{
			BaseURL: getEnv("ML_SERVICE_URL", "http://localhost:8000"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
