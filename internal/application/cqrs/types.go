// Package cqrs implements the Command/Query Responsibility Segregation pattern.
//
// CQRS separates read and write operations into distinct models:
// - Commands: change state (create, update, delete) — dispatched via CommandBus
// - Queries: read state (get, list, search) — dispatched via QueryBus
//
// Each bus supports a middleware pipeline for cross-cutting concerns
// (logging, tracing, recovery) without polluting business logic.
package cqrs

import "context"

// CommandHandler handles a command of type C and returns result of type R.
// Commands represent intentions to change state.
//
// Example:
//
//	type CreateUserHandler struct { ... }
//	func (h *CreateUserHandler) Handle(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error)
type CommandHandler[C any, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// QueryHandler handles a query of type Q and returns result of type R.
// Queries represent read-only data retrieval operations.
//
// Example:
//
//	type GetUserHandler struct { ... }
//	func (h *GetUserHandler) Handle(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error)
type QueryHandler[Q any, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}

// HandlerFunc is the internal untyped handler function used by the bus.
// Generic Register/Dispatch functions wrap typed handlers into this form.
type HandlerFunc func(ctx context.Context, request any) (any, error)

// Middleware wraps a HandlerFunc with additional behavior.
// Middleware is applied in order: first registered = outermost wrapper.
type Middleware func(next HandlerFunc) HandlerFunc
