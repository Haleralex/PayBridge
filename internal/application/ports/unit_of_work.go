// Package ports - UnitOfWork паттерн для управления транзакциями.
//
// SOLID Principles:
// - SRP: UnitOfWork отвечает только за границы транзакций
// - DIP: Application не знает о деталях БД транзакций
//
// Pattern: Unit of Work
// - Обеспечивает атомарность операций
// - Один UnitOfWork = одна БД-транзакция
// - Автоматический rollback при ошибке
package ports

import "context"

// UnitOfWork определяет контракт для управления транзакциями.
//
// Паттерн Unit of Work решает проблему:
// "Как гарантировать, что несколько операций выполнятся атомарно?"
//
// Пример использования:
//   err := uow.Execute(ctx, func(ctx context.Context) error {
//       user, _ := userRepo.FindByID(ctx, userID)
//       wallet, _ := walletRepo.Create(ctx, user.ID(), currency)
//       return eventPublisher.Publish(ctx, WalletCreated{...})
//   })
//   // Если любая операция вернёт error - automatic rollback
//   // Если все успешны - automatic commit
type UnitOfWork interface {
	// Execute выполняет функцию внутри транзакции.
	//
	// Поведение:
	// - Начинает транзакцию
	// - Выполняет fn
	// - Если fn возвращает error: ROLLBACK
	// - Если fn возвращает nil: COMMIT
	//
	// Context:
	// Переданный context содержит транзакцию.
	// Все операции внутри fn должны использовать этот context!
	//
	// Example:
	//   uow.Execute(ctx, func(txCtx context.Context) error {
	//       // Используем txCtx, не ctx!
	//       wallet, err := walletRepo.FindByID(txCtx, walletID)
	//       if err != nil {
	//           return err // Автоматический rollback
	//       }
	//
	//       wallet.Credit(amount)
	//       return walletRepo.Save(txCtx, wallet)
	//   })
	Execute(ctx context.Context, fn func(context.Context) error) error

	// ExecuteWithResult аналогичен Execute, но возвращает результат.
	// Полезно когда нужно вернуть созданную entity.
	//
	// Example:
	//   wallet, err := uow.ExecuteWithResult(ctx, func(txCtx context.Context) (*entities.Wallet, error) {
	//       wallet := entities.NewWallet(userID, currency)
	//       err := walletRepo.Save(txCtx, wallet)
	//       return wallet, err
	//   })
	ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error)
}

// UnitOfWorkFactory создаёт экземпляры UnitOfWork.
// Нужен когда требуется вложенные транзакции или сохранения (savepoints).
//
// В большинстве случаев достаточно одного UnitOfWork на приложение,
// но фабрика позволяет создавать изолированные транзакции.
type UnitOfWorkFactory interface {
	// New создаёт новый UnitOfWork.
	New() UnitOfWork
}
