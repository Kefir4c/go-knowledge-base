package main

/*
  УРОК 5.4: OBSERVABILITY
  Без наблюдаемости K8s — чёрный ящик. Pod'ы падают, трафик растёт, latency скачет — а ты не знаешь почему.
  Observability — это не «логи + метрики + трейсы» как список, а способность задавать вопросы к системе и получать ответы.
  Три столпа: logs (что произошло), metrics (сколько, как часто, как быстро), traces (где именно тормозит).
  Плюс events — события K8s, которые часто отвечают на вопрос «почему Pod не запустился» быстрее, чем логи.
  ВАЖНО: конкретные статусы Pod'ов (CrashLoopBackOff, OOMKilled, Pending и т.д.) и их диагностика.
  Здесь — про инструменты и сигналы: как собирать, где хранить, как использовать.

  СОДЕРЖАНИЕ:
    1.  Что такое observability и зачем она
    2.  Три столпа: logs, metrics, traces
    3.  Events — четвёртый столп K8s
    4.  Логи: kubectl logs и его флаги
    5.  Логи: --previous, --all-containers, -l
    6.  Логи: сбор через sidecar и DaemonSet
    7.  Метрики: metrics-server и kubectl top
    8.  Метрики: Prometheus и PromQL
    9.  Метрики: ServiceMonitor и PodMonitor
    10. Метрики: RED и USE методы
    11. Трейсинг: Jaeger, Tempo, OpenTelemetry
    12. Events: kubectl get events и describe
    13. Связь с Go
    14. Антипаттерны
    15. Финальные выводы

  1. ЧТО ТАКОЕ OBSERVABILITY И ЗАЧЕМ ОНА
  ПРОБЛЕМА: в монолите ты заходишь на сервер, смотришь логи в /var/log, ставишь top, разбираешься.
  В K8s так не работает: Pod'ы эфемерные, их десятки, они на разных нодах,
  они умирают и пересоздаются. Классический дебаг «зайти и посмотреть» не масштабируется.

  OBSERVABILITY — это способность понять внутреннее состояние системы по её внешним сигналам.
  Не просто «собрать логи», а иметь возможность задать вопрос: «почему latency выросла в 3 раза за последние 10 минут?» — и получить ответ.

  MONITORING vs OBSERVABILITY:
    • Monitoring — знаем заранее, что может сломаться, ставим алерты на известные метрики.
    • Observability — можем разобраться в проблеме, которую не предвидели, через исследование данных.

  ЧТО ЭТО ДАЁТ:
    • Быстрая диагностика инцидентов (минуты, а не часы).
    • Понимание, где узкое место (БД, сеть, CPU, GC).
    • Возможность откатить плохой релиз до того, как его заметят пользователи.
    • Данные для capacity planning(планирование мощностей).

  ВАЖНО: observability — не «поставить Prometheus и Grafana». Это культура:
  код должен быть инструментирован, метрики должны быть осмысленные, логи — структурированные.
  Без этого будет «дашборд, на который никто не смотрит» и «логи, в которых ничего не понятно».

  2. ТРИ СТОЛПА: LOGS, METRICS, TRACES
  LOGS — дискретные события. Что произошло, когда, с какими данными.
    • Пример: "user 123 logged in", "error: connection refused to postgres:5432".
    • Формат: структурированный JSON лучше, чем plain text.
    • Объём: высокий. Хранят 7-30 дней обычно.
    • Инструменты: Loki, Elasticsearch, CloudWatch Logs, Datadog.

  METRICS — числовые значения во времени. Сколько, как часто, как быстро.
    • Пример: http_requests_total, http_request_duration_seconds, cpu_usage_percent.
    • Формат: time series (timestamp + value + labels).
    • Объём: средний, агрегируемый. Хранят месяцы и годы.
    • Инструменты: Prometheus, VictoriaMetrics, Datadog, CloudWatch.

  TRACES — путь запроса через систему. Где именно тормозит.
    • Пример: request → API → auth → DB → cache → response, с временем на каждом шаге.
    • Формат: span'ы с trace_id, parent_span_id.
    • Объём: высокий, обычно sampling (1-10% запросов).
    • Инструменты: Jaeger, Tempo, Zipkin, OpenTelemetry.

  КАК ОНИ РАБОТАЮТ ВМЕСТЕ:
    • Метрика говорит: «latency выросла с 50ms до 500ms».
    • Трейс говорит: «500ms уходит на запрос к БД, а не на код».
    • Лог говорит: «в этот момент БД вернула ошибку connection pool exhausted».

  ПРАВИЛО: хорошая observability использует все три. Метрики для алертов, трейсы для локализации, логи для деталей.

  3. EVENTS — ЧЕТВЁРТЫЙ СТОЛП K8S
  Events — это встроенный в K8s механизм. API Server генерирует события о том, что происходит с объектами.
  Это не логи приложения, это логи самого K8s: Scheduler, kubelet, Controller Manager, CNI.

  ЧТО ПОПАДАЕТ В EVENTS:
    • FailedScheduling — Pod не размещён (нет ресурсов, taints).
    • Pulling, Pulled — kubelet тянет образ.
    • Created, Started — контейнер создан и запущен.
    • Unhealthy — probe не прошёл.
    • BackOff — контейнер падает и перезапускается.
    • OOMKilling — kubelet убил Pod по памяти.

  ПОЧЕМУ ЭТО ВАЖНО:
    • Часто events отвечают на вопрос «почему Pod не работает» быстрее, чем логи.
    • Логи приложения показывают, что происходит ВНУТРИ контейнера.
    • Events показывают, что происходит С контейнером на уровне K8s.
  ВАЖНО: events живут ограниченное время (обычно 1 час). Их надо смотреть сразу или собирать в централизованное хранилище (например, через event-exporter).

  4. ЛОГИ: KUBECTL LOGS И ЕГО ФЛАГИ

  Базовый доступ к логам:
    kubectl logs <pod> -n <ns>

  ЛОГИ ПО ИМЕНИ POD'А:
    kubectl logs app-6b9d8f7c4-x9k2p -n demo

  ЛОГИ ПО DEPLOYMENT (выберет один Pod):
    kubectl logs deployment/app -n demo

  ЛОГИ ПО LABEL (все Pod'ы):
    kubectl logs -l app=app -n demo

  ОСНОВНЫЕ ФЛАГИ:
    -f, --follow            — следить в реальном времени (как tail -f).
    --tail=100              — последние 100 строк.
    --since=1h              — за последний час.
    --since-time=RFC3339    — с конкретного времени.
    --timestamps            — добавить timestamp к каждой строке.
    -c <container>          — конкретный контейнер (если их несколько).
    --all-containers        — все контейнеры Pod'а.
    --prefix                — добавить префикс с именем Pod'а.

  ПРИМЕРЫ:
    kubectl logs -f app-6b9d8f7c4-x9k2p -n demo
    kubectl logs --tail=200 app-6b9d8f7c4-x9k2p -n demo
    kubectl logs --since=30m app-6b9d8f7c4-x9k2p -n demo
    kubectl logs --timestamps app-6b9d8f7c4-x9k2p -n demo

  ЧТО ВАЖНО: K8s собирает только stdout/stderr. Если приложение пишет в файл внутри контейнера — kubectl logs ничего не покажет.

  ПРАВИЛО: приложение должно писать логи в stdout/stderr.
  Это ответственность приложения, не K8s. Docker/K8s сам собирает stdout/stderr и хранит в /var/log/pods.

  5. ЛОГИ: --PREVIOUS, --ALL-CONTAINERS, -L

  --previous — логи ПРЕДЫДУЩЕГО контейнера:
    kubectl logs --previous app-6b9d8f7c4-x9k2p -n demo

  ЗАЧЕМ: если Pod упал и был перезапущен — обычный logs покажет логи нового контейнера. Старые логи (того, что упал) — только через --previous.
  Это критично для диагностики падений: смотришь логи последнего упавшего инстанса.

  --all-containers — все контейнеры Pod'а:
    kubectl logs --all-containers app-6b9d8f7c4-x9k2p -n demo
  ЗАЧЕМ: если в Pod'е sidecar (например, istio-proxy, log-shipper), их логи тоже нужны.

  С ПРЕФИКСОМ:
    kubectl logs -l app=app --all-containers --prefix -n demo
  ЗАЧЕМ: когда Pod'ов много, префикс показывает, из какого Pod'а и контейнера пришла строка.

  КОМБИНАЦИЯ ДЛЯ ОТЛАДКИ ПАДЕНИЙ:
    kubectl logs -l app=app --previous --tail=500 -n demo

  ЧТО ВАЖНО: логи хранятся только пока Pod жив. Удалил Pod — логи потеряны. Для долгого хранения — централизованный сбор.

  6. ЛОГИ: СБОР ЧЕРЕЗ SIDECAR И DAEMONSET

  ПРОБЛЕМА: kubectl logs работает, но не масштабируется. 100 Pod'ов, 10 нод — как собрать всё в одно место?

  РЕШЕНИЕ: централизованный сбор логов. Два паттерна.

  ПАТТЕРН 1: SIDECAR (для специфичных случаев)
    Рядом с основным контейнером — контейнер-агент (fluentbit, vector).
    Оба монтируют общий volume. Основной пишет логи в файл. Sidecar читает и отправляет в Loki/ES.
    Плюсы: полный контроль, можно парсить специфичные форматы.
    Минусы: на каждый Pod — свой sidecar. Дорого по ресурсам.

    ПРИМЕР:
      containers:
      - name: app
        volumeMounts:
        - name: logs
          mountPath: /var/log/app
      - name: log-shipper
        image: fluent/fluent-bit:latest
        volumeMounts:
        - name: logs
          mountPath: /var/log/app
          readOnly: true
      volumes:
      - name: logs
        emptyDir: {}

  ПАТТЕРН 2: DAEMONSET (рекомендуется)
    Один Pod-агент на каждой ноде. Собирает stdout/stderr всех Pod'ов на ноде из /var/log/pods.
    Плюсы: один агент на ноду, а не на Pod. Эффективно.
    Минусы: нет доступа к файлам внутри контейнеров (только stdout).

  ПРИМЕР:
    DaemonSet с fluentbit, который монтирует /var/log/pods и /var/lib/docker/containers.
    Читает логи всех контейнеров, обогащает метаданными (namespace, pod, labels), отправляет в Loki/ES.

  ВАЖНО: sidecar оправдан, если приложение пишет в файлы (не stdout) и это нельзя изменить. В остальных случаях — DaemonSet.

  ИНСТРУМЕНТЫ:
    Loki — от Grafana. Лёгкий, дешёвый, интегрирован с Grafana. Хорош для K8s.
    Elasticsearch — мощный поиск, дорогой, сложный в эксплуатации.
    Vector — быстрый агент, пишется на Rust.
    Fluentbit — легковесный, на C.

  7. МЕТРИКИ: METRICS-SERVER И KUBECTL TOP
  METRICS-SERVER — минимальный источник метрик в K8s.
  Собирает CPU и memory с kubelet каждой ноды. Отдаёт через Metrics API.

  ПРОВЕРИТЬ:
    kubectl get deployment metrics-server -n kube-system
    kubectl top nodes
    kubectl top pods -n demo

  ЧТО ПОКАЗЫВАЕТ kubectl top:
    CPU в millicores (m) — например, 50m = 0.05 CPU.
    Memory в Mi/Gi — например, 128Mi.

  ПРИМЕР:
    kubectl top pods -n demo
    # NAME          CPU(cores)   MEMORY(bytes)
    # app-abc123    45m          120Mi
    # app-def456    38m          118Mi

  ЧЕГО НЕ ДАЁТ METRICS-SERVER:
    • Истории (только текущее значение).
    • Кастомных метрик (RPS, latency).
    • Алертов.
    • Долгого хранения.

  ЗАЧЕМ НУЖЕН: HPA на CPU/memory, kubectl top для быстрой проверки, VPA.
  ВАЖНО: metrics-server не заменяет Prometheus. Это разные инструменты. Metrics-server — для K8s-нативных штук (HPA), Prometheus — для полноценного мониторинга.

  8. МЕТРИКИ: PROMETHEUS И PROMQL
  PROMETHEUS — стандарт де-факто для метрик в K8s. Написан на Go.
  Работает по pull-модели: сам ходит на /metrics эндпоинт приложений и собирает метрики.

  КАК УСТРОЕН:
    • Prometheus Server — собирает и хранит метрики.
    • Exporters — отдают метрики (node-exporter, kube-state-metrics).
    • ServiceMonitor — CRD, описывает, что скрейпить.
    • Alertmanager — обрабатывает алерты.
    • Grafana — визуализация.

  ЧЕТЫРЕ ТИПА МЕТРИК:
    Counter — счётчик, только растёт. Пример: http_requests_total.
    Gauge — значение, может расти и падать. Пример: cpu_usage_percent.
    Histogram — распределение. Пример: http_request_duration_seconds.
    Summary — похож на histogram, но проценты на клиенте. Реже используется.

  PROMQL — язык запросов:
    rate(http_requests_total[5m])             — RPS за 5 минут.
    histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))  — p95 latency.
    sum(rate(http_requests_total[5m])) by (status)  — RPS по статусам.

  ПРИМЕР КОНФИГУРАЦИИ SCRAPE:
    scrape_configs:
    - job_name: 'app'
      kubernetes_sd_configs:
      - role: pod
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true

  ВАЖНО: Prometheus — pull-модель. Приложение должно отдавать /metrics по HTTP. Для короткоживущих Job'ов — Pushgateway.

  9. МЕТРИКИ: SERVICEMONITOR И PODMONITOR
  OPERATOR PROMETHEUS (kube-prometheus-stack) добавляет CRD.
  Вместо редактирования конфига Prometheus — описываешь ServiceMonitor в YAML.

  SERVICEMONITOR:
    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
      name: app
      namespace: demo
    spec:
      selector:
        matchLabels:
          app: app
      endpoints:
      - port: http
        path: /metrics
        interval: 15s

  ЧТО ЭТО ЗНАЧИТ: Prometheus будет скрейпить все Service'ы с label app=app на порту http по пути /metrics каждые 15 секунд.

  PODMONITOR — то же, но для Pod'ов напрямую (без Service).
  ПРЕИМУЩЕСТВА:
    • Декларативно, в git.
    • Не надо редактировать конфиг Prometheus.
    • Автоматически подхватывается оператором.

  ПРАВИЛО: не редактируй prometheus.yml руками. Используй ServiceMonitor/PodMonitor. Это стандарт в K8s.

  10. МЕТРИКИ: RED И USE МЕТОДЫ
  Как выбрать, какие метрики собирать? Два подхода.

  RED (для сервисов):
    • Rate — сколько запросов в секунду.
    • Errors — сколько ошибок.
    • Duration — сколько времени занимает запрос (latency).

    Примеры метрик:
      http_requests_total{status="200"}
      http_requests_total{status="500"}
      http_request_duration_seconds_bucket

  USE (для ресурсов):
    • Utilization — насколько занят ресурс (%).
    • Saturation — насколько перегружен (очередь, wait time).
    • Errors — сколько ошибок.

    Примеры метрик:
      cpu_usage_percent
      memory_usage_bytes
      disk_io_wait_seconds
      network_errors_total

  ЧТО ЭТО ДАЁТ:
    • Не собирать всё подряд, а фокусироваться на важном.
    • Быстро понимать, где проблема: сервис тормозит (RED) или ресурс кончается (USE).

  ПРАВИЛО: для HTTP-сервисов — RED. Для нод/дисков/сетей — USE. Для Go-сервиса — оба.

  11. ТРЕЙСИНГ: JAEGER, TEMPO, OPENTELEMETRY
  ПРОБЛЕМА: запрос идёт через 5 сервисов. Latency выросла. Где именно?
  Метрики показывают «500ms», но не показывают, на каком сервисе.

  РЕШЕНИЕ: distributed tracing.
  Каждый запрос получает trace_id. Каждый шаг (span) логируется с этим trace_id.

  КАК РАБОТАЕТ:
    1. Запрос приходит в API. Генерируется trace_id.
    2. API вызывает auth-сервис. Передаёт trace_id в заголовке.
    3. auth вызывает БД. Тоже передаёт trace_id.
    4. Каждый сервис отправляет span'ы (start, end, metadata) в коллектор.
    5. Коллектор собирает всё в один trace.
    6. В UI (Jaeger/Tempo) видно весь путь с временами.

  OPEN TELEMETRY (OTel):
    Единый стандарт для metrics, logs, traces. Пришёл на смену OpenTracing + OpenCensus.
    SDK для Go, Java, Python, etc. Отправляет данные в любой backend (Jaeger, Tempo, Datadog).

  ПРИМЕР SPAN'А:
    Trace ID: abc123
    ├─ span: HTTP GET /users (200ms)
    │  ├─ span: auth.Validate (5ms)
    │  ├─ span: db.Query users (180ms)
    │  │  ├─ span: postgres.query (175ms)
    │  └─ span: json.Marshal (10ms)

  СРАЗУ ВИДНО: 180ms уходит на БД, из них 175ms — на сам запрос Postgres.

  SAMPLING:
    Хранить все трейсы дорого. Обычно sampling: 1-10% запросов.
    Head-based: решение на входе. Простое, но может пропустить редкие ошибки.
    Tail-based: решение после завершения трейса. Сложнее, но сохраняет важные трейсы (ошибки, медленные).

  ИНСТРУМЕНТЫ:
    Jaeger — от Uber. Классика.
    Tempo — от Grafana. Дёшево, интегрирован с Grafana.
    Zipkin — старый, но живой.
    Datadog APM — платно, всё в одном.

  ПРАВИЛО: трейсинг обязателен, если сервисов больше 3-4. В микросервисах без трейсинга дебаг превращается в боль.

  12. EVENTS: KUBECTL GET EVENTS И DESCRIBE
  Events — это не логи, это события K8s о состоянии объектов.

  ПОСМОТРЕТЬ ВСЕ СОБЫТИЯ В NAMESPACE:
    kubectl get events -n demo

  С СОРТИРОВКОЙ ПО ВРЕМЕНИ:
    kubectl get events -n demo --sort-by=.metadata.creationTimestamp

  ТОЛЬКО ПРЕДУПРЕЖДЕНИЯ:
    kubectl get events -n demo --field-selector type=Warning

  ПО КОНКРЕТНОМУ ОБЪЕКТУ:
    kubectl get events -n demo --field-selector involvedObject.name=app-abc123

  WATCH В РЕАЛЬНОМ ВРЕМЕНИ:
    kubectl get events -n demo -w

  DESCRIBE — ЛУЧШИЙ ДРУГ:
    kubectl describe pod app-abc123 -n demo

  В КОНЦЕ ВЫВОДА — секция Events:
    Events:
      Type     Reason            From       Message
      ----     ------            ----       -------
      Normal   Scheduled         scheduler  Successfully assigned demo/app to node-1
      Normal   Pulling           kubelet    Pulling image "my-app:1.0"
      Normal   Pulled            kubelet    Successfully pulled image
      Normal   Created           kubelet    Created container app
      Normal   Started           kubelet    Started container app
      Warning  Unhealthy         kubelet    Readiness probe failed: connection refused
      Warning  BackOff           kubelet    Back-off restarting failed container

  ЧТО ЧИТАТЬ:
    • FailedScheduling — Pod не размещён. Причина: нехватка ресурсов, taints, nodeSelector.
    • ImagePullBackOff — образ не тянется.
    • BackOff — контейнер падает и перезапускается.
    • Unhealthy — probe не проходит.
    • OOMKilling — превышен limits.memory.

  ВАЖНО: events живут ~1 час. Для истории — event-exporter в Loki/ES.

  ПОДРОБНЕЕ О КОНКРЕТНЫХ СТАТУСАХ И ИХ ДИАГНОСТИКЕ — В УРОКЕ 6.1.

  13. СВЯЗЬ С GO
  Что нужно от Go-разработчика для observability:

  1. СТРУКТУРИРОВАННЫЕ ЛОГИ.
     Не fmt.Println, а log/slog (Go 1.21+).
     JSON-формат для прода, text для dev.
     Уровни: debug, info, warn, error.

     ПРИМЕР:
       slog.Info("request handled",
           "method", r.Method,
           "path", r.URL.Path,
           "status", status,
           "duration_ms", duration.Milliseconds(),
           "trace_id", traceID,
       )

  2. МЕТРИКИ ЧЕРЕЗ PROMETHEUS CLIENT.
     Библиотека: github.com/prometheus/client_golang.
     Экспонировать /metrics эндпоинт.

     ПРИМЕР:
       var httpRequests = prometheus.NewCounterVec(
           prometheus.CounterOpts{
               Name: "http_requests_total",
               Help: "Total HTTP requests",
           },
           []string{"method", "path", "status"},
       )

       func init() {
           prometheus.MustRegister(httpRequests)
       }

       func handler(w http.ResponseWriter, r *http.Request) {
           httpRequests.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
       }

  3. ТРЕЙСИНГ ЧЕРЕЗ OPENTELEMETRY.
     Библиотека: go.opentelemetry.io/otel.
     Пробрасывать trace_id через context.
     Создавать span'ы вокруг операций (БД, HTTP-вызовы).

     ПРИМЕР:
       tracer := otel.Tracer("app")
       ctx, span := tracer.Start(ctx, "db.Query")
       defer span.End()

       rows, err := db.QueryContext(ctx, "SELECT ...")
       if err != nil {
           span.RecordError(err)
           span.SetStatus(codes.Error, err.Error())
       }

  4. HEALTH И READY ЭНДПОИНТЫ.
     /health — для liveness. Всегда 200.
     /ready — для readiness. Проверяет БД, Redis.

  5. ОБРАБОТКА SIGTERM.
     Graceful shutdown, чтобы не терять запросы при rolling update.

  6. ПРИМЕР ПОЛНОГО MAIN.GO С OBSERVABILITY:

     package main

     import (
         "context"
         "log/slog"
         "net/http"
         "os"
         "os/signal"
         "sync/atomic"
         "syscall"
         "time"

         "github.com/prometheus/client_golang/prometheus/promhttp"
     )

     var ready atomic.Bool

     func main() {
         logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
         slog.SetDefault(logger)

         ctx, cancel := signal.NotifyContext(
             context.Background(), syscall.SIGTERM)
         defer cancel()

         mux := http.NewServeMux()

         mux.Handle("/metrics", promhttp.Handler())

         mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
             w.WriteHeader(http.StatusOK)
         })

         mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
             if !ready.Load() {
                 w.WriteHeader(http.StatusServiceUnavailable)
                 return
             }
             w.WriteHeader(http.StatusOK)
         })

         srv := &http.Server{Addr: ":8080", Handler: mux}

         go func() {
             slog.Info("starting server", "addr", srv.Addr)
             if err := srv.ListenAndServe(); err != nil {
                 slog.Error("server error", "err", err)
                 os.Exit(1)
             }
         }()

         ready.Store(true)
         slog.Info("ready")

         <-ctx.Done()
         slog.Info("shutdown signal")
         ready.Store(false)

         shutdownCtx, shutdownCancel := context.WithTimeout(
             context.Background(), 20*time.Second)
         defer shutdownCancel()

         if err := srv.Shutdown(shutdownCtx); err != nil {
             slog.Error("shutdown error", "err", err)
         }
         slog.Info("shutdown complete")
     }

  ЧТО ЗДЕСЬ:
    • slog JSON — структурированные логи.
    • /metrics — Prometheus.
    • /health и /ready — probes.
    • Graceful shutdown — SIGTERM.

  ПРАВИЛО: Go-сервис в K8s должен уметь: писать структурированные логи, экспонировать метрики, отвечать на probes, корректно завершаться.

  14. АНТИПАТТЕРНЫ
  14.1. ЛОГИ В ФАЙЛ, А НЕ В STDOUT. K8s не увидит. kubectl logs пусто.
  14.2. НЕТ СТРУКТУРНЫХ ЛОГОВ. Парсить plain text больно. Используй JSON.
  14.3. ЛОГИРОВАТЬ СЕКРЕТЫ. Пароли, токены, PII в логах — утечка.
  14.4. ЛОГИРОВАТЬ ВСЁ НА УРОВНЕ INFO. Шум, дорого. Debug включай по флагу.
  14.5. МЕТРИКИ БЕЗ LABELS. Не поймёшь, какой endpoint тормозит. Добавляй method, path, status.
  14.6. МЕТРИКИ С ВЫСОКОЙ КАРДИНАЛЬНОСТЬЮ. user_id, request_id в labels — убьют Prometheus.
  14.7. АЛЕРТЫ НА ВСЁ. Alert fatigue. Алерты только на то, что требует действия.
  14.8. ДАШБОРД БЕЗ АЛЕРТОВ. Никто не смотрит на дашборд 24/7. Нужны алерты.
  14.9. НЕТ ТРЕЙСИНГА В МИКРОСЕРВИСАХ. 5+ сервисов — без трейсинга дебаг невозможен.
  14.10. TRACE_ID НЕ ПРОБРАСЫВАЕТСЯ. Если trace_id теряется между сервисами — трейс разрывается.
  14.11. ЛОГИ БЕЗ TRACE_ID. Не связать лог с трейсом.
  14.12. ИГНОРИРОВАТЬ EVENTS. Describe и events отвечают быстрее, чем копание в логах.
  14.13. НЕТ LOG ROTATION. Диск ноды переполнится, Pod'ы эвиктятся.
  14.14. PROMETHEUS НА ОДНОЙ НОДЕ. Упала нода — упал мониторинг.
  14.15. ALERTMANAGER БЕЗ ПРОВЕРКИ. Алерт не дошёл до человека — инцидент не заметили.
  14.16. ВЫСОКИЙ SAMPLING В TRACING. 100% трейсов — дорого. 1-10% обычно достаточно.
  14.17. ЛОГИ БЕЗ УРОВНЕЙ. Всё в одну кучу, невозможно фильтровать.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Observability — способность понять состояние системы по внешним сигналам.
  2.  Три столпа: logs (что), metrics (сколько), traces (где). Плюс events (состояние K8s-объектов).
  3.  Логи — только stdout/stderr. K8s сам собирает. Не пиши в файлы.
  4.  kubectl logs, --previous, --all-containers, -l, -f — основные команды.
  5.  Централизованный сбор логов: sidecar (специфика) или DaemonSet (стандарт).
  6.  metrics-server — для HPA и kubectl top. Prometheus — для полноценного мониторинга.
  7.  Prometheus — pull-модель. Приложение отдаёт /metrics.
  8.  Четыре типа метрик: Counter, Gauge, Histogram, Summary.
  9.  ServiceMonitor / PodMonitor — декларативная конфигурация scrape.
  10. RED (Rate, Errors, Duration) для сервисов. USE (Utilization, Saturation, Errors) для ресурсов.
  11. Трейсинг — обязателен в микросервисах. OpenTelemetry — стандарт. Jaeger/Tempo — backend.
  12. Events — встроенный механизм K8s. Живут ~1 час. Смотреть через describe и get events.
  13. Go-сервис: slog JSON, /metrics, /health, /ready, graceful shutdown по SIGTERM.
  14. Антипаттерны: логи в файл, plain text логи, метрики с высокой кардинальностью, алерты на всё, нет трейсинга в микросервисах, trace_id не пробрасывается.
  15. Конкретные статусы Pod'ов (CrashLoopBackOff, OOMKilled, Pending и т.д.) — в уроке 6.1.
  16. На собесе: знать три столпа, метрики RED/USE, инструменты (Prometheus, Jaeger, Loki), что нужно от Go-сервиса.
*/
