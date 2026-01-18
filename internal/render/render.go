package render

import (
	"encoding/json"
	"fmt"
	"gt/internal/store"
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

type Renderer interface {
	Render(w io.Writer, data any, opts ...RendererOptsFunc) error
}

type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

func New(format string, opts ...RendererOptsFunc) (Renderer, error) {
	switch Format(format) {
	case FormatJSON:
		return &JSONRenderer{}, nil
	case FormatTable:
		return &TableRenderer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

type JSONRenderer struct {
	opts *RendererOpts
}

func (j *JSONRenderer) Render(w io.Writer, data any, _ ...RendererOptsFunc) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	return encoder.Encode(data)
}

type TableRenderer struct{}

type RendererOpts struct {
	includeTotals bool

	// use accounts short name (i.e. pizza) when rendering instead of full name
	// (i.e expenses:dining:pizza)
	accountShortName bool
}

func defaultRendererOpts() *RendererOpts {
	return &RendererOpts{
		includeTotals:    false,
		accountShortName: false,
	}
}

type RendererOptsFunc func(*RendererOpts)

func WithAccountShortName(b bool) RendererOptsFunc {
	return func(o *RendererOpts) {
		o.accountShortName = b
	}
}

func WithIncludeTotals(b bool) RendererOptsFunc {
	return func(o *RendererOpts) {
		o.includeTotals = b
	}
}

func renderAccounts(table *tablewriter.Table, opts RendererOpts, accounts []*store.Account) {
	table.Header([]string{"Name", "Account Type", "Description"})
	for _, account := range accounts {
		name := account.FullName
		if opts.accountShortName {
			name = account.Name
		}

		description := ""
		if account.Description != nil {
			description = *account.Description
		}

		table.Append([]string{
			name,
			account.AccountType,
			description,
		})
	}
}

func renderTransactions(table *tablewriter.Table, opts RendererOpts, transactions []*store.Transaction) {
	table.Header([]string{"Date", "Description", "Account", "Debit", "Credit"})

	type AccountTotal struct {
		Name       string
		TotalNum   int64
		TotalDenom int64
	}
	accountTotals := make(map[string]*AccountTotal)

	for _, transaction := range transactions {
		if len(transaction.Splits) == 0 {
			continue
		}

		description := ""
		if transaction.Description != nil {
			description = *transaction.Description
		}

		table.Append([]string{
			transaction.PostDate.Local().Format("2006-01-02"),
			description,
			"",
			"",
			"",
		})

		for _, split := range transaction.Splits {
			debit, credit := formatAmount(split.ValueNum, split.ValueDenom)
			table.Append([]string{
				"",
				"",
				split.Account.Name,
				debit,
				credit,
			})

			accountGUID := split.AccountGUID
			if _, exists := accountTotals[accountGUID]; !exists {
				accountTotals[accountGUID] = &AccountTotal{
					Name:       split.Account.Name,
					TotalNum:   0,
					TotalDenom: split.ValueDenom,
				}
			}
			accountTotals[accountGUID].TotalNum += split.ValueNum
		}

		table.Append([]string{"", "", "", "", ""})
	}

	if len(accountTotals) > 0 && opts.includeTotals {
		table.Append([]string{"", "TOTALS", "", ""})

		type sortableTotal struct {
			name  string
			total *AccountTotal
		}

		sortedAccounts := make([]sortableTotal, 0, len(accountTotals))
		for _, total := range accountTotals {
			sortedAccounts = append(sortedAccounts, sortableTotal{name: total.Name, total: total})
		}

		for i := 0; i < len(sortedAccounts); i++ {
			for j := i + 1; j < len(sortedAccounts); j++ {
				if sortedAccounts[i].name > sortedAccounts[j].name {
					sortedAccounts[i], sortedAccounts[j] = sortedAccounts[j], sortedAccounts[i]
				}
			}
		}

		for _, sortedAccount := range sortedAccounts {
			debit, credit := formatAmount(sortedAccount.total.TotalNum, sortedAccount.total.TotalDenom)
			table.Append([]string{
				"",
				"",
				sortedAccount.name,
				debit,
				credit,
			})
		}
	}
}

func (t *TableRenderer) Render(w io.Writer, data any, opts ...RendererOptsFunc) error {

	o := defaultRendererOpts()
	for _, fn := range opts {
		fn(o)
	}

	cfg := tablewriter.Config{
		Header: tw.CellConfig{
			Alignment: tw.CellAlignment{
				Global: tw.AlignLeft,
			},
		},
		Row: tw.CellConfig{
			Formatting: tw.CellFormatting{
				AutoWrap: int(tw.Off),
			},
		},
	}

	table := tablewriter.NewTable(w, tablewriter.WithConfig(cfg))
	switch v := data.(type) {
	case []*store.Account:
		renderAccounts(table, *o, v)
	case *store.Account:
		renderAccounts(table, *o, []*store.Account{v})
	case *store.Transaction:
		renderTransactions(table, *o, []*store.Transaction{v})
	case []*store.Transaction:
		renderTransactions(table, *o, v)
	default:
		return fmt.Errorf("unsupported model type: %T", data)
	}

	return table.Render()
}

func formatAmount(valueNum, valueDenom int64) (debit, credit string) {
	if valueDenom == 0 {
		return "", ""
	}

	amount := float64(valueNum) / float64(valueDenom)
	if amount < 0 {
		return "", fmt.Sprintf("%.2f", amount)
	}

	if amount > 0 {
		return fmt.Sprintf("%.2f", amount), ""
	}

	return "", ""
}
