package client

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func newDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Manage server databases",
		Long:  "List databases for a server",
	}

	listCmd := &cobra.Command{
		Use:   "list <id|uuid>",
		Short: "List databases for a server",
		Long:  "List databases for a server by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runDatabaseList,
	}

	cmd.AddCommand(listCmd)
	return cmd
}

func runDatabaseList(cmd *cobra.Command, args []string) error {
	serverUUID := args[0]

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	databases, err := client.ListDatabases(serverUUID)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(databases, output.ResourceTypeClientDatabase)
}
