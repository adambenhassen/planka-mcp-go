package tools

// ActionsTool views action history for boards and cards.
var ActionsTool = GroupedToolDefinition{
	Name:        "actions",
	Description: "View action history for boards and cards.",
	Operations: map[string]ToolOperation{
		"boardActions": {
			Method:      "GET",
			Path:        "/boards/{boardId}/actions",
			Description: "Get action history for a board",
		},
		"cardActions": {
			Method:      "GET",
			Path:        "/cards/{cardId}/actions",
			Description: "Get action history for a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"boardActions", "cardActions"},
		map[string]string{
			"boardActions": "Get board action history",
			"cardActions":  "Get card action history",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Board ID (for boardActions) or Card ID (for cardActions)",
				RequiredFor: []string{"boardActions", "cardActions"},
			},
			Query: map[string]QuerySpec{
				"beforeId": {Type: "string", Description: "Get actions before this ID (pagination)"},
			},
		},
	),
}

// AttachmentsTool manages attachments on Planka cards.
var AttachmentsTool = GroupedToolDefinition{
	Name:        "attachments",
	Description: "Manage attachments on Planka cards.",
	Operations: map[string]ToolOperation{
		"create": {
			Method:      "POST",
			Path:        "/cards/{cardId}/attachments",
			Description: "Add an attachment to a card",
			Upload:      "attachment",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/attachments/{id}",
			Description: "Update attachment properties",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/attachments/{id}",
			Description: "Delete an attachment",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"create", "update", "delete"},
		map[string]string{
			"create": "Add attachment to card",
			"update": "Update attachment",
			"delete": "Delete attachment",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID (for create) or Attachment ID (for update, delete)",
				RequiredFor: []string{"create", "update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Attachment data. For an image/file: { type: 'file', name, url: string (image fetched server-side) }. For a link: { type: 'link', name, url }. To show an image on the card face, set it as the card cover afterward via cards.update { coverAttachmentId: <returned attachment id> }.",
				RequiredFor: []string{"create", "update"},
			},
		},
	),
}

// BoardMembersTool manages user memberships on Planka boards.
var BoardMembersTool = GroupedToolDefinition{
	Name:        "boardMembers",
	Description: "Manage user memberships on Planka boards.",
	Operations: map[string]ToolOperation{
		"add": {
			Method:      "POST",
			Path:        "/boards/{boardId}/board-memberships",
			Description: "Add a user to a board",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/board-memberships/{id}",
			Description: "Update membership role",
		},
		"remove": {
			Method:      "DELETE",
			Path:        "/board-memberships/{id}",
			Description: "Remove a user from a board",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"add", "update", "remove"},
		map[string]string{
			"add":    "Add user to board",
			"update": "Update membership role",
			"remove": "Remove user from board",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Board ID (for add) or Board Membership ID (for update, remove)",
				RequiredFor: []string{"add", "update", "remove"},
			},
			Data: &ParamSpec{
				Description: "Membership data: { userId: string, role: 'editor'|'viewer', canComment?: boolean }",
				RequiredFor: []string{"add", "update"},
			},
		},
	),
}

