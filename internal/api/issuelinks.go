package api

import (
	"encoding/json"
	"fmt"
)

// LinkedIssue is one issue link seen from the issue that owns it: Relation is
// the link type's description in the direction that applies (e.g. "blocks" or
// "is blocked by"), followed by the other issue's key, summary and status.
type LinkedIssue struct {
	Relation string
	Key      string
	Summary  string
	Status   string
}

// String renders the link as "<relation> <KEY>: <summary> (<status>)".
func (l LinkedIssue) String() string {
	return fmt.Sprintf("%s %s: %s (%s)", l.Relation, l.Key, l.Summary, l.Status)
}

// linkedIssueRef is the abbreviated issue Jira embeds in each issuelinks entry.
type linkedIssueRef struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// rawIssueLink mirrors one entry of an issue's fields.issuelinks array.
type rawIssueLink struct {
	Type struct {
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	OutwardIssue *linkedIssueRef `json:"outwardIssue,omitempty"`
	InwardIssue  *linkedIssueRef `json:"inwardIssue,omitempty"`
}

// ParseIssueLinks extracts the linked issues from a FetchIssue result. It
// returns nil when the issue has no links or the field is missing/malformed.
func ParseIssueLinks(issue map[string]any) []LinkedIssue {
	fields, _ := issue["fields"].(map[string]any)
	raw, err := json.Marshal(fields["issuelinks"])
	if err != nil {
		return nil
	}
	var links []rawIssueLink
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil
	}
	out := make([]LinkedIssue, 0, len(links))
	for _, link := range links {
		if li, ok := link.toLinkedIssue(); ok {
			out = append(out, li)
		}
	}
	return out
}

// toLinkedIssue resolves which side of the link is the other issue and picks
// the matching relation text. ok is false if neither side is present.
func (l rawIssueLink) toLinkedIssue() (LinkedIssue, bool) {
	ref, relation := l.OutwardIssue, l.Type.Outward
	if ref == nil {
		ref, relation = l.InwardIssue, l.Type.Inward
	}
	if ref == nil {
		return LinkedIssue{}, false
	}
	return LinkedIssue{
		Relation: relation,
		Key:      ref.Key,
		Summary:  ref.Fields.Summary,
		Status:   ref.Fields.Status.Name,
	}, true
}
