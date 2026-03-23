package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMigrateCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate secrets from an external Keychain service into a kc vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			from, _ := cmd.Flags().GetString("from")
			if from == "" {
				return fmt.Errorf("migrate: --from flag is required")
			}
			vault, err := app.resolveVault(cmd)
			if err != nil {
				return err
			}
			entries, err := app.Bulk.ReadRawService(from)
			if err != nil {
				return fmt.Errorf("migrate: read source %q: %w", from, err)
			}
			n, err := app.Bulk.BulkSet(entries, vault)
			if err != nil {
				return fmt.Errorf("migrate: write to vault %q: %w", vault, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Migrated %d key(s) from %q into vault %q.\n", n, from, vault)
			return nil
		},
	}
	cmd.Flags().String("from", "", "source Keychain service name to migrate from")
	return cmd
}
