package cqrs

import "context"

// UseCaseExecutor is a generic interface matching all existing use cases.
// Every use case in the project has: Execute(ctx, Input) (Output, error).
type UseCaseExecutor[In any, Out any] interface {
	Execute(ctx context.Context, input In) (Out, error)
}

// useCaseAdapter wraps an existing use case into a CommandHandler or QueryHandler.
// This allows registering use cases on the bus without modifying their code.
type useCaseAdapter[In any, Out any] struct {
	uc UseCaseExecutor[In, Out]
}

func (a *useCaseAdapter[In, Out]) Handle(ctx context.Context, input In) (Out, error) {
	return a.uc.Execute(ctx, input)
}

// RegisterCommandHandler registers an existing use case as a command handler.
func RegisterCommandHandler[In any, Out any](bus *CommandBus, uc UseCaseExecutor[In, Out]) {
	RegisterCommand[In, Out](bus, &useCaseAdapter[In, Out]{uc: uc})
}

// RegisterQueryHandler registers an existing use case as a query handler.
func RegisterQueryHandler[In any, Out any](bus *QueryBus, uc UseCaseExecutor[In, Out]) {
	RegisterQuery[In, Out](bus, &useCaseAdapter[In, Out]{uc: uc})
}
