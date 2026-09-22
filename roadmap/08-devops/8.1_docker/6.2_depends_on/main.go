package main

/*
  УРОК 6.2: DEPENDS_ON — КОРОТКИЙ И ДЛИННЫЙ
  Compose запускает сервисы параллельно. Postgres, Redis, app
  стартуют одновременно. Но app не может работать без БД —
  подключится, получит "connection refused", упадёт.
  depends_on решает это: задаёт порядок запуска. Но у него
  две формы — короткая (плохая) и длинная (правильная). Разница
  критична: одна ждёт "контейнер запущен", другая — "сервис готов".

  СОДЕРЖАНИЕ:
    1.  Проблема: параллельный запуск
    2.  Короткая форма: depends_on: [db]
    3.  Почему короткая форма плохая
    4.  Длинная форма: condition
    5.  Три условия: service_started, service_healthy,
        service_completed_successfully
    6.  service_healthy: требует healthcheck
    7.  service_completed_successfully: для миграций
    8.  Паттерн: migrate → app
    9.  depends_on не перезапускает
    10. Зависимости транзитивны
    11. depends_on vs wait-for-it.sh
    12. Диагностика
    13. Антипаттерны
    14. Финальные выводы

  1. ПРОБЛЕМА: ПАРАЛЛЕЛЬНЫЙ ЗАПУСК
  По умолчанию Compose запускает сервисы параллельно.

    services:
      postgres:
        image: postgres:16-alpine

      app:
        image: my-app:1.0
        environment:
          DB_HOST: postgres

    docker compose up -d

    Что происходит:
      t=0s: Compose создаёт сеть, volumes.
      t=1s: Docker запускает postgres и app одновременно.
      t=2s: app пытается подключиться к postgres:5432.
      t=2s: Postgres ещё инициализируется.
      t=2s: app падает: connection refused.
      t=3s: Postgres готов.

  App упал. С restart: unless-stopped — перезапустится и
  подключится. Но:
    • В логах ошибка.
    • Пара секунд недоступности.
    • Если миграции в app — упадут.
  РЕШЕНИЕ: depends_on.

  2. КОРОТКАЯ ФОРМА: DEPENDS_ON: [DB]
  Синтаксис — список имён.
    services:
      app:
        depends_on:
          - postgres
          - redis

  Что это значит: Compose запускает app **после** того, как
  postgres и redis перейдут в состояние running.

  ЧТО ЗНАЧИТ "RUNNING":
    Docker запустил процесс (entrypoint).
    Процесс ещё не завершился.
    Но это НЕ значит, что сервис готов принимать запросы.

  ЧТО ПРОИСХОДИТ С POSTGRES:
    t=0s: Docker стартует процесс postgres.
    t=0s: Статус контейнера — running.
    t=1s: Compose видит "postgres running", запускает app.
    t=1s: Postgres только начал инициализацию.
    t=1s: app пытается подключиться — connection refused.
    t=10s: Postgres готов.
  App всё равно упал. Короткая форма **не помогает**.

  3. ПОЧЕМУ КОРОТКАЯ ФОРМА ПЛОХАЯ
  Проблема: "running" — это не "готов".

  Контейнер в статусе running, когда:
    • Процесс запущен.
    • Ещё не упал.

  Готовность сервиса — другое:
    • Postgres принимает соединения.
    • Redis отвечает на PING.
    • Kafka готова принимать продюсеров.
    • HTTP-сервер отвечает на /health.

  МЕЖДУ "RUNNING" И "ГОТОВ" — ПРОПАСТЬ.

  Для Postgres: 5-15 секунд (инициализация, миграции, роли).
  Для Kafka: 10-30 секунд (создание топиков, синхронизация).
  Для Elasticsearch: 30-60 секунд (индексы).

  Короткая форма ждёт только начала. App стартует раньше,
  чем сервис готов. Падение.

  КОГДА КОРОТКАЯ ФОРМА ОК:
    Никогда. Даже для «простых» сервисов. Всегда используй
    длинную форму с condition.

    Исключение: если у сервиса нет healthcheck и его
    готовность невозможно определить. Тогда — короткая
    форма + retry в коде.

  4. ДЛИННАЯ ФОРМА: CONDITION
  Синтаксис — словарь с условием.
    services:
      app:
        depends_on:
          postgres:
            condition: service_healthy
          redis:
            condition: service_started

  Каждый сервис получает своё условие.

  ЧТО ЭТО ДАЁТ:
    Compose ждёт выполнения условия. Только потом запускает
    зависимый сервис.

    Для service_healthy — ждёт, пока healthcheck пройдёт.
    Postgres принимает соединения → Compose запускает app.
    App подключается успешно.

  РАЗНИЦА ПО ВРЕМЕНИ:
    Короткая форма:  app стартует через 1 сек, падает.
    Длинная форма:   app стартует через 15 сек, работает.

  5. ТРИ УСЛОВИЯ

  SERIVCE_STARTED:
    condition: service_started
    Compose ждёт, пока контейнер перейдёт в running.
    Это дефолт для короткой формы.
    Когда использовать: почти никогда. Бесполезно, потому что
    "running" ≠ "готов".

  SERVICE_HEALTHY:
    condition: service_healthy
    Compose ждёт, пока healthcheck пройдёт успешно.
    Требует: у сервиса-зависимости ДОЛЖЕН быть healthcheck.
    Без него — Compose упадёт с ошибкой.
    Когда использовать: для БД, кэшей, брокеров, любых
    сервисов, к которым подключается зависимый.

  SERVICE_COMPLETED_SUCCESSFULLY:
    condition: service_completed_successfully
    Compose ждёт, пока контейнер завершится с кодом 0.
    Когда использовать: для одноразовых задач — миграций,
    seed-скриптов, backup.

  СРАВНЕНИЕ:
    ┌──────────────────────────────────┬────────────┬──────────┐
    │ Условие                          │ Что ждёт   │ Для чего │
    ├──────────────────────────────────┼────────────┼──────────┤
    │ service_started                  │ running    │ редко    │
    │ service_healthy                  │ healthcheck│ БД, кэш  │
    │ service_completed_successfully   │ exit 0     │ миграции │
    └──────────────────────────────────┴────────────┴──────────┘

  6. SERVICE_HEALTHY: ТРЕБУЕТ HEALTHCHECK
  Ключевое правило: **без healthcheck condition не работает**.

  ПЛОХОЙ ПРИМЕР:
    services:
      postgres:
        image: postgres:16-alpine
        # healthcheck отсутствует.

      app:
        depends_on:
          postgres:
            condition: service_healthy

    docker compose up -d

    Ошибка:
      service "postgres" has no healthcheck configured

    Compose не знает, что считать "готов". Не запускает
    ни одного сервиса.

  ХОРОШИЙ ПРИМЕР:
    services:
      postgres:
        image: postgres:16-alpine
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U app"]
          interval: 5s
          timeout: 3s
          retries: 10
          start_period: 10s

      app:
        depends_on:
          postgres:
            condition: service_healthy

    Что происходит:
      Compose запускает postgres.
      Healthcheck запускается каждые 5 секунд.
      pg_isready возвращает 0, когда БД готова.
      Через 10-15 секунд postgres становится healthy.
      Compose запускает app.

  ПРАВИЛО: используешь service_healthy — добавь healthcheck
  к зависимому сервису.

  ГОТОВЫЕ HEALTHCHECK:
    Postgres: pg_isready -U ${POSTGRES_USER}
    MySQL:    mysqladmin ping -h localhost
    Redis:    redis-cli ping
    Mongo:    mongosh --eval "db.adminCommand('ping')"
    Kafka:    kafka-topics.sh --bootstrap-server localhost:9092 --list
    Nginx:    wget -q -O - http://localhost/health

  7. SERVICE_COMPLETED_SUCCESSFULLY: ДЛЯ МИГРАЦИЙ
  Условие для одноразовых задач. Compose ждёт, пока контейнер
  завершится с кодом 0.

  ПРИМЕР:
    services:
      migrate:
        image: my-app:1.0
        command: ["migrate", "up"]
        depends_on:
          postgres:
            condition: service_healthy

      app:
        image: my-app:1.0
        command: ["server"]
        depends_on:
          migrate:
            condition: service_completed_successfully

  ЧТО ПРОИСХОДИТ:
    1. Compose запускает postgres.
    2. Ждёт healthcheck postgres.
    3. Запускает migrate.
    4. Ждёт, пока migrate завершится с кодом 0.
    5. Запускает app.

  ВАЖНО:
    • migrate должен иметь restart: "no" (не перезапускать).
    • Если migrate упадёт (код != 0), app не запустится.
    • Если migrate завершится с 0 — Compose переходит к app.

  ПРАВИЛЬНЫЙ КОНТЕЙНЕР MIGRATE:
    services:
      migrate:
        image: my-app:1.0
        command: ["migrate", "up"]
        restart: "no"              # одноразовая задача
        depends_on:
          postgres:
            condition: service_healthy

  В КОДЕ MIGRATE:
    func main() {
        if os.Args[1] == "migrate" {
            if err := runMigrations(); err != nil {
                log.Fatal(err)     // выход с кодом 1
            }
            return                 // выход с кодом 0
        }
        runServer()
    }

  8. ПАТТЕРН: MIGRATE → APP
  Классический паттерн для приложений с миграциями.

  СХЕМА:
    [postgres] ──service_healthy──► [migrate] ──service_completed_successfully──► [app]

  ПОЛНЫЙ COMPOSE:
    services:
      postgres:
        image: postgres:16-alpine
        environment:
          POSTGRES_USER: ${POSTGRES_USER}
          POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
          POSTGRES_DB: ${POSTGRES_DB}
        volumes:
          - pgdata:/var/lib/postgresql/data
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
          interval: 5s
          timeout: 3s
          retries: 10
          start_period: 10s
        restart: unless-stopped

      migrate:
        image: my-app:1.0
        command: ["migrate", "up"]
        depends_on:
          postgres:
            condition: service_healthy
        restart: "no"
        environment:
          DB_HOST: postgres
          DB_USER: ${POSTGRES_USER}
          DB_PASSWORD: ${POSTGRES_PASSWORD}
          DB_NAME: ${POSTGRES_DB}

      app:
        image: my-app:1.0
        command: ["server"]
        depends_on:
          migrate:
            condition: service_completed_successfully
        restart: unless-stopped
        environment:
          DB_HOST: postgres
          DB_USER: ${POSTGRES_USER}
          DB_PASSWORD: ${POSTGRES_PASSWORD}
          DB_NAME: ${POSTGRES_DB}

    volumes:
      pgdata:

  ЧТО ПРОИСХОДИТ:
    1. postgres стартует.
    2. Healthcheck проверяет pg_isready.
    3. Когда healthy — Compose запускает migrate.
    4. migrate применяет миграции, выходит с 0.
    5. Compose запускает app.
    6. app стартует с уже готовой схемой БД.

  ПРЕИМУЩЕСТВА:
    • Никаких гонок: app стартует после миграций.
    • Никаких "table not found".
    • Миграции всегда применены.

  ВАРИАНТ ДЛЯ DEV:
    В dev можно запускать миграции вручную:
      docker compose up -d postgres
      docker compose run --rm migrate
      docker compose up -d app

    Или автоматически через тот же паттерн.

  9. DEPENDS_ON НЕ ПЕРЕЗАПУСКАЕТ
  Важно понять: depends_on работает **только при старте**.

  ЧТО ЭТО ЗНАЧИТ:
    Если Postgres упадёт **после** старта app — Compose
    НЕ перезапустит app. App продолжит работать (или падать
    сам), но depends_on больше не влияет.

  ПРИМЕР:
    docker compose up -d
    # Всё работает.

    docker compose stop postgres
    # Postgres остановлен.
    # app продолжает работать.
    # depends_on НЕ перезапускает app.

  ЧТО ДЕЛАТЬ:
    Retry в коде приложения. Если БД недоступна — пробовать
    снова с backoff.

    В GO:
      func connectPostgres(dsn string) (*sql.DB, error) {
          var db *sql.DB
          var err error
          for attempt := 1; attempt <= 10; attempt++ {
              db, err = sql.Open("pgx", dsn)
              if err == nil {
                  if err = db.Ping(); err == nil {
                      return db, nil
                  }
              }
              time.Sleep(time.Duration(attempt) * time.Second)
          }
          return nil, err
      }

    Это спасает в двух случаях:
      • app стартует раньше БД (защита от плохого depends_on).
      • БД перезапустилась во время работы app.

  ЗАЧЕМ ТОГДА DEPENDS_ON:
    Чтобы не было ошибок при старте. Retry защищает от
    долгих проблем. Вместе — надёжно.

  10. ЗАВИСИМОСТИ ТРАНЗИТИВНЫ
  Если A зависит от B, а B зависит от C — Compose запускает
  их в порядке C → B → A.

  ПРИМЕР:
    services:
      postgres:
        image: postgres:16-alpine
        healthcheck:
          test: ["CMD-SHELL", "pg_isready"]

      migrate:
        depends_on:
          postgres:
            condition: service_healthy
        restart: "no"

      app:
        depends_on:
          migrate:
            condition: service_completed_successfully

    Порядок:
      1. postgres
      2. migrate (после healthy postgres)
      3. app (после успеха migrate)

  ЯВНЫЕ ЗАВИСИМОСТИ:
    Если app зависит и от postgres напрямую, и от migrate —
    указывай обе:

      app:
        depends_on:
          postgres:
            condition: service_healthy
          migrate:
            condition: service_completed_successfully

    Compose сам разберётся с порядком.

  ЦИКЛЫ ЗАПРЕЩЕНЫ:
    services:
      a:
        depends_on: [b]
      b:
        depends_on: [a]

    Ошибка:
      circular dependency between a and b

  11. DEPENDS_ON VS WAIT-FOR-IT.SH
  Раньше (до Compose v2) depends_on не умел ждать здоровье.
  Использовали скрипты wait-for-it.sh.

  WAIT-FOR-IT.SH:
    #!/bin/bash
    until nc -z postgres 5432; do
      echo "waiting for postgres..."
      sleep 1
    done
    exec "$@"

    В Dockerfile:
      COPY wait-for-it.sh /wait-for-it.sh
      ENTRYPOINT ["/wait-for-it.sh", "postgres:5432", "--"]
      CMD ["/app"]

    Или в compose:
      services:
        app:
          command: ["/wait-for-it.sh", "postgres:5432", "--", "/app"]

  ПРОБЛЕМЫ WAIT-FOR-IT:
    • Проверяет только TCP-порт, не готовность сервиса.
    • Требует nc, bash, sh — не работает на distroless.
    • Скрипт копируется в образ — лишняя сложность.
    • Не понимает, что Postgres готов, а не просто слушает.

  СЕЙЧАС: depends_on с condition — нативный и правильный
  способ.

  Когда wait-for-it всё ещё нужен:
    • Compose v1 (устаревший).
    • Проверка чего-то, что не покрывается healthcheck
      (редкие случаи).

  12. ДИАГНОСТИКА
  Проблема: app стартует раньше postgres и падает.

  ДИАГНОСТИКА:
    # 1. Смотрим логи app.
    docker compose logs app
    # "connection refused" или "database not ready"

    # 2. Смотрим, есть ли depends_on.
    docker compose config | Select-String "depends_on"

    # 3. Проверяем healthcheck у postgres.
    docker inspect <pg-container> -f '{{json .Config.Healthcheck}}'
    # null — healthcheck не задан

    # 4. Смотрим статусы.
    docker compose ps
    # postgres — Up (health: starting) в момент старта app

  РЕШЕНИЕ:
    Добавить healthcheck к postgres.
    Заменить depends_on на длинную форму.
    Добавить start_period, если postgres стартует долго.

  ПРОБЛЕМА: migrate упал, app не запустился.

  ДИАГНОСТИКА:
    docker compose logs migrate
    # Ошибка миграции.

    docker compose ps
    # migrate — Exited (1)    ← код 1, не 0

  РЕШЕНИЕ:
    Починить migrate. Проверить, что схема БД соответствует
    ожиданиям миграции.

  ПРОБЛЕМА: Compose не стартует app, ошибка про healthcheck.

  ДИАГНОСТИКА:
    docker compose up -d
    # service "postgres" has no healthcheck configured

  РЕШЕНИЕ:
    Добавить healthcheck к postgres.

  13. АНТИПАТТЕРНЫ
  13.1. КОРОТКАЯ ФОРМА БЕЗ HEALTHCHECK.
    depends_on: [postgres] — ждёт только running. App
    падает при подключении. Используй condition.
  13.2. service_healthy БЕЗ HEALTHCHECK.
    Compose упадёт с ошибкой "no healthcheck configured".
  13.3. RESTART: ALWAYS ДЛЯ MIGRATE.
    Migrate завершился с 0, потом перезапустился. Миграции
    применяются повторно, ломают данные. Только restart: "no".
  13.4. DEPENDS_ON БЕЗ RETRY В КОДЕ.
    Postgres перезапустился во время работы — app падает.
    Retry в коде защищает.
  13.5. HEALTHCHECK ЗАВИСИТ ОТ ВНЕШНЕГО РЕСУРСА.
    Healthcheck проверяет БД, которая сама может быть
    недоступна. Контейнер никогда не станет healthy.
  13.6. ЦИКЛИЧЕСКИЕ ЗАВИСИМОСТИ.
    a → b → a. Compose упадёт. Найди и разорви цикл.
  13.7. ВСЁ ЧЕРЕЗ DEPENDS_ON.
    App зависит от postgres, redis, kafka, migrate. Один
    сервис — четыре зависимости. Долгий старт, сложная отладка.
  13.8. WAIT-FOR-IT ВМЕСТО CONDITION.
    Compose v2 умеет сам. Не тащи скрипты в образ.
  13.9. SHORT FORM + RETRY ТОЛЬКО.
    Полагаются на retry в коде, без depends_on. Медленный
    старт, ошибки в логах.
  13.10. service_started ДЛЯ БД.
    "Postgres running" ≠ "Postgres готов". Только service_healthy.
  13.11. НЕТ START_PERIOD.
    Postgres стартует 20 секунд, retries=3 × interval=5s
    → unhealthy раньше, чем БД готова.
  13.12. НЕ ЧИТАТЬ ЛОГИ MIGRATE.
    Миграция упала, app не запустился. Ищут причину где
    угодно, кроме логов migrate.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Короткая форма depends_on: [db] ждёт только запуска
      контейнера. Не готовности. Плохо.
  2.  Длинная форма с condition ждёт выполнения условия.
      Правильный способ.
  3.  Три условия: service_started (редко), service_healthy
      (для БД/кэшей), service_completed_successfully (для миграций).
  4.  service_healthy требует healthcheck у зависимого.
      Без него Compose упадёт.
  5.  service_completed_successfully — для одноразовых
      задач. restart: "no" обязателен.
  6.  Классический паттерн: postgres → migrate → app.
      Migrate между ними.
  7.  depends_on работает только при старте. При падении
      после — не помогает. Нужен retry в коде.
  8.  Зависимости транзитивны. A → B → C запускается
      в порядке C, B, A.
  9.  Циклы запрещены. Compose падает при попытке создать.
  10. wait-for-it.sh устарел. Compose v2 умеет condition.
  11. Диагностика: docker compose logs, docker compose
      config, docker inspect.
  12. Антипаттерны: короткая форма без condition,
      service_healthy без healthcheck, restart: always
      для migrate, нет retry в коде, без start_period.
*/
