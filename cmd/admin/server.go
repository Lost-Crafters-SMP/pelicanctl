package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	"go.lostcrafters.com/pelican-cli/internal/bulk"
	"go.lostcrafters.com/pelican-cli/internal/completion"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
		Long:  "List, view, and manage servers",
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
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	suspendCmd := &cobra.Command{
		Use:   "suspend <id|uuid>...",
		Short: "Suspend server(s)",
		Long:  "Suspend server(s) by ID (integer) or UUID (string)",
		RunE:  runSuspendServer,
	}
	addBulkFlags(suspendCmd)
	suspendCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
	carapace.Gen(suspendCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

	unsuspendCmd := &cobra.Command{
		Use:   "unsuspend <id|uuid>...",
		Short: "Unsuspend server(s)",
		Long:  "Unsuspend server(s) by ID (integer) or UUID (string)",
		RunE:  runUnsuspendServer,
	}
	addBulkFlags(unsuspendCmd)
	unsuspendCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
	carapace.Gen(unsuspendCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

	reinstallCmd := &cobra.Command{
		Use:   "reinstall <id|uuid>...",
		Short: "Reinstall server(s)",
		Long:  "Reinstall server(s) by ID (integer) or UUID (string)",
		RunE:  runReinstallServer,
	}
	addBulkFlags(reinstallCmd)
	reinstallCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
	carapace.Gen(reinstallCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

	powerCmd := newPowerCmd()

	// Add all commands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(suspendCmd)
	cmd.AddCommand(unsuspendCmd)
	cmd.AddCommand(reinstallCmd)
	cmd.AddCommand(powerCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	carapace.Gen(viewCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(suspendCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(unsuspendCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(reinstallCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

	return cmd
}

func runServerList(cmd *cobra.Command, _ []string) error {
	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	servers, err := client.ListServers()
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(servers, output.ResourceTypeAdminServer)
}

func runServerView(cmd *cobra.Command, args []string) error {
	uuid := args[0]

	client, err := api.NewApplicationAPI()
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

func runSuspendServer(cmd *cobra.Command, args []string) error {
	return runServerAction(cmd, args, "suspend", func(client *api.ApplicationAPI, uuid string) error {
		return client.SuspendServer(uuid)
	})
}

func runUnsuspendServer(cmd *cobra.Command, args []string) error {
	return runServerAction(cmd, args, "unsuspend", func(client *api.ApplicationAPI, uuid string) error {
		return client.UnsuspendServer(uuid)
	})
}

func runReinstallServer(cmd *cobra.Command, args []string) error {
	return runServerAction(cmd, args, "reinstall", func(client *api.ApplicationAPI, uuid string) error {
		return client.ReinstallServer(uuid)
	})
}

func newPowerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "power",
		Short: "Control server power",
		Long:  "Start, stop, restart, or kill servers",
	}

	startCmd := &cobra.Command{
		Use:   "start <id|uuid>...",
		Short: "Start server(s)",
		Long:  "Start server(s) by ID (integer) or UUID (string)",
		RunE:  runPowerStart,
	}
	addBulkFlags(startCmd)
	startCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	stopCmd := &cobra.Command{
		Use:   "stop <id|uuid>...",
		Short: "Stop server(s)",
		Long:  "Stop server(s) by ID (integer) or UUID (string)",
		RunE:  runPowerStop,
	}
	addBulkFlags(stopCmd)
	stopCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	restartCmd := &cobra.Command{
		Use:   "restart <id|uuid>...",
		Short: "Restart server(s)",
		Long:  "Restart server(s) by ID (integer) or UUID (string)",
		RunE:  runPowerRestart,
	}
	addBulkFlags(restartCmd)
	restartCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	killCmd := &cobra.Command{
		Use:   "kill <id|uuid>...",
		Short: "Kill server(s)",
		Long:  "Kill server(s) by ID (integer) or UUID (string)",
		RunE:  runPowerKill,
	}
	addBulkFlags(killCmd)
	killCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteServers("admin", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	cmd.AddCommand(startCmd)
	cmd.AddCommand(stopCmd)
	cmd.AddCommand(restartCmd)
	cmd.AddCommand(killCmd)

	// Set up carapace completion AFTER adding to parent
	carapace.Gen(startCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(stopCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(restartCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)
	carapace.Gen(killCmd).PositionalAnyCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			completions, err := completion.CompleteServers("admin", c.Value)
			if err != nil || len(completions) == 0 {
				return carapace.ActionValues()
			}
			return carapace.ActionValues(completions...)
		}),
	)

	return cmd
}

func runPowerCommand(cmd *cobra.Command, args []string, command string) error {
	return runServerAction(cmd, args, command, func(client *api.ApplicationAPI, identifier string) error {
		return client.SendPowerCommand(identifier, command)
	})
}

func runPowerStart(cmd *cobra.Command, args []string) error {
	return runPowerCommand(cmd, args, "start")
}

func runPowerStop(cmd *cobra.Command, args []string) error {
	return runPowerCommand(cmd, args, "stop")
}

func runPowerRestart(cmd *cobra.Command, args []string) error {
	return runPowerCommand(cmd, args, "restart")
}

func runPowerKill(cmd *cobra.Command, args []string) error {
	return runPowerCommand(cmd, args, "kill")
}

type serverActionFunc func(client *api.ApplicationAPI, uuid string) error

type bulkFlags struct {
	all             bool
	fromFile        string
	maxConcurrency  int
	continueOnError bool
	failFast        bool
	dryRun          bool
	yes             bool
}

func getBulkFlags(cmd *cobra.Command) bulkFlags {
	all, _ := cmd.Flags().GetBool("all")
	fromFile, _ := cmd.Flags().GetString("from-file")
	maxConcurrency, _ := cmd.Flags().GetInt("max-concurrency")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")

	return bulkFlags{
		all:             all,
		fromFile:        fromFile,
		maxConcurrency:  maxConcurrency,
		continueOnError: continueOnError,
		failFast:        failFast,
		dryRun:          dryRun,
		yes:             yes,
	}
}

func handleConfirmation(formatter *output.Formatter, actionName string, uuidCount int, yes bool) (bool, error) {
	if yes {
		return true, nil
	}

	// Require confirmation for destructive actions
	needsConfirmation := actionName == "reinstall" || actionName == "kill" || (actionName == "stop" && uuidCount > 1)
	if !needsConfirmation {
		return true, nil
	}

	formatter.PrintInfo("This will %s %d server(s). Continue? (y/N): ", actionName, uuidCount)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		return false, nil
	}

	return true, nil
}

func handleDryRun(formatter *output.Formatter, actionName string, uuids []string) {
	formatter.PrintInfo("Dry run - would %s %d server(s):", actionName, len(uuids))
	for _, uuid := range uuids {
		formatter.PrintInfo("  - %s", uuid)
	}
}

func executeBulkOperations(
	ctx context.Context,
	client *api.ApplicationAPI,
	uuids []string,
	action serverActionFunc,
	flags bulkFlags,
) []bulk.Result {
	operations := make([]bulk.Operation, len(uuids))
	for i, uuid := range uuids {
		operations[i] = bulk.Operation{
			ID:   uuid,
			Name: uuid,
			Exec: func() error {
				return action(client, uuid)
			},
		}
	}

	executor := bulk.NewExecutor(flags.maxConcurrency, flags.continueOnError, flags.failFast)
	return executor.Execute(ctx, operations)
}

func printResults(formatter *output.Formatter, results []bulk.Result, actionName string) {
	for _, result := range results {
		if result.Success {
			formatter.PrintSuccess("%s: %s", result.Operation.ID, actionName)
		} else {
			formatter.PrintError("%s: %v", result.Operation.ID, result.Error)
		}
	}
}

func handleSummary(formatter *output.Formatter, results []bulk.Result) error {
	summary := bulk.GetSummary(results)
	formatter.PrintInfo("Summary: %d succeeded, %d failed", summary.Success, summary.Failed)

	if summary.Failed > 0 {
		return fmt.Errorf("%d operation(s) failed", summary.Failed)
	}

	return nil
}

func runServerAction(cmd *cobra.Command, args []string, actionName string, action serverActionFunc) error {
	if len(args) == 0 {
		return errors.New("no servers specified")
	}

	flags := getBulkFlags(cmd)

	uuids := args
	if flags.all || flags.fromFile != "" {
		var err error
		uuids, err = getServerUUIDs(cmd, args, flags.all, flags.fromFile)
		if err != nil {
			return err
		}
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	shouldContinue, err := handleConfirmation(formatter, actionName, len(uuids), flags.yes)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	if flags.dryRun {
		handleDryRun(formatter, actionName, uuids)
		return nil
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	ctx := context.Background()
	results := executeBulkOperations(ctx, client, uuids, action, flags)

	printResults(formatter, results, actionName)

	return handleSummary(formatter, results)
}

func addBulkFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on all servers")
	cmd.Flags().String("from-file", "", "read server UUIDs from file (one per line)")
	const defaultMaxConcurrency = 10
	cmd.Flags().Int("max-concurrency", defaultMaxConcurrency, "maximum parallel operations")
	cmd.Flags().Bool("continue-on-error", true, "continue on errors")
	cmd.Flags().Bool("fail-fast", false, "stop on first error")
	cmd.Flags().Bool("dry-run", false, "preview operations without executing")
	cmd.Flags().Bool("yes", false, "skip confirmation prompts")
}

func getServerUUIDs(_ *cobra.Command, args []string, all bool, fromFile string) ([]string, error) {
	var uuids []string

	switch {
	case all:
		client, err := api.NewApplicationAPI()
		if err != nil {
			return nil, err
		}

		servers, err := client.ListServers()
		if err != nil {
			return nil, err
		}

		for _, server := range servers {
			if uuid, ok := server["uuid"].(string); ok {
				uuids = append(uuids, uuid)
			}
		}
	case fromFile != "":
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				uuids = append(uuids, line)
			}
		}
	default:
		uuids = args
	}

	return uuids, nil
}

// getOutputFormat is defined in common.go
