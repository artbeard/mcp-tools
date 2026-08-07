"""Discover, load, and adapt `.workspace.yaml` manifests."""
from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

MANIFEST_NAME = ".workspace.yaml"
ALLOWED_WORK_BASES = {"master", "main"}
ALLOWED_TYPES = {"multi_repo", "monorepo"}


class ManifestError(ValueError):
	pass


def find_manifest(start: Path) -> Path:
	current = start.resolve()
	if current.is_file():
		current = current.parent
	for directory in (current, *current.parents):
		candidate = directory / MANIFEST_NAME
		if candidate.is_file():
			return candidate
	raise ManifestError(f"no {MANIFEST_NAME} found from {start.resolve()}")


def load_raw_manifest(path: Path) -> dict[str, Any]:
	data = yaml.safe_load(path.read_text(encoding="utf-8"))
	if not isinstance(data, dict):
		raise ManifestError(f"{path}: manifest must be a mapping")
	return data


def _require_str(value: Any, field: str) -> str:
	if not isinstance(value, str) or not value.strip():
		raise ManifestError(f"{field} must be a non-empty string")
	return value.strip()


def validate_manifest(data: dict[str, Any], *, root: Path) -> dict[str, Any]:
	version = data.get("schema_version", 1)
	if version != 1:
		raise ManifestError(f"unsupported schema_version: {version!r}")

	name = _require_str(data.get("name"), "name")
	kind = _require_str(data.get("type"), "type")
	if kind not in ALLOWED_TYPES:
		raise ManifestError(f"type must be one of {sorted(ALLOWED_TYPES)}")

	aliases = data.get("aliases", [])
	if not isinstance(aliases, list) or not all(isinstance(item, str) and item for item in aliases):
		raise ManifestError("aliases must be a string array")

	repositories = data.get("repositories")
	if not isinstance(repositories, list) or not repositories:
		raise ManifestError("repositories must be a non-empty array")
	if kind == "monorepo" and len(repositories) != 1:
		raise ManifestError("monorepo must contain exactly one repository")
	if kind == "multi_repo" and len(repositories) < 2:
		raise ManifestError("multi_repo must contain at least two repositories")

	git = data.get("git") or {}
	if git is None:
		git = {}
	if not isinstance(git, dict):
		raise ManifestError("git must be a mapping")
	protected_base = git.get("protected_base_branches", ["master", "main"])
	protected = git.get("protected_branches", ["master", "main", "dev"])
	if not isinstance(protected_base, list) or not all(isinstance(item, str) and item for item in protected_base):
		raise ManifestError("git.protected_base_branches must be a string array")
	if not isinstance(protected, list) or not all(isinstance(item, str) and item for item in protected):
		raise ManifestError("git.protected_branches must be a string array")

	seen_ids: set[str] = set()
	resolved_repos: list[dict[str, Any]] = []
	for index, repository in enumerate(repositories):
		if not isinstance(repository, dict):
			raise ManifestError(f"repositories[{index}] must be a mapping")
		repo_id = _require_str(repository.get("id") or repository.get("label"), f"repositories[{index}].id")
		if repo_id in seen_ids:
			raise ManifestError(f"duplicate repository id: {repo_id}")
		seen_ids.add(repo_id)
		rel_path = _require_str(repository.get("path"), f"repositories[{index}].path")
		abs_path = (root / rel_path).resolve()
		remote = _require_str(repository.get("remote", "origin"), f"repositories[{index}].remote")
		branches = repository.get("branches") or {}
		if not isinstance(branches, dict):
			raise ManifestError(f"repositories[{index}].branches must be a mapping")
		base = _require_str(branches.get("base", "master"), f"repositories[{index}].branches.base")
		mr_target = _require_str(branches.get("mr_target", base), f"repositories[{index}].branches.mr_target")
		dev = _require_str(branches.get("dev", "dev"), f"repositories[{index}].branches.dev")
		if base not in ALLOWED_WORK_BASES:
			raise ManifestError(f"{repo_id}: branches.base must be master or main, got {base}")
		if mr_target not in ALLOWED_WORK_BASES:
			raise ManifestError(f"{repo_id}: branches.mr_target must be master or main, got {mr_target}")
		if base not in protected_base:
			raise ManifestError(f"{repo_id}: branches.base must be listed in git.protected_base_branches")
		if mr_target not in protected:
			raise ManifestError(f"{repo_id}: branches.mr_target must be listed in git.protected_branches")
		if dev not in protected:
			raise ManifestError(f"{repo_id}: branches.dev must be listed in git.protected_branches")
		resolved_repos.append(
			{
				"id": repo_id,
				"label": repo_id,
				"path": str(abs_path),
				"rel_path": rel_path,
				"remote": remote,
				"base_branch": base,
				"target_branch": mr_target,
				"dev_branch": dev,
			}
		)

	sandbox = data.get("sandbox") or {}
	if sandbox is None:
		sandbox = {}
	if not isinstance(sandbox, dict):
		raise ManifestError("sandbox must be a mapping")
	components_raw = sandbox.get("components", [])
	if components_raw is None:
		components_raw = []
	if not isinstance(components_raw, list):
		raise ManifestError("sandbox.components must be an array")

	repo_by_id = {item["id"]: item for item in resolved_repos}
	components: list[dict[str, Any]] = []
	seen_component_ids: set[str] = set()
	for index, component in enumerate(components_raw):
		if not isinstance(component, dict):
			raise ManifestError(f"sandbox.components[{index}] must be a mapping")
		comp_id = _require_str(component.get("id") or component.get("component"), f"sandbox.components[{index}].id")
		if comp_id in seen_component_ids:
			raise ManifestError(f"duplicate sandbox component id: {comp_id}")
		seen_component_ids.add(comp_id)
		repo_ref = component.get("repository")
		directory_rel = component.get("directory", ".")
		if not isinstance(directory_rel, str) or not directory_rel:
			raise ManifestError(f"sandbox.components[{index}].directory must be a non-empty string")
		if repo_ref:
			repo_ref = _require_str(repo_ref, f"sandbox.components[{index}].repository")
			if repo_ref not in repo_by_id:
				raise ManifestError(f"sandbox.components[{index}].repository unknown: {repo_ref}")
			base_dir = Path(repo_by_id[repo_ref]["path"])
		else:
			base_dir = root
		abs_dir = (base_dir / directory_rel).resolve()
		runtime = _require_str(component.get("runtime"), f"sandbox.components[{index}].runtime")
		start_command = component.get("start_command")
		stop_command = component.get("stop_command")
		if not isinstance(start_command, list) or not start_command or not all(isinstance(x, str) for x in start_command):
			raise ManifestError(f"sandbox.components[{index}].start_command must be a non-empty string array")
		if not isinstance(stop_command, list) or not all(isinstance(x, str) for x in stop_command):
			raise ManifestError(f"sandbox.components[{index}].stop_command must be a string array")
		probe = component.get("probe")
		if not isinstance(probe, dict):
			raise ManifestError(f"sandbox.components[{index}].probe must be a mapping")
		timeout = component.get("timeout_seconds", 60)
		if not isinstance(timeout, int) or timeout <= 0:
			raise ManifestError(f"sandbox.components[{index}].timeout_seconds must be a positive int")

		item: dict[str, Any] = {
			"project": name,
			"component": comp_id,
			"directory": str(abs_dir),
			"runtime": runtime,
			"start_command": list(start_command),
			"stop_command": list(stop_command),
			"probe": probe,
			"timeout_seconds": timeout,
		}
		for optional in (
			"session",
			"compose_files",
			"compose_services",
			"required_docker_images",
		):
			if optional in component:
				item[optional] = component[optional]
		components.append(item)

	return {
		"schema_version": 1,
		"name": name,
		"type": kind,
		"aliases": list(aliases),
		"root": str(root.resolve()),
		"manifest_path": str((root / MANIFEST_NAME).resolve()) if (root / MANIFEST_NAME).is_file() else str(root),
		"git": {
			"protected_base_branches": list(protected_base),
			"protected_branches": list(protected),
		},
		"repositories": resolved_repos,
		"sandbox": {"components": components},
		"raw": data,
	}


