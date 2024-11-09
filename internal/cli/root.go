package cli

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
)

func Execute() {
	cli := &cli{}
	rootCmd := &cobra.Command{
		Use: "gt",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.setup()
		},
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("failed to find home dir: %s", err.Error()))
		os.Exit(1)
	}

	rootCmd.PersistentFlags().BoolVar(&cli.debug, "debug", false, "Enable debug")
	rootCmd.PersistentFlags().StringVar(&cli.configFile, "config-file", path.Join(homeDir, ".gt.json"), "Config file")

	rootCmd.AddCommand(accountCmd(cli))
	rootCmd.AddCommand(transactionCmd(cli))

	if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
