#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


def _defaults():
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import TRACKER_ENV, TIMER_STATE
	return TRACKER_ENV, TIMER_STATE


DEFAULT_ENV_FILE, DEFAULT_STATE_FILE = _defaults()
LEGACY_STATE_FILES = (
	Path("/tmp/.tracker-time.json"),
	Path.home() / ".local" / "state" / "codex-writable" / "tracker-time.json",
)
ISSUE_KEY_RE = re.compile(r"^[A-Z][A-Z0-9]+-\d+$")


def utc_now() -> datetime:
	return datetime.now(timezone.utc)


def parse_dt(value: str) -> datetime:
	return datetime.fromisoformat(value.replace("Z", "+00:00"))


def format_local(value: datetime) -> str:
	return value.astimezone().strftime("%Y-%m-%d %H:%M:%S %z")


def normalize_issue_key(issue_key: str) -> str:
	normalized = issue_key.strip().upper()
	if ISSUE_KEY_RE.match(normalized) is None:
		raise RuntimeError("unsupported issue key format")

	return normalized


def load_state(path: Path) -> dict:
	if not path.exists():
		if path == DEFAULT_STATE_FILE:
			for legacy_path in LEGACY_STATE_FILES:
				if legacy_path.exists():
					return json.loads(legacy_path.read_text(encoding="utf-8"))
		return {"active": None, "sessions": []}

	return json.loads(path.read_text(encoding="utf-8"))


