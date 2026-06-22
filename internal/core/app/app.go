package app

import (
	"context"
	"fmt"

	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/clients/ml"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/core/handlers/http"
	repositoryInterfaces "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	repository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/postgres"
	redisRepository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/redis"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	jwtpkg "github.com/Revachol/SpamBreaker_VK_back/pkg/jwt"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/postgres"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/redis"
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
	mlClient := mlclient.NewClient(mlAddr, app.logger)

	pgx, err := postgres.NewConnect(context.Background(), &cfg.Postgres)
	if err != nil {
		app.logger.Fatal(err)
	}

	runMigrations(&cfg.Postgres, log)

	// 3. Репозитории.
	var messageRepo repositoryInterfaces.MessageRepository = repository.NewMessageRepository(pgx, app.logger)
	var messageBufferRepo repositoryInterfaces.BufferRepository
	if cfg.Redis.Enabled {
		redisClient, err := redis.NewConnect(context.Background(), &cfg.Redis)
		if err != nil {
			app.logger.Fatal(err)
		}
		messageBufferRepo = redisRepository.NewBufferRepository(
			redisClient,
			cfg.Redis.ListLimit,
			cfg.Redis.ListTTL,
			app.logger,
		)
	}
	moderatorRepo := repository.NewModeratorRepository(pgx, app.logger)
	modAccRepo := repository.NewModeratorAccountRepo(pgx, &app.logger)
	applicationRepo := repository.NewApplicationRepository(pgx, app.logger)
	applicationSettingsRepo := repository.NewApplicationSettingsRepository(pgx, app.logger)

	// 5. JWT-менеджер.
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)

	// 6. Бизнес-логика.
	modAccService := service.NewModeratorService(moderatorRepo, modAccRepo, applicationRepo, app.logger)
	moderationUC := service.NewModerationUseCase(mlClient, messageRepo, messageBufferRepo, app.logger)
	authUC := service.NewAuthUseCase(moderatorRepo, jwtManager, app.logger)
	botUC := service.NewBotUseCase(applicationRepo, applicationSettingsRepo, modAccRepo, app.logger)

	// 6. Transport layer.
	handler := httphandler.NewBotHandler(moderationUC, botUC, modAccService, app.logger)
	authHandler := httphandler.NewAuthHandler(authUC, app.logger)
	moderHandler := httphandler.NewModeratorHandler(modAccService, botUC, app.logger)
	botHandler := httphandler.NewBotManageHandler(botUC, modAccService, app.logger)
	router := httphandler.NewRouter(
		handler,
		authHandler,
		moderHandler,
		botHandler,
		jwtManager,
		app.registry,
		app.config,
		app.logger,
	)

	// 7. Старт.
	coreAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	app.logger.Infof("Core API starting on %s  |  ML service: %s", coreAddr, mlAddr)

	if err := router.Run(coreAddr); err != nil {
		app.logger.Fatalf("server error: %v", err)
	}
}
