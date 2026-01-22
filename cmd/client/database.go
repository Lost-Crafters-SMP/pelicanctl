package client

import (
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	"go.lostcrafters.com/pelican-cli/internal/completion"
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
	listCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	// Add subcommand FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)

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
