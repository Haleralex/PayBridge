// Package postgres реализует persistence layer с использованием PostgreSQL.
//
// SOLID Principles:
// - SRP: Каждый файл отвечает за одну сущность
// - DIP: Реализует интерфейсы из ports (не зависит от application layer)
// - OCP: Новые методы добавляются без изменения существующих
//
// Patterns:
// - Repository Pattern: Абстракция доступа к данным
// - Unit of Work: Управление транзакциями
// - Connection Pool: Эффективное использование соединений
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config содержит настройки подключения к PostgreSQL.
type Config struct {
	Host            string        // Хост БД (e.g., "localhost")
	Port            int           // Порт БД (e.g., 5432)
	Database        string        // Имя базы данных
	User            string        // Пользователь
	Password        string        // Пароль
	SSLMode         string        // SSL mode (disable, require, verify-full)
	MaxConns        int32         // Максимум соединений в пуле
	MinConns        int32         // Минимум соединений в пуле
	MaxConnLifetime time.Duration // Максимальное время жизни соединения
	MaxConnIdleTime time.Duration // Максимальное время простоя соединения
	ConnectTimeout  time.Duration // Таймаут подключения
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		Database:        "wallethub",
		User:            "postgres",
		Password:        "postgres",
		SSLMode:         "disable",
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	}
}

// ConnectionString формирует строку подключения из конфигурации.
func (c Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=%d",
		c.Host,
		c.Port,
		c.Database,
		c.User,
		c.Password,
		c.SSLMode,
		int(c.ConnectTimeout.Seconds()),
	)
}

// NewConnectionPool создаёт пул соединений к PostgreSQL.
//
// Возвращает:
// - *pgxpool.Pool: Пул соединений (thread-safe)
// - error: Ошибка подключения
//
// Пул автоматически:
// - Управляет соединениями (создаёт/закрывает по необходимости)
// - Переиспользует соединения (connection pooling)
// - Проверяет здоровье соединений (health checks)
// - Обрабатывает reconnect при потере связи
//
// Example:
//
//	pool, err := NewConnectionPool(ctx, DefaultConfig())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Close()
func NewConnectionPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	// Парсим конфигурацию
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Настраиваем пул
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Создаём пул
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Проверяем подключение
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// HealthCheck проверяет здоровье подключения к БД.
// Используется для readiness/liveness probes в Kubernetes.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return pool.Ping(ctx)
}

// Stats возвращает статистику пула соединений.
// Полезно для мониторинга и дашбордов.
type PoolStats struct {
	TotalConns      int32 // Общее количество соединений
	IdleConns       int32 // Свободные соединения
	AcquiredConns   int32 // Используемые соединения
	MaxConns        int32 // Максимум соединений
	AcquireCount    int64 // Сколько раз запрашивали соединение
	AcquireDuration int64 // Общее время ожидания соединений (ns)
}

// GetPoolStats возвращает текущую статистику пула.
func GetPoolStats(pool *pgxpool.Pool) PoolStats {
	stat := pool.Stat()
	return PoolStats{
		TotalConns:      stat.TotalConns(),
		IdleConns:       stat.IdleConns(),
		AcquiredConns:   stat.AcquiredConns(),
		MaxConns:        stat.MaxConns(),
		AcquireCount:    stat.AcquireCount(),
		AcquireDuration: stat.AcquireDuration().Nanoseconds(),
	}
}
