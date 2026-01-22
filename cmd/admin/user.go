package admin

import (
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/internal/api"
	"go.lostcrafters.com/pelican-cli/internal/completion"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

func newUserCmd() *cobra.Command {
	return newResourceCmd(resourceCommandConfig{
		name:      "user",
		short:     "Manage users",
		long:      "List and view users",
		listShort: "List all users",
		listRunE: makeListRunE(func(c *api.ApplicationAPI) (any, error) {
			return c.ListUsers()
		}, output.ResourceTypeAdminUser),
		viewUse:   "view <user-id>",
		viewShort: "View user details",
		viewRunE: makeViewRunE(func(c *api.ApplicationAPI, id string) (any, error) {
			return c.GetUser(id)
		}),
		completeFunc: completion.CompleteUsers,
	})
}
