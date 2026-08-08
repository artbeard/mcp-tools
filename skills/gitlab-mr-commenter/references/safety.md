# Безопасность Отправки Комментариев

Этот skill выполняет write-действие в GitLab от имени пользователя, чей token лежит в локальном env.

## Требования Перед Отправкой

- Пользователь должен увидеть exact MR URL.
- Пользователь должен увидеть полный exact body комментария.
- Для line discussion пользователь должен увидеть exact file path, line number и line type (`new` или `old`).
- Пользователь должен явно подтвердить отправку после просмотра текста.
- Dry-run должен пройти до реальной отправки.
- Для реальной отправки обязательны оба CLI-флага: `--send` и `--confirm-send`. `CONFIRM_SEND=YES` оставлен только для обратной совместимости.

## Токен

- Токен хранится только в `.env` / `.env.local` в корне mcp-tools.
- Файл должен иметь права `600`.
- Токен не должен попадать в чат, git, logs, body-файлы или error output.
- Нужен GitLab Personal Access Token с правом создавать MR notes. Обычно нужен scope `api`.

## Что Делать При Ошибках

- `.env обязателен` / env file not found: попроси пользователя локально создать `.env` (`cp .env.example .env`).
- `GITLAB_TOKEN is empty`: попроси пользователя локально заполнить token в env-файле.
- `401/403`: токен невалиден, не имеет scope или нет доступа к проекту.
- `404`: MR URL неверный или у пользователя нет доступа к проекту.
- `--send requires --confirm-send or CONFIRM_SEND=YES`: повторно проверь, было ли подтверждение пользователя.

## Границы

- CLI отправляет обычный комментарий к MR или line discussion на строке diff.
- Line discussion разрешён только после dry-run и явного подтверждения exact MR URL, body, file path, line number и line type.
- CLI не меняет статус MR, approvals, labels, assignees или branch.
