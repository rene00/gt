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
	transactions.guid,
	transactions.currency_guid,
	transactions.num,
	transactions.post_date,
	transactions.enter_date,
	transactions.description,
	splits.guid,
	splits.account_guid,
	splits.memo,
	splits.action,
	splits.reconcile_state,
	splits.reconcile_date,
	splits.value_num,
	splits.value_denom,
	splits.quantity_num,
	splits.quantity_denom,
	splits.lot_guid,
	accounts.guid,
	accounts.name,
	accounts.account_type,
	accounts.commodity_guid,
	accounts.commodity_scu,
	accounts.non_std_scu,
	accounts.parent_guid,
	accounts.code,
	accounts.description,
	accounts.hidden,
	accounts.placeholder
FROM transactions
LEFT JOIN splits ON splits.tx_guid = transactions.guid
LEFT JOIN accounts ON accounts.guid = splits.account_guid
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
	q := NewTransactionQuery().Where("transactions.guid=?", guid)
	rows, err := t.db.QueryContext(ctx, q.Build(), q.Args()...)
	if err != nil {
		return nil, err
	}

	transactions, err := scanTransactions(rows)
	if err != nil {
		return nil, err
	}

	if len(transactions) == 0 {
		return nil, sql.ErrNoRows
	}

	return transactions[0], nil
}

