// Package handlers - Health check handlers.
//
// Health checks позволяют оркестраторам (Kubernetes, Docker Swarm)
// проверять состояние приложения.
//
// Два типа health checks:
// - Liveness: Приложение работает? (если нет - restart)
// - Readiness: Приложение готово принимать трафик? (если нет - no traffic)
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Health Check Handler
// ============================================

// HealthHandler обрабатывает health check запросы.
type HealthHandler struct {
	pool      *pgxpool.Pool
	version   string
	buildTime string
	startTime time.Time
}

// NewHealthHandler создаёт новый HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, version, buildTime string) *HealthHandler {
	return &HealthHandler{
		pool:      pool,
		version:   version,
		buildTime: buildTime,
		startTime: time.Now(),
	}
}

// ============================================
// Response Types
// ============================================

// HealthResponse - ответ health check.
type HealthResponse struct {
	Status    string            `json:"status"`           // "healthy", "unhealthy", "degraded"
	Version   string            `json:"version"`          // Версия приложения
	BuildTime string            `json:"build_time"`       // Время сборки
	Uptime    string            `json:"uptime"`           // Время работы
	Timestamp time.Time         `json:"timestamp"`        // Текущее время
	Checks    map[string]string `json:"checks,omitempty"` // Детали проверок
}

// ReadinessResponse - ответ readiness check.
type ReadinessResponse struct {
	Ready     bool              `json:"ready"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// ============================================
// HTTP Handlers
// ============================================

// Health возвращает базовый health статус.
//
// @Summary Health check
// @Description Basic health check endpoint (liveness probe)
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	uptime := time.Since(h.startTime).Round(time.Second).String()

	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		BuildTime: h.buildTime,
		Uptime:    uptime,
		Timestamp: time.Now().UTC(),
	})
}

// Ready проверяет готовность приложения.
//
// @Summary Readiness check
// @Description Readiness probe - checks all dependencies
// @Tags Health
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := make(map[string]string)
	allReady := true

	// Проверяем PostgreSQL
	if h.pool != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.pool.Ping(ctx); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
			allReady = false
		} else {
			checks["database"] = "healthy"
		}
	} else {
		checks["database"] = "not configured"
	}

	// Здесь можно добавить проверки других зависимостей:
	// - Redis
	// - Message Queue
	// - External APIs

	statusCode := http.StatusOK
	if !allReady {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, ReadinessResponse{
		Ready:     allReady,
		Checks:    checks,
		Timestamp: time.Now().UTC(),
	})
}

// Live возвращает статус "живости" приложения.
//
// @Summary Liveness check
// @Description Simple liveness probe
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// DetailedHealth возвращает детальную информацию о состоянии.
//
// @Summary Detailed health check
// @Description Detailed health information including system metrics
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health/detailed [get]
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
	checks := make(map[string]string)

	// Проверяем PostgreSQL
	if h.pool != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.pool.Ping(ctx); err != nil {
			checks["database"] = "unhealthy"
		} else {
			// Добавляем статистику пула соединений
			stats := h.pool.Stat()
			checks["database"] = "healthy"
			checks["db_total_conns"] = strconv.Itoa(int(stats.TotalConns()))
			checks["db_idle_conns"] = strconv.Itoa(int(stats.IdleConns()))
			checks["db_acquired_conns"] = strconv.Itoa(int(stats.AcquiredConns()))

			// Update Prometheus metrics
			middleware.UpdateDBConnections(stats.IdleConns(), stats.AcquiredConns(), stats.MaxConns())
		}
	}

	status := "healthy"
	for _, v := range checks {
		if v == "unhealthy" {
			status = "unhealthy"
			break
		}
	}

	uptime := time.Since(h.startTime).Round(time.Second).String()

	c.JSON(http.StatusOK, HealthResponse{
		Status:    status,
		Version:   h.version,
		BuildTime: h.buildTime,
		Uptime:    uptime,
		Timestamp: time.Now().UTC(),
		Checks:    checks,
	})
}

// RegisterRoutes регистрирует health check маршруты.
//
// Routes:
// - GET /health          - Basic health check
// - GET /health/detailed - Detailed health with metrics
// - GET /ready           - Readiness probe
// - GET /live            - Liveness probe
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.Health)
	router.GET("/health/detailed", h.DetailedHealth)
	router.GET("/ready", h.Ready)
	router.GET("/live", h.Live)
}
