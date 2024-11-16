package marshal

import (
	"encoding/json"
	"gt/models/gnucash"
)

type TransactionMarshal struct {
	*gnucash.Transaction
	TransactionMarshalOptions
}

type TransactionMarshalOptions struct {
	Splits []*gnucash.Split `json:"splits,omitempty"`
}

type TransactionMarshalOptionsFunc func(*TransactionMarshalOptions)

func TransactionMarshalWithSplits(s []*gnucash.Split) TransactionMarshalOptionsFunc {
	return func(opts *TransactionMarshalOptions) {
		opts.Splits = s
	}
}

func NewTransactionMarshal(transaction *gnucash.Transaction, options ...TransactionMarshalOptionsFunc) Marshal {
	opts := TransactionMarshalOptions{Splits: nil}
	t := TransactionMarshal{transaction, opts}
	for _, fn := range options {
		fn(&opts)
	}
	t.TransactionMarshalOptions = opts
	return t
}

func (t TransactionMarshal) JSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "    ")
}

type TransactionsMarshal struct {
	Transactions []TransactionMarshal `json:"transactions"`
}

// NewTransactionmarshal accepts a TransactionsSlice and returns a Marshal. It
// calls NewTransactionMarshal to build a slice of Transactions. By relying on
// TransactionMarshal, support for splits and accounts can be added to
// TransactionMarshal with TransactionsMarshal (slice) benefiting from this.
func NewTransactionsMarshal(transactions gnucash.TransactionSlice) Marshal {
	t := TransactionsMarshal{}
	for _, transaction := range transactions {
		tm := NewTransactionMarshal(transaction)
		v, _ := tm.(TransactionMarshal)
		t.Transactions = append(t.Transactions, v)
	}
	return t
}

func (t TransactionsMarshal) JSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "    ")
}