// CustomFieldsTool manages custom fields and field groups in Planka.
var CustomFieldsTool = GroupedToolDefinition{
	Name:        "customFields",
	Description: "Manage custom fields and field groups in Planka.",
	Operations: map[string]ToolOperation{
		"createBaseGroup": {
			Method:      "POST",
			Path:        "/projects/{projectId}/base-custom-field-groups",
			Description: "Create a base custom field group template",
		},
		"updateBaseGroup": {
			Method:      "PATCH",
			Path:        "/base-custom-field-groups/{id}",
			Description: "Update a base custom field group",
		},
		"deleteBaseGroup": {
			Method:      "DELETE",
			Path:        "/base-custom-field-groups/{id}",
			Description: "Delete a base custom field group",
		},
		"createBoardGroup": {
			Method:      "POST",
			Path:        "/boards/{boardId}/custom-field-groups",
			Description: "Create a custom field group on a board",
		},
		"createCardGroup": {
			Method:      "POST",
			Path:        "/cards/{cardId}/custom-field-groups",
			Description: "Create a custom field group on a card",
		},
		"getGroup": {
			Method:      "GET",
			Path:        "/custom-field-groups/{id}",
			Description: "Get a custom field group with fields",
		},
		"updateGroup": {
			Method:      "PATCH",
			Path:        "/custom-field-groups/{id}",
			Description: "Update a custom field group",
		},
		"deleteGroup": {
			Method:      "DELETE",
			Path:        "/custom-field-groups/{id}",
			Description: "Delete a custom field group",
		},
		"createFieldInBase": {
			Method:      "POST",
			Path:        "/base-custom-field-groups/{baseCustomFieldGroupId}/custom-fields",
			Description: "Create a field in a base group",
		},
		"createField": {
			Method:      "POST",
			Path:        "/custom-field-groups/{customFieldGroupId}/custom-fields",
			Description: "Create a field in a group",
		},
		"updateField": {
			Method:      "PATCH",
			Path:        "/custom-fields/{id}",
			Description: "Update a custom field",
		},
		"deleteField": {
			Method:      "DELETE",
			Path:        "/custom-fields/{id}",
			Description: "Delete a custom field",
		},
		"setValue": {
			Method:      "PATCH",
			Path:        "/cards/{cardId}/custom-field-values/customFieldGroupId:{customFieldGroupId}:customFieldId:{customFieldId}",
			Description: "Set a custom field value on a card",
		},
		"clearValue": {
			Method:      "DELETE",
			Path:        "/cards/{cardId}/custom-field-value/customFieldGroupId:{customFieldGroupId}:customFieldId:{customFieldId}",
			Description: "Clear a custom field value",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"createBaseGroup", "updateBaseGroup", "deleteBaseGroup", "createBoardGroup", "createCardGroup", "getGroup", "updateGroup", "deleteGroup", "createFieldInBase", "createField", "updateField", "deleteField", "setValue", "clearValue"},
		map[string]string{
			"createBaseGroup":   "Create base field group",
			"updateBaseGroup":   "Update base field group",
			"deleteBaseGroup":   "Delete base field group",
			"createBoardGroup":  "Create board field group",
			"createCardGroup":   "Create card field group",
			"getGroup":          "Get field group",
			"updateGroup":       "Update field group",
			"deleteGroup":       "Delete field group",
			"createFieldInBase": "Create field in base group",
			"createField":       "Create field in group",
			"updateField":       "Update field",
			"deleteField":       "Delete field",
			"setValue":          "Set field value",
			"clearValue":        "Clear field value",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Resource ID - varies by action (project, board, card, group, or field ID)",
				RequiredFor: []string{"createBaseGroup", "updateBaseGroup", "deleteBaseGroup", "createBoardGroup", "createCardGroup", "getGroup", "updateGroup", "deleteGroup", "createFieldInBase", "createField", "updateField", "deleteField", "setValue", "clearValue"},
			},
			Data: &ParamSpec{
				Description: "Field/group data: { name?: string (required for createBaseGroup/createFieldInBase/createField; required for createBoardGroup/createCardGroup unless baseCustomFieldGroupId is provided), position: number (required for createBoardGroup/createCardGroup/createFieldInBase/createField), content: string (required for setValue), showOnFrontOfCard?: boolean (for fields), baseCustomFieldGroupId?: string, customFieldGroupId?: string + customFieldId?: string (pass both in data for setValue/clearValue; id holds the card ID) }",
				RequiredFor: []string{"createBaseGroup", "updateBaseGroup", "createBoardGroup", "createCardGroup", "updateGroup", "createFieldInBase", "createField", "updateField", "setValue"},
			},
		},
	),
}

