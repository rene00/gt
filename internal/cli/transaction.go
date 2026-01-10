package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"gt/internal/render"
	"gt/internal/store"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func transactionCmd(cli *cli) *cobra.Command {
	var cmd = &cobra.Command{
		Use: "transaction",
	}
	cmd.AddCommand(bulkUpdateTransactionCmd(cli))
	cmd.AddCommand(updateTransactionCmd(cli))
	cmd.AddCommand(getTransactionCmd(cli))
	cmd.AddCommand(listTransactionCmd(cli))
	return cmd
}

func bulkUpdateTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		descriptionLike    string
		sourceAccount      string
		destinationAccount string
		output             string
	}
	var cmd = &cobra.Command{
		Use: "bulk-update",
		RunE: func(cmd *cobra.Command, args []string) error {
			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			s := store.NewStore(cli.db)
			txStore := s.WithTx(tx)

			sourceAccount := &store.Account{}
			if flags.sourceAccount != "" {
				sourceAccount, err = txStore.Accounts.Get(cmd.Context(), flags.sourceAccount)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						sourceAccount, err = txStore.Accounts.Get(cmd.Context(), flags.sourceAccount, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return accountError(err)
						}
					} else {
						return accountError(err)
					}
				}
			}

			destinationAccount := &store.Account{}
			if flags.destinationAccount != "" {
				destinationAccount, err = txStore.Accounts.Get(cmd.Context(), flags.destinationAccount)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						destinationAccount, err = txStore.Accounts.Get(cmd.Context(), flags.destinationAccount, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return accountError(err)
						}
					} else {
						return accountError(err)
					}
				}
			}

			q := store.NewTransactionQuery()
			if flags.descriptionLike != "" {
				q.Where("transactions.description LIKE ?", flags.descriptionLike)
			}

			transactions, err := txStore.Transactions.All(cmd.Context(), q)
			if err != nil {
				return err
			}

			for _, transaction := range transactions {
				for _, split := range transaction.Splits {
					if split.AccountGUID == sourceAccount.GUID && destinationAccount.GUID != "" {
						split.AccountGUID = destinationAccount.GUID
						split.Account = destinationAccount

						if err := txStore.Splits.Update(cmd.Context(), split); err != nil {
							return err
						}
					}
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			return r.Render(cmd.OutOrStderr(), transactions)
		},
	}
	cmd.Flags().StringVar(&flags.sourceAccount, "source-account", "", "Source Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.destinationAccount, "destination-account", "", "Destination Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.descriptionLike, "description-like", "", "Description like")
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	return cmd
}

func updateTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		sourceAccount      string
		destinationAccount string
		output             string
	}
	var cmd = &cobra.Command{
		Use: "update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return ErrTransactionMissing
			}
			guid := args[0]

			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			s := store.NewStore(cli.db)
			txStore := s.WithTx(tx)

			transaction, err := txStore.Transactions.Get(cmd.Context(), guid)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrTransactionNotFound
				}
				return err
			}

			sourceAccount := &store.Account{}
			if flags.sourceAccount != "" {
				sourceAccount, err = txStore.Accounts.Get(cmd.Context(), flags.sourceAccount)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						sourceAccount, err = txStore.Accounts.Get(cmd.Context(), flags.sourceAccount, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return ErrAccountDoesNotExist
						}
					} else {
						return ErrAccountDoesNotExist
					}
				}
			}

			destinationAccount := &store.Account{}
			if flags.destinationAccount != "" {
				destinationAccount, err = txStore.Accounts.Get(cmd.Context(), flags.destinationAccount)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						destinationAccount, err = txStore.Accounts.Get(cmd.Context(), flags.destinationAccount, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return ErrAccountDoesNotExist
						}
					} else {
						return ErrAccountDoesNotExist
					}
				}
			}

			for _, split := range transaction.Splits {
				if split.AccountGUID == sourceAccount.GUID && destinationAccount.GUID != "" {
					split.AccountGUID = destinationAccount.GUID
					split.Account = destinationAccount
					if err := txStore.Splits.Update(cmd.Context(), split); err != nil {
						return err
					}
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			return r.Render(cmd.OutOrStdout(), transaction)
		},
	}
	cmd.Flags().StringVar(&flags.sourceAccount, "source-account", "", "Source Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.destinationAccount, "destination-account", "", "Destination Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	return cmd
}

func listTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		account         string
		limit           int
		startPostDate   string
		endPostDate     string
		descriptionLike string
		output          string
		orderByPostDate bool
		orderDescending bool
		includeTotals   bool
	}
	var cmd = &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			s := store.NewStore(cli.db)
			transactionQuery := store.NewTransactionQuery()

			var account *store.Account
			if flags.account != "" {
				account, err = s.Accounts.Get(cmd.Context(), flags.account)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						account, err = s.Accounts.Get(cmd.Context(), flags.account, []store.AccountsOptFunc{store.WithAccountTree(true)}...)
						if err != nil {
							return ErrAccountDoesNotExist
						}
					} else {
						return ErrAccountDoesNotExist
					}
				}
			}

			transactionQuery.Limit(flags.limit)

			if flags.orderByPostDate {
				transactionQuery.OrderBy("post_date", flags.orderDescending)
			}

			if flags.startPostDate != "" {
				startPostDate, err := time.Parse("2006-01-02", flags.startPostDate)
				if err != nil {
					return err
				}
				transactionQuery.Where("transactions.post_date > ?", startPostDate.Format("2006-01-02"))
			}

			if flags.endPostDate != "" {
				endPostDate, err := time.Parse("2006-01-02", flags.endPostDate)
				if err != nil {
					return err
				}
				transactionQuery.Where("transactions.post_date<=?", endPostDate.Format("2006-01-02"))
			}

			if flags.descriptionLike != "" {
				transactionQuery.Where("transactions.description LIKE ?", flags.descriptionLike)
			}

			var transactions []*store.Transaction
			if account != nil {
				// NOTE(rene): If account is not nil, user is wanting to list
				// transactions by account. To do this, we must find all splits with
				// account_guid == account then return all transactions for found
				// splits.
				splits, err := s.Splits.All(cmd.Context(), store.NewSplitQuery().Where("account_guid=?", account.GUID))
				if err != nil {
					return err
				}

				txGUIDs := make([]string, 0, len(splits))
				seenGUIDs := make(map[string]bool)
				for _, split := range splits {
					if !seenGUIDs[split.TXGUID] {
						txGUIDs = append(txGUIDs, split.TXGUID)
						seenGUIDs[split.TXGUID] = true
					}
				}

				placeholders := make([]string, len(txGUIDs))
				args := make([]any, len(txGUIDs))
				for i, guid := range txGUIDs {
					placeholders[i] = "?"
					args[i] = guid
				}

				transactions, err = s.Transactions.All(cmd.Context(), transactionQuery.Copy().Where(fmt.Sprintf("guid IN (%s)", strings.Join(placeholders, ",")), args...))
				if err != nil {
					return err
				}
			} else {
				transactions, err = s.Transactions.All(cmd.Context(), transactionQuery)
				if err != nil {
					return err
				}
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			renderOpts := []render.RendererOptsFunc{render.WithIncludeTotals(flags.includeTotals)}
			return r.Render(cmd.OutOrStdout(), transactions, renderOpts...)
		},
	}

	cmd.Flags().IntVar(&flags.limit, "limit", 50, "Limit")
	cmd.Flags().StringVar(&flags.account, "account", "", "Account GUID")
	cmd.Flags().StringVar(&flags.startPostDate, "start-post-date", "", "Start Post Date")
	cmd.Flags().StringVar(&flags.endPostDate, "end-post-date", "", "Start Post Date")
	cmd.Flags().BoolVar(&flags.orderByPostDate, "order-by-post-date", true, "Order by Post Date")
	cmd.Flags().BoolVar(&flags.orderDescending, "order-descending", false, "Order Descending")
	cmd.Flags().StringVar(&flags.descriptionLike, "description-like", "", "Description like")
	cmd.Flags().StringVar(&flags.output, "output", "table", FlagsUsageOutput)
	cmd.Flags().BoolVar(&flags.includeTotals, "include-totals", true, FlagsUsageIncludeTotals)
	return cmd
}

func getTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		output string
	}
	var cmd = &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("missing transaction guid")
			}

			guid := args[0]
			s := store.NewStore(cli.db)
			transaction, err := s.Transactions.Get(cmd.Context(), guid)
			if err != nil {
				return err
			}

			r, err := render.New(flags.output)
			if err != nil {
				return err
			}

			return r.Render(cmd.OutOrStdout(), transaction)
		},
	}
	cmd.Flags().StringVar(&flags.output, "output", "table", "Output format")
	return cmd
}
