package client

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage server backups",
		Long:  "List and create server backups",
	}

	listCmd := &cobra.Command{
		Use:   "list <id|uuid>",
		Short: "List backups for a server",
		Long:  "List backups for a server by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runBackupList,
	}

	createCmd := &cobra.Command{
		Use:   "create <id|uuid>",
		Short: "Create a backup for a server",
		Long:  "Create a backup for a server by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runBackupCreate,
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)

	return cmd
}

func runBackupList(cmd *cobra.Command, args []string) error {
	serverUUID := args[0]

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	backups, err := client.ListBackups(serverUUID)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(backups, output.ResourceTypeClientBackup)
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	serverUUID := args[0]

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	backup, err := client.CreateBackup(serverUUID)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("Backup created successfully")
	return formatter.Print(backup)
}
