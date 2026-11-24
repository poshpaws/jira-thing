package ops

import "jira-thing/internal/config"

// CreateDefaults are the configured fallbacks for a new ticket. Each fills
// its field only when neither the base fields nor the request set it.
type CreateDefaults struct {
	Priority   string
	Components []string
	Labels     []string
}

// DefaultsFromConfig reads default_priority, default_components and
// default_labels from cfg.
func DefaultsFromConfig(cfg config.Config) CreateDefaults {
	return CreateDefaults{Priority: cfg.DefaultPriority, Components: cfg.DefaultComponents, Labels: cfg.DefaultLabels}
}

// apply sets each default on fields whose own value is missing or empty.
func (d CreateDefaults) apply(fields map[string]any) {
	if d.Priority != "" && isUnset(fields["priority"]) {
		fields["priority"] = d.Priority
	}
	if len(d.Components) > 0 && isUnset(fields["components"]) {
		fields["components"] = Components(d.Components)
	}
	if len(d.Labels) > 0 && isUnset(fields["labels"]) {
		fields["labels"] = d.Labels
	}
}

// isUnset reports a missing field, or one holding null, "" or an empty list.
func isUnset(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case []map[string]any:
		return len(v) == 0
	}
	return false
}
