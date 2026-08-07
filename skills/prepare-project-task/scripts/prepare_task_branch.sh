#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${1:-}" == "--list" && $# -eq 1 ]]; then
  exec python3 "${SCRIPT_DIR}/prepare_task_branch.py" --list
fi

if [[ "${1:-}" == "--check" && $# -eq 3 ]]; then
  exec python3 "${SCRIPT_DIR}/prepare_task_branch.py" --check "$3" --project "$2"
fi

if [[ $# -eq 2 && "${1:-}" != --* ]]; then
  exec python3 "${SCRIPT_DIR}/prepare_task_branch.py" --prepare "$2" --project "$1"
fi

exec python3 "${SCRIPT_DIR}/prepare_task_branch.py" "$@"
