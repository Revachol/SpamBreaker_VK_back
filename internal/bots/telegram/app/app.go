package app

import (
	"fmt"
	"os"

	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/telegram/config"
	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/telegram/utils"
	"github.com/Revachol/SpamBreaker_VK_back/internal/clients/telegram"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func Run() {
	// Читаем .env (игнорируем ошибку — в Docker переменные придут через environment).
	if err := godotenv.Load(); err != nil {
		logger.LOG.Info(".env not found, using system environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logger.LOG.Fatal(err)
	}

	if cfg.Telegram.Token == "" {
		logger.LOG.Fatal("Telegram Bot token required")
	}

	// Инициализация Telegram-клиента.
	botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		logger.LOG.Fatalf("failed to connect to Telegram: %v", err)
	}
	botAPI.Debug = os.Getenv("BOT_DEBUG") == "true"
	logger.LOG.Infof("authorized as @%s", botAPI.Self.UserName)

	// Общий клиент к Core API.
	core_addr := fmt.Sprintf("http://%s:%s", cfg.Core.Host, cfg.Core.Port)
	apiClient := telegram.NewAPIClient(core_addr)

	// Старт бота.
	bot := utils.NewBot(botAPI, apiClient)
	bot.Run()
}
