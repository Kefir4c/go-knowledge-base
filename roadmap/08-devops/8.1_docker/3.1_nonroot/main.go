package main

/*
  УРОК 9.3.1: NON-ROOT ПОЛЬЗОВАТЕЛЬ И ПРАВА НА VOLUMES
  По умолчанию контейнер работает под root. Это значит, что
  внутри контейнера процесс имеет UID 0 и полные права. Если
  злоумышленник пробьёт сервис — он root. Не в контейнере вообще,
  а на UID 0 конкретно в этом namespace. Но namespace — не
  абсолютная защита. История знает десятки CVE, где из
  контейнера выходили наружу, и наличие root внутри сильно
  упрощало атаку.
  Плюс — работа под root мешает правам на volumes. Файлы,
  созданные root'ом на хосте, принадлежат root. Другие процессы
  на хосте (например, твой бэкап под обычным пользователем)
  не смогут их прочитать. Это вторая причина, по которой
  non-root — стандарт.
  Эта тема про то, как правильно переключиться на non-root
  в Dockerfile и как не сломаться об права на volumes.

  СОДЕРЖАНИЕ:
    1.  Зачем non-root: security и permissions
    2.  UID и GID: как это работает в Linux
    3.  Как переключиться на non-root: USER в Dockerfile
    4.  Alpine: adduser + USER
    5.  Distroless: :nonroot из коробки
    6.  Scratch: только UID, без passwd
    7.  COPY --chown: файлы сразу с правильным владельцем
    8.  Проблема с bind mount: Permission denied
    9.  Решение 1: chown в Dockerfile
    10. Решение 2: chown на хосте
    11. Решение 3: init-контейнер для named volume
    12. Решение 4: user: "${UID}:${GID}" в compose
    13. Практика: полный Dockerfile
    14. Антипаттерны
    15. Финальные выводы

  1. ЗАЧЕМ NON-ROOT: SECURITY И PERMISSIONS
  ДВЕ ПРИЧИНЫ ИСПОЛЬЗОВАТЬ NON-ROOT.

  ПРИЧИНА 1: БЕЗОПАСНОСТЬ.
    Контейнер — это процесс, изолированный через namespaces.
    Namespaces не идеальны. История CVE:

      • CVE-2019-5736 — runc, из контейнера можно было
        получить root на хосте.
      • CVE-2022-0185 — kernel, escape через unshare.
      • CVE-2024-21626 — runc, утечка file descriptors.

    В каждом из этих случаев наличие root внутри контейнера
    усиливало атаку. Если бы процесс работал под UID 1000 —
    атака была бы ограничена.

  ПРИЧИНА 2: ПРАВА НА VOLUMES.
    Файлы, созданные в bind mount, принадлежат UID процесса
    внутри контейнера. Если процесс — root (UID 0), файлы
    на хосте будут root:root. Твой пользователь на хосте
    (UID 1000) их не прочитает. Бэкапы не сработают. IDE
    не откроет. Это постоянная боль.

    Если процесс — UID 1000 (совпадает с твоим), файлы
    создаются с правильным владельцем. Всё работает.

  KUBERNETES ТРЕБУЕТ:
    Во многих кластерах включён PodSecurity Standard
    restricted, который ЗАПРЕЩАЕТ root в подах. Если твой
    образ не умеет работать под non-root — он не задеплоится.

  ВЫВОД: non-root — стандарт для прода. Всегда.

  2. UID И GID: КАК ЭТО РАБОТАЕТ В LINUX
  В Linux каждый процесс имеет:
    • UID (User ID) — числовой идентификатор пользователя.
    • GID (Group ID) — числовой идентификатор группы.
    • Дополнительные группы.

  ВАЖНО: ЯДРО РАБОТАЕТ ТОЛЬКО С UID И GID, НЕ С ИМЕНАМИ.

    Когда ты пишешь `chown appuser:appgroup file`, ядро
    видит только числа. Имя `appuser` — это удобство для
    человека. Ядро смотрит в /etc/passwd, находит там
    `appuser:x:1000:1000:...`, и использует 1000:1000.

    Если /etc/passwd нет — имя не существует. Только UID.

  СИСТЕМНЫЕ UID:
    • 0 — root. Всегда.
    • 1-999 — системные пользователи (daemon, sshd, postgres).
    • 1000+ — обычные пользователи.

  В КОНТЕЙНЕРАХ:
    UID 1000 внутри контейнера и UID 1000 на хосте — это
    РАЗНЫЕ пользователи. Docker не синхронизирует /etc/passwd
    между хостом и контейнером.

    НО: файлы в bind mount принадлежат по UID/GID, не по
    имени. Если на хосте /home/user принадлежит UID 1000,
    и контейнер пишет под UID 1000 — файлы создадутся с
    правильным владельцем. Потому что ядро смотрит на UID.

    Если контейнер пишет под UID 65532 (distroless nonroot) —
    файлы на хосте будут принадлежать UID 65532. Которого
    в /etc/passwd хоста может не быть. Тогда `ls -la` покажет
    `65532 65532` вместо имени.

  ГЛАВНОЕ ПРАВИЛО:
    Для bind mount UID внутри контейнера должен совпадать
    с UID владельца папки на хосте. Иначе Permission denied.

  3. КАК ПЕРЕКЛЮЧИТЬСЯ НА NON-ROOT: USER В DOCKERFILE
  Инструкция USER переключает пользователя, под которым
  выполняются последующие RUN, CMD, ENTRYPOINT.

    USER 1000:1000
    USER appuser
    USER appuser:appgroup
    USER nonroot:nonroot

  СИНТАКСИС:
    USER <UID>             — только UID. GID остаётся текущий.
    USER <UID>:<GID>       — UID и GID.
    USER <user>            — по имени. Требует /etc/passwd.
    USER <user>:<group>    — по именам.

  КОГДА ПРИМЕНЯЕТСЯ:
    USER действует на все последующие RUN, CMD, ENTRYPOINT.
    Если USER стоит до RUN — RUN выполнится под этим
    пользователем.

  ПОРЯДОК В DOCKERFILE:
    # Сначала root-задачи.
    FROM alpine:3.19
    RUN apk add --no-cache curl
    RUN adduser -D -u 1000 appuser

    # Потом переключаемся на non-root.
    USER appuser

    # Дальше всё под appuser.
    COPY --chown=1000:1000 ./app /app
    ENTRYPOINT ["/app/server"]

  ВАЖНО: если USER стоит слишком рано, apt-get/apk не сработают
  (нужен root). Все установки — до USER.

  ПРОВЕРКА В КОНТЕЙНЕРЕ:
    docker run --rm --entrypoint id myimage
    # uid=1000(appuser) gid=1000(appuser) groups=1000(appuser)

    docker inspect myimage -f '{{.Config.User}}'
    # 1000:1000 или appuser

  4. ALPINE: ADDUSER + USER
  В Alpine создаём пользователя и сразу переключаемся.

    FROM alpine:3.19

    # Создаём пользователя с UID 1000.
    RUN adduser -D -u 1000 appuser

    USER appuser

  РАЗБОР ФЛАГОВ adduser:
    -D                    — без пароля.
    -u 1000               — UID 1000.
    -G appgroup           — добавить в группу appgroup.
    -s /sbin/nologin      — запретить login.
    -h /home/appuser      — домашняя директория.
    -g "App User"         — описание.

  ПРИМЕР С ГРУППОЙ:
    RUN addgroup -g 1000 appgroup && \
        adduser -D -u 1000 -G appgroup appuser

    USER appuser:appgroup

  ПРИМЕР С ПОЛНЫМ НАБОРОМ:
    RUN addgroup -g 1000 -S appgroup && \
        adduser -u 1000 -G appgroup -S -H -s /sbin/nologin appuser

    USER 1000:1000

    Что значат флаги:
      -S — системный пользователь (UID < 1000 по умолчанию,
           но с -u 1000 явно задаём).
      -H — не создавать домашнюю директорию.
      -s /sbin/nologin — запретить shell-логин.

  ДЛЯ ЧЕГО НУЖНА ГРУППА:
    Для общего доступа к файлам. Если несколько процессов
    работают с общей папкой — добавь их в одну группу.

  ВАЖНО ПРО ФАЙЛЫ:
    После adduser создаётся /etc/passwd с записью appuser.
    Значит, USER appuser (по имени) сработает. Без adduser —
    только USER 1000:1000 (по UID).

  5. DISTROLESS: :NONROOT ИЗ КОРОБКИ
  Distroless уже содержит nonroot-пользователя с UID 65532.
  Никаких adduser не нужно.

    FROM gcr.io/distroless/static-debian12:nonroot

    COPY --from=builder /out/server /server

    # USER уже установлен в :nonroot теге.
    # Но можно задать явно для читаемости.
    USER nonroot:nonroot

  ЧТО ВНУТРИ:
    • /etc/passwd с nonroot:x:65532:65532.
    • /etc/group с nonroot:x:65532.
    • Домашняя директория /home/nonroot.

  UID 65532 — стандарт от Google. Всем известно, что
  это nonroot в distroless.

  ПОЧЕМУ 65532, А НЕ 1000:
    Google выбрал «необычный» UID, чтобы избежать конфликта
    с обычными пользователями на хосте. UID 1000 — это часто
    первый пользователь на Ubuntu/Debian. UID 65532 почти
    нигде не занят.

  ЕСЛИ НУЖЕН UID 1000:
    В distroless нельзя создать пользователя (нет adduser).
    Но можно указать числовой UID в USER:

      USER 1000:1000

    Это сработает, но в /etc/passwd нет записи для 1000.
    Некоторые программы смотрят на имя — им будет плохо.

    Лучше оставить :nonroot с 65532.

  ПРОВЕРКА:
    docker run --rm --entrypoint id distroless/static:nonroot
    # uid=65532 gid=65532 groups=65532

  6. SCRATCH: ТОЛЬКО UID, БЕЗ PASSWD
  В scratch нет ничего. Ни /etc/passwd, ни /etc/group.

    FROM scratch
    COPY --from=builder /out/server /server

    # USER только по UID. По имени — ошибка.
    USER 1000:1000

    ENTRYPOINT ["/server"]

  ЧТО ПРОИСХОДИТ ПРИ USER APPUSER В SCRATCH:

    USER appuser
    # Ошибка при запуске:
    # unable to find user appuser: no matching entries in passwd file

  Docker ищет appuser в /etc/passwd, не находит, падает.

  РЕШЕНИЯ ДЛЯ SCRATCH:
    ВАРИАНТ 1: USER только по UID.
      USER 1000:1000

    ВАРИАНТ 2: Скопировать /etc/passwd и /etc/group из builder.
      COPY --from=builder /etc/passwd /etc/passwd
      COPY --from=builder /etc/group /etc/group
      USER appuser

    ВАРИАНТ 3: Создать минимальный /etc/passwd вручную.
      RUN echo "appuser:x:1000:1000::/home/appuser:/sbin/nologin" \
          > /etc/passwd

    ВАРИАНТ 4: Использовать distroless вместо scratch.
      В distroless всё уже есть.

  ПРАВИЛО: если хочется по имени — distroless. Если только
  UID — scratch.

  7. COPY --CHOWN: ФАЙЛЫ СРАЗУ С ПРАВИЛЬНЫМ ВЛАДЕЛЬЦЕМ
  Классическая проблема: копируешь файлы под root, потом
  переключаешься на USER 1000. Файлы принадлежат root:root,
  процесс не может их читать.

    FROM alpine:3.19
    RUN adduser -D -u 1000 appuser

    COPY app /app           # ← файлы root:root

    USER appuser
    ENTRYPOINT ["/app/server"]
    # Ошибка: permission denied на /app/server

  РЕШЕНИЕ: COPY --chown.

    COPY --chown=1000:1000 app /app

  Флаги:

    --chown=1000:1000         — по UID:GID.
    --chown=appuser:appgroup  — по именам (если /etc/passwd есть).
    --chown=appuser           — только владелец, группа не меняется.

  ПРИМЕР ДЛЯ GO-СЕРВИСА:
    FROM alpine:3.19
    RUN adduser -D -u 1000 appuser

    COPY --chown=1000:1000 --from=builder /out/server /server

    USER appuser
    ENTRYPOINT ["/server"]

  ДЛЯ ДИРЕКТОРИЙ:
    COPY --chown=1000:1000 ./config /app/config

  Все файлы в директории получат владельца 1000:1000.

  ВАЖНО: --chown работает ТОЛЬКО с COPY и ADD. Не работает
  с RUN.

  ЕСЛИ НУЖНО ПОМЕНЯТЬ ВЛАДЕЛЬЦА ПОСЛЕ COPY:
    RUN chown -R appuser:appuser /app

    Но это создаёт лишний слой в образе. Лучше сразу
    COPY --chown.

  8. ПРОБЛЕМА С BIND MOUNT: PERMISSION DENIED
  Bind mount монтирует папку с хоста в контейнер. Права
  на файлы определяются UID/GID на хосте.

  КЛАССИЧЕСКИЙ СЦЕНАРИЙ:
    Хост:
      /home/user/data — принадлежит user:user (1000:1000).

    Контейнер:
      USER 1000:1000
      VOLUME /data
      docker run -v /home/user/data:/data myimage

    Контейнер видит /data с владельцем 1000:1000 (потому
    что UID совпал). Всё работает.

  ЛОМАЕТСЯ, ЕСЛИ:
    Хост:
      /home/user/data — принадлежит user:user (1000:1000).

    Контейнер:
      USER 65532:65532 (distroless nonroot)
      docker run -v /home/user/data:/data myimage

    Контейнер видит /data с владельцем 1000:1000. Но процесс
    под UID 65532. Попытка записи → Permission denied.

  ЕЩЁ ХУЖЕ, ЕСЛИ:
    Хост:
      /home/user/data — принадлежит user:user (1000:1000).

    Контейнер:
      USER 0:0 (root)
      docker run -v /home/user/data:/data myimage

    Процесс пишет файлы как root. Файлы на хосте становятся
    root:root. Твой пользователь не может их удалить без sudo.
    Именно так появляются файлы, которые «не удаляются без
    sudo» после работы контейнера.

  ГЛАВНОЕ ПРАВИЛО:
    UID внутри контейнера должен совпадать с UID владельца
    папки на хосте.

  КАК ЭТО РЕШИТЬ — ЧЕТЫРЕ СПОСОБА (СЛЕДУЮЩИЕ РАЗДЕЛЫ).

  9. РЕШЕНИЕ 1: CHOWN В DOCKERFILE
  Самый простой способ. В Dockerfile до USER — сделать chown
  на нужные директории.

    FROM alpine:3.19

    RUN adduser -D -u 1000 appuser

    # Создаём директории и сразу chown.
    RUN mkdir -p /app/data /app/logs && \
        chown -R appuser:appuser /app

    COPY --chown=appuser:appuser ./app /app

    USER appuser

  ПЛЮСЫ:
    • Работает без изменений на хосте.
    • Всё в одном месте.
    • Хорошо для директорий ВНУТРИ образа.

  МИНУСЫ:
    • НЕ РАБОТАЕТ для bind mount.
      Bind mount заменяет содержимое директории образом хоста.
      chown в Dockerfile становится невидимым.

  ВЫВОД:
    Для директорий внутри образа — chown в Dockerfile.
    Для bind mount — не поможет, нужно что-то другое.

  ПРАВИЛЬНАЯ КОМБИНАЦИЯ:
    # Директории внутри образа.
    RUN mkdir -p /app/config && \
        chown appuser:appuser /app/config

    # Файлы копируем сразу с владельцем.
    COPY --chown=appuser:appuser ./config /app/config

    # /app/data будет смонтирован снаружи. chown не поможет.
    # Его владельца решаем на хосте (следующий раздел).
    VOLUME /app/data

    USER appuser

  10. РЕШЕНИЕ 2: CHOWN НА ХОСТЕ
  Для bind mount — владельца определяет хост. Значит, на
  хосте и надо chown.

    # Хост, Linux/macOS.
    sudo chown -R 1000:1000 ./data

    # Хост, если контейнер под distroless (UID 65532).
    sudo chown -R 65532:65532 ./data

  ЧТО ЭТО ДАЁТ:
    Папка ./data теперь принадлежит UID 1000. Владелец
    на хосте. При монтировании в контейнер процесс под
    UID 1000 сможет писать.

  ПРОБЛЕМА НА WINDOWS:
    Windows не поддерживает UID/GID так, как Linux. При
    bind mount Docker Desktop делает маппинг. Обычно
    владельцы файлов виртуально становятся 0:0 (root).
    Приходится мириться или использовать WSL2.

  ПРОБЛЕМА С UID 65532:
    Если твой пользователь на хосте — UID 1000, а контейнер
    под UID 65532, то папка ./data должна принадлежать
    UID 65532. А это не твой пользователь. Ты не сможешь
    её редактировать без sudo.

    ЗАЧЕМ ТАК: обычно distroless используется для статичных
    сервисов без записи. Если сервису нужно писать на диск —
    либо меняй UID на свой (USER 1000:1000), либо используй
    named volume (следующий раздел).

  АВТОМАТИЗАЦИЯ ЧЕРЕЗ SCRIPT:
    Многие проекты имеют скрипт init.sh:
      #!/bin/bash
      mkdir -p ./data
      sudo chown -R 1000:1000 ./data
      docker compose up -d

    Или в Makefile:
      run:
        @mkdir -p data
        @sudo chown -R 1000:1000 data
        @docker compose up

  11. РЕШЕНИЕ 3: INIT-КОНТЕЙНЕР ДЛЯ NAMED VOLUME
  Named volume создаётся Docker'ом от root:root. Если контейнер
  работает под UID 1000, он не сможет писать в volume.

  РЕШЕНИЕ: отдельный контейнер для chown при первом запуске.

    services:
      volume-init:
        image: alpine:3.19
        user: root
        volumes:
          - app-data:/data
        command: sh -c "chown -R 1000:1000 /data && echo 'init done'"
        restart: "no"

      app:
        image: my-app:1.0
        user: "1000:1000"
        volumes:
          - app-data:/data
        depends_on:
          volume-init:
            condition: service_completed_successfully

    volumes:
      app-data:

  ЧТО ПРОИСХОДИТ:

    1. Docker создаёт named volume app-data.
    2. volume-init запускается под root, делает chown,
       завершается.
    3. app запускается под UID 1000, видит volume с
       правильным владельцем.

  ПОЧЕМУ ЭТО ЛУЧШЕ CHOWN В DOCKERFILE:
    Named volume — отдельная сущность Docker, не часть образа.
    chown в Dockerfile на неё не влияет.

  КОГДА ИСПОЛЬЗОВАТЬ:

    • PostgreSQL, MySQL, MongoDB — БД любят писать в volume.
    • Любой сервис с persistent-хранилищем под non-root.

  ОФИЦИАЛЬНЫЕ ОБРАЗЫ ТАК И ДЕЛАЮТ:
    Postgres, MongoDB, Elasticsearch имеют entrypoint-скрипт,
    который при старте делает chown на /var/lib/... Это
    стандартная практика.

  12. РЕШЕНИЕ 4: USER: "${UID}:${GID}" В COMPOSE
  Самое элегантное решение для bind mount при разработке.
  Используем UID и GID хоста.

  В docker-compose.yml:

    services:
      app:
        image: my-app:1.0
        user: "${UID}:${GID}"
        volumes:
          - ./data:/app/data
          - ./config:/app/config

  ПЕРЕД ЗАПУСКОМ:
    # Linux/macOS.
    export UID=$(id -u)
    export GID=$(id -g)
    docker compose up

    # Или в .env файле.
    echo "UID=$(id -u)" > .env
    echo "GID=$(id -g)" >> .env

  ЧТО ПРОИСХОДИТ:
    Docker Compose подставляет твои UID и GID хоста в
    конфиг. Контейнер запускается под твоим UID. Файлы
    в bind mount создаются с твоим владельцем. Никаких
    Permission denied.

  В POWERSHELL НА WINDOWS:
    \$env:UID="1000"
    \$env:GID="1000"
    docker compose up

    Или проще — запускать из WSL2, где UID/GID работают
    нативно.

  ПЛЮСЫ:
    • Работает без chown.
    • Файлы на хосте — твои.
    • Никаких sudo.

  МИНУСЫ:
    • Не работает в проде (там нет «твоего» UID).
    • Зависит от окружения.

  В ПРОДЕ — другой подход. Не используй ${UID} в продовом
  compose. Там чётко фиксированный UID из образа.

  ПРИМЕР ДЛЯ РАЗРАБОТКИ:
    # docker-compose.yml (для dev).
    services:
      app:
        build: .
        user: "${UID:-1000}:${GID:-1000}"
        volumes:
          - .:/app

  Флаг `:-` — дефолт, если переменная не задана.

  13. ПРАКТИКА: ПОЛНЫЙ DOCKERFILE
  Собираем всё вместе. Продакшен Dockerfile для Go-сервиса.

    # STAGE 1: builder
    FROM golang:1.22-alpine AS builder

    WORKDIR /src
    COPY go.mod go.sum* ./
    RUN go mod download
    COPY . .
    RUN CGO_ENABLED=0 go build -o /out/server .

    # STAGE 2: prod
    FROM alpine:3.19 AS prod

    # Ставим нужные пакеты.
    RUN apk add --no-cache ca-certificates tzdata curl

    # Создаём пользователя и группу с фиксированными UID/GID.
    RUN addgroup -g 1000 -S appgroup && \
        adduser -u 1000 -G appgroup -S -H -s /sbin/nologin appuser

    # Создаём директории для данных и логов.
    RUN mkdir -p /app/data /app/logs && \
        chown -R appuser:appgroup /app

    # Копируем бинарник сразу с владельцем.
    COPY --chown=appuser:appgroup --from=builder /out/server /app/server

    # Конфиги тоже.
    COPY --chown=appuser:appgroup ./config /app/config

    # Volume для persistent-данных.
    # Владельца volume решаем при запуске (init-контейнер
    # или bind mount с chown).
    VOLUME ["/app/data"]

    USER appuser:appgroup

    EXPOSE 8080
    ENV APP_ENV=production
    STOPSIGNAL SIGTERM

    # Healthcheck через curl (он есть в alpine).
    HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
      CMD curl -fsS http://localhost:8080/health || exit 1

    ENTRYPOINT ["/app/server"]

  ЗАПУСК:
    # Простой запуск (без persistence).
    docker run -d --name app -p 8080:8080 my-app:1.0

    # С bind mount (dev).
    mkdir -p ./data
    sudo chown -R 1000:1000 ./data
    docker run -d --name app \
      -p 8080:8080 \
      -v $(pwd)/data:/app/data \
      my-app:1.0

    # С named volume (prod).
    docker volume create app-data
    docker run -d --name app-init --rm \
      --user root \
      -v app-data:/data \
      alpine:3.19 chown -R 1000:1000 /data
    docker run -d --name app \
      -p 8080:8080 \
      -v app-data:/app/data \
      my-app:1.0

  ПРОВЕРКА:
    docker run --rm --entrypoint id my-app:1.0
    # uid=1000(appuser) gid=1000(appgroup)

    docker inspect my-app:1.0 -f '{{.Config.User}}'
    # appuser:appgroup

    docker exec app ls -la /app/server
    # -rwxr-xr-x 1 appuser appgroup ...

    docker exec app ls -la /app/data
    # владелец совпадает с UID, заданным при монтировании

  14. АНТИПАТТЕРНЫ
  14.1. ЗАПУСК ПОД ROOT В ПРОДЕ.
    Пробили сервис — root внутри. PodSecurityPolicy в k8s
    может вообще не запустить. Всегда USER.
  14.2. USER ДО APK/APT-GET.
    Установка пакетов требует root. Сначала установки,
    потом USER.
  14.3. USER APPUSER БЕЗ ADDUSER В SCRATCH.
    Нет /etc/passwd — ошибка при запуске. Только UID или
    копируй /etc/passwd.
  14.4. COPY БЕЗ --CHOWN ПОД NON-ROOT.
    Файлы root:root, процесс не читает. Permission denied.
  14.5. CHOWN В DOCKERFILE ДЛЯ BIND MOUNT.
    Bind mount перезаписывает владельца из образа. chown
    в Dockerfile не помогает.
  14.6. ROOT ПИШЕТ В BIND MOUNT.
    Файлы на хосте становятся root:root. Не удаляются
    без sudo.
  14.7. NAMED VOLUME БЕЗ INIT-КОНТЕЙНЕРА.
    Volume создаётся root:root. Non-root процесс не пишет.
  14.8. ${UID} В ПРОДОВОМ COMPOSE.
    В проде нет «твоего» UID. Фиксируй UID в образе.
  14.9. ПОРТ 80 БЕЗ ROOT.
    Порты < 1024 требуют root или CAP_NET_BIND_SERVICE.
    Используй 8080.
  14.10. ФАЙЛЫ С ПРАВАМИ 0777 ДЛЯ ОБХОДА ПРОБЛЕМ.
    Это не решение, это скрытая уязвимость. Решай через
    UID/GID.
  14.11. ОДИН UID НА ВСЕХ В МОНОРЕПО.
    Если у тебя 20 сервисов под одним UID — пробой одного
    открывает доступ к данным других. Разные UID для
    разных сервисов.
  14.12. distroless:nonroot И ЖДЁШЬ UID 1000.
    У distroless UID 65532. Не совпадает с UID хоста.
    Для bind mount это проблема.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Non-root — стандарт для прода. Безопасность плюс
      правильные права на volumes.
  2.  UID и GID — числа. Ядро работает только с ними.
      Имена — удобство для человека.
  3.  USER переключает пользователя для последующих RUN,
      CMD, ENTRYPOINT. Все root-задачи — до USER.
  4.  Alpine: adduser -D -u 1000 appuser && USER appuser.
  5.  Distroless: :nonroot даёт UID 65532 из коробки.
      Никаких adduser.
  6.  Scratch: только USER 1000:1000 или копируй
      /etc/passwd из builder'а.
  7.  COPY --chown=1000:1000 — файлы сразу с правильным
      владельцем. Иначе root:root и Permission denied.
  8.  Bind mount: UID внутри контейнера должен совпадать
      с UID владельца папки на хосте.
  9.  Четыре решения проблем с volumes:
        • chown в Dockerfile — для директорий внутри образа.
        • chown на хосте — для bind mount.
        • init-контейнер — для named volume.
        • user: "${UID}:${GID}" — для разработки.
  10. Named volume создаётся root:root. Non-root не пишет.
      Нужен init-контейнер или официальный entrypoint
      образа (как у Postgres).
  11. ${UID} в compose — только для разработки. В проде
      фиксируй UID.
  12. Антипаттерны: root в проде, USER до установки
      пакетов, COPY без chown, chown в Dockerfile для
      bind mount, 0777, один UID на всё.
*/
