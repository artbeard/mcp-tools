# Структура нового модуля

## Скаффолд бэкенд-модуля
```
src/Modules/{ModuleName}/
├── Controller/
│   └── {ModuleName}Controller.php
├── Service/
│   └── {ModuleName}Service.php
└── Dto/
    ├── Dto{ModuleName}.php
    └── Dto{ModuleName}Create.php
```

Плюс общие файлы:
```
src/Entity/{ModuleName}.php          # через make:entity
src/Repository/{ModuleName}Repository.php  # автогенерация
```

## Скаффолд фронтенд-модуля

**Соответствие `../frontend/structure.md`:** в **новых** проектах те же роли файлов кладутся в **`domain/{ModuleName}/`** (stores, components, types, `index.ts`) и тонкие **`pages/...`**; ниже — классическая раскладка для **legacy** и ориентир по именам.

```
src/pages/{ModuleName}/
└── {ModuleName}List.vue

src/components/{ModuleName}/
├── {ModuleName}Table.vue
├── {ModuleName}Form.vue
└── {ModuleName}Filter.vue

src/store/
└── use{ModuleName}Store.ts

src/interfaces/{ModuleName}/
└── I{ModuleName}.ts
```

> **Замечение**
> Модульная структура для фронта, возможно, немного изменится

## Запись маршрута
```typescript
{
	path: '/{module-name}',
	name: '{ModuleName}List',
	component: () => import('../pages/{ModuleName}/{ModuleName}List.vue'),
	meta: { title: '{Название модуля}' }
}
```

## Запись меню
```typescript
{
	name: '{Название модуля}',
	to: '/{module-name}',
	icon: 'pi pi-{icon}',
	roles: ['ROLE_CAN_USE_{MODULE}']
}
```

## Чеклист для нового модуля
1. `make:entity` → Entity + Repository
2. Создать миграцию → проверить SQL → запустить
3. Backend: Controller + Service + DTO в `Modules/`
4. Frontend: Store + Components + Types + Route
5. Добавить в меню с ролями `ROLE_CAN_USE_*` (создать в SSO admin до guards — §2.2 Guide)
6. Протестировать полный CRUD-флоу
