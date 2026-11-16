package ops

import (
	"errors"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

var (
	// ErrBadCustom means a custom field argument wasn't "Name=value".
	ErrBadCustom = errors.New(`custom field must be "Name=value" (an empty value clears the field)`)
	// ErrUnknownField means no field has the given name or ID.
	ErrUnknownField = errors.New(`no Jira field has that name or ID; search with: diagnose -find-field "part of the name"`)
)

// CustomValue sets Field (a name or an ID such as customfield_10021) to
// Value, converted by the field's type. An empty Value clears the field.
type CustomValue struct {
	Field string
	Value string
}

// ParseCustom reads "Name=value", splitting at the first "=".
func ParseCustom(s string) (CustomValue, error) {
	name, value, ok := strings.Cut(s, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return CustomValue{}, fmt.Errorf("%w: %q", ErrBadCustom, s)
	}
	return CustomValue{Field: name, Value: strings.TrimSpace(value)}, nil
}

// AmbiguousFieldError means a name matched several fields; Matches lists them.
type AmbiguousFieldError struct {
	Name    string
	Matches []api.Field
}

func (e *AmbiguousFieldError) Error() string {
	ids := make([]string, len(e.Matches))
	for i, f := range e.Matches {
		ids[i] = fmt.Sprintf("%s (%s)", termsafe.Line(f.Name), f.ID)
	}
	return fmt.Sprintf("%q matches %d fields; use the ID instead: %s", e.Name, len(e.Matches), strings.Join(ids, ", "))
}

// ResolveCustomFields turns custom values into request fields keyed by
// field ID, reading the field registry once. It returns nil for no values.
func ResolveCustomFields(conn api.JiraConnection, values []CustomValue) (map[string]any, error) {
	if len(values) == 0 {
		return nil, nil
	}
	registry, err := api.FetchFields(conn)
	if err != nil {
		return nil, fmt.Errorf("fetching fields: %w", err)
	}
	out := map[string]any{}
	for _, v := range values {
		field, err := findField(registry, v.Field)
		if err != nil {
			return nil, err
		}
		value, err := coerceCustom(conn, field, v.Value)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", termsafe.Line(field.Name), field.ID, err)
		}
		out[field.ID] = value
	}
	return out, nil
}

// mergeCustom resolves custom values and adds them to fields.
func mergeCustom(conn api.JiraConnection, fields map[string]any, values []CustomValue) error {
	custom, err := ResolveCustomFields(conn, values)
	if err != nil {
		return err
	}
	for id, v := range custom {
		fields[id] = v
	}
	return nil
}

// findField matches an ID exactly, else a name ignoring case.
func findField(registry []api.Field, nameOrID string) (api.Field, error) {
	var matches []api.Field
	for _, f := range registry {
		if f.ID == nameOrID {
			return f, nil
		}
		if strings.EqualFold(f.Name, nameOrID) {
			matches = append(matches, f)
		}
	}
	switch len(matches) {
	case 0:
		return api.Field{}, fmt.Errorf("%q: %w", nameOrID, ErrUnknownField)
	case 1:
		return matches[0], nil
	}
	return api.Field{}, &AmbiguousFieldError{Name: nameOrID, Matches: matches}
}
