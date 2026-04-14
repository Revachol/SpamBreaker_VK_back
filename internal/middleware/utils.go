package middleware

import (
	"regexp"
	"strings"
)

// NormalizePath нормализует путь URL, заменяя числовые ID и UUID на плейсхолдеры
func NormalizePath(path string) string {
	path, _ = strings.CutSuffix(path, "?")

	// Заменяем UUID на :id
	uuidRegex := regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	path = uuidRegex.ReplaceAllString(path, ":id")

	// Заменяем числовые ID на :id
	numericRegex := regexp.MustCompile(`/\d+`)
	path = numericRegex.ReplaceAllString(path, "/:id")

	// Заменяем email-like строки
	emailRegex := regexp.MustCompile(`/[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	path = emailRegex.ReplaceAllString(path, "/:email")

	// Заменяем хеши и токены
	hashRegex := regexp.MustCompile(`/[a-fA-F0-9]{32,}`)
	path = hashRegex.ReplaceAllString(path, "/:hash")

	path, _ = strings.CutSuffix(path, "/")

	return path
}
