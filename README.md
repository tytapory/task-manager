# Task Manager API

Сервис управления задачами и командами с разграничением прав доступа, валидацией, кэшированием и отчетами.

**Основные особенности:**
* Чистая архитектура
* Интеграционные тесты слоя юзкейсов, включающие генерацию SQL-отчетов и бизнес-логику прав доступа через test containers
* Структурированные логи
* Автоматизированное окружение
* Swager документация

---

**Конфигурация**

Сервис настраивается через переменные окружения. При запуске через Docker Compose параметры передаются автоматически.

| Переменная | Назначение | Дефолтное значение |
| :--- | :--- | :--- |
| `SERVER_HOST` | Хост сервера | `0.0.0.0` |
| `SERVER_PORT` | Порт сервера | `8080` |
| `SERVER_TIMEOUT` | Таймаут запросов | `30s` |
| `DB_HOST` | Хост MySQL | `localhost` |
| `DB_PORT` | Порт MySQL | `3306` |
| `DB_USER` | Пользователь БД | `root` |
| `DB_PASSWORD` | Пароль БД | `password` |
| `DB_NAME` | Имя базы данных | `task_manager` |
| `DB_CHARSET` | Кодировка БД | `utf8mb4` |
| `REDIS_HOST` | Хост Redis | `localhost` |
| `REDIS_PORT` | Порт Redis | `6379` |
| `REDIS_PASSWORD` | Пароль Redis | *(пусто)* |
| `REDIS_DB` | Индекс БД Redis | `0` |
| `REDIS_TTL` | Время жизни кэша | `5m` |
| `JWT_SECRET` | Секретный ключ JWT | `super_secret` |
| `JWT_TTL` | Время жизни токена | `72h` |

---

**Запуск и миграции**

**1. Быстрый запуск сервиса**
`make rebuild`

Команда поднимет MySQL, Redis, автоматически накатит все миграции через сервис `migrate` и запустит приложение на порту `8080`.

**2. Управление миграциями вручную**

Для локальной работы с миграциями через `golang-migrate`:

* **Создание новой миграции:** `make migrate-create`
* **Применение миграций:** `make migrate-up`
* **Откат последней миграции:** `make migrate-down`

---

**Примеры запросов**

**1. Регистрация пользователя**
```curl -X POST http://localhost:8080/api/v1/register -H "Content-Type: application/json" -d '{"email": "admin@example.com", "name": "Admin", "password": "password123"}'```

**2. Логин**
```curl -X POST http://localhost:8080/api/v1/login -H "Content-Type: application/json" -d '{"email": "admin@example.com", "password": "password123"}'```

**3. Создание команды**
```curl -X POST http://localhost:8080/api/v1/teams -H "Authorization: Bearer <YOUR_JWT_TOKEN>" -H "Content-Type: application/json" -d '{"name": "DevOps Team"}'```

**4. Создание задачи**
```curl -X POST http://localhost:8080/api/v1/tasks -H "Authorization: Bearer <YOUR_JWT_TOKEN>" -H "Content-Type: application/json" -d '{"team_id": "<TEAM_ID>", "title": "Fix Prod Bug", "description": "Urgent bugfix needed"}'```

**5. Получение списка задач (с пагинацией и фильтрами)**
```curl -X GET "http://localhost:8080/api/v1/tasks?team_id=<TEAM_ID>&status=todo&limit=10&offset=0" -H "Authorization: Bearer <YOUR_JWT_TOKEN>"```

**6. Получение SQL-отчета по статистике команды**
```curl -X GET "http://localhost:8080/api/v1/teams/<TEAM_ID>/stats" -H "Authorization: Bearer <YOUR_JWT_TOKEN>"```

Все запросы можно посмотреть в папке docs

---

**Тестирование**

Для запуска интеграционных тестов слоя юзкейсов:
`make test-integration`
