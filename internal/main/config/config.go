package config

import (
	"os"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
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
	ML          MLConfig            `yaml:"ml"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds"`
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
	var config appConfig
	config.App.Name = "App"
	config.App.Mode = getEnv("MODE", "dev")
	config.App.Host = "localhost"
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
		Migrations: "./infra/migrations/",
	}
	config.App.ML = MLConfig{
		BaseURL: getEnv("ML_SERVICE_URL", "http://localhost:8000"),
	}
	return &config.App
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
