package tools

// AuthTool exposes authentication operations for login flows, token management,
// and terms acceptance.
var AuthTool = GroupedToolDefinition{
	Name:        "auth",
	Description: "Authentication operations for login flows, token management, and terms acceptance.",
	Operations: map[string]ToolOperation{
		"login": {
			Method:      "POST",
			Path:        "/access-tokens",
			NoAuth:      true,
			Description: "Authenticate with email/username and password",
		},
		"logout": {
			Method:      "DELETE",
			Path:        "/access-tokens/me",
			Description: "Logout current user",
		},
		"acceptTerms": {
			Method:      "POST",
			Path:        "/access-tokens/accept-terms",
			NoAuth:      true,
			Description: "Accept terms during authentication flow",
		},
		"oidcExchange": {
			Method:      "POST",
			Path:        "/access-tokens/exchange-with-oidc",
			NoAuth:      true,
			Description: "Exchange OIDC code for access token",
		},
		"revokePending": {
			Method:      "POST",
			Path:        "/access-tokens/revoke-pending-token",
			NoAuth:      true,
			Description: "Revoke pending authentication token",
		},
		"getTerms": {
			Method:      "GET",
			Path:        "/terms",
			NoAuth:      true,
			Description: "Get terms and conditions",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"login", "logout", "acceptTerms", "oidcExchange", "revokePending", "getTerms"},
		map[string]string{
			"login":         "Login with credentials",
			"logout":        "Logout current session",
			"acceptTerms":   "Accept terms during auth",
			"oidcExchange":  "Exchange OIDC code",
			"revokePending": "Revoke pending token",
			"getTerms":      "Get terms document",
		},
		&SchemaParams{
			Data: &ParamSpec{
				Description: "Auth data: { emailOrUsername?: string + password?: string (for login), pendingToken?: string (for acceptTerms/revokePending), signature?: string (for acceptTerms), code?: string + nonce?: string (for oidcExchange) }",
				RequiredFor: []string{"login", "acceptTerms", "oidcExchange", "revokePending"},
			},
			Query: map[string]QuerySpec{
				"language": {Type: "string", Description: "Language code for terms"},
			},
		},
	),
}

// ProjectsTool manages Planka projects, the top-level containers for boards.
var ProjectsTool = GroupedToolDefinition{
	Name:        "projects",
	Description: "Manage Planka projects. Projects are top-level containers for boards.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/projects",
			Description: "List all projects accessible to the current user",
		},
		"get": {
			Method:      "GET",
			Path:        "/projects/{id}",
			Description: "Get detailed project information including boards and memberships",
		},
		"create": {
			Method:      "POST",
			Path:        "/projects",
			Description: "Create a new project (you become the project manager)",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/projects/{id}",
			Description: "Update project settings",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/projects/{id}",
			Description: "Delete a project and all its data",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "get", "create", "update", "delete"},
		map[string]string{
			"list":   "List all accessible projects",
			"get":    "Get project details by ID",
			"create": "Create a new project",
			"update": "Update project settings",
			"delete": "Delete a project",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Project ID",
				RequiredFor: []string{"get", "update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Project data: { name: string, type: 'private'|'shared' (required for create; create only), description?: string. Update only: backgroundType?: 'gradient'|'image', backgroundGradient?: string (one of: old-lime, ocean-dive, tzepesch-style, jungle-mesh, strawberry-dust, purple-rose, sun-scream, warm-rust, sky-change, green-eyes, blue-xchange, blood-orange, sour-peel, green-ninja, algae-green, coral-reef, steel-grey, heat-waves, velvet-lounge, purple-rain, blue-steel, blueish-curve, prism-light, green-mist, red-curtain), backgroundImageId?: string (id from backgroundImages.upload; required with backgroundType:'image'; null to clear), isFavorite?: boolean, isHidden?: boolean }",
				RequiredFor: []string{"create", "update"},
			},
		},
	),
}

