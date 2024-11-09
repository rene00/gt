package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gt/models/gnucash"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var (
	GnuCashSqliteURI   = "/home/rene/.gnucash.sql.gnucash?mode=ro&_query_only=true"
	GnuCashSqliteURIRw = "/home/rene/.gnucash.sql.gnucash"
)

func accountCmd(cli *cli) *cobra.Command {
	var cmd = &cobra.Command{
		Use: "account",
	}
	cmd.AddCommand(getAccountCmd(cli))
	cmd.AddCommand(listAccountCmd(cli))
	return cmd
}

func getAccountCmd(cli *cli) *cobra.Command {
	var cmd = &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("missing account guid or account name")
			}
			guidOrAccountName := args[0]

			db, err := sql.Open("sqlite3", cli.config.GnucashDBFile)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			account, err := gnucash.Accounts(qm.Where("guid=?", guidOrAccountName)).One(cmd.Context(), db)
			if err != nil {
				switch {
				case errors.Is(err, sql.ErrNoRows):
					return fmt.Errorf("find account using name")
				default:
					return err
				}
			}

			resp, err := json.MarshalIndent(account, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	return cmd
}

func listAccountCmd(cli *cli) *cobra.Command {
	var flags struct {
		NameLike string
	}
	var cmd = &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			var q []qm.QueryMod

			db, err := sql.Open("sqlite3", cli.config.GnucashDBFile)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			if flags.NameLike != "" {
				q = append(q, qm.Where("name LIKE ?", flags.NameLike))
			}

			accounts, err := gnucash.Accounts(q...).All(cmd.Context(), db)
			if err != nil {
				return err
			}

			resp, err := json.MarshalIndent(accounts, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.NameLike, "name-like", "", "Name like")
	return cmd
}
