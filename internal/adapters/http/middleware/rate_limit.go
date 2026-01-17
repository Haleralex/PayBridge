// Package middleware - Rate Limiting middleware.
//
// Защита от DDoS и abuse через ограничение количества запросов.
// Использует Token Bucket алгоритм с in-memory хранением.
//
// Для production рекомендуется использовать Redis для distributed rate limiting.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitConfig - конфигурация для rate limiting.
type RateLimitConfig struct {
	// Requests per window
	Limit int
	// Time window
	Window time.Duration
	// KeyFunc - функция для определения ключа лимитирования
	// По умолчанию - IP адрес
	KeyFunc func(*gin.Context) string
	// OnLimitReached - callback при достижении лимита
	OnLimitReached func(*gin.Context)
}

// DefaultRateLimitConfig - конфигурация по умолчанию.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Limit:  100,         // 100 запросов
		Window: time.Minute, // в минуту
		KeyFunc: func(c *gin.Context) string { // по IP
			return c.ClientIP()
		},
		OnLimitReached: nil,
	}
}

// rateLimiter хранит состояние rate limiter.
type rateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	config  *RateLimitConfig
}

// bucket - корзина токенов для одного ключа.
type bucket struct {
	tokens    int
	lastReset time.Time
}

// newRateLimiter создаёт новый rate limiter.
func newRateLimiter(config *RateLimitConfig) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		config:  config,
	}

	// Запускаем cleanup goroutine
	go rl.cleanup()

	return rl
}

// allow проверяет, разрешён ли запрос.
func (rl *rateLimiter) allow(key string) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]

	if !exists {
		// Создаём новую корзину
		rl.buckets[key] = &bucket{
			tokens:    rl.config.Limit - 1, // -1 за текущий запрос
			lastReset: now,
		}
		return true, rl.config.Limit - 1, rl.config.Window
	}

	// Проверяем, нужно ли сбросить корзину
	if now.Sub(b.lastReset) >= rl.config.Window {
		b.tokens = rl.config.Limit - 1
		b.lastReset = now
		return true, b.tokens, rl.config.Window
	}

	// Проверяем доступные токены
	if b.tokens <= 0 {
		retryAfter := rl.config.Window - now.Sub(b.lastReset)
		return false, 0, retryAfter
	}

	b.tokens--
	retryAfter := rl.config.Window - now.Sub(b.lastReset)
	return true, b.tokens, retryAfter
}

// cleanup удаляет устаревшие записи.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.Window * 2)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.buckets {
			if now.Sub(b.lastReset) > rl.config.Window*2 {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit middleware для ограничения количества запросов.
//
// Алгоритм: Fixed Window Counter
// - Каждый IP/ключ имеет лимит запросов за временное окно
// - При достижении лимита возвращается 429 Too Many Requests
// - Добавляет заголовки X-RateLimit-* для клиента
//
// Headers:
// - X-RateLimit-Limit: Максимум запросов
// - X-RateLimit-Remaining: Оставшееся количество
// - X-RateLimit-Reset: Время сброса (Unix timestamp)
// - Retry-After: Секунд до сброса (при 429)
func RateLimit(config *RateLimitConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	limiter := newRateLimiter(config)

	return func(c *gin.Context) {
		key := config.KeyFunc(c)
		allowed, remaining, retryAfter := limiter.allow(key)

		// Добавляем rate limit headers
		c.Header("X-RateLimit-Limit", itoa(config.Limit))
		c.Header("X-RateLimit-Remaining", itoa(remaining))
		c.Header("X-RateLimit-Reset", itoa(int(time.Now().Add(retryAfter).Unix())))

		if !allowed {
			// Добавляем Retry-After header
			retrySeconds := int(retryAfter.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", itoa(retrySeconds))

			// Вызываем callback если есть
			if config.OnLimitReached != nil {
				config.OnLimitReached(c)
			}

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":        "TOO_MANY_REQUESTS",
					"message":     "Rate limit exceeded, please try again later",
					"retry_after": retrySeconds,
				},
				"request_id": GetRequestID(c),
				"timestamp":  time.Now().UTC(),
			})
			return
		}

		c.Next()
	}
}

// itoa простой int -> string конвертер.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := i < 0
	if neg {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// ============================================
// Endpoint-specific rate limiters
// ============================================

// SensitiveEndpointRateLimit - более строгий лимит для sensitive endpoints.
func SensitiveEndpointRateLimit() gin.HandlerFunc {
	return RateLimit(&RateLimitConfig{
		Limit:  10,          // 10 запросов
		Window: time.Minute, // в минуту
		KeyFunc: func(c *gin.Context) string {
			// Комбинируем IP + endpoint
			return c.ClientIP() + ":" + c.Request.URL.Path
		},
	})
}

// TransactionRateLimit - лимит для финансовых операций.
func TransactionRateLimit() gin.HandlerFunc {
	return RateLimit(&RateLimitConfig{
		Limit:  30,          // 30 транзакций
		Window: time.Minute, // в минуту
		KeyFunc: func(c *gin.Context) string {
			// По user ID если авторизован, иначе по IP
			userID := GetAuthUserID(c)
			if userID.String() != "00000000-0000-0000-0000-000000000000" {
				return "user:" + userID.String()
			}
			return "ip:" + c.ClientIP()
		},
	})
}
