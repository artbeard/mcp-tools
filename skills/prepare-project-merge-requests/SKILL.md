---
name: prepare-project-merge-requests
description: Deterministically publish configured local task branches and create or reuse GitLab merge requests for every repository of a multi-repository project or monorepo, with automatic project detection, complete preflight, fast-forward-only task branch publication, and protected-branch push guards. Use when the user asks to prepare, publish, open, or create merge requests for a project task.
---

# Prepare Project Merge Requests

Create GitLab merge requests with `scripts/prepare_merge_requests.py`.

## Workflow

1. Extract the task ID exactly. Never normalize it.
2. Derive the MR title from each task branch head commit unless the user supplies one exact title.
3. Run `python3 scripts/prepare_merge_requests.py --check <task-id> [--project <key>] [--title <title>]`.
4. Report and stop on any failed repository check.
5. After a successful check, run the matching command with `--create` instead of `--check`.
6. Require `status: ok` and use the validated top-level `mr_urls` list as the result. Report every URL. Treat `already_exists` and `no_changes` as idempotent successes.

Let the script detect the project from the current directory. Pass `--project` only when the user explicitly names another project. Use `--description-file` only for exact user-approved UTF-8 text.

The script reads the GitLab token only from the configured local env file. Never print, copy, or ask the user to paste the token.

## Branch direction invariant

- Allow MR targets only `master` or `main`. Never create an MR from `dev` to `master`, `main`, or a task branch.
- Reject `dev`, `master`, and `main` as task/source branch names independently of configuration.
- Before any push, fetch the actual remote target and reject the task branch when its commits outside that target contain any merge commit. This is a hard contamination guard: task branches must be linear and based on `master`/`main`, never on `dev`.
- Do not auto-rebase, auto-cherry-pick, or force-push a contaminated task branch. Stop and require reconstruction from the current remote target with explicitly selected task commits.

## Push boundary

- Push only the exact task refspec `refs/heads/<task>:refs/heads/<task>`.
- Never push `master`, `main`, `dev`, another base branch, tags, or multiple refspecs.
- Never force-push.
- Creating MRs is an external write. Treat an explicit user request to create/publish the task MRs as authorization for the complete checked operation.

## Safety rules

- Require every configured repository to be clean and free of in-progress Git operations.
- Require the local task branch and remote target branch to exist.
- Require the task branch to share history with the remote target and contain no merge commits outside the target history.
- Reject protected branch names as task IDs.
- Require an existing remote task branch to be an ancestor of the local task branch; reject non-fast-forward publication.
- Skip a repository with `no_changes` when the task branch has no commits outside the current remote target; do not push it or create an empty MR.
- Complete preflight for all repositories before the first push.
- Push all task branches before creating the first MR.
- Validate every GitLab `iid` and `web_url` against the expected project and MR path. Never report success if a created or existing MR lacks a validated URL.
- Return all validated links in the top-level `mr_urls` array; exclude `no_changes` repositories from that array.
- Stop on failure and report the phase and exact partial state; never claim project-wide success for a partial result.
- Read ``.workspace.yaml`` only when project membership, paths, remotes, targets, or GitLab settings need inspection.
