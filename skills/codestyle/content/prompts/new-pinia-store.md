# Новый Pinia store

Сценарий: добавляешь Pinia-стор для работы с состоянием и API.

## Обязательные правила (URI)

- `codestyle://frontend/store` — `defineStore`, actions возвращают Promise, `storeToRefs`
- `codestyle://frontend/api-layer` — ky (новые проекты) / axios (legacy), JWT, доменные модули
- `codestyle://frontend/typescript` — типизация state, getters, actions

## Опционально

- `codestyle://frontend/composables` — если часть логики выделяется в `use*`
- `codestyle://patterns/api-contract` — для сложного маппинга backend→frontend

## Чек-лист

- [ ] `defineStore('domain-name', () => { ... })` (composition API setup-стиля).
- [ ] State через `ref<IType>()` с явными типами.
- [ ] Actions возвращают `Promise<T>`, в catch реджектят с понятной ошибкой.
- [ ] Реджект на HTTP-статусе ≠ 200 (`response.status !== 200`).
- [ ] Компоненты используют `storeToRefs(store)` для реактивных полей.
- [ ] API-вызовы идут через `api-layer` модуль, а не прямо в стор.
- [ ] Никакого `any` в типах state/payload/response.
