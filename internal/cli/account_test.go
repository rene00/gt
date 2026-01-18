package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"gt/internal/store"
	"os"
	"testing"
)

func TestAccountCmd(t *testing.T) {
	c := &cli{}
	_, err := executeCommand(accountCmd(c), "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetAccountCmd(t *testing.T) {
	var err error
	ctx := context.Background()

	f, _ := os.CreateTemp("", "testdb-*.sqlite")
	dsn := f.Name()
	f.Close()
	defer os.Remove(dsn)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err = createTestingTables(ctx, db, t); err != nil {
		t.Fatal(err)
	}

	if _, err = db.ExecContext(ctx,
		"INSERT INTO accounts (guid, name, account_type, parent_guid, commodity_scu, non_std_scu) VALUES (?, ?, ?, ?, ?, ?)",
		"2",
		"test1",
		"EXPENSE",
		"EXPENSESGUID",
		100,
		100); err != nil {
		t.Fatal(err)
	}

	c := &cli{db: db}
	out, err := executeCommand(getAccountCmd(c), "2", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}

	var resp store.Account
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Name != "test1" {
		t.Fatalf("expected test1 but got %s", resp.Name)
	}

	out, err = executeCommand(getAccountCmd(c), "expenses:test1", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Name != "test1" {
		t.Fatalf("expected test1 but got %s", resp.Name)
	}
}

func TestListAccountCmd(t *testing.T) {
	ctx := context.Background()

	f, _ := os.CreateTemp("", "testdb-*.sqlite")
	dsn := f.Name()
	f.Close()
	defer os.Remove(dsn)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err = createTestingTables(ctx, db, t); err != nil {
		t.Fatal(err)
	}

	accounts := []struct {
		GUID        string
		Name        string
		AccountType string
	}{
		{GUID: "1", Name: "test1", AccountType: "EXPENSE"},
		{GUID: "2", Name: "test2", AccountType: "EXPENSE"},
	}

	for _, account := range accounts {
		_, err := db.ExecContext(ctx,
			"INSERT INTO accounts (guid, name, account_type, commodity_scu, non_std_scu) VALUES (?, ?, ?, ?, ?)",
			account.GUID,
			account.Name,
			account.AccountType,
			100,
			100,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	c := &cli{db: db}

	out, err := executeCommand(listAccountCmd(c), "--output", "json")
	if err != nil {
		t.Fatal(err)
	}

	var resp []store.Account
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}

	if len(resp) != len(accounts)+2 {
		t.Fatalf("expected lenth of %d but got %d", len(resp), len(accounts)+2)
	}

	for _, i := range accounts {
		exists := false
		for _, ii := range resp {
			if i.Name == ii.Name && i.GUID == ii.GUID {
				exists = true
				break
			}
		}
		if !exists {
			t.Fatal()
		}
	}
}

func TestUpdateAccountCmd(t *testing.T) {
	t.Run("no args", func(t *testing.T) {
		ctx := context.Background()

		f, _ := os.CreateTemp("", "testdb-*.sqlite")
		dsn := f.Name()
		f.Close()
		defer os.Remove(dsn)

		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			t.Fatal(err)
		}

		if err = createTestingTables(ctx, db, t); err != nil {
			t.Fatal(err)
		}

		c := &cli{db: db}

		_, err = executeCommand(updateAccountCmd(c), "does-not-exist", "--output", "json")
		if err != ErrAccountDoesNotExist {
			t.Fatalf("expected ErrAccountMissing error but received %v", err)
		}
	})

	t.Run("account already exists", func(t *testing.T) {
		ctx := context.Background()

		f, _ := os.CreateTemp("", "testdb-*.sqlite")
		dsn := f.Name()
		f.Close()
		defer os.Remove(dsn)

		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			t.Fatal(err)
		}

		if err = createTestingTables(ctx, db, t); err != nil {
			t.Fatal(err)
		}

		accounts := []struct {
			GUID        string
			Name        string
			AccountType string
			ParentGUID  string
		}{
			{GUID: "1", Name: "test1", AccountType: "EXPENSE", ParentGUID: "EXPENSESGUID"},
			{GUID: "2", Name: "test2", AccountType: "EXPENSE", ParentGUID: "EXPENSESGUID"},
		}

		for _, account := range accounts {
			_, err := db.ExecContext(ctx,
				"INSERT INTO accounts (guid, name, account_type, commodity_scu, non_std_scu, parent_guid) VALUES (?, ?, ?, ?, ?, ?)",
				account.GUID,
				account.Name,
				account.AccountType,
				100,
				100,
				account.ParentGUID,
			)
			if err != nil {
				t.Fatal(err)
			}
		}

		c := &cli{db: db}
		_, err = executeCommand(updateAccountCmd(c), "expenses:test1", "--name", "test2")
		if err != ErrAccountAlreadyExists {
			t.Fatalf("expecting ErrAccountAlreadyExists err but received %v", err)
		}
	})

	t.Run("update account", func(t *testing.T) {
		var err error
		ctx := context.Background()

		f, _ := os.CreateTemp("", "testdb-*.sqlite")
		dsn := f.Name()
		f.Close()
		defer os.Remove(dsn)

		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			t.Fatal(err)
		}

		if err = createTestingTables(ctx, db, t); err != nil {
			t.Fatal(err)
		}

		if _, err = db.ExecContext(ctx,
			"INSERT INTO accounts (guid, name, account_type, parent_guid, commodity_scu, non_std_scu) VALUES (?, ?, ?, ?, ?, ?)",
			"2",
			"test1",
			"EXPENSE",
			"EXPENSESGUID",
			100,
			100); err != nil {
			t.Fatal(err)
		}

		c := &cli{db: db}

		out, err := executeCommand(updateAccountCmd(c), "expenses:test1", "--name=test2", "--description=test-2", "--output=json")
		if err != nil {
			t.Fatal(err)
		}

		var resp store.Account
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Name != "test2" {
			t.Fatalf("expected name test2 but got %s", resp.Name)
		}

		if *resp.Description != "test-2" {
			t.Fatalf("expected description test-2 but got %s", resp.Name)
		}
	})
}
