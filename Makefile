.PHONY: build build-mcp-server build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment build-gitlab-mr-context build-gitlab-group-clone build-prepare-task build-prepare-mr build-deploy-task build-start-sandbox build-codestyle test-go

# Собрать MCP-сервер + все Go CLI в bin/
build: build-mcp-server build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment build-gitlab-mr-context build-gitlab-group-clone build-prepare-task build-prepare-mr build-deploy-task build-start-sandbox build-codestyle

# stdio MCP → bin/mcp-server
build-mcp-server:
	mkdir -p bin
	go build -o bin/mcp-server ./cmd/mcp-server

# Смена статуса задачи → bin/tracker-transition
build-tracker-transition:
	mkdir -p bin
	go build -o bin/tracker-transition ./skills/yandex-tracker-commenter/cmd/tracker-transition

# Комментарий к задаче → bin/tracker-comment
build-tracker-comment:
	mkdir -p bin
	go build -o bin/tracker-comment ./skills/yandex-tracker-commenter/cmd/tracker-comment

# Создание задачи → bin/tracker-create-issue
build-tracker-create-issue:
	mkdir -p bin
	go build -o bin/tracker-create-issue ./skills/yandex-tracker-issue-creator/cmd/tracker-create-issue

# Таймер / worklog → bin/tracker-time
build-tracker-time:
	mkdir -p bin
	go build -o bin/tracker-time ./skills/yandex-tracker-timekeeper/cmd/tracker-time

# Комментарий в GitLab MR → bin/gitlab-mr-comment
build-gitlab-mr-comment:
	mkdir -p bin
	go build -o bin/gitlab-mr-comment ./skills/gitlab-mr-commenter/cmd/gitlab-mr-comment

# JSON-контекст MR для ревьюера → bin/gitlab-mr-context
build-gitlab-mr-context:
	mkdir -p bin
	go build -o bin/gitlab-mr-context ./skills/gitlab-mr-reviewer/cmd/gitlab-mr-context

# Клонирование GitLab-группы → bin/gitlab-group-clone
build-gitlab-group-clone:
	mkdir -p bin
	go build -o bin/gitlab-group-clone ./skills/gitlab-group-cloner/cmd/gitlab-group-clone

# Локальные task-ветки → bin/prepare-task
build-prepare-task:
	mkdir -p bin
	go build -o bin/prepare-task ./skills/prepare-project-task/cmd/prepare-task

# Push task + GitLab MR → bin/prepare-mr
build-prepare-mr:
	mkdir -p bin
	go build -o bin/prepare-mr ./skills/prepare-project-merge-requests/cmd/prepare-mr

# Merge task → dev + push origin/dev → bin/deploy-task
build-deploy-task:
	mkdir -p bin
	go build -o bin/deploy-task ./skills/deploy-project-task-to-dev/cmd/deploy-task

# Sandbox compose/tmux lifecycle → bin/start-sandbox
build-start-sandbox:
	mkdir -p bin
	go build -o bin/start-sandbox ./skills/start-project-sandbox/cmd/start-sandbox

# Поиск/чтение правил кодстайла → bin/codestyle
build-codestyle:
	mkdir -p bin
	go build -o bin/codestyle ./skills/codestyle/cmd/codestyle

# Прогнать все Go-тесты
test-go:
	go test ./...
