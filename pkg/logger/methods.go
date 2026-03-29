package logger

import (
	"fmt"
	"os"
	"time"
)

// log записывает сообщение в лог
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var msg string
	if format == "" {
		msg = fmt.Sprint(args...)
	} else {
		msg = fmt.Sprintf(format, args...)
	}

	timestamp := ""
	if l.timestamp {
		timestamp = time.Now().Format("2006-01-02 15:04:05") + " "
	}

	levelName := levelNames[level]
	if l.color {
		levelName = l.colorizeLevel(level, levelName)
	}

	prefix := ""
	if l.prefix != "" {
		prefix = l.prefix
	}

	logEntry := fmt.Sprintf("%s%s %s: %s\n", timestamp, prefix, levelName, msg)
	l.out.Write([]byte(logEntry))

	if level == FATAL {
		os.Exit(1)
	}
}

// colorizeLevel добавляет цвет к уровню логирования
func (l *Logger) colorizeLevel(level Level, levelName string) string {
	if !l.color {
		return levelName
	}

	colors := map[Level]string{
		TRACE: "\033[37m", // Grey
		DEBUG: "\033[36m", // Cyan
		INFO:  "\033[32m", // Green
		WARN:  "\033[33m", // Yellow
		ERROR: "\033[31m", // Red
		FATAL: "\033[35m", // Magenta
	}

	reset := "\033[0m"
	if color, ok := colors[level]; ok {
		return color + levelName + reset
	}

	return levelName
}

// Методы для каждого уровня логирования
func (l *Logger) Trace(args ...interface{}) {
	l.log(TRACE, "", args...)
}

func (l *Logger) Tracef(format string, args ...interface{}) {
	l.log(TRACE, format, args...)
}

// Методы для каждого уровня логирования
func (l *Logger) Debug(args ...interface{}) {
	l.log(DEBUG, "", args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.log(INFO, "", args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.log(WARN, "", args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.log(ERROR, "", args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.log(FATAL, "", args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}
