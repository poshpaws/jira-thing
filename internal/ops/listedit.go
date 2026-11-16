package ops

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyListEdit means a list edit named no value.
var ErrEmptyListEdit = errors.New(`list edit needs a value: "name" or "+name" adds, "-name" removes`)

// ListEdit adds or removes one value of a list field (labels, components,
// fix versions) without touching the rest.
type ListEdit struct {
	Remove bool
	Value  string
}

// ParseListEdit reads "name" or "+name" as an add and "-name" as a remove.
func ParseListEdit(s string) (ListEdit, error) {
	s = strings.TrimSpace(s)
	edit := ListEdit{Value: strings.TrimPrefix(s, "+")}
	if strings.HasPrefix(s, "-") {
		edit = ListEdit{Remove: true, Value: strings.TrimPrefix(s, "-")}
	}
	if edit.Value = strings.TrimSpace(edit.Value); edit.Value == "" {
		return ListEdit{}, fmt.Errorf("%w: %q", ErrEmptyListEdit, s)
	}
	return edit, nil
}

// ListEdits holds add/remove edits per list field.
type ListEdits struct {
	Labels      []ListEdit
	Components  []ListEdit
	FixVersions []ListEdit
}

// listEditField says how one list field's edits are sent: its Jira field
// ID, and whether values are bare strings (labels) or {"name": …} objects.
type listEditField struct {
	id    string
	named bool
	edits func(ListEdits) []ListEdit
}

var listEditFields = []listEditField{
	{"labels", false, func(e ListEdits) []ListEdit { return e.Labels }},
	{"components", true, func(e ListEdits) []ListEdit { return e.Components }},
	{"fixVersions", true, func(e ListEdits) []ListEdit { return e.FixVersions }},
}

// updateVerbs builds Jira's "update" object from the edits. A field that is
// also set outright in fields is an error: Jira rejects it in both places.
func updateVerbs(edits ListEdits, fields map[string]any) (map[string]any, error) {
	update := map[string]any{}
	for _, f := range listEditFields {
		list := f.edits(edits)
		if len(list) == 0 {
			continue
		}
		if _, set := fields[f.id]; set {
			return nil, fmt.Errorf("%s can't be replaced and edited at once: use either the full list or add/remove", f.id)
		}
		update[f.id] = verbList(list, f.named)
	}
	return update, nil
}

// verbList renders edits as [{"add": v}, {"remove": v}].
func verbList(edits []ListEdit, named bool) []any {
	out := make([]any, len(edits))
	for i, e := range edits {
		verb := "add"
		if e.Remove {
			verb = "remove"
		}
		var value any = e.Value
		if named {
			value = map[string]any{"name": e.Value}
		}
		out[i] = map[string]any{verb: value}
	}
	return out
}
