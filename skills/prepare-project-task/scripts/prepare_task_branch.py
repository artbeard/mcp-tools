#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any


ALLOWED_WORK_BASES = {'master', 'main'}
FORBIDDEN_TASK_BRANCHES = {'master', 'main', 'dev'}


class SafetyError(RuntimeError):
	pass


def run(command: list[str], cwd: Path | None = None, timeout: int = 60) -> subprocess.CompletedProcess[str]:
	return subprocess.run(command, cwd=cwd, text=True, capture_output=True, timeout=timeout, check=False)


def git(repository: Path, *arguments: str, timeout: int = 60) -> subprocess.CompletedProcess[str]:
	if arguments and arguments[0] == 'push':
		raise SafetyError('git push is forbidden by prepare-project-task')
	return run(['git', '-C', str(repository), *arguments], timeout=timeout)


def emit(payload: dict[str, Any], exit_code: int) -> int:
	print(json.dumps(payload, ensure_ascii=False, indent=2))
	return exit_code


def command_error(result: subprocess.CompletedProcess[str]) -> str:
	return (result.stderr or result.stdout).strip() or f'command failed with exit code {result.returncode}'


def load_config(path: Path) -> tuple[list[dict[str, Any]], set[str]]:
	data = json.loads(path.read_text(encoding='utf-8'))
	projects = data.get('projects')
	protected = data.get('protected_base_branches')
	if not isinstance(projects, list) or not projects:
		raise ValueError('config must contain a non-empty projects array')
	if not isinstance(protected, list) or not all(isinstance(item, str) and item for item in protected):
		raise ValueError('protected_base_branches must be a non-empty string array')

	seen_keys: set[str] = set()
	seen_paths: set[Path] = set()
	for project in projects:
		missing = [field for field in ('key', 'type', 'workspace', 'repositories') if field not in project]
		if missing:
			raise ValueError(f"project config is missing fields: {', '.join(missing)}")
		key = project['key']
		if not isinstance(key, str) or not key or key in seen_keys:
			raise ValueError(f'invalid or duplicate project key: {key!r}')
		seen_keys.add(key)
		kind = project['type']
		repositories = project['repositories']
		if kind not in ('multi_repo', 'monorepo'):
			raise ValueError(f'{key}: unsupported project type: {kind}')
		if not isinstance(repositories, list) or not repositories:
			raise ValueError(f'{key}: repositories must be a non-empty array')
		if kind == 'monorepo' and len(repositories) != 1:
			raise ValueError(f'{key}: monorepo must contain exactly one repository')
		if kind == 'multi_repo' and len(repositories) < 2:
			raise ValueError(f'{key}: multi_repo must contain at least two repositories')
		for repository in repositories:
			missing_repo = [field for field in ('label', 'path', 'remote', 'base_branch') if field not in repository]
			if missing_repo:
				raise ValueError(f"{key}: repository is missing fields: {', '.join(missing_repo)}")
			repository_path = Path(repository['path']).resolve()
			if repository_path in seen_paths:
				raise ValueError(f'duplicate repository path: {repository_path}')
			seen_paths.add(repository_path)
			if repository['base_branch'] not in ALLOWED_WORK_BASES:
				raise ValueError(
					f"{key}/{repository['label']}: task branches may start only from master or main, "
					f"got: {repository['base_branch']}"
				)
			if repository['base_branch'] not in protected:
				raise ValueError(f"{key}/{repository['label']}: base branch must be protected: {repository['base_branch']}")
	return projects, set(protected)


def select_project(projects: list[dict[str, Any]], explicit: str | None, cwd: Path) -> dict[str, Any]:
	if explicit:
		for project in projects:
			if project['key'] == explicit:
				return project
		raise ValueError(f'unknown project: {explicit}')

	matches = []
	resolved_cwd = cwd.resolve()
	for project in projects:
		workspace = Path(project['workspace']).resolve()
		if resolved_cwd == workspace or workspace in resolved_cwd.parents:
			matches.append((len(workspace.parts), project))
	if not matches:
		raise ValueError(f'unable to detect project from directory: {resolved_cwd}')
	matches.sort(key=lambda item: item[0], reverse=True)
	if len(matches) > 1 and matches[0][0] == matches[1][0]:
		raise ValueError(f'ambiguous project for directory: {resolved_cwd}')
	return matches[0][1]


def validate_task(task_id: str, protected: set[str], project: dict[str, Any]) -> list[str]:
	errors = []
	valid = run(['git', 'check-ref-format', '--branch', task_id])
	if valid.returncode != 0:
		errors.append(f'invalid Git branch name: {task_id}')
	bases = {repository['base_branch'] for repository in project['repositories']}
	if task_id in FORBIDDEN_TASK_BRANCHES or task_id in protected or task_id in bases:
		errors.append(f'task branch cannot be a protected base branch: {task_id}')
	return errors


