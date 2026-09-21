package main

/*
  УРОК 5.3: OVERRIDE-ФАЙЛЫ И .ENV
  Один compose.yaml на все окружения — плохо. В dev нужны
  debug-логи, порты наружу, bind mounts. В prod — минимум
  портов, оптимизированные образы, никаких монтирований.
  Override-файлы решают это: база описывает общее, override —
  специфику окружения. .env держит значения, которые не должны
  попадать в git.

  СОДЕРЖАНИЕ:
    1.  Проблема: один compose на всё
    2.  Override: автоматический merge
    3.  Правила слияния (подробно)
    4.  Явные файлы через -f
    5.  Структура проекта
    6.  Что в override, что в базе
    7.  .env: зачем и как
    8.  Формы подстановки
    9.  .env.example как документация
    10. Приоритет значений
    11. Практика: dev/staging/prod
    12. Частые ошибки
    13. Антипаттерны
    14. Финальные выводы

  1. ПРОБЛЕМА: ОДИН COMPOSE НА ВСЁ

  Три окружения, разные требования:
    DEV:     bind mounts, порты 127.0.0.1, LOG_LEVEL=debug.
    STAGING: прод-образы, тестовые данные, полный стек.
    PROD:    минимум портов, LOG_LEVEL=warn, secrets manager.

  ПЛОХО — один файл с условиями:
    ports:
      - "${BIND_HOST:-0.0.0.0}:${APP_PORT}:8080"
    volumes:
      - ${SRC_MOUNT:-/dev/null}:/app/src

  Нечитаемо. Ошибка в одной ${...} = сломанное окружение.

  ПРАВИЛЬНО — база + override:
    compose.yaml          ← общее
    compose.override.yml  ← dev (авто)
    compose.prod.yml      ← prod (явно)
    compose.test.yml      ← тесты (явно)

  2. OVERRIDE: АВТОМАТИЧЕСКИЙ MERGE
  Compose ищет файлы в порядке:
    1. compose.yaml (или docker-compose.yml).
    2. compose.override.yaml (или docker-compose.override.yml).

  Оба в одной папке — Compose мержит автоматически.

    compose.yaml:
      services:
        app:
          image: my-app:1.0
          environment:
            APP_ENV: production

    compose.override.yml:
      services:
        app:
          environment:
            APP_ENV: development

    docker compose up:
      services:
        app:
          image: my-app:1.0
          environment:
            APP_ENV: development    ← override победил

  ВАЖНО: override подхватывается автоматически **только** если
  имя совпадает с базой:

    compose.yaml + compose.override.yaml           авто
    docker-compose.yml + docker-compose.override.yml   авто
    compose.yaml + docker-compose.override.yml     НЕ авто

  3. ПРАВИЛА СЛИЯНИЯ (ПОДРОБНО)

  СКАЛЯРЫ — заменяются:
    base:      image: my-app:1.0
    override:  image: my-app:dev
    result:    image: my-app:dev

  СПИСКИ — ОБЪЕДИНЯЮТСЯ:
    base:      ports: ["8080:8080"]
    override:  ports: ["9090:9090"]
    result:    ports: ["8080:8080", "9090:9090"]

    Оба порта публикуются. Хочешь заменить — используй
    !reset (Compose v2.24+):

      ports: !reset ["9090:9090"]

  СЛОВАРИ — мержатся рекурсивно:
    base:
      environment:
        A: "1"
        B: "2"
    override:
      environment:
        B: "3"
        C: "4"
    result:
      A: "1", B: "3", C: "4"

  COMMAND, ENTRYPOINT — заменяются целиком:
    base:      command: ["server", "--port", "8080"]
    override:  command: ["server", "--port", "9090", "--debug"]
    result:    ["server", "--port", "9090", "--debug"]

  VOLUMES — ОБЪЕДИНЯЮТСЯ:
    base:
      volumes:
        - appdata:/data
    override:
      volumes:
        - ./src:/app/src
    result:
      - appdata:/data
      - ./src:/app/src

    Оба монтирования применяются. Порядок — сначала из базы,
    потом из override.

  DEPENDS_ON — МЕРЖИТСЯ КАК СЛОВАРЬ:
    base:
      depends_on:
        postgres: { condition: service_healthy }
    override:
      depends_on:
        redis: { condition: service_healthy }
    result:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }

  НЕСКОЛЬКО OVERRIDE:
    docker compose -f base.yml -f staging.yml -f override.yml up
    Мержатся слева направо. Последний файл — приоритетнее.

  4. ЯВНЫЕ ФАЙЛЫ ЧЕРЕЗ -F
  Больше двух файлов — автоматика не справляется.

    docker compose -f compose.yaml -f compose.prod.yml up -d

  ПО ОКРУЖЕНИЯМ:
    Dev:
      docker compose up -d
      # compose.yaml + compose.override.yml (авто)
    Staging:
      docker compose -f compose.yaml -f compose.staging.yml up -d
    Prod:
      docker compose -f compose.yaml -f compose.prod.yml up -d
    Test:
      docker compose -f compose.yaml -f compose.test.yml up -d

  ВАЖНО: при использовании -f автоматический override НЕ
  применяется. Только явно указанные файлы.

  Хочешь всё вместе:
    docker compose \
      -f compose.yaml \
      -f compose.override.yml \
      -f compose.local.yml \
      up

  ПЕРЕМЕННАЯ COMPOSE_FILE:
    export COMPOSE_FILE=compose.yaml:compose.prod.yml
    docker compose up -d

  Разделитель — `:` (Unix) или `;` (Windows).

  ПРОВЕРКА ИТОГОВОГО КОНФИГА:
    docker compose config

  Показывает результат после слияния всех файлов и подстановки
  переменных. Незаменимо, когда не понимаешь, что применилось.

  Только конкретный сервис:
    docker compose config app

  5. СТРУКТУРА ПРОЕКТА
    my-project/
    ├── compose.yaml                    ← база (в git)
    ├── compose.override.yml            ← dev (в .gitignore)
    ├── compose.override.yml.example    ← шаблон (в git)
    ├── compose.prod.yml                ← prod (в git)
    ├── compose.staging.yml             ← staging (в git)
    ├── .env                            ← секреты (в .gitignore)
    ├── .env.example                    ← шаблон (в git)
    ├── .gitignore
    └── Dockerfile

  .GITIGNORE:
    .env
    compose.override.yml
    compose.override.yaml
    data/

  БЫСТРЫЙ СТАРТ для нового разработчика:
    git clone ...
    cd project
    cp .env.example .env
    docker compose up -d

  6. ЧТО В OVERRIDE, ЧТО В БАЗЕ
  В OVERRIDE (локальное):

    # Порты для отладки.
    postgres:
      ports:
        - "127.0.0.1:5432:5432"

    # Bind mount исходников.
    app:
      volumes:
        - ./src:/app/src

    # Hot reload.
    app:
      develop:
        watch:
          - action: sync
            path: ./src
            target: /app/src

    # Debug-логи.
    app:
      environment:
        LOG_LEVEL: debug
        APP_ENV: development

    # Dev-образ.
    app:
      build:
        dockerfile: Dockerfile.dev

  В БАЗЕ (одинаково у всех):
    • Образы и версии.
    • Restart policies.
    • Healthcheck.
    • depends_on.
    • Networks.
    • Volumes.
    • Лимиты ресурсов.

  ЧЕГО НЕТ НИ ТАМ, НИ ТАМ:
    • Секреты — только в .env или secrets manager.
    • Хардкод паролей — никогда.

  ПРИМЕР ПОЛНОГО OVERRIDE:
    services:
      app:
        build:
          dockerfile: Dockerfile.dev
        user: "${UID:-1000}:${GID:-1000}"
        ports:
          - "${APP_PORT:-8080}:8080"
        volumes:
          - ./src:/app/src
          - appdata:/app/data
          - go-cache:/root/.cache/go-build
        environment:
          APP_ENV: development
          LOG_LEVEL: debug
        develop:
          watch:
            - action: sync
              path: ./src
              target: /app/src
              ignore:
            - action: rebuild
              path: go.mod
            - action: rebuild
              path: go.sum

      postgres:
        ports:
          - "127.0.0.1:5432:5432"

      redis:
        ports:
          - "127.0.0.1:6379:6379"

    volumes:
      go-cache:

  7. .ENV: ЗАЧЕМ И КАК
  .env — файл с переменными. Compose читает автоматически из
  папки проекта.

    .env:
      POSTGRES_PASSWORD=supersecret
      NGINX_PORT=8080

    compose.yaml:
      services:
        postgres:
          environment:
            POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
        nginx:
          ports:
            - "${NGINX_PORT}:80"

  При `up`:
    POSTGRES_PASSWORD → supersecret
    NGINX_PORT → 8080

  ДРУГОЙ ФАЙЛ:
    docker compose --env-file .env.prod up -d

  ПРАВИЛА ЗАПИСИ:
    # Комментарии через #.
    POSTGRES_USER=app
    POSTGRES_PASSWORD=changeme

    • Без кавычек (иначе станут частью значения).
    • Без пробелов вокруг =.
    • Одна переменная на строку.
    • Без многострочных значений.

  8. ФОРМЫ ПОДСТАНОВКИ
  4 формы:
    ${VAR}              — простая. Нет → пусто + warning.
    ${VAR:-default}     — нет или пусто → default.
    ${VAR-default}      — не определено → default.
    ${VAR:?error}       — нет или пусто → ошибка и стоп.

  ПРИМЕР:
    services:
      app:
        image: "my-app:${TAG:-latest}"
        ports:
          - "${APP_PORT:-8080}:8080"
        environment:
          APP_ENV: "${APP_ENV:-production}"
          DB_PASSWORD: "${DB_PASSWORD:?DB_PASSWORD required}"
          LOG_LEVEL: "${LOG_LEVEL:-info}"

  РЕКОМЕНДАЦИИ:
    • Секреты → ${VAR:?error}. Падать при отсутствии.
    • Порты, пути, уровни логов → ${VAR:-default}.
    • Остальное → ${VAR}.

  ЗАЧЕМ ${VAR:?error}:
    Плохой сценарий без него:
      Пароль БД потерялся в .env.
      Compose запустил контейнер с пустым паролем.
      Приложение подключилось к Postgres с неправильным
      паролем.
      Через час упал в проде.
      Никто не заметил — потому что тихо.

    С ${VAR:?}:
      Compose падает при `up`.
      Понятная ошибка: «DB_PASSWORD required».
      Исправляешь сразу.

  9. .ENV.EXAMPLE КАК ДОКУМЕНТАЦИЯ
  Шаблон в git, без секретов.

    # Скопируй в .env и заполни.
    # cp .env.example .env

    COMPOSE_PROJECT_NAME=my-project

    # --- Postgres ---
    POSTGRES_USER=app
    POSTGRES_PASSWORD=changeme       # ОБЯЗАТЕЛЬНО поменяй
    POSTGRES_DB=app

    # --- Порты ---
    NGINX_PORT=8080
    ADMINER_PORT=8090

    # --- Приложение ---
    APP_ENV=production
    LOG_LEVEL=info

    # --- Redis ---
    REDIS_PASSWORD=changeme-redis

    # --- UID/GID для non-root ---
    UID=1000
    GID=1000

  ЗАЧЕМ:
    • Новый разработчик знает, что задать.
    • В diff PR видно новые переменные.
    • Документация всегда актуальна.

  10. ПРИОРИТЕТ ЗНАЧЕНИЙ
  От высшего к низшему:

    1. Переменные окружения оболочки (export VAR=value).
    2. --env-file.
    3. .env в папке проекта.
    4. environment внутри сервиса.
    5. env_file внутри сервиса.
    6. ENV в Dockerfile.

  ПРИМЕР:

    .env:
      LOG_LEVEL=info

    Оболочка:
      export LOG_LEVEL=debug

    Контейнер получит LOG_LEVEL=debug.

  ВРЕМЕННОЕ ПЕРЕОПРЕДЕЛЕНИЕ:
    LOG_LEVEL=debug docker compose up -d

  11. ПРАКТИКА: DEV/STAGING/PROD

  БАЗА (compose.yaml):
    services:
      app:
        image: my-app:${TAG:-latest}
        restart: unless-stopped
        healthcheck:
          test: ["CMD", "wget", "-q", "-O", "-", "http://localhost:8080/health"]
        depends_on:
          postgres:
            condition: service_healthy
        networks:
          - backend

      postgres:
        image: postgres:16-alpine
        environment:
          POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?required}
        volumes:
          - pgdata:/var/lib/postgresql/data
        healthcheck:
          test: ["CMD-SHELL", "pg_isready"]
        networks:
          - backend

    volumes:
      pgdata:

    networks:
      backend:

  DEV (compose.override.yml):
    services:
      app:
        build: .
        user: "${UID:-1000}:${GID:-1000}"
        ports:
          - "${APP_PORT:-8080}:8080"
        volumes:
          - ./src:/app/src
        environment:
          LOG_LEVEL: debug
          APP_ENV: development
        develop:
          watch:
            - action: sync
              path: ./src
              target: /app/src

      postgres:
        ports:
          - "127.0.0.1:5432:5432"

  STAGING (compose.staging.yml):
    services:
      app:
        image: my-app:staging
        ports:
          - "9080:8080"
        environment:
          LOG_LEVEL: info
          APP_ENV: staging

      postgres:
        ports:
          - "127.0.0.1:5433:5432"

  PROD (compose.prod.yml):
    services:
      app:
        image: my-app:${TAG}
        ports:
          - "127.0.0.1:8080:8080"    # только nginx проксирует
        environment:
          LOG_LEVEL: warn
          APP_ENV: production
        deploy:
          resources:
            limits:
              memory: 512M

      nginx:
        image: nginx:1.27-alpine
        ports:
          - "80:80"
          - "443:443"
        depends_on:
          app:
            condition: service_healthy

    # Postgres порт наружу НЕ публикуется.

  ЗАПУСК ПО ОКРУЖЕНИЯМ:
    Dev:
      docker compose up -d
      # база + override
    Staging:
      docker compose -f compose.yaml -f compose.staging.yml up -d
    Prod:
      docker compose -f compose.yaml -f compose.prod.yml up -d

  12. ЧАСТЫЕ ОШИБКИ
  12.1. КАВЫЧКИ В .ENV.
    DB_PASSWORD="secret"    ← кавычки станут частью значения.
  12.2. ПРОБЕЛЫ ВОКРУГ =.
    DB_PASSWORD = secret    ← ключ станет "DB_PASSWORD ".
  12.3. ЗАБЫЛИ .ENV В .GITIGNORE.
    Пароли в git. Даже после удаления — в истории.
  12.4. НЕТ .ENV.EXAMPLE.
    Новый разработчик не знает, что задать.
  12.5. ХАРДКОД В COMPOSE.YAML.
    environment: { DB_PASSWORD: supersecret }  ← в git!
  12.6. МНОГОСТРОЧНЫЕ ЗНАЧЕНИЯ.
    .env не поддерживает. Для ключей — secrets.
  12.7. КОММЕНТАРИЙ ПОСЛЕ ЗНАЧЕНИЯ.
    APP_PORT=8080    # порт    ← # станет частью значения.
  12.8. --ENV-FILE И .ENV ВМЕСТЕ.
    --env-file перекрывает .env. Многие не знают.
  12.9. ПЕРЕМЕННАЯ В КЛЮЧЕ YAML.
    ${SERVICE}:    ← не работает. Только в значениях.
  12.10. НЕТ ${VAR:?}.
    Забыли пароль → пустое значение → тихий сбой.
  12.11. СПИСКИ В OVERRIDE БЕЗ !RESET.
    Пытаются заменить ports, а они объединяются.
    Неожиданное поведение.
  12.12. OVERRIDE С PROD-НАСТРОЙКАМИ.
    В override prod-теги, порты, secrets. Разные
    окружения — разные файлы.

  13. АНТИПАТТЕРНЫ
  13.1. .ENV В GIT.
    Все пароли в истории.
  13.2. ХАРДКОД ПАРОЛЕЙ В COMPOSE.YAML.
    Секрет в репозитории.
  13.3. OVERRIDE ДЛЯ PROD.
    Override — для dev. Prod — отдельный compose.prod.yml.
  13.4. ОДИН БОЛЬШОЙ OVERRIDE.
    Сложили всё — нечитаемо. Разделяй по окружениям.
  13.5. ЗАМЕНА СПИСКОВ В OVERRIDE.
    Списки объединяются. Нужно !reset или полное
    переопределение.
  13.6. НЕТ .ENV.EXAMPLE.
    Compose падает у коллег с непонятной ошибкой.
  13.7. СЕКРЕТЫ ЧЕРЕЗ ENV В ПРОДЕ.
    Видны в docker inspect. Docker secrets / Vault.
  13.8. ПЕРЕМЕННЫЕ ТОЛЬКО В OVERRIDE.
    У коллеги override нет — падает. Общее — в базе.
  13.9. КОММЕНТАРИИ ПОСЛЕ ЗНАЧЕНИЯ В .ENV.
    # попадает в значение.
  13.10. ОТСУТСТВИЕ ${VAR:?} ДЛЯ ОБЯЗАТЕЛЬНЫХ.
    Тихие сбои вместо явных ошибок.
  13.11. ОДИН .ENV НА ВСЕ ОКРУЖЕНИЯ.
    В dev и prod разные значения. Разделяй: .env.dev,
    .env.prod, через --env-file.
  13.12. OVERRIDE БЕЗ .EXAMPLE.
    Новый разработчик не знает, какие настройки
    переопределить. Добавь compose.override.yml.example.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Override — для локальных настроек. Авто, если имя
      совпадает с базой.
  2.  Слияние: скаляры заменяются, списки объединяются,
      словари мержатся, command заменяется, volumes и
      depends_on мержатся.
  3.  Для явного управления — `-f base.yml -f prod.yml`.
      При -f авто-override отключается.
  4.  Структура: compose.yaml (база), compose.override.yml
      (dev, .gitignore), compose.prod.yml (prod), .env
      (секреты, .gitignore), .env.example (шаблон).
  5.  В override: порты, bind mounts, debug-логи, dev-образ.
      В базе: общие настройки.
  6.  .env — для подстановки в compose.yaml. Не передаётся
      в контейнер напрямую.
  7.  4 формы: ${VAR}, ${VAR:-default}, ${VAR-default},
      ${VAR:?error}. Секреты — через :?.
  8.  .env.example — документация. Без секретов. В git.
  9.  Приоритет: оболочка > --env-file > .env >
      environment > env_file > ENV из Dockerfile.
  10. `docker compose config` — проверка итогового конфига
      после слияния.
  11. Частые ошибки: кавычки, пробелы вокруг =, нет
      .gitignore, хардкод, комментарии после значения,
      замена списков без !reset.
  12. Безопасность: .env — dev. Docker secrets / Vault —  prod.
  13. Антипаттерны: .env в git, override для prod, один
      большой override, секреты в environment, один .env
      на все окружения.
*/
