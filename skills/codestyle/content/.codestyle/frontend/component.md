# Правила Vue-компонентов

## Структура SFC
```vue
<template>
	<!-- Шаблон первым -->
</template>

<script setup lang="ts">
	// Логика (Composition API)
</script>

<style scoped lang="scss">
	/* Стили последними, всегда scoped */
</style>
```

## Стиль скрипта
- **MUST**: Использовать `<script setup lang="ts">` для всех новых компонентов
- **MUST NOT**: Использовать Options API для нового кода
- **MAY**: Существующие `defineComponent` + `setup()` компоненты (legacy Options API) — рефакторить при касании
- **MUST**: Имена файлов компонентов в PascalCase: `StatusCommentPopover.vue`
- **MUST**: Объявлять имя компонета `defineOptions({ name: 'StatusCommentPopover' });`
- **SHOULD**: Локальные компоненты называть в 2+ слова, если имя может совпасть с PrimeVue или HTML (`AppButton`, `RequestDrawer`)
- **MAY**: Однословные имена страниц и legacy-компонентов допускаются, когда в проекте уже есть такое соглашение


## Props и Emits (типизированные)
```vue
<script setup lang="ts">
// Props с дефолтами
const props = withDefaults(defineProps<{
	title: string;
	count?: number;
	user?: IUser | null;
}>(), {
	count: 0,
	user: null
});

// Emits
const emit = defineEmits<{
	(e: 'update', value: string): void;
	(e: 'close'): void;
}>();

// Двустороннее связывание (Vue 3.4+ defineModel)
const modelValue = defineModel<string>({ default: '' });
</script>
```

## Реактивное состояние
```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useProductStore } from '@/store/useProductStore';

const store = useProductStore();
const commentText = ref<string>('');
const isVisible = ref(false);
const modalRef = ref<InstanceType<typeof Modal> | null>(null);

// Computed — не дублируй вычисляемое состояние
const hasComment = computed<boolean>(() => commentText.value.length > 0);

onMounted(() =>
{
	store.preload();
});
</script>
```

## Правила потока данных
- **MUST**: Данные из props, стора, provide/inject или composables — никогда из DOM
- **MUST NOT**: Прямые манипуляции с DOM через `document.*`
- **MUST**: Побочные эффекты только в `onMounted` / `onUnmounted`
- **MUST NOT**: Мутировать props напрямую — создай реактивную копию для редактирования
- **SHOULD**: Избегать props drilling: для доменного состояния лучше используй Pinia, для локального контекста глубокой ветки — можно типизированный `provide/inject`
- **SHOULD**: Использовать `structuredClone(toRaw(obj))` для глубоких копий (не `JSON.parse/stringify`)

## Декларативные шаблоны и условный рендер
- **MUST NOT**: Использовать `v-html` (риск XSS)
- **MUST NOT**: Конкатенация строк для CSS-классов — используй computed или объектный синтаксис
- **SHOULD**: Для стандартных controls, overlay, таблиц и форм использовать компоненты PrimeVue
- **SHOULD**: Для раскладки и мелких визуальных правок использовать Tailwind CSS утилиты
- **SHOULD**: По умолчанию — **декларативный** шаблон: `v-if` / `v-show`, `v-for`, привязки; условия и разметка остаются на уровне экрана, без лишней оркестрации в родителе.
- **MAY**: Если рендер достаточно сложен или логика фичи **объёмная** - не размазывай ее по родительскому компоненту. Вынеси фичу в **самостоятельный компонент** с **узким контрактом снаружи**.

## Стор в компоненте
- **MUST NOT**: Мутировать state стора из компонента (`push` в массив стора, прямое присвоение полям `ref` из state и т.п.) — только через **экшены** стора (см. [store.md](store.md)).
- **SHOULD**: Данные, которые **много раз** читаются из шаблона и представляют собой **проекцию state** без дополнительных входных параметров от компонента (фильтрация, поиск, агрегация), выносить в **`computed`** внутри стора («геттер»); в компоненте подключать через **`storeToRefs`**, не дублировать тот же `computed` в каждом потребителе.

## Валидация форм (Vuelidate)
```vue
<template>
	<InputText v-model="form.name" :invalid="v$.name.$error" />
	<small v-if="v$.name.$error" class="p-error">{{ v$.name.$errors[0]?.$message }}</small>
</template>

<script setup lang="ts">
import { useVuelidate } from '@vuelidate/core';
import { required, minLength } from '@vuelidate/validators';
import { reactive } from 'vue';

const form = reactive({ name: '', email: '' });
const rules = { name: { required, minLength: minLength(3) }, email: { required } };
const v$ = useVuelidate(rules, form);

const handleSubmit = async (): Promise<void> =>
{
	const valid = await v$.value.$validate();
	if (!valid) return;
	await store.createItem(form);
};
</script>
```
- **MUST**: Использовать Vuelidate для валидации форм (стандартная библиотека)
- **MUST**: Для PrimeVue-полей показывать ошибки через проп `:invalid`

## Антипаттерн
```vue
<!-- НЕПРАВИЛЬНО: Options API в новом компоненте -->
<script lang="ts">
export default defineComponent({  // Используй <script setup> вместо этого
	methods: { ... }
});
</script>

<!-- НЕПРАВИЛЬНО: императивная работа с DOM -->
<script setup>
document.getElementById('myDiv').innerHTML = data;  // Запрещено!
</script>

<!-- НЕПРАВИЛЬНО: мутация props -->
<script setup>
const props = defineProps<{ user: User }>();
props.user.name = 'new';  // Запрещено! Используй реактивную копию
</script>

<!-- НЕПРАВИЛЬНО: мутация стора вне экшена -->
<script setup lang="ts">
import { useProductStore } from '@/store/useProductStore';

const store = useProductStore();
store.items.push({ id: 0, name: 'x' }); // только store.addItem(...) или экшен
</script>
```
