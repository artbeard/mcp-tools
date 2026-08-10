# Правила стилей

## Фреймворк
- **MUST**: PrimeVue 4 как основной UI-фреймворк
- **MUST**: PrimeIcons (`pi pi-*`) как основной набор иконок
- **MUST**: Tailwind CSS как основной слой утилитарных классов и layout-стилей
- **SHOULD**: SCSS использовать только для кастомных стилей, которые не покрываются PrimeVue/Tailwind
- **MUST**: Атрибут `scoped` на стилях компонентов (кроме случаев когда нужны глобальные)

## Утилитарные классы
- **MUST**: Сначала использовать компоненты и props PrimeVue (`Button`, `InputText`, `Select`, `Drawer`, `Toast`, `ConfirmDialog`)
- **MUST**: Для сетки, flex, spacing, размеров и цвета сначала использовать Tailwind CSS (`grid`, `flex`, `gap-*`, `text-*`, `rounded-*`)
- **SHOULD**: Legacy-утилиты допустимы только при точечной правке существующего кода, если они уже подключены в проекте
- **MUST NOT**: Использовать Tabler-классы и Tabler Icons в новом коде
- **MAY**: Добавлять кастомные классы только для реальных пробелов в дизайн-системе

## Сетка
```html
<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
	<div>Левая</div>
	<div>Правая</div>
</div>
```

## Стили компонентов
```vue
<style scoped lang="scss">
.custom-card
{
	border: 1px solid var(--p-content-border-color);
	border-radius: var(--p-border-radius-md);
	padding: 1rem;
}
</style>
```

## Правила
- **MUST**: Табуляция (4 пробела) в SCSS
- **MUST**: Не уникальные стили общих компонентов выносить в preset темы PrimeVue
- **SHOULD**: CSS-переменные PrimeVue (`var(--p-*)`) вместо захардкоженных значений
- **SHOULD**: Значения Tailwind-темы использовать вместо локальных magic numbers
- **SHOULD**: БЕМ-нейминг для кастомных классов: `block__element--modifier`
- **MUST NOT**: Инлайн-стили в шаблонах (используй классы или computed)
- **MAY**: Пробрасывать вложенные стили PrimeVue компонентов через pt

## Иконки
```html
<i class="pi pi-box"></i>
```
- Источник: `primeicons`

## Большие таблицы данных
- **SHOULD**: Пагинация или виртуальный скролл — не 3000 строк за раз
- **SHOULD**: AgGrid для таблиц
