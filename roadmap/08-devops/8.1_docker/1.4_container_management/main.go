package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

/*
  УРОК 1.4: УПРАВЛЕНИЕ КОНТЕЙНЕРОМ
  Контейнер — это процесс. Не «маленькая ВМ», не «песочница»,
  а обычный Linux-процесс, изолированный через namespaces и
  ограниченный через cgroups. У него есть жизненный цикл, как
  у любого процесса: старт, работа, остановка, удаление.
  Управление контейнером — это набор команд и правил, которые
  позволяют контролировать этот жизненный цикл: запустить,
  посмотреть, зайти внутрь, ограничить ресурсы, перезапустить
  при падении, удалить. Без этих навыков ты не отладишь прод
  и не поймёшь, почему сервис «упал и не поднялся».

  СОДЕРЖАНИЕ:
    1.  Жизненный цикл контейнера
    2.  Запуск: docker run и его флаги
    3.  Просмотр: ps, logs, inspect, stats
    4.  Вход внутрь: exec и attach
    5.  Остановка: stop, kill, и что происходит с сигналами
    6.  Перезапуск: start, restart, rm
    7.  Restart policies — поведение при падении
    8.  Ограничение ресурсов: memory, cpu
    9.  Volumes и mounts
    10. Переменные окружения: -e, --env-file
    11. Healthchecks при запуске
    12. Docker events и системные команды
    13. Отладка в проде: distroless и debug-образы
    14. Антипаттерны
    15. Финальные выводы

  1. ЖИЗНЕННЫЙ ЦИКЛ КОНТЕЙНЕРА
  Контейнер — процесс. У него пять состояний:

    CREATED   → создан, но не запущен (docker create)
       ↓
    RUNNING   → работает (docker run / start)
       ↓
    PAUSED    → приостановлен (docker pause, редко)
       ↓
    STOPPED   → остановлен, но существует (docker stop)
       ↓
    REMOVED   → удалён, RW-слой уничтожен (docker rm)

  КЛЮЧЕВОЕ РАЗЛИЧИЕ:
    STOP ≠ RM.

      docker stop — посылает SIGTERM, ждёт stop_grace_period
                    (по умолчанию 10 сек), потом SIGKILL.
                    Контейнер переходит в STOPPED. Файловая
                    система (writable-слой) сохраняется.

      docker rm — удаляет контейнер. Writable-слой исчезает.
                  Всё, что было записано внутри, — потеряно
                  (если не было volumes).

  ПРАКТИЧЕСКОЕ СЛЕДСТВИЕ: контейнеры эфемерны. Не храни
  данные в контейнере — они потеряются. Данные — в volumes,
  в БД, в S3.

  2. ЗАПУСК: DOCKER RUN И ЕГО ФЛАГИ
  docker run — самая используемая команда. Она делает три вещи:
    1. Создаёт контейнер из образа.
    2. Запускает его.
    3. Ждёт или отцепляется (зависит от -d).

  ПОЛЕЗНЫЕ ФЛАГИ:
    -d                    detached: запустить в фоне.
    -it                   interactive + tty: интерактив.
    --name my-app         имя контейнера.
    -p 8080:8080          публикация порта (хост:контейнер).
    -e KEY=value          переменная окружения.
    --env-file .env       все переменные из файла.
    -v /host:/container   bind mount.
    -v myvol:/data        named volume.
    --rm                  удалить контейнер после остановки.
    --restart <policy>    политика перезапуска.
    --memory 512m         лимит памяти.
    --cpus 1.5            лимит CPU.
    --network mynet       сеть.
    --entrypoint /bin/sh  переопределить ENTRYPOINT.
    --health-cmd          команда healthcheck.
    --user 1000:1000      под каким пользователем.

  ПРИМЕРЫ:
    # nginx в фоне на порту 8080
    docker run -d -p 8080:80 --name web nginx:1.27

    # Postgres с volume и env
    docker run -d \
      -e POSTGRES_PASSWORD=secret \
      -v pgdata:/var/lib/postgresql/data \
      --name db postgres:16

    # Зайти в alpine для отладки
    docker run -it --rm alpine:3.19 sh

    # С лимитами и политикой рестарта
    docker run -d \
      --name api \
      --restart unless-stopped \
      --memory 512m --cpus 1 \
      -p 8080:8080 my-app:1.0

  РАЗНИЦА -i И -t:
    -i — оставить STDIN открытым.
    -t — выделить псевдотерминал.
    Вместе (-it) — нормальный интерактивный shell.

  3. ПРОСМОТР: PS, LOGS, INSPECT, STATS

  3.1. DOCKER PS
    docker ps              — запущенные.
    docker ps -a           — все (включая остановленные).
    docker ps -q           — только ID.
    docker ps --filter "status=exited"
    docker ps --filter "name=api"
    docker ps --format "table {{.Names}}\t{{.Status}}"

  Важный столбец — STATUS:
    Up 5 minutes                → всё ок.
    Up 5 minutes (healthy)      → healthcheck проходит.
    Up 5 minutes (unhealthy)    → healthcheck падает.
    Exited (0)                  → нормальное завершение.
    Exited (137)                → SIGKILL, часто OOM.
    Restarting (1) 5 seconds ago → падает и перезапускается.

  3.2. DOCKER LOGS
    docker logs <id>              — все логи.
    docker logs -f <id>           — follow.
    docker logs --tail 100 <id>   — последние 100 строк.
    docker logs --since 5m <id>   — за 5 минут.
    docker logs --timestamps <id> — с таймстемпами.

  КАК ЛОГИРОВАТЬ В GO: писать в stdout/stderr. Не в файлы.
  Тогда docker logs работает.

  РОТАЦИЯ ЛОГОВ:
    docker run --log-opt max-size=10m --log-opt max-file=3 ...

  Без ротации логи растут до бесконечности и забивают диск.

  3.3. DOCKER INSPECT
    docker inspect <id>

  Что доставать:
    docker inspect <id> -f '{{.State.Status}}'        — статус.
    docker inspect <id> -f '{{.State.ExitCode}}'      — exit code.
    docker inspect <id> -f '{{.State.OOMKilled}}'     — OOM?
    docker inspect <id> -f '{{.RestartCount}}'        — рестарты.
    docker inspect <id> -f '{{.NetworkSettings.IPAddress}}'
    docker inspect <id> | grep -A 5 "Mounts"          — volumes.
    docker inspect <id> | grep -A 5 "Env"             — переменные.

  Последние — самые полезные для диагностики.

  3.4. DOCKER STATS
    docker stats                    — все (live).
    docker stats <id> <id2>         — конкретные.
    docker stats --no-stream        — один снимок и выход.

  MEM USAGE / LIMIT — самое важное. Если близко к 100% —
  контейнер скоро упадёт по OOM.

  4. ВХОД ВНУТРЬ: EXEC И ATTACH

  4.1. DOCKER EXEC — ПРАВИЛЬНЫЙ СПОСОБ
  Запустить НОВУЮ команду внутри работающего контейнера.

    docker exec -it <id> sh              — зайти в shell.
    docker exec -it <id> bash            — если bash есть.
    docker exec <id> ls /app             — одна команда.
    docker exec -it -u root <id> sh      — под root.
    docker exec -it -e DEBUG=1 <id> sh   — с env.

  exec запускает команду в ТОМ ЖЕ контейнере, но как отдельный
  процесс. Твой основной сервис (PID 1) продолжает работать.
  Это безопасно.

  4.2. DOCKER ATTACH — ОСТОРОЖНО
    docker attach <id>

  ЧТО НЕ ТАК:
    • Ты видишь вывод процесса, но не можешь запускать команды.
    • Ctrl+C отправляет SIGINT в PID 1 → может убить сервис.
    • Ctrl+P Ctrl+Q — отцепиться без остановки.

  ПРАВИЛО: почти всегда exec, не attach.

  4.3. ЧТО ДЕЛАТЬ, ЕСЛИ SHELL НЕТ
  В distroless/scratch нет sh. Зайти нельзя.
  Решения:
    • Debug-образ на alpine с тем же бинарником.
    • docker cp для копирования файлов.
    • kubectl debug в K8s.
    • nsenter для глубокой отладки (см. раздел 13).

  5. ОСТАНОВКА: STOP, KILL, И СИГНАЛЫ

  5.1. DOCKER STOP
    docker stop <id>            — SIGTERM, 10 сек, SIGKILL.
    docker stop -t 30 <id>      — SIGTERM, 30 сек, SIGKILL.
    docker stop $(docker ps -q) — остановить все.

  Что происходит:
    1. Docker посылает SIGTERM процессу с PID 1.
    2. Ждёт stop_grace_period (дефолт 10 сек).
    3. Если процесс не завершился — SIGKILL.
    4. Exit code 137 (128+9) если убит SIGKILL.
    5. Exit code 0 если завершился корректно.

  Если shell-форма ENTRYPOINT — SIGTERM не доходит, контейнер
  висит 10 секунд и умирает от SIGKILL.

  5.2. DOCKER KILL
    docker kill <id>              — SIGKILL, мгновенно.
    docker kill -s SIGTERM <id>   — послать SIGTERM вручную.
  kill = «убей немедленно». Приложение не успеет ничего сделать.
  Используй только когда stop не помог.

  5.3. ОСТАНОВКА ВСЕХ
    docker stop $(docker ps -q)              — все.
    docker kill $(docker ps -q)              — жёстко все.
    docker rm -f $(docker ps -aq)            — удалить все.
  Последнее — опасно. Осторожно.

  6. ПЕРЕЗАПУСК: START, RESTART, RM

  docker start <id>      — запустить остановленный.
  docker restart <id>    — stop + start.
  docker rm <id>         — удалить.
  docker rm -f <id>      — force: stop + rm.

  docker start ПЕРЕИСПОЛЬЗУЕТ writable-слой. Все изменения
  в файловой системе сохраняются.

  Пример:
    docker run -d --name app alpine sleep 3600
    docker exec app touch /tmp/test
    docker stop app
    docker start app
    docker exec app cat /tmp/test    # файл на месте!

    docker rm -f app
    docker run -d --name app alpine sleep 3600
    docker exec app cat /tmp/test    # файла нет!

  ПРАВИЛО: docker run = новый контейнер. docker start = тот же.

  7. RESTART POLICIES
  NO (дефолт):
    Не перезапускать.

  ALWAYS:
    Всегда перезапускать. Даже после docker stop — начнёт
    перезапускаться при следующей загрузке демона.

  ON-FAILURE:
    Перезапускать только при ненулевом exit code.

    docker run --restart on-failure:5 my-app    — максимум 5 раз

  UNLESS-STOPPED:
    Перезапускать, НО не после явного docker stop.

  ЧТО ВЫБРАТЬ:

    Сервис в проде          → unless-stopped.
    Одноразовая задача      → no (или on-failure:5).
    Демон-сервис на хосте   → always.
    Разработка              → no.

  ПРОБЛЕМА ALWAYS: docker stop → перезагрузка хоста → Docker
  daemon стартует → контейнер снова поднимается.

  ПРОБЛЕМА ON-FAILURE БЕЗ ЛИМИТА: сервис падает сразу при
  старте → бесконечные рестарты → логи и CPU сжигаются.

  ПРАВИЛО ДЛЯ GO-СЕРВИСОВ: unless-stopped.

  8. ОГРАНИЧЕНИЕ РЕСУРСОВ: MEMORY, CPU

  8.1. MEMORY
    docker run --memory 512m my-app
    docker run --memory 1g --memory-swap 1g my-app  — без свопа.
    docker run --memory 512m --memory-reservation 256m my-app

  При превышении — kernel OOM-killer убивает процесс.
    • Exit code 137.
    • docker inspect: State.OOMKilled = true.
    • dmesg: "Killed process ...".

  GOMEMLIMIT: Go 1.19+ учитывает cgroup limit автоматически.
  Но при ручном GOMEMLIMIT оставляй запас:
    --memory 512m → GOMEMLIMIT=450MiB
В реальных проектах GOMEMLIMIT нужно устанавливать вручную.

  8.2. CPU
    docker run --cpus 1.5 my-app
    docker run --cpus 0.5 my-app
    docker run --cpu-shares 512 my-app

  --cpus — жёсткий лимит, throttling при превышении.
  --cpu-shares — относительный вес при конкуренции.

  8.3. ДИАГНОСТИКА
    docker inspect <id> -f '{{.State.OOMKilled}}'   — true/false.
    docker inspect <id> -f '{{.State.ExitCode}}'    — 137 = OOM.
  ПРАВИЛО: всегда ставь --memory для сервисов в проде.

  9. VOLUMES И MOUNTS
  Есть три способа подключить данные к контейнеру.

  9.1. NAMED VOLUME
  Docker управляет хранилищем. Живёт отдельно от контейнера.

    docker run -v pgdata:/var/lib/postgresql/data postgres:16

  Где хранится: /var/lib/docker/volumes/pgdata/_data.
  Переживает docker rm контейнера.
  Удаляется только через `docker volume rm pgdata`.

  ПЛЮСЫ:
    • Управляется Docker.
    • Кроссплатформенно.
    • Производительность — как у нативного диска.
  МИНУСЫ:
    • Файлы не видны в обычной файловой системе хоста.

  ИСПОЛЬЗУЙ ДЛЯ: БД, кэшей, всего что должно пережить контейнер.

  9.2. BIND MOUNT
  Папка хоста монтируется внутрь контейнера.

    docker run -v /home/user/data:/app/data my-app
    docker run -v $(pwd):/app my-app          — текущая папка.
    docker run -v ./config.yaml:/app/config.yaml:ro my-app

  ПЛЮСЫ:
    • Файлы видны на хосте.
    • Можно редактировать в IDE.

  МИНУСЫ:
    • Права доступа (UID/GID) — частая головная боль.
    • На macOS/Windows медленнее named volume (через виртуализацию).
    • Привязка к путям хоста — плохо для переносимости.

  ИСПОЛЬЗУЙ ДЛЯ: разработки (hot reload), конфигов, логов.

  9.3. TMPFS MOUNT
  Данные в памяти. Не сохраняются после остановки.

    docker run --tmpfs /tmp my-app
    docker run --tmpfs /tmp:size=100m,ro my-app

  ПЛЮСЫ:
    • Быстро (RAM).
    • Не попадает на диск.

  ИСПОЛЬЗУЙ ДЛЯ: временных файлов, кэша, чувствительных данных
  (пароли в памяти, чтобы не попали на диск).

  9.4. --MOUNT (СОВРЕМЕННЫЙ СИНТАКСИС)
  Более читаемая альтернатива -v:
    docker run --mount type=volume,src=pgdata,dst=/data postgres:16
    docker run --mount type=bind,src=/host,dst=/container my-app
    docker run --mount type=tmpfs,dst=/tmp my-app

  Флаги:
    • type=volume|bind|tmpfs
    • src= — источник.
    • dst= — точка монтирования.
    • readonly или ro — только для чтения.

  ПРАВИЛО: используй --mount в скриптах (читаемее), -v для
  быстрого запуска в терминале.

  9.5. ЧТО ВЫБРАТЬ
    БД, кэш, постоянные данные → named volume.
    Разработка, конфиги       → bind mount.
    Временные файлы           → tmpfs.

  10. ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ: -E, --ENV-FILE

  Переменные окружения — стандартный способ передать конфиг
  в контейнер. Не хардкодь их в образ.

  10.1. ЧЕРЕЗ -e
    docker run -e APP_ENV=production -e PORT=8080 my-app
  Каждый раз указывать руками — неудобно, если их много.

  10.2. ЧЕРЕЗ --ENV-FILE
    docker run --env-file .env my-app

  Файл .env:
    APP_ENV=production
    PORT=8080
    DB_HOST=postgres
    DB_USER=app

  ПРАВИЛА:
    • Комментарии через #.
    • Пустые строки игнорируются.
    • Формат: KEY=value. Без кавычек.
    • Не коммить .env в git.

  10.3. ENV ИЗ ОБРАЗА (ENV в Dockerfile)
  Если в Dockerfile было ENV APP_ENV=production — она будет
  в контейнере по умолчанию.

  Переопределяется через -e:
    docker run -e APP_ENV=staging my-app

  Проверить все env контейнера:
    docker inspect <id> | grep -A 20 "Env"
    docker exec <id> env

  10.4. ПРАВИЛА БЕЗОПАСНОСТИ
    • Секреты (пароли, токены) НЕЛЬЗЯ пихать в -e или --env-file
      в проде. Они видны через docker inspect.
    • Для секретов — Docker secrets, Vault, AWS Secrets Manager.
    • В образе — не ENV, а ARG (для сборки) или runtime-injection.

  10.5. ENV В GO
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"    // дефолт
    }

    Всегда дефолт на случай, если переменная не задана.

  11. HEALTHCHECKS ПРИ ЗАПУСКЕ
  HEALTHCHECK можно задать не только в Dockerfile, но и при
  run.

  11.1. ЧЕРЕЗ DOCKER RUN
    docker run --health-cmd="curl -f http://localhost:8080/health || exit 1" \
      --health-interval=30s \
      --health-timeout=3s \
      --health-retries=3 \
      --health-start-period=5s \
      my-app

  Параметры:
    • --health-cmd — команда проверки.
    • --health-interval — как часто проверять.
    • --health-timeout — сколько ждать ответа.
    • --health-retries — сколько провалов до unhealthy.
    • --health-start-period — грейс при старте.

  11.2. ПРОСМОТР СТАТУСА
    docker ps
    # STATUS: Up 5 minutes (healthy) или (unhealthy)

    docker inspect <id> -f '{{.State.Health.Status}}'
    # starting / healthy / unhealthy

    docker inspect <id> -f '{{json .State.Health}}' | jq
    # Полная информация, включая последние проверки и их вывод.

  11.3. ПРОБЛЕМА С DISTROLESS
  В distroless нет curl. Решения:
    • Встроенная подкоманда в Go-бинарник.
    • TCP-проверка через /dev/tcp (только если есть bash).
    • Healthcheck переносится в оркестратор (k8s probe).

  ПРАВИЛО: healthcheck обязателен для сервисов в проде.
  Он позволяет оркестратору понять, когда под готов
  принимать трафик, а когда его надо заменить.

  12. DOCKER EVENTS И СИСТЕМНЫЕ КОМАНДЫ

  12.1. DOCKER EVENTS
  Мониторинг событий Docker в реальном времени.
    docker events                          — все события.
    docker events --filter "container=api" — по контейнеру.
    docker events --filter "event=die"     — только смерть.
    docker events --since 1h                — за час.
    docker events --until 5m                — до 5 минут назад.

  Что увидишь:
    container create, start, die, stop, destroy, health_status.

  Полезно для отладки: видно, когда контейнер запустился, упал,
  почему был перезапущен.

  12.2. DOCKER SYSTEM DF
  Сколько места занимает Docker.
    docker system df
    # TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
    # Images          15        8         3.5GB     1.2GB
    # Containers      10        3         250MB     200MB
    # Volumes         5         4         1.5GB     100MB
    # Build Cache     -         -         800MB     800MB

  RECLAIMABLE — сколько можно освободить.

  12.3. DOCKER SYSTEM PRUNE
  Очистка неиспользуемого.
    docker system prune              — удалить неиспользуемые образы, контейнеры, сети.
    docker system prune -a           — ещё и все образы без контейнеров.
    docker system prune -a --volumes — ОПАСНО: удалит все volumes без контейнеров.

    docker container prune           — только остановленные контейнеры.
    docker image prune               — только dangling-образы.
    docker image prune -a            — все образы без контейнеров.
    docker volume prune              — volumes без контейнеров.
    docker builder prune             — кэш сборки.

  ПРАВИЛО: раз в неделю делай `docker system df` и чисти
  неиспользуемое. Иначе диск заполнится.

  12.4. DOCKER SYSTEM INFO
    docker system info
  Показывает: количество контейнеров, образов, драйверы,
  ресурсы, версии. Полезно при отладке проблем с Docker.

  13. ОТЛАДКА В ПРОДЕ: DISTROLESS И DEBUG-ОБРАЗЫ
  Проблема: в проде образ на distroless/scratch. Shell нет.
  РЕШЕНИЯ:

  13.1. DEBUG-ОБРАЗ
  Собирай два образа из одного Dockerfile:

    FROM gcr.io/distroless/static-debian12:nonroot AS prod
    COPY --from=builder /app/server /server
    ENTRYPOINT ["/server"]

    FROM alpine:3.19 AS debug
    RUN apk add --no-cache strace curl jq
    COPY --from=builder /app/server /server
    ENTRYPOINT ["/server"]

  Прод — distroless. Debug — alpine со strace.

  13.2. ЗАПУСК DEBUG С SHELL
    docker run -it --rm --entrypoint sh app:debug
  Ты попадаешь в alpine-версию сервиса. Бинарник, env,
  конфиги — те же.

  13.3. EPHEMERAL CONTAINERS В K8S
    kubectl debug -it <pod> --image=alpine
  Отдельный контейнер в том же pod. Видит те же volumes и
  network namespace.

  13.4. DOCKER CP
    docker cp <id>:/app/logs ./logs
    docker cp ./config.yaml <id>:/app/
  Работает без shell.

  13.5. NSENTER
    PID=$(docker inspect -f '{{.State.Pid}}' <id>)
    nsenter -t $PID -n -p -m
  Заходишь в namespace контейнера прямо с хоста. Мощный
  инструмент для серьёзной отладки.

  14. АНТИПАТТЕРНЫ

  14.1. ИСПОЛЬЗОВАТЬ ATTACH ВМЕСТО EXEC.
    Ctrl+C убивает сервис. Используй exec.
  14.2. DOCKER KILL БЕЗ ПРИЧИНЫ.
    SIGKILL рвёт соединения, теряет данные.
  14.3. НЕТ ЛИМИТА ПАМЯТИ.
    Один контейнер может убить весь хост.
  14.4. НЕТ ЛИМИТА НА ЛОГИ.
    Логи растут до бесконечности. Ставь --log-opt.
  14.5. `restart: always` ДЛЯ ВСЕГО.
    После перезагрузки хоста контейнеры поднимаются, даже
    если их явно остановили.
  14.6. `on-failure` БЕЗ ЛИМИТА.
    Бесконечные рестарты. Ставь on-failure:5.
  14.7. ХРАНИТЬ ДАННЫЕ В КОНТЕЙНЕРЕ.
    docker rm уничтожает всё. Данные — в volumes.
  14.8. ПИСАТЬ ЛОГИ В ФАЙЛЫ ВНУТРИ КОНТЕЙНЕРА.
    docker logs их не покажет.
  14.9. BIND MOUNT С НЕПРАВИЛЬНЫМИ UID/GID.
    Permission denied. Используй --user $(id -u):$(id -g).
  14.10. СЕКРЕТЫ В -e ИЛИ --env-file.
    Видны в docker inspect. Используй Docker secrets или Vault.
  14.11. ЗАБЫТЬ ПРО `--rm` ДЛЯ ОДНОРАЗОВЫХ.
    `docker ps -a` забивается мусором.
  14.12. НЕ ЧИСТИТЬ DOCKER SYSTEM.
    Диск заполняется образами и volumes. Раз в неделю — prune.
  14.13. НЕТ HEALTHCHECK.
    Оркестратор не знает, готов ли сервис, не может
    перезапустить зависший.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  Контейнер — процесс. Жизненный цикл: create → run →
      pause → stop → remove. Stop ≠ rm.
  2.  docker run — основная команда. Флаги: -d, -it, -p, -e,
      -v, --rm, --restart, --memory, --env-file.
  3.  docker ps показывает статус. Exited (137) = OOM-kill.
      Restarting = падает и перезапускается.
  4.  docker logs работает только если приложение пишет в
      stdout/stderr. В Go — slog в os.Stdout.
  5.  docker inspect — мета. Доставай State.Status,
      State.ExitCode, State.OOMKilled через -f.
  6.  docker stats — ресурсы в реальном времени.
  7.  docker exec — единственный безопасный способ зайти
      внутрь. attach — опасно.
  8.  docker stop = SIGTERM + 10 сек + SIGKILL.
      docker kill = SIGKILL сразу. Используй stop.
  9.  docker start переиспользует контейнер. docker run
      создаёт новый. Разные вещи.
  10. Restart policies: unless-stopped для сервисов,
      no или on-failure:5 для одноразовых.
  11. --memory обязателен. Exit code 137 = OOM.
      GOMEMLIMIT на 10-20% меньше лимита.
  12. Volumes: named для данных, bind для разработки,
      tmpfs для временных. --mount читаемее -v.
  13. Env: -e для единичных, --env-file для множества.
      Секреты — НЕ через env, а через Vault/secrets.
  14. Healthcheck: обязателен для сервисов. В distroless —
      через встроенную команду.
  15. docker events и system prune — обязательная гигиена.
      Раз в неделю проверяй df и чисти.
  16. Для distroless — debug-образ, docker cp или nsenter.
  17. Логи ротацией: --log-opt max-size=10m --log-opt
      max-file=3.

*/

