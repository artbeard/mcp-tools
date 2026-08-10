# Правила Pinia-сторов

## Определение
- **MUST**: Для новых сторов использовать setup-store синтаксис `defineStore('name', () => { ... })`
- **MAY**: Options-store синтаксис допускается в legacy-коде; при существенном изменении переводить на setup-store
- **MUST**: Имя стора совпадает с файлом: `useProductStore` в `useProductStore.ts`
- **MUST**: Именованный экспорт: `export const useProductStore = defineStore(...)`

## State
- **MUST**: State в setup-store описывать типизированными `ref` / `shallowRef` / `reactive`
- **MUST**: Для сложной формы состояния определить отдельный интерфейс или тип
- **MUST NOT**: Хранить вычисляемые значения в state — используй `computed`

```typescript
interface IProductQuery {
	page: number;
	limit: number;
	status: string | null;
	mp: string | null;
}

export const useProductStore = defineStore('productStore', () =>
{
	const items = ref<IProduct[]>([]);
	const loading = ref<boolean>(false);
	const query = ref<IProductQuery>({ page: 0, limit: 50, status: null, mp: null });
	const totalCount = computed<number>(() => items.value.length);

	return {
		items,
		loading,
		query,
		totalCount,
	};
});
```

## Actions
- **MUST**: Асинхронные экшены объявлять через `async` и явно типизировать результат `Promise<T>`
- **MUST**: При неожиданном статусе или невалидном ответе бросать `Error`; не возвращать `undefined` как успешный результат
- **MUST**: Ошибки пробрасываются вызывающему — без тихих catch
- **MUST**: Использовать `await` вместо `new Promise` и `.then()`-цепочек

```typescript
const preload = async (reload: boolean = false): Promise<IProduct[]> =>
{
	if (items.value.length > 0 && !reload)
	{
		return items.value;
	}

	loading.value = true;

	try
	{
		const response = await axios.get<IProduct[]>(url.products, { AxiosAnimation: true } as AxiosRequestConfig);
        // Это антипаттерн - проверка статусов должна быть на уровне клиента, статусы проверять только если
        // на них завязано поведеление бизнесс логики
		if (response.status !== 200)
		{
			throw new Error(`API returned ${response.status}`);
		}

		items.value = response.data;

		return response.data;
	}
	finally
	{
		loading.value = false;
	}
};
```

## Computed
- **MUST**: Использовать `computed` для всех вычисляемых значений в setup-store
- **MUST NOT**: Дублировать вычисляемые данные в state

```typescript
const filteredItems = computed<IProduct[]>(() =>
{
	return items.value.filter((item: IProduct): boolean =>
	{
		if (query.value.status !== null && item.status !== query.value.status) return false;
		if (query.value.mp !== null && item.mp !== query.value.mp) return false;
		return true;
	});
});

return {
	filteredItems,
	totalCount,
	preload,
};
```

## Использование стора в компонентах
- **MUST**: Инициализировать стор на верхнем уровне `<script setup>` (не внутри функций)
- **SHOULD**: Использовать `storeToRefs()` при деструктуризации реактивного состояния

```vue
<script setup lang="ts">
import { storeToRefs } from 'pinia';
import { useProductStore } from '@/store/useProductStore';

const store = useProductStore();
const { items, loading } = storeToRefs(store);  // Реактивные рефы

// Наблюдение за изменениями стора
watch(() => store.filteredItems, (newItems) =>
{
	console.log('Элементы изменились:', newItems.length);
});
</script>

<template>
	<p v-if="loading">Загрузка…</p>
	<ul v-else>
		<li v-for="item in items" :key="item.id">{{ item.name }}</li>
	</ul>
	<button type="button" @click="store.preload(true)">Обновить</button>
</template>
```

## Состояния загрузки
- **SHOULD**: Глобальный индикатор (прогресс-бар приложения) — флаг **`AxiosAnimation`** (legacy) или **`showGlobalLoading`** доменного API на **ky** и события **`http:global-loading:on` / `off`** на `window`; см. [api-layer.md](api-layer.md).
- **MAY**: `loading` в `state` для **локального** UI (inline-спиннер списка, кнопки «Сохранить»).
- **MUST NOT**: Один общий `loading: boolean` в сторе как единственный признак загрузки, если из стора уходит **несколько параллельных** запросов — без счётчика или явной модели флаги не отражают число активных вызовов.
- **SHOULD**: Когда запросов несколько, для спиннера конкретного действия в компоненте использовать **`await store.someAction()`** (экшен возвращает `Promise`) вместо гадания по общему `loading`

## Антипаттерн
```typescript
// НЕПРАВИЛЬНО: экшен без возврата Promise
const fetchUser = (id: number): void =>
{
	api.getUser(id).then((loadedUser: IUser): void =>
	{
		user.value = loadedUser;
	});
};

// НЕПРАВИЛЬНО: тихий успех при невалидном ответе
if (response.status !== 200) return [];

// НЕПРАВИЛЬНО: хранение вычисляемых данных
const totalCount = ref<number>(0);  // Должен быть computed!

// НЕПРАВИЛЬНО: деструктуризация state с store без storeToRefs — в шаблоне перестанет обновляться
const { items } = useProductStore();
```