// NotificationsTool manages Planka notifications and notification services.
var NotificationsTool = GroupedToolDefinition{
	Name:        "notifications",
	Description: "Manage Planka notifications and notification services.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/notifications",
			Description: "Get all unread notifications",
		},
		"get": {
			Method:      "GET",
			Path:        "/notifications/{id}",
			Description: "Get a specific notification",
		},
		"markRead": {
			Method:      "PATCH",
			Path:        "/notifications/{id}",
			Description: "Mark a notification as read",
		},
		"markAllRead": {
			Method:      "POST",
			Path:        "/notifications/read-all",
			Description: "Mark all notifications as read",
		},
		"markCardRead": {
			Method:      "POST",
			Path:        "/cards/{id}/read-notifications",
			Description: "Mark all notifications for a card as read",
		},
		"createUserService": {
			Method:      "POST",
			Path:        "/users/{userId}/notification-services",
			Description: "Create a user notification service",
		},
		"createBoardService": {
			Method:      "POST",
			Path:        "/boards/{boardId}/notification-services",
			Description: "Create a board notification service",
		},
		"updateService": {
			Method:      "PATCH",
			Path:        "/notification-services/{id}",
			Description: "Update a notification service",
		},
		"deleteService": {
			Method:      "DELETE",
			Path:        "/notification-services/{id}",
			Description: "Delete a notification service",
		},
		"testService": {
			Method:      "POST",
			Path:        "/notification-services/{id}/test",
			Description: "Test a notification service",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "get", "markRead", "markAllRead", "markCardRead", "createUserService", "createBoardService", "updateService", "deleteService", "testService"},
		map[string]string{
			"list":               "List unread notifications",
			"get":                "Get notification",
			"markRead":           "Mark notification read",
			"markAllRead":        "Mark all read",
			"markCardRead":       "Mark card notifications read",
			"createUserService":  "Create user notification service",
			"createBoardService": "Create board notification service",
			"updateService":      "Update notification service",
			"deleteService":      "Delete notification service",
			"testService":        "Test notification service",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Notification ID, Card ID, User ID, Board ID, or Service ID depending on action",
				RequiredFor: []string{"get", "markRead", "markCardRead", "createUserService", "createBoardService", "updateService", "deleteService", "testService"},
			},
			Data: &ParamSpec{
				Description: "Service data: { url: string, format: 'text'|'markdown'|'html', isRead?: boolean }",
				RequiredFor: []string{"markRead", "createUserService", "createBoardService", "updateService"},
			},
		},
	),
}

// BackgroundImagesTool manages background images for Planka projects.
var BackgroundImagesTool = GroupedToolDefinition{
	Name:        "backgroundImages",
	Description: "Manage background images for Planka projects.",
	Operations: map[string]ToolOperation{
		"upload": {
			Method:      "POST",
			Path:        "/projects/{projectId}/background-images",
			Description: "Upload a background image",
			Upload:      "file",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/background-images/{id}",
			Description: "Delete a background image",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"upload", "delete"},
		map[string]string{
			"upload": "Upload background image",
			"delete": "Delete background image",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Project ID (for upload) or Background Image ID (for delete)",
				RequiredFor: []string{"upload", "delete"},
			},
			Data: &ParamSpec{
				Description: "Background image data (for upload): { url: string (fetched server-side), name? }. After upload, apply it via projects.update { backgroundType: 'image', backgroundImageId: <returned id> }.",
				RequiredFor: []string{"upload"},
			},
		},
	),
}

// CardExtrasTool provides extended card operations in Planka.
var CardExtrasTool = GroupedToolDefinition{
	Name:        "cardExtras",
	Description: "Extended card operations in Planka.",
	Operations: map[string]ToolOperation{
		"duplicate": {
			Method:      "POST",
			Path:        "/cards/{id}/duplicate",
			Description: "Duplicate a card with all content",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"duplicate"},
		map[string]string{
			"duplicate": "Duplicate a card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID to duplicate",
				RequiredFor: []string{"duplicate"},
			},
			Data: &ParamSpec{
				Description: "Duplicate options: { position?: number, name?: string, boardId?: string, listId?: string (target for the copy) }",
			},
		},
	),
}

// CommentExtrasTool provides extended comment operations in Planka.
var CommentExtrasTool = GroupedToolDefinition{
	Name:        "commentExtras",
	Description: "Extended comment operations in Planka.",
	Operations: map[string]ToolOperation{
		"update": {
			Method:      "PATCH",
			Path:        "/comments/{id}",
			Description: "Update a comment",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/comments/{id}",
			Description: "Delete a comment",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"update", "delete"},
		map[string]string{
			"update": "Update a comment",
			"delete": "Delete a comment",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Comment ID",
				RequiredFor: []string{"update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Comment data: { text: string }",
				RequiredFor: []string{"update"},
			},
		},
	),
}

// ListExtrasTool provides extended list operations in Planka.
var ListExtrasTool = GroupedToolDefinition{
	Name:        "listExtras",
	Description: "Extended list operations in Planka.",
	Operations: map[string]ToolOperation{
		"clear": {
			Method:      "POST",
			Path:        "/lists/{id}/clear",
			Description: "Move all cards in a list to trash",
		},
		"moveCards": {
			Method:      "POST",
			Path:        "/lists/{id}/move-cards",
			Description: "Move all cards to another list",
		},
		"sort": {
			Method:      "POST",
			Path:        "/lists/{id}/sort",
			Description: "Sort cards in a list",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"clear", "moveCards", "sort"},
		map[string]string{
			"clear":     "Clear list (trash all cards)",
			"moveCards": "Move all cards to another list",
			"sort":      "Sort cards in list",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "List ID",
				RequiredFor: []string{"clear", "moveCards", "sort"},
			},
			Data: &ParamSpec{
				Description: "Options: { listId?: string (target for moveCards), fieldName?: 'name'|'dueDate'|'createdAt' (for sort), order?: 'asc'|'desc' (for sort) }",
				RequiredFor: []string{"moveCards", "sort"},
			},
		},
	),
}

