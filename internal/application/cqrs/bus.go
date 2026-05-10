package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// CommandBus dispatches commands to their registered handlers through a middleware pipeline.
type CommandBus struct {
	mu         sync.RWMutex
	handlers   map[string]HandlerFunc
	middleware []Middleware
}

// NewCommandBus creates a new CommandBus with the given middleware.
func NewCommandBus(mw ...Middleware) *CommandBus {
	return &CommandBus{
		handlers:   make(map[string]HandlerFunc),
		middleware: mw,
	}
}

// register stores a handler for the given command type name.
func (b *CommandBus) register(name string, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = handler
}

// dispatch finds and invokes the handler for the given command type name.
func (b *CommandBus) dispatch(ctx context.Context, name string, cmd any) (any, error) {
	b.mu.RLock()
	handler, ok := b.handlers[name]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cqrs: no command handler registered for %q", name)
	}

	// Build middleware chain
	final := handler
	for i := len(b.middleware) - 1; i >= 0; i-- {
		final = b.middleware[i](final)
	}

	return final(ctx, cmd)
}

// QueryBus dispatches queries to their registered handlers through a middleware pipeline.
type QueryBus struct {
	mu         sync.RWMutex
	handlers   map[string]HandlerFunc
	middleware []Middleware
}

// NewQueryBus creates a new QueryBus with the given middleware.
func NewQueryBus(mw ...Middleware) *QueryBus {
	return &QueryBus{
		handlers:   make(map[string]HandlerFunc),
		middleware: mw,
	}
}

// register stores a handler for the given query type name.
func (b *QueryBus) register(name string, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = handler
}

// dispatch finds and invokes the handler for the given query type name.
func (b *QueryBus) dispatch(ctx context.Context, name string, query any) (any, error) {
	b.mu.RLock()
	handler, ok := b.handlers[name]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cqrs: no query handler registered for %q", name)
	}

	// Build middleware chain
	final := handler
	for i := len(b.middleware) - 1; i >= 0; i-- {
		final = b.middleware[i](final)
	}

	return final(ctx, query)
}

// typeName returns the short type name used as a bus key.
// For struct types: "CreateUserCommand"
// For pointer types: "*UserCreatedDTO" → "UserCreatedDTO"
func typeName[T any]() string {
	t := reflect.TypeOf((*T)(nil)).Elem()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// RegisterCommand registers a typed CommandHandler on the CommandBus.
// The handler is keyed by the command type name (e.g., "CreateUserCommand").
func RegisterCommand[C any, R any](bus *CommandBus, handler CommandHandler[C, R]) {
	name := typeName[C]()
	bus.register(name, func(ctx context.Context, request any) (any, error) {
		cmd, ok := request.(C)
		if !ok {
			return nil, fmt.Errorf("cqrs: invalid command type: expected %s, got %T", name, request)
		}
		return handler.Handle(ctx, cmd)
	})
}

// RegisterQuery registers a typed QueryHandler on the QueryBus.
// The handler is keyed by the query type name (e.g., "GetUserQuery").
func RegisterQuery[Q any, R any](bus *QueryBus, handler QueryHandler[Q, R]) {
	name := typeName[Q]()
	bus.register(name, func(ctx context.Context, request any) (any, error) {
		query, ok := request.(Q)
		if !ok {
			return nil, fmt.Errorf("cqrs: invalid query type: expected %s, got %T", name, request)
		}
		return handler.Handle(ctx, query)
	})
}

// DispatchCommand dispatches a typed command through the CommandBus.
// It resolves the handler by command type name and returns a typed result.
func DispatchCommand[C any, R any](bus *CommandBus, ctx context.Context, cmd C) (R, error) {
	name := typeName[C]()
	result, err := bus.dispatch(ctx, name, cmd)
	if err != nil {
		var zero R
		return zero, err
	}
	typed, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("cqrs: unexpected result type: expected %s, got %T", typeName[R](), result)
	}
	return typed, nil
}

// DispatchQuery dispatches a typed query through the QueryBus.
// It resolves the handler by query type name and returns a typed result.
func DispatchQuery[Q any, R any](bus *QueryBus, ctx context.Context, query Q) (R, error) {
	name := typeName[Q]()
	result, err := bus.dispatch(ctx, name, query)
	if err != nil {
		var zero R
		return zero, err
	}
	typed, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("cqrs: unexpected result type: expected %s, got %T", typeName[R](), result)
	}
	return typed, nil
}
