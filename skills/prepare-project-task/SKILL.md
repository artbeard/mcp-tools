---
name: prepare-project-task
description: Deterministically prepare a local task branch across every Git repository of a configured multi-repository project or monorepo, with automatic project detection, complete safety preflight, remote and fast-forward verification, and a strict prohibition on Git push. Use when the user asks to start, prepare, initialize, or create branches for a configured project task.
---

# Prepare Project Task

Prepare local task branches with `scripts/prepare_task_branch.py`.

## Workflow

1. Extract the task ID exactly. Never invent or normalize it.
2. Let the script detect the project from the current directory. Pass `--project <key>` only when the user names another configured project explicitly.
3. Run `python3 scripts/prepare_task_branch.py --check <task-id> [--project <key>]`.
4. If the check succeeds, run `python3 scripts/prepare_task_branch.py --prepare <task-id> [--project <key>]`.
5. Report the selected project type and resulting local branch in every repository.

Use `--list` to inspect configured projects. Read ``.workspace.yaml`` only when project membership, type, paths, remotes, or base branches need inspection or updating.

The prepare command repeats the complete preflight, inspects remote branches, fetches only configured base refs, verifies fast-forward compatibility across every repository, updates configured local base branches with `merge --ff-only`, and creates the task branch locally.

## Branch direction invariant

- Create task branches only from the latest configured remote `master` or `main`.
- Never create or update a task branch from `dev`, even when the local checkout is currently on `dev`.
- Treat `dev` only as a downstream demonstration branch. Never merge, rebase, or cherry-pick `dev` into a task branch, `master`, or `main`.
- Reject configuration whose `base_branch` is anything other than `master` or `main`; do not rely only on a configurable protected-branch list.
- Never repair a contaminated task branch in this workflow. Stop and require reconstruction from the current remote `master` or `main` with explicitly selected task commits.

## Absolute push prohibition

- Never run `git push` as part of this skill.
- Never push, force-push, publish commits, or change remote `master`, `main`, or any configured base branch.
- Treat `push_allowed: false` as an invariant, not as a prompt for confirmation.
- Branch publication is a separate user-authorized workflow outside this skill.

## Safety rules

- Treat staged, unstaged, untracked, conflicted, detached, merge, rebase, cherry-pick, revert, and bisect states as unsafe.
- If one repository is unsafe, change no working tree or branch in the project.
- Process every configured repository for a multi-repository project; never use a clean subset.
- Never use stash, reset, clean, rebase, forced checkout, forced branch creation, branch deletion, or history rewriting.
- Never use a protected base branch name as a task ID.
- Never overwrite or reuse a local or remote task branch.
- Stop on any failure and report the exact phase and partial state from the script JSON.