// TaskExtrasTool provides extended task and task-list operations in Planka.
var TaskExtrasTool = GroupedToolDefinition{
	Name:        "taskExtras",
	Description: "Extended task and task list operations in Planka.",
	Operations: map[string]ToolOperation{
		"updateList": {
			Method:      "PATCH",
			Path:        "/task-lists/{id}",
			Description: "Update a task list",
		},
		"deleteList": {
			Method:      "DELETE",
			Path:        "/task-lists/{id}",
			Description: "Delete a task list",
		},
		"deleteTask": {
			Method:      "DELETE",
			Path:        "/tasks/{id}",
			Description: "Delete a task",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"updateList", "deleteList", "deleteTask"},
		map[string]string{
			"updateList": "Update task list",
			"deleteList": "Delete task list",
			"deleteTask": "Delete task",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Task List ID (for updateList, deleteList) or Task ID (for deleteTask)",
				RequiredFor: []string{"updateList", "deleteList", "deleteTask"},
			},
			Data: &ParamSpec{
				Description: "Task list data: { name?: string, position?: number, hideCompletedTasks?: boolean, showOnFrontOfCard?: boolean }",
				RequiredFor: []string{"updateList"},
			},
		},
	),
}

// LabelExtrasTool provides extended label operations in Planka.
var LabelExtrasTool = GroupedToolDefinition{
	Name:        "labelExtras",
	Description: "Extended label operations in Planka.",
	Operations: map[string]ToolOperation{
		"update": {
			Method:      "PATCH",
			Path:        "/labels/{id}",
			Description: "Update a label",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/labels/{id}",
			Description: "Delete a label",
		},
		"removeFromCard": {
			Method:      "DELETE",
			Path:        "/cards/{cardId}/card-labels/labelId:{labelId}",
			Description: "Remove a label from a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"update", "delete", "removeFromCard"},
		map[string]string{
			"update":         "Update a label",
			"delete":         "Delete a label",
			"removeFromCard": "Remove label from card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Label ID (for update, delete) or Card ID (for removeFromCard)",
				RequiredFor: []string{"update", "delete", "removeFromCard"},
			},
			Data: &ParamSpec{
				Description: "Label data: { name?: string, color?: string, position?: number } for update; { labelId: string, cardId: string } for removeFromCard",
				RequiredFor: []string{"update", "removeFromCard"},
			},
		},
	),
}

// CardMemberExtrasTool provides extended card membership operations in Planka.
var CardMemberExtrasTool = GroupedToolDefinition{
	Name:        "cardMemberExtras",
	Description: "Extended card membership operations in Planka.",
	Operations: map[string]ToolOperation{
		"remove": {
			Method:      "DELETE",
			Path:        "/cards/{cardId}/card-memberships/userId:{userId}",
			Description: "Remove a user from a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"remove"},
		map[string]string{
			"remove": "Remove user from card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID",
				RequiredFor: []string{"remove"},
			},
			Data: &ParamSpec{
				Description: "Membership data: { userId: string, cardId: string }",
				RequiredFor: []string{"remove"},
			},
		},
	),
}

// UserInfoTool gets non-admin user profile information.
var UserInfoTool = GroupedToolDefinition{
	Name:        "userInfo",
	Description: "Get user profile information.",
	Operations: map[string]ToolOperation{
		"get": {
			Method:      "GET",
			Path:        "/users/{id}",
			Description: "Get a user's profile",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"get"},
		map[string]string{
			"get": "Get user profile",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "User ID",
				RequiredFor: []string{"get"},
			},
		},
	),
}

// OptionalTools provide extended functionality, enabled via ENABLE_OPTIONAL_TOOLS.
var OptionalTools = []GroupedToolDefinition{
	ActionsTool,
	AttachmentsTool,
	BoardMembersTool,
	CustomFieldsTool,
	NotificationsTool,
	BackgroundImagesTool,
	CardExtrasTool,
	CommentExtrasTool,
	ListExtrasTool,
	TaskExtrasTool,
	LabelExtrasTool,
	CardMemberExtrasTool,
	UserInfoTool,
}
