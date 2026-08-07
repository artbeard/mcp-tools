---
name: start-project-sandbox
description: Start, inspect, diagnose, or stop configured backend, frontend, and supporting development components through one deterministic lifecycle script with Docker, tmux, process, and readiness verification. Use only when explicitly invoked for a sandbox operation on a configured project.
---

# Start Project Sandbox

Manage project components with one host-level invocation of `scripts/start_sandbox.py` per user operation.

## Workflow

1. Extract the exact project key or configured alias and requested action from the explicit invocation.
2. Run exactly one matching command:
   - start: `python3 scripts/start_sandbox.py --start <project>`
   - stop: `python3 scripts/start_sandbox.py --stop <project>`
   - inspect: `python3 scripts/start_sandbox.py --status <project>`
   - diagnose: `python3 scripts/start_sandbox.py --diagnose <project>`
3. Report the result for every configured component.

The user's explicit request to start or stop a sandbox is the semantic authorization for that complete lifecycle operation. Do not ask the user to confirm it again.

The script performs validation, lifecycle actions, readiness checks, and bounded failure diagnostics internally. Treat only `status: ok` plus `ready: true` for every started component as success. Never infer success from command submission or session creation.

Run lifecycle and status commands in the persistent host context, not the ephemeral command sandbox. Make at most one approval request for the complete invocation and use the narrow persistent prefix `python3 .../start-project-sandbox/scripts/start_sandbox.py`. Never issue separate requests for Docker, tmux, process, or readiness checks.

Ask for new authorization only when the intended action expands beyond the requested lifecycle operation, such as deleting volumes or data, rebuilding images, running migrations, or stopping unrelated sessions, containers, or processes.

Do not run separate log commands. The script includes bounded diagnostics only for failed components or explicit `--diagnose`.

Use `python3 scripts/start_sandbox.py --list` to list accepted project keys and aliases. Listing and status are read-only operations and require no semantic user confirmation. Configured aggregate projects may reuse atomic components without duplicating their lifecycle definitions.

Read ``config/sandboxes.json` (see config/README.md)` when project composition, aliases, components, paths, commands, probes, or timeouts need inspection or updating. Keep this personal config outside all project repositories.

## Safety rules

- Never kill, replace, rename, or inject commands into a pre-existing unmanaged tmux session.
- Stop only sessions marked as owned by this skill for the exact project and component. Leave unmarked or mismatched sessions untouched.
- Validate every component before creating the first session.
- Never run package installation, image rebuild, database migration, reset, cleanup, or teardown unless separately requested.
- `--stop` may execute only the configured graceful stop command and kill the owned tmux session. Never remove Docker volumes.
- Never attach interactively or stream logs by default.
- Stop on startup failure and report every component already changed during the attempt.
- Require a successful Compose probe for backend components and an HTTP probe for frontend components.

Read-only SQL `SELECT` operations performed through external DB tools are outside this sandbox lifecycle and do not require user approval. Mutating SQL remains subject to the applicable database tool's safety rules.
