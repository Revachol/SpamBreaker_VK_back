package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Revachol/SpamBreaker_VK_back/config"
	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/client/ml"
	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/handlers/http"
	repository "github.com/Revachol/SpamBreaker_VK_back/internal/repository/postgres"
	"github.com/Revachol/SpamBreaker_VK_back/internal/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/postgres"
)

func main() {
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
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Core API starting on %s  |  ML service: %s", addr, cfg.ML.BaseURL)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
