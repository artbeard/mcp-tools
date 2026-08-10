# Паттерны безопасности

> **Канонический источник по интеграции Passport:** docs auth проекта (SSO/JWT) — актуальные контракты, эндпоинты, env и чек-лист. Этот файл — сжатое резюме для codestyle; при расхождении приоритет у docs auth проекта.

## Архитектура: Passport SSO + JWT

**Passport** (`auth.example.ru` / `auth-dev.example.ru` / `passport.local`) — централизованный SSO. Он логинит пользователя, хранит session cookie (`PHPSESSID`) и выдаёт JWT через `POST /api/refresh-token`.

**Backend проекта** — только **verify** JWT (Lexik JWT Authentication Bundle, RS256, `public.pem` из support/IT). **Не подписывает** JWT и **не** проксирует login/refresh.

| Что | Где | Назначение |
|-----|-----|------------|
| JWT | `Authorization: Bearer` | **Только аутентификация** — claim `username` |
| Роли | `GET /auth/profile` → поле `roles` из view | **Единственный источник** для guards и UI |
| Claim `roles` в JWT | — | **Deprecated** — в новом коде не использовать |

## Backend

> Эталон Symfony: docs auth проекта (SSO/JWT). Обязательные артефакты:

| Артефакт | Назначение |
|----------|------------|
| `config/jwt/public.pem` | Verify-only RS256 |
| `config/packages/lexik_jwt_authentication.yaml` | Только `public_key`; без private key Passport |
| `config/packages/security.yaml` | Firewall `jwt: ~` на `^/(api\|auth)/` |
| `Entity/User` (read-only view) | `USER_VIEW` + `DB_MP_SCHEMA` через env |
| `AuthController::profile` | `GET /auth/profile` |

### Verify-only (Lexik)
- **MUST**: `JWT_PUBLIC_KEY` → `public.pem` (verify-only; без private key Passport)
- **MUST**: User entity — read-only view (`passport__view_users` по умолчанию; схема через `DB_MP_SCHEMA`, не hardcode `mp.`)
- **MUST**: `GET /auth/profile` — минимум `id`, `username`, `roles`, `user_role` из view
- **MUST**: Guards по листовым ролям `ROLE_CAN_USE_*` из `User::getRoles()` (view), не из JWT payload
- **MUST NOT**: `role_hierarchy` в `security.yaml` клиента — view.roles уже развёрнут Passport'ом
- **MUST NOT**: JWT в query string (`?bearer=`, `?token=`)

### Получение текущего пользователя
```php
#[Route('/auth/profile', methods: ['GET'])]
public function profile(#[CurrentUser] ?User $user): JsonResponse
{
	if (null === $user || $user->getStatus() !== 1)
	{
		return $this->json(['message' => 'Unauthorized'], Response::HTTP_UNAUTHORIZED);
	}

	return $this->json([
		'id'        => $user->getId(),
		'username'  => $user->getUsername(),
		'roles'     => $user->getRoles(),   // из view, не из JWT
		'user_role' => $user->getUserRole(),
	]);
}
```

### Ответы 401 (JWT)
| Ситуация | Сообщение |
|----------|-----------|
| Просрочен | `Your token is expired, please renew it.` |
| Невалиден | `Your token is invalid, please login again to get a new one` |

## Frontend

### JWT in-memory + refresh через Passport

Обязательные frontend-модули — таблица в `../frontend/api-layer.md` (имена файлов как в Guide §7).
- **MUST**: JWT — **только in-memory** (отдельный `jwtStore`); **MUST NOT**: localStorage / cookies для JWT
- **MUST**: Refresh — `POST {authHost}/api/refresh-token` через **нативный `fetch`** (не ky/axios — иначе зациклить hooks)
- **MUST**: Каждый refresh: `credentials: 'include'` + body `{ timeZoneOffset: new Date().getTimezoneOffset() }`
- **MUST**: `refreshJwt()` без `force` — no-op, пока JWT валиден и до `exp` ≥ 1 мин; POST на Passport только при отсутствии JWT, proactive (<1 мин) или `refreshJwt(true)` после 401
- **MUST**: Bearer на запросы к **своему** API — в hooks **ky** (новый код) или interceptors **axios** (legacy); см. `../frontend/api-layer.md`
- **MUST**: После получения JWT — `GET {apiHost}/auth/profile`; роли UI/guards — **только из profile**
- **MUST**: Failed refresh → `location.replace({authHost}/login?return=` + `resolvePassportReturnUrl(...)`) — см. Guide §7
- **MUST**: Logout → `{authHost}/logout?return=...`

### 401 от своего API (ky)
```
afterResponse 401 → сброс jwt → refreshJwt(force=true) → один retry → иначе redirect на Passport login
```
Детали и эталонный код — §7 и §10 в docs auth проекта (SSO/JWT).

## Ролевой доступ

### Backend
- Symfony Security: `@IsGranted('ROLE_CAN_USE_*')` — листовые роли из view
- Роли нового проекта создаются в **SSO admin** до `@IsGranted` (§2.2 docs auth проекта)

> **Legacy:** широкие роли (`ROLE_ADMIN`, `ROLE_MANAGER`, …) — устаревшая модель; для новых модулей — только `ROLE_CAN_USE_*` / `ROLE_CAN_EDIT_*` / `ROLE_CAN_MODERATE_*`.

### Frontend
- Пункты меню фильтруются по `roles` из **profile** (см. `../frontend/routing.md` → Система меню)
- Гарды маршрутов — наличие profile / `meta.roles` из profile, не `jwt-decode` (см. `../frontend/routing.md` → Гарды маршрутов)
- **MUST**: Публичные маршруты — `meta.public: true`; остальные требуют загруженного profile

## Правила
- **MUST**: JWT — только аутентификация; авторизация — из `GET /auth/profile`
- **MUST**: `withCredentials: true` (**axios**) / `credentials: 'include'` (**ky** на API; на refresh — обязательно)
- **MUST**: Возвращать 401 при отсутствии/невалидном JWT; 403 при недостаточной роли
- **MUST NOT**: Хранить JWT в localStorage
- **MUST NOT**: Брать `roles` из JWT payload или `jwt-decode`
- **MUST NOT**: Передавать чувствительные данные в URL-параметрах

## Онбординг нового проекта
Перед кодом — заявка в инфраструктуру (CORS + `SESSION_DOMAIN` на Passport) и создание ролей в SSO admin. Шаблон и чек-лист — §2 и §14 docs auth проекта (SSO/JWT).
