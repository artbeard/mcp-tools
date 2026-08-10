# Навигация по правилам Backend

Стек: PHP 8.4+ / Symfony 7.4+ / Doctrine ORM 2.17+ / MySQL

## Архитектура: 4 слоя
```
Controller → Service → DTO → Repository/Entity
```

## Файлы секции

- [controller.md](controller.md) — `#[Route]`, `#[MapRequestPayload]`, коды ответов, DI, формат API
- [service.md](service.md) — DI через конструктор, stateless, только EntityManager, пакетная обработка
- [dto.md](dto.md) — `final readonly class`, `fromArray()`, `fromEntity()`, DtoInterface
- [entity.md](entity.md) — только `make:entity`, `#[ORM\...]`, `Types::*`, комментарии полей, связи
- [repository.md](repository.md) — QueryBuilder, DQL, кастомные методы поиска, ServiceEntityRepository
- [database.md](database.md) — Heredoc SQL, `executeQuery`/`executeStatement`, справка `ParameterType::*`, миграции
- [modules.md](modules.md) — структура `src/Modules/`, изоляция, общий vs модульный код
- [commands.md](commands.md) — консольные команды, SymfonyStyle, `#[AsCommand]`
- [messenger.md](messenger.md) — Symfony Messenger, асинхронные сообщения, обработчики, транспорты

## Помощник выбора
- Создаёшь API-эндпоинт? → `controller.md` + `dto.md` + `service.md`
- Новая таблица в БД? → `entity.md` + `database.md`
- Бизнес-логика? → `service.md`
- Сырой SQL-запрос? → `database.md` (справка ParameterType)
- Новый модуль фичи? → `modules.md` + все вышеперечисленные
- Асинхронная обработка? → `messenger.md` + `commands.md`
- Auth / JWT / profile? → `../patterns/security.md` + docs auth проекта (SSO/JWT)
- Извлечение данных? → `repository.md`
- Связи сущностей? → `entity.md` (раздел «Связи»)