var (
	version = "dev" // переопределяется через -ldflags
	commit  = "none"
)

func main() {
	// Structured logging в stdout — Docker сам подхватит.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("LOG_LEVEL")),
	}))

	logger.Info("starting",
		"version", version,
		"commit", commit,
		"pid", os.Getpid(),
	)

	// Конфиг из env — стандарт для контейнеров.
	cfg := Config{
		Port:        envOr("PORT", "8080"),
		AppEnv:      envOr("APP_ENV", "dev"),
		ShutdownTTL: envDuration("SHUTDOWN_TIMEOUT", 20*time.Second),
	}

	// Собираем HTTP-роуты.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ready", handleReady)
	mux.HandleFunc("/info", handleInfo(version, commit, cfg))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown — критично для контейнеров.
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		logger.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(), cfg.ShutdownTTL)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("stopped cleanly")
}

// --- Handlers ---

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	// Здесь была бы проверка БД, кэша и т.д.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func handleInfo(version, commit string, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"version":"` + version + `",` +
				`"commit":"` + commit + `",` +
				`"app_env":"` + cfg.AppEnv + `",` +
				`"pid":` + strconv.Itoa(os.Getpid()) + `}`,
		))
	}
}

// --- Config ---

type Config struct {
	Port        string
	AppEnv      string
	ShutdownTTL time.Duration
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
