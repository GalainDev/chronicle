// Command chron is the Chronicle CLI: capture and query a vault of
// OKF-format markdown notes. See ../../format/SPEC.md for the note format.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/GalainDev/chronicle/internal/lint"
	"github.com/GalainDev/chronicle/internal/note"
	"github.com/GalainDev/chronicle/internal/spec"
	"github.com/GalainDev/chronicle/internal/vault"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "new":
		err = cmdNew(args)
	case "list":
		err = cmdList(args)
	case "ready":
		err = cmdReady(args)
	case "done":
		err = cmdDone(args)
	case "link":
		err = cmdLink(args)
	case "lint":
		err = cmdLint(args)
	case "search":
		err = cmdSearch(args)
	case "spec":
		err = cmdSpec(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "chron: unknown command %q\n", cmd)
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "chron: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `chron — Chronicle vault CLI

Usage:
  chron init [--global]                scaffold a vault in the current directory
                                       (--global: this repo IS the vault, e.g.
                                       chronicle-vault — no .chronicle/ wrapper)
  chron new <type> "<title>" [--area a]   create a note (type: decision|task|runbook|reference|preference|project)
  chron list [--type t] [--status s] [--json]
  chron ready [--json]                open/in_progress tasks with no unresolved blockers
  chron done <id> [--reason "..."]    close a task, offers to graduate a decision note
  chron link <a> <b>                  add a bidirectional [[wiki-link]] between two notes
  chron lint [--json]                 validate frontmatter, links, orphans
  chron search <query> [--json]       full-text search (ripgrep-backed)

  chron spec new <capability> "<title>" [--area a]      start a capability's spec (v1, proposed)
  chron spec revise <capability> "<title>" [--area a]   new version superseding the current tip
  chron spec implement <capability>                     freeze the current tip as implemented
  chron spec current <capability> [--json]              show the current tip
  chron spec history <capability> [--json]              show every version, oldest to newest
  chron spec list [--json]                               list capabilities and their current status
