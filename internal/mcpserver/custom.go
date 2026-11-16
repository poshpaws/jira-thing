package mcpserver

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
)

// customFieldsHelp describes the custom_fields argument.
const customFieldsHelp = `Any other fields, as {"Field name or customfield_NNNNN": value}, e.g. {"Story Points": 3, "Team": "Platform"}. ` +
	`Values are converted by the field's type; lists may be arrays or comma-separated; "" or null clears a field`

// customFieldsOption is the custom_fields argument shared by create_ticket and update_ticket.
func customFieldsOption() mcp.ToolOption {
	return mcp.WithObject("custom_fields", mcp.Description(customFieldsHelp))
}

// customFields reads custom_fields into custom values, in name order.
func customFields(req mcp.CallToolRequest) ([]ops.CustomValue, error) {
	raw, ok := req.GetArguments()["custom_fields"]
	if !ok || raw == nil {
		return nil, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("custom_fields must be an object of field name to value")
	}
	names := make([]string, 0, len(obj))
	for name := range obj {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]ops.CustomValue, 0, len(names))
	for _, name := range names {
		values = append(values, ops.CustomValue{Field: name, Value: customString(obj[name])})
	}
	return values, nil
}

// customString renders a JSON value as the string ops converts from.
func customString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		parts := make([]string, len(x))
		for i, p := range x {
			parts[i] = customString(p)
		}
		return strings.Join(parts, ",")
	}
	return fmt.Sprint(v)
}
