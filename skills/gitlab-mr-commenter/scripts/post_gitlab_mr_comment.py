#!/usr/bin/env python3
"""Post a GitLab MR comment or diff line discussion with dry-run by default."""

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
	from mcp_paths import GITLAB_ENV
	return GITLAB_ENV


DEFAULT_ENV_FILE = _default_env_file()
MR_URL_RE = re.compile(
	r"^(?P<base>https?://[^/]+)/(?P<project>.+?)/-/merge_requests/(?P<iid>\d+)(?:[/?#].*)?$"
)


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


def parse_mr_url(mr_url: str) -> tuple[str, str, int]:
	match = MR_URL_RE.match(mr_url.strip())
	if match is None:
		raise RuntimeError("unsupported MR URL format")

	base_url = match.group("base").rstrip("/")
	project_path = urllib.parse.unquote(match.group("project"))
	iid = int(match.group("iid"))

	return base_url, project_path, iid


def request_json(
	url: str,
	token: str,
	method: str = "GET",
	data: dict[str, str] | None = None,
	json_data: dict | None = None,
) -> dict | list:
	encoded_data = None
	headers = {"PRIVATE-TOKEN": token, "Accept": "application/json"}

	if data is not None and json_data is not None:
		raise RuntimeError("request_json accepts either data or json_data, not both")

	if json_data is not None:
		encoded_data = json.dumps(json_data).encode("utf-8")
		headers["Content-Type"] = "application/json"
	elif data is not None:
		encoded_data = urllib.parse.urlencode(data).encode("utf-8")
		headers["Content-Type"] = "application/x-www-form-urlencoded"

	request = urllib.request.Request(url, data=encoded_data, headers=headers, method=method)
	try:
		with urllib.request.urlopen(request, timeout=30) as response:
			body = response.read().decode("utf-8")
			return json.loads(body) if body else {}
	except urllib.error.HTTPError as error:
		body = error.read().decode("utf-8", errors="replace")
		raise RuntimeError(f"GitLab API HTTP {error.code}: {body}") from error


def find_diff_line(changes: dict, file_path: str, line: int, line_type: str) -> dict[str, str | int]:
	diff_text = None
	for change in changes.get("changes", []):
		if change.get("new_path") == file_path or change.get("old_path") == file_path:
			diff_text = change.get("diff") or ""
			break
	if diff_text is None:
		raise RuntimeError(f"file is missing from MR diff: {file_path}")

	old_line = 0
	new_line = 0
	hunk_re = re.compile(r"^@@ -(?P<old>\d+)(?:,\d+)? \+(?P<new>\d+)(?:,\d+)? @@")
	for raw_line in diff_text.splitlines():
		hunk_match = hunk_re.match(raw_line)
		if hunk_match:
			old_line = int(hunk_match.group("old"))
			new_line = int(hunk_match.group("new"))
			continue

		if raw_line.startswith("\\"):
			continue

		prefix = raw_line[:1]
		if prefix == "+":
			if line_type == "new" and new_line == line:
				return {"line": line, "line_type": line_type, "diff_marker": "added"}
			new_line += 1
			continue

		if prefix == "-":
			if line_type == "old" and old_line == line:
				return {"line": line, "line_type": line_type, "diff_marker": "removed"}
			old_line += 1
			continue

		if prefix == " ":
			if line_type == "new" and new_line == line:
				return {"line": line, "line_type": line_type, "diff_marker": "context"}
			if line_type == "old" and old_line == line:
				return {"line": line, "line_type": line_type, "diff_marker": "context"}
			old_line += 1
			new_line += 1

	raise RuntimeError(f"line is missing from MR diff: {file_path}:{line} ({line_type})")


