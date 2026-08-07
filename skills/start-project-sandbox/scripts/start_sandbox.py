#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import shlex
import shutil
import subprocess
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any


def run(command: list[str], cwd: str | None = None, timeout: int = 30) -> subprocess.CompletedProcess[str]:
	return subprocess.run(command, cwd=cwd, text=True, capture_output=True, timeout=timeout, check=False)


def load_config(config_path: Path) -> dict[str, Any]:
	data = json.loads(config_path.read_text(encoding='utf-8'))
	components = data.get('components')
	if not isinstance(components, list) or not components:
		raise ValueError('config must contain a non-empty components array')
	return data


def project_sources(data: dict[str, Any]) -> tuple[dict[str, list[str]], dict[str, str]]:
	components = data['components']
	atomic_projects = {
		item.get('project')
		for item in components
		if isinstance(item.get('project'), str) and item['project']
	}
	projects = {project: [project] for project in atomic_projects}
	aliases: dict[str, str] = {}
	definitions = data.get('projects', [])
	if not isinstance(definitions, list):
		raise ValueError('projects must be an array')

	definition_keys = []
	for definition in definitions:
		if not isinstance(definition, dict):
			raise ValueError('each project definition must be an object')
		key = definition.get('key')
		if not isinstance(key, str) or not key:
			raise ValueError('project definition key must be a non-empty string')
		definition_keys.append(key)
	if len(definition_keys) != len(set(definition_keys)):
		raise ValueError('project definition keys must be unique')
	reserved_project_keys = atomic_projects | set(definition_keys)

	for definition in definitions:
		key = definition.get('key')
		sources = definition.get('sources')
		assert isinstance(key, str)
		if not isinstance(sources, list) or not sources or not all(isinstance(source, str) and source for source in sources):
			raise ValueError(f'{key}: sources must be a non-empty string array')
		unknown_sources = sorted(set(sources) - atomic_projects)
		if unknown_sources:
			raise ValueError(f"{key}: unknown component projects: {', '.join(unknown_sources)}")
		projects[key] = list(dict.fromkeys(sources))

		project_aliases = definition.get('aliases', [])
		if not isinstance(project_aliases, list) or not all(isinstance(alias, str) and alias for alias in project_aliases):
			raise ValueError(f'{key}: aliases must be a string array')
		for alias in project_aliases:
			if alias in reserved_project_keys or alias in aliases:
				raise ValueError(f'duplicate project key or alias: {alias}')
			aliases[alias] = key

	return projects, aliases


def available_projects(data: dict[str, Any]) -> list[str]:
	projects, aliases = project_sources(data)
	return sorted(set(projects) | set(aliases))


def load_components(config_path: Path, project: str | None = None) -> list[dict[str, Any]]:
	data = load_config(config_path)
	components = data['components']
	if project is None:
		return components

	projects, aliases = project_sources(data)
	canonical = aliases.get(project, project)
	sources = projects.get(canonical)
	if sources is None:
		available = ', '.join(available_projects(data))
		raise ValueError(f'unknown project: {project}; available projects: {available}')
	source_set = set(sources)
	return [item for item in components if item.get('project') in source_set]


def tmux_has(session: str) -> bool:
	return run(['tmux', 'has-session', '-t', f'={session}']).returncode == 0


def tmux_owner(session: str) -> tuple[str, str]:
	project = run(['tmux', 'show-options', '-qv', '-t', session, '@codex_sandbox_project']).stdout.strip()
	component = run(['tmux', 'show-options', '-qv', '-t', session, '@codex_sandbox_component']).stdout.strip()
	return project, component


def compose_base_command(item: dict[str, Any]) -> list[str]:
	command = ['docker', 'compose']
	for compose_file in item.get('compose_files', []):
		command.extend(['-f', compose_file])
	return command


def compose_state(item: dict[str, Any]) -> tuple[bool, bool, dict[str, Any]]:
	cwd = item['directory']
	base = compose_base_command(item)
	expected_result = run(base + ['config', '--services'], cwd)
	running_result = run(base + ['ps', '--status', 'running', '--services'], cwd)
	created_result = run(base + ['ps', '-q'], cwd)
	if expected_result.returncode != 0 or running_result.returncode != 0 or created_result.returncode != 0:
		return False, False, {
			'error': (expected_result.stderr or running_result.stderr or created_result.stderr).strip(),
		}

	configured_services = item.get('compose_services')
	expected = set(configured_services) if configured_services else {line for line in expected_result.stdout.splitlines() if line}
	running = {line for line in running_result.stdout.splitlines() if line}
	created = {line for line in created_result.stdout.splitlines() if line}
	ready = bool(expected) and expected <= running
	exists = bool(expected & created)
	return ready, exists, {
		'expected_services': sorted(expected),
		'running_services': sorted(running),
	}


