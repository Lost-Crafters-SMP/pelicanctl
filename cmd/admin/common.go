package admin

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

// getOutputFormat gets the output format from command flags.
func getOutputFormat(cmd *cobra.Command) output.OutputFormat {
	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		return output.OutputFormatJSON
	}
	return output.OutputFormatTable
}

// runListCommand handles the common pattern for list operations.
func runListCommand(
	cmd *cobra.Command,
	client *api.ApplicationAPI,
	listFunc func(*api.ApplicationAPI) (any, error),
	resourceType output.ResourceType,
) error {
	items, err := listFunc(client)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(items, resourceType)
}

// runViewCommand handles the common pattern for view operations.
func runViewCommand(
	cmd *cobra.Command,
	id string,
	client *api.ApplicationAPI,
	viewFunc func(*api.ApplicationAPI, string) (any, error),
) error {
	item, err := viewFunc(client, id)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.Print(item)
}

// makeListRunE creates a RunE function that handles client creation and list operations.
func makeListRunE(
	listFunc func(*api.ApplicationAPI) (any, error),
	resourceType output.ResourceType,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		client, err := api.NewApplicationAPI()
		if err != nil {
			return err
		}
		return runListCommand(cmd, client, listFunc, resourceType)
	}
}

// makeViewRunE creates a RunE function that handles client creation and view operations.
func makeViewRunE(viewFunc func(*api.ApplicationAPI, string) (any, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		id := args[0]
		client, err := api.NewApplicationAPI()
		if err != nil {
			return err
		}
		return runViewCommand(cmd, id, client, viewFunc)
	}
}

type resourceCommandConfig struct {
	name      string
	short     string
	long      string
	listShort string
	listRunE  func(*cobra.Command, []string) error
	viewUse   string
	viewShort string
	viewRunE  func(*cobra.Command, []string) error
}

func newResourceCmd(config resourceCommandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   config.name,
		Short: config.short,
		Long:  config.long,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: config.listShort,
		RunE:  config.listRunE,
	}

	viewCmd := &cobra.Command{
		Use:   config.viewUse,
		Short: config.viewShort,
		Args:  cobra.ExactArgs(1),
		RunE:  config.viewRunE,
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)

	return cmd
}
