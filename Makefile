.PHONY: build build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment build-prepare-task build-prepare-mr build-deploy-task build-start-sandbox test-go

# Собрать все Go CLI в bin/
build: build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment build-prepare-task build-prepare-mr build-deploy-task build-start-sandbox

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

# Прогнать все Go-тесты
test-go:
	go test ./...
