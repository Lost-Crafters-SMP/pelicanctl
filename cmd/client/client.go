// Package client provides commands for managing servers using the Client API.
package client

import (
	"github.com/spf13/cobra"
)

// NewClientCmd creates the client command group.
func NewClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client API commands",
		Long:  "Commands for managing servers using the Client API (end-user operations)",
	}

	// Add subcommands
	cmd.AddCommand(newServerCmd())
	cmd.AddCommand(newFileCmd())
	cmd.AddCommand(newBackupCmd())
	cmd.AddCommand(newDatabaseCmd())
	cmd.AddCommand(newPowerCmd())

	return cmd
}
