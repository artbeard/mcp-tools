# Правила TypeScript

## Конфигурация
- **MUST**: `<script setup lang="ts">` во всех Vue-компонентах
- **MUST**: `strict: true` в tsconfig.json (все проекты)
- **MUST**: `target: "ESNext"`, `module: "ESNext"`, `moduleResolution: "bundler"`
- **MUST**: Алиас путей: `@/*` → `src/*`

```json
{
	"compilerOptions": {
		"target": "ESNext",
		"module": "ESNext",
		"strict": true,
		"moduleResolution": "bundler",
		"jsx": "preserve",
		"resolveJsonModule": true,
		"isolatedModules": true,
		"esModuleInterop": true,
		"skipLibCheck": true,
		"noEmit": true,
		"baseUrl": ".",
		"paths": { "@/*": ["src/*"] }
	}
}
```

Примечание: некоторые legacy-репы имеют `strict: false` — мигрировать инкрементально (2-3 файла за MR).
там же линтер довольно жесткий к типам, скорее всего пройдет почти все, можно попробовать `strict: true`

## Интерфейсы и типы
- **MUST**: Определять интерфейсы для состояния стора (`IState`), API-ответов, props компонентов
- **SHOULD**: Префикс `I` для интерфейсов (`IProduct`, `ITrigger`, `IFilter`)
- **SHOULD**: Префикс `T` для типов (`TRealtimeTooltipParamName`)
- **MUST**: Общие типы размещать в `src/types/`
- **MAY**: Типы по фичам в подпапке: `src/types/realtime/`

## Безопасность типов
- **MUST**: Типизированные рефы: `ref<string>('')`, `ref<User | null>(null)`
- **MUST**: Типизированные рефы компонентов: `ref<InstanceType<typeof Modal> | null>(null)`
- **MUST NOT**: Использовать **`any`**; для неизвестной формы данных — **`unknown`**, затем сужение или валидация на границе API (type guards, схема).
- **SHOULD**: Discriminated unions для вариантов с разной формой (`ApiResult`, шаги мастера).
- **SHOULD**: Вместо TypeScript **`enum`** — объект с **`as const`** и выведенным union-типом значений, когда литералы нужны в рантайме (запросы, стор, сравнения).

```typescript
// ПРАВИЛЬНО: discriminated union
type ApiResult<T> =
	| { ok: true; data: T }
	| { ok: false; error: string };

// ПРАВИЛЬНО: const-объект вместо enum + человекочитаемые лейблы для UI / фильтров / тостов
const OrderStatus = {
	Draft: 'draft',
	Exported: 'exported',
	Approved: 'approved',
} as const;
type OrderStatus = (typeof OrderStatus)[keyof typeof OrderStatus];

const OrderStatusLabel: Record<OrderStatus, string> = {
	[OrderStatus.Draft]: 'Черновик',
	[OrderStatus.Exported]: 'Экспортирован',
	[OrderStatus.Approved]: 'Согласован',
};
```

- **MUST**: Для каждого такого перечисления, которое **видит пользователь** (подписи в форме, таблице, фильтре, уведомлении), держать **одну карту лейблов** рядом с константой (`Record<Value, string>` или `readonly { value: OrderStatus; label: string }[]` для `Select`), без дублирования «магических» русских строк отдельно от значения `'draft'`.
- **SHOULD**: Литералы статусов/типов брать только из **константы** (`OrderStatus.Draft`), отображаемый текст — только из **`OrderStatusLabel`** (или колонки `label` в массиве опций).

## Пример
```typescript
// src/interfaces/OrderCard/IOrder.ts
export interface IOrderItem
{
	id: number;
	vendorCode: string;
	productId: number;
	status: number;
	mp: string;
	price: number | null;
}

interface IState
{
	orders: IOrderItem[];
	filter: {
		status: number | null;
		mp: string | null;
	};
}
```

## Обработка null, unknown и optional chaining

### `unknown` и сужение
```typescript
// MUST: сужай unknown/null типы перед использованием
function process(data: unknown): string
{
	if (typeof data !== 'string')
	{
		throw new Error('Expected string');
	}
	return data.toUpperCase();
}
```

### Optional chaining (`?.`) и `??`
- **MAY**: Использовать **`?.`** и **`??`** в **шаблоне** для подстановки текста и плейсхолдеров, когда отсутствие значения **нормально по продукту** (например `{{ user?.profile?.name ?? '—' }}`).
- **MAY**: В скрипте — только там, где поле **по контракту опционально** (DTO, необязательный шаг), и ветка «нет значения» явно обработана.
- **MUST NOT**: Замаскировать **`?.`-ом** данные, **обязательные** для текущего шага бизнес-логики: если значение **должно** быть к этому моменту — проверка, **`throw`**, `assert` или сужение до не-optional; отсутствие — **ошибка**, а не тихий `undefined`.

```typescript
// Допустимо для отображения
const title = row?.meta?.title ?? 'Без названия';

// Недопустимо как замена инварианта «invoice уже загружен»
// if (!invoice) throw new Error('...'); затем работа с invoice.lines без ?.
```

## Ответы API и type guards
- **SHOULD**: На границе **API-слоя** (или сразу после ответа доменной функции) проверять фактическую форму JSON **type guard**-ом (`function isGoods(x: unknown): x is IGoods`), схемой (**Zod** / аналог) или парсером — даже при типизированном DTO на бэке: сеть и прокси не гарантируют тело.
- **SHOULD**: После успешной проверки работать дальше с **суженным** типом, без повторных `as` и без «доверия строке».

```typescript
function isGoodsList(x: unknown): x is IGoods[]
{
	return Array.isArray(x) && x.every((row) => typeof row === 'object' && row !== null && 'id' in row);
}
```

## Антипаттерн
```typescript
// НЕПРАВИЛЬНО: неявный / явный any
const data = ref();              // Нет типа → выводится как any
const items: any[] = [];         // Используй unknown[] + сужение
function process(x) { }         // Нет типов параметров / подразумеваемый any

// НЕПРАВИЛЬНО: обход системы типов
const val = obj as unknown as T; // Двойное приведение

// НЕПРАВИЛЬНО: optional chaining вместо инварианта «объект уже есть»
function total(invoice: IInvoice | undefined): number
{
	return invoice?.lines?.reduce((s, l) => s + l.price, 0) ?? 0; // если invoice обязан быть — сначала assert/throw
}
```
