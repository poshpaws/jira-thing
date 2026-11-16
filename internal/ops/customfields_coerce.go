package ops

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
)

// Custom field plugin types that need special handling.
const (
	customTextarea = "com.atlassian.jira.plugin.system.customfieldtypes:textarea"
	customSprint   = "com.pyxis.greenhopper.jira:gh-sprint"
)

// converter turns one raw string into a request value for a field.
type converter func(conn api.JiraConnection, raw string) (any, error)

// scalarConverters convert a single value, by schema type.
var scalarConverters = map[string]converter{
	"number":    toNumber,
	"string":    func(_ api.JiraConnection, raw string) (any, error) { return raw, nil },
	"option":    named("value"),
	"version":   named("name"),
	"component": named("name"),
	"priority":  named("name"),
	"user":      toUser,
	"date":      toDate,
	"datetime":  func(_ api.JiraConnection, raw string) (any, error) { return raw, nil },
	"team":      func(_ api.JiraConnection, raw string) (any, error) { return raw, nil },
}

// coerceCustom converts raw to the value the field's type expects; an empty
// raw clears the field. Array fields take comma-separated values.
func coerceCustom(conn api.JiraConnection, f api.Field, raw string) (any, error) {
	switch {
	case raw == "":
		return nil, nil
	case f.Schema.Custom == customTextarea:
		return adf.FromMarkdown(raw), nil
	case f.Schema.Custom == customSprint:
		return toInt(raw)
	case f.Schema.Type == "array":
		return coerceArray(conn, f.Schema.Items, raw)
	}
	convert, ok := scalarConverters[f.Schema.Type]
	if !ok {
		return nil, fmt.Errorf("field type %q isn't supported", f.Schema.Type)
	}
	return convert(conn, raw)
}

// coerceArray converts each comma-separated item by the array's item type.
func coerceArray(conn api.JiraConnection, items, raw string) (any, error) {
	convert, ok := scalarConverters[items]
	if !ok {
		return nil, fmt.Errorf("list of %q isn't supported", items)
	}
	var out []any
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		v, err := convert(conn, part)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// named wraps a value as {key: raw}, e.g. {"value": "Red"} for an option.
func named(key string) converter {
	return func(_ api.JiraConnection, raw string) (any, error) { return map[string]any{key: raw}, nil }
}

func toNumber(_ api.JiraConnection, raw string) (any, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%q is not a number", raw)
	}
	return v, nil
}

func toInt(raw string) (any, error) {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("%q is not a sprint ID", raw)
	}
	return v, nil
}

func toDate(_ api.JiraConnection, raw string) (any, error) {
	if _, err := time.Parse(time.DateOnly, raw); err != nil {
		return nil, fmt.Errorf("%q is not a date like 2026-12-31", raw)
	}
	return raw, nil
}

// toUser resolves a name, email, "me" or account ID like -assignee does.
func toUser(conn api.JiraConnection, raw string) (any, error) {
	u, err := ResolveUser(conn, raw)
	if err != nil {
		return nil, err
	}
	return map[string]any{"accountId": u.AccountID}, nil
}
