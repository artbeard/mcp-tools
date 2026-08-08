.PHONY: build build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment test-go

# Собрать все Go CLI в bin/
build: build-tracker-transition build-tracker-comment build-tracker-create-issue build-tracker-time build-gitlab-mr-comment

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

# Прогнать все Go-тесты
test-go:
	go test ./...
