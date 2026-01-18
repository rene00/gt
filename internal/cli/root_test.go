package cli

import (
	"bytes"
	"context"
	"database/sql"
	"testing"

	"github.com/spf13/cobra"
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

	createTableAccounts := `CREATE TABLE accounts(
		guid text(32) PRIMARY KEY NOT NULL, 
		name text(2048) NOT NULL, 
		account_type text(2048) NOT NULL, 
		commodity_guid text(32), 
		commodity_scu integer NOT NULL, 
		non_std_scu integer NOT NULL, 
		parent_guid text(32), 
		code text(2048), 
		description text(2048), 
		hidden integer, 
		placeholder integer
	);`
	if _, err = db.ExecContext(ctx, createTableAccounts); err != nil {
		return err
	}

	rootGUID := "ROOTGUID"
	expensesGUID := "EXPENSESGUID"

	accounts := []struct {
		Name         string
		AccountType  string
		GUID         string
		ParentGUID   *string
		CommoditySCU int64
		NonSTDSCU    int64
	}{
		{Name: "Root Account", AccountType: "ROOT", GUID: rootGUID, ParentGUID: nil, CommoditySCU: 100, NonSTDSCU: 100},
		{Name: "Expenses", ParentGUID: &rootGUID, AccountType: "EXPENSE", GUID: expensesGUID, CommoditySCU: 100, NonSTDSCU: 100},
	}

	for _, account := range accounts {
		if _, err = db.ExecContext(ctx,
			"INSERT INTO accounts (guid, name, account_type, parent_guid, commodity_scu, non_std_scu) VALUES (?, ?, ?, ?, ?, ?)",
			account.GUID,
			account.Name,
			account.AccountType,
			account.ParentGUID,
			account.CommoditySCU,
			account.NonSTDSCU,
		); err != nil {
			return err
		}
	}

	return nil
}