// BoardsTool manages Planka boards, which contain lists and cards.
var BoardsTool = GroupedToolDefinition{
	Name:        "boards",
	Description: "Manage Planka boards. Boards contain lists and cards for organizing work.",
	Operations: map[string]ToolOperation{
		"get": {
			Method:      "GET",
			Path:        "/boards/{id}",
			Description: "Get board with lists, cards, labels, and memberships",
		},
		"create": {
			Method:      "POST",
			Path:        "/projects/{projectId}/boards",
			Description: "Create a new board in a project",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/boards/{id}",
			Description: "Update board settings",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/boards/{id}",
			Description: "Delete a board and all its data",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"get", "create", "update", "delete"},
		map[string]string{
			"get":    "Get board details by ID",
			"create": "Create a new board in a project",
			"update": "Update board settings",
			"delete": "Delete a board",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Board ID (for get, update, delete) or Project ID (for create, use projectId in data)",
				RequiredFor: []string{"get", "update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Board data: { name: string, position: number (required for create), projectId?: string (for create), defaultView?: 'kanban'|'grid'|'list' (update only), defaultCardType?: 'project'|'story' (update only) }",
				RequiredFor: []string{"create", "update"},
			},
		},
	),
}

// ListsTool manages Planka lists, the columns on a board that contain cards.
var ListsTool = GroupedToolDefinition{
	Name:        "lists",
	Description: "Manage Planka lists. Lists are columns on a board that contain cards.",
	Operations: map[string]ToolOperation{
		"get": {
			Method:      "GET",
			Path:        "/lists/{id}",
			Description: "Get a list with its cards",
		},
		"create": {
			Method:      "POST",
			Path:        "/boards/{boardId}/lists",
			Description: "Create a new list on a board",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/lists/{id}",
			Description: "Update list name, position, or type",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/lists/{id}",
			Description: "Delete a list (cards move to trash)",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"get", "create", "update", "delete"},
		map[string]string{
			"get":    "Get list details and cards",
			"create": "Create a new list on a board",
			"update": "Update list settings",
			"delete": "Delete a list",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "List ID (for get, update, delete) or Board ID (for create, use boardId in data)",
				RequiredFor: []string{"get", "update", "delete"},
			},
			Data: &ParamSpec{
				Description: "List data: { name: string, type: 'active'|'closed' (required for create), position: number (required for create), boardId?: string (for create/move), color?: string|null (update only; one of: berry-red, pumpkin-orange, lagoon-blue, pink-tulip, light-mud, orange-peel, bright-moss, antique-blue, dark-granite, turquoise-sea) }",
				RequiredFor: []string{"create", "update"},
			},
		},
	),
}

// CardsTool manages Planka cards, the individual work items on a board.
var CardsTool = GroupedToolDefinition{
	Name:        "cards",
	Description: "Manage Planka cards. Cards are individual work items on a board.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/lists/{listId}/cards",
			Description: "Get cards from a list with filtering, search, and pagination",
		},
		"get": {
			Method:      "GET",
			Path:        "/cards/{id}",
			Description: "Get card with task lists, attachments, and custom fields",
		},
		"create": {
			Method:      "POST",
			Path:        "/lists/{listId}/cards",
			Description: "Create a new card in a list",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/cards/{id}",
			Description: "Update card properties (can move between lists)",
		},
		"delete": {
			Method:      "DELETE",
			Path:        "/cards/{id}",
			Description: "Delete a card permanently",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "get", "create", "update", "delete"},
		map[string]string{
			"list":   "Get cards from a list (requires listId)",
			"get":    "Get card details by ID",
			"create": "Create a new card",
			"update": "Update card properties",
			"delete": "Delete a card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID (for get, update, delete) or List ID (for list, create)",
				RequiredFor: []string{"list", "get", "update", "delete"},
			},
			Data: &ParamSpec{
				Description: "Card data: { name: string, type: 'project'|'story' (required for create), listId?: string (for create/move), boardId?: string (update only: move to another board), position?: number (required when moving to a new list), description?: string, dueDate?: string, isDueCompleted?: boolean, coverAttachmentId?: string (set the card's cover to an uploaded image attachment; null to clear), stopwatch?: { startedAt: string, total: number } }",
				RequiredFor: []string{"create", "update"},
			},
			Query: map[string]QuerySpec{
				"search":   {Type: "string", Description: "Search term to filter cards"},
				"userIds":  {Type: "string", Description: "Comma-separated user IDs to filter by"},
				"labelIds": {Type: "string", Description: "Comma-separated label IDs to filter by"},
				"before": {
					Type:        "object",
					Description: "Pagination cursor: pass BOTH id and listChangedAt from the last card of the previous page",
					Properties: map[string]any{
						"id":            map[string]any{"type": "string", "description": "Card ID from the last result"},
						"listChangedAt": map[string]any{"type": "string", "description": "listChangedAt timestamp from the last result"},
					},
				},
			},
		},
	),
}

