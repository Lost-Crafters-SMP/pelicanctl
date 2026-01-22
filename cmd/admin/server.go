package admin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strconv"
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

func newServerBasicCommands() []*cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all servers",
		RunE:  runServerList,
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long:  "Create a new server. Provide server data as JSON via --data flag or stdin.",
		RunE:  runServerCreate,
	}
	createCmd.Flags().String("data", "", "JSON data for the server (or read from stdin)")

	viewCmd := &cobra.Command{
		Use:   "view <id|uuid>",
		Short: "View server details",
		Long:  "View server details by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runServerView,
	}
	viewCmd.ValidArgsFunction = adminServerValidArgs

	deleteCmd := &cobra.Command{
		Use:   "delete <id|uuid>",
		Short: "Delete a server",
		Long:  "Delete a server by ID (integer) or UUID (string). Use --force to force delete.",
		Args:  cobra.ExactArgs(1),
		RunE:  runServerDelete,
	}
	deleteCmd.Flags().Bool("force", false, "Force delete the server")
	deleteCmd.ValidArgsFunction = adminServerValidArgs

	return []*cobra.Command{listCmd, createCmd, viewCmd, deleteCmd}
}

func newServerActionCommands() []*cobra.Command {
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

	return []*cobra.Command{suspendCmd, unsuspendCmd, reinstallCmd, healthCmd}
}

