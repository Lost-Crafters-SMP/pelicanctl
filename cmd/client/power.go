package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	"go.lostcrafters.com/pelican-cli/internal/bulk"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func setupBulkFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on all servers")
	cmd.Flags().String("from-file", "", "read server IDs or UUIDs from file (one per line)")
	const defaultMaxConcurrency = 10
	cmd.Flags().Int("max-concurrency", defaultMaxConcurrency, "maximum parallel operations")
	cmd.Flags().Bool("continue-on-error", true, "continue on errors")
	cmd.Flags().Bool("fail-fast", false, "stop on first error")
	cmd.Flags().Bool("dry-run", false, "preview operations without executing")
	cmd.Flags().Bool("yes", false, "skip confirmation prompts")
}

type powerCommandConfig struct {
	use    string
	short  string
	action string
}

func createPowerSubcommand(config powerCommandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   config.use,
		Short: config.short,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			fromFile, _ := cmd.Flags().GetString("from-file")
			maxConcurrency, _ := cmd.Flags().GetInt("max-concurrency")
			continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
			failFast, _ := cmd.Flags().GetBool("fail-fast")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")

			return runPowerCommand(
				cmd, args, config.action, all, fromFile, maxConcurrency,
				continueOnError, failFast, dryRun, yes)
		},
	}
	setupBulkFlags(cmd)
	return cmd
}

func newPowerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "power",
		Short: "Control server power",
		Long:  "Start, stop, restart, or kill servers",
	}

	powerCommands := []powerCommandConfig{
		{"start <uuid>...", "Start server(s)", "start"},
		{"stop <uuid>...", "Stop server(s)", "stop"},
		{"restart <uuid>...", "Restart server(s)", "restart"},
		{"kill <uuid>...", "Kill server(s)", "kill"},
	}

	for _, pc := range powerCommands {
		cmd.AddCommand(createPowerSubcommand(pc))
	}

	return cmd
}

func handlePowerConfirmation(formatter *output.Formatter, command string, uuidCount int, yes bool) (bool, error) {
	if yes {
		return true, nil
	}

	needsConfirmation := command == "kill" || (command == "stop" && uuidCount > 1)
	if !needsConfirmation {
		return true, nil
	}

	formatter.PrintInfo("This will %s %d server(s). Continue? (y/N): ", command, uuidCount)
	var response string
	if _, scanErr := fmt.Scanln(&response); scanErr != nil {
		return false, fmt.Errorf("failed to read response: %w", scanErr)
	}

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		return false, nil
	}

	return true, nil
}

func handlePowerDryRun(formatter *output.Formatter, command string, uuids []string) {
	formatter.PrintInfo("Dry run - would %s %d server(s):", command, len(uuids))
	for _, uuid := range uuids {
		formatter.PrintInfo("  - %s", uuid)
	}
}

func executePowerOperations(
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
				return client.SendPowerCommand(uuid, command)
			},
		}
	}

	executor := bulk.NewExecutor(maxConcurrency, continueOnError, failFast)
	return executor.Execute(ctx, operations)
}

func printPowerResults(formatter *output.Formatter, results []bulk.Result, command string) {
	for _, result := range results {
		if result.Success {
			formatter.PrintSuccess("%s: %s", result.Operation.ID, command)
		} else {
			formatter.PrintError("%s: %v", result.Operation.ID, result.Error)
		}
	}
}

func handlePowerSummary(formatter *output.Formatter, results []bulk.Result) error {
	summary := bulk.GetSummary(results)
	formatter.PrintInfo("Summary: %d succeeded, %d failed", summary.Success, summary.Failed)

	if summary.Failed > 0 {
		return fmt.Errorf("%d operation(s) failed", summary.Failed)
	}

	return nil
}

func runPowerCommand(
	cmd *cobra.Command,
	args []string,
	command string,
	all bool,
	fromFile string,
	maxConcurrency int,
	continueOnError bool,
	failFast bool,
	dryRun bool,
	yes bool,
) error {
	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)

	uuids, err := getServerUUIDs(cmd, args, all, fromFile)
	if err != nil {
		return err
	}

	if len(uuids) == 0 {
		return errors.New("no servers specified")
	}

	shouldContinue, err := handlePowerConfirmation(formatter, command, len(uuids), yes)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	if dryRun {
		handlePowerDryRun(formatter, command, uuids)
		return nil
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	ctx := context.Background()
	results := executePowerOperations(ctx, client, uuids, command, maxConcurrency, continueOnError, failFast)

	printPowerResults(formatter, results, command)

	return handlePowerSummary(formatter, results)
}

func getServerUUIDs(_ *cobra.Command, args []string, all bool, fromFile string) ([]string, error) {
	var uuids []string

	switch {
	case all:
		client, err := api.NewClientAPI()
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
