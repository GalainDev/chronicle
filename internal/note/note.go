// Package note reads and writes Chronicle notes: a small, explicit
// frontmatter schema (not a general YAML library — the schema is fixed and
// flat, so a hand-rolled parser keeps chron a dependency-free static binary)
// followed by a markdown body.
package note

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Types is the fixed set of valid `type` values.
var Types = []string{"decision", "task", "runbook", "reference", "preference", "project"}

// TaskStatuses is the fixed set of valid `status` values for type: task notes.
var TaskStatuses = []string{"open", "in_progress", "blocked", "done", "deferred"}

func validType(t string) bool {
	for _, v := range Types {
		if v == t {
			return true
		}
	}
	return false
}

// Frontmatter is the fixed schema for a Chronicle note. Every field maps to
// one YAML-ish `key: value` line; list fields use `[a, b, c]` inline syntax.
type Frontmatter struct {
	Type        string // required: decision|task|runbook|reference|preference|project
	Area        string // work|study|personal, optional
	Tags        []string
	Status      string // task only: open|in_progress|blocked|done|deferred
	Priority    string // task only: 0-3
	Blocks      []string
	BlockedBy   []string
	Created     string
	Updated     string
	Closed      string
	CloseReason string
}

// Note is a parsed Chronicle note file.
type Note struct {
	Path        string
	ID          string // path relative to notes/, without extension — the id used on the CLI
	Title       string // first H1 in the body
	Frontmatter Frontmatter
	Body        string // everything after the frontmatter block
}

var fieldOrder = []string{
	"type", "area", "tags", "status", "priority", "blocks", "blocked_by",
	"created", "updated", "closed", "close_reason",
}

func (fm Frontmatter) get(key string) (string, []string, bool) {
	switch key {
	case "type":
		return fm.Type, nil, fm.Type != ""
	case "area":
		return fm.Area, nil, fm.Area != ""
	case "tags":
		return "", fm.Tags, len(fm.Tags) > 0
	case "status":
		return fm.Status, nil, fm.Status != ""
	case "priority":
		return fm.Priority, nil, fm.Priority != ""
	case "blocks":
		return "", fm.Blocks, len(fm.Blocks) > 0
	case "blocked_by":
		return "", fm.BlockedBy, len(fm.BlockedBy) > 0
	case "created":
		return fm.Created, nil, fm.Created != ""
	case "updated":
		return fm.Updated, nil, fm.Updated != ""
	case "closed":
		return fm.Closed, nil, fm.Closed != ""
	case "close_reason":
		return fm.CloseReason, nil, fm.CloseReason != ""
	}
	return "", nil, false
}

func (fm *Frontmatter) set(key, scalar string, list []string) {
	switch key {
	case "type":
		fm.Type = scalar
	case "area":
		fm.Area = scalar
	case "tags":
		fm.Tags = list
	case "status":
		fm.Status = scalar
	case "priority":
		fm.Priority = scalar
	case "blocks":
		fm.Blocks = list
	case "blocked_by":
		fm.BlockedBy = list
	case "created":
		fm.Created = scalar
	case "updated":
		fm.Updated = scalar
	case "closed":
		fm.Closed = scalar
	case "close_reason":
		fm.CloseReason = scalar
	}
}

// Encode renders the frontmatter block, delimited by `---` lines.
func (fm Frontmatter) Encode() string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range fieldOrder {
		scalar, list, ok := fm.get(key)
		if !ok {
			continue
		}
		if list != nil {
			b.WriteString(fmt.Sprintf("%s: [%s]\n", key, strings.Join(list, ", ")))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s\n", key, scalar))
		}
	}
	b.WriteString("---\n")
	return b.String()
}

var kvLine = regexp.MustCompile(`^([a-z_]+):\s*(.*)$`)

func parseFrontmatter(block string) Frontmatter {
	var fm Frontmatter
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := kvLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, val := m[1], strings.TrimSpace(m[2])
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			inner := strings.TrimSpace(val[1 : len(val)-1])
			var list []string
			if inner != "" {
				for _, item := range strings.Split(inner, ",") {
					item = strings.Trim(strings.TrimSpace(item), `"'`)
					if item != "" {
						list = append(list, item)
					}
				}
			}
			fm.set(key, "", list)
		} else {
			fm.set(key, strings.Trim(val, `"'`), nil)
		}
	}
	return fm
}

