package cli

import (
	"database/sql"
	"encoding/json"
	"os"
	"path"
	"sync"

	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/v4/boil"
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
