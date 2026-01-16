package client

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
		Long:  "List, view, and manage your servers",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all servers",
		RunE:  runServerList,
	}

	viewCmd := &cobra.Command{
		Use:   "view <id|uuid>",
		Short: "View server details",
		Long:  "View server details by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runServerView,
	}

	resourcesCmd := &cobra.Command{
		Use:   "resources <id|uuid>",
		Short: "View server resource usage",
		Long:  "View server resource usage by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runServerResources,
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(resourcesCmd)

	return cmd
}

func runServerList(cmd *cobra.Command, _ []string) error {
	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	servers, err := client.ListServers()
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(servers, output.ResourceTypeClientServer)
}

func runServerView(cmd *cobra.Command, args []string) error {
	uuid := args[0]

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	server, err := client.GetServer(uuid)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.Print(server)
}

func runServerResources(cmd *cobra.Command, args []string) error {
	uuid := args[0]

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	resources, err := client.GetServerResources(uuid)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(resources, output.ResourceTypeServerResource)
}

func getOutputFormat(cmd *cobra.Command) output.OutputFormat {
	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		return output.OutputFormatJSON
	}
	return output.OutputFormatTable
}
