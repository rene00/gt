package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Split struct {
	GUID           string
	TXGUID         string
	AccountGUID    string
	Memo           string
	Action         string
	ReconcileState string
	ReconcileDate  *time.Time
	ValueNum       int64
	ValueDenom     int64
	QuantityNum    int64
	QuantityDenom  int64
	LogGUID        *string
	Account        *Account
}

type SplitQuery struct {
	whereClauses []string
	args         []any
	orderFields  []orderField
	limit        *int
	offset       *int
}

func NewSplitQuery() *SplitQuery {
	return &SplitQuery{
		whereClauses: make([]string, 0),
		args:         make([]any, 0),
		orderFields:  make([]orderField, 0),
	}
}

func (q *SplitQuery) Where(clause string, args ...any) *SplitQuery {
	q.whereClauses = append(q.whereClauses, clause)
	q.args = append(q.args, args...)
	return q
}

func (q *SplitQuery) OrderBy(field string, descending bool) *SplitQuery {
	q.orderFields = append(q.orderFields, orderField{field: field, descending: descending})
	return q
}

func (q *SplitQuery) Limit(limit int) *SplitQuery {
	q.limit = &limit
	return q
}

func (q *SplitQuery) Offset(offset int) *SplitQuery {
	q.offset = &offset
	return q
}

func (q *SplitQuery) Page(page, pageSize int) *SplitQuery {
	offset := (page - 1) * pageSize
	return q.Limit(pageSize).Offset(offset)
}

func (q *SplitQuery) Build() string {
	var b strings.Builder
	b.WriteString(`
SELECT
	guid,
	tx_guid,
	account_guid,
	memo,
	action,
	reconcile_state,
	reconcile_date,
	value_num,
	value_denom,
	quantity_num,
	quantity_denom,
	lot_guid
FROM splits
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

func (q *SplitQuery) Args() []any {
	return q.args
}

type SplitsStorer interface {
	All(ctx context.Context, q *SplitQuery) ([]*Split, error)
	Update(ctx context.Context, split *Split) error
}

type SplitsStore struct {
	db DBTX
}

func (s SplitsStore) All(ctx context.Context, q *SplitQuery) ([]*Split, error) {
	sqlQuery := q.Build()
	args := q.Args()

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	splits, err := s.scanSplits(rows)
	if err != nil {
		return nil, err
	}

	return splits, nil
}

func (s *SplitsStore) scanSplits(rows *sql.Rows) ([]*Split, error) {
	var splits []*Split
	for rows.Next() {
		var split Split
		var reconcileDate, logGUID sql.NullString

		err := rows.Scan(
			&split.GUID,
			&split.TXGUID,
			&split.AccountGUID,
			&split.Memo,
			&split.Action,
			&split.ReconcileState,
			&reconcileDate,
			&split.ValueNum,
			&split.ValueDenom,
			&split.QuantityNum,
			&split.QuantityDenom,
			&logGUID,
		)
		if err != nil {
			return splits, err
		}

		if reconcileDate.Valid {
			rd, err := time.Parse("2006-01-02 15:04:05", reconcileDate.String)
			if err != nil {
				return nil, err
			}
			split.ReconcileDate = &rd
		}

		if logGUID.Valid {
			split.LogGUID = &logGUID.String
		}

		splits = append(splits, &split)
	}

	return splits, rows.Err()
}

func (s SplitsStore) Update(ctx context.Context, split *Split) error {
	query := `
UPDATE splits
SET
	tx_guid = ?,
	account_guid = ?,
	memo = ?,
	action = ?,
	reconcile_state = ?,
	reconcile_date = ?,
	value_num = ?,
	value_denom = ?,
	quantity_num = ?,
	quantity_denom = ?,
	lot_guid = ?
WHERE guid = ?
`

	var reconcileDate sql.NullString
	if split.ReconcileDate != nil {
		reconcileDate = sql.NullString{
			String: split.ReconcileDate.Format("2006-01-02 15:04:05"),
			Valid:  true,
		}
	}

	var logGUID sql.NullString
	if split.LogGUID != nil {
		logGUID = sql.NullString{
			String: *split.LogGUID,
			Valid:  true,
		}
	}

	result, err := s.db.ExecContext(
		ctx,
		query,
		split.TXGUID,
		split.AccountGUID,
		split.Memo,
		split.Action,
		split.ReconcileState,
		reconcileDate,
		split.ValueNum,
		split.ValueDenom,
		split.QuantityNum,
		split.QuantityDenom,
		logGUID,
		split.GUID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
