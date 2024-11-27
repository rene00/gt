package cli

import (
	"context"
	"database/sql"
	"fmt"
	"gt/internal/marshal"
	"gt/models/gnucash"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func accountCmd(cli *cli) *cobra.Command {
	var cmd = &cobra.Command{
		Use: "account",
	}
	cmd.AddCommand(getAccountCmd(cli))
	cmd.AddCommand(listAccountCmd(cli))
	cmd.AddCommand(updateAccountCmd(cli))
	return cmd
}

// getAccountFromAccountTreeString accepts a string which represents a case insensitive absolute
// account name in gnucash syntax (e.g. expenses:automotive:petrol) and returns
// the final child account.
func getAccountFromAccountTreeString(ctx context.Context, db *sql.DB, s string) (*gnucash.Account, error) {
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

func updateAccountCmd(cli *cli) *cobra.Command {
	var flags struct {
		ParentAccount string
		Name          string
		Description   string
	}
	var cmd = &cobra.Command{
		Use: "update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return ErrAccountMissing
			}
			guidOrAccountName := args[0]

			account, err := cli.getAccountFromGUIDOrAccountTree(cmd.Context(), guidOrAccountName)
			if err != nil {
				return ErrAccountDoesNotExist
			}

			columns := []string{}

			parentAccount := &gnucash.Account{}
			if flags.ParentAccount != "" {
				parentAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.ParentAccount)
				if err != nil {
					return err
				}

				if parentAccount.GUID != "" {
					account.ParentGUID = null.StringFrom(parentAccount.GUID)
					columns = append(columns, "parent_guid")
				}
			}

			// TODO(rene): must check that name is a valid is name a valid
			// child account name (i.e. does not contain ":").
			if flags.Name != "" {
				account.Name = flags.Name
				exists, err := cli.accountExists(cmd.Context(), account)
				if err != nil {
					return err
				}
				if exists {
					return ErrAccountAlreadyExists
				}
				columns = append(columns, "name")
			}

			if flags.Description != "" {
				account.Description = null.StringFrom(flags.Description)
				columns = append(columns, "description")
			}

			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			if _, err := account.Update(cmd.Context(), tx, boil.Whitelist(columns...)); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			resp, err := marshal.NewAccountMarshal(account).JSON()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), string(resp))

			return nil
		},
	}
	cmd.Flags().StringVar(&flags.ParentAccount, "parent-account", "", "Parent Account")
	cmd.Flags().StringVar(&flags.Name, "name", "", "Account Name")
	cmd.Flags().StringVar(&flags.Description, "description", "", "Account Description")
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

			account, err := cli.getAccountFromGUIDOrAccountTree(cmd.Context(), guidOrAccountName)
			if err != nil {
				return err
			}

			resp, err := marshal.NewAccountMarshal(account).JSON()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), string(resp))
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

			if flags.NameLike != "" {
				q = append(q, qm.Where("name LIKE ?", flags.NameLike))
			}

			accounts, err := gnucash.Accounts(q...).All(cmd.Context(), cli.db)
			if err != nil {
				return err
			}

			resp, err := marshal.NewAccountsMarshal(accounts).JSON()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.NameLike, "name-like", "", "Name like")
	return cmd
}
