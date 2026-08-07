#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any


ALLOWED_WORK_BASES = {'master', 'main'}
FORBIDDEN_TASK_BRANCHES = {'master', 'main', 'dev'}


class SafetyError(RuntimeError):
	pass


def run(command: list[str], cwd: Path | None = None, timeout: int = 120) -> subprocess.CompletedProcess[str]:
	return subprocess.run(command, cwd=cwd, text=True, capture_output=True, timeout=timeout, check=False)


def git(repository: Path, *arguments: str, timeout: int = 120) -> subprocess.CompletedProcess[str]:
	if arguments and arguments[0] == 'push':
		raise SafetyError('use push_dev; generic git push is forbidden')
	if arguments and arguments[0] in ('reset', 'stash', 'clean', 'rebase'):
		raise SafetyError(f'forbidden Git operation: {arguments[0]}')
	return run(['git', '-C', str(repository), *arguments], timeout=timeout)


def push_dev(repository: Path, remote: str, dev_branch: str) -> subprocess.CompletedProcess[str]:
	if dev_branch != 'dev':
		raise SafetyError(f'only dev may be pushed, got: {dev_branch}')
	return run(['git', '-C', str(repository), 'push', remote, 'refs/heads/dev:refs/heads/dev'], timeout=180)


def emit(payload: dict[str, Any], code: int) -> int:
	print(json.dumps(payload, ensure_ascii=False, indent=2))
	return code


def error_text(result: subprocess.CompletedProcess[str]) -> str:
	return (result.stderr or result.stdout).strip() or f'command failed with exit code {result.returncode}'


def load_config(path: Path) -> tuple[list[dict[str, Any]], set[str]]:
	data = json.loads(path.read_text(encoding='utf-8'))
	projects, protected = data.get('projects'), data.get('protected_branches')
	if not isinstance(projects, list) or not projects:
		raise ValueError('projects must be a non-empty array')
	if not isinstance(protected, list) or not protected:
		raise ValueError('protected_branches must be a non-empty array')
	seen_keys, seen_paths = set(), set()
	for project in projects:
		for field in ('key', 'type', 'workspace', 'repositories'):
			if field not in project:
				raise ValueError(f'project missing field: {field}')
		if project['key'] in seen_keys:
			raise ValueError(f"duplicate project key: {project['key']}")
		seen_keys.add(project['key'])
		repositories = project['repositories']
		if project['type'] == 'monorepo' and len(repositories) != 1:
			raise ValueError(f"{project['key']}: monorepo must contain one repository")
		if project['type'] == 'multi_repo' and len(repositories) < 2:
			raise ValueError(f"{project['key']}: multi_repo must contain at least two repositories")
		if project['type'] not in ('monorepo', 'multi_repo'):
			raise ValueError(f"{project['key']}: unsupported project type")
		for item in repositories:
			for field in ('label', 'path', 'remote', 'base_branch', 'dev_branch'):
				if field not in item:
					raise ValueError(f"{project['key']}: repository missing field: {field}")
			path_value = str(Path(item['path']).resolve())
			if path_value in seen_paths:
				raise ValueError(f'duplicate repository path: {path_value}')
			seen_paths.add(path_value)
			if item['base_branch'] not in ALLOWED_WORK_BASES:
				raise ValueError(
					f"{project['key']}/{item['label']}: task base must be master or main, "
					f"got: {item['base_branch']}"
				)
			if item['dev_branch'] != 'dev':
				raise ValueError(f"{project['key']}/{item['label']}: only dev is an allowed deployment branch")
	return projects, set(protected)


def select_project(projects: list[dict[str, Any]], explicit: str | None, cwd: Path) -> dict[str, Any]:
	if explicit:
		matches = [item for item in projects if item['key'] == explicit]
		if not matches:
			raise ValueError(f'unknown project: {explicit}')
		return matches[0]
	resolved = cwd.resolve()
	matches = [(len(Path(item['workspace']).resolve().parts), item) for item in projects if resolved == Path(item['workspace']).resolve() or Path(item['workspace']).resolve() in resolved.parents]
	if not matches:
		raise ValueError(f'unable to detect project from directory: {resolved}')
	return sorted(matches, key=lambda item: item[0], reverse=True)[0][1]


