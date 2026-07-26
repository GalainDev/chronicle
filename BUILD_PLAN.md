# Chronicle — build plan

## Context

ROADMAP.md Phase 3 called for Chronicle as a standalone durable-knowledge vault
(OKF markdown format, Obsidian-compatible), with Pebbles (task state) as a
separate Go+Dolt build in Phase 4. This session expanded that scope: Chronicle
becomes the all-in-one second brain — durable knowledge AND task/progress
tracking (work + personal study plans), absorbing Pebbles' job entirely so
there's one vault, one format, one sync mechanism instead of two systems that
would otherwise need a boundary maintained between them forever.

Key decisions made this session:
- **Pebbles is retired**, absorbed into Chronicle as a `type: task` note.
  `pebbles/spec.md` gets archived with a pointer, not deleted (its Dolt
  architecture notes remain useful reference if markdown+git ever proves
  insufficient under `afterhours`' parallel-worktree load — that's the one
  scenario where Pebbles' transactional guarantees would have mattered).
- **Two repos**, splitting tool from data:
  - `chronicle` (public) — the Go CLI (`chron`) + the OKF format spec/docs.
    No personal content, genuinely reusable by anyone, harness-agnostic
    (works from Claude, Codex, or bare hands).
  - `chronicle-vault` (private) — the actual notes. Global second-brain vault:
    decisions, runbooks, tasks, study plans, work + personal.
- **Multi-vault, git-style resolution, federated**: `chron` walks up from cwd
  looking for `.chronicle/`, like git finds `.git/`. `chron init` scaffolds
  one in any repo — project-specific notes then live and travel with that
  project's own code/repo. Falls back to the global vault
  (`CHRONICLE_VAULT` env, default `~/developer/chronicle-vault`) otherwise.
  Cross-vault links ARE supported (a project vault note can link out to a
  global decision note and back) — this is real scope beyond a single flat
  vault and needs a small vault registry, not just path resolution.
- **Evals**: build CLI + format first against real content (including the
  Chronicle Candidates already queued in the 2026-07-19 handoff), then eval.
  Three-arm comparison: OKF-structured notes vs. flat unstructured markdown
  vs. raw grep — measuring answer accuracy, tokens, and turns for long-horizon
  recall tasks, same with-skill-vs-baseline methodology as skill-creator.
  A RAG+reranker arm is a plausible future addition (informs whether `chron
  search --semantic` is ever worth building) but is explicitly out of v1 —
  it would require embeddings/vector store, contradicting Chronicle's
  no-runtime-no-DB premise, and duplicates ROADMAP Phase 5/6 semantic-search
  scope already deferred for code search.

## Repo layout

**`chronicle`** (public, `github.com/GalainDev/chronicle`):
```
cmd/chron/          # Go CLI entrypoint
internal/vault/      # vault resolution (walk-up .chronicle/, registry, federation)
internal/note/        # note CRUD, frontmatter parsing, [[link]] parsing
internal/lint/         # frontmatter/link/orphan validation
format/                # OKF format spec docs (the reusable artifact non-Go users read)
skills/chronicle/       # the agent-facing skill (read: index->links, write: one note + update index)
evals/                  # eval harness + fixtures, built after real vault content exists
README.md
```

**`chronicle-vault`** (private, `github.com/GalainDev/chronicle-vault`):
```
notes/
  index.md
  work/            # project decisions, runbooks pulled from AI/pebbles/etc.
  study/           # Go learning, other study plans
  personal/        # progress tracking, anything else
.obsidian/          # gitignored, device-local
```
Frontmatter carries `area:` (work/study/personal) so `chron list`/Obsidian
graph can filter without folder-only structure being load-bearing.

Any other repo (e.g. `~/developer/AI`, `~/developer/pebbles`) can run
`chron init` to get its own `.chronicle/notes/` + `index.md`, committed
alongside that repo's own code and git history.

## Note format (OKF)

- YAML frontmatter, `type` required: `decision | task | runbook | reference |
  preference | project`. `task` replaces everything Pebbles would have owned
  (status: open/in_progress/blocked/done, priority, dependencies via
  `blocks:`/`blocked_by:` frontmatter lists instead of a DB table).
- `area:` (work/study/personal), `tags:`, relative wiki-style `[[links]]`.
- One concept per file. `index.md` per vault dir, updated on write —
  progressive disclosure, same pattern as the memory system already in use
  at `~/.claude/projects/-Users-heman/memory/`.
- Obsidian-compatible by construction: frontmatter = Properties, `tags` =
  Obsidian tags, relative markdown links resolve natively in graph view.

## `chron` CLI v1 command surface

Capture + query core, all with `--json` for agent use:
- `chron init` — scaffold `.chronicle/` in the current repo
- `chron new <type> "<title>"` — create a note from a type template
- `chron list` / `chron ready` — tasks by status; `ready` = open/in_progress
  with no unresolved `blocked_by`
- `chron done <note>` — close a task, prompts for a graduation note if it
  looks like it taught something durable (the manual+handoff-graduation
  capture flow agreed this session)
- `chron search <query>` — ripgrep-backed full-text + frontmatter filter
- `chron link <a> <b>` — add a bidirectional `[[link]]`
- `chron lint` — validate frontmatter schema, broken links, orphaned notes;
  wired into CI on both repos
- Vault resolution: nearest `.chronicle/` walking up from cwd, else
  `CHRONICLE_VAULT` global default, with a small on-disk registry
  (`~/.config/chron/vaults.json`) enabling cross-vault link resolution.

Noted for later, not v1: `chron status`/dashboard rollups, graph queries —
valid asks, but Obsidian's own graph/dashboard plugins already cover this
visually once notes exist; revisit only if `chron` itself needs to answer
"what's my status" without opening Obsidian.

## Build order

1. **`chronicle` repo skeleton** — Go module, `cmd/chron`, `internal/vault`
   (walk-up resolution + registry), `internal/note` (frontmatter/link parse).
   Reuse the git-common-dir-style resolution logic already designed in
   `pebbles/spec.md` (walk up to find the marker dir) — same problem, proven
   design, just pointed at `.chronicle/` instead of `.pebbles/`.
2. **Core commands**: `init`, `new`, `list`/`ready`, `done`, `link`, `lint`.
   `search` via shelling out to `rg` against the resolved vault.
3. **`chronicle-vault` repo** — scaffold via `chron init`, seed with the
   Chronicle Candidates already identified in
   `~/developer/AI/.claude/handoffs/AI/2026-07-19_0223_harness-skills-and-dotfiles-session.md`:
   the audit-then-build-our-own pattern, the Pebbles/Chronicle/spec-doc/
   ROADMAP boundary (superseded now — rewrite as the Chronicle-absorbs-
   Pebbles decision), the embedded-git-fixture gotcha, the progressive-
   disclosure skill-keep reasoning. Also migrate the two existing native
   memory entries (`user_profile.md`, `project_harness.md`) in as real notes.
4. **`chronicle` skill** (in `chronicle/skills/chronicle/`) — teaches an
   agent to read (start at nearest vault's `index.md`, follow links, fall
   back to global) and write (one note per concept, correct frontmatter,
   update index, use `chron` not raw file writes when available). Built
   through skill-creator's draft->test->grade->iterate loop, same as
   `create-handoff`/`resume-handoff`/`git-commit`.
5. **`chron lint` in CI** on both repos.
6. **Retire Pebbles**: archive `pebbles/spec.md` with a header pointing to
   Chronicle's task-note format and this decision; strike ROADMAP Phase 4;
   update `AI/README.md`'s sibling-repo table (drop `pebbles` as a distinct
   build, note it's superseded); add a `type: decision` note in
   `chronicle-vault` recording why (one-directional: this note explains
   the reversal, doesn't restate the old boundary as still-active).
7. **Evals** (after step 3-4 give real content to test against): 3-arm
   harness (OKF+chron vs. flat-md vs. raw grep) using skill-creator's
   fixture/benchmark pattern, long-horizon recall task set. Record results
   in `chronicle/evals/benchmark.json`, same as existing skill evals.
8. **Global instruction wiring**: add a line to `~/CLAUDE.md`/`AGENTS.md`
   (the shared instructions symlinked per ROADMAP Phase 1) telling any
   harness to check the resolved Chronicle vault for durable memory/context
   before assuming, and to write back through `chron` when available.
9. **Native Claude Code auto-memory**: left as-is, separate from Chronicle —
   it stays harness-scoped session recall; Chronicle is the canonical,
   harness-agnostic long-term store. No merge/migration between the two
   systems in v1 (avoids taking on native-memory's internal format as a
   dependency); revisit only if duplication becomes a real friction.

## Verification

- `chron lint` passes on `chronicle-vault` after seeding step 3.
- Manual smoke test: `chron init` in a scratch repo, `chron new task`,
  `chron ready` shows it, `chron done` closes it, `chron search` finds it,
  confirm the same files open correctly as an Obsidian vault (graph view
  renders links, Properties panel shows frontmatter).
- Cross-vault link resolves: a note in a project's local `.chronicle/`
  links to a `chronicle-vault` decision note; `chron` resolves it via the
  registry.
- skill-creator's standard with-skill-vs-baseline eval for the `chronicle`
  skill itself (does having the skill improve read/write behavior over a
  baseline agent editing markdown freehand).
- The 3-arm retrieval eval (step 7) produces a `benchmark.json` showing
  OKF+chron's accuracy/token/turn numbers against flat-md and raw-grep
  baselines — this is the evidence gate before calling Chronicle "done"
  per the standing "skills earn their context through evals" rule.

## Progress (updated 2026-07-26)

- [x] Step 1 — repo skeleton (`cmd/chron`, `internal/vault`, `internal/note`, `internal/lint`)
- [x] Step 2 — core commands (`init/new/list/ready/done/link/lint/search`)
- [ ] Step 3 — `chronicle-vault` repo (not created yet, no GitHub remote)
- [ ] Step 4 — `chronicle` skill (`skills/` is empty)
- [ ] Step 5 — `chron lint` in CI (no `.github/` workflows yet)
- [ ] Step 6 — retire Pebbles (spec.md not archived, ROADMAP Phase 4 not struck,
      AI/README.md sibling table not updated)
- [ ] Step 7 — evals (`evals/` is empty)
- [ ] Step 8 — global instruction wiring (no chronicle mention in `~/.claude/CLAUDE.md`)
- N/A Step 9 — explicitly left as-is, no action needed
