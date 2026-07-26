---
name: chronicle
description: Read and write Chronicle — the markdown-and-git knowledge vault (durable decisions/runbooks/preferences/tasks, second-brain style) and spec-driven development ledger (per-repo capability specs). Use this whenever the user asks to remember/record/look up a decision, runbook, preference, or study/work progress note that should outlive this session; whenever a repo needs a spec written or checked before implementing a feature ("write a spec for X", "what's the spec say", "is this implemented yet", "revise the spec"); or when starting work on an unfamiliar repo/topic and durable context might already exist. Prefer this over ad-hoc markdown files or native session memory for anything meant to be found again later, by this harness or another one.
---

# Chronicle

Chronicle (`chron`) is a CLI over a markdown+git knowledge vault. It does two
distinct jobs — know which one applies before acting:

1. **Second brain** — durable, cross-session knowledge: decisions, runbooks,
   preferences, projects, and lightweight personal task/study tracking.
   Lives in whichever vault resolves for the current directory (see below).
2. **Spec-driven development** — per-repo capability specs, an immutable
   ledger (never rewritten once implemented; changes create new versions).
   The plan an agent implements *from*, not a record of what was decided.

Full format reference: `format/SPEC.md` in the `chronicle` repo. This skill
covers the workflow; read that file if a frontmatter field or file-layout
question comes up that isn't answered here.

## Before anything: resolve the vault

`chron` walks up from cwd looking for `.chronicle/`, exactly like git finds
`.git/`. If none is found above cwd, it falls back to the global vault
(`$CHRONICLE_VAULT`, default `~/developer/chronicle-vault`). Don't guess
which vault applies — run a `chron` command (e.g. `chron list`) and let it
resolve; if `chron` isn't installed, fall back to reading/writing the
markdown directly per `format/SPEC.md`, using the same directory-walk logic
by hand.

Rule of thumb: repo-specific decisions/tasks/specs belong in that repo's
local `.chronicle/` (`chron init` there first if it doesn't exist yet).
Cross-project knowledge (general preferences, patterns that recur
everywhere, non-repo-specific study/progress notes) belongs in the global
vault.

## Reading

1. Start at the resolved vault's `notes/index.md` — grouped by type, a
   ready-made entry point.
2. Follow `[[wiki-links]]` from there rather than grepping blind; the graph
   structure is the point.
3. `chron search "<query>"` for full-text lookup when you don't know where
   to start. `chron list --type decision` (or any type) to browse by kind.
4. `chron ready` for open/in-progress tasks with no unresolved blockers —
   the closest thing to a to-do list, second-brain side only.
5. For a repo capability: `chron spec current <capability>` for the live
   plan, `chron spec history <capability>` to see how it got there and why.
   Don't assume a capability has no spec just because you don't see one in
   the repo root — check `.chronicle/specs/<capability>/` (or run
   `chron spec list`) before concluding nothing exists.

Never fabricate a `[[link]]` to a note you haven't actually confirmed
exists — a broken link is a `chron lint` failure waiting to happen, and an
invented cross-reference is worse than none.

## Writing (second brain)

Use `chron new <type> "<title>" [--area work|study|personal]`, not a raw
file write, whenever `chron` is available — it handles slugging, frontmatter
defaults, and index regeneration for you. One concept per file; don't stuff
multiple decisions into one note because they're topically related.

Pick the type by what's actually true, not by convenience:
- **decision** — a choice + rationale that should outlive this session
  ("why X over Y"). Durable, essentially write-once.
- **runbook** — operational how-to: steps, gotchas, install/setup process.
- **reference** — factual lookup content that isn't a decision or how-to.
- **preference** — how the user likes to work, stated once, applies broadly.
- **project** — durable context about an ongoing initiative.
- **task** — second-brain to-do/progress tracking (personal, study, or work
  chores that don't have a spec). `chron ready`/`chron done` manage the
  lifecycle. **Do not use `task` for repo feature work that has a spec** —
  see below.

After closing a task (`chron done`), it may prompt to graduate a decision
note if the work taught something durable. Take that offer when it applies
— that's the one-directional task→knowledge flow this format is built
around. It never goes the other way; don't turn a decision note back into
task busywork.

`chron link <a> <b>` for an explicit bidirectional cross-reference between
two existing notes, when a `[[link]]` in prose isn't enough. Run
`chron lint` after a batch of writes — fix `missing_type`/`invalid_type`/
`broken_link`/`orphan` findings before considering the writes done.

## Writing (spec-driven development)

Before implementing a non-trivial feature in a repo, check whether it has a
capability spec (`chron spec list`, or `chron spec current <capability>`).
If not, and the work is substantial enough to warrant one, propose writing
one (`chron spec new <capability> "<title>"`) rather than assuming ad-hoc
task notes are enough — the spec's whole value is being the plan an agent
(possibly a different session) implements against.

The lifecycle:
1. `chron spec new` — draft the requirements as a `proposed` spec. This is
   the plan; there's no separate task-tracking step for the work it
   describes.
2. Implement against it directly.
3. `chron spec implement <capability>` once done — this **freezes the file
   going forward**. Never hand-edit an implemented spec's content again,
   even for a typo — that's the whole point of the ledger model.
4. If requirements change later (even for an already-implemented
   capability), `chron spec revise <capability> "<title>"` — a full new
   snapshot of the current requirements plus a short note on what changed
   and why, never a diff against the old version. This automatically
   supersedes the current tip; don't hand-edit `superseded_by` yourself.

If you find yourself about to edit a `status: implemented` (or
`superseded`) spec file directly: stop, that's the immutability rule being
violated — use `chron spec revise` instead, even for a trivial correction.

**The full-snapshot rule is easy to get technically right but
substantively wrong** — using `chron spec revise` correctly (not
hand-editing the old file) still isn't enough if the new version's
Requirements section just points back at the old one instead of restating
it. Concretely, if v1 covered Google + GitHub login and product adds Apple
Sign-In:

- **Wrong** (delta, leans on the reader following a link to know what's
  actually true): `"Google and GitHub login shipped in [[v1]], live in
  prod, no further work needed here. New: add Apple Sign-In."`
- **Right** (full snapshot, self-contained — a reader shouldn't need to
  open v1 to know the current requirements): restate Google login, GitHub
  login, *and* Apple Sign-In as full peer requirements in v2's body. A
  short prose note on *why* the change happened is fine and useful
  alongside this — but it supplements the full requirement list, it
  doesn't replace restating any of it.

## Edge cases

- **`chron` not installed**: read/write the markdown by hand, following
  `format/SPEC.md` exactly (frontmatter shape, file layout, vault
  resolution) — the format is designed to not require the CLI.
- **Ambiguous vault** (no local `.chronicle/`, no global vault configured):
  say so and ask, rather than picking one and writing into the wrong place.
- **Cross-vault link** (`[[vaultname:path]]`): only write one if you've
  confirmed the target vault and note actually exist — check
  `~/.config/chron/vaults.json` for known vault names, don't guess.
- **A capability's spec chain looks forked or broken** (`chron spec
  current` errors, or `chron lint` reports `spec_broken_chain`): don't
  silently pick a version and proceed — that's a data-integrity problem
  worth surfacing, not resolving unilaterally.
