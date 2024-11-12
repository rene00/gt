package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gt/models/gnucash"
	"strings"

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

// getAccountFromGUIDOrAccountTree accepts a string which is an account GUID or
// a case insensitive absolute account name in gnucash syntax and returns the
// account.
func getAccountFromGUIDOrAccountTree(ctx context.Context, db *sql.DB, s string) (*gnucash.Account, error) {
	var account *gnucash.Account
	var err error
	account, err = gnucash.Accounts(qm.Where("guid=?", s)).One(ctx, db)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return getAccountTree(ctx, db, s)
		default:
			return account, err
		}
	}
	return account, nil
}

// getAccountTree accepts a string which represents a case insensitive absolute
// account name in gnucash syntax (e.g. expenses:automotive:petrol) and returns
// the final child account.
func getAccountTree(ctx context.Context, db *sql.DB, s string) (*gnucash.Account, error) {
	var account *gnucash.Account
	var err error
	accounts := strings.Split(s, ":")

	rootAccount, err := gnucash.Accounts(qm.Where("account_type=? AND name=? AND parent_guid IS NULL", "ROOT", "Root Account")).One(ctx, db)
	if err != nil {
		return account, fmt.Errorf("failed to find root account: %w", err)
	}

	parentAccounts := []*gnucash.Account{rootAccount}
	for idx, accountName := range accounts {
		parentAccount := parentAccounts[idx]
		account, err := gnucash.Accounts(qm.Where("name=? COLLATE NOCASE AND parent_guid=?", accountName, parentAccount.GUID)).One(ctx, db)
		if err != nil {
			return account, err
		}
		parentAccounts = append(parentAccounts, account)
	}

	if len(parentAccounts) != len(accounts)+1 {
		return account, fmt.Errorf("failed to correctly find account from tree")
	}

	account = parentAccounts[len(parentAccounts)-1]
	return account, nil
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
					account, err = getAccountTree(cmd.Context(), db, guidOrAccountName)
					if err != nil {
						return err
					}
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
