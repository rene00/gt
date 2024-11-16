package marshal

import (
	"encoding/json"
	"gt/models/gnucash"
)

func NewAccountMarshal(account *gnucash.Account) Marshal {
	return AccountMarshal{account: account}
}

type AccountMarshal struct {
	account *gnucash.Account
}

func (a AccountMarshal) JSON() ([]byte, error) {
	return json.MarshalIndent(a.account, "", "  ")
}

type AccountsMarshal struct {
	accounts []*gnucash.Account
}

func NewAccountsMarshal(accounts []*gnucash.Account) Marshal {
	return AccountsMarshal{accounts: accounts}
}

func (a AccountsMarshal) JSON() ([]byte, error) {
	return json.MarshalIndent(a.accounts, "", "  ")
}
