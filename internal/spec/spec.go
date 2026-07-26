// Package spec implements Chronicle's spec-driven development feature:
// per-repo, capability-scoped specs stored as an immutable ledger. A
// capability's spec is never rewritten once implemented — a later change
// creates a new version linked back via supersedes/superseded_by, and the
// "current" spec is whichever version has no superseded_by set yet.
package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GalainDev/chronicle/internal/note"
)

var versionFile = regexp.MustCompile(`^v(\d+)-`)

func versionNumber(path string) int {
	m := versionFile.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Versions returns every version note for capability, oldest to newest. An
// unknown capability returns an empty slice, not an error.
func Versions(specsDir, capability string) ([]*note.Note, error) {
	dir := filepath.Join(specsDir, capability)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []*note.Note
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		n, err := note.Parse(filepath.Join(dir, e.Name()), specsDir)
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	sort.Slice(notes, func(i, j int) bool {
		return versionNumber(notes[i].Path) < versionNumber(notes[j].Path)
	})
	return notes, nil
}

// Capabilities lists every capability with at least one spec version.
func Capabilities(specsDir string) ([]string, error) {
	entries, err := os.ReadDir(specsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Current resolves the tip of a capability's chain: the one version with no
// superseded_by set. Errors if the capability has no versions, or if the
// chain has forked (more than one tip — a data-integrity problem chron lint
// should catch, not something Current silently picks a winner for).
func Current(specsDir, capability string) (*note.Note, error) {
	versions, err := Versions(specsDir, capability)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no spec found for capability %q", capability)
	}
	var tips []*note.Note
	for _, v := range versions {
		if v.Frontmatter.SupersededBy == "" {
			tips = append(tips, v)
		}
	}
	switch len(tips) {
	case 1:
		return tips[0], nil
	case 0:
		return nil, fmt.Errorf("capability %q has versions but no current tip (chain is broken)", capability)
	default:
		ids := make([]string, len(tips))
		for i, t := range tips {
			ids[i] = t.ID
		}
		return nil, fmt.Errorf("capability %q has forked: multiple tips %v", capability, ids)
	}
}

// History returns every version for capability, oldest to newest.
func History(specsDir, capability string) ([]*note.Note, error) {
	return Versions(specsDir, capability)
}

// New creates the first version (v1) of a capability's spec, status proposed.
func New(specsDir, capability, title, area string) (*note.Note, error) {
	if capability == "" {
		return nil, fmt.Errorf("capability is required")
	}
	existing, err := Versions(specsDir, capability)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("capability %q already has %d version(s); use `chron spec revise` to add another", capability, len(existing))
	}
	return writeVersion(specsDir, capability, 1, title, area, "")
}

// Revise creates a new version superseding the current tip, whatever its
// status. The old tip's content is never rewritten — only its
// status/superseded_by metadata is touched, to point at the new version.
func Revise(specsDir, capability, title, area string) (*note.Note, error) {
	tip, err := Current(specsDir, capability)
	if err != nil {
		return nil, err
	}
	next := versionNumber(tip.Path) + 1
	n, err := writeVersion(specsDir, capability, next, title, area, tip.ID)
	if err != nil {
		return nil, err
	}
	tip.Frontmatter.Status = "superseded"
	tip.Frontmatter.SupersededBy = n.ID
	tip.Frontmatter.Updated = now()
	if err := tip.Write(); err != nil {
		return nil, err
	}
	return n, nil
}

// Implement marks a capability's current tip as implemented, freezing its
// content going forward — any future change must go through Revise, never
// a direct edit.
func Implement(specsDir, capability string) (*note.Note, error) {
	tip, err := Current(specsDir, capability)
	if err != nil {
		return nil, err
	}
	if tip.Frontmatter.Status == "implemented" {
		return nil, fmt.Errorf("capability %q is already implemented at %s", capability, tip.ID)
	}
	tip.Frontmatter.Status = "implemented"
	tip.Frontmatter.Updated = now()
	if err := tip.Write(); err != nil {
		return nil, err
	}
	return tip, nil
}

func writeVersion(specsDir, capability string, version int, title, area, supersedes string) (*note.Note, error) {
	slug := note.Slug(title)
	if slug == "" {
		return nil, fmt.Errorf("title produced an empty slug")
	}
	filename := fmt.Sprintf("v%d-%s.md", version, slug)
	path := filepath.Join(specsDir, capability, filename)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("spec version already exists: %s", path)
	}
	fm := note.Frontmatter{
		Type:       "spec",
		Capability: capability,
		Area:       area,
		Status:     "proposed",
		Supersedes: supersedes,
		Created:    now(),
	}
	body := fmt.Sprintf("# %s\n\n", title)
	if supersedes != "" {
		body += fmt.Sprintf("Supersedes [[%s]].\n\n", supersedes)
	}
	body += "## Requirements\n\n"
	id := strings.TrimSuffix(filepath.Join(capability, filename), ".md")
	n := &note.Note{
		Path:        path,
		ID:          id,
		Title:       title,
		Frontmatter: fm,
		Body:        body,
	}
	if err := n.Write(); err != nil {
		return nil, err
	}
	return n, nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
