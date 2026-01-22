package client

import (
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
	"go.lostcrafters.com/pelicanctl/internal/output"
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
	listCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	createCmd := &cobra.Command{
		Use:   "create <id|uuid>",
		Short: "Create a backup for a server",
		Long:  "Create a backup for a server by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runBackupCreate,
	}
	createCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	// Add subcommands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	carapace.Gen(listCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("client", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(createCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("client", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

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
