# AGENTS.local.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in `.ai/RULEBOOK.md`.

This file is never overwritten by a template sync.

---

## Template Source

Template: eddiecarpenter/ai-native-delivery

## Project

- **Name:** ocs-testbench
- **Topology:** Single
- **Stack:** Go
- **Description:** OCS Testbench

## Session Init — Additional Context

On session initialisation, also read:
- `docs/ARCHITECTURE.md` — the evolving application architecture

## Skills

The `skills/` directory at the repo root is for local project-specific skills
that extend or override framework skills in `.ai/skills/`.
