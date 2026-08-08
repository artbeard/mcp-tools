#!/usr/bin/env python3
"""stdio MCP server wrapping skill CLIs under skills/."""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

from mcp.server.mcpserver.server import MCPServer

from mcp_paths import GITLAB_ENV, ROOT, TIMER_STATE, TRACKER_ENV

SKILLS = ROOT / "skills"
PYTHON = sys.executable

DEFAULT_TRACKER_ENV = TRACKER_ENV
DEFAULT_GITLAB_ENV = GITLAB_ENV
DEFAULT_TIMER_STATE = TIMER_STATE

mcp = MCPServer(
    name="dev-tools",
    version="0.1.0",
    instructions=(
        "Deterministic agent tools for sandbox/git, Yandex Tracker, and GitLab. "
        "Write operations default to dry-run; set dry_run=false only after explicit user confirmation."
    ),
)


def _run(
    script: Path,
    args: list[str],
    *,
    cwd: str | None = None,
    env: dict[str, str] | None = None,
    command: list[str] | None = None,
) -> str:
    # command=None → обычный Python-скрипт; иначе готовая команда (например Go-бинарь)
    if command is None:
        if not script.is_file():
            return f"ERROR: script not found: {script}"
        command = [PYTHON, str(script), *args]
    else:
        command = [*command, *args]
    merged = os.environ.copy()
    if env:
        merged.update(env)
    result = subprocess.run(
        command,
        capture_output=True,
        text=True,
        cwd=cwd or str(ROOT),
        env=merged,
        check=False,
    )
    parts: list[str] = []
    if result.stdout:
        parts.append(result.stdout.rstrip())
    if result.stderr:
        parts.append(result.stderr.rstrip())
    body = "\n".join(parts).strip()
    if result.returncode != 0:
        return f"exit={result.returncode}\n{body}".strip()
    return body or f"exit={result.returncode}"


def _tracker_transition_cmd() -> list[str]:
    # Смена статуса Tracker уже на Go: берём собранный бинарь, иначе go run
    binary = ROOT / "bin" / "tracker-transition"
    if binary.is_file() and os.access(binary, os.X_OK):
        return [str(binary)]
    return ["go", "run", "./skills/yandex-tracker-commenter/cmd/tracker-transition"]


def _tracker_comment_cmd() -> list[str]:
    binary = ROOT / "bin" / "tracker-comment"
    if binary.is_file() and os.access(binary, os.X_OK):
        return [str(binary)]
    return ["go", "run", "./skills/yandex-tracker-commenter/cmd/tracker-comment"]


def _tracker_create_issue_cmd() -> list[str]:
    binary = ROOT / "bin" / "tracker-create-issue"
    if binary.is_file() and os.access(binary, os.X_OK):
        return [str(binary)]
    return ["go", "run", "./skills/yandex-tracker-issue-creator/cmd/tracker-create-issue"]


def _write_temp(text: str, suffix: str = ".txt") -> Path:
    handle = tempfile.NamedTemporaryFile("w", encoding="utf-8", suffix=suffix, delete=False)
    handle.write(text)
    handle.close()
    return Path(handle.name)


def _opt(flag: str, value: Any) -> list[str]:
    if value is None or value == "":
        return []
    return [flag, str(value)]


# --- ping ---


@mcp.tool(description="Health check for the mcp-tools server.")
def ping() -> str:
    return "pong"


# --- start-project-sandbox ---


@mcp.tool(
    description=(
        "Manage project sandboxes (list/check/status/start/stop/diagnose) via "
        "skills/start-project-sandbox. Discovers .workspace.yaml from cwd (walk-up). "
        "project is optional and must match manifest name/aliases when set."
    )
)
def start_project_sandbox(
    action: str,
    project: str | None = None,
    cwd: str | None = None,
    manifest: str | None = None,
) -> str:
    """action: list | check | status | start | stop | diagnose."""
    script = SKILLS / "start-project-sandbox" / "scripts" / "start_sandbox.py"
    action = action.strip().lower()
    allowed = {"list", "check", "status", "start", "stop", "diagnose"}
    if action not in allowed:
        return f"ERROR: action must be one of {sorted(allowed)}"
    args: list[str] = [f"--{action}"]
    if action != "list" and project:
        args.append(project)
    args.extend(_opt("--cwd", cwd))
    args.extend(_opt("--manifest", manifest))
    return _run(script, args, cwd=cwd)


# --- prepare-project-task ---


