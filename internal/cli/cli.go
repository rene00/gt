package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gt/models/gnucash"
	"os"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var (
	ErrTransactionMissing   = errors.New("transaction guid missing")
	ErrAccountDoesNotExist  = errors.New("account does not exist")
	ErrAccountMissingParent = errors.New("account missing parent")
	ErrAccountMissing       = errors.New("account name or guid missing")
	ErrAccountAlreadyExists = errors.New("account already exists")
)

type cli struct {
	debug      bool
	configFile string
	initOnce   sync.Once
	errOnce    error
	config     config
	db         *sql.DB
}

type config struct {
	GnucashDBFile string `json:"gnucash_db_file"`
}

func (c *cli) setup() error {
	return c.init()
}

func (c *cli) init() error {
	c.initOnce.Do(func() {
		if c.errOnce = c.initContext(); c.errOnce != nil {
			return
		}
		cobra.EnableCommandSorting = false

	})
	return c.errOnce
}

func (c *cli) initContext() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c.config = config{
		GnucashDBFile: path.Join(homeDir, ".gnucash.sql.gnucash"),
	}

	if c.configFile != "" {
		f, err := os.ReadFile(c.configFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(f, &c.config); err != nil {
			return err
		}
	}

	c.db, err = sql.Open("sqlite3", c.config.GnucashDBFile)
	if err != nil {
		return err
	}
	boil.SetDB(c.db)

	return nil
}

// accountExists accepts an account and checks if it exists.
func (c *cli) accountExists(ctx context.Context, account *gnucash.Account) (bool, error) {
	p, err := gnucash.Accounts(qm.Where("guid=?", account.ParentGUID.String)).One(ctx, c.db)
	if err != nil {
		return false, err
	}
	return gnucash.Accounts(qm.Where("name=? AND parent_guid=?", account.Name, p.GUID)).Exists(ctx, c.db)
}

// getAccountFromGUIDOrAccountTree accepts a string which is an account GUID or
// a case insensitive absolute account name in gnucash syntax and returns the
// account.
func (c *cli) getAccountFromGUIDOrAccountTree(ctx context.Context, s string) (*gnucash.Account, error) {
	var account *gnucash.Account
	var err error
	account, err = gnucash.Accounts(qm.Where("guid=?", s)).One(ctx, c.db)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return getAccountFromAccountTreeString(ctx, c.db, s)
		default:
			return account, err
		}
	}
	return account, nil
}

func (c *cli) getAccountTreeString(ctx context.Context, accountTree []*gnucash.Account) (s string, err error) {
	var b strings.Builder
	for _, account := range accountTree {
		b.WriteString(fmt.Sprintf("%s:", account.Name))
	}
	return b.String(), nil
}

// getAccountTree returns a slice containing account tree for a gnucash.Account.
// Slice is ordered with child from Root account being the first item.
func (c *cli) getAccountTree(ctx context.Context, account *gnucash.Account) ([]*gnucash.Account, error) {
	accountTree := []*gnucash.Account{account}
	for {
		parentAccount, err := gnucash.Accounts(qm.Where("guid=?", account.ParentGUID.String)).One(ctx, c.db)
		if err != nil {
			return accountTree, err
		}
		accountTree = append(accountTree, parentAccount)
		account = parentAccount
		if parentAccount.ParentGUID.String == "" {
			break
		}
	}
	slices.Reverse(accountTree)
	return accountTree, nil
}
