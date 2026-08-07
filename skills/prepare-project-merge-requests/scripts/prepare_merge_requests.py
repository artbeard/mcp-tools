#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


ALLOWED_MR_TARGETS = {'master', 'main'}
FORBIDDEN_TASK_BRANCHES = {'master', 'main', 'dev'}


class SafetyError(RuntimeError):
	pass


def run(command: list[str], cwd: Path | None = None, timeout: int = 90) -> subprocess.CompletedProcess[str]:
	return subprocess.run(command, cwd=cwd, text=True, capture_output=True, timeout=timeout, check=False)


def git(repository: Path, *arguments: str, timeout: int = 90) -> subprocess.CompletedProcess[str]:
	if arguments and arguments[0] == 'push':
		raise SafetyError('use push_task_branch; generic git push is forbidden')
	return run(['git', '-C', str(repository), *arguments], timeout=timeout)


def push_task_branch(repository: Path, remote: str, task_id: str, protected: set[str]) -> subprocess.CompletedProcess[str]:
	if task_id in FORBIDDEN_TASK_BRANCHES or task_id in protected:
		raise SafetyError(f'cannot publish protected branch: {task_id}')
	refspec = f'refs/heads/{task_id}:refs/heads/{task_id}'
	return run(['git', '-C', str(repository), 'push', remote, refspec], timeout=180)


def emit(payload: dict[str, Any], code: int) -> int:
	print(json.dumps(payload, ensure_ascii=False, indent=2))
	return code


def error_text(result: subprocess.CompletedProcess[str]) -> str:
	return (result.stderr or result.stdout).strip() or f'command failed with exit code {result.returncode}'


def load_env(path: Path) -> dict[str, str]:
	values = {}
	for raw_line in path.read_text(encoding='utf-8').splitlines():
		line = raw_line.strip()
		if not line or line.startswith('#') or '=' not in line:
			continue
		key, value = line.split('=', 1)
		values[key.strip()] = value.strip().strip("\"'")
	return values


def load_config(path: Path) -> tuple[dict[str, Any], list[dict[str, Any]], set[str]]:
	data = json.loads(path.read_text(encoding='utf-8'))
	projects = data.get('projects')
	protected = data.get('protected_branches')
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
		for repository in repositories:
			for field in ('label', 'path', 'remote', 'target_branch'):
				if field not in repository:
					raise ValueError(f"{project['key']}: repository missing field: {field}")
			path_value = str(Path(repository['path']).resolve())
			if path_value in seen_paths:
				raise ValueError(f'duplicate repository path: {path_value}')
			seen_paths.add(path_value)
			if repository['target_branch'] not in ALLOWED_MR_TARGETS:
				raise ValueError(
					f"{project['key']}/{repository['label']}: MR target must be master or main, "
					f"got: {repository['target_branch']}"
				)
			if repository['target_branch'] not in protected:
				raise ValueError(f"unprotected MR target: {repository['target_branch']}")
	return data, projects, set(protected)


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


def parse_remote(remote_url: str) -> tuple[str, str]:
	if remote_url.startswith('git@') and ':' in remote_url:
		host, path = remote_url[4:].split(':', 1)
	elif remote_url.startswith('ssh://'):
		parsed = urllib.parse.urlparse(remote_url)
		host, path = parsed.hostname or '', parsed.path.lstrip('/')
	else:
		parsed = urllib.parse.urlparse(remote_url)
		host, path = parsed.hostname or '', parsed.path.lstrip('/')
	if not host or not path:
		raise ValueError(f'unsupported remote URL: {remote_url}')
	return host, path.removesuffix('.git')


def api_request(base_url: str, token: str, method: str, endpoint: str, query: dict[str, str] | None = None, body: dict[str, str] | None = None) -> Any:
	url = f"{base_url.rstrip('/')}/api/v4{endpoint}"
	if query:
		url += '?' + urllib.parse.urlencode(query)
	data = json.dumps(body).encode('utf-8') if body is not None else None
	request = urllib.request.Request(url, data=data, method=method, headers={'PRIVATE-TOKEN': token, 'Accept': 'application/json', 'Content-Type': 'application/json'})
	opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
	try:
		with opener.open(request, timeout=45) as response:
			content = response.read().decode('utf-8')
			return json.loads(content) if content else {}
	except urllib.error.HTTPError as error:
		content = error.read().decode('utf-8', errors='replace')
		raise RuntimeError(f'GitLab API HTTP {error.code}: {content[:2000]}') from error


