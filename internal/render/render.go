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
	Render(w io.Writer, data any) error
}

type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

func New(format string) (Renderer, error) {
	switch Format(format) {
	case FormatJSON:
		return &JSONRenderer{}, nil
	case FormatTable:
		return &TableRenderer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

type JSONRenderer struct{}

func (j *JSONRenderer) Render(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	return encoder.Encode(data)
}

type TableRenderer struct{}

func (t *TableRenderer) Render(w io.Writer, data any) error {

	var transactions []*store.Transaction

	switch v := data.(type) {
	case *store.Transaction:
		transactions = []*store.Transaction{v}
	case []*store.Transaction:
		transactions = v
	default:
		return fmt.Errorf("unsupported model type: %T", data)
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
	table.Header([]string{"Date", "Description", "Account", "Debit", "Credit"})

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
				"  " + split.Account.Name,
				debit,
				credit,
			})
		}

		table.Append([]string{"", "", "", "", ""})
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
