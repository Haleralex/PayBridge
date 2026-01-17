// Package dtos определяет Data Transfer Objects для передачи данных между слоями.
//
// Почему нужны DTOs? (не использовать domain entities напрямую)
// 1. Разделение concerns: Domain entities могут меняться независимо от API
// 2. Безопасность: Не раскрываем внутренние поля (например, password hashes)
// 3. Простота: API может иметь более простое представление
// 4. Версионирование: Разные версии API могут использовать разные DTOs
//
// SOLID Principles:
// - SRP: DTO отвечает только за передачу данных
// - ISP: Разные DTOs для разных use cases (не один жирный DTO)
//
// Pattern: Data Transfer Object
package dtos

import "time"

// ============================================
// Commands (Write операции - изменяют состояние)
// ============================================

// CreateUserCommand - команда для создания пользователя.
//
// Command Pattern:
// - Инкапсулирует запрос как объект
// - Содержит все параметры для выполнения операции
// - Используется для write-операций
type CreateUserCommand struct {
	Email    string `json:"email" validate:"required,email"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

// StartKYCVerificationCommand - команда для запуска KYC верификации.
type StartKYCVerificationCommand struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

// ApproveKYCCommand - команда для одобрения KYC.
type ApproveKYCCommand struct {
	UserID   string `json:"user_id" validate:"required,uuid"`
	Verified bool   `json:"verified"`
	Reason   string `json:"reason,omitempty"` // Причина (если rejected)
}

// UpdateUserCommand - команда для обновления данных пользователя.
type UpdateUserCommand struct {
	UserID   string  `json:"user_id" validate:"required,uuid"`
	Email    *string `json:"email,omitempty" validate:"omitempty,email"`     // nil = не изменять
	FullName *string `json:"full_name,omitempty" validate:"omitempty,min=2"` // nil = не изменять
}

// ============================================
// Queries (Read операции - не изменяют состояние)
// ============================================

// GetUserQuery - запрос для получения пользователя по ID.
//
// Query Pattern:
// - Используется для read-операций
// - Не изменяет состояние
// - Может содержать параметры фильтрации/пагинации
type GetUserQuery struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

// ListUsersQuery - запрос для получения списка пользователей.
type ListUsersQuery struct {
	Offset int `json:"offset" validate:"min=0"`
	Limit  int `json:"limit" validate:"min=1,max=100"`
}

// ============================================
// Response DTOs (Результаты операций)
// ============================================

// UserDTO - представление пользователя для API.
//
// Отличия от domain entity User:
// - UUID преобразован в string (проще для JSON)
// - Нет методов (только данные)
// - Может содержать вычисляемые поля
// - Не раскрывает внутренние детали
type UserDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	KYCStatus string    `json:"kyc_status"` // "UNVERIFIED", "PENDING", "VERIFIED", "REJECTED"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserListDTO - результат для списка пользователей.
type UserListDTO struct {
	Users      []UserDTO `json:"users"`
	TotalCount int       `json:"total_count"` // Общее количество (для пагинации)
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// UserCreatedDTO - результат создания пользователя.
// Может содержать дополнительные поля, специфичные для операции создания.
type UserCreatedDTO struct {
	User    UserDTO `json:"user"`
	Message string  `json:"message,omitempty"` // Например: "Please verify your email"
}
