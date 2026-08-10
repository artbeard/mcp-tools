# Конфигурация Docker

## Структура
```
project/
├── docker/
│   └── php-fpm/
│       ├── Dockerfile
│       └── php.ini
├── docker-compose.yml
└── ...
```

## Шаблон docker-compose
```yaml
services:
  {project}-php:
    build: ./docker/php-fpm
    volumes:
      - ./:/var/www/html
      - ./docker/php-fpm/php.ini:/usr/local/etc/php/php.ini
    networks:
      - mp

  {project}-nginx:
    image: nginx
    ports:
      - "40XX:80"
    depends_on:
      - {project}-php
    networks:
      - mp

networks:
  mp:
    driver: bridge
    external: true
```

## Правила
- **MUST**: Общая сеть `mp` (external, bridge driver) для всех проектов
- **MUST**: Базовый образ PHP: `php:8.4-fpm-alpine3.17`
- **MUST**: Установить расширения: PDO, MySQLi, PDO_MySQL, OPCache, Zip
- **SHOULD**: Уникальный порт на проект (диапазон 40XX)
- **MAY**: Supervisord для фоновых воркеров (парсер-проекты)

## Вход в контейнер
```bash
docker-compose exec {project}-php bash
```