func setupServerCommandCompletion(cmds []*cobra.Command) {
	for _, cmd := range cmds {
		if cmd.Use == "view" || cmd.Use == "delete" {
			carapace.Gen(cmd).PositionalCompletion(carapace.ActionCallback(adminServerCompletionAction))
		}
	}
}

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
		Long:  "List, view, and manage servers",
	}

	basicCmds := newServerBasicCommands()
	actionCmds := newServerActionCommands()
	powerCmd := newPowerCmd()
	backupCmd := newBackupCmd()

	// Add all commands FIRST (matching carapace example pattern)
	for _, c := range basicCmds {
		cmd.AddCommand(c)
	}
	for _, c := range actionCmds {
		cmd.AddCommand(c)
	}
	cmd.AddCommand(powerCmd)
	cmd.AddCommand(backupCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	setupServerCommandCompletion(basicCmds)

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

func runServerCreate(cmd *cobra.Command, _ []string) error {
	return runCreateCommand(
		cmd,
		func(c *api.ApplicationAPI, data map[string]any) (map[string]any, error) {
			return c.CreateServer(data)
		},
		"Server created successfully",
	)
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

func runServerDelete(cmd *cobra.Command, args []string) error {
	identifier := args[0]
	force, _ := cmd.Flags().GetBool("force")

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	deleteErr := client.DeleteServer(identifier, force)
	if deleteErr != nil {
		return fmt.Errorf("%s", apierrors.HandleError(deleteErr))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("Server deleted successfully")
	return nil
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

func parseSinceFlag(cmd *cobra.Command) (*time.Time, error) {
	sinceStr, _ := cmd.Flags().GetString("since")
	if sinceStr == "" {
		//nolint:nilnil // Optional flag - returning nil, nil is correct when flag is not provided
		return nil, nil
	}
	parsedSince, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid --since format: %w (expected RFC3339 format, e.g., 2006-01-02T15:04:05Z07:00)",
			err,
		)
	}
	return &parsedSince, nil
}

func parseWindowFlag(cmd *cobra.Command) (*int, error) {
	windowVal, _ := cmd.Flags().GetInt("window")
	if windowVal == 0 {
		//nolint:nilnil // Optional flag - returning nil, nil is correct when flag is not provided
		return nil, nil
	}
	if windowVal < 1 || windowVal > 1440 {
		return nil, errors.New("--window must be between 1 and 1440 minutes")
	}
	return &windowVal, nil
}

func validateHealthArgs(args []string, flags bulkFlags) error {
	if len(args) == 0 && !flags.all && flags.fromFile == "" {
		return errors.New("no servers specified")
	}
	return nil
}

func getHealthServerUUIDs(cmd *cobra.Command, args []string, flags bulkFlags) ([]string, error) {
	uuids := args
	if flags.all || flags.fromFile != "" {
		var err error
		uuids, err = getServerUUIDs(cmd, args, flags.all, flags.fromFile)
		if err != nil {
			return nil, err
		}
	}
	if len(uuids) == 0 {
		return nil, errors.New("no servers found - try 'pelicanctl admin server list' to see available servers")
	}
	return uuids, nil
}

func runServerHealthSingle(
	client *api.ApplicationAPI,
	formatter *output.Formatter,
	uuid string,
	since *time.Time,
	window *int,
) error {
	health, healthErr := client.GetServerHealth(uuid, since, window)
	if healthErr != nil {
		return fmt.Errorf("%s", apierrors.HandleError(healthErr))
	}
	return formatter.Print(health)
}

func runServerHealthMultiple(
	cmd *cobra.Command,
	client *api.ApplicationAPI,
	formatter *output.Formatter,
	uuids []string,
	since *time.Time,
	window *int,
	flags bulkFlags,
) error {
	ctx := context.Background()
	results := executeHealthOperations(ctx, client, uuids, since, window, flags)

	if getOutputFormat(cmd) == output.OutputFormatJSON {
		return printHealthResultsJSON(formatter, results)
	}
	return printHealthResultsTable(formatter, results)
}

func runServerHealth(cmd *cobra.Command, args []string) error {
	flags := getBulkFlags(cmd)

	since, err := parseSinceFlag(cmd)
	if err != nil {
		return err
	}

	window, err := parseWindowFlag(cmd)
	if err != nil {
		return err
	}

	if err = validateHealthArgs(args, flags); err != nil {
		return err
	}

	uuids, err := getHealthServerUUIDs(cmd, args, flags)
	if err != nil {
		return err
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

	if len(uuids) == 1 {
		return runServerHealthSingle(client, formatter, uuids[0], since, window)
	}

	return runServerHealthMultiple(cmd, client, formatter, uuids, since, window, flags)
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
			maps.Copy(healthData, result.Health)
			// Add the query identifier separately to preserve the full server object from API
			healthData["server_identifier"] = result.Server
			outputData = append(outputData, healthData)
		}
	}

	return formatter.Print(outputData)
}

const unknownStatus = "unknown"

func extractServerName(health map[string]any) string {
	server, ok := health["server"].(map[string]any)
	if !ok {
		return unknownStatus
	}
	name, ok := server["name"].(string)
	if !ok {
		return unknownStatus
	}
	return name
}

func extractContainerInfo(health map[string]any) (string, string) {
	container, ok := health["container"].(map[string]any)
	if !ok {
		return unknownStatus, unknownStatus
	}
	status := unknownStatus
	if s, okStatus := container["status"].(string); okStatus {
		status = s
	}
	healthy := unknownStatus
	if h, okHealthy := container["healthy"].(bool); okHealthy {
		if h {
			healthy = "true"
		} else {
			healthy = "false"
		}
	}
	return status, healthy
}

func extractCrashedStatus(health map[string]any) string {
	c, ok := health["crashed"].(bool)
	if !ok {
		return unknownStatus
	}
	if c {
		return "true"
	}
	return "false"
}

func extractCheckedAt(health map[string]any) string {
	ca, ok := health["checked_at"].(string)
	if !ok {
		return unknownStatus
	}
	return ca
}

func buildHealthRow(result healthResult) []string {
	if result.Error != nil {
		return []string{
			result.Server,
			"",
			"error",
			"",
			"",
			"",
		}
	}

	serverName := extractServerName(result.Health)
	containerStatus, healthy := extractContainerInfo(result.Health)
	crashed := extractCrashedStatus(result.Health)
	checkedAt := extractCheckedAt(result.Health)

	return []string{
		result.Server,
		serverName,
		containerStatus,
		healthy,
		crashed,
		checkedAt,
	}
}

func printHealthResultsTable(formatter *output.Formatter, results []healthResult) error {
	headers := []string{"Server", "Name", "Container Status", "Healthy", "Crashed", "Checked At"}
	rows := make([][]string, 0, len(results))

	for _, result := range results {
		if result.Error != nil {
			formatter.PrintError("%s: %v", result.Server, result.Error)
		}
		rows = append(rows, buildHealthRow(result))
	}

	return formatter.PrintTable(headers, rows)
}

func adminServerValidArgsFunction(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	completions, err := completion.CompleteServers("admin", toComplete)
	if err != nil || len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func createPowerSubcommand(use, short, long string, runE func(*cobra.Command, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		RunE:  runE,
	}
	addBulkFlags(cmd)
	cmd.ValidArgsFunction = adminServerValidArgsFunction
	return cmd
}

func setupPowerCommandCompletion(cmd *cobra.Command) {
	carapace.Gen(cmd).PositionalAnyCompletion(
		carapace.ActionCallback(adminServerCompletionAction),
	)
}

func newPowerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "power",
		Short: "Control server power",
		Long:  "Start, stop, restart, or kill servers",
	}

	powerCommands := []struct {
		use   string
		short string
		long  string
		runE  func(*cobra.Command, []string) error
	}{
		{"start <id|uuid>...", "Start server(s)", "Start server(s) by ID (integer) or UUID (string)", runPowerStart},
		{"stop <id|uuid>...", "Stop server(s)", "Stop server(s) by ID (integer) or UUID (string)", runPowerStop},
		{
			"restart <id|uuid>...",
			"Restart server(s)",
			"Restart server(s) by ID (integer) or UUID (string)",
			runPowerRestart,
		},
		{"kill <id|uuid>...", "Kill server(s)", "Kill server(s) by ID (integer) or UUID (string)", runPowerKill},
	}

	for _, pc := range powerCommands {
		subCmd := createPowerSubcommand(pc.use, pc.short, pc.long, pc.runE)
		cmd.AddCommand(subCmd)
		setupPowerCommandCompletion(subCmd)
	}

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

func convertServerIDToString(id any) string {
	switch v := id.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case string:
		return v
	default:
		return ""
	}
}

