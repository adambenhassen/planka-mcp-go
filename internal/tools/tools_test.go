package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adambenhassen/planka-mcp-go/internal/tools"
)

// repoRoot walks up from the test's working directory to the directory holding
// go.mod, so tests can locate swagger.json regardless of the package directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above working directory")
		}
		dir = parent
	}
}

// loadSwagger reads and parses swagger.json from the repo root.
func loadSwagger(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "swagger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

var dollarParam = regexp.MustCompile(`\$\{(\w+)\}`)

// normalizePath rewrites the ${param} path variant to {param} so tool paths and
// swagger paths compare equal.
func normalizePath(p string) string {
	return dollarParam.ReplaceAllString(p, "{$1}")
}

func TestToolCategoriesNonEmpty(t *testing.T) {
	if len(tools.CoreTools) == 0 {
		t.Error("core tools should not be empty")
	}
	if len(tools.AdminTools) == 0 {
		t.Error("admin tools should not be empty")
	}
	if len(tools.OptionalTools) == 0 {
		t.Error("optional tools should not be empty")
	}
}

func TestAllToolsCombined(t *testing.T) {
	want := len(tools.CoreTools) + len(tools.AdminTools) + len(tools.OptionalTools)
	if len(tools.AllTools) != want {
		t.Errorf("AllTools = %d, want %d", len(tools.AllTools), want)
	}
}

func TestToolCounts(t *testing.T) {
	c := tools.ToolCounts()
	if c.Core != len(tools.CoreTools) {
		t.Errorf("Core = %d, want %d", c.Core, len(tools.CoreTools))
	}
	if c.Admin != len(tools.AdminTools) {
		t.Errorf("Admin = %d, want %d", c.Admin, len(tools.AdminTools))
	}
	if c.Optional != len(tools.OptionalTools) {
		t.Errorf("Optional = %d, want %d", c.Optional, len(tools.OptionalTools))
	}
	if c.Total != len(tools.AllTools) {
		t.Errorf("Total = %d, want %d", c.Total, len(tools.AllTools))
	}
	if c.CoreOperations == 0 || c.AdminOperations == 0 || c.OptionalOperations == 0 || c.TotalOperations == 0 {
		t.Errorf("operation counts must all be > 0: %+v", c)
	}
}

func TestGetEnabledTools(t *testing.T) {
	core := len(tools.CoreTools)
	admin := len(tools.AdminTools)
	optional := len(tools.OptionalTools)

	cases := []struct {
		name string
		cfg  tools.ToolConfig
		want int
	}{
		{"default core only", tools.ToolConfig{}, core},
		{"all tools", tools.ToolConfig{EnableAllTools: true}, len(tools.AllTools)},
		{"admin enabled", tools.ToolConfig{EnableAdminTools: true}, core + admin},
		{"optional enabled", tools.ToolConfig{EnableOptionalTools: true}, core + optional},
		{"admin and optional", tools.ToolConfig{EnableAdminTools: true, EnableOptionalTools: true}, core + admin + optional},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(tools.GetEnabledTools(tc.cfg)); got != tc.want {
				t.Errorf("GetEnabledTools(%+v) = %d, want %d", tc.cfg, got, tc.want)
			}
		})
	}
}

var validToolName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
var validMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func validateTool(t *testing.T, tool tools.GroupedToolDefinition) {
	t.Helper()
	if tool.Name == "" {
		t.Error("tool must have a name")
	}
	if tool.Description == "" {
		t.Errorf("tool %s must have a description", tool.Name)
	}
	if len(tool.Operations) == 0 {
		t.Errorf("tool %s must have at least one operation", tool.Name)
	}
	if !validToolName.MatchString(tool.Name) {
		t.Errorf("tool name %q must be alphanumeric", tool.Name)
	}
	for opName, op := range tool.Operations {
		if op.Method == "" {
			t.Errorf("operation %s.%s must have a method", tool.Name, opName)
		}
		if !slices.Contains(validMethods, op.Method) {
			t.Errorf("operation %s.%s has invalid method: %s", tool.Name, opName, op.Method)
		}
		if op.Path == "" || !strings.HasPrefix(op.Path, "/") {
			t.Errorf("operation %s.%s path must start with /: %q", tool.Name, opName, op.Path)
		}
	}
	if tool.InputSchema.Type != "object" {
		t.Errorf("tool %s inputSchema must be object type", tool.Name)
	}
	if _, ok := tool.InputSchema.Properties["action"]; !ok {
		t.Errorf("tool %s must have action property", tool.Name)
	}
	if !slices.Contains(tool.InputSchema.Required, "action") {
		t.Errorf("tool %s must require action", tool.Name)
	}
}

func TestToolStructureValidation(t *testing.T) {
	for _, group := range [][]tools.GroupedToolDefinition{tools.CoreTools, tools.AdminTools, tools.OptionalTools} {
		for _, tool := range group {
			validateTool(t, tool)
		}
	}
}

