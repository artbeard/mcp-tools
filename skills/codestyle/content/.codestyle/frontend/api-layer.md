# Правила API-слоя

> JWT refresh, profile, 401-retry — docs auth проекта (SSO/JWT). Эталонный код (`refreshJwt`, `jwtStore`, `http.ts`) — **копировать из Guide**, не выдумывать. Сводка MUST: `../patterns/security.md`.

В **новых** проектах HTTP к **своему** `{apiHost}` — **ky**; **axios** — только legacy. Refresh JWT — **всегда нативный `fetch`** на `{authHost}/api/refresh-token` (не ky/axios — иначе зациклить interceptors/hooks).

## Архитектура
- **MUST** (новые проекты): один экземпляр **ky** с `hooks.beforeRequest` + `hooks.afterResponse` (§7 Guide).
- **MAY** (legacy): один **axios** с request/response interceptors; при доработках — миграция на ky.
- **MUST**: Модули Passport §7 — **те же имена и контракты** во всех проектах (копировать из docs auth проекта (SSO/JWT)):

| Файл | Назначение |
|------|------------|
| `src/config/endpoints.ts` | `authHost`, `apiHost`, `passportLogin`, `passportRefreshToken`, `passportLogout`, `profile` |
| `src/stores/jwt.ts` | `jwtStore`, `isExpiringSoon()` — in-memory JWT |
| `src/api/refreshTokenBody.ts` | `{ timeZoneOffset: new Date().getTimezoneOffset() }` |
| `src/api/refreshToken.ts` | `refreshJwt(force?)`, `startJwtExpiryWatcher()` — **только `fetch`** |
| `src/utils/resolveReturnUrl.ts` | `resolvePassportReturnUrl()` — для login redirect |
| `src/api/http.ts` | ky + JWT hooks (§7) |

Путь может быть `src/...` или `src/shared/...` — по конвенции репозитория; **логика и сигнатуры — как в Guide**.
- **MUST**: `refreshJwt()` без `force` — **no-op**, если JWT есть и до `exp` ≥ 1 мин (`isExpiringSoon()`); POST на Passport только при старте, proactive (<1 мин) или `refreshJwt(true)` после 401.
- **MUST**: Bearer — на запросы к `{apiHost}`; `credentials: 'include'` — на **refresh** (fetch) и на ky/axios к API (CORS cookie, если настроено).
- **MUST NOT**: Гасить ошибки ответа; пробрасывать `Promise.reject` / исходную ошибку.

## Файл эндпоинтов

Auth-ключи — **обязательны** (docs auth проекта §7). Бизнес-URL — по конвенции проекта.

```typescript
// src/config/endpoints.ts — authHost/apiHost: inferHosts + VITE_* (§7 Guide)
export default {
	passportLogin: `${authHost}/login?return=:returnUrl`,
	passportRefreshToken: `${authHost}/api/refresh-token`,
	passportLogout: `${authHost}/logout?return=:returnUrl`,
	profile: `${apiHost}/auth/profile`,
	products: `${apiHost}/api/goods`,
	productUpdate: `${apiHost}/api/goods/:id`,
};
```

## HTTP client (ky) — эталон

```typescript
// src/api/http.ts — полная версия: docs auth проекта §7 «HTTP client (ky)»
import ky from 'ky';
import { jwtStore } from '../stores/jwt';
import { refreshJwt } from './refreshToken';

const http = ky.create({
	credentials: 'include',
	hooks: {
		beforeRequest: [
			async (request) =>
			{
				await refreshJwt(); // proactive: POST только если нет jwt или exp < 1 мин
				request.headers.set('Authorization', `Bearer ${jwtStore.jwt}`);
			},
		],
		afterResponse: [
			async (request, options, response) =>
			{
				if (response.status !== 401 || options.context?.retried)
				{
					return response;
				}
				jwtStore.jwt = '';
				if (!(await refreshJwt(true)))
				{
					return response; // refreshJwt уже редиректит на Passport login
				}
				request.headers.set('Authorization', `Bearer ${jwtStore.jwt}`);
				return ky(request, {
					...options,
					context: { ...options.context, retried: true },
				});
			},
		],
	},
});

export default http;
```

Глобальная анимация загрузки — через `CustomEvent` на `window` (см. **Правила** ниже), не через правки этого файла без необходимости.

## Legacy Axios

Те же обязанности, что у ky. **`refreshJwt` — только через `fetch`**, не через этот axios.

```typescript
// src/use/axios.ts
import axios, { type AxiosRequestConfig as OriginalAxiosRequestConfig } from 'axios';
import { jwtStore } from '../stores/jwt';
import { refreshJwt } from '../api/refreshToken';

interface AxiosRequestConfig extends OriginalAxiosRequestConfig
{
	AxiosAnimation?: boolean;
	_retried?: boolean;
}

axios.interceptors.request.use(async (config) =>
{
	await refreshJwt(); // no-op, пока jwt валиден и exp ≥ 1 мин
	AxiosAnimateStart(config as AxiosRequestConfig);
	config.withCredentials = true;
	config.headers.Authorization = `Bearer ${jwtStore.jwt}`;
	return config;
});

axios.interceptors.response.use(
	(response) =>
	{
		AxiosAnimateStop(response.config as AxiosRequestConfig);
		return response;
	},
	async (error) =>
	{
		AxiosAnimateStop(error.config);
		const config = error.config as AxiosRequestConfig | undefined;
		if (error.response?.status === 401 && config && !config._retried)
		{
			config._retried = true;
			jwtStore.jwt = '';
			if (await refreshJwt(true))
			{
				config.headers.Authorization = `Bearer ${jwtStore.jwt}`;
				return axios(config);
			}
		}
		return Promise.reject(error);
	}
);
```

## Использование в сторах

```typescript
import http from '../api/http';
import endpoints from '../config/endpoints';

// showGlobalLoading / AxiosAnimation — флаг глобальной анимации (см. Правила)
const items = await http.get(endpoints.products, { context: { showGlobalLoading: true } }).json<IProduct[]>();
```

По статусам: успех — **2xx**; ky реджектит не-2xx без ручной проверки. Явно ветви **201** / **204** только где нужна бизнес-логика.

## Правила
- **MUST**: Bearer JWT в `Authorization` на запросы к `{apiHost}`.
- **MUST**: 401 от API → `refreshJwt(true)` → **один** retry → иначе Passport login (§10 Guide).
- **MUST NOT**: Несколько экземпляров ky/axios с дублирующими JWT-hooks.
- **MUST NOT**: Refresh-token через ky/axios.
- **SHOULD**: Глобальный индикатор — `CustomEvent` (`http:global-loading:on` / `off`), только при флаге запроса.
- **SHOULD**: URL-параметры: `endpoints.productUpdate.replace(':id', String(id))`.

```typescript
globalThis.window.addEventListener('http:global-loading:on', () => { /* включить */ });
globalThis.window.addEventListener('http:global-loading:off', () => { /* выключить */ });
```
