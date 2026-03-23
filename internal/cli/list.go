package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all keys in a vault",
		Long:    "Lists all key names stored in the active vault (or --vault). Values are not shown.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := app.resolveVault(cmd)
			if err != nil {
				return err
			}

			keys, err := app.Store.List(vault)
			if err != nil {
				return fmt.Errorf("failed to list keys in vault %q: %w", vault, err)
			}

			if len(keys) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No keys in vault %q.\n", vault)
				return nil
			}

			for _, k := range keys {
				fmt.Fprintln(cmd.OutOrStdout(), k)
			}
			return nil
		},
	}
}
