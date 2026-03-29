package config

import (
	"fmt"
	"os"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"gopkg.in/yaml.v3"
)

type appConfig struct {
	App Config `yaml:"app"`
}

type Config struct {
	Name        string              `yaml:"name"`
	Host        string              `yaml:"host"`
	Port        int                 `yaml:"port"`
	Prefix      string              `yaml:"prefix"`
	Mode        string              `yaml:"mode"`
	SwaggerPath string              `yaml:"swagger_path"`
	Cors        CORSConfig          `yaml:"cors"`
	Logger      logger.LoggerConfig `yaml:"logger"`
	Postgres    PostgresConfig      `yaml:"postgres"`
	ML          ServiceConfig       `yaml:"ml"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds"`
}

type ServiceConfig struct {
	Host string
	Port string
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

func Load() (*Config, error) {
	path := getEnv("CONFIG_PATH", "./config/core_config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config appConfig
	config.App.Name = "App"
	config.App.Host = "0.0.0.0"
	config.App.Port = 8080
	config.App.SwaggerPath = "./api/server/swagger.json"

	config.App.Cors = CORSConfig{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"},
		ExposeHeaders:    []string{"Content-Length", "Authorization"},
		AllowCredentials: true,
		MaxAgeSeconds:    86400,
	}
	config.App.Logger = logger.LoggerConfig{
		Level:     "INFO",
		Prefix:    "",
		Color:     true,
		Timestamp: true,
	}
	config.App.Postgres = PostgresConfig{
		MinConns:            1,
		MaxConns:            10,
		MaxLife:             time.Hour,
		MaxIdle:             30 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,

		Migrated:   false,
		Migrations: "./infra/migrations/",
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	loadEnv(&config.App)
	return &config.App, nil
}

func loadEnv(cfg *Config) {
	cfg.Mode = getEnv(cfg.Mode, "dev")

	cfg.Postgres.Host = getEnv(cfg.Postgres.Host, "localhost")
	cfg.Postgres.Port = getEnv(cfg.Postgres.Port, "5432")
	cfg.Postgres.Base = getEnv(cfg.Postgres.Base, "spambreaker")
	cfg.Postgres.User = getEnv(cfg.Postgres.User, "uservice")
	cfg.Postgres.Password = getEnv(cfg.Postgres.Password, "password")

	cfg.ML.Host = getEnv(cfg.ML.Host, "localhost")
	cfg.ML.Port = getEnv(cfg.ML.Port, "8080")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
