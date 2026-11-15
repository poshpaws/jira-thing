package ops

import (
	"jira-thing/internal/api"
	"jira-thing/internal/template"
)

// FetchTemplate fetches a ticket and returns its reusable fields as a
// template. Unless keepAssignee is set (and the ticket has an assignee), the
// assignee becomes the AssigneeSelf marker, resolved to whoever creates from it.
func FetchTemplate(conn api.JiraConnection, key string, keepAssignee bool) (map[string]any, error) {
	issue, err := api.FetchIssue(conn, key)
	if err != nil {
		return nil, err
	}
	tmpl := template.Build(issue)
	SetTemplateAssignee(tmpl, keepAssignee)
	return tmpl, nil
}

// SetTemplateAssignee keeps a template's assignee when asked to and one is
// present, and otherwise replaces it with the AssigneeSelf marker.
func SetTemplateAssignee(tmpl map[string]any, keep bool) {
	if _, has := tmpl["assignee"].(map[string]any); has && keep {
		return
	}
	tmpl["assignee"] = template.AssigneeSelf
}

// SaveUserTemplate writes tmpl to the per-user default template location,
// returning the path written.
func SaveUserTemplate(tmpl map[string]any) (string, error) {
	path, err := template.UserTemplatePath()
	if err != nil {
		return "", err
	}
	return template.Save(tmpl, path)
}