@mcp.tool(
    description=(
        "Prepare local task branches across repos described by .workspace.yaml. "
        "Never pushes. Modes: list, check, prepare. Discovers manifest from cwd."
    )
)
def prepare_project_task(
    mode: str,
    task_id: str | None = None,
    project: str | None = None,
    cwd: str | None = None,
    manifest: str | None = None,
) -> str:
    script = SKILLS / "prepare-project-task" / "scripts" / "prepare_task_branch.py"
    mode = mode.strip().lower()
    args: list[str] = []
    if mode == "list":
        args.append("--list")
    elif mode in {"check", "prepare"}:
        if not task_id:
            return "ERROR: task_id is required for check/prepare"
        args.extend([f"--{mode}", task_id])
    else:
        return "ERROR: mode must be list|check|prepare"
    args.extend(_opt("--project", project))
    args.extend(_opt("--cwd", cwd))
    args.extend(_opt("--manifest", manifest))
    return _run(script, args, cwd=cwd)


# --- prepare-project-merge-requests ---


@mcp.tool(
    description=(
        "Publish local task branches and create/reuse GitLab MRs using .workspace.yaml. "
        "Modes: list, check, create."
    )
)
def prepare_project_merge_requests(
    mode: str,
    task_id: str | None = None,
    project: str | None = None,
    cwd: str | None = None,
    manifest: str | None = None,
    title: str | None = None,
    description: str | None = None,
) -> str:
    script = SKILLS / "prepare-project-merge-requests" / "scripts" / "prepare_merge_requests.py"
    mode = mode.strip().lower()
    args: list[str] = []
    desc_path: Path | None = None
    try:
        if mode == "list":
            args.append("--list")
        elif mode in {"check", "create"}:
            if not task_id:
                return "ERROR: task_id is required for check/create"
            args.extend([f"--{mode}", task_id])
        else:
            return "ERROR: mode must be list|check|create"
        args.extend(_opt("--project", project))
        args.extend(_opt("--cwd", cwd))
        args.extend(_opt("--manifest", manifest))
        args.extend(_opt("--title", title))
        if description is not None:
            desc_path = _write_temp(description, suffix=".md")
            args.extend(["--description-file", str(desc_path)])
        return _run(script, args, cwd=cwd)
    finally:
        if desc_path is not None:
            desc_path.unlink(missing_ok=True)


# --- deploy-project-task-to-dev ---


@mcp.tool(
    description=(
        "Merge remote task branches into origin/dev using .workspace.yaml and push only origin/dev. "
        "Modes: list, check, deploy, continue."
    )
)
def deploy_project_task_to_dev(
    mode: str,
    task_id: str | None = None,
    project: str | None = None,
    cwd: str | None = None,
    manifest: str | None = None,
) -> str:
    script = SKILLS / "deploy-project-task-to-dev" / "scripts" / "deploy_task_to_dev.py"
    mode = mode.strip().lower()
    args: list[str] = []
    if mode == "list":
        args.append("--list")
    elif mode in {"check", "deploy"}:
        if not task_id:
            return "ERROR: task_id is required"
        args.extend([f"--{mode}", task_id])
    elif mode == "continue":
        if not task_id:
            return "ERROR: task_id is required"
        args.extend(["--continue", task_id])
    else:
        return "ERROR: mode must be list|check|deploy|continue"
    args.extend(_opt("--project", project))
    args.extend(_opt("--cwd", cwd))
    args.extend(_opt("--manifest", manifest))
    return _run(script, args, cwd=cwd)


# --- yandex-tracker-timekeeper ---


@mcp.tool(
    description=(
        "Local Yandex Tracker work timer: start/stop/status/write/clear. "
        "write defaults to dry-run; set dry_run=false to send worklog."
    )
)
def yandex_tracker_timekeeper(
    action: str,
    issue_key: str | None = None,
    comment: str | None = None,
    round_to: int = 1,
    ignore_active: bool = False,
    dry_run: bool = True,
    clear_active: bool = False,
    clear_sessions: bool = False,
    env_file: str | None = None,
    state_file: str | None = None,
) -> str:
    script = SKILLS / "yandex-tracker-timekeeper" / "scripts" / "tracker_time.py"
    action = action.strip().lower()
    args = [
        "--env-file",
        env_file or str(DEFAULT_TRACKER_ENV),
        "--state-file",
        state_file or str(DEFAULT_TIMER_STATE),
    ]
    if action == "start":
        if not issue_key:
            return "ERROR: issue_key required for start"
        args.extend(["start", issue_key])
        args.extend(_opt("--comment", comment or ""))
    elif action == "status":
        args.append("status")
    elif action == "stop":
        args.append("stop")
        args.extend(_opt("--comment", comment or ""))
    elif action == "write":
        args.append("write")
        args.extend(_opt("--issue-key", issue_key))
        args.extend(_opt("--comment", comment or ""))
        args.extend(["--round-to", str(round_to)])
        if ignore_active:
            args.append("--ignore-active")
        if not dry_run:
            args.extend(["--send", "--confirm-send"])
    elif action == "clear":
        args.append("clear")
        if clear_active:
            args.append("--active")
        if clear_sessions:
            args.append("--sessions")
        args.extend(_opt("--issue-key", issue_key))
    else:
        return "ERROR: action must be start|status|stop|write|clear"
    return _run(script, args)


