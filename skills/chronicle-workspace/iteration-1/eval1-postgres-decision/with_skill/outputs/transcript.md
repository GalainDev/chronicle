# Transcript — eval1-postgres-decision (with_skill)

Working directory: `/tmp/claude/skill-eval/eval1-postgres-decision/with_skill/repo`
(fresh empty repo, `git init -q .` already run; `PATH` includes `/tmp/claude` so `chron` resolves)

## Commands run, in order

1. `chron list`
   - Reasoning: per the skill's "Before anything: resolve the vault" step,
     don't guess which vault applies — run a `chron` command and let it
     resolve. This showed no local `.chronicle/` existed yet, so `chron`
     fell back to the global vault (populated with unrelated notes from
     other projects).

2. `chron init`
   - Reasoning: the task is a decision about *this* project's own
     infrastructure (its Postgres connection pool, its retry code, its
     choice of pgbouncer). The skill's rule of thumb is that repo-specific
     decisions belong in that repo's local `.chronicle/`, created with
     `chron init` if it doesn't exist yet — so a local vault was
     initialized in the repo instead of writing into the global vault.

3. `chron list` (again, post-init)
   - Reasoning: sanity check that the local vault now resolves and is
     empty, before writing into it.

4. `chron new decision "Use pgbouncer to fix Postgres connection pool exhaustion instead of raising max_connections" --area work`
   - Reasoning: choosing the note type by what's actually true, per the
     skill's guidance. This is a choice + rationale that should outlive the
     session ("why pgbouncer over just bumping max_connections") — exactly
     the **decision** type, not a runbook (no operational steps/gotchas
     were requested), not a task (the work is already done, nothing to
     track going forward), and not a reference (it's not neutral factual
     lookup content — it's a judgment call with a rationale). `--area work`
     because this is a work/professional engineering decision, not
     personal or study.
   - Used `chron new` rather than a raw file write, per the skill's
     instruction, so slugging/frontmatter/index regeneration were handled
     automatically.

5. Edited the generated note file directly (`Edit` tool) to fill in the
   body: Problem / Decision / Rationale / Follow-up sections, capturing the
   leaked-connection root cause, the fix, and why pgbouncer was chosen over
   simply raising `max_connections` (symptom vs. root cause).
   - Reasoning: `chron new` only scaffolds frontmatter + title; the actual
     content is written by hand into the created file, same as the skill
     describes.

6. `chron lint`
   - Reasoning: skill instructs running `chron lint` after a batch of
     writes and fixing `missing_type`/`invalid_type`/`broken_link`/`orphan`
     findings before considering the writes done. Lint reported one
     `orphan` finding for the new note. This was left as-is: the vault is
     brand new with exactly one note, so there is no other existing note to
     link it from or to — inventing a `[[link]]` to make it non-orphan
     would violate the skill's explicit rule against fabricating links to
     notes that don't exist. No `missing_type`, `invalid_type`, or
     `broken_link` findings were present.

## Output

Final vault copied to
`.../with_skill/outputs/.chronicle/notes/decisions/use-pgbouncer-to-fix-postgres-connection-pool-exhaustion-instead-of-raising-max-connections.md`,
and `index.md` regenerated automatically by `chron new`.
