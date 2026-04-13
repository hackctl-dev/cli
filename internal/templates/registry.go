package templates

import (
	"fmt"
	"sort"
	"strings"
)

type TemplateSource struct {
	ID      string
	RepoURL string
	Ref     string
	Subdir  string
}

var registry = map[string]TemplateSource{
	"mern": {
		ID:      "mern",
		RepoURL: "https://github.com/hackctl-dev/templates.git",
		Ref:     "main",
		Subdir:  "mern",
	},
}

func Resolve(templateID string) (TemplateSource, error) {
	templateID = normalizedTemplateID(templateID)

	source, ok := registry[templateID]
	if ok {
		return source, nil
	}

	return TemplateSource{}, fmt.Errorf("Unsupporteed template %s", templateID)
}

func IsOfficial(templateID string) bool {
	_, ok := registry[normalizedTemplateID(templateID)]
	return ok
}

func SupportedIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	return ids
}

func normalizedTemplateID(templateID string) string {
	return strings.ToLower(strings.TrimSpace(templateID))
}
