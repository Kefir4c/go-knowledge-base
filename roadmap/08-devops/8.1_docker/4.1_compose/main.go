package main

/*
  УРОК 4.1: ЧТО ТАКОЕ DOCKER COMPOSE И ЗАЧЕМ
  Один контейнер — это просто. docker run, и всё работает.
  Два контейнера — уже сложнее: нужно связать их сетью, передать
  переменные, дождаться готовности. Пять контейнеров — это уже
  ад из docker run команд в bash-скрипте. Двадцать — нереально.
  Docker Compose решает эту проблему. Ты описываешь весь стек
  в одном YAML-файле, и одной командой поднимаешь всё: сервис,
  БД, кэш, брокер, миграции. Так же одной командой всё
  останавливаешь.
  Compose — не только для локальной разработки. Это ещё и
  стандарт для интеграционных тестов, CI-пайплайнов и небольших
  продовых деплоев на одном сервере.

  СОДЕРЖАНИЕ:
    1.  Проблема: жизнь без Compose
    2.  Что такое Compose
    3.  Декларативный подход vs императивный
    4.  История и версии: docker-compose vs docker compose
    5.  Compose Specification — открытый стандарт
    6.  Основные команды и жизненный цикл
    7.  Именование проекта
    8.  Многофайловый Compose: base + override
    9.  Что можно описать в compose.yaml
    10. Переменные и интерполяция
    11. Compose как код: коммит в git
    12. Где Compose используют
    13. Compose в CI
    14. Compose НЕ для прода
    15. Compose vs Docker Swarm vs Kubernetes
    16. Практика: минимальный compose для Go-сервиса
    17. Частые ошибки новичков
    18. Антипаттерны
    19. Финальные выводы

  1. ПРОБЛЕМА: ЖИЗНЬ БЕЗ COMPOSE
  Типичный Go-сервис. Ему нужны PostgreSQL, Redis, Kafka,
  Jaeger. Без Compose ты пишешь bash-скрипт:

    #!/bin/bash
    docker network create app-net

    docker run -d --name postgres --network app-net \
      -e POSTGRES_PASSWORD=secret \
      -v pgdata:/var/lib/postgresql/data postgres:16

    docker run -d --name redis --network app-net redis:7-alpine

    docker run -d --name kafka --network app-net -p 9092:9092 \
      -e KAFKA_NODE_ID=1 -e KAFKA_PROCESS_ROLES=broker,controller \
      apache/kafka:3.7.0

    docker run -d --name app --network app-net -p 8080:8080 \
      -e DB_HOST=postgres -e REDIS_HOST=redis my-app:1.0

  ЧТО ПЛОХО:
    • Много повторения (--network, имена).
    • Легко забыть флаг или параметр.
    • Не декларативно. Нельзя посмотреть и понять «что должно быть».
    • Остановить всё — руками.
    • Поделиться с коллегой — «возьми скрипт, но у меня там нестандарт».
    • Обновить образ — найти нужный docker run и запустить заново.

  В команде из 10 человек у каждого своё окружение. У одного
  Redis на 6380, у другого — 6379. Баги не воспроизводятся.
  Compose решает это радикально.

  2. ЧТО ТАКОЕ COMPOSE
  Compose — инструмент для определения и запуска
  многоконтейнерных приложений. Всё в одном файле compose.yaml
  (старое имя — docker-compose.yml).

    services:
      postgres:
        image: postgres:16-alpine
        environment:
          POSTGRES_PASSWORD: secret
        volumes:
          - pgdata:/var/lib/postgresql/data

      redis:
        image: redis:7-alpine

      app:
        build: .
        ports:
          - "8080:8080"
        environment:
          DB_HOST: postgres
          REDIS_HOST: redis

    volumes:
      pgdata:

  ОДНА КОМАНДА:
    docker compose up -d

  Compose создаёт сеть, volumes, скачивает образы, запускает
  контейнеры в правильном порядке, пробрасывает порты,
  передаёт переменные.

  ОСТАНОВКА:
    docker compose down

  3. ДЕКЛАРАТИВНЫЙ ПОДХОД VS ИМПЕРАТИВНЫЙ
  ИМПЕРАТИВНЫЙ (docker run):
    • Пишешь «что делать».
    • Каждая команда меняет состояние.
    • Ошибся в середине — начинай сначала.

  ДЕКЛАРАТИВНЫЙ (Compose):
    • Пишешь «что должно быть».
    • Compose сам решает, как этого достичь.
    • Файл — единственный источник истины.
    • Изменил файл — Compose разберётся, что обновить.

  АНАЛОГИЯ:
    • Императивно: «включи свет, открой дверь, поставь чайник».
    • Декларативно: «должно быть светло, дверь открыта, чайник горячий».

  ИДЕМПОТЕНТНОСТЬ:
    docker compose up -d       # запустил
    docker compose up -d       # ничего не делает
    # Поменял версию образа в yaml.
    docker compose up -d       # пересоздаст только изменённый.

  С docker run так не работает. Каждый запуск — новый контейнер.

  4. ИСТОРИЯ И ВЕРСИИ: DOCKER-COMPOSE VS DOCKER COMPOSE
  COMPOSE V1 (2014-2023):
    • Написан на Python.
    • Отдельный бинарник docker-compose (с дефисом).
    • Формат: version: "3.8" в yaml.
    • Устанавливался через pip отдельно от Docker.

  COMPOSE V2 (2020 - настоящее время):
    • Написан на Go.
    • Встроен в Docker CLI как плагин.
    • Команда: docker compose (без дефиса).
    • version: устарел, не нужен.
    • Активно развивается.

  ЧТО ИСПОЛЬЗОВАТЬ:
    docker compose up -d          ← правильно
    docker-compose up -d          ← старые туториалы

  КАК ПРОВЕРИТЬ СВОЮ ВЕРСИЮ:
    docker compose version
    # Docker Compose version v2.24.0

  Если у тебя docker-compose (с дефисом) — это старая v1.
  Она уже не поддерживается. Обнови Docker Desktop.
  ПРАВИЛО: docker compose (без дефиса), без version: в yaml.

  5. COMPOSE SPECIFICATION — ОТКРЫТЫЙ СТАНДАРТ
  В 2020 Docker передал спецификацию Compose в OCI как
  открытый стандарт — Compose Specification.

  ЧТО ЭТО ДАЁТ:
    • Не привязан к Docker. Любая система может реализовать.
    • PODMAN COMPOSE — drop-in замена. Читает тот же yaml.
    • AWS COPILOT, AZURE CONTAINER APPS, CLOUD RUN — умеют.
    • DOCKER COMPOSE (v2) — референсная реализация.

  compose.yaml — стандартный файл описания многоконтейнерного
  приложения. Не «файл для Docker». Работает с Podman, ECS,
  Cloud Run без изменений.

  На собесе: «Compose — открытый стандарт под OCI. Docker
  Compose — референсная реализация. compose.yaml переносим
  между инструментами».

  6. ОСНОВНЫЕ КОМАНДЫ И ЖИЗНЕННЫЙ ЦИКЛ
  ВСЕ КОМАНДЫ ЗАПУСКАЮТСЯ ИЗ ПАПКИ С compose.yaml.

  UP — ПОДНЯТЬ СТЕК:
    docker compose up              # foreground, логи в терминал
    docker compose up -d           # detached
    docker compose up app          # только сервис app
    docker compose up --build      # пересобрать образы

  DOWN — ОСТАНОВИТЬ И УДАЛИТЬ:
    docker compose down              # удалить контейнеры и сеть
    docker compose down -v           # + удалить volumes
    docker compose down --rmi all    # + удалить образы

  PS, LOGS, EXEC:
    docker compose ps                # запущенные
    docker compose ps -a             # все
    docker compose logs -f app       # follow логи
    docker compose logs --tail 100   # последние 100
    docker compose exec app sh       # shell в app

  BUILD, RESTART, STOP, START:
    docker compose build             # собрать
    docker compose restart app       # перезапуск
    docker compose stop              # стоп без удаления
    docker compose start             # старт остановленного

  WATCH — HOT RELOAD (v2.22+):
    docker compose watch

  ДОПОЛНИТЕЛЬНЫЕ:
    docker compose config            # итоговый yaml после merge
    docker compose top               # процессы
    docker compose pull              # скачать образы
    docker compose cp app:/log /tmp  # копирование файлов

  ГЛАВНЫЕ 90% РАБОТЫ:
    docker compose up -d
    docker compose down
    docker compose ps
    docker compose logs -f app
    docker compose exec app sh

  ЖИЗНЕННЫЙ ЦИКЛ СТЕКА:
    NOT CREATED  → ничего не запущено.
    CREATED      → контейнеры созданы, не запущены.
    RUNNING      → контейнеры работают.
    STOPPED      → остановлены, не удалены (docker compose stop).
    REMOVED      → контейнеры и сеть удалены (docker compose down).

  РАЗНИЦА STOP И DOWN:
    stop  → контейнеры остановлены, состояние сохранено.
            Быстрый start вернёт всё как было.
    down  → контейнеры удалены, сеть удалена.
            Volumes остаются (если не указан -v).

  КОГДА ЧТО:
    stop  → временная остановка.
    down  → завершение работы.
    down -v → полная очистка с данными.

  7. ИМЕНОВАНИЕ ПРОЕКТА
  Compose использует «имя проекта» для группировки ресурсов.

  ОТКУДА БЕРЁТСЯ:
    1. Флаг -p/--project-name.
    2. Переменная COMPOSE_PROJECT_NAME.
    3. Ключ name: в compose.yaml.
    4. Имя папки (дефолт).

  ЧТО ПОЛУЧАЕТ ИМЯ:
    • Контейнеры: <project>_<service>_<num> (myapp_postgres_1).
    • Сеть: <project>_default (myapp_default).
    • Volumes: <project>_<volumename> (myapp_pgdata).

  ЗАЧЕМ ЗНАТЬ:
    • 5 проектов на одной машине — у каждого своя сеть
      и volumes. Не пересекаются.
    • Монорепо — даёшь каждому compose своё имя.
    • CI — COMPOSE_PROJECT_NAME=pr-123, чтобы job'ы не
      конфликтовали.

  ПЕРЕОПРЕДЕЛЕНИЕ:
    docker compose -p myapp up -d
    # или
    name: myapp    # в compose.yaml

  ПРИМЕР КОНФЛИКТА БЕЗ ПЕРЕОПРЕДЕЛЕНИЯ:

    У тебя проект в папке my-project. Запускаешь compose up.
    Создаётся контейнер my-project-postgres-1.

    Коллега клонирует проект в my-project-copy, запускает
    compose up. Создаётся my-project-copy-postgres-1.

    Работают параллельно. Никаких конфликтов.

    Но если оба запустят compose с флагом -p shared, оба
    будут пытаться создать shared_postgres_1. Второй упадёт
    с ошибкой «name already in use».

    Поэтому в CI всегда задавай уникальное имя проекта.

  8. МНОГОФАЙЛОВЫЙ COMPOSE: BASE + OVERRIDE
  Compose поддерживает слияние нескольких файлов. Это
  стандартный способ разделять dev, staging и prod.

  АВТОМАТИЧЕСКОЕ СЛИЯНИЕ:
    Файл compose.yaml — база.
    Файл compose.override.yml — автоматически подхватывается
    при `docker compose up`. Ничего указывать не надо.

    Используется для локальных настроек, которые не коммитятся.

  ЯВНОЕ УКАЗАНИЕ ЧЕРЕЗ -f:
    docker compose -f compose.yaml -f compose.prod.yml up -d

    Файлы сливаются слева направо. Последний побеждает.

  ПРИМЕР:
    compose.yaml (база):
      services:
        app:
          image: my-app:${TAG:-latest}
          environment:
            APP_ENV: production

    compose.override.yml (для локальной разработки):
      services:
        app:
          build: .
          volumes:
            - ./src:/app/src
          environment:
            APP_ENV: development

    Что произойдёт при docker compose up:
      services:
        app:
          image: my-app:latest      ← из базы
          build: .                   ← из override
          volumes:
            - ./src:/app/src         ← из override
          environment:
            APP_ENV: development     ← override побеждает

  ПРАВИЛА СЛИЯНИЯ:
    • Скаляры (строки, числа) — последнее значение.
    • Списки — объединяются.
    • Словари — рекурсивно сливаются.

  ЧТО ХРАНИТЬ В OVERRIDE:
    • build: . (сборка локально вместо pulling).
    • volumes: bind mount с исходниками.
    • ports: локальные порты (если у тебя заняты стандартные).
    • environment: APP_ENV=development.
    • command: переопределение для hot reload.

  ЧТО НЕ ХРАНИТЬ В OVERRIDE:
    • Секреты (это в .env).
    • Прод-специфику (это в compose.prod.yml).
    • Всё, что должно быть у всех одинаково.

  OVERRIDE В GITIGNORE:
    compose.override.yml — личные настройки. Не коммитится.
    Но пример можно положить как compose.override.yml.example,
    чтобы новый разработчик знал, что там писать.

  9. ЧТО МОЖНО ОПИСАТЬ В COMPOSE.YAML
  ТРИ ВЕРХНИХ КЛЮЧА:
    services:      # контейнеры — что запускать
    networks:      # сети — как связаны
    volumes:       # volumes — где хранятся данные

  ВНУТРИ СЕРВИСА:
    build:           # Dockerfile, args
    image:           # какой образ
    ports:           # публикация портов (хост:контейнер)
    expose:          # порты только для внутренней сети
    environment:     # переменные
    env_file:        # переменные из файла
    volumes:         # монтирование
    networks:        # в каких сетях
    depends_on:      # порядок запуска + healthcheck
    restart:         # политика перезапуска
    command:         # переопределить CMD
    entrypoint:      # переопределить ENTRYPOINT
    healthcheck:     # проверка здоровья
    stop_grace_period:  # сколько ждать при stop
    stop_signal:     # какой сигнал послать
    user:            # под каким UID
    deploy:          # лимиты, replicas
    logging:         # настройки логирования
    profiles:        # включать по требованию

  НА СТАРТЕ ХВАТИТ 5-7 КЛЮЧЕЙ:
    image/build, ports, environment, volumes, depends_on,
    networks, healthcheck.

  10. ПЕРЕМЕННЫЕ И ИНТЕРПОЛЯЦИЯ
  Compose поддерживает подстановку переменных в yaml.

  ФОРМЫ:
    ${VAR}              — простая подстановка. Если нет —
                          пустая строка + warning.
    ${VAR:-default}     — если не задана, использовать default.
    ${VAR-default}      — если не определена (не пустая),
                          использовать default.
    ${VAR:?error}       — если не задана, ошибка и стоп.
                          Для обязательных переменных.

  ПРИМЕР:
    services:
      app:
        image: "my-app:${TAG:-latest}"
        ports:
          - "${APP_PORT:-8080}:8080"
        environment:
          DB_PASSWORD: "${DB_PASSWORD:?DB_PASSWORD required}"

  ОТКУДА БЕРУТСЯ ЗНАЧЕНИЯ (по приоритету):
    1. Переменные окружения оболочки (export VAR=value).
    2. Файл .env в папке проекта.
    3. Файл, указанный через --env-file.
    4. Дефолты в compose.yaml.

  ФАЙЛ .env:
    TAG=1.2.3
    APP_PORT=9000
    DB_PASSWORD=supersecret

  ФАЙЛ .ENV.EXAMPLE (коммитится):
    # Скопируй в .env и заполни.
    TAG=latest
    APP_PORT=8080
    DB_PASSWORD=changeme

  ЧАСТАЯ ОШИБКА — КАВЫЧКИ В .ENV:
    DB_PASSWORD="secret"     ← кавычки станут частью значения!

    .env не умеет обрабатывать кавычки как bash.
    Значение берётся буквально. Пиши без кавычек.

  ЕЩЁ ОДНА ОШИБКА — ПРОБЕЛЫ ВОКРУГ =:
    DB_PASSWORD = secret     ← пробелы станут частью ключа!

    Пиши плотно: DB_PASSWORD=secret

  11. COMPOSE КАК КОД: КОММИТ В GIT
  compose.yaml коммитится в git вместе с кодом.

    my-project/
    ├── compose.yaml
    ├── compose.override.yml     # локальные (не коммитится)
    ├── .env                     # секреты (не коммитится)
    ├── .env.example             # шаблон для коллег
    ├── Dockerfile
    └── main.go

  ГЛАВНАЯ ЦЕННОСТЬ:
    Новый разработчик клонирует репо. Читает README: «запусти
    docker compose up». Через минуту у него работает весь стек.
    Не надо ставить Postgres, Redis, Kafka вручную.

    Onboarding: было 2 дня, стало 5 минут.

  ЧТО В GITIGNORE:
    .env
    compose.override.yml
    data/
    *.log

  12. ГДЕ COMPOSE ИСПОЛЬЗУЮТ

  12.1. ЛОКАЛЬНАЯ РАЗРАБОТКА.
    Главный сценарий. В dev часто НЕ ВСЁ через Compose:
    свой сервис запускаешь через go run, а зависимости —
    через Compose.

      docker compose up -d postgres redis kafka
      go run ./cmd/order

    Быстрая итерация без пересборки образа. Альтернатива —
    всё через Compose с hot reload (см. docker compose watch).

  12.2. ИНТЕГРАЦИОННЫЕ ТЕСТЫ.
    testcontainers, dockertest или чистый Compose.

      docker compose -f compose.test.yml up -d
      go test -tags=integration ./...
      docker compose -f compose.test.yml down -v

    В CI стандартный паттерн. Каждый тест — чистый стек.

  12.3. НЕБОЛЬШИЕ ПРОДОВЫЕ ДЕПЛОИ.
    На одном сервере без Kubernetes. Пет-проекты, маленькие
    стартапы, внутренние сервисы. Без HA и автоскейла.

  13. COMPOSE В CI
  GITHUB ACTIONS:

    jobs:
      test:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4

          - name: Start dependencies
            run: docker compose up -d postgres redis

          - name: Wait for healthy
            run: |
              timeout 60 sh -c 'until docker compose ps | grep -q healthy; do sleep 1; done'

          - name: Run tests
            run: go test -tags=integration ./...

          - name: Stop
            if: always()
            run: docker compose down -v

  ЧТО ВАЖНО:
    • `-d postgres redis` — только зависимости, не сервис.
    • `timeout 60` — не ждём бесконечно.
    • `if: always()` — clean up даже при провале.
    • Уникальный COMPOSE_PROJECT_NAME — иначе конфликты.

  АЛЬТЕРНАТИВА — SERVICE CONTAINERS:
    GitHub умеет поднимать контейнеры своим API без compose:

      services:
        postgres:
          image: postgres:16-alpine
          env: { POSTGRES_PASSWORD: secret }
          ports: ["5432:5432"]
          options: >-
            --health-cmd "pg_isready -U postgres"

  ПРАВИЛО: 1-2 сервисов — service containers. Сложный стек —
  compose.

  14. COMPOSE НЕ ДЛЯ ПРОДА
  ЧТО COMPOSE НЕ УМЕЕТ:
    • HIGH AVAILABILITY. Один хост — точка отказа.
    • AUTO-SCALING. Все реплики на одном хосте.
    • ROLLING UPDATES. Обновление = короткий downtime.
    • SELF-HEALING. Нода упала — контейнеры не переедут.
    • MULTI-HOST. Compose работает на одном хосте.

  ЧТО УМЕЕТ И ЧЕМ ОПРАВДАН В МАЛОМ ПРОДЕ:
    • Restart policies — упал, поднялся.
    • Healthchecks — проверка здоровья.
    • Resource limits — CPU/память.
    • Graceful shutdown — stop_grace_period.
    • Logs rotation — ограничение размеров.
    • Named volumes — persistent-данные.

  КОГДА ОПРАВДАН:
    • Личные проекты.
    • Внутренние сервисы для команды из 5 человек.
    • MVP стартапа без DevOps и кластера.
    • On-prem решения — клиент хочет «поставить и запустить».

  КОГДА НЕ ГОДИТСЯ:
    • SaaS с SLA. Нельзя downtime.
    • Высокие нагрузки. Нужны автоскейл и ноды.
    • Микросервисы 20+. Управлять вручную невозможно.
    • Регулируемые индустрии. Compliance требует HA и audit.

    Тогда — Kubernetes, Nomad, ECS, Cloud Run.

  ИНСТРУМЕНТЫ ДЛЯ УДОБСТВА COMPOSE В ПРОДЕ:
    • Coolify — самохостед PaaS с UI и деплоем из git.
    • Dokploy — аналог, активно развивается.
    • CapRover — старый, стабильный.
    • Portainer — UI для Docker и Compose.

    Не превращают Compose в Kubernetes, но упрощают деплой,
    логи, бэкапы, мониторинг.

  ПРАВИЛО: Compose — dev, тесты, малые деплои. Kubernetes —
  прод с серьёзными требованиями.

  15. COMPOSE VS DOCKER SWARM VS KUBERNETES
  DOCKER SWARM:
    • Встроенный в Docker оркестратор.
    • Умеет multi-host: несколько серверов в кластер.
    • Синтаксис почти как compose, но `deploy:`.
    • Статус: угасает. Docker Inc. рекомендует Kubernetes.
    • Использовать: только для legacy.

  KUBERNETES:
    • Полноценный оркестратор.
    • Multi-host, автоскейл, HA, rolling updates, self-healing.
    • Сложность: высокая. YAML для pods, services, ingress.
    • Использовать: прод с серьёзными требованиями.

  ┌──────────────────┬───────────┬─────────┬──────────────┐
  │                  │ Compose   │ Swarm   │ Kubernetes   │
  ├──────────────────┼───────────┼─────────┼──────────────┤
  │ Хосты            │ 1         │ Много   │ Много        │
  │ HA               │ Нет       │ Да      │ Да           │
  │ Автоскейл        │ Нет       │ Огранич.│ Да           │
  │ Rolling update   │ Нет       │ Да      │ Да           │
  │ Сложность        │ Низкая    │ Средняя │ Высокая      │
  │ Статус           │ Активен   │ Угасает │ Стандарт     │
  └──────────────────┴───────────┴─────────┴──────────────┘

  ГЛАВНОЕ: Compose и Kubernetes — не конкуренты. Compose
  для dev, Kubernetes для прода. Часто вместе.

16.
Compose НЕ для прода с серьёзными требованиями.
Чего у него нет:

      • HA (High Availability, высокая доступность) —
        способность системы работать, когда часть
        инфраструктуры упала. Если сервер выгорел,
        резервный подхватывает нагрузку, пользователь
        ничего не замечает. Требует несколько нод,
        мониторинг, автоматический перезапуск контейнеров
        на живой ноде. У Compose один хост — упал сервер,
        упало всё. У Kubernetes — под переезжает за секунды.

      • AUTO-SCALING (автоматическое масштабирование) —
        способность добавлять и убирать ресурсы под
        нагрузку. Ночью 10 RPS — 1 контейнер. В обед
        10000 RPS — 50 контейнеров. Всё само, по метрикам
        (CPU, RPS, latency). У Compose все реплики на
        одном хосте — даже 10 реплик не спасут от перегрузки.
        У Kubernetes HPA сам решает, сколько подов нужно.

      • ROLLING UPDATES (постепенное обновление без
        простоя) — способность обновлять версию без
        остановки сервиса. Деплоишь v2 — Kubernetes
        постепенно заменяет поды: поднял новый, дождался
        готовности (healthcheck), убил старый, повторил.
        Пользователь не замечает. У Compose обновление =
        короткий downtime: старые контейнеры гасятся,
        новые поднимаются.

      • MULTI-HOST (несколько серверов) — способность
        раскидывать контейнеры по нескольким машинам
        в кластере. Compose работает только на одном
        хосте. Точка. Kubernetes, Nomad, Swarm — умеют
        multi-host.

      • SELF-HEALING (самовосстановление) — если нода
        упала, контейнеры автоматически переезжают на
        живую. У Compose нода упала — контейнеры просто
        исчезли.

Стандартный сценарий: dev — Compose, prod —
Kubernetes. Это разные инструменты, не конкуренты.
Для небольших проектов (пет-проект, MVP, внутренний
сервис на 5 человек) Compose в проде допустим —
там HA и автоскейл не критичны».

  17. МИНИМАЛЬНЫЙ COMPOSE ДЛЯ GO-СЕРВИСА
  compose.yaml:

    services:
      postgres:
        image: postgres:16-alpine
        environment:
          POSTGRES_USER: app
          POSTGRES_PASSWORD: secret
          POSTGRES_DB: app
        volumes:
          - pgdata:/var/lib/postgresql/data
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U app"]
          interval: 5s
          timeout: 3s
          retries: 5

      redis:
        image: redis:7-alpine
        healthcheck:
          test: ["CMD", "redis-cli", "ping"]
          interval: 5s

      app:
        build: .
        ports:
          - "8080:8080"
        environment:
          DB_HOST: postgres
          DB_PORT: "5432"
          REDIS_HOST: redis
        depends_on:
          postgres:
            condition: service_healthy
          redis:
            condition: service_healthy
        restart: unless-stopped

    volumes:
      pgdata:

  ЗАПУСК:

    docker compose up -d

  ЧТО ПРОИСХОДИТ:
    1. Создаётся сеть my-project_default.
    2. Создаётся volume my-project_pgdata.
    3. Запускаются postgres и redis.
    4. Compose ждёт их healthcheck.
    5. Собирается образ app из Dockerfile.
    6. Запускается app с DB_HOST=postgres.
    7. Пробрасывается порт 8080.

  ПРОВЕРКА:
    docker compose ps
    # NAME        STATUS
    # app-1       Up (healthy)
    # postgres-1  Up (healthy)
    # redis-1     Up (healthy)

    docker compose logs -f app
    docker compose exec app sh

  ВНУТРИ APP:
    ping postgres
    # PING postgres (172.x.x.x): 56 data bytes
    # ← работает, потому что Compose создал DNS

  ОСТАНОВКА:
    docker compose down            # контейнеры удалены
    docker compose down -v         # + volume pgdata

  18. ЧАСТЫЕ ОШИБКИ НОВИЧКОВ
  18.1. ЗАПУСК НЕ ИЗ ПАПКИ С COMPOSE.YAML.
    Compose ищет файл в текущей папке. Или -f явно.
  18.2. ЗАБЫЛИ -d.
    Без -d логи идут в терминал. Ctrl+C убьёт всё.
  18.3. DOWN -V СЛУЧАЙНО.
    Удалит volumes. В dev не страшно, в проде — потеря данных.
  18.4. НЕ ЖДУТ HEALTHCHECK.
    Без `condition: service_healthy` сервис стартует
    раньше БД и падает.
  18.5. ПОРТЫ ЗАНЯТЫ.
    Postgres на 5432 уже есть на хосте. Compose упадёт.
    Меняй на `"5433:5432"`.
  18.6. ОТСУТСТВИЕ .ENV.EXAMPLE.
    Новый разработчик не знает, какие переменные нужны.
  18.7. COMPOSE.OVERRIDE.YML В GIT.
    Личные настройки в общем репо. Добавь в .gitignore.
  18.8. VERSION: В YAML.
    Warning при каждом up. Убери.

  19. АНТИПАТТЕРНЫ
  19.1. COMPOSE ДЛЯ ПРОДА БЕЗ ПОНИМАНИЯ ОГРАНИЧЕНИЙ.
    Нет HA и автоскейла. Не для критичных сервисов.
  19.2. HARDCODED ПАРОЛИ В COMPOSE.YAML.
    Файл в git. Пароли в открытом виде. Используй .env.
  19.3. НЕТ HEALTHCHECK.
    depends_on без healthcheck ждёт старта, не готовности.
  19.4. ОДИН COMPOSE НА ВСЁ: DEV + PROD.
    Разные требования. Разделяй через override.
  19.5. ПОРТЫ ВСЕХ СЕРВИСОВ НАРУЖУ.
    Postgres, Redis, Kafka не нужны снаружи. Только app.
  19.6. LATEST В ОБРАЗАХ.
    Может внезапно обновиться и сломать совместимость.
  19.7. DOWN -V БЕЗ ПОНИМАНИЯ.
    Удаляет volumes. В проде — потеря данных.
  19.8. VERSION: В YAML.
    В Compose v2 не нужен. Убери.
  19.9. КАВЫЧКИ В .ENV.
    DB_PASSWORD="secret" передаст кавычки как часть значения.
  19.10. ЗАБЫЛИ ПРО OVERRIDE В GIT.
    compose.override.yml — локальные. В .gitignore.
  19.11. НЕТ .DOCKERIGNORE ДЛЯ BUILD.
    Build context раздувается. Особенно в монорепо.
  19.12. ИМЯ ПРОЕКТА ПО УМОЛЧАНИЮ В CI.
    Несколько job'ов конфликтуют за имена. Задавай
    COMPOSE_PROJECT_NAME с уникальным значением.

  20. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Compose — декларативный YAML для запуска нескольких
      контейнеров одной командой.
  2.  Один up — весь стек. Один down — всё остановлено.
  3.  Декларативный подход лучше императивного. Файл —
      единственный источник истины.
  4.  Compose v2 (docker compose, без дефиса) — текущий
      стандарт. v1 (docker-compose) устарел.
  5.  Compose Specification — открытый стандарт OCI.
      Работает с Docker, Podman, Cloud Run, ECS.
  6.  Основные команды: up, down, ps, logs, exec.
      90% работы — эти пять.
  7.  Жизненный цикл: created → running → stopped → removed.
      Stop ≠ down. Stop сохраняет, down удаляет.
  8.  Имя проекта определяет имена всех ресурсов.
      По умолчанию — имя папки.
  9.  Многофайловый Compose: база + override. Автоматическое
      слияние или явное через -f.
  10. Три ключа: services, networks, volumes. Внутри
      сервисов — build/image, ports, environment, depends_on,
      healthcheck.
  11. Интерполяция: ${VAR}, ${VAR:-default}, ${VAR:?error}.
      Значения из env, .env, дефолтов. Кавычки в .env — зло.
  12. compose.yaml в git. .env и compose.override.yml — нет.
  13. Главная ценность — onboarding нового разработчика за
      5 минут вместо 2 дней.
  14. Используется: локальная разработка, тесты, CI,
      небольшие прод-деплои.
  15. Compose НЕ для прода с серьёзными требованиями.
      Нет HA, автоскейла, rolling updates, multi-host.
  16. Kubernetes — для прода. Compose и Kubernetes —
      разные инструменты, не конкуренты.
  17. Docker Swarm угасает. Не начинай на нём.
  18. Частые ошибки: запуск не из той папки, забыли -d,
      down -v случайно, нет healthcheck, порты заняты.
  19. Антипаттерны: Compose для прода, пароли в yaml,
      порты всех сервисов наружу, version: в yaml,
      кавычки в .env.
*/
