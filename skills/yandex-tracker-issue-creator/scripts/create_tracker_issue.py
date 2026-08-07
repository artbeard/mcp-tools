#!/usr/bin/env python3
"""Create a Yandex Tracker issue with dry-run by default."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


def _default_env_file() -> Path:
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import TRACKER_ENV
	return TRACKER_ENV


DEFAULT_ENV_FILE = _default_env_file()
QUEUE_RE = re.compile(r"^[A-Z][A-Z0-9_]*$")


def load_env(path: Path) -> dict[str, str]:
	values: dict[str, str] = {}
	if not path.exists():
		raise RuntimeError(f"env file not found: {path}")

	for raw_line in path.read_text(encoding="utf-8").splitlines():
		line = raw_line.strip()
		if not line or line.startswith("#"):
			continue
		if "=" not in line:
			continue
		key, value = line.split("=", 1)
		values[key.strip()] = value.strip().strip("\"'")

	return values


def resolve_org(env_values: dict[str, str]) -> tuple[str, str]:
	cloud_org_id = env_values.get("TRACKER_CLOUD_ORG_ID", "")
	if cloud_org_id:
		return "X-Cloud-Org-ID", cloud_org_id

	org_id = env_values.get("TRACKER_ORG_ID", "")
	if org_id:
		return "X-Org-ID", org_id

	raise RuntimeError("TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required")


def request_json(
	url: str,
	token: str,
	org_id: str,
	org_header: str,
	method: str = "GET",
	data: dict[str, Any] | None = None,
) -> dict[str, Any]:
	encoded_data = None
	headers = {
		"Authorization": f"OAuth {token}",
		org_header: org_id,
		"Accept": "application/json",
	}

	if data is not None:
		encoded_data = json.dumps(data, ensure_ascii=False).encode("utf-8")
		headers["Content-Type"] = "application/json"

	request = urllib.request.Request(url, data=encoded_data, headers=headers, method=method)
	try:
		with urllib.request.urlopen(request, timeout=30) as response:
			body = response.read().decode("utf-8")
			return json.loads(body) if body else {}
	except urllib.error.HTTPError as error:
		body = error.read().decode("utf-8", errors="replace")
		raise RuntimeError(f"Tracker API HTTP {error.code}: {body}") from error


def parse_selector(value: str) -> str | int | dict[str, str | int]:
	value = value.strip()
	if not value:
		raise RuntimeError("selector value is empty")

	if ":" in value:
		kind, raw_identifier = value.split(":", 1)
		kind = kind.strip().lower()
		identifier = raw_identifier.strip()
		if not identifier:
			raise RuntimeError("selector identifier is empty")
		if kind == "login":
			return {"login": identifier}
		if kind == "id":
			return {"id": identifier}
		if kind == "cloud":
			return {"cloudUid": identifier}
		if kind == "uid":
			return {"uid": int(identifier)}
		if kind == "passport":
			return {"passportUid": int(identifier)}
		if kind == "tracker":
			return {"trackerUid": int(identifier)}
		raise RuntimeError(f"unsupported selector kind: {kind}")

	return {"login": value}


def parse_tags(values: list[str]) -> list[str]:
	tags: list[str] = []
	for raw_value in values:
		for item in raw_value.split(","):
			tag = item.strip()
			if tag and tag not in tags:
				tags.append(tag)
	return tags


def parse_jsonish(value: str) -> Any:
	value = value.strip()
	if not value:
		return value
	try:
		return json.loads(value)
	except json.JSONDecodeError:
		return value


def parse_extra_field(raw_value: str) -> tuple[str, Any]:
	if "=" not in raw_value:
		raise RuntimeError("--field must be KEY=VALUE")
	key, value = raw_value.split("=", 1)
	key = key.strip()
	if not key:
		raise RuntimeError("--field key is empty")
	return key, parse_jsonish(value)


def add_optional_field(payload: dict[str, Any], field_key: str | None, value: Any) -> None:
	if value is None:
		return
	if field_key is None or not field_key.strip():
		return
	payload[field_key.strip()] = value


def build_payload(args: argparse.Namespace, env_values: dict[str, str]) -> dict[str, Any]:
	queue = args.queue.strip().upper()
	if QUEUE_RE.match(queue) is None:
		raise RuntimeError("unsupported queue format")

	summary = args.summary.strip()
	if not summary:
		raise RuntimeError("summary is empty")

	description = args.description_file.read_text(encoding="utf-8")
	if not description.strip():
		raise RuntimeError("description is empty")

	payload: dict[str, Any] = {
		"queue": queue,
		"summary": summary,
		"description": description,
	}

	tags = parse_tags(args.tag)
	if tags:
		payload["tags"] = tags

	if args.assignee:
		payload["assignee"] = parse_selector(args.assignee)

	project_field = args.project_field or env_values.get("TRACKER_PROJECT_FIELD") or "project"
	period_field = args.period_field or env_values.get("TRACKER_PERIOD_FIELD") or "period"
	sp_field = args.sp_field or env_values.get("TRACKER_SP_FIELD") or "storyPoints"

	add_optional_field(payload, project_field, args.project)
	add_optional_field(payload, period_field, args.period)
	if args.sp is not None:
		add_optional_field(payload, sp_field, args.sp)

	for raw_field in args.field:
		key, value = parse_extra_field(raw_field)
		payload[key] = value

	return payload


def main() -> int:
	parser = argparse.ArgumentParser(description="Create a Yandex Tracker issue.")
	parser.add_argument("--queue", required=True, help="Tracker queue key, e.g. IT.")
	parser.add_argument("--summary", required=True, help="Issue summary.")
	parser.add_argument("--description-file", required=True, type=Path, help="UTF-8 text file with issue description.")
	parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, type=Path, help="Path to tracker.env.")
	parser.add_argument("--project", help="Project value. Field key defaults to TRACKER_PROJECT_FIELD or project.")
	parser.add_argument("--project-field", help="Tracker field key for project.")
	parser.add_argument("--tag", action="append", default=[], help="Issue tag. Repeatable; comma-separated values are accepted.")
	parser.add_argument("--period", help="Period value. Field key defaults to TRACKER_PERIOD_FIELD or period.")
	parser.add_argument("--period-field", help="Tracker field key for period.")
	parser.add_argument("--sp", type=float, help="Story points. Field key defaults to TRACKER_SP_FIELD or storyPoints.")
	parser.add_argument("--sp-field", help="Tracker field key for story points.")
	parser.add_argument("--assignee", help="Assignee selector: login, login:<login>, id:<id>, uid:<uid>, passport:<id>, tracker:<id>, cloud:<id>.")
	parser.add_argument("--field", action="append", default=[], help="Extra Tracker field as KEY=VALUE. VALUE may be JSON.")
	parser.add_argument("--send", action="store_true", help="Actually create the issue. Default is dry-run.")
	args = parser.parse_args()

	env_values = load_env(args.env_file)
	base_url = env_values.get("TRACKER_BASE_URL", "https://api.tracker.yandex.net").rstrip("/")
	token = env_values.get("TRACKER_OAUTH_TOKEN", "")
	org_header, org_id = resolve_org(env_values)
	payload = build_payload(args, env_values)
	endpoint = f"{base_url}/v2/issues"

	if not args.send:
		print(json.dumps({
			"status": "DRY_RUN",
			"base_url": base_url,
			"endpoint": endpoint,
			"org_header": org_header,
			"payload": payload,
		}, ensure_ascii=False, indent=2))
		return 0

	if os.environ.get("CONFIRM_SEND") != "YES":
		raise RuntimeError("--send requires CONFIRM_SEND=YES")
	if not token:
		raise RuntimeError("TRACKER_OAUTH_TOKEN is empty")

	result = request_json(
		endpoint,
		token=token,
		org_id=org_id,
		org_header=org_header,
		method="POST",
		data=payload,
	)
	key = result.get("key") or result.get("id")
	self_url = result.get("self")
	print(json.dumps({
		"status": "CREATED",
		"key": key,
		"self": self_url,
		"display": result.get("display"),
	}, ensure_ascii=False, indent=2))
	return 0


if __name__ == "__main__":
	try:
		raise SystemExit(main())
	except Exception as exc:
		print(f"ERROR: {exc}", file=sys.stderr)
		raise SystemExit(1)