func TestUniqueToolNames(t *testing.T) {
	seen := make(map[string]bool, len(tools.AllTools))
	for _, tool := range tools.AllTools {
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestSwaggerPathCoverage(t *testing.T) {
	swagger := loadSwagger(t)
	paths, ok := swagger["paths"].(map[string]any)
	if !ok {
		t.Fatal("swagger.paths missing or not an object")
	}

	specPaths := make([]string, 0, len(paths))
	for p := range paths {
		specPaths = append(specPaths, p)
	}

	seen := make(map[string]bool)
	var toolPaths []string
	for _, tool := range tools.AllTools {
		for _, op := range tool.Operations {
			if !seen[op.Path] {
				seen[op.Path] = true
				toolPaths = append(toolPaths, op.Path)
			}
		}
	}

	for _, tp := range toolPaths {
		if !slices.ContainsFunc(specPaths, func(sp string) bool { return normalizePath(sp) == normalizePath(tp) }) {
			t.Errorf("tool path missing from swagger: %s", tp)
		}
	}
	for _, sp := range specPaths {
		if !slices.ContainsFunc(toolPaths, func(tp string) bool { return normalizePath(tp) == normalizePath(sp) }) {
			t.Errorf("swagger path not covered by tools: %s", sp)
		}
	}
}

func TestSwaggerRequiredBodyFields(t *testing.T) {
	swagger := loadSwagger(t)
	paths, ok := swagger["paths"].(map[string]any)
	if !ok {
		t.Fatal("swagger.paths missing or not an object")
	}

	// requiredBodyFields returns the required JSON body fields declared in the
	// swagger spec for the given normalized path + lowercase method.
	requiredBodyFields := func(opPath, method string) []string {
		for specPath, item := range paths {
			if normalizePath(specPath) != normalizePath(opPath) {
				continue
			}
			methodObj, ok := item.(map[string]any)[strings.ToLower(method)].(map[string]any)
			if !ok {
				return nil
			}
			schema, ok := dig(methodObj, "requestBody", "content", "application/json", "schema")
			if !ok {
				return nil
			}
			reqRaw, ok := schema.(map[string]any)["required"].([]any)
			if !ok {
				return nil
			}
			fields := make([]string, 0, len(reqRaw))
			for _, f := range reqRaw {
				if s, ok := f.(string); ok {
					fields = append(fields, s)
				}
			}
			return fields
		}
		return nil
	}

	var missing []string
	for _, tool := range tools.AllTools {
		dataDesc := ""
		if d, ok := tool.InputSchema.Properties["data"].(map[string]any); ok {
			if s, ok := d["description"].(string); ok {
				dataDesc = s
			}
		}
		for action, op := range tool.Operations {
			if !slices.Contains([]string{"POST", "PUT", "PATCH"}, op.Method) {
				continue
			}
			for _, field := range requiredBodyFields(op.Path, op.Method) {
				if !strings.Contains(dataDesc, field) {
					missing = append(missing, tool.Name+"."+action+": "+field)
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("swagger-required body fields missing from data descriptions: %s", strings.Join(missing, "; "))
	}
}

// dig walks a nested map[string]any by successive string keys.
func dig(m map[string]any, keys ...string) (any, bool) {
	var cur any = m
	for _, k := range keys {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = asMap[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func TestAuthGetTermsHasNoID(t *testing.T) {
	idx := slices.IndexFunc(tools.AllTools, func(x tools.GroupedToolDefinition) bool { return x.Name == "auth" })
	if idx < 0 {
		t.Fatal("auth tool should exist")
	}
	auth := tools.AllTools[idx]
	if auth.Operations["getTerms"].Path != "/terms" {
		t.Errorf("getTerms path = %q, want /terms", auth.Operations["getTerms"].Path)
	}
	if _, ok := auth.InputSchema.Properties["id"]; ok {
		t.Error("auth tool should not have an id property")
	}
}

func TestCustomFieldValuePathVariants(t *testing.T) {
	idx := slices.IndexFunc(tools.OptionalTools, func(x tools.GroupedToolDefinition) bool { return x.Name == "customFields" })
	if idx < 0 {
		t.Fatal("customFields tool should exist")
	}
	cf := tools.OptionalTools[idx]

	setValue := []string{
		"/cards/{cardId}/custom-field-values/customFieldGroupId:{customFieldGroupId}:customFieldId:{customFieldId}",
		"/cards/{cardId}/custom-field-values/customFieldGroupId:{customFieldGroupId}:customFieldId:${customFieldId}",
	}
	clearValue := []string{
		"/cards/{cardId}/custom-field-value/customFieldGroupId:{customFieldGroupId}:customFieldId:{customFieldId}",
		"/cards/{cardId}/custom-field-value/customFieldGroupId:{customFieldGroupId}:customFieldId:${customFieldId}",
	}
	if !slices.Contains(setValue, cf.Operations["setValue"].Path) {
		t.Errorf("unexpected setValue path: %s", cf.Operations["setValue"].Path)
	}
	if !slices.Contains(clearValue, cf.Operations["clearValue"].Path) {
		t.Errorf("unexpected clearValue path: %s", cf.Operations["clearValue"].Path)
	}
}
