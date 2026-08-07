#!/usr/bin/env python3
"""Generate `.workspace.yaml` files from legacy central JSON configs in config/."""
from __future__ import annotations

import argparse
import json
import os
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any

import yaml

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
	sys.path.insert(0, str(ROOT))

from mcp_paths import (  # noqa: E402
	LEGACY_PROJECTS_DEPLOY,
	LEGACY_PROJECTS_MR,
	LEGACY_PROJECTS_PREPARE,
	LEGACY_SANDBOXES,
)


def load_json(path: Path) -> dict[str, Any]:
	if not path.is_file():
		raise FileNotFoundError(path)
	return json.loads(path.read_text(encoding="utf-8"))


def rel_to(workspace: Path, absolute: str | Path) -> str:
	target = Path(absolute).resolve()
	relative = os.path.relpath(str(target), str(workspace.resolve()))
	return "." if relative in ("", ".") else relative.replace("\\", "/")


def under(workspace: Path, absolute: Path) -> bool:
	try:
		absolute.resolve().relative_to(workspace.resolve())
		return True
	except ValueError:
		return False


def merge_repos(
	prepare_project: dict[str, Any],
	mr_by_key: dict[str, dict[str, Any]],
	deploy_by_key: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
	key = prepare_project["key"]
	mr_repos = {item["label"]: item for item in (mr_by_key.get(key) or {}).get("repositories", [])}
	deploy_repos = {item["label"]: item for item in (deploy_by_key.get(key) or {}).get("repositories", [])}
	workspace = Path(prepare_project["workspace"]).resolve()
	repos: list[dict[str, Any]] = []
	for item in prepare_project["repositories"]:
		label = item["label"]
		mr = mr_repos.get(label, {})
		deploy = deploy_repos.get(label, {})
		repos.append(
			{
				"id": label,
				"path": rel_to(workspace, item["path"]),
				"remote": item.get("remote", "origin"),
				"branches": {
					"base": item.get("base_branch", "master"),
					"mr_target": mr.get("target_branch", item.get("base_branch", "master")),
					"dev": deploy.get("dev_branch", "dev"),
				},
			}
		)
	return repos


def sandbox_components_for(
	workspace: Path,
	components: list[dict[str, Any]],
	repo_paths: dict[str, Path],
) -> list[dict[str, Any]]:
	result: list[dict[str, Any]] = []
	for component in components:
		directory = Path(component["directory"]).resolve()
		if not under(workspace, directory):
			continue
		repo_id = None
		repo_base = None
		for label, path in sorted(repo_paths.items(), key=lambda item: len(str(item[1])), reverse=True):
			if under(path, directory):
				repo_id = label
				repo_base = path
				break
		entry: dict[str, Any] = {
			"id": component["component"],
			"runtime": component["runtime"],
			"start_command": component["start_command"],
			"stop_command": component.get("stop_command", []),
			"probe": component["probe"],
			"timeout_seconds": component.get("timeout_seconds", 60),
		}
		if repo_id and repo_base is not None:
			entry["repository"] = repo_id
			entry["directory"] = rel_to(repo_base, directory)
		else:
			entry["directory"] = rel_to(workspace, directory)
		for optional in ("session", "compose_files", "compose_services", "required_docker_images"):
			if optional in component:
				entry[optional] = component[optional]
		result.append(entry)
	return result


def aliases_map(sandboxes: dict[str, Any]) -> dict[str, list[str]]:
	result: dict[str, list[str]] = defaultdict(list)
	for definition in sandboxes.get("projects", []):
		key = definition.get("key")
		if not isinstance(key, str):
			continue
		for source in definition.get("sources") or []:
			if isinstance(source, str) and source != key:
				result[source].append(key)
		for alias in definition.get("aliases") or []:
			if isinstance(alias, str):
				result[key].append(alias)
	return result


def build_manifests() -> dict[Path, dict[str, Any]]:
	prepare = load_json(LEGACY_PROJECTS_PREPARE)
	mr = load_json(LEGACY_PROJECTS_MR) if LEGACY_PROJECTS_MR.is_file() else {"projects": []}
	deploy = load_json(LEGACY_PROJECTS_DEPLOY) if LEGACY_PROJECTS_DEPLOY.is_file() else {"projects": []}
	sandboxes = load_json(LEGACY_SANDBOXES) if LEGACY_SANDBOXES.is_file() else {"components": [], "projects": []}

	mr_by_key = {item["key"]: item for item in mr.get("projects", [])}
	deploy_by_key = {item["key"]: item for item in deploy.get("projects", [])}
	components = list(sandboxes.get("components", []))
	alias_map = aliases_map(sandboxes)

	manifests: dict[Path, dict[str, Any]] = {}
	for project in prepare.get("projects", []):
		workspace = Path(project["workspace"]).resolve()
		repos = merge_repos(project, mr_by_key, deploy_by_key)
		repo_paths = {item["id"]: (workspace / item["path"]).resolve() for item in repos}
		manifests[workspace] = {
			"schema_version": 1,
			"name": project["key"],
			"type": project["type"],
			"aliases": sorted(set(alias_map.get(project["key"], []))),
			"repositories": repos,
			"git": {
				"protected_base_branches": prepare.get("protected_base_branches", ["master", "main"]),
				"protected_branches": deploy.get("protected_branches")
				or mr.get("protected_branches")
				or ["master", "main", "dev"],
			},
			"sandbox": {
				"components": sandbox_components_for(workspace, components, repo_paths),
			},
		}

	# Sandbox-only workspaces (not covered by prepare projects.json)
	covered = list(manifests.keys())
	remaining = []
	for component in components:
		directory = Path(component["directory"]).resolve()
		if any(under(workspace, directory) for workspace in covered):
			continue
		remaining.append(component)

	by_project: dict[str, list[dict[str, Any]]] = defaultdict(list)
	for component in remaining:
		by_project[component["project"]].append(component)

	for project_key, project_components in sorted(by_project.items()):
		dirs = [Path(item["directory"]).resolve() for item in project_components]
		common = Path(os.path.commonpath([str(path) for path in dirs])).resolve()
		if common in manifests:
			# merge sandbox into existing
			existing = manifests[common]
			repo_paths = {
				item["id"]: (common / item["path"]).resolve() if item["path"] != "." else common
				for item in existing["repositories"]
			}
			extra = sandbox_components_for(common, project_components, repo_paths)
			seen = {item["id"] for item in existing["sandbox"]["components"]}
			for item in extra:
				if item["id"] not in seen:
					existing["sandbox"]["components"].append(item)
			continue

		child_names = sorted(
			{
				rel_to(common, Path(item["directory"]).resolve()).split("/", 1)[0]
				for item in project_components
			}
		)
		child_names = [name for name in child_names if name not in (".", "")]
		if len(child_names) >= 2:
			kind = "multi_repo"
			repos = [
				{
					"id": name,
					"path": name,
					"remote": "origin",
					"branches": {"base": "master", "mr_target": "master", "dev": "dev"},
				}
				for name in child_names
			]
			repo_paths = {item["id"]: (common / item["path"]).resolve() for item in repos}
		else:
			kind = "monorepo"
			repos = [
				{
					"id": "monorepo",
					"path": ".",
					"remote": "origin",
					"branches": {"base": "master", "mr_target": "master", "dev": "dev"},
				}
			]
			repo_paths = {"monorepo": common}

		manifests[common] = {
			"schema_version": 1,
			"name": project_key,
			"type": kind,
			"aliases": sorted(set(alias_map.get(project_key, []))),
			"repositories": repos,
			"git": {
				"protected_base_branches": ["master", "main"],
				"protected_branches": ["master", "main", "dev"],
			},
			"sandbox": {
				"components": sandbox_components_for(common, project_components, repo_paths),
			},
		}

	return manifests


def main() -> int:
	parser = argparse.ArgumentParser(description=__doc__)
	parser.add_argument("--write", action="store_true", help="Write .workspace.yaml files")
	parser.add_argument("--force", action="store_true", help="Overwrite existing manifests")
	args = parser.parse_args()
	try:
		manifests = build_manifests()
	except FileNotFoundError as error:
		print(f"ERROR: missing legacy config: {error}", file=sys.stderr)
		print("Expected config/projects-prepare.json (+ optional mr/deploy/sandboxes).", file=sys.stderr)
		return 2

	failures = 0
	for workspace, manifest in sorted(manifests.items(), key=lambda item: str(item[0])):
		target = workspace / ".workspace.yaml"
		print(
			f"{workspace} -> {target} "
			f"({manifest['name']}, repos={len(manifest['repositories'])}, "
			f"sandbox={len(manifest['sandbox']['components'])})"
		)
		if not args.write:
			continue
		if target.exists() and not args.force:
			print(f"  skip existing (use --force): {target}")
			continue
		try:
			workspace.mkdir(parents=True, exist_ok=True)
			target.write_text(yaml.safe_dump(manifest, sort_keys=False, allow_unicode=True), encoding="utf-8")
			print(f"  wrote {target}")
		except OSError as error:
			print(f"  ERROR writing {target}: {error}", file=sys.stderr)
			failures += 1
			continue
	if not args.write:
		print("Dry-run only. Re-run with --write to create files.")
	return 1 if failures else 0


if __name__ == "__main__":
	raise SystemExit(main())
