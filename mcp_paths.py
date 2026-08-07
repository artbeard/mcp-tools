"""Repo-root path helpers for skill scripts and the MCP server."""
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parent
CONFIG_DIR = ROOT / "config"

TRACKER_ENV = CONFIG_DIR / "tracker.env"
GITLAB_ENV = CONFIG_DIR / "gitlab.env"
TIMER_STATE = Path.home() / ".local" / "state" / "mcp-tools" / "tracker-time.json"

PROJECTS_PREPARE = CONFIG_DIR / "projects-prepare.json"
PROJECTS_MR = CONFIG_DIR / "projects-mr.json"
PROJECTS_DEPLOY = CONFIG_DIR / "projects-deploy.json"
SANDBOXES = CONFIG_DIR / "sandboxes.json"


def resolve_config_path(value: str | Path, *, base: Path = ROOT) -> Path:
    path = Path(value).expanduser()
    if not path.is_absolute():
        path = base / path
    return path
