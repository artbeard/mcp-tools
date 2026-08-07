#!/usr/bin/env python3
import argparse
from pathlib import Path
import json
import os
import sys
import urllib.error
import urllib.request


def _env_path() -> str:
    root = Path(__file__).resolve().parents[3]
    if str(root) not in sys.path:
        sys.path.insert(0, str(root))
    from mcp_paths import TRACKER_ENV
    return str(TRACKER_ENV)


ENV_PATH = _env_path()


def read_env(path: str) -> dict[str, str]:
    values: dict[str, str] = {}
    with open(path, "r", encoding="utf-8") as handle:
        for raw_line in handle:
            line = raw_line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def request(method: str, url: str, env: dict[str, str], payload: dict | None = None) -> dict | list:
    token = env.get("TRACKER_OAUTH_TOKEN", "")
    org_id = env.get("TRACKER_ORG_ID", "")
    cloud_org_id = env.get("TRACKER_CLOUD_ORG_ID", "")
    if not token:
        raise RuntimeError("TRACKER_OAUTH_TOKEN is empty")
    if not org_id and not cloud_org_id:
        raise RuntimeError("TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required")

    body = None if payload is None else json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Authorization", f"OAuth {token}")
    req.add_header("Content-Type", "application/json")
    if org_id:
        req.add_header("X-Org-ID", org_id)
    else:
        req.add_header("X-Cloud-Org-ID", cloud_org_id)

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            text = response.read().decode("utf-8")
            return json.loads(text) if text else {}
    except urllib.error.HTTPError as exc:
        text = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {exc.code}: {text}") from exc


def normalized_values(item: dict) -> set[str]:
    to = item.get("to") if isinstance(item.get("to"), dict) else {}
    raw = [
        item.get("id"),
        item.get("key"),
        item.get("display"),
        item.get("name"),
        to.get("id"),
        to.get("key"),
        to.get("display"),
    ]
    return {str(value).strip().lower() for value in raw if value not in (None, "")}


def target_aliases(target: str) -> set[str]:
    target = target.strip().lower()
    aliases = {target}
    if target in {"открыто", "открыт", "open", "opened"}:
        aliases.update({"открыто", "открыт", "open", "opened"})
    if target in {"в работе", "inprogress", "in_progress", "in progress"}:
        aliases.update({"в работе", "inprogress", "in_progress", "in progress"})
    return aliases


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--issue-key", required=True)
    parser.add_argument("--target", default="Открыто")
    parser.add_argument("--send", action="store_true")
    parser.add_argument("--confirm-send", action="store_true")
    args = parser.parse_args()

    env = read_env(ENV_PATH)
    base_url = env.get("TRACKER_BASE_URL", "https://api.tracker.yandex.net").rstrip("/")
    issue_url = f"{base_url}/v2/issues/{args.issue_key}"
    issue = request("GET", issue_url, env)
    if not isinstance(issue, dict):
        raise RuntimeError(f"Unexpected issue response: {issue!r}")

    current_status_value = issue.get("status")
    if isinstance(current_status_value, dict):
        current_status = {
            "id": current_status_value.get("id"),
            "key": current_status_value.get("key"),
            "display": current_status_value.get("display"),
        }
    else:
        current_status = current_status_value

    transitions_url = f"{base_url}/v2/issues/{args.issue_key}/transitions"
    transitions = request("GET", transitions_url, env)
    if not isinstance(transitions, list):
        raise RuntimeError(f"Unexpected transitions response: {transitions!r}")

    compact = [
        {
            "id": item.get("id"),
            "key": item.get("key"),
            "display": item.get("display"),
            "to": item.get("to"),
        }
        for item in transitions
    ]
    aliases = target_aliases(args.target)
    match = None
    for item in transitions:
        if normalized_values(item) & aliases:
            match = item
            break

    if match is None:
        print(json.dumps({
            "status": "NO_MATCH",
            "issue_key": args.issue_key,
            "current_status": current_status,
            "target": args.target,
            "transitions": compact,
        }, ensure_ascii=False, indent=2))
        return 2

    transition_id = match.get("id")
    if not transition_id:
        raise RuntimeError(f"Transition has no id: {match!r}")

    result = {
        "status": "DRY_RUN",
        "issue_key": args.issue_key,
        "current_status": current_status,
        "target": args.target,
        "transition": {
            "id": transition_id,
            "key": match.get("key"),
            "display": match.get("display"),
            "to": match.get("to"),
        },
        "all_transitions": compact,
    }
    if not args.send:
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0

    if not args.confirm_send and os.environ.get("CONFIRM_SEND") != "YES":
        raise RuntimeError("--send requires --confirm-send or CONFIRM_SEND=YES")

    execute_url = f"{transitions_url}/{transition_id}/_execute"
    sent = request("POST", execute_url, env, {})
    result["status"] = "POSTED"
    result["result"] = sent
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