def http_ready(url: str) -> tuple[bool, dict[str, Any]]:
	opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
	try:
		with opener.open(url, timeout=2) as response:
			return 200 <= response.status < 500, {'url': url, 'http_status': response.status}
	except Exception as error:
		return False, {'url': url, 'error': str(error)}


def component_status(item: dict[str, Any]) -> dict[str, Any]:
	probe = item['probe']
	runtime = item['runtime']
	result: dict[str, Any] = {
		'component': item['component'],
		'runtime': runtime,
		'ready': False,
	}

	if runtime == 'compose':
		ready, exists, details = compose_state(item)
		running = set(details.get('running_services', []))
		expected = set(details.get('expected_services', []))
		partially_running = bool(expected & running)
		result.update({'state': 'running' if ready else ('degraded' if partially_running else 'stopped'), 'ready': ready, 'details': details})
		return result

	if runtime != 'tmux':
		result.update({'state': 'invalid', 'details': {'error': f'unsupported runtime: {runtime}'}})
		return result

	session = item['session']
	result['session'] = session
	if not tmux_has(session):
		result.update({'state': 'stopped', 'details': {'session_exists': False}})
		return result

	owner_project, owner_component = tmux_owner(session)
	owned = owner_project == item['project'] and owner_component == item['component']
	if not owned:
		result.update({
			'state': 'unmanaged',
			'details': {'session_exists': True, 'owner_project': owner_project, 'owner_component': owner_component},
		})
		return result

	if probe['type'] == 'http':
		ready, details = http_ready(probe['url'])
	else:
		ready, details = True, {'session_exists': True}
	result.update({'state': 'running' if ready else 'degraded', 'ready': ready, 'details': details})
	return result


def validate(items: list[dict[str, Any]]) -> list[str]:
	errors: list[str] = []
	seen_sessions: set[str] = set()
	for item in items:
		label = f"{item.get('project')}/{item.get('component')}"
		missing_fields = []
		for field in ('project', 'component', 'directory', 'runtime', 'start_command', 'stop_command', 'probe', 'timeout_seconds'):
			if field not in item:
				missing_fields.append(field)
		if missing_fields:
			errors.extend(f'{label}: missing field {field}' for field in missing_fields)
			continue
		if not Path(item['directory']).is_dir():
			errors.append(f"{label}: directory does not exist: {item['directory']}")
		for command_field in ('start_command', 'stop_command'):
			command = item[command_field]
			if command and shutil.which(command[0]) is None:
				errors.append(f'{label}: executable unavailable: {command[0]}')
		if item['runtime'] == 'tmux':
			session = item.get('session', '')
			if not session or session in seen_sessions:
				errors.append(f'{label}: invalid or duplicate tmux session: {session}')
			seen_sessions.add(session)
			if shutil.which('tmux') is None:
				errors.append(f'{label}: tmux is unavailable')
		if item['runtime'] not in ('tmux', 'compose'):
			errors.append(f"{label}: unsupported runtime: {item['runtime']}")
		for image in item.get('required_docker_images', []):
			image_result = run(['docker', 'image', 'inspect', image])
			if image_result.returncode != 0:
				errors.append(f'{label}: required Docker image is unavailable: {image}')
	return errors


def diagnostics(item: dict[str, Any]) -> str:
	if item['runtime'] == 'compose':
		parts = []
		base = compose_base_command(item)
		for command in (base + ['ps'], base + ['logs', '--tail', '60']):
			result = run(command, item['directory'])
			parts.append(f"$ {shlex.join(command)}\n{result.stdout}{result.stderr}")
		return '\n'.join(parts)[-12000:]

	session = item['session']
	if not tmux_has(session):
		return f'tmux session absent: {session}'
	result = run(['tmux', 'capture-pane', '-p', '-S', '-80', '-t', f'{session}:0.0'])
	return (result.stdout + result.stderr)[-12000:]


def wait_until_ready(item: dict[str, Any]) -> dict[str, Any]:
	deadline = time.monotonic() + int(item['timeout_seconds'])
	last = component_status(item)
	while time.monotonic() < deadline:
		last = component_status(item)
		if last['ready']:
			return last
		if last['state'] in ('stopped', 'unmanaged', 'invalid') and item['runtime'] == 'tmux':
			return last
		time.sleep(0.5)
	return last


