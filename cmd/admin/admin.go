// Package admin provides commands for managing the Pelican panel using the Application API.
package admin

import (
	"github.com/spf13/cobra"
)

// NewAdminCmd creates the admin command group.
func NewAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Application API commands",
		Long:  "Commands for managing the Pelican panel using the Application API (admin operations)",
	}

	// Add subcommands
	cmd.AddCommand(newNodeCmd())
	cmd.AddCommand(newServerCmd())
	cmd.AddCommand(newUserCmd())

	return cmd
}
