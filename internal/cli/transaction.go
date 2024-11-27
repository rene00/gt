package cli

import (
	"fmt"
	"gt/internal/marshal"
	"gt/models/gnucash"
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
		Account         string
		Limit           int
		StartPostDate   string
		EndPostDate     string
		DescriptionLike string
	}
	var cmd = &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			unbounded := false
			q := []qm.QueryMod{}

			gAccount := &gnucash.Account{}
			if flags.Account != "" {
				gAccount, err = cli.getAccountFromGUIDOrAccountTree(cmd.Context(), flags.Account)
				if err != nil {
					return err
				}
				unbounded = true
			}

			limit := 50
			if flags.Limit != 0 {
				limit = flags.Limit
			}
			if !unbounded {
				q = append(q, qm.Limit(limit))
			}

			if flags.StartPostDate != "" {
				startPostDate, err := time.Parse("2006-01-02", flags.StartPostDate)
				if err != nil {
					return err
				}
				q = append(q, qm.Where("transactions.post_date>=?", startPostDate.Format("2006-01-02")))
			}

			if flags.EndPostDate != "" {
				endPostDate, err := time.Parse("2006-01-02", flags.EndPostDate)
				if err != nil {
					return err
				}
				q = append(q, qm.Where("transactions.post_date<=?", endPostDate.Format("2006-01-02")))
			}

			if flags.DescriptionLike != "" {
				q = append(q, qm.Where("transactions.description LIKE ?", flags.DescriptionLike))
			}

			transactions := gnucash.TransactionSlice{}
			splits := []*gnucash.Split{}
			if flags.Account != "" {
				q = append(q, []qm.QueryMod{
					qm.Where("splits.account_guid=?", gAccount.GUID),
					qm.InnerJoin("transactions ON splits.tx_guid = transactions.guid"),
					qm.OrderBy("transactions.post_date"),
				}...)
				splits, err = gnucash.Splits(q...).All(cmd.Context(), cli.db)
				if err != nil {
					return err
				}

				for _, split := range splits {
					transaction, err := gnucash.Transactions(qm.Where("guid=?", split.TXGUID)).One(cmd.Context(), cli.db)
					if err != nil {
						return err
					}
					if len(transactions) == limit {
						break
					}
					transactions = append(transactions, transaction)
				}
			} else {
				transactions, err = gnucash.Transactions(q...).All(cmd.Context(), cli.db)
				if err != nil {
					return err
				}
			}

			resp, err := marshal.NewTransactionsMarshal(transactions).JSON()
			if err != nil {
				return err
			}

			fmt.Println(string(resp))
			return nil
		},
	}

	cmd.Flags().IntVar(&flags.Limit, "limit", 0, "Limit")
	cmd.Flags().StringVar(&flags.Account, "account", "", "Account GUID")
	cmd.Flags().StringVar(&flags.StartPostDate, "start-post-date", "", "Start Post Date")
	cmd.Flags().StringVar(&flags.DescriptionLike, "description-like", "", "Description like")
	return cmd
}

func getTransactionCmd(cli *cli) *cobra.Command {
	var cmd = &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("missing transaction guid")
			}

			guid := args[0]
			transaction, err := gnucash.Transactions(qm.Where("guid=?", guid)).One(cmd.Context(), cli.db)
			if err != nil {
				return err
			}

			splits, err := gnucash.Splits(qm.Where("tx_guid=?", transaction.GUID)).All(cmd.Context(), cli.db)
			if err != nil {
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
	return cmd
}