// CommentsTool manages comments on Planka cards.
var CommentsTool = GroupedToolDefinition{
	Name:        "comments",
	Description: "Manage comments on Planka cards.",
	Operations: map[string]ToolOperation{
		"list": {
			Method:      "GET",
			Path:        "/cards/{cardId}/comments",
			Description: "Get comments for a card with pagination",
		},
		"create": {
			Method:      "POST",
			Path:        "/cards/{cardId}/comments",
			Description: "Add a comment to a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"list", "create"},
		map[string]string{
			"list":   "Get comments for a card",
			"create": "Add a comment to a card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID to get/add comments",
				RequiredFor: []string{"list", "create"},
			},
			Data: &ParamSpec{
				Description: "Comment data: { text: string }",
				RequiredFor: []string{"create"},
			},
			Query: map[string]QuerySpec{
				"beforeId": {Type: "string", Description: "Get comments before this ID (pagination)"},
			},
		},
	),
}

// TasksTool manages task lists and tasks (checklist items) on Planka cards.
var TasksTool = GroupedToolDefinition{
	Name:        "tasks",
	Description: "Manage task lists and tasks on Planka cards. Tasks are checklist items within a card.",
	Operations: map[string]ToolOperation{
		"getList": {
			Method:      "GET",
			Path:        "/task-lists/{id}",
			Description: "Get a task list with all its tasks",
		},
		"createList": {
			Method:      "POST",
			Path:        "/cards/{cardId}/task-lists",
			Description: "Create a new task list on a card",
		},
		"create": {
			Method:      "POST",
			Path:        "/task-lists/{taskListId}/tasks",
			Description: "Create a new task in a task list",
		},
		"update": {
			Method:      "PATCH",
			Path:        "/tasks/{id}",
			Description: "Update task (name, completion status, assignee)",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"getList", "createList", "create", "update"},
		map[string]string{
			"getList":    "Get a task list by ID",
			"createList": "Create a task list on a card",
			"create":     "Create a task in a task list",
			"update":     "Update a task",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Task List ID (for getList), Card ID (for createList), Task List ID (for create, use taskListId in data), or Task ID (for update)",
				RequiredFor: []string{"getList", "createList", "create", "update"},
			},
			Data: &ParamSpec{
				Description: "Data: { name?: string (required for createList; required for create unless linkedCardId is provided), position: number (required for createList/create), cardId?: string (for createList), taskListId?: string (for create, or on update to move the task to another list), isCompleted?: boolean, assigneeUserId?: string (update only), linkedCardId?: string (for create) }",
				RequiredFor: []string{"createList", "create", "update"},
			},
		},
	),
}

