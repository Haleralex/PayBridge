// Package middleware содержит HTTP middleware для обработки запросов.
//
// Middleware в Gin - это функции, которые выполняются до/после handlers.
// Они используются для cross-cutting concerns: логирование, auth, tracing.
//
// SOLID Principles:
// - SRP: Каждый middleware отвечает за одну задачу
// - OCP: Новые middleware добавляются без изменения существующих
//
// Pattern: Chain of Responsibility
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader - имя заголовка для Request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDContextKey - ключ для хранения Request ID в контексте
	RequestIDContextKey = "request_id"
)

// RequestID middleware добавляет уникальный ID к каждому запросу.
//
// Зачем нужен Request ID:
// 1. Трассировка: Связывание логов одного запроса
// 2. Debugging: Поиск проблем по ID
// 3. Client tracking: Клиент может использовать свой ID
//
// Если клиент передаёт X-Request-ID - используем его,
// иначе генерируем новый UUID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем ID из заголовка или генерируем новый
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Сохраняем в контекст
		c.Set(RequestIDContextKey, requestID)

		// Добавляем в response headers
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID извлекает Request ID из контекста Gin.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDContextKey); exists {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}
