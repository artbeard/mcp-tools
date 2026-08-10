# Новая Entity и миграция

Сценарий: добавляешь новую сущность Doctrine ORM с миграцией БД.

## Обязательные правила (URI)

- `codestyle://backend/entity` — `make:entity`, `Types::*`, plain int FK, обязательные комментарии полей
- `codestyle://backend/database` — Doctrine-миграции только для DDL, формат миграций
- `codestyle://general/git-workflow` — раздел про ревью миграций

## Опционально

- `codestyle://backend/repository` — если нужны кастомные методы поиска
- `codestyle://patterns/data-flow` — для понимания полного жизненного цикла данных

## Workflow

1. Создать Entity: `php bin/console make:entity` (только так, не руками).
2. Заполнить поля, комментарии (`#[ORM\Column(options: ['comment' => '...'])]`), `Types::*`.
3. Связи — `plain int` FK, без `ManyToOne`/`JoinColumn`. Join делаем в QueryBuilder/SQL.
4. Сгенерировать миграцию: `php bin/console doctrine:migrations:diff`.
5. Проверить миграцию: только DDL (`CREATE`/`ALTER`/`DROP`). Никаких `INSERT`/`UPDATE`/`DELETE` бизнес-данных — это антипаттерн.
6. Прогнать локально: `php bin/console doctrine:migrations:migrate`.

## Чек-лист

- [ ] Entity создана через `make:entity`, не руками.
- [ ] Все поля имеют `comment` в `#[ORM\Column]`.
- [ ] FK как `int` колонки, без `ManyToOne` / `JoinColumn`.
- [ ] Миграция содержит ТОЛЬКО DDL.
- [ ] Имя миграции и schema соответствует проектному префиксу (`pf_project__`, `manager_project__`, `mp_core__`, `wb_core__`, `ozon_core__`).
- [ ] Поля nullable где это бизнес-валидно, без надуманных `default`.
