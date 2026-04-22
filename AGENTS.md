# AGENTS.md

@.ai/RULEBOOK.md
@AGENTS.local.md

## Bootstrap Rule

If the .ai/ directory does not exist, stop immediately.

- **Interactive context:** Instruct the user to run:
  `gh agentic mount`

- **CI context:** Fail with the message:
  "Framework not mounted. Add a mount step before the pipeline:
  `gh agentic mount`"

Do not proceed with any other work until the framework is mounted.
