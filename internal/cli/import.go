package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newImportCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "import FILE",
		Short: "Import KEY=VALUE pairs from a .env file into the active vault",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := app.resolveVault(cmd)
			if err != nil {
				return err
			}
			path := args[0]
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("import: cannot open %q: %w", path, err)
			}
			defer f.Close()

			entries := parseEnvReader(f)
			n, err := app.Bulk.BulkSet(entries, vault)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported %d keys into vault %s\n", n, vault)
			return nil
		},
	}
}
