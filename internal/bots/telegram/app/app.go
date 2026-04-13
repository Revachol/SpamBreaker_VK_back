package app

import (
	"fmt"
	"os"

	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/telegram/config"
	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/telegram/handlers/bot"
	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/bots/telegram/handlers/http"
	"github.com/Revachol/SpamBreaker_VK_back/internal/clients/telegram"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
)

type App struct {
	logger   logger.Log
	config   *config.Config
	registry *prometheus.Registry
}

func Run() {
	app := &App{}

	// Читаем .env (игнорируем ошибку — в Docker переменные придут через environment).
	if err := godotenv.Load(); err != nil {
		logger.LOG.Info(".env not found, using system environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logger.LOG.Fatal(err)
	}
	app.config = cfg

	log := logger.New(&cfg.Logger)
	app.logger = log

	app.registry = prometheus.NewRegistry()
	coll := botmetrics.NewPrometheusBotCollector(app.config.Name, app.registry)

	if cfg.Telegram.Token == "" {
		log.Fatal("Telegram Bot token required")
	}

	// Инициализация Telegram-клиента.
	botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		log.Fatalf("failed to connect to Telegram: %v", err)
	}
	botAPI.Debug = os.Getenv("BOT_DEBUG") == "true"
	log.Infof("authorized as @%s", botAPI.Self.UserName)

	// Общий клиент к Core API.
	coreAddr := fmt.Sprintf("http://%s:%s", cfg.Core.Host, cfg.Core.Port)
	apiClient := telegram.NewAPIClient(coreAddr)

	router := httphandler.NewRouter(app.registry)
	coreAddr = fmt.Sprintf("%s:%s", cfg.Core.Host, cfg.Metrics.Port)
	go func() {
		if err := router.Run(coreAddr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Старт бота.
	tgBot := bot.NewBot(botAPI, apiClient, app.logger)
	tgBot.Run(coll)
}
