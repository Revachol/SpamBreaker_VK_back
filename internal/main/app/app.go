package app

import (
	"context"
	"fmt"
	"log"

	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/clients/ml"
	"github.com/Revachol/SpamBreaker_VK_back/internal/main/config"
	"github.com/Revachol/SpamBreaker_VK_back/internal/main/handlers/http"
	"github.com/Revachol/SpamBreaker_VK_back/internal/main/repository/postgres"
	"github.com/Revachol/SpamBreaker_VK_back/internal/main/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/postgres"
)

func Run() {
	// 1. Конфигурация из ENV.
	cfg := config.Load()

	// 2. Инфраструктурные зависимости.
	mlClient := mlclient.NewClient(cfg.ML.BaseURL)
	pgx, err := postgres.NewConnect(context.Background(), &cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	repo := repository.NewMessageRepository(pgx)

	// 3. Бизнес-логика.
	moderationUC := service.NewModerationUseCase(mlClient, repo)

	// 4. Transport layer.
	handler := httphandler.NewHandler(moderationUC)
	router := httphandler.NewRouter(handler)

	// 5. Старт.
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Core API starting on %s  |  ML service: %s", addr, cfg.ML.BaseURL)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
