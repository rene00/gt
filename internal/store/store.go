package store

import "database/sql"

type Store struct {
	Transactions TransactionsStorer
	Splits       SplitsStorer
	Accounts     AccountsStorer
}

func NewStore(db *sql.DB) Store {
	return Store{
		Transactions: TransactionsStore{db: db},
		Splits:       SplitsStore{db: db},
		Accounts:     AccountsStore{db: db},
	}
}

type orderField struct {
	field      string
	descending bool
}

type rowScanner interface {
	Scan(dest ...any) error
}