def git_dir(repository: Path) -> Path | None:
	result = git(repository, 'rev-parse', '--absolute-git-dir')
	return Path(result.stdout.strip()) if result.returncode == 0 else None


def merge_active(repository: Path) -> bool:
	directory = git_dir(repository)
	return bool(directory and (directory / 'MERGE_HEAD').exists())


def other_operations(repository: Path) -> list[str]:
	directory = git_dir(repository)
	if not directory:
		return ['UNKNOWN_GIT_DIR']
	return [name for name in ('CHERRY_PICK_HEAD', 'REVERT_HEAD', 'BISECT_LOG', 'rebase-apply', 'rebase-merge') if (directory / name).exists()]


def static_preflight(project: dict[str, Any], task_id: str, allow_merge: bool) -> tuple[list[dict[str, Any]], list[str]]:
	results, errors = [], []
	active_count = 0
	for item in project['repositories']:
		repository = Path(item['path']).resolve()
		entry: dict[str, Any] = {'label': item['label'], 'path': str(repository), 'ready': False, 'errors': []}
		inside = git(repository, 'rev-parse', '--is-inside-work-tree')
		if inside.returncode != 0:
			entry['errors'].append('not a Git work tree')
			results.append(entry)
			continue
		branch = git(repository, 'branch', '--show-current').stdout.strip()
		entry['current_branch'] = branch or None
		if not branch:
			entry['errors'].append('detached HEAD is unsafe')
		active = merge_active(repository)
		entry['merge_active'] = active
		if active:
			active_count += 1
			if not allow_merge:
				entry['errors'].append('merge is already in progress; use --continue')
			if branch != item['dev_branch']:
				entry['errors'].append('active merge is not on dev')
		operations = other_operations(repository)
		if operations:
			entry['in_progress'] = operations
			entry['errors'].append('unsupported Git operation is in progress')
		status = git(repository, 'status', '--porcelain=v1', '--untracked-files=all')
		if status.returncode != 0:
			entry['errors'].append(error_text(status))
		elif status.stdout.strip() and not (allow_merge and active):
			entry['changes'] = status.stdout.splitlines()
			entry['errors'].append('repository is not clean')
		entry['ready'] = not entry['errors']
		results.append(entry)
		errors.extend(f"{item['label']}: {message}" for message in entry['errors'])
	if active_count > 1:
		errors.append('more than one repository has an active merge')
	return results, errors


def refresh_refs(project: dict[str, Any], task_id: str) -> tuple[list[dict[str, Any]], list[str]]:
	results, errors = [], []
	for item in project['repositories']:
		repository = Path(item['path']).resolve()
		remote = item['remote']
		dev, base = item['dev_branch'], item['base_branch']
		remote_refs = git(
			repository,
			'ls-remote',
			'--heads',
			remote,
			f'refs/heads/{base}',
			f'refs/heads/{dev}',
			f'refs/heads/{task_id}',
			timeout=120,
		)
		entry: dict[str, Any] = {'label': item['label'], 'path': str(repository), 'fetched': False}
		if remote_refs.returncode != 0:
			message = error_text(remote_refs)
			entry['error'] = message
			errors.append(f"{item['label']}: cannot inspect remote: {message}")
			results.append(entry)
			continue
		refs = {line.split('\t', 1)[1]: line.split('\t', 1)[0] for line in remote_refs.stdout.splitlines() if '\t' in line}
		base_ref, dev_ref, task_ref = f'refs/heads/{base}', f'refs/heads/{dev}', f'refs/heads/{task_id}'
		if base_ref not in refs:
			errors.append(f"{item['label']}: remote base branch does not exist: {base}")
			entry['error'] = f'remote base branch does not exist: {base}'
			results.append(entry)
			continue
		if dev_ref not in refs:
			errors.append(f"{item['label']}: remote dev branch does not exist")
			entry['error'] = 'remote dev branch does not exist'
			results.append(entry)
			continue
		if task_ref not in refs:
			errors.append(f"{item['label']}: remote task branch does not exist: {task_id}")
			entry['error'] = f'remote task branch does not exist: {task_id}'
			results.append(entry)
			continue
		entry['observed_remote_dev_sha'] = refs[dev_ref]
		entry['remote_base_sha'] = refs[base_ref]
		entry['remote_task_sha'] = refs[task_ref]
		fetched = git(
			repository,
			'fetch',
			'--no-tags',
			remote,
			f'{base_ref}:refs/remotes/{remote}/{base}',
			f'{dev_ref}:refs/remotes/{remote}/{dev}',
			f'{task_ref}:refs/remotes/{remote}/{task_id}',
			timeout=180,
		)
		if fetched.returncode != 0:
			message = error_text(fetched)
			entry['error'] = message
			errors.append(f"{item['label']}: fetch failed: {message}")
		else:
			entry['fetched'] = True
		results.append(entry)
	return results, errors