def operation_markers(repository: Path) -> list[str]:
	result = git(repository, 'rev-parse', '--absolute-git-dir')
	if result.returncode != 0:
		return ['UNKNOWN_GIT_DIR']
	git_dir = Path(result.stdout.strip())
	markers = ('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD', 'BISECT_LOG', 'rebase-apply', 'rebase-merge')
	return [marker for marker in markers if (git_dir / marker).exists()]


def preflight_repository(project_key: str, item: dict[str, Any], task_id: str) -> dict[str, Any]:
	repository = Path(item['path']).resolve()
	result: dict[str, Any] = {
		'label': item['label'],
		'path': str(repository),
		'base_branch': item['base_branch'],
		'remote': item['remote'],
		'ready': False,
		'errors': [],
	}
	errors: list[str] = result['errors']

	inside = git(repository, 'rev-parse', '--is-inside-work-tree')
	if inside.returncode != 0 or inside.stdout.strip() != 'true':
		errors.append('not a Git work tree')
		return result

	branch = git(repository, 'branch', '--show-current')
	current_branch = branch.stdout.strip() if branch.returncode == 0 else ''
	result['current_branch'] = current_branch or None
	if not current_branch:
		errors.append('detached HEAD is unsafe')

	status = git(repository, 'status', '--porcelain=v1', '--untracked-files=all')
	if status.returncode != 0:
		errors.append(command_error(status))
	elif status.stdout.strip():
		result['changes'] = status.stdout.splitlines()
		errors.append('repository has staged, unstaged, conflicted, or untracked changes')

	markers = operation_markers(repository)
	if markers:
		result['in_progress'] = markers
		errors.append('Git operation is in progress')

	remote = git(repository, 'remote', 'get-url', item['remote'])
	if remote.returncode != 0:
		errors.append(f"remote does not exist: {item['remote']}")

	base_ref = f"refs/heads/{item['base_branch']}"
	local_base = git(repository, 'show-ref', '--verify', '--quiet', base_ref)
	if local_base.returncode != 0:
		errors.append(f"local base branch does not exist: {item['base_branch']}")

	task_ref = f'refs/heads/{task_id}'
	local_task = git(repository, 'show-ref', '--verify', '--quiet', task_ref)
	if local_task.returncode == 0:
		errors.append(f'local task branch already exists: {task_id}')

	if remote.returncode == 0:
		remote_refs = git(
			repository,
			'ls-remote',
			'--heads',
			item['remote'],
			base_ref,
			f'refs/heads/{task_id}',
			timeout=90,
		)
		if remote_refs.returncode != 0:
			errors.append(f"cannot inspect remote {item['remote']}: {command_error(remote_refs)}")
		else:
			refs = {line.split('\t', 1)[1] for line in remote_refs.stdout.splitlines() if '\t' in line}
			if base_ref not in refs:
				errors.append(f"remote base branch does not exist: {item['remote']}/{item['base_branch']}")
			if f'refs/heads/{task_id}' in refs:
				errors.append(f"remote task branch already exists: {item['remote']}/{task_id}")

	result['ready'] = not errors
	return result


def preflight(project: dict[str, Any], task_id: str) -> list[dict[str, Any]]:
	return [preflight_repository(project['key'], item, task_id) for item in project['repositories']]


def fetch_and_verify(project: dict[str, Any]) -> tuple[list[dict[str, Any]], list[str]]:
	results = []
	errors = []
	for item in project['repositories']:
		repository = Path(item['path']).resolve()
		remote_ref = f"refs/remotes/{item['remote']}/{item['base_branch']}"
		refspec = f"refs/heads/{item['base_branch']}:{remote_ref}"
		fetched = git(repository, 'fetch', '--no-tags', item['remote'], refspec, timeout=120)
		entry = {'label': item['label'], 'path': str(repository), 'fetched': fetched.returncode == 0}
		if fetched.returncode != 0:
			message = command_error(fetched)
			entry['error'] = message
			errors.append(f"{item['label']}: fetch failed: {message}")
		results.append(entry)
	if errors:
		return results, errors

	for index, item in enumerate(project['repositories']):
		repository = Path(item['path']).resolve()
		remote_ref = f"refs/remotes/{item['remote']}/{item['base_branch']}"
		ancestor = git(repository, 'merge-base', '--is-ancestor', f"refs/heads/{item['base_branch']}", remote_ref)
		if ancestor.returncode != 0:
			message = f"local {item['base_branch']} cannot be fast-forwarded to {item['remote']}/{item['base_branch']}"
			results[index]['fast_forward_ready'] = False
			results[index]['error'] = message
			errors.append(f"{item['label']}: {message}")
		else:
			results[index]['fast_forward_ready'] = True
	return results, errors


