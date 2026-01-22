package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

func clientServerCompletionAction(c carapace.Context) carapace.Action {
	completions, err := completion.CompleteServers("client", c.Value)
	if err != nil || len(completions) == 0 {
		return carapace.ActionValues()
	}
	return carapace.ActionValues(completions...)
}

func clientFileCompletionAction(serverUUID string) carapace.Action {
	return carapace.ActionCallback(func(c carapace.Context) carapace.Action {
		completions, err := completion.CompleteFiles(serverUUID, "", c.Value)
		if err != nil || len(completions) == 0 {
			return carapace.ActionValues()
		}
		return carapace.ActionValues(completions...)
	})
}

func clientServerValidArgsFunction(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	completions, err := completion.CompleteServers("client", toComplete)
	if err != nil || len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func clientFileValidArgsFunction(
	serverUUID string,
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completions, err := completion.CompleteFiles(serverUUID, "", toComplete)
		if err != nil || len(completions) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

func setupListCmdCompletion(cmd *cobra.Command) {
	carapace.Gen(cmd).PositionalCompletion(
		carapace.ActionCallback(clientServerCompletionAction),
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			if len(c.Args) > 0 {
				return clientFileCompletionAction(c.Args[0])
			}
			return carapace.ActionValues()
		}),
	)
}

func setupDownloadCmdCompletion(cmd *cobra.Command) {
	carapace.Gen(cmd).PositionalCompletion(
		carapace.ActionCallback(clientServerCompletionAction),
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			if len(c.Args) > 0 {
				return clientFileCompletionAction(c.Args[0])
			}
			return carapace.ActionValues()
		}),
		carapace.ActionFiles(), // Third argument: local path (file system)
	)
}

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage server files",
		Long:  "List, download, and upload server files",
	}

	listCmd := &cobra.Command{
		Use:   "list <id|uuid> [directory]",
		Short: "List files in a directory",
		Long:  "List files in a directory for a server by ID (integer) or UUID (string)",
		Args:  cobra.RangeArgs(1, 2), //nolint:mnd // Valid range for optional directory argument
		RunE:  runFileList,
	}
	listCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return clientServerValidArgsFunction(nil, nil, toComplete)
		}
		return clientFileValidArgsFunction(args[0])(nil, nil, toComplete)
	}

	downloadCmd := &cobra.Command{
		Use:   "download <id|uuid> <remote-path> [local-path]",
		Short: "Download a file from the server",
		Long:  "Download a file from a server by ID (integer) or UUID (string)",
		Args:  cobra.RangeArgs(2, 3), //nolint:mnd // Valid range for optional local-path argument
		RunE:  runFileDownload,
	}
	downloadCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return clientServerValidArgsFunction(nil, nil, toComplete)
		}
		if len(args) == 1 {
			return clientFileValidArgsFunction(args[0])(nil, nil, toComplete)
		}
		return nil, cobra.ShellCompDirectiveDefault
	}

	// Add subcommands FIRST (matching carapace example pattern)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(downloadCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	setupListCmdCompletion(listCmd)
	setupDownloadCmdCompletion(downloadCmd)

	return cmd
}

func runFileList(cmd *cobra.Command, args []string) error {
	serverUUID := args[0]
	directory := ""
	if len(args) > 1 {
		directory = args[1]
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	files, err := client.ListFiles(serverUUID, directory)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	return formatter.PrintWithConfig(files, output.ResourceTypeClientFile)
}

func runFileDownload(cmd *cobra.Command, args []string) error {
	serverUUID := args[0]
	remotePath := args[1]
	localPath := filepath.Base(remotePath)
	const maxArgsWithOptional = 3
	if len(args) > maxArgsWithOptional-1 {
		localPath = args[2]
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return err
	}

	reader, err := client.DownloadFile(serverUUID, remotePath)
	if err != nil {
		return fmt.Errorf("%s", apierrors.HandleError(err))
	}
	defer reader.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	if _, copyErr := io.Copy(localFile, reader); copyErr != nil {
		return fmt.Errorf("failed to write file: %w", copyErr)
	}

	formatter := output.NewFormatter(getOutputFormat(cmd), os.Stdout)
	formatter.PrintSuccess("Downloaded %s to %s", remotePath, localPath)
	return nil
}
