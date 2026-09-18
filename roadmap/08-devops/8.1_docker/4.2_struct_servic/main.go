package main

/*
  УРОК 4.2: СТРУКТУРА ФАЙЛА И СЕРВИСЫ
  compose.yaml — декларативное описание стека. Три верхних ключа:
  services (что запускать), networks (как связаны), volumes
  (где данные). Всё остальное — внутри сервисов.
  За простотой скрывается 30+ параметров сервиса. Неправильная
  структура = сервис не поднимается, не видит БД, не перезапускается
  или роняет соседей.

  СОДЕРЖАНИЕ:
    1.  Структура верхнего уровня
    2.  image vs build
    3.  ports vs expose
    4.  environment и env_file
    5.  command и entrypoint
    6.  volumes и networks в сервисе
    7.  depends_on и healthcheck
    8.  restart, stop_grace_period, stop_signal
    9.  deploy, logging, profiles
    10. Частые ошибки
    11. Антипаттерны
    12. Финальные выводы

  1. СТРУКТУРА ВЕРХНЕГО УРОВНЯ
  ОБЯЗАТЕЛЬНЫЕ И ЧАСТО ИСПОЛЬЗУЕМЫЕ КЛЮЧИ:
    name:        # имя проекта (дефолт — имя папки)
    services:    # ОБЯЗАТЕЛЬНО. Что запускать.
    networks:    # опционально. Какие сети создавать.
    volumes:     # опционально. Named volumes.
    secrets:     # опционально. Docker secrets.
    configs:     # опционально. Docker configs.

  УСТАРЕВШИЕ:
    version:     # был обязателен в v1. В v2 — игнорируется.
                 # Убери, чтобы не было warning.

  СКЕЛЕТ:
    services:
      postgres:
        image: postgres:16-alpine
      app:
        build: .

    networks:
      frontend:
      backend:

    volumes:
      pgdata:

  SERVICES — обязательный ключ. Внутри — словарь сервисов.
  Имена сервисов — это DNS-имена внутри сети:
  `http://postgres:5432`. Lowercase, без пробелов.

  NETWORKS — если не описан, Compose создаёт одну `default`.
  Несколько сетей для изоляции:

    networks:
      frontend:
      backend:

    services:
      nginx:
        networks: [frontend, backend]
      postgres:
        networks: [backend]

  nginx видит postgres, postgres НЕ видит frontend.

  VOLUMES — named volumes, создаёт Docker.

    volumes:
      pgdata:
      redisdata:

  КЛЮЧ NAME — задаёт имя проекта:
    name: my-project

  От него зависят имена контейнеров, сетей, volumes:
    my-project_postgres_1
    my-project_default
    my-project_pgdata

  Без name — используется имя папки. Это может быть неудобно
  (например, папка называется `go-knowledge-base`).

  SECRETS И CONFIGS — отдельные темы, для прода:
    secrets:
      db_password:
        file: ./secrets/db_password.txt

    services:
      app:
        secrets:
          - db_password

  Секреты монтируются как файлы в /run/secrets/, не как env.
  Безопаснее, чем переменные (env видны в docker inspect).

  2. IMAGE VS BUILD
  Каждый сервис имеет image или build. Или оба.

  IMAGE — готовый образ из registry:

    services:
      postgres:
        image: postgres:16-alpine

  pull_policy: always | never | missing | build.

  ALWAYS — тянуть при каждом up. Для latest-тегов, которые
  должны всегда обновляться.

  NEVER — никогда не тянуть, только локальный образ. Для
  своих образов, собранных отдельно.

  MISSING — тянуть, если нет локально (дефолт).

  BUILD — собрать из Dockerfile:
    services:
      app:
        build:
          context: .
          dockerfile: Dockerfile
          args:
            VERSION: "1.0.0"
          target: prod    # для multi-stage

  CONTEXT — путь к папке с Dockerfile. Обычно `.` — корень
  проекта.

  DOCKERFILE — имя файла, если не `Dockerfile`. Полезно, если
  у тебя несколько Dockerfile в одной папке:

    Dockerfile
    Dockerfile.debug
    Dockerfile.test

  ARGS — build arguments. Передаются в ARG в Dockerfile:

    # в Dockerfile
    ARG VERSION=dev
    RUN go build -ldflags="-X main.version=${VERSION}"

    # в compose.yaml
    build:
      args:
        VERSION: "1.0.0"

  TARGET — какой stage собирать (в multi-stage):
    # в Dockerfile
    FROM golang:1.22-alpine AS builder
    FROM alpine:3.19 AS prod

    # в compose.yaml
    build:
      target: prod

  IMAGE + BUILD вместе — собрать и присвоить тег:
    services:
      app:
        image: my-app:1.0.0
        build: .

  Compose соберёт образ и присвоит ему тег. Полезно для
  дальнейшего пуша или запуска по тегу.

  ПРАВИЛО: внешние сервисы (Postgres, Redis, Kafka) — image.
  Свой сервис — build + image (с тегом).

  3. PORTS VS EXPOSE
  PORTS — публикация наружу (с хоста):

    ports:
      - "8080:8080"              # хост:контейнер
      - "127.0.0.1:9000:9000"    # только с localhost
      - "5432"                   # случайный порт хоста

  Формат "0.0.0.0:8080:8080" — доступен с любой сетевой карты
  (опасно в интернете). "127.0.0.1:8080:8080" — только с
  localhost (безопаснее).

  UDP-порты:
    ports:
      - "53:53/udp"

  Диапазон портов:
    ports:
      - "8000-8010:8000-8010"

  Случайный порт хоста (Docker выберет сам):
    ports:
      - "8080"    # контейнер слушает 8080, хост — случайный

  Полезно, если не важно, какой порт на хосте, и надо избежать
  конфликтов.

  EXPOSE — только внутри Compose-сети:

    expose:
      - "5432"

  Наружу не торчит. Дефолт для внутренних сервисов.

  ЧТО ПРОИСХОДИТ, ЕСЛИ НЕ УКАЗЫВАТЬ НИЧЕГО:
    Все порты контейнера доступны другим контейнерам в той же
    сети. Docker не блокирует по умолчанию.
    Но снаружи (с хоста) — недоступен. Разве что через
    published ports.

  expose — это документация + потенциальное использование
  балансировщиками. Реальная защита — отсутствие ports.

  ПРАВИЛО:
    • Наружу — только API-gateway / nginx / frontend.
    • Postgres, Redis, Kafka — expose или вообще ничего.
    • Локально, если нужен доступ с хоста — 127.0.0.1:PORT.

  ПРИМЕР С РАЗНЫМИ ТИПАМИ ПОРТОВ:
    services:
      nginx:
        ports:
          - "80:80"
          - "443:443"

      postgres:
        expose:
          - "5432"

      redis:
        # вообще ничего — доступен внутри сети по дефолту.

      debug:
        # в dev открываем наружу через override:
        ports:
          - "127.0.0.1:9229:9229"    # только с localhost

  4. ENVIRONMENT И ENV_FILE
  ENVIRONMENT — inline переменные:

    environment:
      APP_ENV: production
      LOG_LEVEL: info
      DB_PORT: "5432"    # числа в кавычках

  Формат map (KEY: value) предпочтительнее list (KEY=value).

  ENV_FILE — из файла:
    env_file:
      - .env
      - .env.app

  Файл `.env`: `APP_ENV=production`, без кавычек (иначе кавычки
  станут частью значения), без пробелов вокруг `=`.

  Несколько env_file — сливаются в порядке. Последний
  перезаписывает предыдущий.

  Опциональный env_file:
    env_file:
      - path: .env.app
        required: false

  Без `required: false` Compose упадёт, если файла нет.

  ПРИОРИТЕТ:
    environment > env_file > ENV из образа

  Переменная без значения берётся из окружения хоста:

    environment:
      PATH:      # взять из хоста
      HOME: ${HOME}    # явно из хоста

  ПОДСТАНОВКА В YAML:
    services:
      app:
        image: "my-app:${TAG:-latest}"
        environment:
          DB_PASSWORD: "${DB_PASSWORD:?required}"

  Формы:
    ${VAR}              — простая.
    ${VAR:-default}     — default, если не задано.
    ${VAR-default}      — default, если не определено.
    ${VAR:?error}       — ошибка и стоп, если не задано.

  ГДЕ ХРАНИТЬ СЕКРЕТЫ:
    НЕ В environment (видны в docker inspect).
    НЕ В env_file (файл в git).
    В Docker secrets или внешних системах (Vault).

  5. COMMAND И ENTRYPOINT
  Переопределяют то, что было в Dockerfile.

    services:
      api:
        image: app:1.0
        command: ["server"]           # CMD из образа заменён
      worker:
        image: app:1.0
        command: ["worker"]
      migrate:
        image: app:1.0
        entrypoint: ["/app/migrate"]
        command: ["up"]
        restart: "no"

  ДЛЯ GO — exec-форма (массив), не shell-строка. Shell ломает
  graceful shutdown.

  ОДИН ОБРАЗ — РАЗНЫЕ РОЛИ — стандартный паттерн через command.
  Один Dockerfile, три сервиса в compose.

  ПРИМЕР С АРГУМЕНТАМИ:
    services:
      app:
        image: app:1.0
        command: ["server", "--port", "8080", "--log-level", "debug"]

  ПРИМЕР С ПЕРЕМЕННОЙ:
    command: ["server", "--port", "${PORT:-8080}"]

  6. VOLUMES И NETWORKS В СЕРВИСЕ
  NAMED VOLUME — данные живут вне контейнера:
    volumes:
      - pgdata:/var/lib/postgresql/data

  BIND MOUNT — папка хоста в контейнер:
    volumes:
      - ./src:/app/src
      - ./config.yaml:/etc/app/config.yaml:ro    # read-only

  Режимы: `:ro` (read-only), `:z` (SELinux), `:cached` (macOS).

  NAMED — для данных. BIND — для разработки. `:ro` — для конфигов.

  АНОНИМНЫЙ VOLUME (плохо):
    volumes:
      - /app/data    # без источника

  Docker создаёт анонимный volume. Копится мусор. Используй
  named.

  ПОРЯДОК МОНТИРОВАНИЯ ВАЖЕН:

    volumes:
      - ./config:/app/config       # более специфичный
      - ./:/app                    # менее специфичный

  Более специфичный должен быть позже. Иначе перекроется.

  NETWORKS в сервисе:

    services:
      nginx:
        networks:
          - frontend
          - backend

      postgres:
        networks:
          - backend

  Сервис в нескольких сетях — мост между ними. Без networks —
  попадает в `default`.

  Алиасы:
    postgres:
      networks:
        backend:
          aliases:
            - db
            - database

  Теперь `ping db` и `ping database` работают.

  ПРИОРИТЕТ IPv4:
    networks:
      backend:
        ipv4_address: 172.20.0.10

  Можно задать статический IP, если нужен.

  7. DEPENDS_ON И HEALTHCHECK
  КОРОТКАЯ ФОРМА (плохая):
    depends_on:
      - postgres

  Ждёт только старта контейнера. Не готовности.

  ДЛИННАЯ ФОРМА (правильная):
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully

  Условия:
    • service_started — контейнер запущен.
    • service_healthy — healthcheck прошёл.
    • service_completed_successfully — завершился с кодом 0.

  HEALTHCHECK:

    postgres:
      healthcheck:
        test: ["CMD-SHELL", "pg_isready -U app"]
        interval: 5s
        timeout: 3s
        retries: 5
        start_period: 10s

  Параметры:
    • test — команда (CMD или CMD-SHELL).
    • interval — как часто (дефолт 30s).
    • timeout — сколько ждать ответа (дефолт 30s).
    • retries — до unhealthy (дефолт 3).
    • start_period — грейс при старте.

  БЕЗ HEALTHCHECK condition: service_healthy НЕ РАБОТАЕТ.
  Compose не знает, что считать «готов».

  ЧАСТЫЕ HEALTHCHECK:
    postgres: pg_isready -U user
    redis:    redis-cli ping
    mysql:    mysqladmin ping -h localhost
    nginx:    wget -q -O - http://localhost/health
    kafka:    kafka-topics.sh --bootstrap-server localhost:9092 --list
    go-distroless: ["CMD", "/app/server", "healthcheck"]

  ВАЖНО: depends_on работает только при старте. Если Postgres
  упадёт после — app не перезапустится.

  ПРИМЕР С МИГРАЦИЯМИ:
    services:
      migrate:
        image: app:1.0
        command: ["migrate", "up"]
        depends_on:
          postgres: { condition: service_healthy }
        restart: "no"

      app:
        image: app:1.0
        depends_on:
          migrate: { condition: service_completed_successfully }

  8. RESTART, STOP_GRACE_PERIOD, STOP_SIGNAL
  RESTART — что делать при падении:
    restart: "no"              # не перезапускать (дефолт)
    restart: always            # всегда
    restart: on-failure        # при ненулевом exit code
    restart: on-failure:5      # с лимитом 5
    restart: unless-stopped    # кроме явного stop

  ВЫБОР:
    • Сервис в проде → unless-stopped.
    • Одноразовая задача (миграции) → "no".
    • Разработка → "no" (не поднимать сломанный код).

  ПОЧЕМУ НЕ ALWAYS:
    docker compose stop app    # явно остановили.
    # Перезагрузка хоста.
    # Docker daemon стартует.
    # always → контейнер поднимается заново.
    # unless-stopped → остаётся остановленным.

  always опасен в проде. Всегда используй unless-stopped.

  STOP_SIGNAL — какой сигнал послать первым:

    stop_signal: SIGTERM    # дефолт

  STOP_GRACE_PERIOD — сколько ждать после SIGTERM перед SIGKILL:

    stop_grace_period: 10s

  По умолчанию 10 секунд. Для Go-сервиса с graceful shutdown
  этого может не хватить, если запросы долгие.

  ПРАВИЛЬНАЯ КОМБИНАЦИЯ:
    services:
      app:
        stop_grace_period: 30s
        stop_signal: SIGTERM

  9. DEPLOY, LOGGING, PROFILES
  DEPLOY — лимиты:
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 512M

  При превышении памяти — OOM-kill. Без лимитов один контейнер
  может съесть всю память хоста.

  replicas в Compose v2 без Swarm игнорируется. Используй:

    docker compose up -d --scale app=3

  LOGGING — ротация:
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

  Один файл ≤ 10 МБ, максимум 3 файла (30 МБ на сервис). Без
  этого логи забивают диск.

  ДРУГИЕ ДРАЙВЕРЫ:
    local, syslog, journald, fluentd, gelf.

  PROFILES — включать по требованию:
    jaeger:
      image: jaegertracing/all-in-one:1.62.0
      profiles: [trace]

    adminer:
      image: adminer:4
      profiles: [tools]

  Запуск:
    docker compose up -d                      # без jaeger/adminer
    docker compose --profile trace up -d
    docker compose --profile tools --profile trace up -d

  Сервисы без profiles — в профиле `default`, запускаются всегда.

  ПОЛЕЗНЫЕ ПРОФИЛИ:

    • trace — Jaeger, Tempo.
    • tools — adminer, redis-commander.
    • monitoring — Prometheus, Grafana.
    • migrate — сервисы миграций.
    • load — нагрузочные генераторы.

  10. ЧАСТЫЕ ОШИБКИ
  10.1. VERSION: В YAML.
    Устарело в v2. Warning при каждом up. Убери.
  10.2. ТАБЫ ВМЕСТО ПРОБЕЛОВ.
    YAML запрещает табы для отступов. Только пробелы (2).
  10.3. LIST И MAP В ENVIRONMENT.
    Не смешивай формы:
      # Плохо:
      environment:
        - APP_ENV=dev
        LOG_LEVEL: info
      # Хорошо:
      environment:
        APP_ENV: dev
        LOG_LEVEL: info
  10.4. ЧИСЛА БЕЗ КАВЫЧЕК.
    DB_PORT: 5432 (число) → Compose ругается. Пиши "5432".
  10.5. НЕТ HEALTHCHECK У ЗАВИСИМОСТИ.
    condition: service_healthy не работает без healthcheck.
  10.6. ПОРТЫ НАРУЖУ У ВНУТРЕННИХ.
    Postgres на 0.0.0.0:5432 в проде — дыра. Только expose.
  10.7. VOLUME НЕ ОПИСАН В ВЕРХНЕМ КЛЮЧЕ.
    Named volume в сервисе, но не в верхнем volumes — Compose
    создаст анонимный. Потом забудешь удалить.
  10.8. NETWORK НЕ ОПИСАН.
    Если в сервисе указана сеть, а в верхнем networks её нет —
    Compose создаст как external или упадёт.
  10.9. ИМЯ СЕРВИСА С ЗАГЛАВНОЙ.
    Postgres и postgres — разные DNS. Только lowercase.
  10.10. ЗАБЫЛИ DEPENDS_ON.
    App стартует раньше БД → падает. Добавь condition.


  11. АНТИПАТТЕРНЫ
  11.1. VERSION: В YAML.
  11.2. ОДИН ФАЙЛ НА ВСЁ (DEV + PROD).
    Разные требования. Override или отдельные файлы.
  11.3. ПОРТЫ ВСЕХ СЕРВИСОВ НАРУЖУ.
    Только API-gateway. Внутренние — expose.
  11.4. LATEST В IMAGE.
    Внезапное обновление ломает совместимость.
  11.5. DEPENDS_ON БЕЗ HEALTHCHECK.
    Ждёт старта, не готовности.
  11.6. НЕТ STOP_GRACE_PERIOD.
    Go-сервис не успевает graceful shutdown.
  11.7. НЕТ ЛИМИТОВ.
    Один контейнер съедает всю память хоста.
  11.8. НЕТ РОТАЦИИ ЛОГОВ.
    Диск забивается за неделю.
  11.9. HEALTHCHECK КАЖДУЮ СЕКУНДУ.
    Нагрузка на сервис и Docker. 10-30 секунд — норм.
  11.10. HARDCODED ПАРОЛИ В YAML.
    Файл в git. Используй .env или secrets.
  11.11. RESTART: ALWAYS ДЛЯ ВСЕГО.
    После перезагрузки хоста поднимаются даже остановленные.
    Используй unless-stopped.
  11.12. НЕТ ПРОФИЛЕЙ ДЛЯ ИНСТРУМЕНТОВ.
    Jaeger, adminer запускаются всегда.

  12. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Структура: три верхних ключа — services, networks,
      volumes. version: устарел.
  2.  services — обязательный. Внутри — именованные сервисы в lowercase.
  3.  image или build. Внешние — image, свои — build + image.
  4.  ports — наружу. expose — внутри. Внутренние не публикуй.
  5.  environment (map, числа в кавычках) + env_file. Приоритет:
      environment > env_file > ENV из образа.
  6.  command и entrypoint — exec-форма для Go.
  7.  volumes: named для данных, bind для dev, :ro для конфигов.
  8.  networks: одна или несколько. Изоляция через раздельные.
  9.  depends_on с condition: service_healthy — правильный
      порядок. Без healthcheck не работает.
  10. healthcheck: pg_isready, redis-cli ping, wget.
      start_period для медленно стартующих.
  11. restart: unless-stopped. stop_grace_period: 30s.
  12. deploy: limits для ресурсов. logging: max-size, max-file.
  13. profiles: инструменты по требованию.
  14. Частые ошибки: version:, табы, list вместо map, числа
      без кавычек, нет depends_on, нет healthcheck.
  15. Антипаттерны: один файл на всё, latest, hardcoded
      пароли, порты наружу, restart: always.
*/
