package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"gt/models/gnucash"
	"testing"

	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
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

	c := &cli{}
	c.db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	boil.SetDB(c.db)

	if err = createTestingTables(ctx, c.db, t); err != nil {
		t.Fatal(err)
	}

	account := gnucash.Account{
		Name: "test1", GUID: "2", ParentGUID: null.StringFrom("EXPENSESGUID"),
	}

	if err := account.Insert(ctx, c.db, boil.Infer()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand(getAccountCmd(c), "2")
	if err != nil {
		t.Fatal(err)
	}

	var resp gnucash.Account
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Name != "test1" {
		t.Fatalf("expected test1 but got %s", resp.Name)
	}

	out, err = executeCommand(getAccountCmd(c), "expenses:test1")
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

func TestGetAccountTree(t *testing.T) {
	var err error
	ctx := context.Background()

	c := &cli{}
	c.db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	boil.SetDB(c.db)

	if err = createTestingTables(ctx, c.db, t); err != nil {
		t.Fatal(err)
	}

	account := gnucash.Account{
		Name: "test1", GUID: "2", ParentGUID: null.StringFrom("EXPENSESGUID"),
	}
	if err := account.Insert(ctx, c.db, boil.Infer()); err != nil {
		t.Fatal(err)
	}

	accountTree, err := c.getAccountTree(ctx, &account)
	if err != nil {
		t.Fatal(err)
	}

	if len(accountTree) != 3 {
		t.Fatalf("expect length of 3 but got %d", len(accountTree))
	}

	rootAccount := accountTree[0]
	if rootAccount.Name != "Root Account" {
		t.Fatal()
	}

	lastAccount := accountTree[len(accountTree)-1]
	if lastAccount.Name != "test1" {
		t.Fatalf("expected name of test1 but got %s", lastAccount.Name)
	}
}

func TestListAccountCmd(t *testing.T) {
	var err error
	ctx := context.Background()

	c := &cli{}
	c.db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	boil.SetDB(c.db)

	if err = createTestingTables(ctx, c.db, t); err != nil {
		t.Fatal(err)
	}

	accounts := []*gnucash.Account{
		{Name: "test1", GUID: "1"}, {Name: "test2", GUID: "2"},
	}

	for _, account := range accounts {
		if err := account.Insert(ctx, c.db, boil.Infer()); err != nil {
			t.Fatal(err)
		}
	}

	out, err := executeCommand(listAccountCmd(c), "")
	if err != nil {
		t.Fatal(err)
	}

	var resp []gnucash.Account
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
		var err error
		ctx := context.Background()

		c := &cli{}
		c.db, err = sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		boil.SetDB(c.db)

		if err = createTestingTables(ctx, c.db, t); err != nil {
			t.Fatal(err)
		}

		_, err = executeCommand(updateAccountCmd(c), "does-not-exist")
		if err != ErrAccountDoesNotExist {
			t.Fatalf("expected ErrAccountMissing error but received %v", err)
		}
	})

	t.Run("account already exists", func(t *testing.T) {
		var err error
		ctx := context.Background()

		c := &cli{}
		c.db, err = sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		boil.SetDB(c.db)

		if err = createTestingTables(ctx, c.db, t); err != nil {
			t.Fatal(err)
		}

		accounts := []*gnucash.Account{
			{Name: "test1", GUID: "2", ParentGUID: null.StringFrom("EXPENSESGUID")}, {Name: "test2", GUID: "3", ParentGUID: null.StringFrom("EXPENSESGUID")},
		}

		for _, account := range accounts {
			if err := account.Insert(ctx, c.db, boil.Infer()); err != nil {
				t.Fatal(err)
			}
		}

		_, err = executeCommand(updateAccountCmd(c), "expenses:test1", "--name=test2")
		if err != ErrAccountAlreadyExists {
			t.Fatalf("expecting ErrAccountAlreadyExists err but received %v", err)
		}
	})

	t.Run("update account", func(t *testing.T) {
		var err error
		ctx := context.Background()

		c := &cli{}
		c.db, err = sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		boil.SetDB(c.db)

		if err = createTestingTables(ctx, c.db, t); err != nil {
			t.Fatal(err)
		}

		account := gnucash.Account{Name: "test1", GUID: "2", Description: null.StringFrom("test-1"), ParentGUID: null.StringFrom("EXPENSESGUID")}
		if err := account.Insert(ctx, c.db, boil.Infer()); err != nil {
			t.Fatal(err)
		}

		out, err := executeCommand(updateAccountCmd(c), "expenses:test1", "--name=test2", "--description=test-2")
		if err != nil {
			t.Fatal(err)
		}

		var resp gnucash.Account
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Name != "test2" {
			t.Fatalf("expected name test2 but got %s", resp.Name)
		}

		if resp.Description.String != "test-2" {
			t.Fatalf("expected description test-2 but got %s", resp.Name)
		}
	})
}