def conflict_files_from_merge_tree(output: str) -> list[str]:
	files = []
	for line in output.splitlines():
		value = line.strip()
		if not value or re.fullmatch(r'[0-9a-f]{40,64}', value) or value.startswith('CONFLICT ') or value.startswith('Auto-merging '):
			continue
		if '\t' not in value and not value.startswith(('100644 ', '100755 ', '160000 ')):
			files.append(value)
	return sorted(set(files))


def task_history_guard(repository: Path, base_ref: str, task_ref: str) -> tuple[dict[str, Any], list[str]]:
	result: dict[str, Any] = {'base_ref': base_ref, 'task_ref': task_ref}
	errors: list[str] = []
	merge_base = git(repository, 'merge-base', base_ref, task_ref)
	if merge_base.returncode != 0 or not merge_base.stdout.strip():
		errors.append('task branch does not share valid history with remote master/main base')
		return result, errors
	result['branch_point'] = merge_base.stdout.strip()
	merges = git(repository, 'rev-list', '--merges', f'{base_ref}..{task_ref}')
	if merges.returncode != 0:
		errors.append(f'cannot inspect task merge history: {error_text(merges)}')
		return result, errors
	unexpected_merges = merges.stdout.splitlines()
	result['unexpected_merge_commits'] = unexpected_merges
	if unexpected_merges:
		errors.append(
			'task branch contains merge commits outside remote master/main; '
			'possible dev contamination, reconstruct the branch from the current base'
		)
	return result, errors


def integration_plan(project: dict[str, Any], task_id: str, fetches: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], list[str]]:
	results, errors = [], []
	for item, fetched in zip(project['repositories'], fetches):
		repository = Path(item['path']).resolve()
		dev, base, remote = item['dev_branch'], item['base_branch'], item['remote']
		dev_ref = f'refs/heads/{dev}'
		remote_base = f'refs/remotes/{remote}/{base}'
		remote_dev = f'refs/remotes/{remote}/{dev}'
		remote_task = f'refs/remotes/{remote}/{task_id}'
		entry: dict[str, Any] = {'label': item['label'], 'path': str(repository), 'mergeable': False, 'conflict_files': []}
		history, history_errors = task_history_guard(repository, remote_base, remote_task)
		entry['history_guard'] = history
		if history_errors:
			entry['error'] = history_errors[0]
			errors.extend(f"{item['label']}: {message}" for message in history_errors)
			results.append(entry)
			continue
		local_dev = git(repository, 'show-ref', '--verify', '--hash', dev_ref)
		if local_dev.returncode != 0:
			entry['error'] = 'local dev branch does not exist'
			errors.append(f"{item['label']}: local dev branch does not exist")
			results.append(entry)
			continue
		remote_contains_task = git(repository, 'merge-base', '--is-ancestor', remote_task, remote_dev).returncode == 0
		if remote_contains_task:
			entry.update({'mergeable': True, 'already_deployed': True, 'observed_remote_dev_sha': fetched['observed_remote_dev_sha']})
			results.append(entry)
			continue
		local_contains_task = git(repository, 'merge-base', '--is-ancestor', remote_task, dev_ref).returncode == 0
		remote_is_ancestor = git(repository, 'merge-base', '--is-ancestor', remote_dev, dev_ref).returncode == 0
		local_is_ancestor = git(repository, 'merge-base', '--is-ancestor', dev_ref, remote_dev).returncode == 0
		if local_contains_task and remote_is_ancestor:
			entry.update({'mergeable': True, 'prepared_local': True, 'observed_remote_dev_sha': fetched['observed_remote_dev_sha']})
			results.append(entry)
			continue
		if not local_is_ancestor:
			entry['error'] = 'local dev has commits not present in remote dev and does not contain the task merge'
			errors.append(f"{item['label']}: {entry['error']}")
			results.append(entry)
			continue
		merge_tree = git(repository, 'merge-tree', '--write-tree', '--name-only', '--messages', remote_dev, remote_task)
		entry['observed_remote_dev_sha'] = fetched['observed_remote_dev_sha']
		if merge_tree.returncode == 0:
			entry['mergeable'] = True
		else:
			entry['needs_resolution'] = True
			entry['conflict_files'] = conflict_files_from_merge_tree(merge_tree.stdout)
			entry['diagnostics'] = (merge_tree.stdout + merge_tree.stderr)[-8000:]
		results.append(entry)
	return results, errors


