package tools

// ConfigTool manages Planka application configuration (admin only).
var ConfigTool = GroupedToolDefinition{
	Name:        "config",
	Description: "Manage Planka application configuration. Requires admin privileges.",
	Operations: map[string]ToolOperation{
		"get": {
			Method:      "GET",
			Path:        "/config",
			Description: "Get application configuration including SMTP settings",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/config",
			Description: "Update application configuration",
		},
		"testSmtp": {
			Method:      "POST",
			Path:        "/config/test-smtp",
			Description: "Send a test email to verify SMTP configuration",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"get", "update", "testSmtp"},
		map[string]string{
			"get":      "Get application configuration",
			"update":   "Update application configuration",
			"testSmtp": "Test SMTP configuration by sending a test email",
		},
		&SchemaParams{
			Data: &ParamSpec{
				Description: "Config data: { smtpHost?: string, smtpPort?: number, smtpName?: string, smtpUser?: string, smtpPassword?: string, smtpFrom?: string, smtpSecure?: boolean, smtpTlsRejectUnauthorized?: boolean }",
				RequiredFor: []string{"update"},
			},
		},
	),
}

// UsersTool manages Planka user accounts (admin only for most operations).
var UsersTool = GroupedToolDefinition{
	Name:        "users",
	Description: "Manage Planka user accounts. Requires admin privileges for most operations.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/users",
			Description: "List all users",
		},
		"create": {
			Method:      "POST",
			Path:        "/users",
			Description: "Create a new user account",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/users/{id}",
			Description: "Update user profile",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/users/{id}",
			Description: "Delete a user account",
		},
		"updateEmail": {
			Method:      "PATCH",
			Path:        "/users/{id}/email",
			Description: "Update user's email address",
		},
		"updatePassword": {
			Method:      "PATCH",
			Path:        "/users/{id}/password",
			Description: "Update user's password",
		},
		"updateUsername": {
			Method:      "PATCH",
			Path:        "/users/{id}/username",
			Description: "Update user's username",
		},
		"updateAvatar": {
			Method:      "POST",
			Path:        "/users/{id}/avatar",
			Description: "Update user's avatar image",
			Upload:      "file",
		},
		"createApiKey": {
			Method:      "POST",
			Path:        "/users/{id}/api-key",
			Description: "Generate an API key for a user",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "create", "update", "delete", "updateEmail", "updatePassword", "updateUsername", "updateAvatar", "createApiKey"},
		map[string]string{ //nolint:gosec // G101 false positive: these are human-readable action descriptions, not credentials.
			"list":           "List all users",
			"create":         "Create a new user",
			"update":         "Update user profile",
			"delete":         "Delete a user",
			"updateEmail":    "Update user's email",
			"updatePassword": "Update user's password",
			"updateUsername": "Update user's username",
			"updateAvatar":   "Update user's avatar",
			"createApiKey":   "Generate API key for user",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "User ID",
				RequiredFor: []string{"update", "delete", "updateEmail", "updatePassword", "updateUsername", "updateAvatar", "createApiKey"},
			},
			Data: &ParamSpec{
				Description: "User data. For create: { email: string (required), password: string (required), name: string (required), role: 'admin'|'projectOwner'|'boardUser' (required), username?: string }. For update: { name?: string, role?: string, isDeactivated?: boolean } — email/password/username can NOT be changed via update; use updateEmail { email }, updatePassword { password }, updateUsername { username }, each plus currentPassword?: string (required when changing your own account). For updateAvatar: { url: string (image fetched server-side) }.",
				RequiredFor: []string{"create", "update", "updateEmail", "updatePassword", "updateUsername", "updateAvatar"},
			},
		},
	),
}

// WebhooksTool manages Planka webhooks for event notifications (admin only).
var WebhooksTool = GroupedToolDefinition{
	Name:        "webhooks",
	Description: "Manage Planka webhooks for event notifications. Requires admin privileges.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/webhooks",
			Description: "List all configured webhooks",
		},
		"create": {
			Method:      "POST",
			Path:        "/webhooks",
			Description: "Create a new webhook",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/webhooks/{id}",
			Description: "Update webhook configuration",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/webhooks/{id}",
			Description: "Delete a webhook",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "create", "update", "delete"},
		map[string]string{
			"list":   "List all webhooks",
			"create": "Create a new webhook",
			"update": "Update webhook configuration",
			"delete": "Delete a webhook",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Webhook ID",
				RequiredFor: []string{"update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Webhook data: { name: string (required for create), url: string (required for create), accessToken?: string, events?: string (comma-separated, e.g. 'cardCreate,cardUpdate'), excludedEvents?: string (comma-separated) }",
				RequiredFor: []string{"create", "update"},
			},
		},
	),
}

// ProjectManagersTool manages project manager assignments (admin only for
// shared projects).
var ProjectManagersTool = GroupedToolDefinition{
	Name:        "projectManagers",
	Description: "Manage project manager assignments. Requires admin privileges for shared projects.",
	Operations: map[string]ToolOperation{
		"add": {
			Method:      "POST",
			Path:        "/projects/{projectId}/project-managers",
			Description: "Add a user as project manager",
		},
		"remove": {
			Method:      "DELETE",
			Path:        "/project-managers/{id}",
			Description: "Remove a project manager",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"add", "remove"},
		map[string]string{
			"add":    "Add a project manager",
			"remove": "Remove a project manager",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Project ID (for add) or Project Manager ID (for remove)",
				RequiredFor: []string{"add", "remove"},
			},
			Data: &ParamSpec{
				Description: "Manager data: { userId: string }",
				RequiredFor: []string{"add"},
			},
		},
	),
}

// AdminTools require admin privileges and are enabled via ENABLE_ADMIN_TOOLS.
var AdminTools = []GroupedToolDefinition{
	ConfigTool,
	UsersTool,
	WebhooksTool,
	ProjectManagersTool,
}