def validate_mr_record(record: Any, project_path: str) -> dict[str, Any]:
	if not isinstance(record, dict):
		raise ValueError('GitLab MR response must be an object')
	iid = record.get('iid')
	url = record.get('web_url')
	if not isinstance(iid, int) or isinstance(iid, bool) or iid < 1:
		raise ValueError('GitLab MR response has no valid iid')
	if not isinstance(url, str) or not url.strip():
		raise ValueError('GitLab MR response has no web_url')
	parsed = urllib.parse.urlparse(url.strip())
	if parsed.scheme not in ('http', 'https') or not parsed.netloc:
		raise ValueError('GitLab MR web_url must be an absolute HTTP(S) URL')
	expected_path = f'/{project_path}/-/merge_requests/{iid}'
	if urllib.parse.unquote(parsed.path).rstrip('/') != expected_path:
		raise ValueError(f'GitLab MR web_url does not match project and iid: {url}')
	return {'iid': iid, 'url': url.strip(), 'title': record.get('title')}


def find_existing_mr(base_url: str, token: str, project_path: str, source_branch: str, target_branch: str) -> dict[str, Any] | None:
	encoded = urllib.parse.quote(project_path, safe='')
	existing = api_request(base_url, token, 'GET', f'/projects/{encoded}/merge_requests', query={'state': 'opened', 'source_branch': source_branch, 'target_branch': target_branch})
	if not isinstance(existing, list):
		raise ValueError('unexpected GitLab response while checking existing MR')
	return validate_mr_record(existing[0], project_path) if existing else None


def mr_urls(records: list[dict[str, Any]]) -> list[str]:
	return [item['url'] for item in records if item.get('operation') in ('created', 'already_exists') and isinstance(item.get('url'), str)]


def operation_markers(repository: Path) -> list[str]:
	git_dir_result = git(repository, 'rev-parse', '--absolute-git-dir')
	if git_dir_result.returncode != 0:
		return ['UNKNOWN_GIT_DIR']
	git_dir = Path(git_dir_result.stdout.strip())
	return [name for name in ('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD', 'BISECT_LOG', 'rebase-apply', 'rebase-merge') if (git_dir / name).exists()]


def task_history_guard(repository: Path, target_ref: str, task_ref: str) -> tuple[dict[str, Any], list[str]]:
	result: dict[str, Any] = {'target_ref': target_ref, 'task_ref': task_ref}
	errors: list[str] = []
	merge_base = git(repository, 'merge-base', target_ref, task_ref)
	if merge_base.returncode != 0 or not merge_base.stdout.strip():
		errors.append('task branch does not share valid history with remote master/main target')
		return result, errors
	result['branch_point'] = merge_base.stdout.strip()
	merges = git(repository, 'rev-list', '--merges', f'{target_ref}..{task_ref}')
	if merges.returncode != 0:
		errors.append(f'cannot inspect task merge history: {error_text(merges)}')
		return result, errors
	unexpected_merges = merges.stdout.splitlines()
	result['unexpected_merge_commits'] = unexpected_merges
	if unexpected_merges:
		errors.append(
			'task branch contains merge commits outside remote master/main; '
			'possible dev contamination, reconstruct the branch from the current target'
		)
	return result, errors


