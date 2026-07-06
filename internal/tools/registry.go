package tools

import "slices"

// AllTools is every defined tool: core, then admin, then optional.
var AllTools = slices.Concat(CoreTools, AdminTools, OptionalTools)

// ToolConfig selects which tool categories are enabled.
type ToolConfig struct {
	// EnableAllTools enables every category regardless of the others.
	EnableAllTools bool
	// EnableAdminTools adds the admin tools to the core set.
	EnableAdminTools bool
	// EnableOptionalTools adds the optional tools to the core set.
	EnableOptionalTools bool
}

// GetEnabledTools returns the tools enabled by cfg. Core tools are always
// included; admin and optional tools are added when their flags are set, and
// EnableAllTools returns everything.
func GetEnabledTools(cfg ToolConfig) []GroupedToolDefinition {
	if cfg.EnableAllTools {
		return AllTools
	}

	enabled := slices.Clone(CoreTools)
	if cfg.EnableAdminTools {
		enabled = append(enabled, AdminTools...)
	}
	if cfg.EnableOptionalTools {
		enabled = append(enabled, OptionalTools...)
	}
	return enabled
}

// Counts reports tool and operation counts per category and in total.
type Counts struct {
	// Core is the number of core tools.
	Core int
	// CoreOperations is the number of operations across core tools.
	CoreOperations int
	// Admin is the number of admin tools.
	Admin int
	// AdminOperations is the number of operations across admin tools.
	AdminOperations int
	// Optional is the number of optional tools.
	Optional int
	// OptionalOperations is the number of operations across optional tools.
	OptionalOperations int
	// Total is the number of tools across all categories.
	Total int
	// TotalOperations is the number of operations across all categories.
	TotalOperations int
}

// countOperations sums the operations across the given tools.
func countOperations(tools []GroupedToolDefinition) int {
	total := 0
	for _, t := range tools {
		total += len(t.Operations)
	}
	return total
}

// ToolCounts returns the tool and operation counts by category.
func ToolCounts() Counts {
	return Counts{
		Core:               len(CoreTools),
		CoreOperations:     countOperations(CoreTools),
		Admin:              len(AdminTools),
		AdminOperations:    countOperations(AdminTools),
		Optional:           len(OptionalTools),
		OptionalOperations: countOperations(OptionalTools),
		Total:              len(AllTools),
		TotalOperations:    countOperations(AllTools),
	}
}
