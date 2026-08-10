# Навигация по правилам Frontend

Стек: Vue 3.4+ / TypeScript 5.x (strict: true) / Pinia 3.x / Vite / PrimeVue 4.* / Tailwind CSS / SCSS

## Слои архитектуры
```
Router → Pages → Components → Stores → Composables → API Layer
```

## Файлы секции

- [component.md](component.md) — `<script setup lang="ts">`, props/emits, валидация форм (Vuelidate)
- [store.md](store.md) — Pinia `defineStore`, экшены возвращают `Promise`, `storeToRefs()`, состояния загрузки
- [composables.md](composables.md) — префикс `use*`, чистые vs реактивные, общие форматтеры
- [typescript.md](typescript.md) — `strict: true`, интерфейсы `I*`, нет `any`, discriminated unions
- [routing.md](routing.md) — Vue Router, ленивая загрузка, мета-заголовки, ролевое меню
- [structure.md](structure.md) — гибридная архитектура `shared` + `domain`, политика для новых/legacy проектов, правила слоёв
- [styles.md](styles.md) — PrimeVue, Tailwind CSS, SCSS, scoped, не изобретай CSS
- [api-layer.md](api-layer.md) — ky (новые проекты), legacy axios, JWT refresh/Passport, флаг глобальной загрузки (docs auth проекта (SSO/JWT))
- [eslint.md](eslint.md) — ESLint flat config, Allman-скобки, табы
- Хелперы без реактивности Vue держим отдельно от composables: `src/helpers/` или существующая проектная папка

## Помощник выбора
- Новая страница/view? → `routing.md` + `component.md`
- Новый компонент? → `component.md` + `styles.md`
- Архитектура модулей и слоёв? → `structure.md`
- Управление состоянием? → `store.md` + `api-layer.md`
- Переиспользуемая логика? → `composables.md`
- TypeScript-типы? → `typescript.md`
- Интеграция с API? → `api-layer.md` + `store.md`
- Авторизация / Passport / роли? → `../patterns/security.md` + docs auth проекта (SSO/JWT)
- Форма с валидацией? → `component.md` (раздел Vuelidate)
- Форматирование кода? → `eslint.md` + `styles.md`
