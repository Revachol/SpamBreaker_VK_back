package app

import (
	"fmt"

	httphandler "github.com/Revachol/SpamBreaker_VK_back/internal/bots/utils"
	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/vk/config"
	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/vk/handlers"
	check_client "github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
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

	if cfg.Vk.Token == "" {
		log.Fatal("Vk Bot token required")
	}

	// Инициализация VK Teams бота
	//dbg := os.Getenv("BOT_DEBUG") == "true"
	vk := api.NewVK(cfg.Vk.Token)

	// get information about the group
	group, err := vk.GroupsGetByID(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Initializing Long Poll
	lp, err := longpoll.NewLongPoll(vk, group[0].ID)
	if err != nil {
		log.Fatalf("Vk Bot could not be created with error: %s", err.Error())
	}
	log.Infof("authorized as @%s", lp.Server)

	coreAddr := fmt.Sprintf("http://%s:%s/api/bot/v1/vk", cfg.Core.Host, cfg.Core.Port)
	apiClient := check_client.NewAPIClient(coreAddr)

	router := httphandler.NewRouter(app.registry)
	coreAddr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Metrics.Port)
	go func() {
		if err := router.Run(coreAddr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Старт бота.
	vkBot := handlers.NewBot(vk, lp, apiClient, app.logger)
	vkBot.Run(coll)
}
