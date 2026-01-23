package client

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/bulk"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

const (
	statusSuccess = "success"
	statusError   = "error"
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
	viewCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	resourcesCmd := &cobra.Command{
		Use:   "resources <id|uuid>",
		Short: "View server resource usage",
		Long:  "View server resource usage by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runServerResources,
	}
	resourcesCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	commandCmd := &cobra.Command{
		Use:   "command <uuid>... --command <command>",
		Short: "Send command to server(s)",
		Long:  "Send a console command to one or more running servers by UUID. Supports bulk operations with --all or --from-file.",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runServerCommand,
	}
	commandCmd.Flags().String("command", "", "The command to send to the server console (required)")
	_ = commandCmd.MarkFlagRequired("command")
	setupBulkFlags(commandCmd)
	commandCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("client", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	// Add subcommands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(resourcesCmd)
	cmd.AddCommand(commandCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	carapace.Gen(viewCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("client", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(resourcesCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("client", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(commandCmd).PositionalAnyCompletion(
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

func runServerCommand(cmd *cobra.Command, args []string) error {
	command, _ := cmd.Flags().GetString("command")
	if command == "" {
		return errors.New("--command flag is required")
	}

	all, _ := cmd.Flags().GetBool("all")
	fromFile, _ := cmd.Flags().GetString("from-file")
	maxConcurrency, _ := cmd.Flags().GetInt("max-concurrency")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	uuids, err := getServerUUIDs(cmd, args, all, fromFile)
	if err != nil {
		return err
	}

	if len(uuids) == 0 {
		return errors.New("no servers specified")
	}

	if dryRun {
		formatter.PrintInfo("Dry run - would send command '%s' to %d server(s):", command, len(uuids))
		for _, uuid := range uuids {
			formatter.PrintInfo("  - %s", uuid)
		}
		return nil
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	ctx := context.Background()
	results := executeCommandOperations(ctx, client, uuids, command, maxConcurrency, continueOnError, failFast)

	// Handle JSON output specially
	if getOutputFormat(cmd) == output.OutputFormatJSON {
		summary := bulk.GetSummary(results)
		return printCommandResultsJSON(formatter, results, command, summary, continueOnError)
	}

	printCommandResults(formatter, results, command)

	return handleCommandSummary(formatter, results, continueOnError)
}

func executeCommandOperations(
	ctx context.Context,
	client *api.ClientAPI,
	uuids []string,
	command string,
	maxConcurrency int,
	continueOnError bool,
	failFast bool,
) []bulk.Result {
	operations := make([]bulk.Operation, len(uuids))
	for i, uuid := range uuids {
		operations[i] = bulk.Operation{
			ID:   uuid,
			Name: uuid,
			Exec: func() error {
				return client.SendCommand(uuid, command)
			},
		}
	}

	executor := bulk.NewExecutor(maxConcurrency, continueOnError, failFast)
	return executor.Execute(ctx, operations)
}

func printCommandResultsJSON(
	formatter *output.Formatter,
	results []bulk.Result,
	command string,
	summary bulk.Summary,
	continueOnError bool,
) error {
	outputData := make([]map[string]any, 0, len(results))

	for _, result := range results {
		resultData := map[string]any{
			"server_identifier": result.Operation.ID,
			"command":           command,
		}
		if result.Success {
			resultData["status"] = statusSuccess
		} else {
			resultData["status"] = statusError
			resultData["error"] = result.Error.Error()
		}
		outputData = append(outputData, resultData)
	}

	// Include summary in the output
	response := map[string]any{
		"results": outputData,
		"summary": map[string]any{
			"succeeded": summary.Success,
			"failed":    summary.Failed,
		},
	}

	if err := formatter.Print(response); err != nil {
		return err
	}

	// Check failures based on continue-on-error flag
	if summary.Failed > 0 && !continueOnError {
		return fmt.Errorf("%d operation(s) failed", summary.Failed)
	}

	return nil
}

func printCommandResults(formatter *output.Formatter, results []bulk.Result, command string) {
	for _, result := range results {
		if result.Success {
			formatter.PrintSuccess("%s: command '%s' sent", result.Operation.ID, command)
		} else {
			formatter.PrintError("%s: %v", result.Operation.ID, result.Error)
		}
	}
}

func handleCommandSummary(formatter *output.Formatter, results []bulk.Result, continueOnError bool) error {
	summary := bulk.GetSummary(results)
	formatter.PrintInfo("Summary: %d succeeded, %d failed", summary.Success, summary.Failed)

	if summary.Failed > 0 && !continueOnError {
		return fmt.Errorf("%d operation(s) failed", summary.Failed)
	}

	return nil
}

func getOutputFormat(cmd *cobra.Command) output.OutputFormat {
	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		return output.OutputFormatJSON
	}
	return output.OutputFormatTable
}
