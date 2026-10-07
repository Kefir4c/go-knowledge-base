package main

/*
  УРОК 2.4: PROBES — LIVENESS, READINESS, STARTUP
  K8s должен знать три вещи о Pod'е:
    • Живой ли он вообще?
    • Готов ли принимать трафик?
    • Ещё не умер посреди старта?
  На эти три вопроса отвечают три типа probes:
    livenessProbe, readinessProbe, startupProbe.
  Без probes K8s слеп. Он думает, что Pod работает, если процесс
  не упал. Но процесс может быть живым и при этом не отвечать
  на запросы: deadlock, потеря связи с БД, бесконечный цикл.
  Probes решают это.

  СОДЕРЖАНИЕ:
    1.  Зачем нужны probes
    2.  Liveness — жив ли
    3.  Readiness — готов ли
    4.  Startup — стартовал ли
    5.  Три типа проверок: httpGet, tcpSocket, exec
    6.  Параметры: initialDelay, period, timeout, threshold
    7.  Порядок работы probes
    8.  Антипаттерн: проверять БД в liveness
    9.  Правильные /health и /ready в Go
    10. terminationGracePeriod и readiness
    11. Типичные ошибки
    12. Антипаттерны
    13. Финальные выводы

  1. ЗАЧЕМ НУЖНЫ PROBES
  Процесс может быть живым, но не работающим:
    • HTTP-сервер завис на deadlock. Порт открыт, запросы висят.
    • Приложение потеряло соединение с БД, но процесс жив.
    • Бесконечный цикл съел CPU, сервис не отвечает.
    • Приложение стартует 60 секунд, за это время K8s
      шлёт трафик в мёртвый Pod.

  K8s сам не знает об этом. Без probes он думает:
  «процесс жив → Pod здоров».

  Probes дают K8s способ проверить состояние:
    • Liveness — если не отвечает, перезапусти.
    • Readiness — если не готов, убери из Service.
    • Startup — если стартует, не проверяй liveness/readiness.

  2. LIVENESS — ЖИВ ЛИ
  livenessProbe проверяет: «Pod ещё работает?».

  ЧТО ДЕЛАЕТ ПРИ ПРОВАЛЕ:
    Kubelet перезапускает контейнер.
    Счётчик RESTARTS растёт.
    Pod остаётся, но контейнер внутри новый.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Процесс может зависнуть (deadlock).
    • Приложение может застрять в бесконечном цикле.
    • Нужно автоматическое восстановление.

  ПРИМЕР:
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
      failureThreshold: 3
      timeoutSeconds: 3

  ПРОВЕРКА ИДЕАЛЬНОГО /health:
    • Простой ответ «ok».
    • НЕ проверяет БД, Redis, Kafka.
    • НЕ проверяет внешние API.
    • Только «жив ли мой процесс».

  ПОЧЕМУ НЕ ПРОВЕРЯТЬ БД:
    Если БД упала, liveness начнёт падать во всех Pod'ах.
    Kubelet будет их перезапускать. БД от этого не поднимется.
    Получишь каскадные рестарты и потерю всего стейта.

  3. READINESS — ГОТОВ ЛИ
  readinessProbe проверяет: «Pod готов принимать трафик?».

  ЧТО ДЕЛАЕТ ПРИ ПРОВАЛЕ:
    Pod убирается из Endpoints Service.
    Трафик на него не идёт.
    Pod НЕ перезапускается.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Сервис стартует — не готов принимать трафик.
    • Потерял соединение с БД — не может обрабатывать запросы.
    • Прогревает кэш — ещё не готов.
    • Graceful shutdown — сразу убрать из балансировки.

  ПРИМЕР:
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
      failureThreshold: 3
      timeoutSeconds: 3

  ПРОВЕРКА ПРАВИЛЬНОГО /ready:
    • Проверяет БД (Ping).
    • Проверяет Redis (если критично).
    • Проверяет, что миграции применены.
    • Проверяет, что кэш прогрет.

  ОСОБЕННОСТЬ ПРИ SHUTDOWN:
    Если при SIGTERM сразу отдать 503 на /ready — kubelet
    уберёт Pod из Service. Трафик перестанет идти, пока
    активные запросы завершаются.

    В Go: ready.Store(false) сразу после ctx.Done().

  4. STARTUP — СТАРТОВАЛ ЛИ
  startupProbe проверяет: «Pod уже стартовал?».

  ЧТО ДЕЛАЕТ:
    Пока startupProbe не пройден — liveness и readiness НЕ ПРОВЕРЯЮТСЯ.
    Даёт приложению время на старт.

  КОГДА ИСПОЛЬЗУЕТСЯ:
    • Приложение стартует долго (30+ секунд).
    • Загрузка данных в память.
    • Прогрев кэша.
    • Миграции при старте (если в коде).

  ПРИМЕР:
    startupProbe:
      httpGet:
        path: /health
        port: 8080
      failureThreshold: 30
      periodSeconds: 2

  Что значит: приложение может стартовать до 60 секунд
  (30 × 2). За это время kubelet не будет рестартить его из-за liveness.

  ПОЧЕМУ НЕ ПРОСТО БОЛЬШОЙ initialDelaySeconds:
    Если поставить initialDelaySeconds: 60 на liveness —
    все рестарты будут ждать 60 секунд. Даже если приложение стартует за 5.
    startupProbe решает: ждём ровно столько, сколько нужно.
    Как только стартовал — сразу переходим к обычным probes.

  ИДЕАЛЬНАЯ КОМБИНАЦИЯ:
    startupProbe:   /health, failure 30, period 2 → до 60s на старт
    livenessProbe:  /health, initialDelay 0, period 10 → работает после старта
    readinessProbe: /ready,  initialDelay 0, period 5  → работает после старта

  5. ТРИ ТИПА ПРОВЕРОК

  HTTPGET:
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
        httpHeaders:
        - name: Custom-Header
          value: value

    Kubelet делает HTTP GET. Успех = код 200-399.
    Для Go-сервисов — стандарт.

  TCPSOCKET:
    livenessProbe:
      tcpSocket:
        port: 5432

    Kubelet пытается открыть TCP-соединение.
    Успех = порт слушается.
    Просто, но неглубоко. Не проверяет, отвечает ли сервис.

  EXEC:
    livenessProbe:
      exec:
        command:
        - pg_isready
        - -U
        - app

    Kubelet выполняет команду в контейнере.
    Успех = exit code 0.

    Для БД, где нет HTTP. Требует бинарник внутри образа.

  ЧТО ВЫБРАТЬ:
    • HTTP-сервисы → httpGet.
    • БД → exec (pg_isready, redis-cli ping).
    • Простые сервисы → tcpSocket.
    • distroless → httpGet (не нужен shell).

  6. ПАРАМЕТРЫ
  У каждой probe есть параметры.

  initialDelaySeconds (дефолт 0):
    Сколько ждать перед первой проверкой.
    Для быстрых сервисов — 5-10 сек.
    Если есть startupProbe — можно 0.

  periodSeconds (дефолт 10):
    Как часто проверять.
    Часто (2-5 сек) — быстрая реакция, больше нагрузки.
    Редко (30-60 сек) — медленная реакция, меньше нагрузки.

  timeoutSeconds (дефолт 1):
    Сколько ждать ответа.
    Мало (1-2 сек) — быстрая реакция.
    Много (5-10 сек) — терпимее к нагрузке.

    ВАЖНО: timeoutSeconds должен быть МЕНЬШЕ periodSeconds.
    Иначе проверки будут накладываться.

  successThreshold (дефолт 1):
    Сколько успехов подряд для перехода в healthy. Обычно 1.

  failureThreshold (дефолт 3):
    Сколько провалов подряд для перехода в unhealthy.
    Мало (1) — агрессивно.
    Много (5-10) — терпимо к временным сбоям.

  ТИПИЧНЫЕ ЗНАЧЕНИЯ:

    LIVENESS:
      initialDelaySeconds: 10-30
      periodSeconds: 10-30
      timeoutSeconds: 3-5
      failureThreshold: 3-5

    READINESS:
      initialDelaySeconds: 0-5
      periodSeconds: 5-10
      timeoutSeconds: 3-5
      failureThreshold: 2-3

    STARTUP:
      initialDelaySeconds: 0
      periodSeconds: 2-5
      timeoutSeconds: 3-5
      failureThreshold: 30-60

  7. ПОРЯДОК РАБОТЫ PROBES
  ЖИЗНЕННЫЙ ЦИКЛ POD'А С PROBES:

    t=0       Pod создан, статус Pending.
    t=1s      Контейнер запущен, статус Running.
    t=1s      Начинается startupProbe.
    t=10s     Startup прошёл. Начинаются liveness и readiness.
    t=15s     Readiness прошёл. Service добавил в Endpoints.
    t=20s     Liveness проверяет каждые 10 сек.

  ЕСЛИ READINESS ПАДАЕТ:
    Service убирает Pod из Endpoints.
    Трафик не идёт.
    Pod остаётся Running.
    Когда readiness пройдёт — Pod вернётся.

  ЕСЛИ LIVENESS ПАДАЕТ:
    Kubelet перезапускает контейнер.
    Startup и readiness начинают заново.
    RESTARTS увеличивается.

  ЕСЛИ STARTUP ПАДАЕТ:
    Kubelet перезапускает контейнер.
    Никогда не доходит до liveness/readiness.

  8. АНТИПАТТЕРН: ПРОВЕРЯТЬ БД В LIVENESS
  САМАЯ ЧАСТАЯ ОШИБКА.

  ПЛОХО:
    livenessProbe:
      httpGet:
        path: /health
        port: 8080

    # /health проверяет БД.

  ЧТО ПРОИСХОДИТ:
    БД упала.
    /health падает у ВСЕХ Pod'ов.
    Kubelet перезапускает ВСЕ Pod'ы.
    БД от этого не поднялась.
    Через 10 секунд снова перезапуск.
    Каскадные рестарты.

  ПРАВИЛЬНО:

    readinessProbe:
      httpGet:
        path: /ready     # проверяет БД
        port: 8080

  ЧТО ПРОИСХОДИТ:
    БД упала.
    /ready падает у всех Pod'ов.
    Pod'ы убираются из Service (трафик не идёт).
    Pod'ы НЕ перезапускаются.
    Когда БД вернулась — /ready проходит, Pod'ы возвращаются.

  9. ПРАВИЛЬНЫЕ /HEALTH И /READY В GO
  РАЗДЕЛЕНИЕ ЛОГИКИ:
    /health  →  liveness + startup
      Простой 200 OK.
      НЕ проверяет зависимости.

    /ready   →  readiness
      Проверяет БД, Redis, кэш.
      Отдаёт 503, если что-то не работает.

  ПРИМЕР:
    var ready atomic.Bool

    func main() {
        mux := http.NewServeMux()

        // Liveness — процесс жив.
        mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("ok"))
        })

        // Readiness — готов принимать трафик.
        mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
            if !ready.Load() {
                http.Error(w, "starting", 503)
                return
            }
            ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
            defer cancel()
            if err := db.PingContext(ctx); err != nil {
                http.Error(w, "db not ready", 503)
                return
            }
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("ready"))
        })

        // Signal handling.
        ctx, cancel := signal.NotifyContext(
            context.Background(), syscall.SIGTERM)
        defer cancel()

        srv := &http.Server{Addr: ":8080", Handler: mux}

        go func() {
            ready.Store(true)
            srv.ListenAndServe()
        }()

        <-ctx.Done()
        // Сразу убираем себя из Service.
        ready.Store(false)

        // Graceful shutdown.
        shutdownCtx, cancelShutdown := context.WithTimeout(
            context.Background(), 20*time.Second)
        defer cancelShutdown()
        srv.Shutdown(shutdownCtx)
    }

  ЧТО ЗДЕСЬ ВАЖНО:
    • /health — просто ok, не трогает БД.
    • /ready — ping БД + atomic-флаг.
    • ready=false при SIGTERM — сразу убираем из Service.
    • Graceful shutdown с таймаутом меньше terminationGracePeriodSeconds.

  10. TERMINATIONGRACEPERIOD И READINESS

  ПРИ УДАЛЕНИИ POD'А:
    Шаг 1: Pod помечается Terminating.
    Шаг 2: Kubelet вызывает preStop hook (если есть).
    Шаг 3: Kubelet посылает SIGTERM контейнерам.
    Шаг 4: Ждёт terminationGracePeriodSeconds.
    Шаг 5: Если не завершился — SIGKILL.

  НО ЕСТЬ НЮАНС: SERVICE ENDPOINTS ОБНОВЛЯЮТСЯ НЕ МГНОВЕННО.

    Пока kubelet удаляет Pod, Service Controller тоже
    обновляет Endpoints. Это занимает время.
    Если приложение сразу закрывает listener — последние
    запросы придут в мёртвый Pod.

  РЕШЕНИЕ: preStop hook.
    lifecycle:
      preStop:
        exec:
          command: ["/bin/sh", "-c", "sleep 5"]

  ЧТО ПРОИСХОДИТ:
    t=0     Pod Terminating. preStop: sleep 5.
    t=0-5s  Service обновляет Endpoints. Трафик идёт в другие Pod'ы.
            Но этот Pod ещё слушает и обрабатывает запросы.
    t=5s    SIGTERM. ready=false. Graceful shutdown.
    t=25s   Если всё ок — контейнер завершился.
    t=30s   terminationGracePeriodSeconds — SIGKILL, если не успел.

  ИДЕАЛЬНАЯ КОМБИНАЦИЯ:
    • preStop: sleep 5.
    • SIGTERM handler: ready=false, потом srv.Shutdown.
    • terminationGracePeriodSeconds: 30.
    • Shutdown timeout: 20.

  11. ТИПИЧНЫЕ ОШИБКИ
  11.1. TIMEOUT > PERIOD.
    timeoutSeconds: 30, periodSeconds: 10.
    Проверки накладываются друг на друга.

  11.2. failureTHRESHOLD: 1.
    Любой временный сбой — рестарт. Один сетевой сбой — и Pod
    перезапускается. Ставь минимум 3.

  11.3. initialDelaySeconds БОЛЬШОЙ.
    60 секунд initial delay для быстрого сервиса — бесполезно
    ждём. Если стартует за 5 — теряем 55 секунд.

  11.4. PROBES БЕЗ TIMEOUT.
    Дефолт 1 секунда. Если приложение отвечает 2 — падает.

  11.5. НЕТ STARTUP ДЛЯ МЕДЛЕННЫХ.
    Сервис стартует 60 секунд. Liveness с initialDelay 10
    начинает проверять — падает. Каскадные рестарты.

  11.6. LIVENESS ПРОВЕРЯЕТ БД.
    Каскадные рестарты. БД не поднимется от перезапуска Pod'ов.

  11.7. READINESS ТОЛЬКО НА /.
    Проверяет, что сервер отвечает. Но не что зависимости живы.
    Трафик идёт в Pod, который не может обработать.

  11.8. PROBES НЕ ЛОГИРУЮТСЯ.
    Не видно, почему падает. Иногда полезно логировать.

  11.9. HTTP-СТАТУС БЕЗ 200.
    Kubelet считает успехом 200-399. 401, 403 — провал.

  11.10. ЗАБЫЛИ УБРАТЬ readiness ПРИ SHUTDOWN.
    Трафик идёт в Pod во время graceful shutdown. Часть
    запросов обрывается.

  12. АНТИПАТТЕРНЫ

  12.1. LIVENESS ПРОВЕРЯЕТ БД.
    Каскадные рестарты.

  12.2. READINESS НЕ ПРОВЕРЯЕТ ЗАВИСИМОСТИ.
    Трафик идёт в Pod, не готовый обрабатывать.

  12.3. STARTUP НЕ ЗАДАН ДЛЯ МЕДЛЕННЫХ.
    Liveness валит Pod до старта.

  12.4. failureThreshold: 1.
    Любой сетевой сбой — рестарт.

  12.5. timeout > period.
    Наложение проверок.

  12.6. ОДИН /health ДЛЯ ВСЕГО.
    Liveness и readiness на один эндпоинт. Не разделяют логику.

  12.7. PROBES ЗАВИСЯТ ОТ ВНЕШНЕГО API.
    Если Stripe недоступен — Pod перезапускается. Каскад.

  12.8. НЕТ preStop HOOK.
    SIGTERM сразу после Terminating. Последние запросы теряются.

  12.9. terminationGracePeriodSeconds МЕНЬШЕ SHUTDOWN.
    SIGKILL посреди graceful shutdown.

  12.10. ЖЁСТКАЯ ПРОВЕРКА DNS В PROBE.
    Если DNS тормозит — Pod перезапускается. Не надо.

  12.11. PROBES НЕ ЛОГИРУЮТ.
    Проблема не видна. Хотя бы ERROR level.

  12.12. ОТСУТСТВИЕ PROBES.
    K8s не знает состояние. Трафик идёт в мёртвые Pod'ы.

  13. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Probes дают K8s способ проверить состояние Pod'а.
  2.  Liveness — жив ли. Падает → рестарт.
      Readiness — готов ли. Падает → убрать из Service.
      Startup — стартовал ли. Даёт время на старт.
  3.  Liveness НЕ проверяет БД. Readiness — проверяет.
  4.  Startup для медленных сервисов. Пока не пройден —
      liveness/readiness не проверяются.
  5.  Три типа: httpGet, tcpSocket, exec.
  6.  Параметры: initialDelay, period, timeout,
      successThreshold, failureThreshold.
7.    timeout < period. failureThreshold >= 3.
  8.  При shutdown: ready=false, потом graceful shutdown.
      preStop hook даёт время на обновление Endpoints.
  9.  В Go: /health — простой ok, /ready — проверка БД + atomic-флаг.
  10. Для distroless — httpGet (не нужен shell).
  11. Антипаттерны: liveness проверяет БД, нет startup,
      failureThreshold 1, timeout > period, нет preStop.
*/
