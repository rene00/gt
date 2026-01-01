package cli

import (
	"bytes"
	"context"
	"database/sql"
	"gt/models/gnucash"
	"testing"

	"github.com/spf13/cobra"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	_, output, err = executeCommandC(root, args...)
	return output, err
}

func executeCommandC(root *cobra.Command, args ...string) (*cobra.Command, string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	c, err := root.ExecuteC()
	return c, buf.String(), err
}

func createTestingTables(ctx context.Context, db *sql.DB, t *testing.T) error {
	var err error
	t.Helper()

	createTableAccounts := `CREATE TABLE accounts(guid text(32) PRIMARY KEY NOT NULL, name text(2048) NOT NULL, account_type text(2048) NOT NULL, commodity_guid text(32), commodity_scu integer NOT NULL, non_std_scu integer NOT NULL, parent_guid text(32), code text(2048), description text(2048), hidden integer, placeholder integer);`
	if _, err = queries.Raw(createTableAccounts).ExecContext(ctx, db); err != nil {
		return err
	}

	accounts := []gnucash.Account{
		gnucash.Account{Name: "Root Account", ParentGUID: null.StringFromPtr(nil), AccountType: "ROOT", GUID: "ROOTGUID"},
		gnucash.Account{Name: "Expenses", ParentGUID: null.StringFrom("ROOTGUID"), AccountType: "EXPENSE", GUID: "EXPENSESGUID"},
	}
	for _, account := range accounts {
		if err = account.Insert(ctx, db, boil.Infer()); err != nil {
			return err
		}
	}

	return nil
}

func createTestingAccount(ctx context.Context, db *sql.DB, t *testing.T, account *gnucash.Account) error {
	t.Helper()
	return account.Insert(ctx, db, boil.Infer())
}
