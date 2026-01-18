package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

type Account struct {
	GUID          string
	Name          string
	FullName      string
	AccountType   string
	CommodityGUID *string
	CommoditySCU  int64
	NonSTDSCU     int64
	ParentGUID    *string
	Code          *string
	Description   *string
	Hidden        *int64
	Placeholder   *int64
}

type AccountQuery struct {
	whereClauses []string
	args         []any
	orderFields  []orderField
	limit        *int
	offset       *int
}

func NewAccountQuery() *AccountQuery {
	return &AccountQuery{
		whereClauses: make([]string, 0),
		args:         make([]any, 0),
		orderFields:  make([]orderField, 0),
	}
}

func (q *AccountQuery) Where(clause string, args ...any) *AccountQuery {
	q.whereClauses = append(q.whereClauses, clause)
	q.args = append(q.args, args...)
	return q
}

func (q *AccountQuery) OrderBy(field string, descending bool) *AccountQuery {
	q.orderFields = append(q.orderFields, orderField{field: field, descending: descending})
	return q
}

func (q *AccountQuery) Limit(limit int) *AccountQuery {
	q.limit = &limit
	return q
}

func (q *AccountQuery) Offset(offset int) *AccountQuery {
	q.offset = &offset
	return q
}

func (q *AccountQuery) Page(page, pageSize int) *AccountQuery {
	offset := (page - 1) * pageSize
	return q.Limit(pageSize).Offset(offset)
}