func extractUUIDsFromServers(servers []map[string]any) ([]string, error) {
	if len(servers) == 0 {
		return nil, errors.New("no servers found in the system")
	}

	var uuids []string
	for _, server := range servers {
		// Try UUID first
		if uuid, ok := server["uuid"].(string); ok && uuid != "" {
			uuids = append(uuids, uuid)
			continue
		}
		// Fallback to ID if UUID not available
		if id := extractServerID(server); id != nil {
			if idStr := convertServerIDToString(id); idStr != "" {
				uuids = append(uuids, idStr)
			}
		}
	}

	if len(uuids) == 0 {
		return nil, errors.New("no valid server identifiers found (servers may be missing uuid or id fields)")
	}
	return uuids, nil
}

func getServerUUIDsFromAll() ([]string, error) {
	client, err := api.NewApplicationAPI()
	if err != nil {
		return nil, err
	}

	servers, err := client.ListServers()
	if err != nil {
		return nil, err
	}

	return extractUUIDsFromServers(servers)
}

func getServerUUIDsFromFile(fromFile string) ([]string, error) {
	data, err := os.ReadFile(fromFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var uuids []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			uuids = append(uuids, line)
		}
	}
	return uuids, nil
}

func getServerUUIDsFromArgs(args []string) []string {
	var uuids []string
	// Support both space-separated and comma-separated arguments
	// e.g., "123 456" or "123,456" or "123,456 789" (mixed)
	for _, arg := range args {
		// Split by comma and trim whitespace
		parts := strings.SplitSeq(arg, ",")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				uuids = append(uuids, part)
			}
		}
	}
	return uuids
}

