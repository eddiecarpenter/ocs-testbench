# AGENTS.md:OK
 
# LOAD ORDER (ENFORCED)
1. .ai/RULEBOOK.md
2. LOCALRULES.md
3. skills/session-init/SKILL.md

## Bootstrap Rule

The agentic framework lives under `.ai/` and is managed by `gh agentic mount`.
The directory is gitignored — every fresh clone, worktree, or CI checkout
starts without it. Mounting is therefore **step zero of every session**, before
any other initialisation.

### Procedure

Run these checks at session start, in order, **before** invoking `session-init`:

1. **Mount is missing.** If `.ai/` does not exist, run `gh agentic mount`
   and proceed. Do not ask — mounting is idempotent and side-effect-free.

2. **Mount is stale.** If `.ai/` exists, run `gh agentic info` and compare
   the reported `Framework (local)` version against the repo variable
   `AGENTIC_FRAMEWORK_VERSION` (if set). If they differ, run
   `gh agentic mount <AGENTIC_FRAMEWORK_VERSION>` to sync, then proceed.

3. **Mount step failed.** Only in this case halt and report:
   - **Interactive context:** surface the exact error from `gh agentic mount`
     and ask the human to resolve.
   - **CI context:** fail the job with the same error and the message
     `"Framework mount failed — add a mount step before the pipeline."`

4. **Mount is healthy.** Invoke `session-init` as defined in `.ai/skills/session-init.md`.

### Why this is proactive

A missing or stale mount is the single most common cause of session confusion
(wrong skills loaded, stale RULEBOOK, broken workflow references). It is
cheap to detect, cheap to fix, and never destructive — so the agent mounts
automatically rather than stopping to ask. The only time the agent halts is
when the mount step itself returns a non-zero exit code.