`)
}

func flagValue(args []string, name string) (string, []string) {
	out := make([]string, 0, len(args))
	val := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--"+name && i+1 < len(args) {
			val = args[i+1]
			i++
			continue
		}
		out = append(out, args[i])
	}
	return val, out
}

func hasFlag(args []string, name string) (bool, []string) {
	out := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == "--"+name {
			found = true
			continue
		}
		out = append(out, a)
	}
	return found, out
}

// --- init ---

func cmdInit(args []string) error {
	global, args := hasFlag(args, "global")
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name := filepath.Base(cwd)
	root := filepath.Join(cwd, ".chronicle")
	if global {
		root = cwd
	}
	v, err := vault.Init(root, name)
	if err != nil {
		return err
	}
	reg, err := vault.LoadRegistry()
	if err != nil {
		return err
	}
	reg.Register(v.Name, root)
	if err := reg.Save(); err != nil {
		return err
	}
	fmt.Printf("initialized vault %q at %s\n", v.Name, v.NotesDir)
	return nil
}

// --- new ---

func cmdNew(args []string) error {
	area, args := flagValue(args, "area")
	if len(args) < 2 {
		return fmt.Errorf("usage: chron new <type> \"<title>\" [--area work|study|personal]")
	}
	typ, title := args[0], strings.Join(args[1:], " ")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := note.New(v.NotesDir, typ, title, area)
	if err != nil {
		return err
	}
	if err := note.UpdateIndex(v.NotesDir); err != nil {
		return err
	}
	fmt.Println(n.Path)
	return nil
}

// --- list / ready ---

func cmdList(args []string) error {
	typeFilter, args := flagValue(args, "type")
	statusFilter, args := flagValue(args, "status")
	asJSON, _ := hasFlag(args, "json")

	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	notes, err := note.List(v.NotesDir)
	if err != nil {
		return err
	}
	var filtered []*note.Note
	for _, n := range notes {
		if typeFilter != "" && n.Frontmatter.Type != typeFilter {
			continue
		}
		if statusFilter != "" && n.Frontmatter.Status != statusFilter {
			continue
		}
		filtered = append(filtered, n)
	}
	return printNotes(filtered, asJSON)
}

func cmdReady(args []string) error {
	asJSON, _ := hasFlag(args, "json")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	notes, err := note.List(v.NotesDir)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	byID := map[string]*note.Note{}
	for _, n := range notes {
		byID[n.ID] = n
		if n.Frontmatter.Type == "task" && n.Frontmatter.Status == "done" {
			done[n.ID] = true
			done[lastSegment(n.ID)] = true
		}
	}
	var ready []*note.Note
	for _, n := range notes {
		if n.Frontmatter.Type != "task" {
			continue
		}
		if n.Frontmatter.Status != "open" && n.Frontmatter.Status != "in_progress" {
			continue
		}
		blocked := false
		for _, b := range n.Frontmatter.BlockedBy {
			if !done[b] {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, n)
		}
	}
	return printNotes(ready, asJSON)
}

func lastSegment(id string) string {
	if i := strings.LastIndex(id, "/"); i != -1 {
		return id[i+1:]
	}
	return id
}

func printNotes(notes []*note.Note, asJSON bool) error {
	if asJSON {
		type row struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Type   string `json:"type"`
			Status string `json:"status,omitempty"`
			Area   string `json:"area,omitempty"`
		}
		rows := make([]row, len(notes))
		for i, n := range notes {
			rows[i] = row{n.ID, n.Title, n.Frontmatter.Type, n.Frontmatter.Status, n.Frontmatter.Area}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, n := range notes {
		status := n.Frontmatter.Status
		if status == "" {
			status = n.Frontmatter.Type
		}
		fmt.Printf("%-10s %-30s %s\n", status, n.ID, n.Title)
	}
	return nil
}

// --- done ---

func cmdDone(args []string) error {
	reason, args := flagValue(args, "reason")
	if len(args) < 1 {
		return fmt.Errorf("usage: chron done <id> [--reason \"...\"]")
	}
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := note.Find(v.NotesDir, args[0])
	if err != nil {
		return err
	}
	n.Frontmatter.Status = "done"
	n.Frontmatter.Closed = time.Now().UTC().Format(time.RFC3339)
	n.Frontmatter.Updated = n.Frontmatter.Closed
	if reason != "" {
		n.Frontmatter.CloseReason = reason
	}
	if err := n.Write(); err != nil {
		return err
	}
	fmt.Printf("closed %s\n", n.ID)

	if isInteractive() {
		fmt.Print("did this teach something durable worth a decision note? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp)), "y") {
			fmt.Print("decision note title: ")
			title, _ := reader.ReadString('\n')
			title = strings.TrimSpace(title)
			if title != "" {
				dn, err := note.New(v.NotesDir, "decision", title, n.Frontmatter.Area)
				if err != nil {
					return err
				}
				dn.Body += fmt.Sprintf("Graduated from [[%s]].\n", n.ID)
				if err := dn.Write(); err != nil {
					return err
				}
				n.Body += fmt.Sprintf("\nGraduated: [[%s]]\n", dn.ID)
				if err := n.Write(); err != nil {
					return err
				}
				fmt.Println(dn.Path)
			}
		}
	}
	return note.UpdateIndex(v.NotesDir)
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// --- link ---

func cmdLink(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: chron link <a> <b>")
	}
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	a, err := note.Find(v.NotesDir, args[0])
	if err != nil {
		return err
	}
	b, err := note.Find(v.NotesDir, args[1])
	if err != nil {
		return err
	}
	if !strings.Contains(a.Body, "[["+b.ID+"]]") {
		a.Body += fmt.Sprintf("\nSee also: [[%s]]\n", b.ID)
		if err := a.Write(); err != nil {
			return err
		}
	}
	if !strings.Contains(b.Body, "[["+a.ID+"]]") {
		b.Body += fmt.Sprintf("\nSee also: [[%s]]\n", a.ID)
		if err := b.Write(); err != nil {
			return err
		}
	}
	fmt.Printf("linked %s <-> %s\n", a.ID, b.ID)
	return note.UpdateIndex(v.NotesDir)
}

// --- spec ---

func cmdSpec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: chron spec <new|revise|implement|current|history|list> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "new":
		return cmdSpecNew(rest)
	case "revise":
		return cmdSpecRevise(rest)
	case "implement":
		return cmdSpecImplement(rest)
	case "current":
		return cmdSpecCurrent(rest)
	case "history":
		return cmdSpecHistory(rest)
	case "list":
		return cmdSpecList(rest)
	default:
		return fmt.Errorf("chron spec: unknown subcommand %q", sub)
	}
}

func cmdSpecNew(args []string) error {
	area, args := flagValue(args, "area")
	if len(args) < 2 {
		return fmt.Errorf(`usage: chron spec new <capability> "<title>" [--area work|study|personal]`)
	}
	capability, title := args[0], strings.Join(args[1:], " ")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := spec.New(v.SpecsDir, capability, title, area)
	if err != nil {
		return err
	}
	fmt.Println(n.Path)
	return nil
}

func cmdSpecRevise(args []string) error {
	area, args := flagValue(args, "area")
	if len(args) < 2 {
		return fmt.Errorf(`usage: chron spec revise <capability> "<title>" [--area work|study|personal]`)
	}
	capability, title := args[0], strings.Join(args[1:], " ")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := spec.Revise(v.SpecsDir, capability, title, area)
	if err != nil {
		return err
	}
	fmt.Println(n.Path)
	return nil
}

func cmdSpecImplement(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: chron spec implement <capability>")
	}
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := spec.Implement(v.SpecsDir, args[0])
	if err != nil {
		return err
	}
	fmt.Printf("implemented %s (%s)\n", args[0], n.ID)
	return nil
}

func cmdSpecCurrent(args []string) error {
	asJSON, args := hasFlag(args, "json")
	if len(args) < 1 {
		return fmt.Errorf("usage: chron spec current <capability> [--json]")
	}
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	n, err := spec.Current(v.SpecsDir, args[0])
	if err != nil {
		return err
	}
	return printNotes([]*note.Note{n}, asJSON)
}

func cmdSpecHistory(args []string) error {
	asJSON, args := hasFlag(args, "json")
	if len(args) < 1 {
		return fmt.Errorf("usage: chron spec history <capability> [--json]")
	}
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	hist, err := spec.History(v.SpecsDir, args[0])
	if err != nil {
		return err
	}
	if len(hist) == 0 {
		return fmt.Errorf("no spec found for capability %q", args[0])
	}
	return printNotes(hist, asJSON)
}

func cmdSpecList(args []string) error {
	asJSON, _ := hasFlag(args, "json")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	caps, err := spec.Capabilities(v.SpecsDir)
	if err != nil {
		return err
	}
	tips := make([]*note.Note, 0, len(caps))
	for _, c := range caps {
		n, err := spec.Current(v.SpecsDir, c)
		if err != nil {
			return err
		}
		tips = append(tips, n)
	}
	return printNotes(tips, asJSON)
}

// --- lint ---

func cmdLint(args []string) error {
	asJSON, _ := hasFlag(args, "json")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	reg, err := vault.LoadRegistry()
	if err != nil {
		return err
	}
	issues, err := lint.Run(v, reg)
	if err != nil {
		return err
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].NoteID < issues[j].NoteID })
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(issues); err != nil {
			return err
		}
	} else {
		for _, iss := range issues {
			fmt.Printf("%-12s %-30s %s\n", iss.Kind, iss.NoteID, iss.Message)
		}
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
	return nil
}

// --- search ---

func cmdSearch(args []string) error {
	asJSON, args := hasFlag(args, "json")
	if len(args) < 1 {
		return fmt.Errorf("usage: chron search <query> [--json]")
	}
	query := strings.Join(args, " ")
	v, err := vault.Resolve(".")
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("rg"); err == nil {
		rgArgs := []string{"-n", "--no-heading"}
		if asJSON {
			rgArgs = []string{"--json"}
		}
		rgArgs = append(rgArgs, query, v.NotesDir)
		c := exec.Command("rg", rgArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return nil // rg: no matches
			}
			return err
		}
		return nil
	}
	return searchFallback(v.NotesDir, query, asJSON)
}

func searchFallback(notesDir, query string, asJSON bool) error {
	notes, err := note.List(notesDir)
	if err != nil {
		return err
	}
	type hit struct {
		ID   string `json:"id"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	var hits []hit
	q := strings.ToLower(query)
	for _, n := range notes {
		lines := strings.Split(n.Body, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), q) {
				hits = append(hits, hit{n.ID, i + 1, strings.TrimSpace(line)})
			}
		}
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}
	for _, h := range hits {
		fmt.Printf("%s:%d: %s\n", h.ID, h.Line, h.Text)
	}
	return nil
}
