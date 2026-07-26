# chronicle

`chron` — a small Go CLI for a markdown-and-git knowledge vault: decisions,
runbooks, tasks, study/progress notes. Harness-agnostic (Claude, Codex, or
bare hands all read/write the same files) and Obsidian-vault-compatible by
construction (point Obsidian at the same `notes/` folder — no export step).

No server, no database, no runtime dependency. Sync is `git push`/`pull`.

See [`format/SPEC.md`](format/SPEC.md) for the note format.

## Install

```sh
go install github.com/GalainDev/chronicle/cmd/chron@latest
```

## Quickstart

```sh
# a project-local vault, travels with this repo's own git history
cd ~/developer/some-project
chron init
chron new task "Fix the flaky retry test"
chron ready --json

# the global second-brain vault (a dedicated repo where notes/ IS the root)
cd ~/developer/chronicle-vault
chron init --global
chron new decision "Why Dolt over SQLite"
```

## Commands

| Command | Does |
|---|---|
| `chron init [--global]` | scaffold a vault (local `.chronicle/`, or `--global` if this repo IS the vault) |
| `chron new <type> "<title>" [--area a]` | create a note (`decision\|task\|runbook\|reference\|preference\|project`) |
| `chron list [--type t] [--status s] [--json]` | list notes |
| `chron ready [--json]` | open/in_progress tasks with no unresolved `blocked_by` |
| `chron done <id> [--reason "..."]` | close a task; offers to graduate a decision note |
| `chron link <a> <b>` | add a bidirectional `[[wiki-link]]` |
| `chron lint [--json]` | validate frontmatter, links (incl. cross-vault), orphans |
| `chron search <query> [--json]` | full-text search (ripgrep-backed, falls back to a built-in scan) |

Vault resolution walks up from cwd looking for `.chronicle/`, like git finds
`.git/`; falls back to `$CHRONICLE_VAULT` (default `~/developer/chronicle-vault`).
Every `chron init` registers itself in `~/.config/chron/vaults.json`, which
is what lets `[[vaultname:path]]` links resolve across vaults.

## Sibling repos

- [`chronicle-vault`](https://github.com/GalainDev/chronicle-vault) (private) — the actual notes
- [`AI`](https://github.com/GalainDev/AI) — harness (skills/hooks/mcp for Claude Code + Codex)
- [`dotfiles`](https://github.com/GalainDev/dotfiles) — the machine