# --- yandex-tracker-issue-creator ---


@mcp.tool(
    description=(
        "Create a Yandex Tracker issue. Defaults to dry-run; set dry_run=false only after "
        "explicit user approval of the exact payload."
    )
)
def yandex_tracker_issue_creator(
    queue: str,
    summary: str,
    description: str,
    dry_run: bool = True,
    project: str | None = None,
    project_field: str | None = None,
    tags: list[str] | None = None,
    period: str | None = None,
    period_field: str | None = None,
    sp: float | None = None,
    sp_field: str | None = None,
    assignee: str | None = None,
    fields: list[str] | None = None,
    env_dir: str | None = None,
) -> str:
    # Go CLI tracker-create-issue; description через tempfile
    desc_path = _write_temp(description, suffix=".md")
    try:
        args = [
            "--queue",
            queue,
            "--summary",
            summary,
            "--description-file",
            str(desc_path),
        ]
        args.extend(_opt("--env-dir", env_dir))
        args.extend(_opt("--project", project))
        args.extend(_opt("--project-field", project_field))
        for tag in tags or []:
            args.extend(["--tag", tag])
        args.extend(_opt("--period", period))
        args.extend(_opt("--period-field", period_field))
        if sp is not None:
            args.extend(["--sp", str(sp)])
        args.extend(_opt("--sp-field", sp_field))
        args.extend(_opt("--assignee", assignee))
        for field in fields or []:
            args.extend(["--field", field])
        if not dry_run:
            args.extend(["--send", "--confirm-send"])
        return _run(Path(), args, command=_tracker_create_issue_cmd())
    finally:
        desc_path.unlink(missing_ok=True)


# --- yandex-tracker-commenter ---


@mcp.tool(
    description=(
        "Post a Yandex Tracker issue comment. Defaults to dry-run; set dry_run=false only after "
        "explicit confirmation of issue key and full body."
    )
)
def yandex_tracker_comment(
    issue_key: str,
    body: str,
    dry_run: bool = True,
    markup_type: str | None = None,
    summonees: list[str] | None = None,
    reply_to_comment: str | None = None,
    env_dir: str | None = None,
) -> str:
    # Go CLI tracker-comment; body через tempfile
    body_path = _write_temp(body, suffix=".md")
    try:
        args = [
            "--issue-key",
            issue_key,
            "--body-file",
            str(body_path),
        ]
        args.extend(_opt("--env-dir", env_dir))
        args.extend(_opt("--markup-type", markup_type))
        for summonee in summonees or []:
            args.extend(["--summonee", summonee])
        args.extend(_opt("--reply-to-comment", reply_to_comment))
        if not dry_run:
            args.extend(["--send", "--confirm-send"])
        return _run(Path(), args, command=_tracker_comment_cmd())
    finally:
        body_path.unlink(missing_ok=True)


@mcp.tool(
    description=(
        "Transition a Yandex Tracker issue status. Defaults to dry-run; set dry_run=false only "
        "after explicit confirmation."
    )
)
def yandex_tracker_transition(
    issue_key: str,
    target: str = "Открыто",
    dry_run: bool = True,
) -> str:
    # Не Python-скрипт: зовём Go CLI tracker-transition
    args = ["--issue-key", issue_key, "--target", target]
    if not dry_run:
        args.extend(["--send", "--confirm-send"])
    return _run(Path(), args, command=_tracker_transition_cmd())


# --- gitlab-mr-commenter ---


@mcp.tool(
    description=(
        "Post a GitLab MR note or line discussion. Defaults to dry-run; set dry_run=false only "
        "after explicit confirmation of MR URL, place, and full body."
    )
)
def gitlab_mr_comment(
    mr_url: str,
    body: str,
    dry_run: bool = True,
    line_comment: bool = False,
    file_path: str | None = None,
    line: int | None = None,
    line_type: str = "new",
    env_file: str | None = None,
) -> str:
    script = SKILLS / "gitlab-mr-commenter" / "scripts" / "post_gitlab_mr_comment.py"
    body_path = _write_temp(body, suffix=".md")
    try:
        args = [
            "--mr-url",
            mr_url,
            "--body-file",
            str(body_path),
            "--env-file",
            env_file or str(DEFAULT_GITLAB_ENV),
            "--line-type",
            line_type,
        ]
        if line_comment:
            args.append("--line-comment")
            args.extend(_opt("--file-path", file_path))
            if line is not None:
                args.extend(["--line", str(line)])
        env: dict[str, str] | None = None
        if not dry_run:
            args.append("--send")
            env = {"CONFIRM_SEND": "YES"}
        return _run(script, args, env=env)
    finally:
        body_path.unlink(missing_ok=True)


if __name__ == "__main__":
    mcp.run(transport="stdio")
