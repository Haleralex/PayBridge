// Package http - HTTP Server configuration and lifecycle management.
//
// Server управляет жизненным циклом HTTP сервера:
// - Graceful startup
// - Graceful shutdown
// - Timeout configuration
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================
// Server Configuration
// ============================================

// ServerConfig - конфигурация HTTP сервера.
type ServerConfig struct {
	// Host для прослушивания (e.g., "0.0.0.0", "localhost")
	Host string
	// Port для прослушивания
	Port string
	// ReadTimeout - максимальное время чтения запроса
	ReadTimeout time.Duration
	// WriteTimeout - максимальное время записи ответа
	WriteTimeout time.Duration
	// IdleTimeout - максимальное время ожидания следующего запроса
	IdleTimeout time.Duration
	// ShutdownTimeout - время на graceful shutdown
	ShutdownTimeout time.Duration
	// Logger для логирования
	Logger *slog.Logger
}

// DefaultServerConfig - конфигурация по умолчанию.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:            "0.0.0.0",
		Port:            "8080",
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Logger:          slog.Default(),
	}
}

// Address возвращает адрес для прослушивания.
func (c *ServerConfig) Address() string {
	return c.Host + ":" + c.Port
}

// ============================================
// Server
// ============================================

// Server - HTTP сервер с graceful shutdown.
type Server struct {
	config     *ServerConfig
	httpServer *http.Server
	router     *gin.Engine
}

// NewServer создаёт новый HTTP сервер.
func NewServer(config *ServerConfig, router *gin.Engine) *Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	httpServer := &http.Server{
		Addr:         config.Address(),
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return &Server{
		config:     config,
		httpServer: httpServer,
		router:     router,
	}
}

// Start запускает сервер.
func (s *Server) Start() error {
	s.config.Logger.Info("Starting HTTP server",
		slog.String("address", s.config.Address()),
	)

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// StartTLS запускает HTTPS сервер.
func (s *Server) StartTLS(certFile, keyFile string) error {
	s.config.Logger.Info("Starting HTTPS server",
		slog.String("address", s.config.Address()),
	)

	if err := s.httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Shutdown выполняет graceful shutdown сервера.
func (s *Server) Shutdown(ctx context.Context) error {
	s.config.Logger.Info("Shutting down HTTP server...")

	// Создаём контекст с таймаутом для shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.config.Logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
		return err
	}

	s.config.Logger.Info("HTTP server stopped gracefully")
	return nil
}

// ============================================
// Run with Graceful Shutdown
// ============================================

// Run запускает сервер с обработкой сигналов для graceful shutdown.
//
// Сигналы для остановки:
// - SIGINT (Ctrl+C)
// - SIGTERM (kill)
//
// При получении сигнала:
// 1. Прекращает приём новых соединений
// 2. Дожидается завершения активных запросов
// 3. Завершает работу
func (s *Server) Run() error {
	// Канал для ошибок сервера
	errChan := make(chan error, 1)

	// Запускаем сервер в горутине
	go func() {
		if err := s.Start(); err != nil {
			errChan <- err
		}
	}()

	// Канал для сигналов ОС
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ждём либо ошибку, либо сигнал остановки
	select {
	case err := <-errChan:
		return err
	case sig := <-quit:
		s.config.Logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
	}

	// Graceful shutdown
	ctx := context.Background()
	return s.Shutdown(ctx)
}

// RunWithContext запускает сервер с возможностью отмены через контекст.
//
// Удобно для тестирования и программного управления.
func (s *Server) RunWithContext(ctx context.Context) error {
	// Канал для ошибок сервера
	errChan := make(chan error, 1)

	// Запускаем сервер в горутине
	go func() {
		if err := s.Start(); err != nil {
			errChan <- err
		}
	}()

	// Ждём либо ошибку, либо отмену контекста
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.config.Logger.Info("Context cancelled, initiating shutdown")
	}

	// Graceful shutdown
	shutdownCtx := context.Background()
	return s.Shutdown(shutdownCtx)
}

// ============================================
// Helper Functions
// ============================================

// QuickStart - быстрый запуск сервера с минимальной конфигурацией.
//
// Использование:
//
//	router := http.NewDevelopmentRouter()
//	http.QuickStart(router, ":8080")
func QuickStart(router *gin.Engine, addr string) error {
	host, port := parseAddress(addr)
	config := &ServerConfig{
		Host:            host,
		Port:            port,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Logger:          slog.Default(),
	}

	server := NewServer(config, router)
	return server.Run()
}

// parseAddress разбирает адрес на host и port.
func parseAddress(addr string) (host, port string) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host = addr[:i]
			port = addr[i+1:]
			return
		}
	}
	return "", addr
}
