---
name: start-project-sandbox
description: Start, inspect, diagnose, or stop configured backend, frontend, and supporting development components through one deterministic lifecycle CLI with Docker, tmux, and readiness verification. Use only when explicitly invoked for a sandbox operation on a configured project.
---

# Start Project Sandbox

Manage project components with Go CLI `bin/start-sandbox` (собрать: `make build-start-sandbox`).

## Workflow

1. Extract the exact project key or configured alias and requested action from the explicit invocation.
2. Run exactly one matching command:
   - start: `./bin/start-sandbox --start <project>`
   - stop: `./bin/start-sandbox --stop <project>`
   - inspect: `./bin/start-sandbox --status <project>`
   - diagnose: `./bin/start-sandbox --diagnose <project>`
3. Report the result for every configured component.

The user's explicit request to start or stop a sandbox is the semantic authorization for that complete lifecycle operation. Do not ask the user to confirm it again.

The CLI performs validation, lifecycle actions, readiness checks, and bounded failure diagnostics internally. Treat only `status: ok` plus `ready: true` for every started component as success. Never infer success from command submission or session creation.

Run lifecycle and status commands in the persistent host context, not the ephemeral agent sandbox. Make at most one approval request for the complete invocation. Never issue separate requests for Docker, tmux, or readiness checks.

Ask for new authorization only when the intended action expands beyond the requested lifecycle operation, such as deleting volumes or data, rebuilding images, running migrations, or stopping unrelated sessions, containers, or processes.

Do not run separate log commands. The CLI includes bounded diagnostics only for failed components or explicit `--diagnose`.

Use `./bin/start-sandbox --list` to list accepted project keys and aliases. Listing and status are read-only operations and require no semantic user confirmation.

Read `.workspace.yaml` (see config/workspace.example.yaml) when project composition, aliases, components, paths, commands, probes, or timeouts need inspection or updating. Layout stays in the workspace `.workspace.yaml`; host env supplies credentials for docker/npm as needed.

## Safety rules

- Never kill, replace, rename, or inject commands into a pre-existing unmanaged tmux session.
- Stop only sessions marked as owned by this skill for the exact project and component. Leave unmarked or mismatched sessions untouched.
- Validate every component before creating the first session.
- Never run package installation, image rebuild, database migration, reset, cleanup, or teardown unless separately requested.
- `--stop` may execute only the configured graceful stop command and kill the owned tmux session. Never remove Docker volumes.
- Never invent component definitions. If configuration is missing or invalid, stop and report the exact validation error.
- Supported runtimes: `compose` and `tmux` only (`process` is not implemented).
