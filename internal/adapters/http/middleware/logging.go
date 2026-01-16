// Package middleware - Logging middleware для структурированного логирования.
package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingConfig - конфигурация для logging middleware.
type LoggingConfig struct {
	Logger          *slog.Logger
	SkipPaths       []string // Пути для пропуска логирования (e.g., /health)
	LogRequestBody  bool     // Логировать тело запроса (осторожно с PII!)
	LogResponseBody bool     // Логировать тело ответа
	MaxBodySize     int      // Максимальный размер тела для логирования
}

// DefaultLoggingConfig - конфигурация по умолчанию.
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Logger:          slog.Default(),
		SkipPaths:       []string{"/health", "/ready", "/metrics"},
		LogRequestBody:  false,
		LogResponseBody: false,
		MaxBodySize:     1024, // 1KB
	}
}

// Logging middleware для структурированного логирования HTTP запросов.
//
// Логируемые данные:
// - HTTP метод и путь
// - Статус код ответа
// - Время обработки
// - Request ID
// - IP клиента
// - User-Agent
// - Размер ответа
//
// Pattern: Structured Logging
func Logging(config *LoggingConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultLoggingConfig()
	}

	// Создаём map для быстрой проверки skip paths
	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		// Пропускаем определённые пути
		if skipMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Запоминаем время начала
		start := time.Now()

		// Читаем request body если нужно
		var requestBody string
		if config.LogRequestBody {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			if len(bodyBytes) > 0 {
				requestBody = truncateString(string(bodyBytes), config.MaxBodySize)
			}
		}

		// Используем response writer для захвата response body
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		if config.LogResponseBody {
			c.Writer = blw
		}

		// Вызываем следующий handler
		c.Next()

		// Вычисляем duration
		duration := time.Since(start)

		// Собираем атрибуты для лога
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("query", c.Request.URL.RawQuery),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", duration),
			slog.String("request_id", GetRequestID(c)),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("response_size", c.Writer.Size()),
		}

		// Добавляем request body если логируем
		if config.LogRequestBody && requestBody != "" {
			attrs = append(attrs, slog.String("request_body", requestBody))
		}

		// Добавляем response body если логируем
		if config.LogResponseBody && blw.body.Len() > 0 {
			attrs = append(attrs, slog.String("response_body",
				truncateString(blw.body.String(), config.MaxBodySize)))
		}

		// Добавляем ошибки если есть
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		// Определяем уровень логирования по статусу
		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}

		// Логируем
		config.Logger.LogAttrs(c.Request.Context(), level, "HTTP Request", attrs...)
	}
}

// bodyLogWriter - ResponseWriter с захватом body.
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write записывает в оригинальный writer и буфер.
func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// truncateString обрезает строку до максимальной длины.
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