def inspect_repository(item: dict[str, Any], task_id: str, title_override: str | None, protected: set[str], base_url: str, token: str) -> dict[str, Any]:
	repository = Path(item['path']).resolve()
	result: dict[str, Any] = {'label': item['label'], 'path': str(repository), 'ready': False, 'errors': []}
	errors: list[str] = result['errors']
	if task_id in FORBIDDEN_TASK_BRANCHES or task_id in protected:
		errors.append(f'task branch is protected: {task_id}')
		return result
	valid = run(['git', 'check-ref-format', '--branch', task_id])
	if valid.returncode != 0:
		errors.append(f'invalid task branch: {task_id}')
		return result
	inside = git(repository, 'rev-parse', '--is-inside-work-tree')
	if inside.returncode != 0:
		errors.append('not a Git work tree')
		return result
	status = git(repository, 'status', '--porcelain=v1', '--untracked-files=all')
	if status.returncode != 0 or status.stdout.strip():
		if status.stdout.strip():
			result['changes'] = status.stdout.splitlines()
		errors.append('repository is not clean')
	markers = operation_markers(repository)
	if markers:
		result['in_progress'] = markers
		errors.append('Git operation is in progress')
	local_task = git(repository, 'show-ref', '--verify', '--hash', f'refs/heads/{task_id}')
	if local_task.returncode != 0:
		errors.append(f'local task branch does not exist: {task_id}')
		return result
	local_sha = local_task.stdout.strip()
	result['local_sha'] = local_sha
	remote_url_result = git(repository, 'remote', 'get-url', item['remote'])
	if remote_url_result.returncode != 0:
		errors.append(f"remote does not exist: {item['remote']}")
		return result
	host, project_path = parse_remote(remote_url_result.stdout.strip())
	result['gitlab_project'] = project_path
	remote_refs = git(repository, 'ls-remote', '--heads', item['remote'], f"refs/heads/{item['target_branch']}", f'refs/heads/{task_id}', timeout=120)
	if remote_refs.returncode != 0:
		errors.append(f'cannot inspect remote: {error_text(remote_refs)}')
		return result
	refs = {line.split('\t', 1)[1]: line.split('\t', 1)[0] for line in remote_refs.stdout.splitlines() if '\t' in line}
	target_ref = f"refs/heads/{item['target_branch']}"
	task_ref = f'refs/heads/{task_id}'
	if target_ref not in refs:
		errors.append(f"remote target branch does not exist: {item['target_branch']}")
		return result
	target_tracking_ref = f"refs/remotes/{item['remote']}/{item['target_branch']}"
	fetched_target = git(repository, 'fetch', '--no-tags', item['remote'], f'{target_ref}:{target_tracking_ref}', timeout=120)
	if fetched_target.returncode != 0:
		errors.append(f'cannot fetch remote target branch: {error_text(fetched_target)}')
		return result
	history, history_errors = task_history_guard(repository, target_tracking_ref, local_sha)
	result['history_guard'] = history
	errors.extend(history_errors)
	if history_errors:
		return result
	unique_commits = git(repository, 'rev-list', '--count', f'{target_tracking_ref}..{local_sha}')
	if unique_commits.returncode != 0:
		errors.append(f'cannot compare task with target: {error_text(unique_commits)}')
		return result
	commits_ahead = int(unique_commits.stdout.strip())
	result['commits_ahead_of_target'] = commits_ahead
	result['needs_mr'] = commits_ahead > 0
	remote_task_sha = refs.get(task_ref)
	result['remote_task_sha'] = remote_task_sha
	if result['needs_mr'] and remote_task_sha and remote_task_sha != local_sha:
		fetched = git(repository, 'fetch', '--no-tags', item['remote'], f'{task_ref}:refs/remotes/{item["remote"]}/{task_id}', timeout=120)
		if fetched.returncode != 0:
			errors.append(f'cannot fetch remote task branch: {error_text(fetched)}')
		else:
			ancestor = git(repository, 'merge-base', '--is-ancestor', remote_task_sha, local_sha)
			if ancestor.returncode != 0:
				errors.append('remote task branch is not an ancestor of local task branch')
	message = git(repository, 'log', '-1', '--format=%s', f'refs/heads/{task_id}')
	commit_subject = message.stdout.strip() if message.returncode == 0 else task_id
	result['title'] = title_override or (commit_subject if commit_subject.startswith(task_id) else f'{task_id} | {commit_subject}')
	if result['needs_mr']:
		existing = find_existing_mr(base_url, token, project_path, task_id, item['target_branch'])
		if existing:
			result['existing_mr'] = existing
	result['target_branch'] = item['target_branch']
	result['ready'] = not errors
	return result


