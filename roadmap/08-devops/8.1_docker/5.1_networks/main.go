package main

/*
  УРОК 5.1: NETWORKS В COMPOSE
  Без сетей контейнеры были бы одинокими островами. Каждый в своём
  namespace, со своим IP, не видящий соседей. Но микросервис без
  соседей — это не микросервис, а бесполезный процесс.
  Compose решает это элегантно: создаёт сеть, подключает к ней все
  сервисы, и даёт каждому DNS-имя. Теперь `postgres:5432` работает
  из любого контейнера — без хардкода IP, без service discovery.
  Но у сетей есть второй смысл — изоляция. Не все сервисы должны
  видеть друг друга. Frontend не имеет права стучаться напрямую в
  БД. Debug-инструменты не должны попадать в prod-сеть.
  Эта тема — про то, как правильно организовать сети в Compose:
  одна или несколько, кто кого видит, как работают алиасы и зачем
  нужны internal/external networks.

  СОДЕРЖАНИЕ:
    1.  Проблема: как без сети контейнеры общаются
    2.  Default сеть Compose
    3.  DNS между сервисами
    4.  Явные сети в compose.yaml
    5.  Изоляция через несколько сетей
    6.  Сервис в нескольких сетях (мост)
    7.  Aliases: дополнительные DNS-имена
    8.  External networks
    9.  Internal networks
    10. Управление сетями: ls, inspect, rm
    11. Именование и префиксы
    12. Диагностика проблем
    13. Антипаттерны
    14. Финальные выводы

  1. ПРОБЛЕМА: КАК БЕЗ СЕТИ КОНТЕЙНЕРЫ ОБЩАЮТСЯ
  Docker без Compose. Три контейнера. Хочешь, чтобы app видел
  postgres и redis. Пишешь руками:
    docker network create app-net

    docker run -d --name postgres --network app-net postgres:16
    docker run -d --name redis --network app-net redis:7
    docker run -d --name app --network app-net my-app:1.0

  Что происходит:
    • Docker создаёт bridge-сеть app-net.
    • Каждый контейнер в сети получает IP (например, 172.20.0.x).
    • Docker встраивает DNS: контейнеры видят друг друга по имени.
    • app может стучаться на `postgres:5432` и `redis:6379`.

  Это работает. Но:
    • Надо помнить про --network в каждом run.
    • Забыл — контейнер в default bridge, не видит соседей.
    • Default bridge (172.17.0.0/16) не имеет DNS по имени.
    • Приходится вручную создавать сеть, вести учёт.

  Compose делает это автоматически. Один `up` — сеть создана,
  все сервисы подключены, DNS работает.

  2. DEFAULT СЕТЬ COMPOSE
  Если в compose.yaml нет секции `networks`, Compose создаёт
  одну сеть автоматически.

    services:
      postgres:
        image: postgres:16-alpine
      redis:
        image: redis:7-alpine
      app:
        image: my-app:1.0

  Что произойдёт при `docker compose up`:
    • Создаётся сеть `<project>_default` (bridge driver).
    • Все три сервиса подключаются к ней.
    • DNS работает: `postgres`, `redis`, `app` резолвятся.

  Имя сети:
    <project>_default

  Где `<project>` — имя проекта (по умолчанию имя папки). Пример:

    compose-default_practice_default

  ПРОВЕРКА:
    docker network ls | Select-String compose
    # NETWORK ID     NAME                              DRIVER
    # abc123         my-project_default                bridge

  ВСЕ СЕРВИСЫ В ОДНОЙ СЕТИ ВИДЯТ ДРУГ ДРУГА:
    docker compose exec app ping postgres
    # PING postgres (172.x.x.x): 56 data bytes

  ЭТО ДЕФОЛТ. Работает из коробки. Но у него есть минус:
  **все видят всех**. Postgres видит app, app видит postgres,
  redis видит обоих. Никакой изоляции.

  3. DNS МЕЖДУ СЕРВИСАМИ
  В Compose-сети работает автоматический DNS. Это то, что делает
  Compose удобным.

  КАК ЭТО РАБОТАЕТ:
    • Docker поднимает встроенный DNS-сервер (127.0.0.11 внутри
      каждого контейнера).
    • При создании сервиса Docker регистрирует его имя в DNS.
    • Контейнеры резолвят имена через 127.0.0.11.
    • Имена — это названия сервисов в compose.yaml.

  ЧТО ЭТО ДАЁТ:
    • Никаких IP-адресов в коде.
    • При перезапуске сервиса IP меняется — код не трогается.
    • Не нужно service discovery (Consul, etcd).
    • Не нужны переменные окружения с адресами.

  ПРИМЕР:
    # compose.yaml
    services:
      postgres:
        image: postgres:16-alpine

      app:
        image: my-app:1.0
        environment:
          DB_HOST: postgres    # ← DNS-имя, не IP
          DB_PORT: "5432"

  Внутри app:
    # Псевдокод.
    conn = connect("postgres", 5432)
    # Работает, потому что Docker зарезолвит "postgres"
    # в IP контейнера.

  ЧТО МОЖНО РЕЗОЛВИТЬ:
    • Имя сервиса — `postgres`, `redis`, `app`.
    • Имя сервиса в namespace проекта — тоже работает, но
      обычно не нужно.
    • Aliases — дополнительные имена (см. раздел 7).
    • Несколько реплик сервиса (при scale) — DNS вернёт
      все IP. Это используется для client-side балансировки.

  ПРОВЕРКА DNS ИЗНУТРИ КОНТЕЙНЕРА:
    docker compose exec app sh
    # Внутри:
    nslookup postgres
    # Server:    127.0.0.11
    # Address:   127.0.0.11#53
    # Name:      postgres
    # Address:   172.20.0.5    ← IP контейнера
    exit

  Или:
    docker compose exec app ping -c 1 postgres
    # PING postgres (172.20.0.5): 56 data bytes

  ПОЧЕМУ НЕ LOCALHOST:
    Внутри контейнера localhost — это сам контейнер. Не хост.
    Не другой контейнер. Только свой loopback.

    Если app стучится на localhost:5432 — он ищет Postgres
    внутри себя. Не найдёт.

    Правильно: `postgres:5432` — по имени сервиса в сети.

  4. ЯВНЫЕ СЕТИ В COMPOSE.YAML
  Если дефолтной сети не хватает — объявляй явно.

    services:
      postgres:
        image: postgres:16-alpine
        networks:
          - backend

      app:
        image: my-app:1.0
        networks:
          - backend

    networks:
      backend:
        driver: bridge

  Три верхних ключа:
    networks:
      backend:                    # имя сети
        driver: bridge            # драйвер
        driver_opts:              # опции драйвера
          com.docker.network.bridge.name: br-backend
        ipam:                     # IP-адресация
          config:
            - subnet: 172.28.0.0/16
        internal: false           # без выхода в интернет
        external: false           # не создавать, использовать
        attachable: false         # можно ли подключать контейнеры
                                  # вне Compose
        name: my-custom-name      # явное имя сети

  ЧТО МОЖНО НАСТРОИТЬ:
    DRIVER:
      • bridge (дефолт) — для одного хоста.
      • overlay — для Swarm (multi-host).
      • host — использовать сеть хоста.
      • none — без сети.

    IPAM — IP Address Management:
      • subnet — CIDR подсети.
      • gateway — IP шлюза.
      • ip_range — диапазон для контейнеров.

    INTERNAL:
      • true — сеть без выхода в интернет.
      • Контейнеры видят друг друга, но не могут ходить
        наружу. Полезно для БД.

    EXTERNAL:
      • true — использовать уже существующую сеть.
      • Compose не создаёт и не удаляет её.

  ПРИМЕР С ЯВНОЙ ПОДСЕТЬЮ:
    networks:
      backend:
        driver: bridge
        ipam:
          config:
            - subnet: 172.28.0.0/16
              gateway: 172.28.0.1

  Если у тебя 5 приложений на одном хосте, у каждого своя подсеть,
  чтобы IP не пересекались.

  5. ИЗОЛЯЦИЯ ЧЕРЕЗ НЕСКОЛЬКО СЕТЕЙ
  Одна из главных причин использовать явные сети — изоляция.
  Не все сервисы должны видеть друг друга.

  ПРИМЕР: два фронта, один backend, одна БД.
    services:
      nginx:
        image: nginx:1.27-alpine
        networks:
          - frontend

      app:
        image: my-app:1.0
        networks:
          - frontend
          - backend

      postgres:
        image: postgres:16-alpine
        networks:
          - backend

    networks:
      frontend:
      backend:

  ЧТО ВИДНО:
    nginx → видит app, не видит postgres.
    app   → видит nginx и postgres (он в обеих сетях).
    postgres → видит app, не видит nginx.

  ЗАЧЕМ ЭТО:
    • Безопасность. Если nginx скомпрометируют, он не сможет
      стучаться в postgres напрямую.
    • Изоляция отказов. Проблемы в frontend не влияют на
      backend.
    • Соответствие политикам. В некоторых compliance-требованиях
      нельзя, чтобы web-сервер имел доступ к БД.

  СХЕМА:
    Интернет
       ↓
    [nginx]  (frontend)
       ↓
    [app]    (frontend + backend)   ← мост между сетями
       ↓
    [postgres]  (backend)

  6. СЕРВИС В НЕСКОЛЬКИХ СЕТЯХ (МОСТ)
  Сервис может быть в нескольких сетях одновременно. Это
  делается так:
    services:
      app:
        networks:
          - frontend
          - backend

  Теперь app — член обеих сетей. Он получает:
    • IP в frontend (например, 172.20.0.5).
    • IP в backend (например, 172.21.0.5).
    • Разные DNS-имена не нужны — `app` резолвится в обеих сетях.

  ЗАЧЕМ ЭТО НУЖНО:
    • API-gateway, который связывает внешнюю и внутреннюю сеть.
    • Prometheus, который собирает метрики из нескольких сетей.
    • Debug-контейнер, которому нужен доступ ко всему.

  ПРИМЕР С API-GATEWAY:
    services:
      gateway:
        image: traefik:3
        networks:
          - frontend
          - backend
          - monitoring

      app:
        networks:
          - backend

      postgres:
        networks:
          - backend

      prometheus:
        networks:
          - monitoring

  Gateway видит и внешний мир, и app, и postgres, и prometheus.
  Остальные — только свой сегмент.

  7. ALIASES: ДОПОЛНИТЕЛЬНЫЕ DNS-ИМЕНА
  Иногда хочется, чтобы сервис имел не одно имя в сети, а
  несколько. Например, `postgres`, `db`, `database` — все
  ведут к одному контейнеру.

    services:
      postgres:
        image: postgres:16-alpine
        networks:
          backend:
            aliases:
              - db
              - database

    networks:
      backend:

  ТЕПЕРЬ ИЗ ДРУГИХ КОНТЕЙНЕРОВ РАБОТАЕТ:
    docker compose exec app ping postgres
    docker compose exec app ping db
    docker compose exec app ping database

  Все три имени резолвятся в один IP.

  ЗАЧЕМ ЭТО НУЖНО:
    • Миграция. Переименовал сервис — старые конфиги с alias
      продолжают работать.
    • Семантика. У тебя несколько БД, хочешь называть их
      `users-db`, `orders-db`, но не привязываться к именам
      сервисов.
    • Абстракция. Код использует `db`, а в compose можно
      поменять реальный сервис.

  ALIASES НА УРОВНЕ СЕТИ:
    services:
      postgres:
        networks:
          backend:
            aliases:
              - db
          monitoring:
            aliases:
              - pg-monitor

  Один и тот же сервис может иметь разные алиасы в разных сетях.

  ALIASES НА УРОВНЕ СЕТИ — ВАЖНО ПОНИМАТЬ:
  Alias действует **только в той сети**, где указан. Если
  сервис в двух сетях, alias в одной сети не виден из другой.

  8. EXTERNAL NETWORKS
  Иногда сеть уже создана — снаружи. Compose должен её
  использовать, а не создавать заново.

    networks:
      shared:
        external: true
        name: my-shared-network

  ЧТО ПРОИСХОДИТ:
    • Compose не создаёт сеть при `up`.
    • Compose не удаляет её при `down`.
    • Сервисы подключаются к уже существующей сети.

  ПРИМЕР: ДВА ПРОЕКТА ОБЩАЮТСЯ ЧЕРЕЗ ОБЩУЮ СЕТЬ.
    Проект A:
      networks:
        shared:
          external: true
          name: cross-project
        internal:

      services:
        app-a:
          networks:
            - shared
            - internal

    Проект B:
      networks:
        shared:
          external: true
          name: cross-project

      services:
        app-b:
          networks:
            - shared

    app-a и app-b видят друг друга через cross-project.
    Внутри своих проектов — изолированы.

  СОЗДАНИЕ СЕТИ ЗАРАНЕЕ:
    docker network create cross-project
  ВАЖНО: без этой команды `up` упадёт с ошибкой
  «network not found».

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Несколько Compose-проектов, которым нужно общаться.
    • Интеграция с существующей инфраструктурой.
    • Подключение к сети, созданной вручную или другим
      инструментом.

  9. INTERNAL NETWORKS
  Сеть без выхода в интернет. Контейнеры видят друг друга,
  но не могут ходить наружу.

    networks:
      backend:
        internal: true

  ПРИМЕР:
    services:
      postgres:
        image: postgres:16-alpine
        networks:
          - backend

      app:
        image: my-app:1.0
        networks:
          - backend
          - frontend

    networks:
      backend:
        internal: true
      frontend:

  ЧТО ПРОИСХОДИТ:
    • postgres — только в backend. Не может ходить в интернет.
    • app — в обеих сетях. Может.

  ПРОВЕРКА:
    docker compose exec postgres ping google.com
    # ping: bad address 'google.com'
    # (или connection timeout)

    docker compose exec postgres ping app
    # PING app (172.x.x.x): 56 data bytes
    # Работает, потому что внутренняя сеть.

  ЗАЧЕМ ЭТО:
    • БД не должна ходить в интернет. Ей нужно только
      принимать подключения от app.
    • Защита от data exfiltration. Если БД скомпрометируют,
      злоумышленник не сможет слить данные наружу.
    • Compliance. Регулируемые индустрии требуют изоляции.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Postgres, MySQL, MongoDB — БД.
    • Redis, Memcached — кэши.
    • Kafka, RabbitMQ — брокеры, если они только для
      внутренней коммуникации.

  10. УПРАВЛЕНИЕ СЕТЯМИ: LS, INSPECT, RM
  Сети можно смотреть и управлять ими через CLI.

  СПИСОК СЕТЕЙ:
    docker network ls
    # NETWORK ID     NAME                       DRIVER
    # abc123         bridge                     bridge
    # def456         host                       host
    # ghi789         none                       null
    # jkl012         my-project_default         bridge
    # mno345         my-project_backend         bridge

  Фильтр по имени проекта:
    docker network ls | Select-String my-project

  ИНФОРМАЦИЯ О СЕТИ:
    docker network inspect my-project_backend

  Показывает:
    • ID, Name, Driver.
    • Subnet, Gateway.
    • Containers в сети — все контейнеры и их IP.
    • Labels.

  Сокращённый вывод:
    docker network inspect my-project_backend `
      -f '{{range .Containers}}{{.Name}} {{.IPv4Address}}{{"\n"}}{{end}}'
    # my-project-pg-1   172.20.0.5/16
    # my-project-app-1  172.20.0.6/16

  УДАЛЕНИЕ:
    docker network rm my-project_backend
    # Ошибка, если есть подключённые контейнеры.

    docker network rm -f my-project_backend
    # Force.

  УДАЛЕНИЕ ЧЕРЕЗ COMPOSE:
    docker compose down
    # Удаляет все сети проекта, кроме external.

    docker compose down --remove-orphans
    # + убирает контейнеры, которые были в сети, но не в compose.

  ПОДКЛЮЧЕНИЕ КОНТЕЙНЕРА К СЕТИ ВРУЧНУЮ:
    docker network connect my-project_backend my-other-container
    docker network disconnect my-project_backend my-other-container
  Полезно для отладки — подключить debug-контейнер к сети.

  11. ИМЕНОВАНИЕ И ПРЕФИКСЫ
  Compose автоматически префиксит имена сетей именем проекта.

    networks:
      backend:

  Создастся сеть `<project>_backend`. Например, если проект
  называется `my-shop` — `my-shop_backend`.

  ЗАЧЕМ ЭТО:
    • Два проекта на одной машине не пересекаются.
    • Можно параллельно запускать dev и staging на одном хосте.
    • Сети удаляются вместе с проектом.

  ПЕРЕОПРЕДЕЛЕНИЕ ИМЕНИ:
    networks:
      backend:
        name: my-explicit-name

  Теперь сеть будет называться `my-explicit-name`, а не
  `<project>_backend`.

  ЗАЧЕМ ЭТО НУЖНО:
    • Связка с external network, которую создаёт другая система.
    • Стабильное имя для внешних инструментов.
    • Миграция с другого compose — сохранить имя сети.

  ПОЛНЫЙ ПРИМЕР:
    services:
      app:
        networks:
          - internal

    networks:
      internal:
        name: my-app-internal
        driver: bridge

  Сеть называется `my-app-internal`, без префикса проекта.

  12. ДИАГНОСТИКА ПРОБЛЕМ
  Частые проблемы с сетями и как их решать.

  12.1. СЕРВИС НЕ ВИДИТ ДРУГОЙ СЕРВИС
    Симптом: `ping postgres` → `bad address postgres`.
    Причины:
      • Сервисы в разных сетях.
      • Postgres не подключён к сети app.
      • Опечатка в имени.
    Решение:
      docker network inspect my-project_backend `
        -f '{{range .Containers}}{{.Name}}{{"\n"}}{{end}}'

    Смотри, кто в сети. Если postgres не там — добавь
    `networks: [backend]`.

  12.2. СЕРВИС НЕ ХОДИТ В ИНТЕРНЕТ
    Симптом: `curl https://example.com` → timeout.
    Причина: сеть помечена `internal: true`.
    Решение: убери internal или добавь сервис в другую сеть.

  12.3. IP КОНФЛИКТЫ
    Симптом: контейнер не запускается, «address already in use».
    Причина: две сети с одинаковой подсетью.
    Решение: задай явные подсети:
      networks:
        backend:
          ipam:
            config:
              - subnet: 172.28.0.0/16

  12.4. ALIAS НЕ РАБОТАЕТ
    Симптом: alias не резолвится.
    Причина: alias указан в одной сети, а обращение — из
    другой.
    Решение: alias действует только в своей сети. Добавь
    сервис в ту же сеть или используй основное имя.

  12.5. СЕТЬ НЕ УДАЛЯЕТСЯ
    Симптом: `docker compose down` оставляет сеть.
    Причина: сеть external или к ней подключены сторонние
    контейнеры.
    Решение:
      docker network inspect <net> -f '{{range .Containers}}{{.Name}}{{"\n"}}{{end}}'
      # Смотри, кто висит.
      docker network disconnect <net> <container>
      docker network rm <net>

  13. АНТИПАТТЕРНЫ
  13.1. ВСЁ В ОДНОЙ СЕТИ.
    Postgres, nginx, app, adminer, prometheus — все в
    default. Нет изоляции.
  13.2. IP-АДРЕСА ВМЕСТО DNS.
    `DB_HOST=172.20.0.5` вместо `DB_HOST=postgres`. IP
    меняется при пересоздании — код ломается.
  13.3. ОДНА СЕТЬ НА НЕСКОЛЬКО ПРОЕКТОВ.
    Все в default bridge. Никакой изоляции между проектами.
  13.4. INTERNAL БЕЗ НУЖДЫ.
    Все сети internal. Сервис не может скачать обновления
    или обратиться к Stripe API.
  13.5. EXTERNAL БЕЗ РУЧНОГО СОЗДАНИЯ.
    `external: true`, но сеть не создана заранее. Compose
    упадёт с «network not found».
  13.6. ХАРДКОД SUBNET БЕЗ ПРОВЕРКИ.
    Задал 172.17.0.0/16, а там уже другая сеть. Конфликт.
  13.7. НЕ ЧИТАТЬ ЛОГИ СЕТИ.
    `docker network inspect` показывает, кто где. Не
    пользуются, гадают.
  13.8. ALIAS КАК ОСНОВНОЕ ИМЯ.
    Alias меняется легко, основное имя — стабильно.
    Не полагайся на alias для прода.
  13.9. ПОДКЛЮЧЕНИЕ DEBUG-КОНТЕЙНЕРА К PROD-СЕТИ.
    Оставил debug-контейнер подключённым. Потом забыл.
    Лишний attack surface.
  13.10. ИГНОРИРОВАНИЕ DRIVER.
    Не понимают разницу bridge/overlay/host. Пытаются
    использовать bridge для multi-host.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Compose создаёт default-сеть автоматически, если
      networks не указаны. Все сервисы в одной сети видят
      друг друга.
  2.  DNS работает из коробки: `postgres:5432` резолвится
      через встроенный DNS Docker (127.0.0.11).
  3.  Никогда не используй localhost внутри контейнера
      для обращения к другому сервису. Только имя сервиса.
  4.  Явные сети (`networks:` в сервисе) нужны для изоляции.
      Не все сервисы должны видеть всех.
  5.  Сервис может быть в нескольких сетях. Это делает его
      мостом между сегментами (API-gateway, Prometheus).
  6.  Alias — дополнительные DNS-имена. Действует только
      в той сети, где указан.
  7.  External networks — для общения нескольких Compose-
      проектов. Создаются вручную, не удаляются через `down`.
  8.  Internal networks — без выхода в интернет. Для БД,
      кэшей, брокеров. Защита от data exfiltration.
  9.  Управление: `docker network ls/inspect/rm`. Inspect
      показывает, кто в сети и с какими IP.
  10. Имена сетей префиксятся именем проекта. Переопределяется
      через `name:`.
  11. Частые проблемы: сервисы в разных сетях, alias не в
      той сети, конфликт подсетей, external без создания.
  12. Антипаттерны: одна сеть на всё, IP вместо DNS,
      internal без нужды, хардкод подсети, alias как
      основное имя.
*/
