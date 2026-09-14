package main

/*
  УРОК 7.8: DISTRIBUTED TRACING
  В монолите отладка проста: ставишь точку остановки или смотришь
  stack trace. Один процесс, один поток, одна история. В микросервисах
  один запрос пользователя проходит через 10-20 сервисов, каждый
  со своим логом, своей БД, своим кэшем. Где именно задержка?
  Какой сервис вернул ошибку? Почему заказ не создался?
  Логи говорят "error in payment-service", но не говорят, что этому
  предшествовало в order-service, что произошло параллельно
  в inventory-service, и сколько времени занял каждый шаг.
  Метрики говорят "p99 latency выросла", но не говорят, какой
  именно путь запроса стал медленным.
  Distributed Tracing отвечает на вопрос "что произошло с конкретным
  запросом на всём его пути через систему". Это единственный способ
  отлаживать сложные распределённые системы в продакшене.

  СОДЕРЖАНИЕ:
    1.  Проблема: почему логи и метрики не спасают
    2.  Что такое distributed tracing
    3.  Trace, Span, Span Context — базовые концепции
    4.  W3C Trace Context — стандарт propagation
    5.  Propagation через HTTP, gRPC, Kafka
    6.  OpenTelemetry — стандарт индустрии
    7.  Архитектура OTel: API, SDK, Collector
    8.  Instrumentation: auto vs manual
    9.  Sampling — что и как записывать
    10. Exporters и OTLP
    11. Jaeger, Tempo, Zipkin — backend для визуализации
    12. Что инструментировать: HTTP, DB, Kafka, gRPC
    13. Span: attributes, events, links, status
    14. Context propagation в Go
    15. Корреляция с логами и метриками
    16. Стоимость и производительность
    17. Антипаттерны
    18. Финальные выводы

  1. ПРОБЛЕМА: ПОЧЕМУ ЛОГИ И МЕТРИКИ НЕ СПАСАЮТ
  Три столпа observability: logs, metrics, traces. Они отвечают
  на разные вопросы.

  ЛОГИ отвечают: "что произошло в этом сервисе?".
  Метрики отвечают: "сколько запросов, какая latency, сколько ошибок?".
  Трейсы отвечают: "как прошёл КОНКРЕТНЫЙ запрос через ВСЮ систему?".

  Представь, пользователь жалуется: "заказ оформлялся 15 секунд".
  У тебя 20 микросервисов. Что ты будешь делать?

  С логами: пойдёшь по логам каждого сервиса, пытаясь найти
  correlation_id. Если его нет — вообще не найдёшь. Если есть —
  будешь grep'ать по 20 файлам и вручную собирать таймлайн.

  С метриками: увидишь, что p99 latency выросла. Но не увидишь,
  какой именно путь запроса стал медленным. Может быть, 90% запросов
  быстрые, а медленные только те, что идут через inventory-service
  в зоне eu-west.

  С трейсами: откроешь Jaeger, найдёшь trace по ID, увидишь
  waterfall: order-service 100ms, payment-service 200ms,
  inventory-service 14 секунд, notification-service 50ms.
  За 10 секунд понял, кто виноват.

  Распределённый трейс — это не замена логам и метрикам.
  Это третий инструмент, который отвечает на вопрос о пути
  запроса через систему.

  2. ЧТО ТАКОЕ DISTRIBUTED TRACING
  Distributed Tracing — это техника отслеживания пути запроса
  через распределённую систему. Каждый шаг запроса записывается
  как "span". Все span'ы одного запроса объединяются в "trace"
  через общий trace ID.

  Представь, что пользователь нажал "оформить заказ". Запрос
  прошёл через:
    API Gateway → order-service → payment-service → bank API
                                → inventory-service → warehouse DB
                                → notification-service → email

  Distributed tracing создаёт дерево:
    [Trace: 4bf92f3577b34da6a3ce929d0e0e4736]
    │
    ├─ span: API Gateway (10ms)
    │
    ├─ span: order-service (250ms)
    │   ├─ span: validate_cart (5ms)
    │   ├─ span: reserve_payment (200ms)
    │   │   └─ span: bank API call (180ms)
    │   ├─ span: reserve_inventory (30ms)
    │   │   └─ span: warehouse DB query (25ms)
    │   └─ span: publish_event (10ms)
    │
    └─ span: notification-service (50ms)
        └─ span: send_email (45ms)

  Каждый span знает:
    • Свой trace_id (общий для всего трейса).
    • Свой span_id (уникальный).
    • parent_span_id (кто его вызвал).
    • start_time, duration.
    • status (ok, error).
    • attributes (метаданные: http.method, db.statement, user.id).

  Из этих данных строится waterfall-диаграмма. Ты видишь не только
  общее время, но и что происходило параллельно, где узкие места,
  где ошибки.

  3. TRACE, SPAN, SPAN CONTEXT — БАЗОВЫЕ КОНЦЕПЦИИ
  TRACE — весь путь запроса. Идентифицируется 128-битным
  trace_id (16 байт в hex). Один trace — один пользовательский
  запрос.

  SPAN — одна операция внутри трейса. Это может быть HTTP-запрос,
  запрос к БД, вызов внешнего API, публикация в Kafka, вычисление
  хэша. Span имеет:

    • span_id — уникальный 64-битный идентификатор.
    • parent_span_id — id родительского span'а.
    • trace_id — id всего трейса.
    • name — короткое имя операции ("GET /api/orders").
    • kind — тип span'а (SERVER, CLIENT, PRODUCER, CONSUMER, INTERNAL).
    • start_time, end_time.
    • status — OK, ERROR, UNSET.
    • attributes — key-value метаданные.
    • events — временные метки внутри span'а ("cache miss" в момент t).
    • links — связи с другими span'ами (например, при batch processing).

  SPAN CONTEXT — минимальный набор данных для propagation:
  trace_id, span_id, trace_flags (sampled или нет). Это то,
  что передаётся между сервисами в заголовках.

  ROOT SPAN — первый span в трейсе. У него нет parent_span_id.
  Обычно это span HTTP-запроса на входе в систему.

  CHILD SPAN — span, вызванный внутри другого span'а. Иерархия
  строится через parent_span_id.

  ВАЖНО: span'ы создаются не для каждой функции. Span — это
  граница операции, которая может пересечь процесс, сеть
  или поток. Создавать span на каждый вызов функции — оверинжиниринг.
  Span должен быть на:
    • HTTP-запрос.
    • Запрос к БД.
    • Вызов внешнего API.
    • Публикация/чтение из очереди.
    • Значимая бизнес-операция (например, "reserve_payment").

  4. W3C TRACE CONTEXT — СТАНДАРТ PROPAGATION
  Чтобы трейс собрался воедино, контекст должен передаваться
  между сервисами. Это делается через заголовки HTTP/gRPC.

  Долгое время у каждого вендора был свой формат: B3 (Zipkin),
  Jaeger, Datadog, AWS X-Ray. Это создавало проблему при
  миксе инструментов.

  В 2020 году W3C стандартизировал формат Trace Context.
  Сейчас это де-факто стандарт.

  Формат заголовка traceparent:

    traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
                 │  │                                │                │
                 │  │                                │                └─ flags (01 = sampled)
                 │  │                                └─ parent span_id (8 байт hex)
                 │  └─ trace_id (16 байт hex)
                 └─ version (00)

  Дополнительный заголовок tracestate для вендор-специфичных данных:
    tracestate: congo=t61rcWkgMzE,rojo=00f067aa0ba902b7

  ПРАВИЛО: если сервис получил traceparent, он должен использовать
  тот же trace_id для всех исходящих вызовов. И создавать новые
  span_id для каждого span'а.

  ЕСЛИ СЕРВИС НЕ ПОЛУЧИЛ traceparent — он создаёт новый trace_id
  и становится корнем трейса.

  5. PROPAGATION ЧЕРЕЗ HTTP, GRPC, KAFKA
  HTTP — стандартный заголовок traceparent. OpenTelemetry
  автоматически прокидывает его через http.RoundTripper.
  Достаточно обернуть transport.

  gRPC — метаданные в context. OTel перехватчики (interceptors)
  автоматически прокидывают trace context.

  KAFKA — trace context передаётся в headers сообщения.
  При publish создаётся PRODUCER span, при consume — CONSUMER span,
  связанный через parent.

  БД — к сожалению, trace context не всегда прокидывается
  до БД. Некоторые БД поддерживают SQL comment injection
  (traceparent в комментарии к запросу) — тогда в логах БД
  видно, к какому трейсу относится запрос.

  ВАЖНО: propagation должен быть везде. Если хотя бы один
  сервис не прокидывает context — трейс разрывается. Ты увидишь
  два независимых трейса вместо одного длинного.

  6. OPENTELEMETRY — СТАНДАРТ ИНДУСТРИИ
  OpenTelemetry (OTel) — это CNCF-проект, который объединил
  OpenCensus и OpenTracing в 2019 году. Это стандарт сбора
  traces, metrics и logs.

  Главная идея: единый API для инструментирования, единый
  формат данных, единый протокол экспорта (OTLP). Ты пишешь
  код один раз — и можешь отправлять данные в любой backend
  (Jaeger, Tempo, Datadog, New Relic, Grafana Cloud).

  Что даёт OTel:
    • VENDOR-NEUTRAL — не привязан к вендору.
    • ЕДИНЫЙ API — для Go, Java, Python, JS, .NET, Ruby.
    • AUTO-INSTRUMENTATION — библиотеки для популярных фреймворков.
    • MANUAL-INSTRUMENTATION — API для своих span'ов.
    • COLLECTOR — прокси для приёма, обработки и экспорта.
    • СЕМАНТИЧЕСКИЕ КОНВЕНЦИИ — единые имена атрибутов.

  В Go SDK: go.opentelemetry.io/otel.

  7. АРХИТЕКТУРА OTel: API, SDK, COLLECTOR
  OTel разделяет API и SDK. Это важно для библиотек.

  API — интерфейсы для инструментирования. Библиотеки зависят
  только от API. Если приложение не настраивает SDK — все span'ы
  no-op (никаких накладных расходов).

  SDK — реализация. Приложение настраивает SDK: экспортёры,
  sampler, resource. SDK создаёт реальные span'ы и экспортирует их.

  COLLECTOR — отдельный процесс (или sidecar). Принимает данные
  по OTLP, обрабатывает (batching, sampling, filtering),
  экспортирует в backend'ы.

  Зачем Collector:
    • DECOUPLING — приложение шлёт в Collector, а не в backend.
      Можно менять backend без изменения приложения.
    • BATCHING — собирает span'ы в батчи, снижает нагрузку на сеть.
    • SAMPLING — tail-based sampling на стороне Collector.
    • MULTI-EXPORT — отправлять в несколько backend'ов.
    • RETRY — если backend упал, Collector ретраит.
    • FILTERING — выкидывать шумные span'ы (health checks).

  Схема:
    App (SDK) → OTLP → Collector → Jaeger/Tempo/etc
                      │
                      ├─ Batching
                      ├─ Filtering
                      └─ Sampling

  8. INSTRUMENTATION: AUTO VS MANUAL

  AUTO-INSTRUMENTATION — библиотеки, которые сами создают span'ы
  для HTTP, gRPC, БД, Kafka. Не требует изменения бизнес-логики.

  Для Go auto-instrumentation слабее, чем для Java/Python.
  Есть otelhttp, otelgrpc, otelpgx, otelkafka. Они дают базовые
  span'ы на границах.

  MANUAL-INSTRUMENTATION — ты сам создаёшь span'ы в коде.
  Нужно для бизнес-операций, которые auto-инструментация
  не видит.

    ctx, span := tracer.Start(ctx, "reserve_payment")
    defer span.End()

    if err := doReservePayment(ctx); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "payment failed")
        return err
    }

    span.SetAttributes(
        attribute.String("payment.provider", "stripe"),
        attribute.Float64("payment.amount", amount),
    )

  КОГДА MANUAL ОБЯЗАТЕЛЕН:
    • Бизнес-операции, не привязанные к сети ("validate_cart").
    • Долгие операции внутри сервиса ("generate_report").
    • Циклы/батчи (не создавай span на каждую итерацию —
      создавай один на батч, или используй span links).
    • Сложные вычисления.

  9. SAMPLING — ЧТО И КАК ЗАПИСЫВАТЬ
  100% трейсов записывать нельзя. Это дорого. Каждый span —
  это ~1 KB данных. 1000 RPS = 1 GB/sec. За сутки — 86 TB.

  Решения:
  HEAD-BASED SAMPLING — решение принимается на входе, до создания
  span'а. Root span решает "sampled" или "not sampled", и это
  решение распространяется на весь трейс.

    • ALWAYS_ON — 100% (только для dev).
    • ALWAYS_OFF — 0% (для отключения).
    • TRACE_ID_RATIO — N% трейсов (обычно 1-10%).
    • PARENT_BASED — следовать за родителем (для сервисов).
    • JAEGER_REMOTE — конфиг из Jaeger backend.

  Плюсы: дёшево, нет оверхеда.
  Минусы: теряешь редкие ошибки. Sampling 1% — не увидишь
  тот единственный медленный запрос, который нужно отладить.

  TAIL-BASED SAMPLING — решение принимается на стороне Collector
  после завершения трейса. Можно посмотреть на весь трейс и решить:
  "есть ошибка — оставить, latency > 1s — оставить".

  Плюсы: не теряешь важные трейсы.
  Минусы: Collector должен буферизовать весь трейс, сложнее, дороже.

  КОМПРОМИСС: head-based для 1-10% + tail-based для ошибок
  и медленных запросов.

  ПРАВИЛО: НЕ выключай sampling на 100% даже в dev.
  Ты не заметишь, как трейсинг начнёт тормозить систему.

  10. EXPORTERS И OTLP
  EXPORTER — компонент SDK, который отправляет данные в backend.

  OTLP (OpenTelemetry Protocol) — стандартный протокол для
  передачи данных. Это HTTP или gRPC/protobuf. Работает с
  любым OTel-совместимым backend'ом.

  Другие экспортёры (устаревают в пользу OTLP):
    • JAEGER — для Jaeger (native).
    • ZIPKIN — для Zipkin.
    • STDOUT — в stdout (для отладки).
    • PROMETHEUS — для метрик.

  BATCH SPAN PROCESSOR — экспортёр не отправляет span'ы сразу,
  а собирает в батч (BatchSpanProcessor). Это снижает нагрузку
  на сеть и backend.

    exporter, _ := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("collector:4317"),
        otlptracegrpc.WithInsecure(),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter,
            trace.WithBatchTimeout(5*time.Second),
            trace.WithMaxExportBatchSize(512),
        ),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("order-service"),
            semconv.ServiceVersion("1.0.0"),
        )),
        trace.WithSampler(trace.TraceIDRatioBased(0.1)),
    )
    otel.SetTracerProvider(tp)
    defer tp.Shutdown(ctx)

  11. JAEGER, TEMPO, ZIPKIN — BACKEND ДЛЯ ВИЗУАЛИЗАЦИИ
  Backend — это хранилище трейсов и UI для их визуализации.

  JAEGER — самый популярный. Разработан Uber, передан в CNCF.
  Хранит данные в Elasticsearch, Cassandra или Badger.
  Мощный UI, поддерживает сервисные графы, сравнение трейсов.
  Классический выбор для self-hosted.

  TEMPO — от Grafana Labs. Только хранит данные, не индексирует
  их. Очень дёшево. Интеграция с Grafana через TraceQL.
  Идеален, если у тебя уже есть стек Grafana + Loki + Prometheus.

  ZIPKIN — старожил от Twitter. Проще Jaeger, но менее
  активный. Хорош для базовых случаев.

  СРАВНЕНИЕ:
    ┌──────────────┬──────────┬──────────┬──────────────┐
    │              │ Jaeger   │ Tempo    │ Zipkin       │
    ├──────────────┼──────────┼──────────┼──────────────┤
    │ Индексация   │ Да       │ Нет      │ Да           │
    │ Стоимость    │ Средняя  │ Низкая   │ Средняя      │
    │ UI           │ Отдельный│ Grafana  │ Отдельный    │
    │ Активность   │ Высокая  │ Высокая  │ Средняя      │
    │ CNCF         │ Да       │ Да       │ Нет          │
    └──────────────┴──────────┴──────────┴──────────────┘

  Что смотреть в UI:
    • WATERFALL — таймлайн всех span'ов, где видно параллельные
      и последовательные вызовы.
    • SERVICE GRAPH — карта зависимостей между сервисами.
    • FLAME GRAPH — агрегированный CPU/latency профиль.
    • COMPARE — сравнение двух трейсов (быстрый vs медленный).
    • TRACE DETAILS — атрибуты, события, ошибки.

  12. ЧТО ИНСТРУМЕНТИРОВАТЬ
  Инструментируй границы, а не внутренности.

  ОБЯЗАТЕЛЬНО:
    • HTTP SERVER — входящие запросы. otelhttp.NewHandler.
    • HTTP CLIENT — исходящие запросы. otelhttp.NewTransport.
    • gRPC SERVER — otelgrpc.NewServerHandler.
    • gRPC CLIENT — otelgrpc.NewClientHandler.
    • DB — otelpgx, otelsql, otelmongo.
    • KAFKA — otelkafka (producer + consumer).
    • REDIS — otelredis.

  ПОЛЕЗНО:
    • Бизнес-операции (reserve_payment, place_order).
    • Внешние API (вызовы Stripe, Twilio).
    • Кэш (cache hit/miss события внутри span'а).
    • Долгие циклы (один span на батч).

  НЕ НУЖНО:
    • Каждый вызов функции. Оверинжиниринг.
    • Health checks (шум в трейсах). Фильтруй в Collector.
    • /metrics, /debug эндпоинты.
    • Быстрые операции (< 1ms), если не критичны.

  ПРАВИЛО: span должен отвечать на вопрос "почему это медленно
  или почему это упало". Если он не помогает ответить — не нужен.

  13. SPAN: ATTRIBUTES, EVENTS, LINKS, STATUS
  SPAN ATTRIBUTES — key-value метаданные. Единые имена через
  семантические конвенции OTel:
    • http.method, http.route, http.status_code.
    • db.system, db.statement, db.operation.
    • messaging.system, messaging.destination.
    • rpc.system, rpc.service, rpc.method.
    • enduser.id, user.id.
    • Custom: payment.provider, order.value.

  ВАЖНО: НЕ клади в attributes PII (email, телефоны,
  номера карт). Используй хэши или идентификаторы.

  SPAN EVENTS — временные метки внутри span'а. Для значимых
  моментов: "cache miss", "retry #2", "circuit breaker opened".
    span.AddEvent("retry", trace.WithAttributes(
        attribute.Int("attempt", 2),
    ))

  SPAN LINKS — связи с другими span'ами. Используется при
  batch processing: один span обработки батча линкуется
  со span'ами создания каждого элемента батча.

  SPAN STATUS:
    • UNSET (по умолчанию).
    • OK (успех, явно).
    • ERROR (ошибка, с описанием).

  ПРАВИЛО: status = ERROR, только если операция реально
  провалилась. HTTP 404 — это OK (нормальный ответ).
  HTTP 500 — ERROR. Не ставь ERROR на каждую мелочь.

  14. CONTEXT PROPAGATION В GO

  В Go trace context живёт в context.Context. Это значит:
    • ctx должен передаваться через все функции первым
      параметром.
    • Нельзя использовать context.Background() внутри
      обработки запроса — разорвёшь propagation.
    • Нельзя сохранять ctx в структурах.
    • goroutines должны получать ctx от родителя.

  Создание span'а:
    ctx, span := tracer.Start(ctx, "operation_name")
    defer span.End()

    // работа внутри span'а
    result, err := doWork(ctx)

  ВАЖНО: defer span.End() обязателен. Если забудешь — span
  не завершится, трейс будет неправильным.

  Передача context в HTTP:
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    // otelhttp.Transport автоматически добавит traceparent

  Передача context в goroutine:
    go func(ctx context.Context) {
        ctx, span := tracer.Start(ctx, "background_work")
        defer span.End()
        doWork(ctx)
    }(ctx)

  15. КОРРЕЛЯЦИЯ С ЛОГАМИ И МЕТРИКАМИ
  Трейсы — только один из столпов. Чтобы получить полную
  картину, нужно коррелировать их с логами и метриками.

  В ЛОГАХ — обязательно добавляй trace_id и span_id в каждую
  запись. Тогда из трейса можно перейти в логи и обратно.

    logger.Info("payment processed",
        "trace_id", trace.SpanContextFromContext(ctx).TraceID().String(),
        "span_id", trace.SpanContextFromContext(ctx).SpanID().String(),
        "amount", amount,
    )

  В МЕТРИКАХ — используй exemplars. Это идентификаторы trace'ов,
  привязанные к гистограммам. В Grafana/Prometheus можно
  кликнуть на точку latency и перейти в конкретный trace.

  ПРАВИЛО: одно и то же событие не должно дублироваться в логах
  и трейсах. Если что-то видно в трейсе — логировать это не надо.
  Логи дополняют трейсы, а не дублируют.

  16. СТОИМОСТЬ И ПРОИЗВОДИТЕЛЬНОСТЬ

  Tracing — не бесплатно. Накладные расходы:
    • SDK создание span'а: ~1-2 микросекунды.
    • Сериализация: ~5-10 микросекунд на span.
    • Отправка по сети: ~50-100 микросекунд на батч.
    • Backend storage: 1 KB на span.

  Для 1000 RPS с 10 span'ами на запрос:
    • 10 000 span'ов/сек.
    • 10 GB данных в час.
    • 240 GB данных в день.

  Это накладно. Решения:
    • HEAD SAMPLING 1-10% в проде.
    • TAIL SAMPLING для ошибок.
    • BATCHING в SDK (5 сек, 512 span'ов).
    • COLLECTOR как прокси (снижает нагрузку).
    • RETENTION 7-30 дней в backend.
    • СТИРАЙ ненужные attributes.

  PERFORMANCE-КРИТИЧНЫЕ СЕРВИСЫ: включай sampling 0.1-1%.
  Трейсы нужны для отладки, а не для 100% трафика.

  17. АНТИПАТТЕРНЫ

  17.1. TRACE БЕЗ PROPAGATION
    Каждый сервис создаёт свой trace. Единого трейса нет.
    Обязательно прокидывай traceparent в headers.
  17.2. SPAN НА КАЖДУЮ ФУНКЦИЮ
    Оверинжиниринг. Span — на границы операций, а не
    на вызовы методов.
  17.3. 100% SAMPLING В ПРОДЕ
    Дорого, тормозит систему. 1-10% достаточно.
  17.4. PII В ATTRIBUTES
    Email, телефоны, номера карт в span'ах — это утечка.
    Хэшируй или не клади.
  17.5. ЗАБЫТЬ defer span.End()
    Span не завершится, трейс будет сломан.
  17.6. context.Background() ВНУТРИ HANDLER
    Разрывает цепочку propagation. Теряешь parent span.
  17.7. ИНСТРУМЕНТИРОВАНИЕ ТОЛЬКО ОДНОГО СЕРВИСА
    Трейс обрывается на первом неинструментированном сервисе.
    Инструментируй ВСЕ сервисы на пути запроса.
  17.8. ЗАПИСЬ ВСЕХ ЛОГОВ В ТРЕЙС
    Логи и трейсы — разные инструменты. Не дублируй.
  17.9. TRACING БЕЗ HEALTH-CHECK FILTER
    /health, /metrics засоряют трейсы. Фильтруй в Collector.
  17.10. TRACING КАК ЗАМЕНА ЛОГАМ
    Трейсы не заменяют логи. Они дополняют их.

  18. ФИНАЛЬНЫЕ ВЫВОДЫ

  1.  Distributed Tracing — единственный способ отлаживать
      путь запроса через 10-20 сервисов. Логи и метрики
      этого не дают.
  2.  Trace = дерево span'ов. Один trace = один пользовательский
      запрос. Span = одна операция.
  3.  W3C Trace Context — стандарт propagation через
      traceparent header. Де-факто индустриальный стандарт.
  4.  OpenTelemetry — единый API, SDK и протокол. Не привязывайся
      к вендору.
  5.  Collector — обязательный компонент в проде. Batching,
      sampling, multi-export.
  6.  Sampling — 1-10% head-based в проде. Для ошибок —
      tail-based.
  7.  Jaeger для self-hosted, Tempo для Grafana-стека, Zipkin
      для простых случаев.
  8.  Инструментируй ГРАНИЦЫ: HTTP, gRPC, БД, Kafka. Не каждый
      метод.
  9.  Span attributes — не для PII. Семантические конвенции
      для имён.
  10. Контекст прокидывается через context.Context. Никогда
      не используй context.Background() внутри обработки.
  11. Коррелируй с логами через trace_id/span_id, с метриками
      через exemplars.
  12. Tracing не бесплатно. Считай стоимость: 10 GB/час
      при 1000 RPS без sampling.
  13. Инструментируй ВСЕ сервисы. Один неинструментированный
      сервис разрывает трейс.
  14. Начинай с auto-instrumentation. Добавляй manual для
      бизнес-операций.
  15. Tracing — инвестиция в отладку. Первые полгода
      кажется бесполезным, в первом серьёзном инциденте
      окупается на годы.
*/
