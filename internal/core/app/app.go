package app

import (
	"context"
	"fmt"

	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/clients/ml"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/core/handlers/http"
	repository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/postgres"
	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	jwtpkg "github.com/Revachol/SpamBreaker_VK_back/pkg/jwt"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	postgres "github.com/Revachol/SpamBreaker_VK_back/pkg/postgres"
)

func Run() {
	// 1. Конфигурация из ENV.
	cfg, err := config.Load()
	if err != nil {
		logger.LOG.Fatal(err)
	}

	log := logger.New(&cfg.Logger)

	// 2. Инфраструктурные зависимости.
	mlAddr := fmt.Sprintf("http://%s:%s", cfg.ML.Host, cfg.ML.Port)
	mlClient := mlclient.NewClient(mlAddr)

	pgx, err := postgres.NewConnect(context.Background(), &cfg.Postgres)
	if err != nil {
		logger.LOG.Fatal(err)
	}

	runMigrations(&cfg.Postgres, log)

	// 3. Репозитории.
	messageRepo := repository.NewMessageRepository(pgx)
	moderatorRepo := repository.NewModeratorRepository(pgx)

	// 4. JWT-менеджер.
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)

	// 5. Бизнес-логика.
	moderationUC := service.NewModerationUseCase(mlClient, messageRepo)
	authUC := service.NewAuthUseCase(moderatorRepo, jwtManager)

	// 6. Transport layer.
	handler := httphandler.NewHandler(moderationUC)
	authHandler := httphandler.NewAuthHandler(authUC)
	router := httphandler.NewRouter(handler, authHandler, jwtManager)

	// 7. Старт.
	coreAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Infof("Core API starting on %s  |  ML service: %s", coreAddr, mlAddr)

	if err := router.Run(coreAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
