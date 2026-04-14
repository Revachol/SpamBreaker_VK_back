package botmetrics

// BotMetricsIface собирает метрики для телеграм ботов.
// Поле botName позволяет различать несколько ботов в одном приложении.
type BotMetricsIface interface {
	// IncBotRequest увеличивает счётчик запросов для команды/типа сообщения.
	// status: "success", "error" или другой статус по желанию.
	IncBotRequest(command, status string)
	// ObserveBotDuration записывает длительность обработки запроса.
	ObserveBotDuration(command, status string, duration float64)
}
