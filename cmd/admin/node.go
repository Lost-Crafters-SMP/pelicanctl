package admin

import (
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

func newNodeCmd() *cobra.Command {
	return newResourceCmd(resourceCommandConfig{
		name:      "node",
		short:     "Manage nodes",
		long:      "List and view nodes",
		listShort: "List all nodes",
		listRunE: makeListRunE(func(c *api.ApplicationAPI) (any, error) {
			return c.ListNodes()
		}, output.ResourceTypeAdminNode),
		viewUse:   "view <node-id>",
		viewShort: "View node details",
		viewRunE: makeViewRunE(func(c *api.ApplicationAPI, id string) (any, error) {
			return c.GetNode(id)
		}),
		completeFunc: completion.CompleteNodes,
	})
}

// getOutputFormat is defined in common.go
