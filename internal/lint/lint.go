// Package lint validates a vault's notes: frontmatter schema, broken
// wiki-links, and orphaned notes.
package lint

import (
	"fmt"
	"strings"

	"github.com/GalainDev/chronicle/internal/note"
	"github.com/GalainDev/chronicle/internal/vault"
)

// Issue is a single lint finding.
type Issue struct {
	NoteID  string
	Kind    string // "missing_type" | "invalid_type" | "invalid_status" | "broken_link" | "orphan"
	Message string
}

// Run lints every note in v, resolving cross-vault links via reg.
func Run(v *vault.Vault, reg *vault.Registry) ([]Issue, error) {
	notes, err := note.List(v.NotesDir)
	if err != nil {
		return nil, err
	}

	ids := map[string]bool{}
	for _, n := range notes {
		ids[n.ID] = true
		ids[strings.TrimSuffix(n.ID, "/"+lastSegment(n.ID))] = true // tolerate bare-slug refs too
		ids[lastSegment(n.ID)] = true
	}

	linkedFrom := map[string]bool{}
	var issues []Issue

	for _, n := range notes {
		if n.Frontmatter.Type == "" {
			issues = append(issues, Issue{n.ID, "missing_type", "note has no `type` frontmatter field"})
		} else if !validType(n.Frontmatter.Type) {
			issues = append(issues, Issue{n.ID, "invalid_type", fmt.Sprintf("unknown type %q", n.Frontmatter.Type)})
		}
		if n.Frontmatter.Type == "task" && n.Frontmatter.Status != "" && !validStatus(n.Frontmatter.Status) {
			issues = append(issues, Issue{n.ID, "invalid_status", fmt.Sprintf("unknown status %q", n.Frontmatter.Status)})
		}

		for _, target := range note.ExtractWikiLinks(n.Body) {
			if strings.Contains(target, ":") {
				parts := strings.SplitN(target, ":", 2)
				if reg == nil {
					issues = append(issues, Issue{n.ID, "broken_link", fmt.Sprintf("cross-vault link %q but no registry available", target)})
					continue
				}
				if _, err := reg.ResolveCrossLink(parts[0], parts[1]+".md"); err != nil {
					issues = append(issues, Issue{n.ID, "broken_link", err.Error()})
				}
				continue
			}
			if !ids[target] {
				issues = append(issues, Issue{n.ID, "broken_link", fmt.Sprintf("[[%s]] does not resolve to any note", target)})
				continue
			}
			linkedFrom[target] = true
		}
	}

	for _, n := range notes {
		if !linkedFrom[n.ID] && !linkedFrom[lastSegment(n.ID)] {
			issues = append(issues, Issue{n.ID, "orphan", "not linked from any other note"})
		}
	}

	return issues, nil
}

func lastSegment(id string) string {
	if i := strings.LastIndex(id, "/"); i != -1 {
		return id[i+1:]
	}
	return id
}

func validType(t string) bool {
	for _, v := range note.Types {
		if v == t {
			return true
		}
	}
	return false
}

func validStatus(s string) bool {
	for _, v := range note.TaskStatuses {
		if v == s {
			return true
		}
	}
	return false
}
