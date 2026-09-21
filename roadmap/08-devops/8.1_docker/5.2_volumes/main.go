package main

/*
  УРОК 9.5.2: VOLUMES, ПРАВА И HOT RELOAD
  Контейнер эфемерный. Удалил — всё, что внутри, потеряно.
  Это правильно для stateless-сервисов, но данные должны жить
  где-то ещё. Тома (volumes) — механизм, который выносит данные
  за пределы контейнера.
  Второй слой темы — разработка. Пока пишешь код, хочется видеть
  изменения без пересборки образа. Bind mount + hot reload
  решают это. Но тут же всплывают проблемы с правами: под non-root
  файлы из bind mount часто недоступны.
  Эта тема — про три типа монтирования, разницу между named volume
  и bind mount, права UID/GID и современный hot reload через
  docker compose watch.

  СОДЕРЖАНИЕ:
    1.  Проблема: где живут данные в контейнере
    2.  Три типа монтирования
    3.  Named volume — что это
    4.  Bind mount — что это
    5.  Tmpfs
    6.  Named vs bind: когда что
    7.  Права доступа: UID/GID
    8.  Проблема bind mount под non-root
    9.  Решение 1: chown в Dockerfile
    10. Решение 2: user: "${UID}:${GID}" в compose
    11. Решение 3: init-контейнер для named volume
    12. Hot reload — зачем
    13. docker compose watch — современный стандарт
    14. Три режима watch
    15. Air как альтернатива
    16. Антипаттерны
    17. Финальные выводы

  1. ПРОБЛЕМА: ГДЕ ЖИВУТ ДАННЫЕ В КОНТЕЙНЕРЕ
  Контейнер = образ (read-only) + writable-слой. Всё, что
  записывается во время работы, идёт в writable-слой.

  Что происходит при удалении контейнера:
    • Writable-слой удаляется.
    • Все данные в нём — потеряны.
    • Образ остаётся.

  Классический сценарий:
    docker run -d --name pg postgres:16
    # Postgres пишет данные в /var/lib/postgresql/data.
    # Файлы в writable-слое контейнера.

    docker rm -f pg
    # Контейнер удалён. База данных потеряна.

  То же самое с Redis, Mongo, любой БД и любым сервисом,
  который пишет данные на диск.

  РЕШЕНИЕ: вынести данные за пределы writable-слоя. Это
  делается через монтирование (mount).

  Три типа монтирования:
    • Named volume — Docker управляет хранилищем.
    • Bind mount — папка хоста в контейнере.
    • Tmpfs — в памяти, эфемерный.

  2. ТРИ ТИПА МОНТИРОВАНИЯ
  В compose.yaml монтирование описывается в сервисе:

    services:
      postgres:
        volumes:
          # named volume
          - pgdata:/var/lib/postgresql/data

          # bind mount
          - ./init:/docker-entrypoint-initdb.d:ro

          # tmpfs
          - type: tmpfs
            target: /tmp

  Верхний уровень `volumes:` нужен только для named:

    volumes:
      pgdata:      # объявление named volume

  А bind mount — это просто путь на хосте, его не надо
  объявлять в верхнем ключе.

  3. NAMED VOLUME — ЧТО ЭТО
  Named volume — хранилище, которым управляет Docker.

    services:
      postgres:
        volumes:
          - pgdata:/var/lib/postgresql/data

    volumes:
      pgdata:

  Что происходит:
    • Docker создаёт volume с именем `<project>_pgdata`
      (или `pgdata`, если задано name).
    • Volume лежит в /var/lib/docker/volumes/ на Linux.
    • Монтируется в контейнер в /var/lib/postgresql/data.
    • Переживает удаление контейнера.

  ПОЛНЫЙ СИНТАКСИС:
    volumes:
      pgdata:
        name: my-postgres-data
        driver: local
        driver_opts:
          type: none
          o: bind
          device: /var/lib/postgres-data
        labels:
          com.example.description: "Postgres data"

  НАЗНАЧЕНИЕ:
    • Данные БД, кэшей, брокеров.
    • Всё, что должно пережить пересоздание контейнера.
    • Всё, что не нужно видеть в файловой системе хоста.

  ПРОВЕРКА:
    docker volume ls | Select-String pgdata
    # local   my-project_pgdata

    docker volume inspect my-project_pgdata -f '{{.Mountpoint}}'
    # /var/lib/docker/volumes/my-project_pgdata/_data

  Жизненный цикл:
    • Создаётся при первом `docker compose up`.
    • Живёт независимо от контейнеров.
    • Удаляется через `docker compose down -v` или
      `docker volume rm`.

  4. BIND MOUNT — ЧТО ЭТО
  Bind mount — монтирование папки или файла с хоста.

    services:
      app:
        volumes:
          - ./src:/app/src
          - ./config.yaml:/etc/app/config.yaml:ro

  Что происходит:
    • Папка ./src на хосте монтируется в /app/src контейнера.
    • Двусторонняя связь: изменения на хосте видны в контейнере.
    • Изменения в контейнере видны на хосте.

  Режимы:
    • `:ro` — read-only. Контейнер не может писать.
    • `:rw` — read-write (по умолчанию).
    • `:z` — SELinux relabel.
    • `:cached`, `:delegated` — macOS, ускорение I/O.

  НАЗНАЧЕНИЕ:
    • Разработка — исходный код на хосте, контейнер видит
      изменения.
    • Конфиги — файл с хоста монтируется в контейнер.
    • Логи — из контейнера на хост.
    • Init-скрипты Postgres.

  ПРИМЕР С КОНФИГОМ:
    services:
      nginx:
        volumes:
          - ./nginx/default.conf:/etc/nginx/conf.d/default.conf:ro

  Nginx читает конфиг с хоста. Меняешь — перезапускаешь nginx,
  изменения применяются.

  ВАЖНО:
    Bind mount перезаписывает содержимое директории в контейнере.
    Если в образе в /app/src лежали файлы, а ты монтируешь
    папку с хоста — они «исчезнут» (перекроются).

    Иногда это полезно (dev), иногда опасно (prod).

  5. TMPFS
  Tmpfs — монтирование в оперативной памяти.

    services:
      app:
        tmpfs:
          - /tmp
          - /run

  Или через volumes:
    services:
      app:
        volumes:
          - type: tmpfs
            target: /tmp
            tmpfs:
              size: 1000000    # 1 МБ в байтах

  Что происходит:
    • Данные живут в RAM.
    • Не сохраняются между перезапусками.
    • Не попадают на диск.

  НАЗНАЧЕНИЕ:
    • Временные файлы, кэш.
    • Секреты, чтобы не попадали на диск.
    • Место для записи при `--read-only` корневой ФС.

  6. NAMED VS BIND: КОГДА ЧТО
  ┌────────────────────┬──────────────────┬──────────────────┐
  │ Характеристика     │ Named volume     │ Bind mount       │
  ├────────────────────┼──────────────────┼──────────────────┤
  │ Управление         │ Docker           │ Хост             │
  │ Видно в ls         │ Нет              │ Да               │
  │ Портативно         │ Да               │ Нет              │
  │ Права              │ Настраиваются    │ Зависит от хоста │
  │ Резервное копиро-  │ docker volume    │ Обычные файлы    │
  │ вание              │ + tar            │                  │
  │ Использование      │ Данные, прод     │ Разработка,      │
  │                    │                  │ конфиги          │
  └────────────────────┴──────────────────┴──────────────────┘

  ПРАВИЛО:
    • Данные БД, кэшей, брокеров → named volume.
    • Исходный код, конфиги, логи → bind mount.
    • Временные файлы → tmpfs.

  В ПРОДЕ:
    • Named volumes — для всего, что должно переживать
      пересоздание.
    • Bind mounts — только для конфигов (read-only).

  В DEV:
    • Named volumes — для БД (данные сохраняются между
      перезапусками).
    • Bind mounts — для исходного кода (hot reload).

  7. ПРАВА ДОСТУПА: UID/GID
  В Linux каждый файл принадлежит UID и GID. Ядро смотрит
  только на числа, не на имена.

  Когда контейнер пишет файл:
    • Файл получает UID процесса внутри контейнера.
    • На хосте (для bind mount) этот UID виден.
    • Если UID не совпадает с твоим — не сможешь читать/писать.

  ПРИМЕР:
    Ты на хосте — UID 1000 (обычный пользователь).
    Контейнер работает под root (UID 0).
    Bind mount ./data.

    Контейнер пишет /data/file.txt.
    На хосте file.txt принадлежит root:root.

    Ты (1000) не можешь удалить его без sudo.

  Это классическая боль. Лечится через non-root в контейнере
  с UID, совпадающим с хостом.

  В NAMED VOLUME:
    Volume создаётся Docker'ом от root:root.
    Если контейнер работает под non-root, писать не сможет.

    Решение: init-контейнер, который делает chown (раздел 11).

  8. ПРОБЛЕМА BIND MOUNT ПОД NON-ROOT
  Хост: папка ./src принадлежит UID 1000.
  Контейнер: работает под UID 10001 (non-root).

  Что происходит:
    • Bind mount монтирует ./src в /app/src.
    • В контейнере /app/src принадлежит UID 1000.
    • Процесс под UID 10001 не может писать в /app/src.
    • Permission denied.

  ЕЩЁ ХУЖЕ:
    • chown в Dockerfile не помогает — bind mount перезаписывает.
    • Файлы в контейнере всегда с UID хоста.

  СИМПТОМ:
    docker compose up
    # В логах: permission denied: /app/src/main.go

  ИЛИ:
    docker compose exec app touch /app/src/test.txt
    # touch: cannot touch '/app/src/test.txt': Permission denied

  ТРИ РЕШЕНИЯ:
    • chown в Dockerfile — только для файлов внутри образа,
      не для bind mount.
    • user: "${UID}:${GID}" в compose — контейнер работает
      под твоим UID.
    • chown на хосте — sudo chown -R 1000:1000 ./src.

  9. РЕШЕНИЕ 1: CHOWN В DOCKERFILE
  Работает для директорий внутри образа. НЕ работает для
  bind mount.

    FROM alpine:3.19

    RUN adduser -D -u 1000 appuser
    RUN mkdir -p /app/data && chown -R appuser:appuser /app

    COPY --chown=appuser:appuser ./app /app

    USER appuser

  Директория /app и файлы в ней принадлежат appuser.
  Процесс под appuser может писать.

  ДЛЯ BIND MOUNT:
    chown в Dockerfile не поможет. Bind mount перезаписывает
    содержимое /app/src из хоста с UID хоста.

  Используй это для директорий, которые НЕ монтируются
  (например, /app/config, /app/bin).

  10. РЕШЕНИЕ 2: USER: "${UID}:${GID}" В COMPOSE
  Самое удобное для разработки. Контейнер работает под
  UID/GID хоста.

    services:
      app:
        image: my-app:1.0
        user: "${UID:-1000}:${GID:-1000}"
        volumes:
          - ./src:/app/src

  ПЕРЕД ЗАПУСКОМ:
    # Linux/macOS/WSL2.
    export UID=$(id -u)
    export GID=$(id -g)

    docker compose up

  Или через .env:
    UID=1000
    GID=1000

  ЧТО ПРОИСХОДИТ:
    • Compose подставляет твои UID/GID.
    • Контейнер запускается под этими UID/GID.
    • Файлы в bind mount создаются с твоим владельцем.
    • Никаких Permission denied.

  В POWERSHELL:
    $env:UID="1000"
    $env:GID="1000"
    docker compose up

  МИНУСЫ:
    • Только для dev. В проде UID фиксирован в образе.
    • Не работает, если образ требует конкретный UID
      для внутренних файлов.

  ПРИМЕР:
    services:
      app:
        image: my-app:1.0
        user: "${UID:-1000}:${GID:-1000}"
        volumes:
          - ./src:/app/src
          - ./config:/app/config:ro

  11. РЕШЕНИЕ 3: INIT-КОНТЕЙНЕР ДЛЯ NAMED VOLUME
  Named volume создаётся Docker'ом от root:root. Контейнер
  под UID 1000 не может писать.

  РЕШЕНИЕ: отдельный init-контейнер, который делает chown.

    services:
      volume-init:
        image: alpine:3.19
        user: root
        volumes:
          - pgdata:/data
        command: sh -c "chown -R 1000:1000 /data && echo 'init done'"
        restart: "no"

      app:
        image: my-app:1.0
        user: "1000:1000"
        volumes:
          - pgdata:/app/data
        depends_on:
          volume-init:
            condition: service_completed_successfully

    volumes:
      pgdata:

  ЧТО ПРОИСХОДИТ:
    1. Docker создаёт volume pgdata (root:root).
    2. volume-init запускается под root, делает chown, выходит.
    3. app запускается под 1000, видит volume с правильным
       владельцем.

  ИМЕННО ТАК ДЕЛАЮТ POSTGRES, MONGO, ELASTICSEARCH:
    Их entrypoint-скрипты при первом старте делают chown
    на /var/lib/...

  12. HOT RELOAD — ЗАЧЕМ
  При разработке Go-сервиса хочется:
    • Писать код в IDE на хосте.
    • Чтобы контейнер видел изменения сразу.
    • Не пересобирать образ при каждой правке.

  Классический цикл без hot reload:
    1. Изменил .go файл.
    2. docker compose build.
    3. docker compose up -d.
    4. Ждёшь 30 секунд.
    5. Тестируешь.

  С hot reload:
    1. Изменил .go файл.
    2. Ждёшь 1-2 секунды.
    3. Тестируешь.

  Экономия — минуты на каждой итерации. За день — часы.

  ТРИ СПОСОБА:
    • docker compose watch — современный стандарт.
    • Air (cosmtrek/air) — внешний инструмент.
    • Свой скрипт с inotify.

  13. DOCKER COMPOSE WATCH — СОВРЕМЕННЫЙ СТАНДАРТ
  Встроено в Compose v2.22+. Ничего дополнительно не ставить.

  ПРОВЕРКА ВЕРСИИ:
    docker compose version
    # Docker Compose version v2.24.0    ← подходит

  ЕСЛИ ВЕРСИЯ СТАРАЯ:
    Обнови Docker Desktop.

  КОНФИГ В COMPOSE.YAML:

    services:
      app:
        build: .
        develop:
          watch:
            - action: sync
              path: ./src
              target: /app/src
              ignore:

- action: rebuild
path: go.mod
- action: sync+restart
path: ./config
target: /app/config

ЗАПУСК:
docker compose watch

Compose запускает стек, следит за файлами и реагирует на
изменения.

ЧТО ПРОИСХОДИТ ПРИ ИЗМЕНЕНИИ:
Изменил ./src/main.go:
→ action: sync. Файл копируется в /app/src/main.go
внутри контейнера.
→ Go-сервис должен сам перечитать (например, через
go run или air внутри контейнера).

Изменил go.mod:
→ action: rebuild. Образ пересобирается и контейнер
перезапускается.

Изменил ./config:
→ action: sync+restart. Файл копируется, контейнер
перезапускается.

ЧЕМ ОТЛИЧАЕТСЯ ОТ BIND MOUNT:
Bind mount — постоянное монтирование. Изменения на хосте
видны сразу, но контейнер не перезапускается.

Watch — реагирует на изменения. Копирует файлы, при
необходимости перезапускает контейнер.
Оба подхода можно комбинировать.

14. ТРИ РЕЖИМА WATCH

ACTION: SYNC
Копирует файлы из path в target внутри контейнера.
Контейнер не перезапускается.

Используй для: исходного кода, статики, шаблонов.

Пример:
- action: sync
path: ./src
target: /app/src

Внутри контейнера должен быть механизм, который
перечитывает файлы (например, go run, air, nodemon).

ACTION: REBUILD
Пересобирает образ и пересоздаёт контейнер.

Используй для: Dockerfile, go.mod, go.sum, любых файлов,
меняющих образ.

Пример:
- action: rebuild
path: go.mod

Медленно (полная сборка), но нужно для зависимостей.

ACTION: SYNC+RESTART

Копирует файлы и перезапускает контейнер. Образ не
пересобирается.

Используй для: конфигов, которые читаются только при
старте.

Пример:
- action: sync+restart
path: ./config
target: /app/config

ПОЛНЫЙ ПРИМЕР С ТРЕМЯ РЕЖИМАМИ:

services:
app:
build: .
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

- action: sync+restart
path: ./config
target: /app/config

15. AIR КАК АЛЬТЕРНАТИВА
Air — Go-инструмент для hot reload. Следит за .go файлами
и перезапускает бинарник.

INSTALL:
go install github.com/cosmtrek/air@latest

ЧТО ДЕЛАЕТ:
• Следит за .go файлами в проекте.
• При изменении — пересобирает и перезапускает.
• Работает либо на хосте, либо внутри контейнера.

ДЛЯ DOCKER — ВНУТРИ КОНТЕЙНЕРА:
Dockerfile.dev:

FROM golang:1.22-alpine

RUN go install github.com/cosmtrek/air@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["air", "-c", ".air.toml"]

.air.toml:

root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/main ."
bin = "tmp/main"
include_ext = ["go"]
exclude_dir = ["tmp", "vendor"]
delay = 1000

В COMPOSE:
services:
app:
build:
context: .
dockerfile: Dockerfile.dev
volumes:
- ./:/app
ports:
- "8080:8080"

ЧТО ПРОИСХОДИТ:
• Bind mount ./ в /app.
• Air внутри контейнера следит за /app/**
• Изменил файл → Air пересобирает и перезапускает.

СРАВНЕНИЕ С COMPOSE WATCH:
Compose watch:
• Встроено в Compose.
• Копирует файлы в контейнер.
• Не требует установки.

Air:
• Отдельный инструмент.
• Работает через bind mount.
• Пересобирает бинарник при каждом изменении.
Оба варианта рабочие. Compose watch — современнее, Air —
гибче.

16. АНТИПАТТЕРНЫ
16.1. ХРАНИТЬ ДАННЫЕ БД В WRITABLE-СЛОЕ.
Контейнер удалили — база пропала. Только named volumes.
16.2. ПИСАТЬ ЛОГИ В КОНТЕЙНЕР.
Bind mount для логов или logging driver. Иначе логи
копятся в writable-слое.
16.3. BIND MOUNT ПОД NON-ROOT БЕЗ CHOWN.
Permission denied. Решай через user: "${UID}:${GID}".
16.4. NAMED VOLUME ПОД NON-ROOT БЕЗ INIT.
Volume root:root, контейнер не пишет. Init-контейнер.
16.5. ОДИН NAMED VOLUME НА НЕСКОЛЬКО СЕРВИСОВ.
Postgres и Redis пишут в один volume — конфликт.
Отдельные volumes.
16.6. BIND MOUNT С ПАПКОЙ /APP В ПРОДЕ.
Перекрывает всё, что было в образе. В проде только
named volume.
16.7. NO HOT RELOAD В DEV.
Пересобирают образ на каждое изменение. Медленно.
Compose watch или Air.
16.8. AIR БЕЗ EXCLUDE.
Air следит за tmp/, vendor/ — бесконечная петля.
exclude_dir в .air.toml.
16.9. BIND MOUNT READ-WRITE ДЛЯ КОНФИГОВ.
Конфиг случайно перезаписывается контейнером.
Всегда :ro для конфигов.
16.10. WATCH БЕЗ IGNORE.
Файлы IDE (.idea, .vscode) вызывают постоянные
перезапуски. ignore в watch.
16.11. NAMED VOLUME С ОТНОСИТЕЛЬНЫМ ПУТЁМ.
Volumes в сервисе — только имена. Относительные пути —
для bind mount.
16.12. TMPFS ДЛЯ БОЛЬШИХ ДАННЫХ.
RAM ограничен. tmpfs для больших файлов — OOM.
Только для мелких временных.

17. ФИНАЛЬНЫЕ ВЫВОДЫ
1.  Контейнер эфемерный. Данные — в volumes, иначе
потеряются.
2.  Три типа: named volume (Docker), bind mount (хост),
tmpfs (RAM).
3.  Named volume — для данных БД, кэшей, брокеров.
Переживает пересоздание контейнера.
4.  Bind mount — для разработки, конфигов, логов.
Двусторонняя связь с хостом.
5.  Tmpfs — для временных файлов и секретов. Не сохраняется.
6.  Права UID/GID — частая боль. Ядро работает с числами,
не с именами.
7.  Bind mount под non-root → Permission denied, если
UID хоста ≠ UID контейнера.
8.  Три решения: chown в Dockerfile (для файлов в образе),
user: "${UID}:${GID}" в compose (для dev),
init-контейнер (для named volume).
9.  Hot reload экономит часы в день. Актуально для dev.
10. docker compose watch — современный стандарт.
Встроено в Compose v2.22+. Три режима: sync, rebuild,
sync+restart.
11. Air — альтернатива. Гибче, но требует установки.
Работает через bind mount.
12. В проде — named volumes. В dev — bind mounts +
watch/Air.
13. Антипаттерны: данные в writable-слое, bind mount
под non-root без chown, один volume на несколько
сервисов, нет hot reload в dev.
*/
