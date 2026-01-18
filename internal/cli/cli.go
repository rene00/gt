package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path"
	"sync"

	"github.com/spf13/cobra"
)

var (
	ErrTransactionMissing   = errors.New("transaction guid missing")
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrAccountDoesNotExist  = errors.New("account does not exist")
	ErrAccountMissingParent = errors.New("account missing parent")
	ErrAccountMissing       = errors.New("account name or guid missing")
	ErrAccountAlreadyExists = errors.New("account already exists")
)

var (
	FlagsUsageOutput           = "Output format (json, table)"
	FlagsUsageIncludeTotals    = "Include account totals when rendering table"
	FlagsUsageAccountShortName = "Output accounts short name"
)

func accountError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrAccountDoesNotExist
	default:
		return err
	}
}

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

	return nil
}
