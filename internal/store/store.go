package store

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db           *sql.DB
	Transactions TransactionsStorer
	Splits       SplitsStorer
	Accounts     AccountsStorer
}

func NewStore(db *sql.DB) Store {
	return Store{
		db:           db,
		Transactions: TransactionsStore{db: db},
		Splits:       SplitsStore{db: db},
		Accounts:     AccountsStore{db: db},
	}
}

func (s *Store) WithTx(tx *sql.Tx) *Store {
	return &Store{
		db:           s.db,
		Transactions: TransactionsStore{db: tx},
		Splits:       SplitsStore{db: tx},
		Accounts:     AccountsStore{db: tx},
	}
}

func (s *Store) ExecTx(ctx context.Context, fn func(*Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txStore := s.WithTx(tx)
	if err := fn(txStore); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

type orderField struct {
	field      string
	descending bool
}

type rowScanner interface {
	Scan(dest ...any) error
}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
)
