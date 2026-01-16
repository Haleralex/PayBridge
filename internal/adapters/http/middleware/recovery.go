// Package middleware - Recovery middleware для обработки паник.
package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// RecoveryConfig - конфигурация для recovery middleware.
type RecoveryConfig struct {
	Logger           *slog.Logger
	EnableStackTrace bool // Включать stack trace в логи
	PrintStack       bool // Выводить stack trace в консоль
}

// DefaultRecoveryConfig - конфигурация по умолчанию.
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		Logger:           slog.Default(),
		EnableStackTrace: true,
		PrintStack:       false,
	}
}

// Recovery middleware перехватывает панику и возвращает 500 ошибку.
//
// Зачем нужен Recovery:
// 1. Не даёт приложению упасть при панике в handler
// 2. Логирует stack trace для debugging
// 3. Возвращает клиенту понятную ошибку
//
// Pattern: Graceful Error Handling
func Recovery(config *RecoveryConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Получаем stack trace
				stack := debug.Stack()

				// Логируем ошибку
				attrs := []slog.Attr{
					slog.String("error", fmt.Sprintf("%v", err)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("request_id", GetRequestID(c)),
					slog.String("client_ip", c.ClientIP()),
				}

				if config.EnableStackTrace {
					attrs = append(attrs, slog.String("stack", string(stack)))
				}

				config.Logger.LogAttrs(c.Request.Context(), slog.LevelError, "Panic recovered", attrs...)

				// Выводим в консоль если включено
				if config.PrintStack {
					fmt.Printf("[Recovery] panic recovered:\n%v\n%s\n", err, stack)
				}

				// Формируем response
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
					"request_id": GetRequestID(c),
					"timestamp":  time.Now().UTC(),
				})
			}
		}()

		c.Next()
	}
}