func (q *AccountQuery) Build() string {
	var b strings.Builder
	b.WriteString(`
SELECT
	guid,
	name,
	account_type,
	commodity_guid,
	commodity_scu,
	non_std_scu,
	parent_guid,
	code,
	description,
	hidden,
	placeholder
FROM accounts
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

func (q *AccountQuery) Args() []any {
	return q.args
}

type AccountsStorer interface {
	All(ctx context.Context, q *AccountQuery) ([]*Account, error)
	Get(ctx context.Context, s string, opts ...AccountsOptFunc) (*Account, error)
	Update(ctx context.Context, account *Account) error
}

type AccountsStore struct {
	db   DBTX
	Opts AccountsOpts
}

type AccountsOpts struct {
	withAccountTree bool
}

func defaultAccountsOpts() *AccountsOpts {
	return &AccountsOpts{
		withAccountTree: false,
	}
}

type AccountsOptFunc func(*AccountsOpts)

func WithAccountTree(b bool) AccountsOptFunc {
	return func(o *AccountsOpts) {
		o.withAccountTree = b
	}
}

// getFullAccountName takes a store.Account and attempts to return its full account name (e.g. expenses:dining:pizza)
func getFullAccountName(ctx context.Context, db DBTX, account *Account) (string, error) {
	s := []string{account.Name}
	for account.ParentGUID != nil {
		var err error
		q := NewAccountQuery().Where("guid=?", account.ParentGUID)
		row := db.QueryRowContext(ctx, q.Build(), q.Args()...)
		account, err = scanAccount(row)
		if err != nil {
			return "", err
		}
		if strings.ToLower(account.AccountType) == "root" {
			break
		}
		s = append(s, account.Name)
	}
	slices.Reverse(s)
	return strings.Join(s, ":"), nil
}

func getAccountFromAccountTree(ctx context.Context, db DBTX, s string) (*Account, error) {
	q := NewAccountQuery().Where("account_type=? AND name=? AND parent_guid IS NULL", "ROOT", "Root Account")
	row := db.QueryRowContext(ctx, q.Build(), q.Args()...)
	rootAccount, err := scanAccount(row)
	if err != nil {
		return nil, err
	}

	parentAccounts := []*Account{rootAccount}
	accounts := strings.Split(s, ":")
	for idx, accountName := range accounts {
		parentAccount := parentAccounts[idx]
		q = NewAccountQuery().Where("name=? COLLATE NOCASE and parent_guid=?", accountName, parentAccount.GUID)
		row := db.QueryRowContext(ctx, q.Build(), q.Args()...)
		account, err := scanAccount(row)
		if err != nil {
			return nil, err
		}
		parentAccounts = append(parentAccounts, account)
	}

	if len(parentAccounts) != len(accounts)+1 {
		return nil, fmt.Errorf("failed to find account from tree")
	}

	return parentAccounts[len(parentAccounts)-1], nil
}

func (s AccountsStore) Get(ctx context.Context, guidOrName string, opts ...AccountsOptFunc) (*Account, error) {
	var account *Account
	o := defaultAccountsOpts()
	for _, fn := range opts {
		fn(o)
	}

	if o.withAccountTree {
		account, err := getAccountFromAccountTree(ctx, s.db, guidOrName)
		if err != nil {
			return nil, err
		}
		fullName, err := getFullAccountName(ctx, s.db, account)
		if err != nil {
			return nil, err
		}
		account.FullName = fullName
		return account, nil
	}

	q := NewAccountQuery()
	q.Where("guid= ?", guidOrName)

	sqlQuery := q.Build()
	args := q.Args()

	row := s.db.QueryRowContext(ctx, sqlQuery, args...)
	account, err := scanAccount(row)
	if err != nil {
		return nil, err
	}

	fullName, err := getFullAccountName(ctx, s.db, account)
	if err != nil {
		return nil, err
	}
	account.FullName = fullName

	return account, nil
}

func (s AccountsStore) All(ctx context.Context, q *AccountQuery) ([]*Account, error) {
	sqlQuery := q.Build()
	args := q.Args()

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		fullName, err := getFullAccountName(ctx, s.db, account)
		if err != nil {
			return nil, err
		}
		account.FullName = fullName
		accounts = append(accounts, account)
	}

	return accounts, rows.Err()
}

func scanAccount(scanner rowScanner) (*Account, error) {
	var account Account
	var commodityGUID, parentGUID, code, description sql.NullString
	var hidden, placeholder sql.NullInt64

	err := scanner.Scan(
		&account.GUID,
		&account.Name,
		&account.AccountType,
		&commodityGUID,
		&account.CommoditySCU,
		&account.NonSTDSCU,
		&parentGUID,
		&code,
		&description,
		&hidden,
		&placeholder,
	)
	if err != nil {
		return nil, err
	}

	if commodityGUID.Valid {
		account.CommodityGUID = &commodityGUID.String
	}

	if parentGUID.Valid {
		account.ParentGUID = &parentGUID.String
	}

	if code.Valid {
		account.Code = &code.String
	}

	if description.Valid {
		account.Description = &description.String
	}

	if hidden.Valid {
		account.Hidden = &hidden.Int64
	}

	if placeholder.Valid {
		account.Placeholder = &placeholder.Int64
	}

	return &account, nil
}

func (a AccountsStore) Update(ctx context.Context, account *Account) error {
	query := `
UPDATE accounts
SET
	name = ?,
	account_type = ?,
	commodity_guid = ?,
	commodity_scu = ?,
	non_std_scu = ?,
	parent_guid = ?,
	code = ?,
	description = ?,
	hidden = ?,
	placeholder = ?
WHERE guid = ? AND rowid IN (
    SELECT rowid FROM accounts WHERE guid = ? LIMIT 1
)
`

	var commodityGUID sql.NullString
	if account.CommodityGUID != nil {
		commodityGUID = sql.NullString{
			String: *account.CommodityGUID,
			Valid:  true,
		}
	}

	var parentGUID sql.NullString
	if account.ParentGUID != nil {
		parentGUID = sql.NullString{
			String: *account.ParentGUID,
			Valid:  true,
		}
	}

	var code sql.NullString
	if account.Code != nil {
		code = sql.NullString{
			String: *account.Code,
			Valid:  true,
		}
	}

	var description sql.NullString
	if account.Description != nil {
		description = sql.NullString{
			String: *account.Description,
			Valid:  true,
		}
	}

	var hidden sql.NullInt64
	if account.Hidden != nil {
		hidden = sql.NullInt64{
			Int64: *account.Hidden,
			Valid: true,
		}
	}

	var placeholder sql.NullInt64
	if account.Placeholder != nil {
		placeholder = sql.NullInt64{
			Int64: *account.Placeholder,
			Valid: true,
		}
	}

	result, err := a.db.ExecContext(
		ctx,
		query,
		account.Name,
		account.AccountType,
		commodityGUID,
		account.CommoditySCU,
		account.NonSTDSCU,
		parentGUID,
		code,
		description,
		hidden,
		placeholder,
		account.GUID,
		account.GUID,
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
