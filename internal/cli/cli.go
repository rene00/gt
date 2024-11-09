package cli

import (
	"encoding/json"
	"os"
	"path"
	"sync"

	"github.com/spf13/cobra"
)

type cli struct {
	debug      bool
	configFile string
	initOnce   sync.Once
	errOnce    error
	config     config
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
	return nil
}
