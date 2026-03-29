package logger

import "io"

type LoggerConfig struct {
	Level     string `yaml:"level"`
	Prefix    string `yaml:"prefix"`
	Color     bool   `yaml:"color"`
	Timestamp bool   `yaml:"timestamp"`
}

// SetLevel устанавливает уровень логирования
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput устанавливает вывод для логгера
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}
