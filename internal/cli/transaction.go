package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"gt/internal/marshal"
	"gt/internal/render"
	"gt/internal/store"
	"gt/models/gnucash"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
		DescriptionLike    string
		SourceAccount      string
		DestinationAccount string
	}
	var cmd = &cobra.Command{
		Use: "bulk-update",
		RunE: func(cmd *cobra.Command, args []string) error {
			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			sourceAccount := &gnucash.Account{}
			if flags.SourceAccount != "" {
				sourceAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.SourceAccount)
				if err != nil {
					return err
				}
			}

			destinationAccount := &gnucash.Account{}
			if flags.DestinationAccount != "" {
				destinationAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.DestinationAccount)
				if err != nil {
					return err
				}
			}

			var q []qm.QueryMod

			if flags.DescriptionLike != "" {
				q = append(q, qm.Where("transactions.description LIKE ?", flags.DescriptionLike))
			}

			gTransactions, err := gnucash.Transactions(q...).All(cmd.Context(), cli.db)
			if err != nil {
				return err
			}

			transactions := gnucash.TransactionSlice{}
			splits := []*gnucash.Split{}
			for _, transaction := range gTransactions {
				gSplits, err := gnucash.Splits(qm.Where("tx_guid=?", transaction.GUID)).All(cmd.Context(), cli.db)
				if err != nil {
					return err
				}
				for _, split := range gSplits {
					if split.AccountGUID == sourceAccount.GUID {
						split.AccountGUID = destinationAccount.GUID
						_, err := split.Update(cmd.Context(), tx, boil.Infer())
						if err != nil {
							return err
						}
						transactions = append(transactions, transaction)
						splits = append(splits, split)
					}
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			resp, err := marshal.NewTransactionsMarshal(transactions).JSON()
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.SourceAccount, "source-account", "", "Source Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.DestinationAccount, "destination-account", "", "Destination Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.DescriptionLike, "description-like", "", "Description like")
	return cmd
}

func updateTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		SourceAccount      string
		DestinationAccount string
	}
	var cmd = &cobra.Command{
		Use: "update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("missing transaction guid")
			}
			guid := args[0]

			tx, err := cli.db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			sourceAccount := &gnucash.Account{}
			if flags.SourceAccount != "" {
				sourceAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.SourceAccount)
				if err != nil {
					return err
				}
			}

			destinationAccount := &gnucash.Account{}
			if flags.DestinationAccount != "" {
				destinationAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.DestinationAccount)
				if err != nil {
					return err
				}
			}

			transaction, err := gnucash.Transactions(qm.Where("guid=?", guid)).One(cmd.Context(), cli.db)
			if err != nil {
				return err
			}

			splits, err := gnucash.Splits(qm.Where("tx_guid=?", transaction.GUID)).All(cmd.Context(), cli.db)
			if err != nil {
				return err
			}

			for _, split := range splits {
				if split.AccountGUID == sourceAccount.GUID {
					split.AccountGUID = destinationAccount.GUID
					_, err := split.Update(cmd.Context(), tx, boil.Infer())
					if err != nil {
						return err
					}
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			resp, err := marshal.NewTransactionMarshal(transaction, marshal.TransactionMarshalWithSplits(splits)).JSON()
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.SourceAccount, "source-account", "", "Source Account GUID or Full Account Name")
	cmd.Flags().StringVar(&flags.DestinationAccount, "destination-account", "", "Destination Account GUID or Full Account Name")
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

			limit := 50
			if flags.limit != 0 {
				limit = flags.limit
			}

			transactionQuery.Limit(limit)

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

			return r.Render(cmd.OutOrStdout(), transactions)
		},
	}

	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Limit")
	cmd.Flags().StringVar(&flags.account, "account", "", "Account GUID")
	cmd.Flags().StringVar(&flags.startPostDate, "start-post-date", "", "Start Post Date")
	cmd.Flags().StringVar(&flags.endPostDate, "end-post-date", "", "Start Post Date")
	cmd.Flags().BoolVar(&flags.orderByPostDate, "order-by-post-date", false, "Order by Post Date")
	cmd.Flags().BoolVar(&flags.orderDescending, "order-descending", false, "Order Descending")
	cmd.Flags().StringVar(&flags.descriptionLike, "description-like", "", "Description like")
	cmd.Flags().StringVar(&flags.output, "output", "table", "Output format")
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
			fmt.Println(string(transaction.GUID))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.output, "output", "table", "Output format")
	return cmd
}
