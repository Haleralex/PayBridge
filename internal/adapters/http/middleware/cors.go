// Package middleware - CORS middleware.
//
// Cross-Origin Resource Sharing (CORS) позволяет браузерам
// делать запросы к API с других доменов.
package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig - конфигурация CORS.
type CORSConfig struct {
	// AllowOrigins - разрешённые origins (домены)
	// "*" - разрешить все (не рекомендуется для production)
	AllowOrigins []string
	// AllowMethods - разрешённые HTTP методы
	AllowMethods []string
	// AllowHeaders - разрешённые заголовки запроса
	AllowHeaders []string
	// ExposeHeaders - заголовки, доступные клиенту
	ExposeHeaders []string
	// AllowCredentials - разрешить credentials (cookies, auth headers)
	AllowCredentials bool
	// MaxAge - время кеширования preflight запроса (секунды)
	MaxAge int
}

// DefaultCORSConfig - конфигурация по умолчанию.
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
			"X-Idempotency-Key",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
		},
		AllowCredentials: false,
		MaxAge:           86400, // 24 часа
	}
}

// ProductionCORSConfig - конфигурация для production.
func ProductionCORSConfig(allowedOrigins []string) *CORSConfig {
	config := DefaultCORSConfig()
	config.AllowOrigins = allowedOrigins
	config.AllowCredentials = true
	return config
}

// CORS middleware для обработки Cross-Origin запросов.
//
// CORS работает так:
// 1. Браузер отправляет OPTIONS preflight запрос
// 2. Сервер отвечает с разрешёнными origins/methods/headers
// 3. Браузер проверяет ответ и решает, делать ли основной запрос
//
// Заголовки:
// - Access-Control-Allow-Origin: Разрешённые домены
// - Access-Control-Allow-Methods: Разрешённые методы
// - Access-Control-Allow-Headers: Разрешённые заголовки
// - Access-Control-Expose-Headers: Заголовки, видимые клиенту
// - Access-Control-Allow-Credentials: Разрешены ли credentials
// - Access-Control-Max-Age: Время кеширования preflight
func CORS(config *CORSConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultCORSConfig()
	}

	// Предварительно формируем строки для заголовков
	allowMethods := strings.Join(config.AllowMethods, ", ")
	allowHeaders := strings.Join(config.AllowHeaders, ", ")
	exposeHeaders := strings.Join(config.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(config.MaxAge)

	// Создаём map для быстрой проверки origins
	allowAllOrigins := len(config.AllowOrigins) == 1 && config.AllowOrigins[0] == "*"
	originsMap := make(map[string]bool)
	if !allowAllOrigins {
		for _, origin := range config.AllowOrigins {
			originsMap[origin] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Определяем, разрешён ли origin
		var allowedOrigin string
		if allowAllOrigins {
			allowedOrigin = "*"
		} else if originsMap[origin] {
			allowedOrigin = origin
		}

		// Если origin не разрешён - пропускаем CORS headers
		if allowedOrigin == "" && origin != "" {
			c.Next()
			return
		}

		// Устанавливаем CORS headers
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		c.Header("Access-Control-Expose-Headers", exposeHeaders)
		c.Header("Access-Control-Max-Age", maxAge)

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Обрабатываем preflight запрос
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