// LabelsTool manages labels on Planka boards and cards.
var LabelsTool = GroupedToolDefinition{
	Name:        "labels",
	Description: "Manage labels on Planka boards and cards. Labels help categorize and filter cards.",
	Operations: map[string]ToolOperation{
		"create": {
			Method:      "POST",
			Path:        "/boards/{boardId}/labels",
			Description: "Create a new label on a board",
		},
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
		"addToCard": {
			Method:      "POST",
			Path:        "/cards/{cardId}/card-labels",
			Description: "Add a label to a card",
		},
		"removeFromCard": {
			Method:      "DELETE",
			Path:        "/cards/{cardId}/card-labels/labelId:{labelId}",
			Description: "Remove a label from a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"create", "update", "delete", "addToCard", "removeFromCard"},
		map[string]string{
			"create":         "Create a label on a board",
			"update":         "Update a label's name, color, or position",
			"delete":         "Delete a label from a board",
			"addToCard":      "Add a label to a card",
			"removeFromCard": "Remove a label from a card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Board ID (for create), Label ID (for update, delete), or Card ID (for addToCard, removeFromCard)",
				RequiredFor: []string{"create", "update", "delete", "addToCard", "removeFromCard"},
			},
			Data: &ParamSpec{
				Description: "Label data: { name?: string, color: string, position: number } for create/update, { labelId: string } for addToCard/removeFromCard. Colors: muddy-grey, autumn-leafs, morning-sky, antique-blue, egg-yellow, desert-sand, dark-granite, fresh-salad, lagoon-blue, midnight-blue, light-orange, pumpkin-orange, light-concrete, sunny-grass, navy-blue, lilac-eyes, apricot-red, orange-peel, silver-glint, bright-moss, deep-ocean, summer-sky, berry-red, light-cocoa, grey-stone, tank-green, coral-green, sugar-plum, pink-tulip, shady-rust, wet-rock, wet-moss, turquoise-sea, lavender-fields, piggy-red, light-mud, gun-metal, modern-green, french-coast, sweet-lilac, red-burgundy, pirate-gold",
				RequiredFor: []string{"create", "update", "addToCard", "removeFromCard"},
			},
		},
	),
}

// CardMembersTool manages user assignments on Planka cards.
var CardMembersTool = GroupedToolDefinition{
	Name:        "cardMembers",
	Description: "Manage user assignments on Planka cards.",
	Operations: map[string]ToolOperation{
		"add": {
			Method:      "POST",
			Path:        "/cards/{cardId}/card-memberships",
			Description: "Assign a user to a card",
		},
		"remove": {
			Method:      "DELETE",
			Path:        "/cards/{cardId}/card-memberships/userId:{userId}",
			Description: "Remove a user from a card",
		},
	},
	InputSchema: BuildGroupedSchema(
		[]string{"add", "remove"},
		map[string]string{
			"add":    "Assign a user to a card",
			"remove": "Remove a user from a card",
		},
		&SchemaParams{
			ID: &ParamSpec{
				Description: "Card ID",
				RequiredFor: []string{"add", "remove"},
			},
			Data: &ParamSpec{
				Description: "Membership data: { userId: string }",
				RequiredFor: []string{"add", "remove"},
			},
		},
	),
}

// BootstrapTool retrieves Planka application initialization data.
var BootstrapTool = GroupedToolDefinition{
	Name:        "bootstrap",
	Description: "Get Planka application bootstrap data including current user, projects, boards, and notifications.",
	Operations: map[string]ToolOperation{
		"get": {
			Method:      "GET",
			Path:        "/bootstrap",
			Description: "Get application bootstrap data",
		},
	},
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"get"},
				"description": "Action: 'get' - Retrieve bootstrap data",
			},
		},
		Required: []string{"action"},
	},
}

// CoreTools are the essential Kanban tools, always enabled.
var CoreTools = []GroupedToolDefinition{
	AuthTool,
	BootstrapTool,
	ProjectsTool,
	BoardsTool,
	ListsTool,
	CardsTool,
	CommentsTool,
	TasksTool,
	LabelsTool,
	CardMembersTool,
}