func getServerUUIDs(_ *cobra.Command, args []string, all bool, fromFile string) ([]string, error) {
	switch {
	case all:
		return getServerUUIDsFromAll()
	case fromFile != "":
		return getServerUUIDsFromFile(fromFile)
	default:
		return getServerUUIDsFromArgs(args), nil
	}
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

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage server backups",
		Long:  "List, create, view, and manage server backups",
	}

	listCmd := &cobra.Command{
		Use:   "list <server-id|uuid>",
		Short: "List backups for a server",
		Long:  "List all backups for a server by ID (integer) or UUID (string)",
		Args:  cobra.ExactArgs(1),
		RunE:  runBackupList,
	}
	listCmd.ValidArgsFunction = adminServerValidArgs

	createCmd := &cobra.Command{
		Use:   "create <server-id|uuid>...",
		Short: "Create backup(s) for server(s)",
		Long:  "Create backup(s) for server(s) by ID (integer) or UUID (string). Supports bulk operations with --all or --from-file.",
		RunE:  runBackupCreate,
	}
	addBulkFlags(createCmd)
	createCmd.Flags().String("ignore", "", "Comma-separated list of files/patterns to ignore")
	createCmd.Flags().String("ignore-file", "", "File containing ignore patterns (newline-separated, like .gitignore)")
	createCmd.Flags().String("name", "", "Backup name")
	createCmd.Flags().Bool("locked", false, "Lock the backup after creation")
	createCmd.Flags().String("save-pairs", "", "Save server+backup pairs to file (format: server-id,backup-uuid)")
	createCmd.ValidArgsFunction = adminServerValidArgs
	carapace.Gen(createCmd).PositionalAnyCompletion(carapace.ActionCallback(adminServerCompletionAction))

	viewCmd := &cobra.Command{
		Use:   "view [<server-id|uuid> <backup-uuid>]... | --from-file <file>",
		Short: "View backup details",
		Long:  "View one or more backups. Can specify server+backup pairs as alternating arguments, or use --from-file to load pairs from a file (comma-separated format: server-id,backup-uuid).",
		Args: func(cmd *cobra.Command, args []string) error {
			fromFile, _ := cmd.Flags().GetString("from-file")
			if fromFile != "" {
				return cobra.NoArgs(cmd, args)
			}
			if len(args)%2 != 0 {
				return errors.New("requires server+backup pairs (even number of arguments)")
			}
			return cobra.MinimumNArgs(minBackupViewArgs)(cmd, args)
		},
		RunE: runBackupView,
	}
	viewCmd.Flags().String("from-file", "", "File containing server+backup pairs (one per line: server-id,backup-uuid)")

	deleteCmd := &cobra.Command{
		Use:   "delete <server-id|uuid> <backup-uuid>",
		Short: "Delete a backup",
		Long:  "Delete a backup by server ID/UUID and backup UUID",
		Args:  cobra.ExactArgs(minBackupViewArgs),
		RunE:  runBackupDelete,
	}
	deleteCmd.ValidArgsFunction = adminServerValidArgs

	// Add subcommands
	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(deleteCmd)

	// Set up carapace completion
	carapace.Gen(listCmd).PositionalCompletion(carapace.ActionCallback(adminServerCompletionAction))
	carapace.Gen(deleteCmd).PositionalCompletion(carapace.ActionCallback(adminServerCompletionAction))

	return cmd
}

func runBackupList(cmd *cobra.Command, args []string) error {
	serverIdentifier := args[0]

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	backups, err := client.ListBackups(serverIdentifier)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(backups, output.ResourceTypeAdminBackup)
}

