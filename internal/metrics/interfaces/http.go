package interfaces

// HTTP метрики
type HttpMetrics interface {
	// IncHTTPRequest увеличивает счетчик HTTP запросов
	// method - HTTP метод (GET, POST, PUT, DELETE и т.д.)
	// path - путь запроса (нормализованный, например /api/v1/users/:id)
	// statusCode - HTTP статус код ответа
	IncHTTPRequest(method, path string, statusCode int)

	// ObserveHTTPDuration измеряет продолжительность HTTP запроса
	// method - HTTP метод
	// path - путь запроса
	// statusCode - HTTP статус код
	// duration - продолжительность в секундах
	ObserveHTTPDuration(method, path string, statusCode int, duration float64)
}
