---
name: deploy-project-task-to-dev
description: Deterministically merge configured remote task branches into dev across every repository of a multi-repository project or monorepo, then push only origin/dev, with automatic project detection, complete preflight, conflict prediction, resumable manual conflict resolution, remote race checks, and exact partial-state reporting. Use when the user asks to upload, merge, deploy, or deliver a project task branch to dev.
---

# Deploy Project Task to Dev

Integrate task branches with Go CLI `bin/deploy-task` (собрать: `make build-deploy-task`).

## Workflow

1. Extract the task ID exactly and let the CLI detect the project from the current directory.
2. Run `./bin/deploy-task --check <task-id> [--project <key>]`.
3. If the result is `ok`, run the matching `--deploy` command.
4. If either command returns `needs_resolution`, resolve only the reported repository and files, preserving both intended task behavior and current dev behavior. Stage every resolved file with `git add`, then run `--continue <task-id>`.
5. Repeat resolution and `--continue` until the CLI returns `status: ok` and `push_summary.failed` plus `push_summary.not_attempted` are empty.

An explicit user request to deploy the task to dev authorizes the checked merge commits and exact dev pushes. Never broaden it to another branch.

## Branch direction invariant

- `dev` is a downstream demonstration branch and may only receive task branches or be fast-forwarded/merged from `master` or `main` in a separately authorized workflow.
- Never use `dev` as the base or source for a task branch, `master`, or `main`.
- Before integrating task → dev, fetch the configured `master`/`main` base and reject the task branch when its commits outside that base contain any merge commit. Task branches must be linear; a merge commit outside base history is treated as possible `dev` contamination.
- Never merge, rebase, cherry-pick, push, or open an MR from `dev` into `master`, `main`, or a task branch.
- Do not auto-clean a contaminated task branch. Stop and require reconstruction from the current remote `master` or `main`.

## Conflict contract

- Treat exit code `4` and `status: needs_resolution` as an expected resumable outcome, not success or a generic failure.
- Never choose `ours`, `theirs`, or an automatic conflict strategy for the user.
- Never claim the deployment completed while a repository has unmerged or unstaged resolution files.
- Leave the actual merge open only in the single repository named by `active_resolution`.
- Run `--continue` only after inspecting and staging every resolution.
- Do not start a second manual merge while one is active.

## Push boundary

- Push only `refs/heads/dev:refs/heads/dev` to the configured origin.
- Never push `master`, `main`, task branches, tags, multiple refspecs, or force updates.
- Create all required local merge commits before the first dev push.
- Recheck every remote dev SHA immediately before the push phase.
- Cross-repository push cannot be atomic. On a push failure, finish the report without additional pushes and classify every repository as `pushed`, `failed`, `not_attempted`, or `already_deployed` in `push_summary`.

## Safety rules

- Require clean work trees except during an authorized active merge on `dev`.
- Reject detached HEAD and unsupported in-progress Git operations.
- Never use stash, reset, clean, rebase, force checkout, or history rewriting.
- Never push anything other than `dev`.
- Stop on failure and report the exact phase and partial state from the CLI JSON.
- Read ``.workspace.yaml`` only when project membership, paths, remotes, or branch settings need inspection.
