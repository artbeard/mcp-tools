# Безопасность Создания Задач В Tracker

Этот skill выполняет write-действие в Yandex Tracker: создает новую задачу.

## Требования Перед Созданием

- Пользователь должен увидеть exact queue.
- Пользователь должен увидеть exact summary.
- Пользователь должен увидеть полный description.
- Пользователь должен увидеть project/tags/period/SP/assignee и field mapping.
- Пользователь должен увидеть exact JSON payload из dry-run.
- Пользователь должен явно подтвердить создание после dry-run.
- Для реального создания обязательны оба CLI-флага: `--send` и `--confirm-send`. `CONFIRM_SEND=YES` оставлен только для обратной совместимости.

## Токен

- Токен хранится только в `.env` / `.env.local` в корне mcp-tools.
- Файл должен иметь права `600`.
- Токен, ClientID и Client secret не должны попадать в чат, git, logs, body-файлы или error output.
- Нужен OAuth token с правом создавать задачи в нужной очереди Tracker.
- Для организаций Yandex Cloud используй `TRACKER_CLOUD_ORG_ID`; для обычной организации `TRACKER_ORG_ID`.

## Что Делать При Ошибках

- `.env обязателен` / env file not found: попроси пользователя локально создать `.env` (`cp .env.example .env`).
- `TRACKER_OAUTH_TOKEN is empty`: попроси пользователя локально заполнить OAuth token.
- `TRACKER_ORG_ID or TRACKER_CLOUD_ORG_ID is required`: попроси указать org id в env.
- `401/403`: token невалиден, истек, не имеет прав или нет доступа к очереди.
- `404`: очередь или field key неверные, либо нет доступа.
- `422/400`: payload не соответствует схеме очереди; проверь field mapping.
- `--send requires --confirm-send or CONFIRM_SEND=YES`: реальная отправка заблокирована safety guard.

## Границы

- CLI только создает новую задачу.
- CLI не меняет существующие задачи.
- CLI не добавляет комментарии, worklog, связи, чеклисты или workflow transitions.
- CLI не выполняет OAuth flow и не хранит Client secret.
