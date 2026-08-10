# Правила Composables

## Именование
- **MUST**: Префикс `use` для имён composables: `useFormatters`, `useNoteManager`
- **MUST**: Имя файла совпадает с экспортом: `useFormatters.ts`

## Расположение
- **SHOULD**: `src/composables/` (типичные frontend-репы) или `src/use/` (пример проекта)
- **SHOULD**: Один проект — одно соглашение. Смотри существующую структуру перед созданием
- **MAY**: Подпапка по фиче: `src/use/realtime/`, `src/composables/goods/`

## Разделение

### 1. Чистые helper-функции (без реактивности Vue)
```typescript
// src/helpers/date.ts
export function formatDate(dateStr?: string | null): string | null
{
	if (!dateStr || dateStr === '0000-00-00') return null;
	return new Date(dateStr).toLocaleDateString('ru-RU');
}

export function dateToISOFormat(date: Date, dateOnly: boolean = false): string
{
	const pad = (num: number): string => num.toString().padStart(2, '0');
	const Y = date.getFullYear();
	const m = pad(date.getMonth() + 1);
	const d = pad(date.getDate());
	if (dateOnly) return `${Y}-${m}-${d}`;
	return `${Y}-${m}-${d} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
```

### 2. Vue Composables (с реактивным состоянием)
```typescript
// useNoteManager.ts
import { ref, reactive } from 'vue';

export function useNoteManager()
{
	const notes = reactive<Map<string, NoteInstance>>(new Map());

	const show = (config: NoteConfig): string =>
	{
		const id = `note-${Date.now()}`;
		// ... создание уведомления
		notes.set(id, instance);
		return id;
	};

	const close = (id: string) =>
	{
		notes.get(id)?.app.unmount();
		notes.delete(id);
	};

	return { show, close, getNotes: () => Array.from(notes.values()) };
}
```

## Правила
- **MUST**: Чистые функции без `ref`, `computed`, `watch`, lifecycle hooks хранить в `src/helpers/`
- **MUST**: Именованные экспорты для helper-функций
- **MUST**: Возвращать объект из Vue composables
- **SHOULD**: Null-safe паттерны с ранним возвратом
- **SHOULD**: Использовать единые helper-модули форматтеров — без ad-hoc `toFixed`, форматирования дат
- **MUST NOT**: Терять реактивность через `JSON.parse(JSON.stringify(...))` — используй `structuredClone`

## Антипаттерн
```typescript
// НЕПРАВИЛЬНО: ad-hoc форматирование разбросано по кодобазе
const price = (val * 100).toFixed(2);  // Используй общий helper форматирования

// НЕПРАВИЛЬНО: потеря реактивности
const copy = JSON.parse(JSON.stringify(reactiveObj));  // Используй structuredClone
```
