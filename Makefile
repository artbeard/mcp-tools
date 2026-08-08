.PHONY: build build-tracker-transition build-tracker-comment test-go

# Собрать оба Tracker CLI в bin/
build: build-tracker-transition build-tracker-comment

# Смена статуса задачи → bin/tracker-transition
build-tracker-transition:
	mkdir -p bin
	go build -o bin/tracker-transition ./skills/yandex-tracker-commenter/cmd/tracker-transition

# Комментарий к задаче → bin/tracker-comment
build-tracker-comment:
	mkdir -p bin
	go build -o bin/tracker-comment ./skills/yandex-tracker-commenter/cmd/tracker-comment

# Прогнать все Go-тесты
test-go:
	go test ./...
