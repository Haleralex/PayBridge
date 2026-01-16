// Package middleware - Authentication middleware.
//
// Это базовая реализация auth middleware.
// В production следует использовать JWT/OAuth2/API Keys.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// AuthUserIDKey - ключ для хранения User ID в контексте
	AuthUserIDKey = "auth_user_id"
	// AuthUserEmailKey - ключ для хранения email пользователя
	AuthUserEmailKey = "auth_user_email"
	// AuthUserRoleKey - ключ для хранения роли пользователя
	AuthUserRoleKey = "auth_user_role"
)

// AuthConfig - конфигурация для authentication middleware.
type AuthConfig struct {
	// TokenValidator - функция для валидации токена
	// В production здесь будет JWT validator или вызов auth service
	TokenValidator func(token string) (*AuthClaims, error)
	// SkipPaths - пути, которые не требуют авторизации
	SkipPaths []string
}

// AuthClaims - данные из токена авторизации.
//
// Pattern: Claims object (как в JWT)
type AuthClaims struct {
	UserID string
	Email  string
	Role   string
	Exp    time.Time
}

// Auth middleware для проверки авторизации.
//
// Схема работы:
// 1. Извлекает токен из заголовка Authorization
// 2. Валидирует токен через TokenValidator
// 3. Добавляет данные пользователя в контекст
// 4. Продолжает обработку или возвращает 401
//
// Pattern: Bearer Token Authentication
func Auth(config *AuthConfig) gin.HandlerFunc {
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

		// Извлекаем токен из заголовка
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithUnauthorized(c, "Authorization header is required")
			return
		}

		// Проверяем формат "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			abortWithUnauthorized(c, "Invalid authorization header format")
			return
		}

		token := parts[1]
		if token == "" {
			abortWithUnauthorized(c, "Token is required")
			return
		}

		// Валидируем токен
		claims, err := config.TokenValidator(token)
		if err != nil {
			abortWithUnauthorized(c, "Invalid or expired token")
			return
		}

		// Проверяем expiration
		if claims.Exp.Before(time.Now()) {
			abortWithUnauthorized(c, "Token has expired")
			return
		}

		// Сохраняем claims в контекст
		c.Set(AuthUserIDKey, claims.UserID)
		c.Set(AuthUserEmailKey, claims.Email)
		c.Set(AuthUserRoleKey, claims.Role)

		c.Next()
	}
}

// abortWithUnauthorized отправляет 401 ответ.
func abortWithUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"error": gin.H{
			"code":    "UNAUTHORIZED",
			"message": message,
		},
		"request_id": GetRequestID(c),
		"timestamp":  time.Now().UTC(),
	})
}

// RequireRole middleware проверяет роль пользователя.
//
// Используется после Auth middleware для проверки разрешений.
func RequireRole(roles ...string) gin.HandlerFunc {
	roleMap := make(map[string]bool)
	for _, role := range roles {
		roleMap[role] = true
	}

	return func(c *gin.Context) {
		userRole := GetAuthUserRole(c)
		if userRole == "" {
			abortWithForbidden(c, "User role not found")
			return
		}

		if !roleMap[userRole] {
			abortWithForbidden(c, "Insufficient permissions")
			return
		}

		c.Next()
	}
}

// abortWithForbidden отправляет 403 ответ.
func abortWithForbidden(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"error": gin.H{
			"code":    "FORBIDDEN",
			"message": message,
		},
		"request_id": GetRequestID(c),
		"timestamp":  time.Now().UTC(),
	})
}

// ============================================
// Helper functions для извлечения auth данных
// ============================================

// GetAuthUserID возвращает ID авторизованного пользователя.
func GetAuthUserID(c *gin.Context) uuid.UUID {
	if id, exists := c.Get(AuthUserIDKey); exists {
		if strID, ok := id.(string); ok {
			if uid, err := uuid.Parse(strID); err == nil {
				return uid
			}
		}
	}
	return uuid.Nil
}

// GetAuthUserEmail возвращает email авторизованного пользователя.
func GetAuthUserEmail(c *gin.Context) string {
	if email, exists := c.Get(AuthUserEmailKey); exists {
		if strEmail, ok := email.(string); ok {
			return strEmail
		}
	}
	return ""
}

// GetAuthUserRole возвращает роль авторизованного пользователя.
func GetAuthUserRole(c *gin.Context) string {
	if role, exists := c.Get(AuthUserRoleKey); exists {
		if strRole, ok := role.(string); ok {
			return strRole
		}
	}
	return ""
}

// ============================================
// Development/Testing Helpers
// ============================================

// MockTokenValidator - mock validator для development/testing.
//
// ВАЖНО: Использовать ТОЛЬКО для разработки!
// В production должен быть реальный JWT validator.
func MockTokenValidator(token string) (*AuthClaims, error) {
	// Для development: токен = user_id
	return &AuthClaims{
		UserID: token,
		Email:  "test@example.com",
		Role:   "user",
		Exp:    time.Now().Add(24 * time.Hour),
	}, nil
}

// AdminMockTokenValidator - mock validator для admin.
func AdminMockTokenValidator(token string) (*AuthClaims, error) {
	return &AuthClaims{
		UserID: token,
		Email:  "admin@example.com",
		Role:   "admin",
		Exp:    time.Now().Add(24 * time.Hour),
	}, nil
}