def main() -> int:
	parser = argparse.ArgumentParser(description="Post a GitLab MR comment.")
	parser.add_argument("--mr-url", required=True, help="GitLab merge request URL.")
	parser.add_argument("--body-file", required=True, type=Path, help="UTF-8 text file with the final comment body.")
	parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, type=Path, help="Path to gitlab.env.")
	parser.add_argument("--line-comment", action="store_true", help="Create a diff line discussion instead of an MR-level note.")
	parser.add_argument("--file-path", help="Path in MR diff for --line-comment.")
	parser.add_argument("--line", type=int, help="Line number for --line-comment.")
	parser.add_argument("--line-type", choices=("new", "old"), default="new", help="Use new_line or old_line for --line-comment.")
	parser.add_argument("--send", action="store_true", help="Actually post the comment. Default is dry-run.")
	args = parser.parse_args()

	env_values = load_env(args.env_file)
	mr_base_url, project_path, iid = parse_mr_url(args.mr_url)
	base_url = env_values.get("GITLAB_BASE_URL", mr_base_url).rstrip("/")
	token = env_values.get("GITLAB_TOKEN", "")

	body = args.body_file.read_text(encoding="utf-8")
	if not body.strip():
		raise RuntimeError("comment body is empty")
	if args.line_comment and (not args.file_path or args.line is None):
		raise RuntimeError("--line-comment requires --file-path and --line")
	if not args.line_comment and (args.file_path or args.line is not None):
		raise RuntimeError("--file-path/--line require --line-comment")

	project_encoded = urllib.parse.quote(project_path, safe="")
	api_base = f"{base_url}/api/v4"
	project_url = f"{api_base}/projects/{project_encoded}"
	note_url = f"{project_url}/merge_requests/{iid}/notes"
	discussion_url = f"{project_url}/merge_requests/{iid}/discussions"

	if args.line_comment:
		if not token:
			raise RuntimeError("GITLAB_TOKEN is required for line-comment dry-run")
		mr = request_json(f"{project_url}/merge_requests/{iid}", token)
		diff_refs = mr.get("diff_refs") or {}
		base_sha = diff_refs.get("base_sha")
		start_sha = diff_refs.get("start_sha")
		head_sha = diff_refs.get("head_sha")
		if not base_sha or not start_sha or not head_sha:
			raise RuntimeError("MR diff_refs are missing; cannot create line discussion")
		changes = request_json(f"{project_url}/merge_requests/{iid}/changes", token)
		line_check = find_diff_line(changes, args.file_path, args.line, args.line_type)
		position = {
			"position_type": "text",
			"base_sha": base_sha,
			"start_sha": start_sha,
			"head_sha": head_sha,
			"old_path": args.file_path,
			"new_path": args.file_path,
		}
		if args.line_type == "new":
			position["new_line"] = args.line
		else:
			position["old_line"] = args.line

		if not args.send:
			print(json.dumps({
				"status": "DRY_RUN",
				"comment_type": "line_discussion",
				"base_url": base_url,
				"project_path": project_path,
				"merge_request_iid": iid,
				"file_path": args.file_path,
				"line": args.line,
				"line_type": args.line_type,
				"body_chars": len(body),
				"endpoint": discussion_url,
				"diff_refs_present": True,
				"line_check": line_check,
				"position": position,
			}, ensure_ascii=False, indent=2))
			return 0

		if os.environ.get("CONFIRM_SEND") != "YES":
			raise RuntimeError("--send requires CONFIRM_SEND=YES")
		result = request_json(
			discussion_url,
			token,
			method="POST",
			json_data={"body": body, "position": position},
		)
		notes = result.get("notes") or []
		first_note = notes[0] if notes else {}
		print(json.dumps({
			"status": "POSTED",
			"comment_type": "line_discussion",
			"discussion_id": result.get("id"),
			"id": first_note.get("id"),
			"url": first_note.get("url"),
			"created_at": first_note.get("created_at"),
		}, ensure_ascii=False, indent=2))
		return 0

	if not args.send:
		print(json.dumps({
			"status": "DRY_RUN",
			"comment_type": "mr_note",
			"base_url": base_url,
			"project_path": project_path,
			"merge_request_iid": iid,
			"body_chars": len(body),
			"endpoint": note_url,
		}, ensure_ascii=False, indent=2))
		return 0

	if os.environ.get("CONFIRM_SEND") != "YES":
		raise RuntimeError("--send requires CONFIRM_SEND=YES")
	if not token:
		raise RuntimeError("GITLAB_TOKEN is empty")

	result = request_json(note_url, token, method="POST", data={"body": body})
	print(json.dumps({
		"status": "POSTED",
		"comment_type": "mr_note",
		"id": result.get("id"),
		"url": result.get("url"),
		"created_at": result.get("created_at"),
	}, ensure_ascii=False, indent=2))
	return 0


if __name__ == "__main__":
	try:
		raise SystemExit(main())
	except Exception as exc:
		print(f"ERROR: {exc}", file=sys.stderr)
		raise SystemExit(1)