// processIgnorePatterns processes ignore patterns from file or string flag.
func processIgnorePatterns(ignoreFile, ignoreStr string) (string, error) {
	if ignoreFile != "" {
		data, err := os.ReadFile(ignoreFile)
		if err != nil {
			return "", fmt.Errorf("failed to read ignore file: %w", err)
		}
		// Process file: trim whitespace, filter empty lines, join with newlines
		lines := strings.Split(string(data), "\n")
		var nonEmptyLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				nonEmptyLines = append(nonEmptyLines, trimmed)
			}
		}
		return strings.Join(nonEmptyLines, "\n"), nil
	}
	return ignoreStr, nil
}

// buildBackupData builds the backup request data map from flags.
func buildBackupData(name, ignorePatterns string, locked bool) map[string]any {
	backupData := make(map[string]any)
	if name != "" {
		backupData["name"] = name
	}
	if ignorePatterns != "" {
		backupData["ignored"] = ignorePatterns
	}
	if locked {
		backupData["is_locked"] = true
	}
	return backupData
}

// getBackupCreateServerUUIDs gets server UUIDs for backup creation.
func getBackupCreateServerUUIDs(cmd *cobra.Command, args []string, flags bulkFlags) ([]string, error) {
	uuids := args
	if flags.all || flags.fromFile != "" {
		var err error
		uuids, err = getServerUUIDs(cmd, args, flags.all, flags.fromFile)
		if err != nil {
			return nil, err
		}
	}
	if len(uuids) == 0 {
		return nil, errors.New("no servers specified")
	}
	return uuids, nil
}

// createBackupOperations creates bulk operations for backup creation.
func createBackupOperations(
	client *api.ApplicationAPI,
	uuids []string,
	backupData map[string]any,
	pairs *[]backupPair,
) []bulk.Operation {
	operations := make([]bulk.Operation, len(uuids))
	for i, uuid := range uuids {
		operations[i] = bulk.Operation{
			ID:   uuid,
			Name: uuid,
			Exec: func() error {
				backup, createErr := client.CreateBackup(uuid, backupData)
				if createErr != nil {
					return createErr
				}
				// Extract backup UUID from response
				if backupUUID, ok := backup["uuid"].(string); ok {
					*pairs = append(*pairs, backupPair{ServerID: uuid, BackupUUID: backupUUID})
				}
				return nil
			},
		}
	}
	return operations
}

// printBackupCreateResults prints the results of backup creation operations.
func printBackupCreateResults(formatter *output.Formatter, results []bulk.Result) {
	for _, result := range results {
		if result.Success {
			formatter.PrintSuccess("%s: backup created", result.Operation.ID)
		} else {
			formatter.PrintError("%s: %v", result.Operation.ID, result.Error)
		}
	}
}

