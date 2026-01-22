//nolint:dupl // Acceptable duplication: same pattern with different resource-specific values
package admin

import (
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

func newNodeCmd() *cobra.Command {
	return newCRUDResourceCmd(crudResourceConfig{
		name:          "node",
		short:         "Manage nodes",
		long:          "List and view nodes",
		listShort:     "List all nodes",
		listFunc:      func(c *api.ApplicationAPI) (any, error) { return c.ListNodes() },
		viewUse:       "view <node-id>",
		viewShort:     "View node details",
		viewFunc:      func(c *api.ApplicationAPI, id string) (any, error) { return c.GetNode(id) },
		createFunc:    func(c *api.ApplicationAPI, data map[string]any) (map[string]any, error) { return c.CreateNode(data) },
		updateFunc:    func(c *api.ApplicationAPI, id string) (map[string]any, error) { return c.UpdateNode(id) },
		deleteFunc:    func(c *api.ApplicationAPI, id string) error { return c.DeleteNode(id) },
		completeFunc:  completion.CompleteNodes,
		resourceType:  output.ResourceTypeAdminNode,
		createMessage: "Node created successfully",
		updateMessage: "Node updated successfully",
		deleteMessage: "Node deleted successfully",
		createLong:    "Create a new node. Provide node data as JSON via --data flag or stdin.",
		dataFlagHelp:  "JSON data for the node (or read from stdin)",
	})
}
