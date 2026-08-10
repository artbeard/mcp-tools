# Codestyle bootstrap — критичные правила стека

Универсальные соглашения для **PHP/Symfony** и **Vue/TypeScript** (не привязаны к конкретной компании). Используй MCP tools `codestyle_find_rules` и `codestyle_read_rule`.

## Обязательный workflow

**Перед написанием или изменением кода на этом стеке:**

1. Вызови `codestyle_find_rules` с описанием задачи (ru/en). Писать «по памяти» / по общим best practices нельзя — эти правила отличаются от PSR-12 и популярных конвенций.
2. Загрузи через `codestyle_read_rule(uri)` все матчи с `priority="critical"` (обычно 2–3 файла). Только релевантное.
3. Соблюдай уровни RFC 2119: `MUST` / `MUST NOT` = обязательно.

## ТОП-7 критичных правил

1. **Табы** для отступов: 4sp в PHP/JS/TS/SCSS, 3sp в HTML/Twig. Пробелы запрещены.
2. **Allman-скобки** — `{` на новой строке для `if`/`for`/`while`/`foreach`/`function`/класса. Не K&R, не PSR-12.
3. **Entity только через `make:entity`** — не пишем сущности руками. Plain int FK без `ManyToOne`/`JoinColumn`.
4. **`<script setup lang="ts">`** для всех новых Vue-компонентов. Options API в новом коде запрещён.
5. **`strict: true`** в `tsconfig.json`. Никакого `any`. Интерфейсы префиксуем `I*`.
6. **MUST NOT: success/status-обёртки** в API-ответах. HTTP-код = результат.
7. **MUST NOT: прямая инъекция Repository** в сервисы — `EntityManagerInterface`, репозиторий через `$em->getRepository(Entity::class)`.

## Пример

Задача: «Создать API-эндпоинт POST /api/orders».

```
→ codestyle_find_rules({ task: "новый Symfony контроллер с DTO" })
→ codestyle_read_rule("codestyle://backend/controller")
→ codestyle_read_rule("codestyle://backend/dto")
→ codestyle_read_rule("codestyle://patterns/crud-flow")
→ код по прочитанным правилам
```

## Стек

- Backend: PHP 8.4+ / Symfony 7.4+ / Doctrine ORM / MySQL
- Frontend: Vue 3.4+ / TypeScript strict / Pinia / Vite / PrimeVue / Tailwind / SCSS
- Auth: JWT (часто Lexik) + централизованный SSO при наличии; асинхронность: Symfony Messenger
