# Гибридная архитектура Vue 3: `shared` + `domain`

## Архитектура

Проект на **Vue 3**, в котором исходники делятся на два опорных слоя — **`shared`** (переиспользуемый код **без** бизнес-сущностей и **без** импортов из `domain`) и **`domain`** (бизнес-модули: компоненты, сторы, API, типы; **наружу** — только через публичный **`index.ts`**). Поверх них — **`app/`** (bootstrap, глобальный router) и **`pages/`** (тонкие страницы под маршруты: **без** бизнес-логики, только компоновка и вызов доменных компонентов). **Pinia** и **Vue Router** остаются стандартными; меняется **раскладка папок** и **правила импорта**. Между доменами зависимости **однонаправленные**, **без** циклических импортов (детали — в конце документа). Цель — снизить связность при росте числа фич (в т.ч. в крупных CRM-подобных приложениях).

## Применение

**Политика использования:** **новые** проекты **всегда** строим на архитектуре из этого документа. **Существующие** проекты **не обязаны** целиком переезжать на неё (**опционально**): глобальный рефакторинг — только по необходимости или **явной** постановке; при этом новый код **желательно** сразу класть в **`shared`** + **`domain`**.

- **MUST** (новые проекты): раскладка **`shared`** + **`domain`** (+ **`pages`**, **`app`**) — как ниже, без отступлений.
- **MAY** (существующие проекты, в целом): оставить прежнюю структуру папок; тотальный перенос на гибридную схему не требуется кодстайлом.
- **SHOULD** (существующие проекты, новые модули): оформлять в стиле **`shared`** + **`domain`**; старые области трогать только при доработке этой зоны или по отдельной задаче на рефакторинг.

## Структура каталогов

```bash
src/
│
├── app/                      # глобальная инициализация приложения
│   ├── App.vue
│   ├── main.ts
│   └── router.ts             # глобальная конфигурация роутера (связывает pages)
│
├── pages/                    # страницы – только роутинг (по возможности "тонкие")
│   ├── dashboard/
│   │   └── index.vue
│   └── billing/
│       └── invoices.vue
│
├── shared/                   # абсолютно переиспользуемый, независимый от бизнес-логики слой
│   ├── api/                  # HTTP-слой Passport §7 (http.ts, refreshToken.ts, …)
│   │   ├── http.ts
│   │   ├── refreshToken.ts
│   │   └── refreshTokenBody.ts
│   ├── stores/
│   │   └── jwt.ts            # in-memory JWT (Passport §7)
│   ├── utils/
│   │   └── resolveReturnUrl.ts
│   ├── components/           # UI‑кит (кнопки, инпуты, модалки, тосты)
│   │   ├── AppButton.vue
│   │   ├── AppInput.vue
│   │   └── AppModal.vue
│   ├── composables/          # универсальные хуки (без бизнес-знаний)
│   │   ├── useLocalStorage.ts
│   │   ├── useDebounce.ts
│   │   └── useMediaQuery.ts
│   ├── utils/                # чистые утилиты (формат даты, currency, валидация)
│   │   ├── formatDate.ts
│   │   └── validateEmail.ts
│   ├── types/                # общие типы/интерфейсы (инфраструктурные)
│   │   └── global.d.ts
│   └── config/               # endpoints.ts: authHost, apiHost, passport* (Passport §7)
│       └── endpoints.ts
│
├── domain/                   # бизнес-домены (каждый – самодостаточный модуль)
│   │
│   ├── auth/                 # домен сессии: profile, роли, logout (login — на Passport SSO)
│   │   ├── composables/
│   │   │   └── useAuth.ts
│   │   ├── stores/
│   │   │   └── authStore.ts     # profile из GET /auth/profile
│   │   ├── types/
│   │   │   └── auth.types.ts    # UserProfile
│   │   └── index.ts
│   │
│   ├── billing/              # домен платежей и подписок
│   │   ├── components/
│   │   │   └── SubscriptionCard.vue
│   │   ├── composables/
│   │   │   └── useBilling.ts
│   │   ├── stores/
│   │   ├── types/
│   │   └── services/
│   │
│   └── users/                # домен пользователей (профили, списки)
│       ├── components/
│       │   └── UserAvatar.vue
│       ├── composables/
│       └── stores/
│
└── assets/                   # статика (стили, изображения)
    └── main.css
```

- **MUST**: В **`pages/`** нет бизнес-логики — только компоновка и делегирование доменным компонентам.
- **MUST NOT**: Локальные страницы login/register и формы с POST credentials — SSO только через Passport (docs auth проекта (SSO/JWT)).

## Правила взаимодействия слоёв

| Слой        | Что может?                                                                 | Что запрещено?                                    |
|-------------|----------------------------------------------------------------------------|---------------------------------------------------|
| `pages/`    | Импортировать из `shared/` и `domain/*/index.ts`. Рендерить доменные компоненты. | Не должна содержать сложной логики, прямых API-вызовов, работы со сторами. |
| `domain/*/` | Импортировать из `shared/`, из других доменов (через их публичный API). Может сам регистрировать свои сторы. | Не должен зависеть от `pages/`. Циклические зависимости между доменами – под строгим контролем. |
| `shared/`   | Импортировать только из `shared/` (не из `domain`). Не имеет понятия о бизнес-объектах. | Не содержит бизнес-терминологии (User, Invoice, Auth). Не вызывает доменные стор-экшены. |

