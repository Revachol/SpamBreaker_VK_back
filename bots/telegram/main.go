package main

import (
	"log"
	"os"

	"github.com/Revachol/SpamBreaker_VK_back/bots/shared"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Читаем .env (игнорируем ошибку — в Docker переменные придут через environment).
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env not found, using system environment")
	}

	token := mustEnv("TELEGRAM_TOKEN")
	apiURL := mustEnv("CORE_API_URL") // например: http://core-api:8080

	// Инициализация Telegram-клиента.
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("[FATAL] failed to connect to Telegram: %v", err)
	}
	botAPI.Debug = os.Getenv("BOT_DEBUG") == "true"
	log.Printf("[INFO] authorized as @%s", botAPI.Self.UserName)

	// Общий клиент к Core API.
	apiClient := shared.NewAPIClient(apiURL)

	// Старт бота.
	bot := NewBot(botAPI, apiClient)
	bot.Run()
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[FATAL] required env variable %q is not set", key)
	}
	return v
}
