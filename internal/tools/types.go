// Package tools defines the grouped Planka MCP tool set: each tool bundles
// several related API operations under a single MCP tool with an "action"
// selector, mirroring the TypeScript @adambenhassen/planka-mcp definitions 1:1.
package tools

import (
	"maps"
	"strings"
)

// ToolOperation defines a single Planka API operation within a grouped tool.
type ToolOperation struct {
	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE).
	Method string
	// Path is the API path with parameter placeholders (e.g. /projects/{id}).
	Path string
	// NoAuth reports that the endpoint does not require authentication. The zero
	// value (false) means auth is required, matching the API's default.
	NoAuth bool
	// Description explains this specific operation.
	Description string
	// Upload, when set, makes the executor send multipart/form-data instead of
	// JSON: "attachment" (type/name/url fields + optional file part) or "file"
	// (bare file part). Empty means a normal JSON body.
	Upload string
}

// GroupedToolDefinition is a grouped tool exposing multiple operations under one
// MCP tool identifier.
type GroupedToolDefinition struct {
	// Name is the MCP tool identifier.
	Name string `json:"name"`
	// Description is a human-readable summary of what the tool does.
	Description string `json:"description"`
	// Operations maps action names to their API operations. It is never sent to
	// MCP clients (only name/description/inputSchema are).
	Operations map[string]ToolOperation `json:"-"`
	// InputSchema is the JSON Schema advertised to MCP clients.
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is the JSON Schema object advertised for a tool's arguments.
type InputSchema struct {
	// Type is always "object".
	Type string `json:"type"`
	// Properties holds the JSON Schema property definitions.
	Properties map[string]any `json:"properties"`
	// Required lists the required property names.
	Required []string `json:"required"`
}

// ParamSpec describes the id or data property injected into a tool's schema.
type ParamSpec struct {
	// Description is the base human-readable description.
	Description string
	// RequiredFor lists the actions the parameter is required for; when set it
	// is appended to Description as "(required for: ...)".
	RequiredFor []string
}

// QuerySpec describes a single query parameter property.
type QuerySpec struct {
	// Type is the JSON Schema type of the query parameter.
	Type string
	// Description explains the query parameter.
	Description string
	// Properties, when non-nil, holds nested object properties (e.g. a
	// pagination cursor).
	Properties map[string]any
}

// SchemaParams collects the optional pieces used to build a grouped schema.
type SchemaParams struct {
	// ID configures the optional "id" property.
	ID *ParamSpec
	// Data configures the optional "data" property.
	Data *ParamSpec
	// Query maps query-parameter names to their specs.
	Query map[string]QuerySpec
	// Extra holds additional top-level properties copied in verbatim.
	Extra map[string]any
}

// requiredForSuffix renders the "(required for: ...)" clause appended to id/data
// descriptions, or an empty string when no actions are listed.
func requiredForSuffix(requiredFor []string) string {
	if len(requiredFor) == 0 {
		return ""
	}
	return " (required for: " + strings.Join(requiredFor, ", ") + ")"
}

// BuildGroupedSchema builds the JSON Schema for a grouped tool from its action
// list, per-action descriptions, and optional id/data/query/extra parameters.
func BuildGroupedSchema(actions []string, actionDescriptions map[string]string, params *SchemaParams) InputSchema {
	descParts := make([]string, 0, len(actions))
	for _, a := range actions {
		descParts = append(descParts, "'"+a+"' - "+actionDescriptions[a])
	}

	properties := map[string]any{
		"action": map[string]any{
			"type":        "string",
			"enum":        actions,
			"description": "Action to perform: " + strings.Join(descParts, "; "),
		},
	}

	if params != nil {
		if params.ID != nil {
			properties["id"] = map[string]any{
				"type":        "string",
				"description": params.ID.Description + requiredForSuffix(params.ID.RequiredFor),
			}
		}

		if params.Data != nil {
			properties["data"] = map[string]any{
				"type":                 "object",
				"description":          params.Data.Description + requiredForSuffix(params.Data.RequiredFor),
				"additionalProperties": true,
			}
		}

		if params.Query != nil {
			queryProps := make(map[string]any, len(params.Query))
			for name, spec := range params.Query {
				entry := map[string]any{
					"type":        spec.Type,
					"description": spec.Description,
				}
				if spec.Properties != nil {
					entry["properties"] = spec.Properties
				}
				queryProps[name] = entry
			}
			properties["query"] = map[string]any{
				"type":        "object",
				"description": "Query parameters for filtering/pagination",
				"properties":  queryProps,
			}
		}

		maps.Copy(properties, params.Extra)
	}

	return InputSchema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"action"},
	}
}
