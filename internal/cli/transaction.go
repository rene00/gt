package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gt/internal/types"
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
			db, err := sql.Open("sqlite3", cli.config.GnucashDBFile)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			tx, err := db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			sourceAccount := &gnucash.Account{}
			if flags.SourceAccount != "" {
				sourceAccount, err = gnucash.Accounts(qm.Where("guid=?", flags.SourceAccount)).One(cmd.Context(), db)
				if err != nil {
					return err
				}
			}

			destinationAccount := &gnucash.Account{}
			if flags.DestinationAccount != "" {
				destinationAccount, err = gnucash.Accounts(qm.Where("guid=?", flags.DestinationAccount)).One(cmd.Context(), db)
				if err != nil {
					return err
				}
			}

			var q []qm.QueryMod

			if flags.DescriptionLike != "" {
				q = append(q, qm.Where("transactions.description LIKE ?", flags.DescriptionLike))
			}

			gTransactions, err := gnucash.Transactions(q...).All(cmd.Context(), db)
			if err != nil {
				return err
			}

			transactions := []types.Transaction{}
			for _, gTransaction := range gTransactions {
				splits, err := gnucash.Splits(qm.Where("tx_guid=?", gTransaction.GUID)).All(cmd.Context(), db)
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
						transaction, err := types.NewTransaction(cmd.Context(), db, *gTransaction)
						if err != nil {
							return err
						}
						transactions = append(transactions, transaction)
					}
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			resp, err := json.MarshalIndent(transactions, "", "    ")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.SourceAccount, "source-account", "", "Source Account GUID")
	cmd.Flags().StringVar(&flags.DestinationAccount, "destination-account", "", "Destination Account GUID")
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

			db, err := sql.Open("sqlite3", cli.config.GnucashDBFile)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			tx, err := db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			sourceAccount := &gnucash.Account{}
			if flags.SourceAccount != "" {
				sourceAccount, err = gnucash.Accounts(qm.Where("guid=?", flags.SourceAccount)).One(cmd.Context(), db)
				if err != nil {
					return err
				}
			}

			destinationAccount := &gnucash.Account{}
			if flags.DestinationAccount != "" {
				destinationAccount, err = gnucash.Accounts(qm.Where("guid=?", flags.DestinationAccount)).One(cmd.Context(), db)
				if err != nil {
					return err
				}
			}

			gTransaction, err := gnucash.Transactions(qm.Where("guid=?", guid)).One(cmd.Context(), db)
			if err != nil {
				return err
			}

			splits, err := gnucash.Splits(qm.Where("tx_guid=?", gTransaction.GUID)).All(cmd.Context(), db)
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

			transaction, err := types.NewTransaction(cmd.Context(), db, *gTransaction)
			if err != nil {
				return err
			}

			resp, err := json.MarshalIndent(transaction, "", "    ")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.SourceAccount, "source-account", "", "Source Account GUID")
	cmd.Flags().StringVar(&flags.DestinationAccount, "destination-account", "", "Destination Account GUID")
	return cmd
}

func listTransactionCmd(cli *cli) *cobra.Command {
	var flags struct {
		Account         string
		Limit           int
		StartPostDate   string
		DescriptionLike string
	}
	var cmd = &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open("sqlite3", cli.config.GnucashDBFile)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			unbounded := false
			q := []qm.QueryMod{}

			gAccount := &gnucash.Account{}
			if flags.Account != "" {
				gAccount, err = gnucash.Accounts(qm.Where("guid=?", flags.Account)).One(cmd.Context(), db)
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

			if flags.DescriptionLike != "" {
				q = append(q, qm.Where("transactions.description LIKE ?", flags.DescriptionLike))
			}

			gTransactions := gnucash.TransactionSlice{}
			if flags.Account != "" {
				q = append(q, []qm.QueryMod{
					qm.Where("splits.account_guid=?", gAccount.GUID),
					qm.InnerJoin("transactions ON splits.tx_guid = transactions.guid"),
					qm.OrderBy("transactions.post_date"),
				}...)
				splits, err := gnucash.Splits(q...).All(cmd.Context(), db)
				if err != nil {
					return err
				}
				for _, split := range splits {
					gTransaction, err := gnucash.Transactions(qm.Where("guid=?", split.TXGUID)).One(cmd.Context(), db)
					if err != nil {
						return err
					}
					if len(gTransactions) < limit {
						gTransactions = append(gTransactions, gTransaction)
					}
				}
			} else {
				gTransactions, err = gnucash.Transactions(q...).All(cmd.Context(), db)
				if err != nil {
					return err
				}
			}

			transactions := []types.Transaction{}
			for _, gTransaction := range gTransactions {
				transaction, err := types.NewTransaction(cmd.Context(), db, *gTransaction)
				if err != nil {
					return err
				}
				transactions = append(transactions, transaction)
			}

			resp, err := json.MarshalIndent(transactions, "", "    ")
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

			db, err := sql.Open("sqlite3", GnuCashSqliteURI)
			if err != nil {
				return err
			}
			boil.SetDB(db)

			guid := args[0]
			transaction, err := gnucash.Transactions(qm.Where("guid=?", guid)).One(cmd.Context(), db)
			if err != nil {
				return err
			}
			resp, err := json.MarshalIndent(transaction, "", "    ")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
	return cmd
}
