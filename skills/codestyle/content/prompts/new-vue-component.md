# Новый Vue 3 компонент

Сценарий: добавляешь новый компонент во Vue 3 / TypeScript / Pinia проект.

## Обязательные правила (URI)

- `codestyle://frontend/component` — `<script setup lang="ts">`, props/emits, Vuelidate для форм
- `codestyle://frontend/typescript` — strict: true, интерфейсы I*, никакого any
- `codestyle://frontend/styles` — PrimeVue, Tailwind CSS, SCSS scoped, не изобретать CSS

## Опционально

- `codestyle://frontend/store` — если компонент работает с Pinia-стором
- `codestyle://frontend/api-layer` — если делает прямые API-вызовы
- `codestyle://frontend/composables` — если выделяешь переиспользуемую логику в `use*`
- `codestyle://frontend/structure` — если непонятно куда положить файл (shared vs domain)

## Чек-лист перед коммитом

- [ ] `<script setup lang="ts">` — Options API запрещён.
- [ ] Props через `defineProps<{...}>()` или `withDefaults`, типы обязательны.
- [ ] Emits через `defineEmits<{(e: '...', ...): void}>()`.
- [ ] Имя файла — `PascalCase.vue`.
- [ ] Для UI использованы PrimeVue-компоненты, для layout/spacing — Tailwind CSS, кастомные стили в `<style scoped lang="scss">`.
- [ ] Allman-скобки в JS/TS. Табы 4sp.
- [ ] Если форма — Vuelidate-валидация, не самописная.
- [ ] Никакого `any`. Интерфейсы префиксованы `I*`.
