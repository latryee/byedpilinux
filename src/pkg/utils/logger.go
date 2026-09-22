package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	mu        sync.Mutex
	level     LogLevel
	writer    io.Writer
	useColors bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func GetLogger() *Logger {
	once.Do(func() {
		// Determine color support
		isTerm := os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
		defaultLogger = &Logger{
			level:     LevelInfo,
			writer:    os.Stderr,
			useColors: isTerm,
		}
	})
	return defaultLogger
}

func (l *Logger) SetLevel(levelStr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch strings.ToLower(strings.TrimSpace(levelStr)) {
	case "debug":
		l.level = LevelDebug
	case "info":
		l.level = LevelInfo
	case "warn", "warning":
		l.level = LevelWarn
	case "error":
		l.level = LevelError
	}
}

// sanitize prevents logging sensitive patterns like tokens or authorization headers
func sanitize(msg string) string {
	lower := strings.ToLower(msg)
	sensitiveKeys := []string{"token", "authorization", "password", "secret", "cookie"}
	for _, key := range sensitiveKeys {
		if strings.Contains(lower, key) {
			// Redact potential values following the key
			parts := strings.Split(msg, " ")
			for i, p := range parts {
				if strings.Contains(strings.ToLower(p), key) && i+1 < len(parts) {
					parts[i+1] = "[REDACTED]"
				}
			}
			return strings.Join(parts, " ")
		}
	}
	return msg
}

func (l *Logger) log(level LogLevel, tag string, color string, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	msg = sanitize(msg)

	if l.useColors {
		fmt.Fprintf(l.writer, "%s%s%s %s[%-5s]%s %s\n", colorGray, timestamp, colorReset, color, tag, colorReset, msg)
	} else {
		fmt.Fprintf(l.writer, "%s [%-5s] %s\n", timestamp, tag, msg)
	}
}

func Debug(format string, args ...interface{}) {
	GetLogger().log(LevelDebug, "DEBUG", colorGray, format, args...)
}

func Info(format string, args ...interface{}) {
	GetLogger().log(LevelInfo, "INFO", colorGreen, format, args...)
}

func Warn(format string, args ...interface{}) {
	GetLogger().log(LevelWarn, "WARN", colorYellow, format, args...)
}

func Error(format string, args ...interface{}) {
	GetLogger().log(LevelError, "ERROR", colorRed, format, args...)
}

func Tag(tag string, color string, format string, args ...interface{}) {
	GetLogger().log(LevelInfo, tag, color, format, args...)
}
