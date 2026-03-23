package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Print shell export statements for all secrets in the active vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := app.resolveVault(cmd)
			if err != nil {
				return err
			}
			entries, err := app.Bulk.GetAll(vault)
			if err != nil {
				return fmt.Errorf("env: %w", err)
			}
			for _, k := range sortedKeys(entries) {
				fmt.Fprintf(cmd.OutOrStdout(), "export %s=%s\n", k, shellQuote(entries[k]))
			}
			return nil
		},
	}
}