def resolution_payload(project: dict[str, Any], task_id: str, item: dict[str, Any], diagnostics: str = '') -> dict[str, Any]:
	repository = Path(item['path']).resolve()
	conflicts = git(repository, 'diff', '--name-only', '--diff-filter=U').stdout.splitlines()
	unstaged = git(repository, 'diff', '--name-only').stdout.splitlines()
	return {
		'status': 'needs_resolution',
		'action': 'deploy',
		'project': project['key'],
		'project_type': project['type'],
		'task_id': task_id,
		'active_resolution': {
			'label': item['label'],
			'path': str(repository),
			'branch': item['dev_branch'],
			'conflict_files': conflicts,
			'unstaged_files': unstaged,
			'next_command': f'--continue {task_id}',
		},
		'diagnostics': diagnostics[-8000:],
		'push_started': False,
	}


def begin_conflict(project: dict[str, Any], task_id: str, item: dict[str, Any]) -> tuple[dict[str, Any], int]:
	repository = Path(item['path']).resolve()
	dev, remote = item['dev_branch'], item['remote']
	checked = git(repository, 'switch', dev)
	if checked.returncode != 0:
		return {'status': 'failed', 'phase': 'begin_resolution', 'errors': [f"{item['label']}: {error_text(checked)}"]}, 3
	updated = git(repository, 'merge', '--ff-only', f'refs/remotes/{remote}/{dev}')
	if updated.returncode != 0:
		return {'status': 'failed', 'phase': 'begin_resolution', 'errors': [f"{item['label']}: {error_text(updated)}"]}, 3
	merged = git(repository, 'merge', '--no-ff', '--no-commit', f'refs/remotes/{remote}/{task_id}')
	if merged.returncode != 0 and merge_active(repository):
		return resolution_payload(project, task_id, item, merged.stdout + merged.stderr), 4
	if merged.returncode != 0:
		return {'status': 'failed', 'phase': 'begin_resolution', 'errors': [f"{item['label']}: {error_text(merged)}"]}, 3
	committed = git(repository, 'commit', '-m', f"Merge branch '{task_id}' into dev")
	if committed.returncode != 0:
		return {'status': 'failed', 'phase': 'commit_merge', 'errors': [f"{item['label']}: {error_text(committed)}"]}, 3
	return {'status': 'local_merge_created'}, 0


def continue_active_merge(project: dict[str, Any], task_id: str) -> tuple[dict[str, Any] | None, int]:
	active = [item for item in project['repositories'] if merge_active(Path(item['path']).resolve())]
	if not active:
		return None, 0
	if len(active) != 1:
		return {'status': 'failed', 'phase': 'continue_resolution', 'errors': ['more than one active merge']}, 3
	item = active[0]
	repository = Path(item['path']).resolve()
	conflicts = git(repository, 'diff', '--name-only', '--diff-filter=U').stdout.splitlines()
	unstaged = git(repository, 'diff', '--name-only').stdout.splitlines()
	if conflicts or unstaged:
		return resolution_payload(project, task_id, item), 4
	committed = git(repository, 'commit', '--no-edit')
	if committed.returncode != 0:
		return {'status': 'failed', 'phase': 'continue_resolution', 'errors': [f"{item['label']}: {error_text(committed)}"]}, 3
	return None, 0