var h1 = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// Parse reads and parses a note file. notesDir is used to compute ID.
func Parse(path, notesDir string) (*Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	var fm Frontmatter
	body := content
	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		if end := strings.Index(rest, "\n---"); end != -1 {
			block := rest[:end]
			afterDelim := rest[end+4:]
			body = strings.TrimPrefix(afterDelim, "\n")
			fm = parseFrontmatter(block)
		}
	}
	title := filepath.Base(path)
	if m := h1.FindStringSubmatch(body); m != nil {
		title = m[1]
	}
	rel, err := filepath.Rel(notesDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	id := strings.TrimSuffix(rel, filepath.Ext(rel))
	return &Note{Path: path, ID: id, Title: title, Frontmatter: fm, Body: body}, nil
}

// Write serialises the note (frontmatter + body) back to Path.
func (n *Note) Write() error {
	if err := os.MkdirAll(filepath.Dir(n.Path), 0o755); err != nil {
		return err
	}
	content := n.Frontmatter.Encode() + "\n" + strings.TrimLeft(n.Body, "\n")
	return os.WriteFile(n.Path, []byte(content), 0o644)
}

// Slug turns a title into a filesystem-safe filename stem.
func Slug(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// New creates a note file of the given type/title in the vault's typeDir
// (e.g. notesDir/tasks, or notesDir directly), populated with a Created
// timestamp and sensible per-type defaults.
func New(notesDir, typ, title, area string) (*Note, error) {
	if !validType(typ) {
		return nil, fmt.Errorf("invalid type %q, must be one of %v", typ, Types)
	}
	slug := Slug(title)
	if slug == "" {
		return nil, fmt.Errorf("title produced an empty slug")
	}
	sub := typ + "s"
	path := filepath.Join(notesDir, sub, slug+".md")
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("note already exists: %s", path)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	fm := Frontmatter{Type: typ, Area: area, Created: now}
	if typ == "task" {
		fm.Status = "open"
		fm.Priority = "2"
	}
	n := &Note{
		Path:        path,
		ID:          strings.TrimSuffix(filepath.Join(sub, slug), ".md"),
		Title:       title,
		Frontmatter: fm,
		Body:        fmt.Sprintf("# %s\n\n", title),
	}
	if err := n.Write(); err != nil {
		return nil, err
	}
	return n, nil
}

var wikiLink = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// ExtractWikiLinks returns every [[target]] reference in body, deduplicated.
func ExtractWikiLinks(body string) []string {
	matches := wikiLink.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		if !seen[target] {
			seen[target] = true
			out = append(out, target)
		}
	}
	sort.Strings(out)
	return out
}

// List parses every .md note under notesDir except index.md files.
func List(notesDir string) ([]*Note, error) {
	var notes []*Note
	err := filepath.WalkDir(notesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" || filepath.Base(path) == "index.md" {
			return nil
		}
		n, err := Parse(path, notesDir)
		if err != nil {
			return err
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	return notes, nil
}

// Find locates a note by ID (relative path without extension) or by exact
// slug match if the type-subdirectory prefix is omitted.
func Find(notesDir, ref string) (*Note, error) {
	direct := filepath.Join(notesDir, ref+".md")
	if _, err := os.Stat(direct); err == nil {
		return Parse(direct, notesDir)
	}
	notes, err := List(notesDir)
	if err != nil {
		return nil, err
	}
	var candidates []*Note
	for _, n := range notes {
		if filepath.Base(n.ID) == ref || n.ID == ref {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}
		return nil, fmt.Errorf("ambiguous note reference %q, matches: %v", ref, ids)
	}
	return nil, fmt.Errorf("no note found for %q", ref)
}

// UpdateIndex regenerates notes/index.md with a flat listing grouped by type.
func UpdateIndex(notesDir string) error {
	notes, err := List(notesDir)
	if err != nil {
		return err
	}
	byType := map[string][]*Note{}
	for _, n := range notes {
		byType[n.Frontmatter.Type] = append(byType[n.Frontmatter.Type], n)
	}
	var b strings.Builder
	b.WriteString("# Index\n\n")
	b.WriteString("Chronicle vault index. Generated by `chron`; follow links from here.\n")
	for _, typ := range Types {
		group := byType[typ]
		if len(group) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n## %s\n\n", strings.Title(typ)+"s"))
		for _, n := range group {
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", n.ID, n.Title))
		}
	}
	return os.WriteFile(filepath.Join(notesDir, "index.md"), []byte(b.String()), 0o644)
}

// Writer exposes a buffered stdout writer for CLI output helpers.
func Writer() *bufio.Writer {
	return bufio.NewWriter(os.Stdout)
}
