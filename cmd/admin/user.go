//nolint:dupl // Acceptable duplication: same pattern with different resource-specific values
package admin

import (
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/internal/api"
	"go.lostcrafters.com/pelicanctl/internal/completion"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

func newUserCmd() *cobra.Command {
	return newCRUDResourceCmd(crudResourceConfig{
		name:          "user",
		short:         "Manage users",
		long:          "List and view users",
		listShort:     "List all users",
		listFunc:      func(c *api.ApplicationAPI) (any, error) { return c.ListUsers() },
		viewUse:       "view <user-id>",
		viewShort:     "View user details",
		viewFunc:      func(c *api.ApplicationAPI, id string) (any, error) { return c.GetUser(id) },
		createFunc:    func(c *api.ApplicationAPI, data map[string]any) (map[string]any, error) { return c.CreateUser(data) },
		updateFunc:    func(c *api.ApplicationAPI, id string) (map[string]any, error) { return c.UpdateUser(id) },
		deleteFunc:    func(c *api.ApplicationAPI, id string) error { return c.DeleteUser(id) },
		completeFunc:  completion.CompleteUsers,
		resourceType:  output.ResourceTypeAdminUser,
		createMessage: "User created successfully",
		updateMessage: "User updated successfully",
		deleteMessage: "User deleted successfully",
		createLong:    "Create a new user. Provide user data as JSON via --data flag or stdin.",
		dataFlagHelp:  "JSON data for the user (or read from stdin)",
	})
}