def main() -> int:
	parser = argparse.ArgumentParser()
	action = parser.add_mutually_exclusive_group(required=True)
	action.add_argument('--list', action='store_true')
	action.add_argument('--check', metavar='TASK_ID')
	action.add_argument('--create', metavar='TASK_ID')
	parser.add_argument('--project')
	parser.add_argument('--cwd', default=os.getcwd())
	parser.add_argument('--title')
	parser.add_argument('--description-file', type=Path)
	args = parser.parse_args()
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import PROJECTS_MR, GITLAB_ENV, resolve_config_path
	config_path = Path(os.environ.get('PROJECT_MR_CONFIG', PROJECTS_MR))
	try:
		config, projects, protected = load_config(config_path)
		if args.list:
			return emit({'status': 'ok', 'action': 'list', 'projects': [{'key': item['key'], 'type': item['type'], 'repositories': len(item['repositories'])} for item in projects]}, 0)
		project = select_project(projects, args.project, Path(args.cwd))
		task_id = args.check or args.create
		assert task_id
		env_file = resolve_config_path(config.get('gitlab_env_file') or GITLAB_ENV)
		env = load_env(env_file)
		token = env.get('GITLAB_TOKEN', '')
		base_url = env.get('GITLAB_BASE_URL', '')
		if not token or not base_url:
			raise ValueError('GITLAB_TOKEN and GITLAB_BASE_URL must be set in the configured env file')
		description = args.description_file.read_text(encoding='utf-8') if args.description_file else ''
		checks = [inspect_repository(item, task_id, args.title, protected, base_url, token) for item in project['repositories']]
	except (OSError, ValueError, json.JSONDecodeError, RuntimeError, SafetyError, subprocess.TimeoutExpired) as error:
		return emit({'status': 'failed', 'errors': [str(error)]}, 2)
	errors = [f"{item['label']}: {error}" for item in checks for error in item['errors']]
	if errors or args.check:
		check_urls = [item['existing_mr']['url'] for item in checks if item.get('existing_mr')]
		return emit({'status': 'failed' if errors else 'ok', 'action': 'check', 'project': project['key'], 'project_type': project['type'], 'task_id': task_id, 'repositories': checks, 'mr_urls': check_urls, 'errors': errors}, 1 if errors else 0)

	push_results = []
	for item, check in zip(project['repositories'], checks):
		if not check['needs_mr']:
			push_results.append({'label': item['label'], 'published': False, 'operation': 'no_changes', 'sha': check['local_sha']})
			continue
		pushed = push_task_branch(Path(item['path']).resolve(), item['remote'], task_id, protected)
		entry = {'label': item['label'], 'published': pushed.returncode == 0, 'sha': check['local_sha']}
		if pushed.returncode != 0:
			entry['error'] = error_text(pushed)
			push_results.append(entry)
			return emit({'status': 'failed', 'action': 'create', 'phase': 'publish_task_branches', 'project': project['key'], 'task_id': task_id, 'repositories': push_results, 'mr_urls': [], 'errors': [f"{item['label']}: {entry['error']}"]}, 3)
		push_results.append(entry)

	mr_results = []
	for item, check in zip(project['repositories'], checks):
		if not check['needs_mr']:
			mr_results.append({'label': item['label'], 'operation': 'no_changes', 'url': None})
			continue
		if check.get('existing_mr'):
			mr_results.append({'label': item['label'], 'operation': 'already_exists', **check['existing_mr']})
			continue
		project_encoded = urllib.parse.quote(check['gitlab_project'], safe='')
		try:
			created = api_request(base_url, token, 'POST', f'/projects/{project_encoded}/merge_requests', body={'source_branch': task_id, 'target_branch': item['target_branch'], 'title': check['title'], 'description': description})
			validated = validate_mr_record(created, check['gitlab_project'])
		except (RuntimeError, ValueError) as create_error:
			try:
				validated = find_existing_mr(base_url, token, check['gitlab_project'], task_id, item['target_branch'])
			except (RuntimeError, ValueError):
				validated = None
			if validated is not None:
				mr_results.append({'label': item['label'], 'operation': 'already_exists', **validated})
				continue
			error = create_error
			mr_results.append({'label': item['label'], 'operation': 'failed', 'error': str(error)})
			return emit({'status': 'failed', 'action': 'create', 'phase': 'create_merge_requests', 'project': project['key'], 'task_id': task_id, 'published_branches': push_results, 'merge_requests': mr_results, 'mr_urls': mr_urls(mr_results), 'errors': [f"{item['label']}: {error}"]}, 3)
		mr_results.append({'label': item['label'], 'operation': 'created', **validated})
	urls = mr_urls(mr_results)
	expected_url_count = sum(1 for item in mr_results if item['operation'] in ('created', 'already_exists'))
	if len(urls) != expected_url_count:
		return emit({'status': 'failed', 'action': 'create', 'phase': 'validate_merge_request_urls', 'project': project['key'], 'task_id': task_id, 'published_branches': push_results, 'merge_requests': mr_results, 'mr_urls': urls, 'errors': ['not every created or existing merge request has a validated URL']}, 3)
	return emit({'status': 'ok', 'action': 'create', 'project': project['key'], 'project_type': project['type'], 'task_id': task_id, 'published_branches': push_results, 'merge_requests': mr_results, 'mr_urls': urls, 'errors': []}, 0)


if __name__ == '__main__':
	sys.exit(main())
