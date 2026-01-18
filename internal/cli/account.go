package cli

import (
	"database/sql"
	"errors"
	"gt/internal/render"
	"gt/internal/store"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
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

func updateAccountCmd(cli *cli) *cobra.Command {
	var flags struct {
		parentAccount string
		name          string
		description   string
		output        string
	}
	var cmd = &cobra.Command{
		Use:   "update [account]",
		Short: "Update an account",
		Args:  cobra.ExactArgs(1),
		Long: `Update an existing account with new properties.

This command allows you to modify an account's name, description, 
and parent account. You must specify the account to update 
using its GUID or full account name path.`,
		Example: `  gt account update "expenses:automotive registration" \
    --name "registration" \
    --description "auto registration" \
    --parent-account "expenses:automotive"
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			guidOrAccountName := args[0]

			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			s := store.NewStore(cli.db)
			txStore := s.WithTx(tx)

			account, err := s.Accounts.Get(cmd.Context(), guidOrAccountName)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					account, err = s.Accounts.Get(cmd.Context(), guidOrAccountName, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
					if err != nil {
						return accountError(err)
					}
				} else {
					return accountError(err)
				}
			}

			var parentAccount *store.Account
			if flags.parentAccount != "" {
				parentAccount, err = s.Accounts.Get(cmd.Context(), guidOrAccountName)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						account, err = s.Accounts.Get(cmd.Context(), guidOrAccountName, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return accountError(err)
						}
					} else {
						return accountError(err)
					}
				}
				account.ParentGUID = &parentAccount.GUID
			}

			// TODO(rene): must check that name is a valid is name a valid
			// child account name (i.e. does not contain ":").
			if flags.name != "" {
				account.Name = flags.name
				newFullName := account.FullName[:strings.LastIndex(account.FullName, ":")+1] + account.Name
				exists := true
				_, err := s.Accounts.Get(cmd.Context(), newFullName, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						exists = false
					} else {
						return err
					}
				}

				if exists {
					return ErrAccountAlreadyExists
				}
			}

			if flags.description != "" {
				account.Description = &flags.description
			}

			if err := txStore.Accounts.Update(cmd.Context(), account); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			account, err = s.Accounts.Get(cmd.Context(), account.GUID)
			if err != nil {
				return err
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			return r.Render(cmd.OutOrStderr(), account)
		},
	}
	cmd.Flags().StringVar(&flags.parentAccount, "parent-account", "", "Parent Account")
	cmd.Flags().StringVar(&flags.name, "name", "", "Account Name")
	cmd.Flags().StringVar(&flags.description, "description", "", "Account Description")
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	return cmd
}

func getAccountCmd(cli *cli) *cobra.Command {
	var flags struct {
		output string
	}
	var cmd = &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Usage()
			}

			guidOrAccountName := args[0]
			s := store.NewStore(cli.db)

			account, err := s.Accounts.Get(cmd.Context(), guidOrAccountName)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					account, err = s.Accounts.Get(cmd.Context(), guidOrAccountName, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
					if err != nil {
						return accountError(err)
					}
				} else {
					return accountError(err)
				}
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			return r.Render(cmd.OutOrStderr(), account)
		},
	}
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	return cmd
}

func listAccountCmd(cli *cli) *cobra.Command {
	var flags struct {
		limit           int
		output          string
		descriptionLike string
		nameLike        string
		shortName       bool
	}
	var cmd = &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			s := store.NewStore(cli.db)
			q := store.NewAccountQuery()
			q.Limit(flags.limit)

			if flags.nameLike != "" {
				q.Where("accounts.name LIKE ?", flags.nameLike)
			}

			if flags.descriptionLike != "" {
				q.Where("accounts.description LIKE ?", flags.descriptionLike)
			}

			accounts, err := s.Accounts.All(cmd.Context(), q)
			if err != nil {
				return err
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			renderOpts := []render.RendererOptsFunc{render.WithAccountShortName(flags.shortName)}
			return r.Render(cmd.OutOrStdout(), accounts, renderOpts...)
		},
	}
	cmd.Flags().IntVar(&flags.limit, "limit", 50, "Limit")
	cmd.Flags().StringVar(&flags.descriptionLike, "description-like", "", "Description like")
	cmd.Flags().StringVar(&flags.nameLike, "name-like", "", "Name like")
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	cmd.Flags().BoolVar(&flags.shortName, "short-name", false, FlagsUsageAccountShortName)
	return cmd
}
