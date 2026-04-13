package app

import (
	"context"
	"fmt"

	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/clients/ml"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/handlers/http"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/postgres"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/postgres"
	"github.com/prometheus/client_golang/prometheus"
)

type App struct {
	config   *config.Config
	logger   logger.Log
	registry *prometheus.Registry
}

func Run() {
	app := &App{}

	// 1. Конфигурация из ENV.
	cfg, err := config.Load()
	if err != nil {
		logger.LOG.Fatal(err)
	}
	app.config = cfg

	log := logger.New(&cfg.Logger)
	app.logger = log

	app.registry = prometheus.NewRegistry()

	// 2. Инфраструктурные зависимости.
	mlAddr := fmt.Sprintf("http://%s:%s", cfg.ML.Host, cfg.ML.Port)
	mlClient := mlclient.NewClient(mlAddr)
	pgx, err := postgres.NewConnect(context.Background(), &cfg.Postgres)
	if err != nil {
		logger.LOG.Fatal(err)
	}
	repo := repository.NewMessageRepository(pgx)
	runMigrations(&cfg.Postgres, log)

	// 3. Бизнес-логика.
	moderationUC := service.NewModerationUseCase(mlClient, repo)

	// 4. Transport layer.
	handler := httphandler.NewHandler(moderationUC)
	router := httphandler.NewRouter(handler, app.registry, app.config.Name, app.logger)

	// 5. Старт.
	coreAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Infof("Core API starting on %s  |  ML service: %s", coreAddr, mlAddr)

	if err := router.Run(coreAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
