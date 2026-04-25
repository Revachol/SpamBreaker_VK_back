package config

import (
	"fmt"
	"os"

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
	Logger      logger.LoggerConfig `yaml:"logger"`
	Vk          VkConfig            `yaml:"vk"`
	Metrics     MetricsConfig       `yaml:"metrics"`
	Core        ServiceConfig       `yaml:"core_service"`
}

type VkConfig struct {
	Token string `yaml:"token"`
}

type MetricsConfig struct {
	Port int `yaml:"port"`
}

type ServiceConfig struct {
	Host string
	Port string
}

func Load() (*Config, error) {
	path := getEnv("CONFIG_PATH", "./configs/vk_config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config appConfig
	config.App.Name = "App"
	config.App.Host = "0.0.0.0"
	config.App.Port = 8080
	config.App.SwaggerPath = "./api/server/swagger.json"

	config.App.Logger = logger.LoggerConfig{
		Level:     "INFO",
		Prefix:    "",
		Color:     true,
		Timestamp: true,
	}
	config.App.Metrics = MetricsConfig{
		Port: 8081,
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	loadEnv(&config.App)
	return &config.App, nil
}

func loadEnv(cfg *Config) {
	cfg.Mode = getEnv(cfg.Mode, "dev")

	cfg.Core.Host = getEnv(cfg.Core.Host, "localhost")
	cfg.Core.Port = getEnv(cfg.Core.Port, "8080")

	cfg.Vk.Token = getEnv(cfg.Vk.Token, "")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
