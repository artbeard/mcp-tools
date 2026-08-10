# Руководство по стилю кода

## Стек
Backend: PHP 8.4+ / Symfony 7.4+ / Doctrine ORM 2.17+ / MySQL
Frontend: Vue 3.4+ / TypeScript 5.x (strict) / Pinia 3.x / Vite / PrimeVue 4.* / Tailwind CSS / SCSS
Авторизация: Passport SSO + JWT verify (Lexik) — docs auth проекта (SSO/JWT) | Контейнеры: Docker | Асинхронность: Symfony Messenger | Realtime: Mercure

## Навигация

### Общие правила (весь код)
- [general/formatting.md](general/formatting.md) — табы, Allman-скобки, кодировка, переводы строк
- [general/naming.md](general/naming.md) — соглашения camelCase, PascalCase, snake_case
- [general/error-handling.md](general/error-handling.md) — правила исключений, политика try/catch
- [general/dependencies.md](general/dependencies.md) — процесс согласования, запрет самовольных добавлений
- [general/git-workflow.md](general/git-workflow.md) — ветки, MR, коммиты, ревью миграций
- [general/docker.md](general/docker.md) — docker-compose, php-fpm, общая сеть

### Backend (PHP / Symfony)
- [backend/INDEX.md](backend/INDEX.md) — навигация секции с помощником выбора

### Frontend (Vue 3 / TypeScript)
- [frontend/INDEX.md](frontend/INDEX.md) — навигация секции с помощником выбора

### Архитектурные паттерны
- [patterns/INDEX.md](patterns/INDEX.md) — сквозные паттерны и потоки данных

## Критичные правила (топ-7)
- **MUST**: Табы для отступов (4sp PHP/JS/TS, 3sp HTML/Twig)
- **MUST**: Allman-скобки — `{` на новой строке для if/func/loop (кроме стрелочных `=>`)
- **MUST**: Entity только через `make:entity`
- **MUST**: `<script setup lang="ts">` для всех новых Vue-компонентов
- **MUST**: `strict: true` в tsconfig.json
- **MUST NOT**: Обёртки `"success"` / `"status"` в API-ответах — HTTP-код = результат
- **MUST NOT**: Прямая инъекция репозитория в сервисы — использовать `EntityManagerInterface`

## Уровни приоритета (RFC 2119)
- **MUST** — обязательно, нарушение = MR отклоняется
- **MUST NOT** — запрещено, нарушение = критический баг
- **SHOULD** — рекомендуется, отклонение требует комментария с обоснованием
- **MAY** — опционально, на усмотрение разработчика

## Примечание о стиле скобок
Проект использует **Allman-стиль** (открывающая `{` на новой строке). Это отличается от PSR-12 (K&R-стиль). ESLint контролирует это через `'brace-style': ['error', 'allman']`. Все примеры в документации следуют Allman-стилю.

## Как использовать
1. Прочитай этот файл (~200 токенов)
2. Перейди к нужной секции INDEX.md (~300 токенов)
3. Загрузи только конкретный файл правил (~500-1000 токенов)
4. Следуй приоритету MUST > SHOULD > MAY