## Примеры

### Публичный API домена (`domain/auth/index.ts`)

```ts
// domain/auth/index.ts
export { useAuth } from './composables/useAuth';
export { useAuthStore } from './stores/authStore';
export type { UserProfile } from './types/auth.types';
```

### Доменный стор (Pinia)

> SSO через Passport: JWT — in-memory в `jwtStore` (см. docs auth проекта (SSO/JWT)); auth store хранит **profile** из `GET /auth/profile`, не token.

```ts
// domain/auth/stores/authStore.ts
import { defineStore } from 'pinia';
import type { UserProfile } from '../types/auth.types';
import http from '@shared/api/http';
import endpoints from '@shared/config/endpoints';

export const useAuthStore = defineStore('auth', {
  state: () => ({
    profile: null as UserProfile | null,
  }),
  actions: {
    async init() {
      this.profile = await http.get(endpoints.profile).json<UserProfile>();
    },
    logout() {
      const returnUrl = encodeURIComponent(window.location.href);
      window.location.href = endpoints.passportLogout.replace(':returnUrl', returnUrl);
    },
  },
  getters: {
    isAuthenticated: (state) => state.profile !== null,
    hasRole: (state) => (role: string) => state.profile?.roles.includes(role) ?? false,
  },
});
```

### Доменный композабл

```ts
// domain/auth/composables/useAuth.ts
import { useAuthStore } from '../stores/authStore';
import { storeToRefs } from 'pinia';

export function useAuth() {
  const store = useAuthStore();
  const { profile, isAuthenticated } = storeToRefs(store);

  return {
    profile,
    isAuthenticated,
    hasRole: store.hasRole,
    logout: store.logout,
  };
}
```

### Главная точка входа (`main.ts`)

> Passport §8: `auth.init()` → `GET /auth/profile` через `http.ts`; JWT refresh — в `http.beforeRequest`, не отдельно в main.

```ts
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import App from './app/App.vue';
import router from './app/router';
import { useAuthStore } from '@domain/auth';

const app = createApp(App);
app.use(createPinia());
app.use(router);

const auth = useAuthStore();
auth.init()
  .then(() => app.mount('#app'))
  .catch(() => { /* refreshJwt уже редиректит на Passport login */ });
```

### Регистрация роутера

```ts
// app/router.ts — без локального /login; failed auth → Passport SSO
import { createRouter, createWebHistory } from 'vue-router';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('../pages/dashboard/index.vue'),
      meta: { title: 'Dashboard' },
    },
    {
      path: '/billing/invoices',
      component: () => import('../pages/billing/invoices.vue'),
      meta: { title: 'Счета', roles: ['ROLE_CAN_USE_BILLING'] },
    },
  ],
});

export default router;
```

### Автоимпорты (опционально, Vite)

Настройка `vite.config.ts` для автоматического импорта из `shared` и `domain`:

```ts
import AutoImport from 'unplugin-auto-import/vite';
import Components from 'unplugin-vue-components/vite';

export default defineConfig({
  plugins: [
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia'],
      dirs: ['./src/shared/composables', './src/domain/*/composables'],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      dirs: [
        './src/shared/components',
        './src/domain/*/components',
      ],
      deep: true,
      dts: 'src/components.d.ts',
    }),
  ],
});
```

Теперь компоненты из `shared/components` и `domain/*/components` (например `AppButton`) можно использовать без явного импорта.



## Междоменные импорты сторов без циклов

**SHOULD**: считать циклические импорты между доменами главным риском

Ниже — паттерн **однонаправленной** иерархии (Vue 3 + Pinia + TypeScript).

---

## Паттерн «только в одном направлении»: иерархия доменов

**Идея:** Установить явный порядок доменов: домены нижнего слоя не знают о верхних. Например:
- `shared` — самый низ (не знает ни о каких доменах)
- `domain/core` — базовые домены (auth, users) – не знают о прикладных (`billing`, `notifications`)
- `domain/feature` — прикладные домены могут импортировать из `core`

**Пример структуры:**

```
domain/
├── core/               # не зависят друг от друга и от фич
│   ├── auth/
│   ├── users/
│   └── organizations/
└── features/           # могут импортировать из core, но не друг из друга
    ├── billing/
    ├── notifications/
    └── dashboard/
```

**Правило импорта:**
- `features/billing/stores/billingStore.ts` может импортировать `useAuthStore` из `core/auth`
- `core/auth` **НЕ** импортирует ничего из `features/`

**Код:**

```ts
// domain/core/auth/stores/authStore.ts — profile + hasRole (Passport §8)
export const useAuthStore = defineStore('auth', {
  state: () => ({ profile: null as UserProfile | null }),
  getters: {
    hasRole: (state) => (role: string) => state.profile?.roles.includes(role) ?? false,
    isPremium: (state) => state.profile?.roles.includes('ROLE_CAN_USE_PREMIUM') ?? false,
  },
});

// domain/features/billing/stores/billingStore.ts
import { useAuthStore } from '@domain/core/auth'; // разрешённый импорт «вверх по иерархии»

export const useBillingStore = defineStore('billing', {
  actions: {
    async generateInvoice() {
      const authStore = useAuthStore();
      if (!authStore.isPremium) throw new Error('Only premium users');
      // ...
    },
  },
});
```

---
