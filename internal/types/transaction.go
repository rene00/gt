package types

import (
	"context"
	"database/sql"
	"gt/models/gnucash"

	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type Transaction struct {
	GUID         string  `boil:"guid" json:"guid" toml:"guid" yaml:"guid"`
	CurrencyGUID string  `boil:"currency_guid" json:"currency_guid" toml:"currency_guid" yaml:"currency_guid"`
	Num          string  `boil:"num" json:"num" toml:"num" yaml:"num"`
	PostDate     string  `boil:"post_date" json:"post_date,omitempty" toml:"post_date" yaml:"post_date,omitempty"`
	EnterDate    string  `boil:"enter_date" json:"enter_date,omitempty" toml:"enter_date" yaml:"enter_date,omitempty"`
	Description  string  `boil:"description" json:"description,omitempty" toml:"description" yaml:"description,omitempty"`
	Splits       []Split `json:"splits,omitempty"`
}

func NewTransaction(ctx context.Context, db *sql.DB, t gnucash.Transaction) (Transaction, error) {
	transaction := Transaction{
		GUID:         t.GUID,
		CurrencyGUID: t.CurrencyGUID,
		Num:          t.Num,
		PostDate:     t.PostDate.String,
		EnterDate:    t.EnterDate.String,
		Description:  t.Description.String,
		Splits:       []Split{},
	}

	splits, err := gnucash.Splits(qm.Where("tx_guid=?", transaction.GUID)).All(ctx, db)
	if err != nil {
		return transaction, err
	}

	for _, split := range splits {
		transaction.Splits = append(transaction.Splits,
			Split{
				GUID:           split.GUID,
				TXGUID:         split.TXGUID,
				AccountGUID:    split.AccountGUID,
				Memo:           split.Memo,
				Action:         split.Action,
				ReconcileState: split.ReconcileState,
				ReconcileDate:  split.ReconcileDate.String,
				ValueNum:       split.ValueNum,
				ValueDenom:     split.ValueDenom,
				QuantityNum:    split.QuantityNum,
				QuantityDenom:  split.QuantityDenom,
				LotGUID:        split.LotGUID.String,
			},
		)
	}

	return transaction, nil
}

type Split struct {
	GUID           string `json:"guid"`
	TXGUID         string `json:"tx_guid"`
	AccountGUID    string `json:"account_guid"`
	Memo           string `json:"memo"`
	Action         string `json:"action"`
	ReconcileState string `json:"reconcile_state"`
	ReconcileDate  string `json:"reconcile_date,omitempty"`
	ValueNum       int64  `json:"value_num"`
	ValueDenom     int64  `json:"value_denom"`
	QuantityNum    int64  `json:"quantity_num"`
	QuantityDenom  int64  `json:"quantity_denom"`
	LotGUID        string `json:"lot_guid,omitempty"`
}