func (t TransactionsStore) All(ctx context.Context, q *TransactionQuery) ([]*Transaction, error) {

	var guidQuery strings.Builder
	guidQuery.WriteString("SELECT guid FROM transactions")

	if len(q.whereClauses) > 0 {
		guidQuery.WriteString("\nWHERE ")
		guidQuery.WriteString(strings.Join(q.whereClauses, " AND "))
	}

	if len(q.orderFields) > 0 {
		guidQuery.WriteString("\nORDER BY ")
		orders := make([]string, len(q.orderFields))
		for i, field := range q.orderFields {
			direction := "ASC"
			if field.descending {
				direction = "DESC"
			}
			orders[i] = fmt.Sprintf("%s %s", field.field, direction)
		}
		guidQuery.WriteString(strings.Join(orders, ", "))
	}

	if q.limit != nil {
		guidQuery.WriteString(fmt.Sprintf("\nLIMIT %d", *q.limit))
	}

	if q.offset != nil {
		guidQuery.WriteString(fmt.Sprintf("\nOFFSET %d", *q.offset))
	}

	guidRows, err := t.db.QueryContext(ctx, guidQuery.String(), q.Args()...)
	if err != nil {
		return nil, err
	}
	defer guidRows.Close()

	var guids []string
	for guidRows.Next() {
		var guid string
		if err := guidRows.Scan(&guid); err != nil {
			return nil, err
		}
		guids = append(guids, guid)
	}

	if err := guidRows.Err(); err != nil {
		return nil, err
	}

	transactions := []*Transaction{}
	if len(guids) == 0 {
		return transactions, nil
	}

	placeholders := make([]string, len(guids))
	guidArgs := make([]any, len(guids))
	for i, guid := range guids {
		placeholders[i] = "?"
		guidArgs[i] = guid
	}

	fullQuery := NewTransactionQuery()
	fullQuery.Where(fmt.Sprintf("transactions.guid IN (%s)", strings.Join(placeholders, ",")), guidArgs...)

	for _, orderField := range q.orderFields {
		fullQuery.OrderBy(orderField.field, orderField.descending)
	}

	rows, err := t.db.QueryContext(ctx, fullQuery.Build(), fullQuery.Args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactions(rows)
}

func scanTransactions(rows *sql.Rows) ([]*Transaction, error) {
	transactionMap := make(map[string]*Transaction)
	var orderedGUIDs []string

	for rows.Next() {
		var transactionDescription, transactionPostDate, transactionEnterDate sql.NullString
		var transactionGUID, transactionCurrencyGUID, transactionNum sql.NullString
		var splitGUID, splitAccountGUID, splitMemo, splitAction, splitReconcileState sql.NullString
		var splitReconcileDate, splitLogGUID sql.NullString
		var splitValueNum, splitValueDenom, splitQuantityNum, splitQuantityDenom sql.NullInt64
		var accountGUID, accountName, accountAccountType sql.NullString
		var accountCommodityGUID, accountParentGUID, accountCode, accountDescription sql.NullString
		var accountCommoditySCU, accountNonStdSCU, accountHidden, accountPlaceholder sql.NullInt64

		err := rows.Scan(
			&transactionGUID,
			&transactionCurrencyGUID,
			&transactionNum,
			&transactionPostDate,
			&transactionEnterDate,
			&transactionDescription,
			&splitGUID,
			&splitAccountGUID,
			&splitMemo,
			&splitAction,
			&splitReconcileState,
			&splitReconcileDate,
			&splitValueNum,
			&splitValueDenom,
			&splitQuantityNum,
			&splitQuantityDenom,
			&splitLogGUID,
			&accountGUID,
			&accountName,
			&accountAccountType,
			&accountCommodityGUID,
			&accountCommoditySCU,
			&accountNonStdSCU,
			&accountParentGUID,
			&accountCode,
			&accountDescription,
			&accountHidden,
			&accountPlaceholder,
		)
		if err != nil {
			return nil, err
		}

		transaction, exists := transactionMap[transactionGUID.String]
		if !exists {
			transaction = &Transaction{
				GUID:         transactionGUID.String,
				CurrencyGUID: transactionCurrencyGUID.String,
				Num:          transactionNum.String,
				Splits:       make([]*Split, 0),
			}

			if transactionPostDate.Valid {
				pd, err := time.Parse("2006-01-02 15:04:05", transactionPostDate.String)
				if err != nil {
					return nil, err
				}
				transaction.PostDate = &pd
			}

			if transactionEnterDate.Valid {
				ed, err := time.Parse("2006-01-02 15:04:05", transactionEnterDate.String)
				if err != nil {
					return nil, err
				}
				transaction.EnterDate = &ed
			}

			if transactionDescription.Valid {
				transaction.Description = &transactionDescription.String
			}

			transactionMap[transactionGUID.String] = transaction
			orderedGUIDs = append(orderedGUIDs, transaction.GUID)
		}

		if splitGUID.Valid {
			split := Split{
				GUID:           splitGUID.String,
				TXGUID:         transactionGUID.String,
				AccountGUID:    splitAccountGUID.String,
				ReconcileState: splitReconcileState.String,
			}

			if splitMemo.Valid {
				split.Memo = splitMemo.String
			}

			if splitAction.Valid {
				split.Action = splitAction.String
			}

			if splitValueNum.Valid && splitValueDenom.Valid {
				split.ValueNum = splitValueNum.Int64
				split.ValueDenom = splitValueDenom.Int64
			}

			if splitQuantityNum.Valid && splitQuantityDenom.Valid {
				split.QuantityNum = splitQuantityNum.Int64
				split.QuantityDenom = splitQuantityDenom.Int64
			}

			if splitReconcileDate.Valid {
				rd, err := time.Parse("2006-01-02 15:04:05", splitReconcileDate.String)
				if err != nil {
					return nil, err
				}
				split.ReconcileDate = &rd
			}

			if splitLogGUID.Valid {
				split.LogGUID = &splitLogGUID.String
			}

			if accountGUID.Valid {
				account := Account{
					GUID:         accountGUID.String,
					Name:         accountName.String,
					AccountType:  accountAccountType.String,
					CommoditySCU: accountCommoditySCU.Int64,
					NonSTDSCU:    accountNonStdSCU.Int64,
				}

				if accountHidden.Valid {
					account.Hidden = &accountHidden.Int64
				}

				if accountPlaceholder.Valid {
					account.Placeholder = &accountPlaceholder.Int64
				}

				split.Account = &account
			}

			transaction.Splits = append(transaction.Splits, &split)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]*Transaction, 0, len(orderedGUIDs))
	for _, guid := range orderedGUIDs {
		result = append(result, transactionMap[guid])
	}

	return result, nil
}
