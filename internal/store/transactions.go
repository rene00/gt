package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Transaction struct {
	GUID         string
	CurrencyGUID string
	Num          string
	PostDate     *time.Time
	EnterDate    *time.Time
	Description  *string
	Splits       []*Split
}

type TransactionQuery struct {
	whereClauses []string
	args         []any
	orderFields  []orderField
	limit        *int
	offset       *int
}

func NewTransactionQuery() *TransactionQuery {
	return &TransactionQuery{
		whereClauses: make([]string, 0),
		args:         make([]any, 0),
		orderFields:  make([]orderField, 0),
	}
}

func (q *TransactionQuery) Where(clause string, args ...any) *TransactionQuery {
	q.whereClauses = append(q.whereClauses, clause)
	q.args = append(q.args, args...)
	return q
}

func (q *TransactionQuery) OrderBy(field string, descending bool) *TransactionQuery {
	q.orderFields = append(q.orderFields, orderField{field: field, descending: descending})
	return q
}

func (q *TransactionQuery) Limit(limit int) *TransactionQuery {
	q.limit = &limit
	return q
}

func (q *TransactionQuery) Offset(offset int) *TransactionQuery {
	q.offset = &offset
	return q
}

func (q *TransactionQuery) Page(page, pageSize int) *TransactionQuery {
	offset := (page - 1) * pageSize
	return q.Limit(pageSize).Offset(offset)
}

func (q *TransactionQuery) Build() string {
	var b strings.Builder
	b.WriteString(`
SELECT
	guid,
	currency_guid,
	num,
	post_date,
	enter_date,
	description
FROM transactions
`)

	if len(q.whereClauses) > 0 {
		b.WriteString("\nWHERE ")
		b.WriteString(strings.Join(q.whereClauses, " AND "))
	}

	if len(q.orderFields) > 0 {
		b.WriteString("\nORDER BY ")
		orders := make([]string, len(q.orderFields))
		for i, field := range q.orderFields {
			direction := "ASC"
			if field.descending {
				direction = "DESC"
			}
			orders[i] = fmt.Sprintf("%s %s", field.field, direction)
		}
		b.WriteString(strings.Join(orders, ", "))
	}

	if q.limit != nil {
		b.WriteString(fmt.Sprintf("\nLIMIT %d", *q.limit))
	}

	if q.offset != nil {
		b.WriteString(fmt.Sprintf("\nOFFSET %d", *q.offset))
	}

	return b.String()
}

func (q *TransactionQuery) Copy() *TransactionQuery {
	if q == nil {
		return nil
	}

	copied := &TransactionQuery{
		whereClauses: make([]string, len(q.whereClauses)),
		args:         make([]any, len(q.args)),
		orderFields:  make([]orderField, len(q.orderFields)),
	}

	copy(copied.whereClauses, q.whereClauses)
	copy(copied.args, q.args)
	copy(copied.orderFields, q.orderFields)

	if q.limit != nil {
		limit := *q.limit
		copied.limit = &limit
	}

	if q.offset != nil {
		offset := *q.offset
		copied.offset = &offset
	}

	return copied
}

func (q *TransactionQuery) Args() []any {
	return q.args
}

type TransactionsStorer interface {
	All(ctx context.Context, q *TransactionQuery) ([]*Transaction, error)
	Get(ctx context.Context, guid string) (*Transaction, error)
}

type TransactionsStore struct {
	db *sql.DB
}

func (t TransactionsStore) Get(ctx context.Context, guid string) (*Transaction, error) {
	q := NewTransactionQuery().Where("guid=?", guid)
	row := t.db.QueryRowContext(ctx, q.Build(), q.Args()...)
	transaction, err := scanTransaction(row)
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

func (t TransactionsStore) All(ctx context.Context, q *TransactionQuery) ([]*Transaction, error) {
	sqlQuery := q.Build()
	args := q.Args()

	rows, err := t.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func scanTransaction(scanner rowScanner) (*Transaction, error) {
	var transaction Transaction
	var description, postDate, enterDate sql.NullString

	err := scanner.Scan(
		&transaction.GUID,
		&transaction.CurrencyGUID,
		&transaction.Num,
		&postDate,
		&enterDate,
		&description,
	)
	if err != nil {
		return nil, err
	}

	if postDate.Valid {
		pd, err := time.Parse("2006-01-02 15:04:05", postDate.String)
		if err != nil {
			return nil, err
		}
		transaction.PostDate = &pd
	}

	if enterDate.Valid {
		ed, err := time.Parse("2006-01-02 15:04:05", enterDate.String)
		if err != nil {
			return nil, err
		}
		transaction.EnterDate = &ed
	}

	if description.Valid {
		transaction.Description = &description.String
	}

	return &transaction, nil
}
