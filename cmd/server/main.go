package main

import (
	"fmt"
	"log"

	"github.com/Revachol/SpamBreaker_VK_back/config"
	mlclient "github.com/Revachol/SpamBreaker_VK_back/internal/client/ml"
	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/handlers/http"
	memrepo "github.com/Revachol/SpamBreaker_VK_back/internal/repository/memory"
	"github.com/Revachol/SpamBreaker_VK_back/internal/service"
)

func main() {
	// 1. Конфигурация из ENV.
	cfg := config.Load()

	// 2. Инфраструктурные зависимости.
	mlClient := mlclient.NewClient(cfg.ML.BaseURL)
	repo := memrepo.NewRepository() // ← заменить на postgres.NewRepository(...) когда будет БД

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
