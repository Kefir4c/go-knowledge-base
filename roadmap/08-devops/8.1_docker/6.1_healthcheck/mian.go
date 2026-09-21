package main

/*
  УРОК 6.1: HEALTHCHECK И ЕГО ОСОБЕННОСТИ ДЛЯ DISTROLESS
  Docker-контейнер может быть в статусе running, но при этом
  приложение внутри уже мертво: deadlock, зациклилось, потеряло
  соединение с БД. Процесс формально жив, но отвечать не может.
  Без healthcheck оркестратор не знает об этом. Kubernetes
  продолжает слать трафик в мёртвый контейнер. depends_on
  запускает следующий сервис раньше, чем предыдущий готов.
  Load balancer отправляет запросы в пустоту.
  Healthcheck — механизм, через который Docker сам проверяет,
  что сервис реально готов. И тут начинается проблема: в
  distroless и scratch нет curl, wget, sh. Классический
  healthcheck не работает. Нужны обходные решения.

  СОДЕРЖАНИЕ:
    1.  Проблема: running ≠ healthy
    2.  Что такое healthcheck
    3.  Три состояния контейнера
    4.  Параметры healthcheck
    5.  Форматы test: CMD и CMD-SHELL
    6.  Готовые healthcheck для популярных образов
    7.  Проблема distroless и scratch
    8.  Решение 1: подкоманда healthcheck в Go-бинарнике
    9.  Решение 2: healthcheck в оркестраторе (k8s)
    10. Решение 3: TCP-проверка через /dev/tcp
    11. Healthcheck в Dockerfile vs в compose
    12. depends_on и condition: service_healthy
    13. Антипаттерны
    14. Финальные выводы

  1. ПРОБЛЕМА: RUNNING ≠ HEALTHY
  Контейнер может быть в статусе Up, но приложение внутри
  уже не работает.

  Типичные сценарии:
    • Deadlock. Горутины ждут друг друга, процесс жив,
      но не отвечает.
    • Потеря соединения с БД. Приложение стартовало, но
      Postgres перезапустился и соединение порвалось.
    • Утечка памяти. Процесс на грани OOM, отвечает
      медленно.
    • Бесконечный цикл. CPU съеден, HTTP-сервер не успевает
      обрабатывать запросы.
    • Миграции не применились. Сервис стартовал, но
      таблиц нет.

  Что видит Docker:

    docker compose ps
    # STATUS: Up 5 minutes

  Что видит пользователь:

    curl http://localhost:8080/
    # 500 Internal Server Error
    # или
    # timeout

  Контейнер формально работает, но фактически — мёртв.
  Оркестратор не знает об этом и не перезапускает.
  РЕШЕНИЕ: healthcheck.

  2. ЧТО ТАКОЕ HEALTHCHECK
  Healthcheck — команда, которую Docker выполняет периодически
  внутри контейнера. Если команда возвращает 0 — healthy.
  Если не 0 — unhealthy.

  ТРИ МЕСТА, ГДЕ ЗАДАЁТСЯ:

    1. В Dockerfile:
       HEALTHCHECK CMD curl -f http://localhost:8080/health

    2. В compose.yaml:
       services:
         app:
           healthcheck:
             test: ["CMD", "curl", "-f", "http://localhost:8080/health"]

    3. При docker run:
       docker run --health-cmd="curl -f http://localhost:8080/health" ...

  Приоритет: compose > Dockerfile > run. Значения мержатся,
  compose переопределяет.

  ЧТО ЭТО ДАЁТ:
    • Статус в `docker compose ps`: healthy / unhealthy.
    • `depends_on` с `condition: service_healthy` работает.
    • Load balancer (Traefik, nginx) может исключать unhealthy.
    • Мониторинг видит проблему раньше, чем пользователи.

  3. ТРИ СОСТОЯНИЯ КОНТЕЙНЕРА
  У healthcheck три состояния:

    STARTING  — до первого успешного чека или истечения
                start_period. Провалы не считаются.

    HEALTHY   — команда вернула 0.

    UNHEALTHY — retries провалов подряд.

  СХЕМА ПЕРЕХОДОВ:
    [STARTING]
       │
       ├─ успех ─────► [HEALTHY]
       │
       └─ start_period истёк ─► проверка считается с этого момента

    [HEALTHY]
       │
       └─ retries провалов ─► [UNHEALTHY]

    [UNHEALTHY]
       │
       └─ успех ──────────► [HEALTHY]

  ЧТО ПРОИСХОДИТ ПРИ UNHEALTHY:
    Docker НЕ перезапускает контейнер автоматически.
    Docker только помечает статус.

    Перезапуск — задача оркестратора:
      • Kubernetes: liveness probe → рестарт пода.
      • Docker Compose: ничего не делает автоматически.
        Нужен restart: unless-stopped + ручной триггер.

  ВАЖНОЕ РАЗЛИЧИЕ:
    В Docker / Compose:
      healthcheck только помечает статус.

    В Kubernetes:
      liveness probe → kubelet рестартует контейнер.
      readiness probe → убирает из балансировки.

  В `docker compose ps` видно:
    NAME      STATUS
    app-1     Up 5 minutes (healthy)
    app-2     Up 3 minutes (unhealthy)
    app-3     Up 10 seconds (health: starting)

  4. ПАРАМЕТРЫ HEALTHCHECK

  ЧЕТЫРЕ ОСНОВНЫХ ПАРАМЕТРА:

    test:          команда проверки.
    interval:      как часто проверять. Дефолт 30s.
    timeout:       сколько ждать ответа. Дефолт 30s.
    retries:       сколько провалов до unhealthy. Дефолт 3.
    start_period:  грейс при старте. Дефолт 0.

  ПРИМЕР В COMPOSE:
    services:
      app:
        healthcheck:
          test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
          interval: 10s
          timeout: 3s
          retries: 3
          start_period: 15s

  ЧТО ЗНАЧИТ КАЖДЫЙ:
    interval: 10s
      Проверка запускается каждые 10 секунд.

    timeout: 3s
      Если команда не ответила за 3 секунды — провал.

    retries: 3
      Три провала подряд — переход в unhealthy.

    start_period: 15s
      Первые 15 секунд провалы не считаются.
      Полезно для медленно стартующих приложений.

  ТИПИЧНЫЕ ЗНАЧЕНИЯ:
    Быстрые сервисы (nginx, redis):
      interval: 10s, timeout: 3s, retries: 3

    БД (postgres, mysql):
      interval: 5s, timeout: 3s, retries: 10, start_period: 15s

    Go-сервисы с миграциями:
      interval: 10s, timeout: 5s, retries: 5, start_period: 30s

  ВАЖНО ПРО START_PERIOD:
    Без start_period, если приложение стартует 20 секунд,
    а interval 10s — за это время накопится 2 провала.
    С retries: 3 — контейнер станет unhealthy до того,
    как вообще запустится.

    start_period даёт грейс. Провалы в течение него не
    считаются.

  5. ФОРМАТЫ TEST: CMD И CMD-SHELL
  У test две формы записи.

  CMD (exec-форма):
    test: ["CMD", "curl", "-f", "http://localhost:8080/health"]

    Docker запускает процесс напрямую. Без shell. Быстрее.
    Если файла нет — ошибка сразу, без сообщения от shell.

  CMD-SHELL (shell-форма):
    test: ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]

    Docker запускает через /bin/sh -c. Позволяет использовать
    pipe, &&, ||, переменные.

  КОГДА ЧТО:
    CMD:
      • Одна команда без сложной логики.
      • Есть бинарник в PATH.
      • Пример: ["CMD", "pg_isready", "-U", "app"]

    CMD-SHELL:
      • Несколько команд с || или &&.
      • Нужны переменные окружения.
      • Пример: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]

  УСТАРЕВШАЯ СТРОЧНАЯ ФОРМА:
    test: curl -f http://localhost:8080/health

    Трактуется как CMD-SHELL. Работает, но менее явно.
    Лучше писать массив.

  6. ГОТОВЫЕ HEALTHCHECK ДЛЯ ПОПУЛЯРНЫХ ОБРАЗОВ
  PG_ISREADY (postgres):
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 15s

    pg_isready встроен в postgres-образ. Возвращает 0,
    когда БД принимает подключения.

  REDIS-CLI PING (redis):
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

    Если redis с паролем:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]

  MYSQLADMIN (mysql / mariadb):
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 30s

  MONGOSH (mongo):
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 5s
      timeout: 3s
      retries: 5

  KAFKA-TOPICS (kafka):
    healthcheck:
      test: ["CMD-SHELL", "kafka-topics.sh --bootstrap-server localhost:9092 --list"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s

  WGET (nginx, alpine-сервисы с shell):
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "-", "http://localhost/health"]
      interval: 10s
      timeout: 3s
      retries: 3

  CURL (ubuntu, debian, alpine + curl):
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 3

    Флаг -f — упасть на HTTP-ошибке (4xx, 5xx).
    Флаг -sS — тихо, но с ошибками.

  7. ПРОБЛЕМА DISTROLESS И SCRATCH
  Distroless и scratch не содержат ни одного из этих бинарников.

  ЧТО ЕСТЬ В DISTROLESS/STATIC:
    • Твой бинарник.
    • ca-certificates (для HTTPS).
    • tzdata.
    • /etc/passwd для nonroot.
    • Больше ничего.

  ЧЕГО НЕТ:
    • curl.
    • wget.
    • sh, bash.
    • nc.
    • Любых утилит.

  ЧТО ПРОИСХОДИТ ПРИ КЛАССИЧЕСКОМ HEALTHCHECK:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]

    При запуске контейнера:

      OCI runtime exec failed: exec: "curl": executable file
      not found in $PATH

  Healthcheck не работает. Контейнер становится unhealthy
  или вообще не стартует.

  ЧТО ДЕЛАТЬ:
    Три решения (следующие разделы):
      1. Подкоманда healthcheck в Go-бинарнике.
      2. Healthcheck в оркестраторе (k8s probe).
      3. TCP-проверка через /dev/tcp (только если есть bash).

  8. РЕШЕНИЕ 1: ПОДКОМАНДА HEALTHCHECK В GO-БИНАРНИКЕ
  Самый чистый способ для дистрибутивов без shell. Твой
  бинарник сам умеет проверять себя.

  ИДЕЯ:
    app                 → запускает сервер.
    app healthcheck     → делает HTTP-запрос к /health,
                          возвращает 0 или 1.

  В GO:
    func main() {
        if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
            if err := doHealthcheck(); err != nil {
                fmt.Fprintln(os.Stderr, err)
                os.Exit(1)
            }
            os.Exit(0)
        }

        // основной сервер.
        runServer()
    }

    func doHealthcheck() error {
        ctx, cancel := context.WithTimeout(
            context.Background(), 2*time.Second)
        defer cancel()

        req, _ := http.NewRequestWithContext(ctx,
            http.MethodGet, "http://localhost:8080/health", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return err
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("status: %d", resp.StatusCode)
        }
        return nil
    }

  В COMPOSE:
    services:
      app:
        image: my-app:1.0
        healthcheck:
          test: ["CMD", "/app", "healthcheck"]
          interval: 10s
          timeout: 3s
          retries: 3
          start_period: 5s

  В DOCKERFILE:
    HEALTHCHECK --interval=10s --timeout=3s --start-period=5s \
      --retries=3 \
      CMD ["/app", "healthcheck"]

  ЧТО ЭТО ДАЁТ:
    • Работает на distroless и scratch.
    • Не нужны curl, wget, sh.
    • Точная проверка: бинарник знает, что значит «готов».
    • Может проверять не только HTTP, но и БД, кэш, всё что
      угодно.

  ВАРИАЦИИ:

    Healthcheck с проверкой зависимостей:
      func doHealthcheck() error {
          // Проверка HTTP-сервера.
          if err := checkHTTP(); err != nil {
              return err
          }
          // Проверка БД.
          if err := checkDB(); err != nil {
              return err
          }
          return nil
      }

    Healthcheck с логированием:
      func doHealthcheck() error {
          start := time.Now()
          defer func() {
              log.Printf("healthcheck took %v", time.Since(start))
          }()
          return checkHTTP()
      }

  ПРАВИЛА:
    • Один бинарник — разные роли через аргументы.
    • healthcheck не должен занимать больше пары секунд.
    • Возвращает 0 при успехе, != 0 при провале.
    • В stderr пишет причину — Docker её покажет.

  9. РЕШЕНИЕ 2: HEALTHCHECK В ОРКЕСТРАТОРЕ (K8S)
  В Kubernetes healthcheck переносится из Docker на уровень
  пода. kubelet сам делает HTTP-запросы.

  LIVENESS PROBE:

    spec:
      containers:
      - name: app
        image: my-app:1.0
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3

    Если /health не отвечает — kubelet рестартует контейнер.

  READINESS PROBE:
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 3
      periodSeconds: 5

    Если /ready не отвечает — под убирается из балансировки,
    но не рестартуется.

  STARTUP PROBE:
    startupProbe:
      httpGet:
        path: /health
        port: 8080
      failureThreshold: 30
      periodSeconds: 5

    Для медленно стартующих приложений. Пока не пройдёт —
    liveness и readiness не проверяются.

  ЧТО ЭТО ДАЁТ:
    • Healthcheck работает на уровне kubelet.
    • Не нужны curl, wget, sh в образе.
    • Разделение liveness (жив) и readiness (готов).
    • Более гибкая настройка, чем Docker healthcheck.

  ВАЖНО:
    В K8s Docker healthcheck из Dockerfile/compose ИГНОРИРУЕТСЯ.
    kubelet использует только probes из манифеста.

  ПРАКТИКА:
    В distroless-образе не задавай HEALTHCHECK вообще.
    Полагайся на k8s probes. Docker в этом случае ничего
    не проверяет — но в k8s всё работает.

  10. РЕШЕНИЕ 3: TCP-ПРОВЕРКА ЧЕРЕЗ /DEV/TCP
  Если в образе есть bash — можно использовать встроенную
  функцию /dev/tcp.

    healthcheck:
      test: ["CMD-SHELL", "exec 3<>/dev/tcp/127.0.0.1/8080"]
      interval: 10s
      timeout: 3s
      retries: 3

  ЧТО ПРОИСХОДИТ:
    bash пытается открыть TCP-соединение с 127.0.0.1:8080.
    Если порт слушается — exit 0.
    Если нет — exit 1.

  КОГДА РАБОТАЕТ:
    • Только если есть bash (не sh).
    • Alpine по умолчанию — sh (busybox). Не работает.
    • Нужно поставить bash: apk add bash.

  НЕ РАБОТАЕТ НА:
    • distroless.
    • scratch.
    • alpine без bash.

  ПРИМЕР ДЛЯ ALPINE С BASH:
    FROM alpine:3.19

    RUN apk add --no-cache bash

    HEALTHCHECK --interval=10s --timeout=3s \
      CMD bash -c "exec 3<>/dev/tcp/127.0.0.1/8080"

  ОГРАНИЧЕНИЯ:
    • Проверяет только «порт слушается», не «сервис отвечает».
    • Приложение может слушать порт, но не отвечать на запросы
      (deadlock) — TCP-проверка это не поймает.

  ВЫВОД: это компромисс, не оптимальное решение. Лучше
  использовать подкоманду в бинарнике.

  11. HEALTHCHECK В DOCKERFILE VS В COMPOSE
  Можно задавать в двух местах. Что выбрать?

  В DOCKERFILE:
    FROM alpine:3.19
    ...
    HEALTHCHECK --interval=30s --timeout=3s \
      CMD wget -q -O - http://localhost:8080/health

  Плюсы:
    • Часть образа, работает всегда.
    • В docker run / docker-compose / k8s.
    • Документирует, как проверять сервис.

  Минусы:
    • Требует пересборки образа для изменения.
    • Жёсткие значения, нельзя адаптировать под окружение.

  В COMPOSE:
    services:
      app:
        healthcheck:
          test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
          interval: 10s
          timeout: 3s
          retries: 3

  Плюсы:
    • Можно адаптировать под окружение.
    • Не требует пересборки.
    • Меняется через override-файлы.

  Минусы:
    • Не работает при docker run без compose.
    • Может расходиться с Dockerfile.

  ПРИОРИТЕТ:
    compose переопределяет Dockerfile. Если в Dockerfile
    HEALTHCHECK, а в compose свой — работает compose.

  КОГДА ЧТО:
    В Dockerfile:
      • Базовое значение по умолчанию.
      • Образ публикуется в registry, healthcheck — часть
        контракта.

    В compose:
      • Адаптация под конкретное окружение.
      • Debug-значения в dev (interval=5s, retries=10).
      • Прод-значения в проде (interval=30s, retries=3).

  РЕКОМЕНДАЦИЯ:
    Простой дефолт в Dockerfile. Точная настройка в compose
    через override-файлы для разных окружений.

  12. DEPENDS_ON И CONDITION: SERVICE_HEALTHY
  Зачем нужен healthcheck в Compose? Чтобы depends_on знал,
  когда сервис реально готов.

  БЕЗ HEALTHCHECK:
    services:
      postgres:
        image: postgres:16-alpine

      app:
        depends_on:
          - postgres

    Что происходит:
      Docker запускает postgres.
      Docker сразу запускает app.
      App пытается подключиться — БД ещё стартует.
      App падает с "connection refused".

  С HEALTHCHECK:
    services:
      postgres:
        image: postgres:16-alpine
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U app"]
          interval: 5s
          retries: 10

      app:
        depends_on:
          postgres:
            condition: service_healthy

    Что происходит:
      Docker запускает postgres.
      Docker ждёт healthcheck postgres.
      Postgres становится healthy.
      Docker запускает app.
      App успешно подключается.

  УСЛОВИЯ DEPENDS_ON:
    service_started — контейнер запущен (дефолт).
    service_healthy — healthcheck прошёл.
    service_completed_successfully — завершился с кодом 0
                                      (для одноразовых задач).

  ПРИМЕР С МИГРАЦИЯМИ:
    services:
      migrate:
        image: my-app:1.0
        command: ["migrate", "up"]
        depends_on:
          postgres:
            condition: service_healthy
        restart: "no"

      app:
        image: my-app:1.0
        depends_on:
          migrate:
            condition: service_completed_successfully

    App стартует после того, как миграции применились.

  ВАЖНО:
    Без healthcheck у postgres условие service_healthy
    не сработает — Compose не знает, что считать «готов».

  13. АНТИПАТТЕРНЫ
  13.1. НЕТ HEALTHCHECK.
    depends_on с condition не работает. Оркестратор не
    видит проблемы.
  13.2. HEALTHCHECK = ПРОСТО PID.
    test: ["CMD", "kill", "-0", "1"] — проверяет, что
    процесс жив, но не что он отвечает.
  13.3. HEALTHCHECK КАЖДУЮ СЕКУНДУ.
    Нагрузка на сервис и Docker. Минимум — 5 секунд.
  13.4. TIMEOUT БОЛЬШЕ INTERVAL.
    Если timeout=60s, а interval=10s — проверки накапливаются,
    Docker запускает их параллельно.
  13.5. БЕЗ START_PERIOD ДЛЯ МЕДЛЕННЫХ СЕРВИСОВ.
    Приложение стартует 30 секунд, а retries=3 × interval=10s
    → unhealthy раньше, чем оно вообще готово.
  13.6. HEALTHCHECK ЗАВИСИТ ОТ ВНЕШНЕГО РЕСУРСА.
    Проверять БД в healthcheck — плохо. Если БД упала,
    контейнер станет unhealthy, оркестратор его
    перезапустит, но БД всё равно недоступна → цикл
    рестартов.
  13.7. CURL В DISTROLESS.
    `test: ["CMD", "curl", ...]` не работает. Нужна
    подкоманда или k8s probe.
  13.8. ПРОВЕРКА ВСЕГО В ОДНОМ HEALTHCHECK.
    HTTP + БД + Redis + Kafka в одном test. Один сбой —
    всё unhealthy. Разделяй на liveness и readiness.
  13.9. HEALTHCHECK БЕЗ TIMEOUT.
    Дефолт 30 секунд. Зависший процесс держит healthcheck
    30 секунд → нагрузка.
  13.10. HEALTHCHECK ДЛЯ ОДНОРАЗОВЫХ ЗАДАЧ.
    Migrate-контейнер с healthcheck — бессмысленно.
    Он коротко живёт.
  13.11. IGNORE В K8S.
    HEALTHCHECK из Dockerfile есть, но в k8s он игнорируется.
    Нужны probes.
  13.12. HTTP-СТАТУС БЕЗ ПРОВЕРКИ.
    `curl -s http://localhost/health` вернёт 0 даже при 500.
    Нужен флаг `-f`.

  14. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Running ≠ healthy. Контейнер может быть жив, но
      приложение внутри уже не работает.
  2.  Healthcheck — команда, которую Docker выполняет
      периодически. 0 = healthy, != 0 = unhealthy.
  3.  Три состояния: starting, healthy, unhealthy.
  4.  Параметры: test, interval, timeout, retries,
      start_period.
  5.  Две формы test: CMD (exec) и CMD-SHELL (shell).
  6.  Готовые healthcheck: pg_isready, redis-cli ping,
      mysqladmin ping, wget.
  7.  В distroless/scratch нет curl/wget/sh. Классический
      healthcheck не работает.
  8.  Решение 1: подкоманда healthcheck в Go-бинарнике.
      Рекомендуется для distroless.
  9.  Решение 2: k8s probes. Healthcheck из Dockerfile
      игнорируется, работают только probes.
  10. Решение 3: TCP через /dev/tcp. Только если есть bash.
      Редко подходит.
  11. Healthcheck в Dockerfile — дефолт. В compose —
      точная настройка под окружение.
  12. depends_on с condition: service_healthy требует
      healthcheck. Иначе не работает.
  13. Антипаттерны: нет healthcheck, проверка PID,
      interval=1s, timeout > interval, нет start_period,
      healthcheck зависит от внешнего ресурса, curl на
      distroless.
*/