def load_manifest(path: Path) -> dict[str, Any]:
	path = path.resolve()
	if path.name != MANIFEST_NAME and path.is_dir():
		path = path / MANIFEST_NAME
	if not path.is_file():
		raise ManifestError(f"manifest not found: {path}")
	root = path.parent
	return validate_manifest(load_raw_manifest(path), root=root)


def discover_manifest(
	*,
	cwd: Path | None = None,
	manifest: Path | None = None,
) -> dict[str, Any]:
	if manifest is not None:
		return load_manifest(Path(manifest))
	start = Path(cwd or Path.cwd())
	return load_manifest(find_manifest(start))


def assert_project_name(manifest: dict[str, Any], explicit: str | None) -> None:
	if not explicit:
		return
	allowed = {manifest["name"], *manifest.get("aliases", [])}
	if explicit not in allowed:
		raise ManifestError(
			f"project {explicit!r} does not match manifest name/aliases "
			f"{sorted(allowed)} ({manifest.get('manifest_path')})"
		)


def as_prepare_project(manifest: dict[str, Any]) -> dict[str, Any]:
	return {
		"key": manifest["name"],
		"type": manifest["type"],
		"workspace": manifest["root"],
		"repositories": [
			{
				"label": repo["label"],
				"path": repo["path"],
				"remote": repo["remote"],
				"base_branch": repo["base_branch"],
			}
			for repo in manifest["repositories"]
		],
	}


def as_mr_project(manifest: dict[str, Any]) -> dict[str, Any]:
	return {
		"key": manifest["name"],
		"type": manifest["type"],
		"workspace": manifest["root"],
		"repositories": [
			{
				"label": repo["label"],
				"path": repo["path"],
				"remote": repo["remote"],
				"target_branch": repo["target_branch"],
			}
			for repo in manifest["repositories"]
		],
	}


def as_deploy_project(manifest: dict[str, Any]) -> dict[str, Any]:
	return {
		"key": manifest["name"],
		"type": manifest["type"],
		"workspace": manifest["root"],
		"repositories": [
			{
				"label": repo["label"],
				"path": repo["path"],
				"remote": repo["remote"],
				"base_branch": repo["base_branch"],
				"dev_branch": repo["dev_branch"],
			}
			for repo in manifest["repositories"]
		],
	}


def list_payload(manifest: dict[str, Any]) -> dict[str, Any]:
	return {
		"status": "ok",
		"action": "list",
		"name": manifest["name"],
		"type": manifest["type"],
		"aliases": manifest.get("aliases", []),
		"root": manifest["root"],
		"manifest_path": manifest.get("manifest_path"),
		"repositories": [
			{
				"id": repo["id"],
				"path": repo["path"],
				"remote": repo["remote"],
				"branches": {
					"base": repo["base_branch"],
					"mr_target": repo["target_branch"],
					"dev": repo["dev_branch"],
				},
			}
			for repo in manifest["repositories"]
		],
		"sandbox_components": [item["component"] for item in manifest["sandbox"]["components"]],
	}