def start_component(item: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
	current = component_status(item)
	if current['ready']:
		current['operation'] = 'already_running'
		return current, None
	if current['state'] == 'unmanaged' or (item['runtime'] == 'tmux' and current['state'] == 'degraded'):
		return current, f"refusing to replace component in state {current['state']}"

	if item['runtime'] == 'compose':
		started = run(item['start_command'], item['directory'], int(item['timeout_seconds']))
		if started.returncode != 0:
			return component_status(item), (started.stdout + started.stderr).strip()
	else:
		command = shlex.join(item['start_command'])
		created = run(['tmux', 'new-session', '-d', '-s', item['session'], '-c', item['directory'], '-n', item['component'], command])
		if created.returncode != 0:
			return component_status(item), (created.stdout + created.stderr).strip()
		for option, value in (('@codex_sandbox_project', item['project']), ('@codex_sandbox_component', item['component'])):
			marked = run(['tmux', 'set-option', '-q', '-t', item['session'], option, value])
			if marked.returncode != 0:
				run(['tmux', 'kill-session', '-t', item['session']])
				return component_status(item), (marked.stdout + marked.stderr).strip()

	status = wait_until_ready(item)
	status['operation'] = 'started' if status['ready'] else 'failed'
	if status['ready']:
		return status, None
	return status, 'readiness timeout or process exited'


def stop_component(item: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
	current = component_status(item)
	if current['state'] == 'stopped':
		current['operation'] = 'already_stopped'
		return current, None
	if current['state'] == 'unmanaged':
		return current, 'refusing to stop unmanaged tmux session'

	if item['stop_command']:
		stopped = run(item['stop_command'], item['directory'], int(item['timeout_seconds']))
		if stopped.returncode != 0:
			return component_status(item), (stopped.stdout + stopped.stderr).strip()
	if item['runtime'] == 'tmux' and tmux_has(item['session']):
		killed = run(['tmux', 'kill-session', '-t', item['session']])
		if killed.returncode != 0:
			return component_status(item), (killed.stdout + killed.stderr).strip()

	status = component_status(item)
	status['operation'] = 'stopped' if status['state'] == 'stopped' else 'failed'
	if status['state'] == 'stopped':
		return status, None
	return status, 'component is still present after stop'


def emit(payload: dict[str, Any], exit_code: int) -> int:
	print(json.dumps(payload, ensure_ascii=False, indent=2))
	return exit_code


def main() -> int:
	parser = argparse.ArgumentParser()
	group = parser.add_mutually_exclusive_group(required=True)
	group.add_argument('--list', action='store_true')
	group.add_argument('--check', metavar='PROJECT')
	group.add_argument('--status', metavar='PROJECT')
	group.add_argument('--start', metavar='PROJECT')
	group.add_argument('--stop', metavar='PROJECT')
	group.add_argument('--diagnose', metavar='PROJECT')
	args = parser.parse_args()
	root = Path(__file__).resolve().parents[3]
	if str(root) not in sys.path:
		sys.path.insert(0, str(root))
	from mcp_paths import SANDBOXES
	config_path = Path(os.environ.get('SANDBOX_CONFIG_FILE', SANDBOXES))

	try:
		data = load_config(config_path)
		all_items = data['components']
		if args.list:
			return emit({'status': 'ok', 'action': 'list', 'projects': available_projects(data)}, 0)
		project = args.check or args.status or args.start or args.stop or args.diagnose
		items = load_components(config_path, project)
	except (OSError, ValueError, json.JSONDecodeError) as error:
		return emit({'status': 'failed', 'errors': [str(error)]}, 2)

	errors = validate(items)
	if errors:
		return emit({'status': 'failed', 'action': 'check', 'project': project, 'errors': errors}, 2)
	if args.check:
		return emit({'status': 'ok', 'action': 'check', 'project': project, 'components': [item['component'] for item in items]}, 0)
	if args.status:
		components = [component_status(item) for item in items]
		return emit({'status': 'ok', 'action': 'status', 'project': project, 'components': components}, 0)
	if args.diagnose:
		components = []
		for item in items:
			status = component_status(item)
			if not status['ready']:
				status['diagnostics'] = diagnostics(item)
			components.append(status)
		return emit({'status': 'ok', 'action': 'diagnose', 'project': project, 'components': components}, 0)

	results = []
	lifecycle_errors = []
	operation_items = items if args.start else list(reversed(items))
	for item in operation_items:
		status, error = start_component(item) if args.start else stop_component(item)
		if error:
			status['error'] = error
			status['diagnostics'] = diagnostics(item)
			lifecycle_errors.append(f"{item['component']}: {error}")
		results.append(status)
		if error and args.start:
			break
	return emit({
		'status': 'failed' if lifecycle_errors else 'ok',
		'action': 'start' if args.start else 'stop',
		'project': project,
		'components': results,
		'errors': lifecycle_errors,
	}, 1 if lifecycle_errors else 0)


if __name__ == '__main__':
	sys.exit(main())