def create_local_merges(project: dict[str, Any], task_id: str, plan: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], list[str]]:
	results, errors = [], []
	for item, entry in zip(project['repositories'], plan):
		repository = Path(item['path']).resolve()
		if entry.get('already_deployed'):
			results.append({'label': item['label'], 'already_deployed': True})
			continue
		if entry.get('prepared_local'):
			sha = git(repository, 'rev-parse', f"refs/heads/{item['dev_branch']}").stdout.strip()
			results.append({'label': item['label'], 'local_merge_sha': sha, 'created': False})
			continue
		checked = git(repository, 'switch', item['dev_branch'])
		if checked.returncode != 0:
			errors.append(f"{item['label']}: {error_text(checked)}")
			break
		updated = git(repository, 'merge', '--ff-only', f"refs/remotes/{item['remote']}/{item['dev_branch']}")
		if updated.returncode != 0:
			errors.append(f"{item['label']}: {error_text(updated)}")
			break
		merged = git(repository, 'merge', '--no-ff', f"refs/remotes/{item['remote']}/{task_id}", '-m', f"Merge branch '{task_id}' into dev")
		if merged.returncode != 0:
			errors.append(f"{item['label']}: unexpected merge failure: {error_text(merged)}")
			break
		sha = git(repository, 'rev-parse', f"refs/heads/{item['dev_branch']}").stdout.strip()
		results.append({'label': item['label'], 'local_merge_sha': sha, 'created': True})
	return results, errors


def verify_remote_unchanged(project: dict[str, Any], plan: list[dict[str, Any]]) -> list[str]:
	errors = []
	for item, entry in zip(project['repositories'], plan):
		if entry.get('already_deployed'):
			continue
		repository = Path(item['path']).resolve()
		result = git(repository, 'ls-remote', '--heads', item['remote'], f"refs/heads/{item['dev_branch']}", timeout=120)
		actual = result.stdout.split('\t', 1)[0] if result.returncode == 0 and '\t' in result.stdout else None
		if actual != entry['observed_remote_dev_sha']:
			errors.append(f"{item['label']}: remote dev changed during integration")
	return errors


def push_summary(results: list[dict[str, Any]]) -> dict[str, list[str]]:
	operations = ('pushed', 'failed', 'not_attempted', 'already_deployed')
	return {operation: [item['label'] for item in results if item.get('operation') == operation] for operation in operations}


def push_all_dev(project: dict[str, Any], plan: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], dict[str, list[str]], list[str]]:
	results, errors = [], []
	failed = False
	for item, entry in zip(project['repositories'], plan):
		if entry.get('already_deployed'):
			results.append({'label': item['label'], 'operation': 'already_deployed', 'already_deployed': True, 'pushed': False})
			continue
		if failed:
			results.append({'label': item['label'], 'operation': 'not_attempted', 'pushed': False})
			continue
		pushed = push_dev(Path(item['path']).resolve(), item['remote'], item['dev_branch'])
		result = {'label': item['label'], 'operation': 'pushed' if pushed.returncode == 0 else 'failed', 'pushed': pushed.returncode == 0}
		if pushed.returncode != 0:
			result['error'] = error_text(pushed)
			errors.append(f"{item['label']}: {result['error']}")
			results.append(result)
			failed = True
			continue
		result['remote_dev_sha'] = git(Path(item['path']).resolve(), 'rev-parse', f"refs/heads/{item['dev_branch']}").stdout.strip()
		results.append(result)
	return results, push_summary(results), errors


