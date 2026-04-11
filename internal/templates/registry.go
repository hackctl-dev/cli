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
	templateID = strings.ToLower(strings.TrimSpace(templateID))

	source, ok := registry[templateID]
	if ok {
		return source, nil
	}

	return TemplateSource{}, fmt.Errorf("Unsupporteed template %s", templateID)
}

func SupportedIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	return ids
}
