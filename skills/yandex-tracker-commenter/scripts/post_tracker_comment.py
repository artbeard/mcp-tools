#!/usr/bin/env python3
"""Post a Yandex Tracker issue comment with dry-run by default."""

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


def _default_env_file() -> Path:
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import TRACKER_ENV
	return TRACKER_ENV


DEFAULT_ENV_FILE = _default_env_file()
ISSUE_KEY_RE = re.compile(r"^[A-Z][A-Z0-9]+-\d+$")
OBJECT_ID_RE = re.compile(r"^[0-9a-f]{24}$")


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


def request_json(
	url: str,
	token: str,
	org_id: str,
	org_header: str,
	method: str = "GET",
	data: dict[str, str] | None = None,
) -> dict:
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


def resolve_org(env_values: dict[str, str]) -> tuple[str, str]:
	cloud_org_id = env_values.get("TRACKER_CLOUD_ORG_ID", "")
	if cloud_org_id:
		return "X-Cloud-Org-ID", cloud_org_id

	org_id = env_values.get("TRACKER_ORG_ID", "")
	if org_id:
		return "X-Org-ID", org_id

	raise RuntimeError("TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required")


def parse_summonee(value: str) -> str | int | dict[str, str | int]:
	value = value.strip()
	if not value:
		raise RuntimeError("summonee value is empty")

	if ":" in value:
		kind, raw_identifier = value.split(":", 1)
		kind = kind.strip().lower()
		identifier = raw_identifier.strip()
		if not identifier:
			raise RuntimeError("summonee identifier is empty")
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
		raise RuntimeError(f"unsupported summonee selector: {kind}")

	if value.isdigit():
		return {"id": value}

	return {"login": value}


def resolve_reply_to(
	reply_to: str | None,
	base_url: str,
	token: str,
	org_id: str,
	org_header: str,
	issue_key: str,
) -> str | None:
	if reply_to is None:
		return None

	reply_to = reply_to.strip()
	if not reply_to:
		raise RuntimeError("reply-to-comment is empty")
	if OBJECT_ID_RE.match(reply_to):
		return reply_to
	if not reply_to.isdigit():
		raise RuntimeError("reply-to-comment must be a numeric comment id or 24-char longId")
	if not token:
		raise RuntimeError("TRACKER_OAUTH_TOKEN is required to resolve numeric reply-to-comment")

	issue_encoded = urllib.parse.quote(issue_key, safe="")
	comment_encoded = urllib.parse.quote(reply_to, safe="")
	comment_url = f"{base_url}/v2/issues/{issue_encoded}/comments/{comment_encoded}"
	comment = request_json(comment_url, token, org_id, org_header)
	long_id = comment.get("longId") or comment.get("long_id")
	if not isinstance(long_id, str) or OBJECT_ID_RE.match(long_id) is None:
		raise RuntimeError("Tracker comment longId is missing; cannot create reply")

	return long_id


def main() -> int:
	parser = argparse.ArgumentParser(description="Post a Yandex Tracker issue comment.")
	parser.add_argument("--issue-key", required=True, help="Tracker issue key, e.g. IT-123.")
	parser.add_argument("--body-file", required=True, type=Path, help="UTF-8 text file with the final comment body.")
	parser.add_argument(
		"--markup-type",
		choices=("md",),
		help="Tracker markupType for the comment body (use 'md' for YFM/Markdown).",
	)
	parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, type=Path, help="Path to tracker.env.")
	parser.add_argument(
		"--summonee",
		action="append",
		default=[],
		help=(
			"User to summon without @ in text. Repeatable. "
			"Formats: login:example.user, id:<tracker-user-id>, cloud:<cloudUid>, uid:<uid>, "
			"passport:<passportUid>, tracker:<trackerUid>. Plain non-numeric value is treated as login; plain digits as id."
		),
	)
	parser.add_argument(
		"--reply-to-comment",
		help="Reply to an existing comment. Accepts numeric comment id or 24-char Tracker longId.",
	)
	parser.add_argument("--send", action="store_true", help="Actually post the comment. Default is dry-run.")
	parser.add_argument("--confirm-send", action="store_true", help="Confirm the previously approved send operation.")
	args = parser.parse_args()

	issue_key = args.issue_key.strip().upper()
	if ISSUE_KEY_RE.match(issue_key) is None:
		raise RuntimeError("unsupported issue key format")

	body = args.body_file.read_text(encoding="utf-8")
	if not body.strip():
		raise RuntimeError("comment body is empty")

	env_values = load_env(args.env_file)
	base_url = env_values.get("TRACKER_BASE_URL", "https://api.tracker.yandex.net").rstrip("/")
	token = env_values.get("TRACKER_OAUTH_TOKEN", "")
	org_header, org_id = resolve_org(env_values)
	issue_encoded = urllib.parse.quote(issue_key, safe="")
	comment_url = f"{base_url}/v2/issues/{issue_encoded}/comments"
	summonees = [parse_summonee(value) for value in args.summonee]
	reply_to = resolve_reply_to(
		args.reply_to_comment,
		base_url=base_url,
		token=token,
		org_id=org_id,
		org_header=org_header,
		issue_key=issue_key,
	)

	if not args.send:
		print(json.dumps({
			"status": "DRY_RUN",
			"base_url": base_url,
			"issue_key": issue_key,
			"body_chars": len(body),
			"markup_type": args.markup_type,
			"summonees": summonees,
			"reply_to": reply_to,
			"endpoint": comment_url,
			"org_header": org_header,
		}, ensure_ascii=False, indent=2))
		return 0

	if not args.confirm_send and os.environ.get("CONFIRM_SEND") != "YES":
		raise RuntimeError("--send requires --confirm-send or CONFIRM_SEND=YES")
	if not token:
		raise RuntimeError("TRACKER_OAUTH_TOKEN is empty")

	result = request_json(
		comment_url,
		token=token,
		org_id=org_id,
		org_header=org_header,
		method="POST",
		data={
			"text": body,
			**({"markupType": args.markup_type} if args.markup_type else {}),
			**({"summonees": summonees} if summonees else {}),
			**({"replyTo": [reply_to]} if reply_to is not None else {}),
		},
	)
	comment_id = result.get("id") or result.get("self")
	print(json.dumps({
		"status": "POSTED",
		"issue_key": issue_key,
		"id": comment_id,
		"self": result.get("self"),
		"reply_to": reply_to,
		"summonees": result.get("summonees"),
		"created_at": result.get("createdAt") or result.get("created_at"),
	}, ensure_ascii=False, indent=2))
	return 0


if __name__ == "__main__":
	try:
		raise SystemExit(main())
	except Exception as exc:
		print(f"ERROR: {exc}", file=sys.stderr)
		raise SystemExit(1)