def save_state(path: Path, state: dict) -> None:
	path.parent.mkdir(parents=True, exist_ok=True)
	path.write_text(json.dumps(state, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def load_env(path: Path) -> dict[str, str]:
	if not path.exists():
		raise RuntimeError(f"env file not found: {path}")

	values: dict[str, str] = {}
	for raw_line in path.read_text(encoding="utf-8").splitlines():
		line = raw_line.strip()
		if not line or line.startswith("#") or "=" not in line:
			continue
		key, value = line.split("=", 1)
		values[key.strip()] = value.strip().strip("\"'")

	return values


def resolve_org(env: dict[str, str]) -> tuple[str, str]:
	if env.get("TRACKER_CLOUD_ORG_ID"):
		return "X-Cloud-Org-ID", env["TRACKER_CLOUD_ORG_ID"]
	if env.get("TRACKER_ORG_ID"):
		return "X-Org-ID", env["TRACKER_ORG_ID"]
	raise RuntimeError("TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required")


def request_json(url: str, token: str, org_header: str, org_id: str, payload: dict[str, str]) -> dict:
	request = urllib.request.Request(
		url,
		data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
		headers={
			"Authorization": f"OAuth {token}",
			org_header: org_id,
			"Accept": "application/json",
			"Content-Type": "application/json",
		},
		method="POST",
	)
	try:
		with urllib.request.urlopen(request, timeout=30) as response:
			body = response.read().decode("utf-8")
			return json.loads(body) if body else {}
	except urllib.error.HTTPError as error:
		body = error.read().decode("utf-8", errors="replace")
		raise RuntimeError(f"Tracker API HTTP {error.code}: {body}") from error


def seconds_to_human(seconds: int) -> str:
	minutes = max(0, int(round(seconds / 60)))
	hours, remainder = divmod(minutes, 60)
	if hours and remainder:
		return f"{hours} ч {remainder} мин"
	if hours:
		return f"{hours} ч"
	return f"{remainder} мин"


def seconds_to_iso_duration(seconds: int) -> str:
	minutes = max(1, int(round(seconds / 60)))
	hours, remainder = divmod(minutes, 60)
	if hours and remainder:
		return f"PT{hours}H{remainder}M"
	if hours:
		return f"PT{hours}H"
	return f"PT{remainder}M"


def round_seconds(seconds: int, round_to_minutes: int) -> int:
	if round_to_minutes <= 1:
		return max(60, seconds)

	step = round_to_minutes * 60
	return max(step, int(round(seconds / step) * step))


def active_elapsed_seconds(active: dict) -> int:
	return int((utc_now() - parse_dt(active["started_at"])).total_seconds())


def command_start(args: argparse.Namespace) -> None:
	state = load_state(args.state_file)
	if state.get("active") is not None:
		active = state["active"]
		raise RuntimeError(
			f"timer already active for {active['issue_key']} since {format_local(parse_dt(active['started_at']))}"
		)

	issue_key = normalize_issue_key(args.issue_key)
	state["active"] = {
		"issue_key": issue_key,
		"started_at": utc_now().isoformat(),
		"comment": args.comment.strip(),
	}
	save_state(args.state_file, state)
	print(json.dumps({"status": "STARTED", "issue_key": issue_key}, ensure_ascii=False, indent=2))


def command_status(args: argparse.Namespace) -> None:
	state = load_state(args.state_file)
	active = state.get("active")
	sessions = state.get("sessions", [])
	result = {
		"active": None,
		"pending_sessions": len(sessions),
		"pending_seconds": sum(int(item["duration_seconds"]) for item in sessions),
	}
	if active is not None:
		elapsed = active_elapsed_seconds(active)
		result["active"] = {
			"issue_key": active["issue_key"],
			"started_at": active["started_at"],
			"started_at_local": format_local(parse_dt(active["started_at"])),
			"elapsed_seconds": elapsed,
			"elapsed": seconds_to_human(elapsed),
			"comment": active.get("comment", ""),
		}
	result["pending"] = seconds_to_human(result["pending_seconds"])
	print(json.dumps(result, ensure_ascii=False, indent=2))


def command_stop(args: argparse.Namespace) -> None:
	state = load_state(args.state_file)
	active = state.get("active")
	if active is None:
		raise RuntimeError("no active timer")

	stopped_at = utc_now()
	started_at = parse_dt(active["started_at"])
	duration_seconds = max(60, int((stopped_at - started_at).total_seconds()))
	comment = args.comment.strip() or active.get("comment", "")
	session = {
		"issue_key": active["issue_key"],
		"started_at": active["started_at"],
		"stopped_at": stopped_at.isoformat(),
		"duration_seconds": duration_seconds,
		"comment": comment,
	}
	state["active"] = None
	state.setdefault("sessions", []).append(session)
	save_state(args.state_file, state)
	print(json.dumps({
		"status": "STOPPED",
		"issue_key": session["issue_key"],
		"duration_seconds": duration_seconds,
		"duration": seconds_to_human(duration_seconds),
		"comment": comment,
	}, ensure_ascii=False, indent=2))


def select_sessions(state: dict, issue_key: str | None) -> list[dict]:
	sessions = state.get("sessions", [])
	if issue_key is None:
		return sessions

	normalized = normalize_issue_key(issue_key)
	return [item for item in sessions if item["issue_key"] == normalized]


def command_write(args: argparse.Namespace) -> None:
	state = load_state(args.state_file)
	if state.get("active") is not None and not args.ignore_active:
		active = state["active"]
		raise RuntimeError(
			f"timer is active for {active['issue_key']}; stop it first or pass --ignore-active"
		)

	sessions = select_sessions(state, args.issue_key)
	if not sessions:
		raise RuntimeError("no stopped sessions to write")

	issue_keys = {item["issue_key"] for item in sessions}
	if len(issue_keys) != 1:
		raise RuntimeError("sessions contain multiple issues; pass --issue-key")

	issue_key = next(iter(issue_keys))
	started_at = min(parse_dt(item["started_at"]) for item in sessions)
	raw_seconds = sum(int(item["duration_seconds"]) for item in sessions)
	duration_seconds = round_seconds(raw_seconds, args.round_to)
	comment = args.comment.strip()
	if not comment:
		comments = [item.get("comment", "").strip() for item in sessions if item.get("comment", "").strip()]
		comment = "; ".join(dict.fromkeys(comments)) or "Работа по задаче."

	payload = {
		"start": started_at.astimezone().strftime("%Y-%m-%dT%H:%M:%S.000%z"),
		"duration": seconds_to_iso_duration(duration_seconds),
		"comment": comment,
	}
	base_result = {
		"issue_key": issue_key,
		"sessions": len(sessions),
		"raw_seconds": raw_seconds,
		"raw_duration": seconds_to_human(raw_seconds),
		"duration_seconds": duration_seconds,
		"duration": seconds_to_human(duration_seconds),
		"payload": payload,
	}

	if not args.send:
		print(json.dumps({"status": "DRY_RUN", **base_result}, ensure_ascii=False, indent=2))
		return

	if not args.confirm_send and os.environ.get("CONFIRM_SEND") != "YES":
		raise RuntimeError("--send requires --confirm-send or CONFIRM_SEND=YES")

	env = load_env(args.env_file)
	base_url = env.get("TRACKER_BASE_URL", "https://api.tracker.yandex.net").rstrip("/")
	token = env.get("TRACKER_OAUTH_TOKEN", "")
	if not token:
		raise RuntimeError("TRACKER_OAUTH_TOKEN is empty")
	org_header, org_id = resolve_org(env)
	url = f"{base_url}/v2/issues/{urllib.parse.quote(issue_key, safe='')}/worklog"
	result = request_json(url, token, org_header, org_id, payload)

	written_ids = {id(item) for item in sessions}
	state["sessions"] = [item for item in state.get("sessions", []) if id(item) not in written_ids]
	save_state(args.state_file, state)
	print(json.dumps({"status": "POSTED", **base_result, "result": result}, ensure_ascii=False, indent=2))


def command_clear(args: argparse.Namespace) -> None:
	state = load_state(args.state_file)
	if args.active:
		state["active"] = None
	if args.sessions:
		if args.issue_key:
			issue_key = normalize_issue_key(args.issue_key)
			state["sessions"] = [item for item in state.get("sessions", []) if item["issue_key"] != issue_key]
		else:
			state["sessions"] = []
	save_state(args.state_file, state)
	print(json.dumps({"status": "CLEARED"}, ensure_ascii=False, indent=2))


def build_parser() -> argparse.ArgumentParser:
	parser = argparse.ArgumentParser(description="Local Yandex Tracker task timer.")
	parser.add_argument("--state-file", type=Path, default=DEFAULT_STATE_FILE)
	parser.add_argument("--env-file", type=Path, default=DEFAULT_ENV_FILE)
	subparsers = parser.add_subparsers(dest="command", required=True)

	start = subparsers.add_parser("start", help="Start timer for an issue.")
	start.add_argument("issue_key")
	start.add_argument("--comment", default="")
	start.set_defaults(func=command_start)

	status = subparsers.add_parser("status", help="Show active timer and pending sessions.")
	status.set_defaults(func=command_status)

	stop = subparsers.add_parser("stop", help="Stop active timer and store a pending session.")
	stop.add_argument("--comment", default="")
	stop.set_defaults(func=command_stop)

	write = subparsers.add_parser("write", help="Write stopped sessions to Tracker worklog.")
	write.add_argument("--issue-key")
	write.add_argument("--comment", default="")
	write.add_argument("--round-to", type=int, default=1, help="Round duration to N minutes.")
	write.add_argument("--ignore-active", action="store_true")
	write.add_argument("--send", action="store_true")
	write.add_argument("--confirm-send", action="store_true")
	write.set_defaults(func=command_write)

	clear = subparsers.add_parser("clear", help="Clear local timer state.")
	clear.add_argument("--active", action="store_true")
	clear.add_argument("--sessions", action="store_true")
	clear.add_argument("--issue-key")
	clear.set_defaults(func=command_clear)

	return parser


def main() -> int:
	args = build_parser().parse_args()
	args.func(args)
	return 0


if __name__ == "__main__":
	try:
		raise SystemExit(main())
	except Exception as exc:
		print(f"ERROR: {exc}", file=sys.stderr)
		raise SystemExit(1)
