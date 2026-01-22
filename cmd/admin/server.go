package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/bulk"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

func adminServerCompletionAction(c carapace.Context) carapace.Action {
	completions, err := completion.CompleteServers("admin", c.Value)
	if err != nil || len(completions) == 0 {
		return carapace.ActionValues()
	}
	return carapace.ActionValues(completions...)
}

func adminServerValidArgs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	completions, err := completion.CompleteServers("admin", toComplete)
	if err != nil || len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

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
	viewCmd.ValidArgsFunction = adminServerValidArgs

	suspendCmd := &cobra.Command{
		Use:   "suspend <id|uuid>...",
		Short: "Suspend server(s)",
		Long:  "Suspend server(s) by ID (integer) or UUID (string)",
		RunE:  runSuspendServer,
	}
	addBulkFlags(suspendCmd)
	suspendCmd.ValidArgsFunction = adminServerValidArgs
	carapace.Gen(suspendCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

	unsuspendCmd := &cobra.Command{
		Use:   "unsuspend <id|uuid>...",
		Short: "Unsuspend server(s)",
		Long:  "Unsuspend server(s) by ID (integer) or UUID (string)",
		RunE:  runUnsuspendServer,
	}
	addBulkFlags(unsuspendCmd)
	unsuspendCmd.ValidArgsFunction = adminServerValidArgs
	carapace.Gen(unsuspendCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

	reinstallCmd := &cobra.Command{
		Use:   "reinstall <id|uuid>...",
		Short: "Reinstall server(s)",
		Long:  "Reinstall server(s) by ID (integer) or UUID (string)",
		RunE:  runReinstallServer,
	}
	addBulkFlags(reinstallCmd)
	reinstallCmd.ValidArgsFunction = adminServerValidArgs
	carapace.Gen(reinstallCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

	healthCmd := &cobra.Command{
		Use:   "health <id|uuid>...",
		Short: "Get server health status",
		Long:  "Get the health status of server(s) by ID (integer) or UUID (string), including container status and optional crash detection",
		RunE:  runServerHealth,
	}
	addBulkFlags(healthCmd)
	healthCmd.Flags().String("since", "", "check for crashes since this date-time (RFC3339 format)")
	healthCmd.Flags().Int("window", 0, "time window in minutes (1-1440) for crash detection")
	healthCmd.ValidArgsFunction = adminServerValidArgs
	carapace.Gen(healthCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

	powerCmd := newPowerCmd()

	// Add all commands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(suspendCmd)
	cmd.AddCommand(unsuspendCmd)
	cmd.AddCommand(reinstallCmd)
	cmd.AddCommand(healthCmd)
	cmd.AddCommand(powerCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	carapace.Gen(viewCmd).PositionalCompletion(carapace.ActionCallback(adminServerCompletionAction))
	carapace.Gen(suspendCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))
	carapace.Gen(unsuspendCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))
	carapace.Gen(reinstallCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))
	carapace.Gen(healthCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

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

func runServerHealth(cmd *cobra.Command, args []string) error {
	flags := getBulkFlags(cmd)

	// Parse --since flag
	var since *time.Time
	sinceStr, _ := cmd.Flags().GetString("since")
	if sinceStr != "" {
		parsedSince, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since format: %w (expected RFC3339 format, e.g., 2006-01-02T15:04:05Z07:00)", err)
		}
		since = &parsedSince
	}

	// Parse and validate --window flag
	var window *int
	windowVal, _ := cmd.Flags().GetInt("window")
	if windowVal != 0 {
		if windowVal < 1 || windowVal > 1440 {
			return fmt.Errorf("--window must be between 1 and 1440 minutes")
		}
		window = &windowVal
	}

	// Validate that we have either positional args or bulk flags
	if len(args) == 0 && !flags.all && flags.fromFile == "" {
		return errors.New("no servers specified")
	}

	// Get server UUIDs
	uuids := args
	if flags.all || flags.fromFile != "" {
		var err error
		uuids, err = getServerUUIDs(cmd, args, flags.all, flags.fromFile)
		if err != nil {
			return err
		}
	}

	if len(uuids) == 0 {
		return fmt.Errorf("no servers found - try 'pelicanctl admin server list' to see available servers")
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	if flags.dryRun {
		formatter.PrintInfo("Dry run - would check health for %d server(s):", len(uuids))
		for _, uuid := range uuids {
			formatter.PrintInfo("  - %s", uuid)
		}
		return nil
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	// Single server - simple output
	if len(uuids) == 1 {
		health, err := client.GetServerHealth(uuids[0], since, window)
		if err != nil {
			return fmt.Errorf("%s", apierrors.HandleError(err))
		}
		return formatter.Print(health)
	}

	// Multiple servers - bulk operation
	ctx := context.Background()
	results := executeHealthOperations(ctx, client, uuids, since, window, flags)

	// Format results based on output format
	if getOutputFormat(cmd) == output.OutputFormatJSON {
		return printHealthResultsJSON(formatter, results)
	}

	return printHealthResultsTable(formatter, results)
}

type healthResult struct {
	Server string
	Health map[string]any
	Error  error
}

func executeHealthOperations(
	ctx context.Context,
	client *api.ApplicationAPI,
	uuids []string,
	since *time.Time,
	window *int,
	flags bulkFlags,
) []healthResult {
	operations := make([]bulk.Operation, len(uuids))
	resultsMap := make(map[string]*healthResult, len(uuids))

	// Initialize results map
	for _, uuid := range uuids {
		resultsMap[uuid] = &healthResult{Server: uuid}
	}

	// Create operations
	for i, uuid := range uuids {
		uuid := uuid // capture loop variable
		result := resultsMap[uuid]
		operations[i] = bulk.Operation{
			ID:   uuid,
			Name: uuid,
			Exec: func() error {
				health, err := client.GetServerHealth(uuid, since, window)
				if err != nil {
					result.Error = err
					return err
				}
				result.Health = health
				return nil
			},
		}
	}

	executor := bulk.NewExecutor(flags.maxConcurrency, flags.continueOnError, flags.failFast)
	bulkResults := executor.Execute(ctx, operations)

	// Update results with errors from bulk executor if not already set
	for _, bulkResult := range bulkResults {
		if !bulkResult.Success {
			if result, ok := resultsMap[bulkResult.Operation.ID]; ok && result.Error == nil {
				result.Error = bulkResult.Error
			}
		}
	}

	// Convert map to slice maintaining order
	results := make([]healthResult, len(uuids))
	for i, uuid := range uuids {
		results[i] = *resultsMap[uuid]
	}

	return results
}

func printHealthResultsJSON(formatter *output.Formatter, results []healthResult) error {
	outputData := make([]map[string]any, 0, len(results))

	for _, result := range results {
		if result.Error != nil {
			// Include error in JSON output
			outputData = append(outputData, map[string]any{
				"server_identifier": result.Server,
				"error":             result.Error.Error(),
			})
		} else {
			// Copy all health data (includes nested server, container, crash_details objects)
			healthData := make(map[string]any)
			for k, v := range result.Health {
				healthData[k] = v
			}
			// Add the query identifier separately to preserve the full server object from API
			healthData["server_identifier"] = result.Server
			outputData = append(outputData, healthData)
		}
	}

	return formatter.Print(outputData)
}

func printHealthResultsTable(formatter *output.Formatter, results []healthResult) error {
	headers := []string{"Server", "Name", "Container Status", "Healthy", "Crashed", "Checked At"}
	rows := make([][]string, 0, len(results))

	for _, result := range results {
		if result.Error != nil {
			rows = append(rows, []string{
				result.Server,
				"",
				"error",
				"",
				"",
				"",
			})
			formatter.PrintError("%s: %v", result.Server, result.Error)
		} else {
			// Extract server name
			serverName := "unknown"
			if server, ok := result.Health["server"].(map[string]any); ok {
				if name, ok := server["name"].(string); ok {
					serverName = name
				}
			}

			// Extract container status and healthy
			containerStatus := "unknown"
			healthy := "unknown"
			if container, ok := result.Health["container"].(map[string]any); ok {
				if s, ok := container["status"].(string); ok {
					containerStatus = s
				}
				if h, ok := container["healthy"].(bool); ok {
					if h {
						healthy = "true"
					} else {
						healthy = "false"
					}
				}
			}

			// Extract crashed
			crashed := "unknown"
			if c, ok := result.Health["crashed"].(bool); ok {
				if c {
					crashed = "true"
				} else {
					crashed = "false"
				}
			}

			// Extract checked_at
			checkedAt := "unknown"
			if ca, ok := result.Health["checked_at"].(string); ok {
				checkedAt = ca
			}

			rows = append(rows, []string{
				result.Server,
				serverName,
				containerStatus,
				healthy,
				crashed,
				checkedAt,
			})
		}
	}

	return formatter.PrintTable(headers, rows)
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

		if len(servers) == 0 {
			return nil, fmt.Errorf("no servers found in the system")
		}

		for _, server := range servers {
			// Try UUID first
			if uuid, ok := server["uuid"].(string); ok && uuid != "" {
				uuids = append(uuids, uuid)
				continue
			}
			// Fallback to ID if UUID not available
			if id := extractServerID(server); id != nil {
				var idStr string
				switch v := id.(type) {
				case int:
					idStr = fmt.Sprintf("%d", v)
				case int64:
					idStr = fmt.Sprintf("%d", v)
				case float64:
					idStr = fmt.Sprintf("%.0f", v)
				case string:
					idStr = v
				default:
					continue
				}
				uuids = append(uuids, idStr)
			}
		}

		if len(uuids) == 0 {
			return nil, fmt.Errorf("no valid server identifiers found (servers may be missing uuid or id fields)")
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
		// Support both space-separated and comma-separated arguments
		// e.g., "123 456" or "123,456" or "123,456 789" (mixed)
		for _, arg := range args {
			// Split by comma and trim whitespace
			parts := strings.Split(arg, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					uuids = append(uuids, part)
				}
			}
		}
	}

	return uuids, nil
}

// extractServerID extracts the server ID from a server map, checking both root and attributes.
func extractServerID(server map[string]any) any {
	if id, hasID := server["id"]; hasID {
		return id
	}
	if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
		if id, hasID := attrs["id"]; hasID {
			return id
		}
	}
	return nil
}

// getOutputFormat is defined in common.go
