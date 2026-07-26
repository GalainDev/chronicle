// Package lint validates a vault's notes: frontmatter schema, broken
// wiki-links, and orphaned notes. It also validates the spec-driven-dev
// ledger under specs/: capability/filename consistency, dangling
// supersedes/superseded_by links, and broken or forked chains.
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GalainDev/chronicle/internal/note"
	"github.com/GalainDev/chronicle/internal/spec"
	"github.com/GalainDev/chronicle/internal/vault"
)

// Issue is a single lint finding.
type Issue struct {
	NoteID  string
	Kind    string // "missing_type" | "invalid_type" | "invalid_status" | "broken_link" | "orphan" | "spec_*"
	Message string
}

// Run lints every note in v, resolving cross-vault links via reg, plus the
// spec-driven-dev ledger under v.SpecsDir.
func Run(v *vault.Vault, reg *vault.Registry) ([]Issue, error) {
	notes, err := note.List(v.NotesDir)
	if err != nil {
		return nil, err
	}
	specNotes, err := listSpecs(v.SpecsDir)
	if err != nil {
		return nil, err
	}

	ids := map[string]bool{}
	for _, n := range notes {
		ids[n.ID] = true
		ids[strings.TrimSuffix(n.ID, "/"+lastSegment(n.ID))] = true // tolerate bare-slug refs too
		ids[lastSegment(n.ID)] = true
	}
	for _, n := range specNotes {
		ids[n.ID] = true // notes may [[link]] out to a spec version; don't flag as broken
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

	specIssues, err := lintSpecs(v.SpecsDir, specNotes)
	if err != nil {
		return nil, err
	}
	issues = append(issues, specIssues...)

	return issues, nil
}

var specVersionFile = regexp.MustCompile(`^v(\d+)-`)

// listSpecs parses every version note under specsDir. A missing specs/ dir
// (no specs created yet) is not an error.
func listSpecs(specsDir string) ([]*note.Note, error) {
	notes, err := note.List(specsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// lintSpecs validates the spec ledger: each version's capability field
// matches its containing directory, filenames follow v<N>-<slug>.md,
// supersedes/superseded_by resolve to a real version in the same
// capability, and each capability resolves to exactly one current tip
// (spec.Current would error on a forked or broken chain).
func lintSpecs(specsDir string, notes []*note.Note) ([]Issue, error) {
	var issues []Issue
	byID := map[string]bool{}
	for _, n := range notes {
		byID[n.ID] = true
	}

	for _, n := range notes {
		if n.Frontmatter.Type != "spec" {
			issues = append(issues, Issue{n.ID, "spec_invalid_type", fmt.Sprintf("note under specs/ has type %q, want spec", n.Frontmatter.Type)})
			continue
		}
		capabilityDir := filepath.Dir(n.ID)
		if n.Frontmatter.Capability != capabilityDir {
			issues = append(issues, Issue{n.ID, "spec_capability_mismatch", fmt.Sprintf("capability %q does not match its directory %q", n.Frontmatter.Capability, capabilityDir)})
		}
		if !specVersionFile.MatchString(filepath.Base(n.ID)) {
			issues = append(issues, Issue{n.ID, "spec_bad_filename", "filename does not match v<N>-<slug>.md"})
		}
		if n.Frontmatter.Status != "" && !validSpecStatus(n.Frontmatter.Status) {
			issues = append(issues, Issue{n.ID, "invalid_status", fmt.Sprintf("unknown spec status %q", n.Frontmatter.Status)})
		}
		if s := n.Frontmatter.Supersedes; s != "" && !byID[s] {
			issues = append(issues, Issue{n.ID, "spec_dangling_link", fmt.Sprintf("supersedes %q does not resolve to any spec version", s)})
		}
		if s := n.Frontmatter.SupersededBy; s != "" && !byID[s] {
			issues = append(issues, Issue{n.ID, "spec_dangling_link", fmt.Sprintf("superseded_by %q does not resolve to any spec version", s)})
		}
	}

	caps, err := spec.Capabilities(specsDir)
	if err != nil {
		return nil, err
	}
	for _, c := range caps {
		if _, err := spec.Current(specsDir, c); err != nil {
			issues = append(issues, Issue{c, "spec_broken_chain", err.Error()})
		}
	}
	return issues, nil
}

func validSpecStatus(s string) bool {
	for _, v := range note.SpecStatuses {
		if v == s {
			return true
		}
	}
	return false
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
