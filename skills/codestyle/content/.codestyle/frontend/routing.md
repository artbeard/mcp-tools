# Правила Vue Router

## Конфигурация
- **MUST**: Централизованно в `src/router/router.ts`
- History mode: `createWebHistory()` (чистые URL)
- managerfront: `createWebHashHistory()` (hash URL)

## Определение маршрутов
```typescript
const routes: RouteRecordRaw[] = [
	{
		path: '/',
		component: Dashboard,
		meta: { title: 'Dashboard' }
	},
	{
		path: '/goods-list',
		name: 'GoodsList',
		component: () => import('../pages/Goods/GoodsList.vue'),
		meta: { title: 'Товары' }
	},
	{
		path: '/config/goods-charc/:niche_id(\\d+)',
		name: 'GoodsCharc',
		component: () => import('../pages/Config/GoodsCharc.vue'),
		meta: { title: 'Характеристики' }
	}
];
```

## Правила
- **MUST**: Ленивая загрузка страничных компонентов: `() => import('../pages/...')`
- **MUST**: Устанавливать `meta.title` для каждого маршрута
- **SHOULD**: Регулярные выражения для валидации параметров: `:id(\\d+)`
- **SHOULD**: Именованные маршруты для программной навигации

## Система меню
```typescript
export const menuItemList = [
	{
		name: 'Товары',
		to: '/goods-list',
		icon: 'pi pi-box',
		roles: ['ROLE_CAN_USE_GOODS']
	},
	{
		name: 'Настройки',
		icon: 'pi pi-cog',
		roles: ['ROLE_CAN_USE_SETTINGS'],
		child: [
			{ name: 'Профиль', to: '/profile', icon: 'pi pi-user' }
		]
	}
];
```

- **MUST**: Видимость пунктов меню — `item.roles.some((r) => authStore.hasRole(r))` по **profile.roles**, не JWT
- **MUST**: Использовать роли уровня действия/раздела (`ROLE_CAN_USE_*`), а не широкие роли вроде `ROLE_ADMIN`
- **SHOULD**: Иконки из PrimeIcons (`pi pi-*`)
- **MAY**: Вложенные меню через массив `child`

## Управление заголовками
```typescript
router.beforeEach((to, from, next) =>
{
	document.title = (to.meta.title as string) || 'App';
	next();
});
```

## Гарды маршрутов

> Refresh JWT, redirect на Passport login и загрузка profile — docs auth проекта (SSO/JWT), `../patterns/security.md`.

- **MUST**: Инициализация приложения — `authStore.init()` → `GET /auth/profile` **до** монтирования роутов (роли из profile)
- **MUST**: Гарды проверяют **profile** (и `meta.roles` по `profile.roles`), не JWT payload
- **MUST**: При failed refresh / 401 — `resolvePassportReturnUrl` + redirect на `{authHost}/login?return=...`, не локальный `/login`
- **SHOULD**: Публичные маршруты — `meta.public: true`

```typescript
// main.ts — docs auth проекта §8 (refreshJwt вызывается в http.beforeRequest при init)
const auth = useAuthStore();
auth.init()
	.then(() => app.mount('#app'))
	.catch(() => { /* refreshJwt уже редиректит на Passport login */ });

router.beforeEach((to, from, next) =>
{
	document.title = (to.meta.title as string) || 'App';

	if (to.meta.public)
	{
		return next();
	}

	const authStore = useAuthStore();
	if (!authStore.profile)
	{
		return next(false); // init не прошёл; refreshJwt уже мог сделать redirect
	}

	const requiredRoles = to.meta.roles as string[] | undefined;
	if (requiredRoles?.length && !requiredRoles.some((r) => authStore.hasRole(r)))
	{
		return next({ path: '/forbidden' });
	}

	next();
});
```