def main() -> int:
	parser = argparse.ArgumentParser()
	action = parser.add_mutually_exclusive_group(required=True)
	action.add_argument('--list', action='store_true')
	action.add_argument('--check', metavar='TASK_ID')
	action.add_argument('--deploy', metavar='TASK_ID')
	action.add_argument('--continue', dest='continue_task', metavar='TASK_ID')
	parser.add_argument('--project')
	parser.add_argument('--cwd', default=os.getcwd())
	parser.add_argument('--manifest', type=Path)
	args = parser.parse_args()
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from workspace_manifest import (
		ManifestError,
		as_deploy_project,
		assert_project_name,
		discover_manifest,
		list_payload,
	)
	try:
		manifest = discover_manifest(cwd=Path(args.cwd), manifest=args.manifest)
		assert_project_name(manifest, args.project)
		if args.list:
			payload = list_payload(manifest)
			payload['push_target'] = 'origin/dev'
			return emit(payload, 0)
		project = as_deploy_project(manifest)
		protected = set(manifest['git']['protected_branches'])
		task_id = args.check or args.deploy or args.continue_task
		assert task_id
		if run(['git', 'check-ref-format', '--branch', task_id]).returncode != 0:
			raise ValueError(f'invalid task branch: {task_id}')
		if task_id in FORBIDDEN_TASK_BRANCHES or task_id in protected:
			raise ValueError(f'task branch is protected: {task_id}')
		checks, check_errors = static_preflight(project, task_id, allow_merge=bool(args.continue_task))
		if check_errors:
			return emit({'status': 'failed', 'action': 'check', 'project': project['key'], 'task_id': task_id, 'repositories': checks, 'errors': check_errors, 'push_started': False}, 1)
		if args.continue_task:
			continued, code = continue_active_merge(project, task_id)
			if continued is not None:
				return emit(continued, code)
		fetches, fetch_errors = refresh_refs(project, task_id)
		if fetch_errors:
			return emit({'status': 'failed', 'action': 'check', 'phase': 'fetch', 'project': project['key'], 'task_id': task_id, 'repositories': fetches, 'errors': fetch_errors, 'push_started': False}, 1)
		plan, plan_errors = integration_plan(project, task_id, fetches)
		if plan_errors:
			return emit({'status': 'failed', 'action': 'check', 'phase': 'plan', 'project': project['key'], 'task_id': task_id, 'repositories': plan, 'errors': plan_errors, 'push_started': False}, 1)
		conflicts = [item for item in plan if item.get('needs_resolution')]
		if args.check:
			status = 'needs_resolution' if conflicts else 'ok'
			return emit({'status': status, 'action': 'check', 'project': project['key'], 'project_type': project['type'], 'task_id': task_id, 'repositories': plan, 'conflicts': [{'label': item['label'], 'files': item['conflict_files']} for item in conflicts], 'push_started': False}, 4 if conflicts else 0)
		if conflicts:
			conflict_label = conflicts[0]['label']
			config_item = next(item for item in project['repositories'] if item['label'] == conflict_label)
			payload, code = begin_conflict(project, task_id, config_item)
			return emit(payload, code)
		local_merges, merge_errors = create_local_merges(project, task_id, plan)
		if merge_errors:
			return emit({'status': 'failed', 'action': 'deploy', 'phase': 'create_local_merges', 'project': project['key'], 'task_id': task_id, 'repositories': local_merges, 'errors': merge_errors, 'push_started': False}, 3)
		race_errors = verify_remote_unchanged(project, plan)
		if race_errors:
			return emit({'status': 'failed', 'action': 'deploy', 'phase': 'remote_race_check', 'project': project['key'], 'task_id': task_id, 'local_merges': local_merges, 'errors': race_errors, 'push_started': False}, 3)
		pushes, summary, push_errors = push_all_dev(project, plan)
		return emit({'status': 'failed' if push_errors else 'ok', 'action': 'deploy', 'phase': 'push_dev', 'project': project['key'], 'project_type': project['type'], 'task_id': task_id, 'local_merges': local_merges, 'repositories': pushes, 'push_summary': summary, 'errors': push_errors, 'push_target': 'origin/dev'}, 3 if push_errors else 0)
	except (OSError, ValueError, json.JSONDecodeError, SafetyError, subprocess.TimeoutExpired, ManifestError) as error:
		return emit({'status': 'failed', 'errors': [str(error)], 'push_target': 'origin/dev'}, 2)


if __name__ == '__main__':
	sys.exit(main())