// saveBackupPairs saves server+backup pairs to a file if requested.
func saveBackupPairs(formatter *output.Formatter, pairs []backupPair, savePairs string) error {
	if savePairs == "" || len(pairs) == 0 {
		return nil
	}

	var lines []string
	for _, pair := range pairs {
		lines = append(lines, fmt.Sprintf("%s,%s", pair.ServerID, pair.BackupUUID))
	}

	if writeErr := os.WriteFile(savePairs, []byte(strings.Join(lines, "\n")), 0600); writeErr != nil {
		formatter.PrintError("Failed to save pairs to file: %v", writeErr)
		return writeErr
	}

	formatter.PrintSuccess("Saved %d server+backup pairs to %s", len(pairs), savePairs)
	return nil
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	flags := getBulkFlags(cmd)

	// Get flags
	ignoreStr, _ := cmd.Flags().GetString("ignore")
	ignoreFile, _ := cmd.Flags().GetString("ignore-file")
	name, _ := cmd.Flags().GetString("name")
	locked, _ := cmd.Flags().GetBool("locked")
	savePairs, _ := cmd.Flags().GetString("save-pairs")

	// Process ignore patterns
	ignorePatterns, err := processIgnorePatterns(ignoreFile, ignoreStr)
	if err != nil {
		return err
	}

	// Build backup request data
	backupData := buildBackupData(name, ignorePatterns, locked)

	// Get server identifiers
	uuids, err := getBackupCreateServerUUIDs(cmd, args, flags)
	if err != nil {
		return err
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	// Handle dry run
	if flags.dryRun {
		formatter.PrintInfo("Dry run - would create backups for %d server(s):", len(uuids))
		for _, uuid := range uuids {
			formatter.PrintInfo("  - %s", uuid)
		}
		return nil
	}

	// Create API client
	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	// Store pairs for saving
	var pairs []backupPair

	// Create and execute operations
	ctx := context.Background()
	operations := createBackupOperations(client, uuids, backupData, &pairs)
	executor := bulk.NewExecutor(flags.maxConcurrency, flags.continueOnError, flags.failFast)
	results := executor.Execute(ctx, operations)

	// Print results
	printBackupCreateResults(formatter, results)

	// Save pairs if requested
	_ = saveBackupPairs(formatter, pairs, savePairs)

	// Print summary
	summary := bulk.GetSummary(results)
	if summary.Failed > 0 {
		return fmt.Errorf("%d backup creation(s) failed", summary.Failed)
	}

	return nil
}

type backupPair struct {
	ServerID   string
	BackupUUID string
}

func parseBackupPairsFromFile(fromFile string) ([]backupPair, error) {
	data, err := os.ReadFile(fromFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var pairs []backupPair
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != backupPairPartsCount {
			return nil, fmt.Errorf("line %d: expected format 'server-id,backup-uuid', got '%s'", i+1, line)
		}

		pairs = append(pairs, backupPair{
			ServerID:   strings.TrimSpace(parts[0]),
			BackupUUID: strings.TrimSpace(parts[1]),
		})
	}
	return pairs, nil
}

func parseBackupPairsFromArgs(args []string) ([]backupPair, error) {
	var pairs []backupPair
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return nil, errors.New("requires server+backup pairs (even number of arguments)")
		}
		pairs = append(pairs, backupPair{
			ServerID:   args[i],
			BackupUUID: args[i+1],
		})
	}
	return pairs, nil
}

func runBackupView(cmd *cobra.Command, args []string) error {
	fromFile, _ := cmd.Flags().GetString("from-file")

	var pairs []backupPair
	var err error
	if fromFile != "" {
		pairs, err = parseBackupPairsFromFile(fromFile)
	} else {
		pairs, err = parseBackupPairsFromArgs(args)
	}
	if err != nil {
		return err
	}

	if len(pairs) == 0 {
		return errors.New("no backup pairs specified")
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	// Fetch all backups
	var allBackups []map[string]any
	for _, pair := range pairs {
		backup, getErr := client.GetBackup(pair.ServerID, pair.BackupUUID)
		if getErr != nil {
			formatter.PrintError("%s/%s: %v", pair.ServerID, pair.BackupUUID, getErr)
			continue
		}
		allBackups = append(allBackups, backup)
	}

	if len(allBackups) == 0 {
		return errors.New("no backups found")
	}

	return formatter.PrintWithConfig(allBackups, output.ResourceTypeAdminBackup)
}

func runBackupDelete(cmd *cobra.Command, args []string) error {
	serverIdentifier := args[0]
	backupUUID := args[1]

	client, err := api.NewApplicationAPI()
	if err != nil {
		return err
	}

	err = client.DeleteBackup(serverIdentifier, backupUUID)
	if err != nil {
		// Return formatted error message directly to avoid duplicate printing
		return errors.New(apierrors.HandleError(err))
	}

	// Only show success message if no error was returned.
	// API returns 204 No Content on success, so if we get here, deletion succeeded.
	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("Backup deleted successfully")
	return nil
}

// getOutputFormat is defined in common.go
