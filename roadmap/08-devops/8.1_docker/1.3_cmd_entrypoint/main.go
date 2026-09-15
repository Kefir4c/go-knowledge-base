package cmdentrypoint

/*
  УРОК 9.1.3: CMD vs ENTRYPOINT
  Кажется, что две инструкции делают одно и то же — задают команду
  запуска контейнера. На самом деле разница критична, и половина
  багов в проде связана с тем, что разработчики не понимают, как
  они работают вместе.
  Особенно это важно для Go-сервисов: неправильная форма ENTRYPOINT
  ломает graceful shutdown, из-за чего при деплое рвутся соединения
  и теряются данные.

  СОДЕРЖАНИЕ:
    1.  Проблема: что задаёт команду запуска
    2.  CMD — дефолтные аргументы
    3.  ENTRYPOINT — фиксированная команда
    4.  Как они работают вместе + таблица комбинаций
    5.  Exec-форма vs shell-форма
    6.  Что под капотом при shell-форме и почему ломается shutdown
    7.  PID 1 problem и Unix signals
    8.  Tini и dumb-init
    9.  STOPSIGNAL и stop_grace_period
    10. ENTRYPOINT в distroless и scratch
    11. ENTRYPOINT в Kubernetes и docker-compose
    12. Что использовать в Go-сервисе
    13. Как отлаживать проблемы с сигналами
    14. Антипаттерны
    15. Финальные выводы

  1. ПРОБЛЕМА: ЧТО ЗАДАЁТ КОМАНДУ ЗАПУСКА
  Когда ты пишешь `docker run my-app`, Docker должен понять,
  что именно запустить внутри контейнера. Есть три источника:

    • ENTRYPOINT — «команда».
    • CMD — «аргументы по умолчанию».
    • Аргументы в `docker run my-app <args>` — «аргументы
      пользователя».

  Docker берёт ENTRYPOINT + (аргументы из run ИЛИ CMD, если
  аргументов не передали). Из этого складывается финальная
  команда.

  Нюансы:
    • Что если ENTRYPOINT нет, а CMD есть?
    • Что если оба есть?
    • Что если пользователь передал свои аргументы?
    • Что если ENTRYPOINT в shell-форме?

  Разберём каждое.

  2. CMD — ДЕФОЛТНЫЕ АРГУМЕНТЫ
  CMD задаёт ДЕФОЛТ: что запускать, если пользователь ничего
  не передал. Это легко переопределить.

    CMD ["./server"]

  Что происходит при `docker run my-app`: запустится ./server

  Что происходит при `docker run my-app --port 9090`:
    запустится --port 9090 — попытка выполнить файл с именем
    «--port», упадёт с ошибкой «executable not found».

  Видишь проблему? CMD полностью перезаписывается аргументами
  из `docker run`. Это не «добавление аргументов», это «замена
  всей команды».

  ОСОБЕННОСТЬ: CMD может быть только один в Dockerfile. Если
  написать два — сработает последний.

  3. ENTRYPOINT — ФИКСИРОВАННАЯ КОМАНДА
  ENTRYPOINT задаёт саму команду. Она фиксирована: аргументы
  из `docker run` не заменяют её, а ДОБАВЛЯЮТСЯ к ней.

    ENTRYPOINT ["./server"]

  Что происходит при `docker run my-app`: запустится ./server

  Что происходит при `docker run my-app --port 9090`:
    запустится ./server --port 9090

  Аргументы из run приклеиваются к ENTRYPOINT.

  Переопределить ENTRYPOINT можно только явным флагом:

    docker run --entrypoint /bin/sh my-app

  Это редко используется. ENTRYPOINT тоже может быть только один.

  4. КАК ОНИ РАБОТАЮТ ВМЕСТЕ + ТАБЛИЦА
  Самое мощное — комбинация. ENTRYPOINT задаёт команду, CMD —
  дефолтные аргументы для неё.

    ENTRYPOINT ["/app/server"]
    CMD ["--port", "8080", "--config", "/etc/app.yaml"]

  Что происходит при `docker run my-app`:
    /app/server --port 8080 --config /etc/app.yaml

  Что происходит при `docker run my-app --port 9090`:
    /app/server --port 9090
    ← CMD переопределён целиком. Обрати внимание: --config пропал!

  ВАЖНЫЙ НЮАНС: если пользователь передаёт хоть один аргумент —
  CMD заменяется ЦЕЛИКОМ, а не дополняется. Если в CMD было
  несколько аргументов — все исчезнут, кроме тех, что передал
  пользователь. Это сбивает с толку.

  ТАБЛИЦА ВСЕХ КОМБИНАЦИЙ:
  ┌──────────────────┬─────────────────────┬───────────────────────┐
  │ Dockerfile       │ docker run          │ Что запустится        │
  ├──────────────────┼─────────────────────┼───────────────────────┤
  │ CMD ["app"]      │ docker run img      │ app                   │
  │ CMD ["app"]      │ docker run img a b  │ a b                   │
  ├──────────────────┼─────────────────────┼───────────────────────┤
  │ ENTRY ["app"]    │ docker run img      │ app                   │
  │ ENTRY ["app"]    │ docker run img a b  │ app a b               │
  ├──────────────────┼─────────────────────┼───────────────────────┤
  │ ENTRY ["app"]    │ docker run img      │ app def               │
  │ CMD   ["def"]    │                     │                       │
  │ ENTRY ["app"]    │ docker run img x    │ app x                 │
  │ CMD   ["def"]    │                     │ (CMD целиком = x)     │
  └──────────────────┴─────────────────────┴───────────────────────┘

  5. EXEC-ФОРМА VS SHELL-ФОРМА
  У CMD и ENTRYPOINT есть две формы записи.

  EXEC-ФОРМА — массив строк:
    ENTRYPOINT ["/app/server"]
    CMD ["--port", "8080"]

  Прямой запуск, без шелла. Docker вызывает syscall exec().
  Процесс становится PID 1 контейнера.

  SHELL-ФОРМА — обычная строка:
    ENTRYPOINT /app/server
    CMD --port 8080

  Docker оборачивает в /bin/sh -c:
    ENTRYPOINT ["/bin/sh", "-c", "/app/server"]

  Реально запускается /bin/sh, а он уже запускает /app/server.
  PID 1 — это /bin/sh, а не твой сервис.

  Проверить можно через `docker exec <id> ps aux`:

    EXEC-форма:   PID 1 = /app/server
    SHELL-форма:  PID 1 = /bin/sh -c /app/server
                  /app/server — это PID 7 или 8

  ПРАВИЛО: для Go-сервисов — ВСЕГДА exec-форма. Shell-форма
  допустима только для RUN (там && удобны), но не для
  ENTRYPOINT и CMD.

  6. ЧТО ПОД КАПОТОМ ПРИ SHELL-ФОРМЕ И ПОЧЕМУ ЛОМАЕТСЯ SHUTDOWN
  Когда Docker видит shell-форму, он делает:

    /bin/sh -c "<твоя команда>"

  Что делает /bin/sh -c:
    1. Парсит строку команды (переменные, кавычки, pipe'ы, &&).
    2. Делает fork() — создаёт дочерний процесс.
    3. В дочернем вызывает execve() для твоей команды.
    4. Родитель (sh) ждёт завершения дочернего.
    5. Возвращает его exit code.

  В контейнере появляется ДВА процесса:

    PID 1: /bin/sh -c "/app/server"    ← родитель
    PID 7: /app/server                 ← дочерний

  Твой сервис — НЕ PID 1. Он дочерний процесс шелла.

  Теперь смотри, что происходит с сигналами. Когда Docker делает
  `docker stop`, он:
    1. Посылает SIGTERM процессу с PID 1 в контейнере.
    2. Ждёт stop_grace_period (по умолчанию 10 секунд).
    3. Если процесс не завершился — посылает SIGKILL.
    4. SIGKILL перехватить НЕЛЬЗЯ.

  SIGTERM уходит в /bin/sh. Что делает /bin/sh? Ничего. Он
  в состоянии wait() и ждёт, пока дочерний процесс завершится
  сам. Он НЕ пробрасывает SIGTERM дочернему.

  Твой Go-сервис не получает сигнал. Продолжает работать.
  Через 10 секунд Docker посылает SIGKILL — уже всему контейнеру.
  Сервис умирает жёстко, БЕЗ graceful shutdown.

  ЧТО ЭТО ЗНАЧИТ НА ПРАКТИКЕ:
    • Открытые соединения с БД не закрылись.
    • Незакоммиченные транзакции потеряны.
    • Запросы в полёте получили обрыв вместо ответа.
    • Логи не дописались.
    • Kafka-консьюмер не закоммитил offset — перечитает с начала.

  ВАЖНО: некоторые современные shell'и умеют пробрасывать
  сигналы, но это НЕ гарантировано. Busybox ash (alpine) —
  в некоторых случаях. Dash (ubuntu) — нет. Полагаться нельзя.

  7. PID 1 PROBLEM И UNIX SIGNALS
  В Linux у PID 1 особые обязанности. Это не «просто первый
  процесс» — у него две работы:

  7.1. ОБРАБОТКА СИГНАЛОВ.
  Ядро относится к PID 1 по-особому. Некоторые сигналы,
  которые для обычных процессов работают, для PID 1
  могут игнорироваться по умолчанию. Если PID 1 = shell,
  он получает сигнал, но не пробрасывает.

  7.2. REAPING ЗОМБИ-ПРОЦЕССОВ.
  Когда дочерний процесс завершается, родитель должен
  вызвать wait() и «собрать» статус. Если не вызовет —
  процесс остаётся зомби (defunct). Для PID 1 это обязанность:
  собирать всех осиротевших детей.

  Для Go-сервиса без subprocess'ов — reaping не нужен.
  Если Go-сервис запускает команды через os/exec — он
  сам вызывает cmd.Wait(), так что зомби не копятся.

  UNIX SIGNALS — ЧТО НУЖНО ЗНАТЬ:
    SIGTERM (15) — «вежливая просьба завершиться». Можно
                   перехватить, обработать, отложить.
                   Docker посылает его первым.

    SIGKILL (9)  — «немедленно умереть». НЕЛЬЗЯ перехватить.
                   Docker посылает после stop_grace_period.

    SIGINT (2)   — Ctrl+C. Тоже «вежливая просьба».

    SIGHUP (1)   — «терминал отключился». Для reload конфига.

    SIGQUIT (3)  — «выход с core dump». Для graceful shutdown
                   в nginx и некоторых сервисах.

  В GO:
    • signal.Notify позволяет ловить сигналы.
    • signal.NotifyContext — обёртка, отменяет context.
    • SIGKILL нельзя поймать — можно только не доводить.

  8. TINI И DUMB-INIT
  Tini и dumb-init — минимальные init-процессы. Задача:
    • Стать PID 1.
    • Запустить твой сервис.
    • Слушать сигналы и пробрасывать их сервису.
    • Собирать зомби.

  КОГДА НУЖНЫ:
    • ENTRYPOINT — bash-скрипт, который делает setup и запускает
      сервис. Скрипт не пробрасывает сигналы.
    • Python/Node.js-сервис с subprocess'ами.
    • Любой сервис, который сам не умеет быть PID 1.

  КОГДА НЕ НУЖНЫ:
    • Go-сервис, который сам ловит SIGTERM.
    • Любой бинарник, умеющий быть PID 1.

  TINI В DOCKER:
    FROM alpine:3.19
    RUN apk add --no-cache tini
    COPY --from=builder /app/server /app/server
    ENTRYPOINT ["/sbin/tini", "--", "/app/server"]

  Что происходит: PID 1 = tini, tini запускает сервис, слушает
  SIGTERM и пересылает его, потом выходит.

  DUMB-INIT — то же самое, чуть больше фич. Для Go — не нужны.

  9. STOPSIGNAL И STOP_GRACE_PERIOD
  STOPSIGNAL в Dockerfile меняет сигнал, который Docker посылает
  при stop.

    STOPSIGNAL SIGTERM    ← дефолт
    STOPSIGNAL SIGQUIT    ← nginx, исторически
    STOPSIGNAL SIGINT     ← некоторые сервисы

  Для Go — дефолт (SIGTERM). Go-сервис ловит его через
  signal.NotifyContext.

  STOP_GRACE_PERIOD — время между SIGTERM и SIGKILL. По
  умолчанию 10 секунд.

    docker stop -t 30 my-app
    docker run --stop-timeout 30 my-app

  В docker-compose:
    services:
      app:
        stop_grace_period: 30s

  ТИПИЧНЫЕ ЗНАЧЕНИЯ:
    • 10s — дефолт, для быстрых сервисов.
    • 30s — если есть активные запросы с долгими таймаутами.
    • 60s+ — для batch-обработчиков.

  Если graceful shutdown дольше grace_period — SIGKILL убьёт
  сервис на середине. Увеличивай.

  10. ENTRYPOINT В DISTROLESS И SCRATCH
  В distroless и scratch НЕТ /bin/sh. Shell-форма не работает:
    ENTRYPOINT /app/server
    # exec: "/bin/sh": stat /bin/sh: no such file or directory

  Только exec-форма:
    ENTRYPOINT ["/app/server"]

  Это ещё одна причина использовать exec-форму — она
  универсальна.

  HEALTHCHECK в distroless — та же проблема. Нет curl/wget.
  Решение — встроенная подкоманда в Go-бинарник:

    HEALTHCHECK CMD ["/app/server", "healthcheck"]

  Полный пример:
    FROM gcr.io/distroless/static-debian12:nonroot
    COPY --from=builder /app/server /server
    USER nonroot:nonroot
    HEALTHCHECK --interval=30s --timeout=3s \
      CMD ["/server", "healthcheck"]
    ENTRYPOINT ["/server"]

  11. ENTRYPOINT В KUBERNETES И DOCKER-COMPOSE
  KUBERNETES: те же понятия, другие имена.

    Docker          Kubernetes
    ENTRYPOINT  →   command
    CMD         →   args

  В YAML:
    spec:
      containers:
      - name: app
        image: my-app:1.0
        command: ["/app/server"]     # ENTRYPOINT
        args: ["--port", "8080"]     # CMD

  ВАЖНО: Kubernetes посылает SIGTERM при удалении пода, потом
  ждёт terminationGracePeriodSeconds (по умолчанию 30s), потом
  SIGKILL.

    spec:
      terminationGracePeriodSeconds: 60

  DOCKER-COMPOSE: можно переопределить для каждого сервиса.
    services:
      app:
        image: my-app:1.0
        entrypoint: ["/app/server"]
        command: ["--port", "8080"]

  КЛАССИЧЕСКИЙ ПАТТЕРН — ОДИН ОБРАЗ, РАЗНЫЕ ЗАДАЧИ:
    services:
      app:
        entrypoint: ["/app"]
        command: ["server"]
      worker:
        entrypoint: ["/app"]
        command: ["worker"]
      migrate:
        entrypoint: ["/app"]
        command: ["migrate", "up"]

  В GO — подкоманды:
    switch os.Args[1] {
    case "server":  runServer()
    case "worker":  runWorker()
    case "migrate": runMigrations()
    }

  12. ЧТО ИСПОЛЬЗОВАТЬ В GO-СЕРВИСЕ

  Правильный шаблон:
    ENTRYPOINT ["/app/server"]

  Или, если нужны дефолтные аргументы:
    ENTRYPOINT ["/app/server"]
    CMD ["--config", "/etc/app/config.yaml"]

  ПОЧЕМУ ENTRYPOINT, А НЕ CMD:
    • Контейнер предназначен для одной задачи — запуск сервиса.
    • Случайный аргумент из docker run не должен ломать сервис.
    • Семантически «это и есть контейнер».

  ПОЧЕМУ EXEC-ФОРМА:
    • SIGTERM приходит в Go-сервис напрямую.
    • Graceful shutdown работает.
    • Go-сервис сам становится PID 1.
    • Никаких tini/dumb-init не нужно.

  ПОЛНЫЙ DOCKERFILE:
    FROM gcr.io/distroless/static-debian12:nonroot
    COPY --from=builder /app/server /server
    USER nonroot:nonroot
    EXPOSE 8080
    ENTRYPOINT ["/server"]

  ПОЛНЫЙ GO-КОД:
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGTERM, syscall.SIGINT)
    defer cancel()

    go srv.ListenAndServe()

    <-ctx.Done()
    shutdownCtx, cancelShutdown := context.WithTimeout(
        context.Background(), 20*time.Second)
    defer cancelShutdown()
    srv.Shutdown(shutdownCtx)

  Всё вместе: docker stop → SIGTERM → сервис ловит → дожидается
  запросов → закрывает соединения → exit 0. Чисто.

  13. КАК ОТЛАЖИВАТЬ ПРОБЛЕМЫ С СИГНАЛАМИ
  Симптом: `docker stop` висит 10 секунд, потом контейнер умирает.
  В логах нет «shutting down».

  13.1. ПРОВЕРЬ, КТО PID 1.
    docker exec <id> ps aux
    # или если нет ps:
    docker exec <id> cat /proc/1/cmdline | tr '\0' ' '

  Если PID 1 = /bin/sh — shell-форма. Перепиши на exec.
  13.2. ПРОВЕРЬ, ЛОВИТ ЛИ GO SIGTERM.

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        sig := <-sigCh
        log.Printf("received signal: %v", sig)
    }()

  Запусти, сделай `docker stop` — увидишь в логах «received signal».

  13.3. ПРОВЕРЬ STOPSIGNAL.
    docker inspect <id> | grep StopSignal

  Если там не SIGTERM — поправь в Dockerfile.

  13.4. УВЕЛИЧЬ STOP_TIMEOUT ДЛЯ ОТЛАДКИ.
    docker stop -t 60 <id>

  Если с 60 секундами сервис всё равно умирает жёстко — сигнал
  не доходит. Если умирает за 5 секунд — graceful shutdown
  работает, просто не укладывался в 10 секунд.

  13.5. STRACE (ЕСЛИ ЕСТЬ В ОБРАЗЕ).
    docker exec <id> strace -p 1

  Увидишь, какие сигналы приходят. В distroless strace нет —
  временно пересобери на alpine для отладки.

  КЛАССИЧЕСКИЙ КЕЙС:
    • Dockerfile: ENTRYPOINT /app/server (shell-форма).
    • docker stop висит 10 секунд.
    • В логах нет graceful shutdown.
    • ps aux: PID 1 = /bin/sh.
    • Fix: ENTRYPOINT ["/app/server"].
    • Теперь stop завершается за 1-2 секунды с логами.

  14. АНТИПАТТЕРНЫ

  14.1. SHELL-ФОРМА ENTRYPOINT ДЛЯ СЕРВИСА.
    ENTRYPOINT /app/server — sh становится PID 1, SIGTERM
    теряется. Используй exec-форму.
  14.2. CMD КАК ЕДИНСТВЕННАЯ ИНСТРУКЦИЯ ДЛЯ СЕРВИСА.
    Легко случайно переопределить аргументом. Лучше ENTRYPOINT.
  14.3. ДВА CMD ИЛИ ДВА ENTRYPOINT.
    Второй перезаписывает первый. Только один каждого.
  14.4. SHELL-ФОРМА В CMD.
    CMD /app/server — тоже shell-форма. Используй ["..."].
  14.5. `sh -c` КАК ENTRYPOINT ДЛЯ СЕРВИСА.
    ENTRYPOINT ["sh", "-c", "/app/server"] — снова sh в PID 1,
    SIGTERM не доходит.
  14.6. SHELL-ФОРМА В DISTROLESS/SCRATCH.
    Не работает — нет /bin/sh. Только exec-форма.
  14.7. ЗАБЫЛИ GRACEFUL SHUTDOWN В КОДЕ.
    Даже с exec-формой, если Go-сервис не ловит SIGTERM —
    умрёт молча через SIGKILL.
  14.8. СЛОЖНЫЙ BASH-СКРИПТ КАК ENTRYPOINT БЕЗ TINI.
    Bash станет PID 1, не пробросит сигналы. Нужен trap
    или tini.
  14.9. МАЛЕНЬКИЙ STOP_GRACE_PERIOD ДЛЯ ДОЛГОГО SHUTDOWN.
    Если shutdown занимает 30 секунд, а grace_period — 10,
    SIGKILL убьёт на середине.
  14.10. НЕ ЛОГИРОВАТЬ SHUTDOWN В КОДЕ.
    Без логов «shutting down» ты не знаешь, сработал ли
    graceful shutdown или SIGKILL.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  CMD — дефолтные аргументы. Легко переопределить.
      ENTRYPOINT — сама команда. Аргументы приклеиваются.
  2.  Комбинация ENTRYPOINT + CMD = «команда + дефолтные
      аргументы». Классика для CLI-утилит.
  3.  Exec-форма (`["bin"]`) — прямой запуск процесса,
      PID 1 = твой процесс. Shell-форма (`bin arg`) —
      обёртка через /bin/sh, PID 1 = sh.
  4.  Для Go-сервисов — ВСЕГДА exec-форма. Иначе SIGTERM
      не доходит, graceful shutdown ломается.
  5.  Симптом shell-формы: при `docker stop` контейнер
      висит 10 секунд и умирает от SIGKILL, в логах нет
      «shutting down».
  6.  Диагностика: `docker exec <id> ps aux` — если PID 1
      не твой сервис, а sh — ты нашёл проблему.
  7.  Для Go-сервиса — ENTRYPOINT в exec-форме:
        ENTRYPOINT ["/app/server"]
  8.  CMD опционально для дефолтных аргументов.
  9.  Переопределение:
        • CMD — аргументами в docker run.
        • ENTRYPOINT — через флаг --entrypoint.
  10. PID 1 problem: shell не пересылает сигналы, не собирает
      зомби. Для Go — не критично, если не запускаешь
      subprocess'ов. Важно, чтобы SIGTERM доходил.
  11. Tini и dumb-init — только для скриптов и языков, которые
      не умеют быть PID 1. Для Go — не нужны.
  12. В distroless/scratch — только exec-форма. Shell
      отсутствует.
  13. STOPSIGNAL меняет сигнал остановки (обычно SIGTERM).
      STOP_GRACE_PERIOD — время между SIGTERM и SIGKILL.
  14. В Kubernetes: ENTRYPOINT → command, CMD → args.
      terminationGracePeriodSeconds — аналог stop_grace_period.
  15. В docker-compose: entrypoint и command можно
      переопределить для каждого сервиса. Один образ —
      разные задачи (server, worker, migrate).
  16. Проверить всё правильно:
        docker exec <id> ps aux
        # PID 1 = твой сервис
        docker stop <id>
        # Завершение быстро, в логах — graceful shutdown.
*/
