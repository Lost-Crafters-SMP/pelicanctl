package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

const (
	// stdinBufferSize is the buffer size for reading from stdin.
	stdinBufferSize = 4096
	// backupPairPartsCount is the expected number of parts in a backup pair (server-id,backup-uuid).
	backupPairPartsCount = 2
	// minBackupViewArgs is the minimum number of arguments for backup view (server-id, backup-uuid).
	minBackupViewArgs = 2
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
	name         string
	short        string
	long         string
	listShort    string
	listRunE     func(*cobra.Command, []string) error
	viewUse      string
	viewShort    string
	viewRunE     func(*cobra.Command, []string) error
	completeFunc func(string) ([]string, error)
}

type crudResourceConfig struct {
	name          string
	short         string
	long          string
	listShort     string
	listFunc      func(*api.ApplicationAPI) (any, error)
	viewUse       string
	viewShort     string
	viewFunc      func(*api.ApplicationAPI, string) (any, error)
	createFunc    func(*api.ApplicationAPI, map[string]any) (map[string]any, error)
	updateFunc    func(*api.ApplicationAPI, string) (map[string]any, error)
	deleteFunc    func(*api.ApplicationAPI, string) error
	completeFunc  func(string) ([]string, error)
	resourceType  output.ResourceType
	createMessage string
	updateMessage string
	deleteMessage string
	createLong    string
	dataFlagHelp  string
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
	// Add completion if provided
	if config.completeFunc != nil {
		viewCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			completions, err := config.completeFunc(toComplete)
			if err != nil || len(completions) == 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// Add subcommands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	if config.completeFunc != nil {
		carapace.Gen(viewCmd).PositionalCompletion(
			carapace.ActionCallback(func(c carapace.Context) carapace.Action {
				completions, err := config.completeFunc(c.Value)
				if err != nil || len(completions) == 0 {
					return carapace.ActionValues()
				}
				return carapace.ActionValues(completions...)
			}),
		)
	}

	return cmd
}

// readStdin reads data from stdin.
func readStdin() ([]byte, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, errors.New("stdin is not a pipe or file")
	}

	data := make([]byte, 0, stdinBufferSize)
	buf := make([]byte, stdinBufferSize)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, os.ErrInvalid) {
				return nil, fmt.Errorf("failed to read from stdin: %w", err)
			}
			// EOF or other non-critical errors - return what we have
			break
		}
	}
	return data, nil
}

// parseJSONData parses JSON data from either a flag or stdin.
func parseJSONData(cmd *cobra.Command) (map[string]any, error) {
	dataFlag, _ := cmd.Flags().GetString("data")

	if dataFlag != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(dataFlag), &data); err != nil {
			return nil, fmt.Errorf("failed to parse JSON data: %w", err)
		}
		return data, nil
	}

	// Read from stdin
	data, err := readStdin()
	if err != nil {
		return nil, fmt.Errorf("failed to read from stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("no data provided. Use --data flag or provide JSON via stdin")
	}

	var result map[string]any
	if unmarshalErr := json.Unmarshal(data, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", unmarshalErr)
	}
	return result, nil
}

// runCreateCommand handles the common pattern for create operations.
func runCreateCommand(
	cmd *cobra.Command,
	createFunc func(*api.ApplicationAPI, map[string]any) (map[string]any, error),
	successMessage string,
) error {
	data, err := parseJSONData(cmd)
	if err != nil {
		return err
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	result, err := createFunc(client, data)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("%s", successMessage)
	return formatter.Print(result)
}

// runUpdateCommand handles the common pattern for update operations.
func runUpdateCommand(
	cmd *cobra.Command,
	args []string,
	updateFunc func(*api.ApplicationAPI, string) (map[string]any, error),
	successMessage string,
) error {
	id := args[0]

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	result, err := updateFunc(client, id)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("%s", successMessage)
	return formatter.Print(result)
}

// runDeleteCommand handles the common pattern for delete operations.
func runDeleteCommand(
	cmd *cobra.Command,
	args []string,
	deleteFunc func(*api.ApplicationAPI, string) error,
	successMessage string,
) error {
	id := args[0]

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	if deleteErr := deleteFunc(client, id); deleteErr != nil {
		return fmt.Errorf("%s", apierrors.HandleError(deleteErr))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("%s", successMessage)
	return nil
}

// makeCreateRunE creates a RunE function for create operations.
func makeCreateRunE(
	createFunc func(*api.ApplicationAPI, map[string]any) (map[string]any, error),
	successMessage string,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		return runCreateCommand(cmd, createFunc, successMessage)
	}
}

// makeUpdateRunE creates a RunE function for update operations.
func makeUpdateRunE(
	updateFunc func(*api.ApplicationAPI, string) (map[string]any, error),
	successMessage string,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return runUpdateCommand(cmd, args, updateFunc, successMessage)
	}
}

// makeDeleteRunE creates a RunE function for delete operations.
func makeDeleteRunE(
	deleteFunc func(*api.ApplicationAPI, string) error,
	successMessage string,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return runDeleteCommand(cmd, args, deleteFunc, successMessage)
	}
}

// makeCompletionValidArgsFunction creates a ValidArgsFunction for completion.
func makeCompletionValidArgsFunction(
	completeFunc func(string) ([]string, error),
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completeFunc(toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// newCRUDResourceCmd creates a complete CRUD command with list, view, create, update, and delete subcommands.
func newCRUDResourceCmd(config crudResourceConfig) *cobra.Command {
	cmd := newResourceCmd(resourceCommandConfig{
		name:         config.name,
		short:        config.short,
		long:         config.long,
		listShort:    config.listShort,
		listRunE:     makeListRunE(config.listFunc, config.resourceType),
		viewUse:      config.viewUse,
		viewShort:    config.viewShort,
		viewRunE:     makeViewRunE(config.viewFunc),
		completeFunc: config.completeFunc,
	})

	createCmd := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create a new %s", config.name),
		Long:  config.createLong,
		RunE:  makeCreateRunE(config.createFunc, config.createMessage),
	}
	createCmd.Flags().String("data", "", config.dataFlagHelp)

	updateCmd := &cobra.Command{
		Use:   fmt.Sprintf("update <%s-id>", config.name),
		Short: fmt.Sprintf("Update a %s", config.name),
		Long:  fmt.Sprintf("Update a %s by ID", config.name),
		Args:  cobra.ExactArgs(1),
		RunE:  makeUpdateRunE(config.updateFunc, config.updateMessage),
	}
	updateCmd.ValidArgsFunction = makeCompletionValidArgsFunction(config.completeFunc)

	deleteCmd := &cobra.Command{
		Use:   fmt.Sprintf("delete <%s-id>", config.name),
		Short: fmt.Sprintf("Delete a %s", config.name),
		Long:  fmt.Sprintf("Delete a %s by ID", config.name),
		Args:  cobra.ExactArgs(1),
		RunE:  makeDeleteRunE(config.deleteFunc, config.deleteMessage),
	}
	deleteCmd.ValidArgsFunction = makeCompletionValidArgsFunction(config.completeFunc)

	cmd.AddCommand(createCmd)
	cmd.AddCommand(updateCmd)
	cmd.AddCommand(deleteCmd)

	return cmd
}
