package ops

import (
	"slices"

	"jira-thing/internal/config"
)

// DefaultStoryPointFields are the field IDs tried for story points when none
// is configured: the "Story point estimate" and "Story Points" custom fields
// on common Jira Cloud instances, then a legacy alias.
var DefaultStoryPointFields = []string{"customfield_10016", "customfield_10308", "story_points"}

// StoryPointFields lists the story point field IDs to read and write, the
// configured story_points_field first.
func StoryPointFields(cfg config.Config) []string {
	if cfg.StoryPointsField == "" || slices.Contains(DefaultStoryPointFields, cfg.StoryPointsField) {
		return DefaultStoryPointFields
	}
	return append([]string{cfg.StoryPointsField}, DefaultStoryPointFields...)
}

// StoryPoints returns the first set story point value among candidates.
func StoryPoints(fields map[string]any, candidates []string) (any, bool) {
	for _, field := range candidates {
		if v, ok := fields[field]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}