def prepare(project: dict[str, Any], task_id: str) -> tuple[list[dict[str, Any]], list[str]]:
	results = []
	errors = []
	for item in project['repositories']:
		repository = Path(item['path']).resolve()
		entry: dict[str, Any] = {'label': item['label'], 'path': str(repository), 'branch': None}
		base = item['base_branch']
		remote_ref = f"refs/remotes/{item['remote']}/{base}"

		checked_out = git(repository, 'switch', base)
		if checked_out.returncode != 0:
			message = command_error(checked_out)
			entry['error'] = message
			errors.append(f"{item['label']}: checkout failed: {message}")
			results.append(entry)
			break

		updated = git(repository, 'merge', '--ff-only', remote_ref)
		if updated.returncode != 0:
			message = command_error(updated)
			entry['error'] = message
			errors.append(f"{item['label']}: fast-forward failed: {message}")
			results.append(entry)
			break

		created = git(repository, 'switch', '-c', task_id, base)
		if created.returncode != 0:
			message = command_error(created)
			entry['error'] = message
			errors.append(f"{item['label']}: branch creation failed: {message}")
			results.append(entry)
			break

		entry['branch'] = task_id
		entry['base_branch'] = base
		entry['published'] = False
		results.append(entry)
	return results, errors


def main() -> int:
	parser = argparse.ArgumentParser()
	action = parser.add_mutually_exclusive_group(required=True)
	action.add_argument('--list', action='store_true')
	action.add_argument('--check', metavar='TASK_ID')
	action.add_argument('--prepare', metavar='TASK_ID')
	parser.add_argument('--project')
	parser.add_argument('--cwd', default=os.getcwd())
	args = parser.parse_args()
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import PROJECTS_PREPARE
	config_path = Path(os.environ.get('PREPARE_PROJECT_CONFIG', PROJECTS_PREPARE))

	try:
		projects, protected = load_config(config_path)
		if args.list:
			return emit({
				'status': 'ok',
				'action': 'list',
				'projects': [
					{
						'key': item['key'],
						'type': item['type'],
						'workspace': item['workspace'],
						'repositories': len(item['repositories']),
					}
					for item in projects
				],
				'push_allowed': False,
			}, 0)
		project = select_project(projects, args.project, Path(args.cwd))
		task_id = args.check or args.prepare
		assert task_id is not None
		task_errors = validate_task(task_id, protected, project)
	except (OSError, ValueError, json.JSONDecodeError, SafetyError) as error:
		return emit({'status': 'failed', 'errors': [str(error)], 'push_allowed': False}, 2)

	if task_errors:
		return emit({
			'status': 'failed',
			'action': 'check' if args.check else 'prepare',
			'project': project['key'],
			'task_id': task_id,
			'errors': task_errors,
			'push_allowed': False,
		}, 2)

	checks = preflight(project, task_id)
	check_errors = [f"{item['label']}: {error}" for item in checks for error in item['errors']]
	if check_errors or args.check:
		return emit({
			'status': 'failed' if check_errors else 'ok',
			'action': 'check',
			'project': project['key'],
			'project_type': project['type'],
			'task_id': task_id,
			'repositories': checks,
			'errors': check_errors,
			'push_allowed': False,
		}, 1 if check_errors else 0)

	fetches, fetch_errors = fetch_and_verify(project)
	if fetch_errors:
		return emit({
			'status': 'failed',
			'action': 'prepare',
			'phase': 'fetch_and_verify',
			'project': project['key'],
			'project_type': project['type'],
			'task_id': task_id,
			'repositories': fetches,
			'errors': fetch_errors,
			'working_trees_changed': False,
			'push_allowed': False,
		}, 1)

	prepared, prepare_errors = prepare(project, task_id)
	return emit({
		'status': 'failed' if prepare_errors else 'ok',
		'action': 'prepare',
		'phase': 'create_branches',
		'project': project['key'],
		'project_type': project['type'],
		'task_id': task_id,
		'repositories': prepared,
		'errors': prepare_errors,
		'push_performed': False,
		'push_allowed': False,
	}, 3 if prepare_errors else 0)


if __name__ == '__main__':
	try:
		sys.exit(main())
	except (subprocess.TimeoutExpired, SafetyError) as error:
		sys.exit(emit({'status': 'failed', 'errors': [str(error)], 'push_allowed': False}, 2))
