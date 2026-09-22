package main

/*
  УРОК 6.4: RETRY В ПРИЛОЖЕНИИ
  Ты настроил healthcheck. Ты настроил depends_on с
  condition: service_healthy. Ты настроил stop_grace_period.
  Почему? Потому что healthcheck и depends_on — не серебряная
  пуля. Они уменьшают вероятность гонки, но не убирают её.
  Остаются случаи:
    • База перезапустилась во время работы — соединение
      прервалось.
    • Сеть моргнула — пакеты потерялись.
    • База ещё не готова, хотя healthcheck уже прошёл
      (например, реплика).
    • Внешний API временно недоступен.
  Единственный способ жить с этим — retry в коде. Не как
  костыль, а как часть архитектуры.

  СОДЕРЖАНИЕ:
    1.  Почему healthcheck не панацея
    2.  Что такое retry и когда он нужен
    3.  Что можно и что нельзя повторять
    4.  Exponential backoff
    5.  Jitter — зачем и как
    6.  Retry для подключения к БД
    7.  Retry для Kafka
    8.  Retry для HTTP-запросов
    9.  Общая библиотека retry
    10. Контекст и отмена
    11. wait-for-it.sh и его место
    12. Идемпотентность при retry
    13. Связь с circuit breaker
    14. Антипаттерны
    15. Финальные выводы

  1. ПОЧЕМУ HEALTHCHECK НЕ ПАНАЦЕЯ
  Healthcheck проверяет, что сервис готов. depends_on ждёт
  healthcheck. Это работает при старте. Но:

  СЦЕНАРИЙ 1: БД ПЕРЕЗАПУСТИЛАСЬ ПОСЛЕ СТАРТА.
    t=0     app стартует
    t=5     Postgres healthy
    t=5     app стартует успешно
    t=60    Postgres перезапустился (админ, обновление, OOM)
    t=60    app получает "connection refused"
    t=60    app падает

    depends_on уже не помогает — он работает только при старте.

  СЦЕНАРИЙ 2: HEALTHCHECK ПРОШЁЛ, НО БД НЕ ГОТОВА.
    Postgres с репликацией. Healthcheck на master проходит.
    Но реплика ещё догоняет. Приложение читает с реплики —
    данные устаревшие или её нет вообще.

  СЦЕНАРИЙ 3: ВНЕШНИЙ API ВРЕМЕННО НЕДОСТУПЕН.
    Stripe, AWS, любой внешний сервис. Отдаёт 503.
    Через секунду — работает. Но app уже упал.

  СЦЕНАРИЙ 4: СЕТЕВЫЕ ПРОБЛЕМЫ.
    Пакеты теряются. DNS не резолвится. TCP-соединение не
    устанавливается.

  ВЫВОД: healthcheck + depends_on + retry = надёжность.
  Убирать retry нельзя — это защита от runtime-проблем.

  2. ЧТО ТАКОЕ RETRY И КОГДА ОН НУЖЕН
  Retry — повторное выполнение операции, если предыдущая попытка
  упала с временной ошибкой.

  КОГДА НУЖЕН:
    • Подключение к БД при старте.
    • Подключение к Redis, Kafka, любому брокеру.
    • HTTP-запросы к внешним API.
    • Чтение и запись в БД при временных ошибках.
    • Подключение к DNS, service discovery.

  КОГДА НЕ НУЖЕН:
    • Ошибка валидации (400, 422). Повтор не поможет.
    • Ошибка авторизации (401, 403).
    • Ресурс не найден (404).
    • Бизнес-ошибка (недостаточно средств, товар не в наличии).
    • Неидемпотентная операция без idempotency key.

  ПРАВИЛО: retry только для ВРЕМЕННЫХ (transient) ошибок.

  3. ЧТО МОЖНО И ЧТО НЕЛЬЗЯ ПОВТОРЯТЬ

  ИДЕМПОТЕНТНЫЕ ОПЕРАЦИИ — можно повторять:
    • GET-запросы.
    • PUT, DELETE (в REST — идемпотентны по спецификации).
    • SELECT в БД.
    • Установка значения: SET balance = 100.
    • Публикация события с уникальным event_id (consumer
      дедуплицирует).

  НЕИДЕМПОТЕНТНЫЕ — нельзя без idempotency key:
    • POST-запросы (создание ресурса).
    • INSERT без уникального ключа.
    • Инкремент: balance = balance + 10.
    • Отправка SMS, email.
    • Списание денег.

  РЕШЕНИЕ ДЛЯ НЕИДЕМПОТЕНТНЫХ:
    Idempotency key. Клиент генерирует уникальный ключ на
    операцию. Сервер проверяет: если операция с таким ключом
    уже выполнена — возвращает прошлый результат, не выполняя
    заново.

    Пример:
      POST /payments
      Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000

    При retry клиент посылает тот же ключ. Сервер видит
    знакомый ключ → возвращает прошлый ответ.

  ДЛЯ RETRY ВСЕГДА:
    • GET, HEAD — да.
    • PUT, DELETE — да.
    • POST — только с idempotency key.
    • PATCH — зависит от семантики.

  4. EXPONENTIAL BACKOFF
  Если повторять сразу — это добивает систему. Плохо.

  Плохой retry:
    Попытка 1: сразу (упала)
    Попытка 2: сразу (упала)
    Попытка 3: сразу (упала)

    Три запроса за миллисекунду. Если сервис перегружен —
    только ухудшаем.

  Exponential backoff — увеличивает задержку между попытками.

  ФОРМУЛА:
    delay = initialDelay * multiplier^(attempt)

  Пример:
    initialDelay = 100ms
    multiplier = 2

    Попытка 1: сразу
    Попытка 2: через 100ms
    Попытка 3: через 200ms
    Попытка 4: через 400ms
    Попытка 5: через 800ms
    Попытка 6: через 1.6s
    ...

  С МАКСИМУМОМ:
    delay = min(maxDelay, initialDelay * multiplier^attempt)

    maxDelay = 30s

    Тогда после 1.6s, 3.2s, 6.4s задержка упирается в 30s
    и не растёт.

  ПРИМЕР В GO:
    func backoff(attempt int, initial, max time.Duration) time.Duration {
        delay := initial * time.Duration(math.Pow(2, float64(attempt)))
        if delay > max {
            delay = max
        }
        return delay
    }

  СКОЛЬКО ПОПЫТОК:
    Обычно 3-5. Больше — растёт время до отказа.
    5 попыток с backoff до 30s = около 1 минуты. Хватит,
    чтобы БД поднялась.

  5. JITTER — ЗАЧЕМ И КАК
  Проблема чистого exponential backoff: синхронизация.

  Если 100 сервисов одновременно получили ошибку и одновременно
  начали retry с одинаковыми задержками — все 100 ударят по
  сервису в одну миллисекунду. Это thundering herd.

  JITTER добавляет случайность к задержке.

  FULL JITTER:
    delay = random(0, baseDelay)
  Агрессивный. Полностью заменяет backoff случайной величиной.

  EQUAL JITTER:
    delay = baseDelay/2 + random(0, baseDelay/2)
  Половина фиксирована, половина случайна.

  DECORRELATED JITTER:
    delay = min(max, random(baseDelay, prevDelay*3))
  Каждая задержка зависит от предыдущей.

  FULL JITTER В GO:
    func backoffWithJitter(attempt int, initial, max time.Duration) time.Duration {
        base := initial * time.Duration(math.Pow(2, float64(attempt)))
        if base > max {
            base = max
        }
        return time.Duration(rand.Int63n(int64(base)))
    }

  ЭФФЕКТ:
    Без jitter: 100 сервисов retry в одну и ту же миллисекунду.
    С jitter: 100 сервисов retry в случайные моменты на
    интервале [0, base). Нагрузка распределяется.

  ПРАВИЛО: используй full jitter. Это стандарт в AWS,
  Google, во всех серьёзных системах.

  6. RETRY ДЛЯ ПОДКЛЮЧЕНИЯ К БД
  Самое частое место для retry — подключение к БД при старте.

  ПРОБЛЕМА:
    Сервис стартует.
    Подключается к Postgres.
    "connection refused".
    Падает.

    А Postgres через 2 секунды готов.

  РЕШЕНИЕ:
    Retry подключения с backoff и jitter.

  ПРИМЕР ДЛЯ POSTGRES (pgxpool):
    func connectPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
        var pool *pgxpool.Pool
        var err error

        for attempt := 0; attempt < 10; attempt++ {
            pool, err = pgxpool.New(ctx, dsn)
            if err == nil {
                if pingErr := pool.Ping(ctx); pingErr == nil {
                    return pool, nil
                } else {
                    err = pingErr
                    pool.Close()
                }
            }

            // Backoff с jitter.
            delay := time.Duration(rand.Int63n(int64(
                time.Second * time.Duration(1<<uint(attempt)))))
            if delay > 30*time.Second {
                delay = 30 * time.Second
            }

            log.Printf("postgres not ready, retry in %v: %v", delay, err)
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
        return nil, fmt.Errorf("postgres unreachable after 10 attempts: %w", err)
    }

  ЧТО ВАЖНО:
    • Контекст. При отмене (shutdown) — выйти сразу.
    • Jitter. Не долбить все сервисы одинаково.
    • Максимум попыток. Не вечный цикл.
    • Логи. Видно, что происходит.
    • Возврат ошибки после лимита.

  ПРОКСИ ЧЕРЕЗ PING:
    sql.Open не подключается. Только готовит пул.
    Ping — реальное подключение.
    Проверяй именно Ping.

  7. RETRY ДЛЯ KAFKA
  Kafka — брокер. Может быть недоступен при старте
  или перезапущен.

  ПРОБЛЕМА:
    Sarama / segmentio/kafka-go — клиенты сами ретраят
    отправку. Но если подключиться к Kafka нельзя — падают.

  РЕШЕНИЕ:
    • Инициализация с retry.
    • При ошибках отправки — повтор.

  ПРИМЕР ДЛЯ KAFKA-GO WRITER:
    func kafkaWriter(ctx context.Context, brokers []string, topic string) (*kafka.Writer, error) {
        w := &kafka.Writer{
            Addr:     kafka.TCP(brokers...),
            Topic:    topic,
            Balancer: &kafka.LeastBytes{},
        }

        // Проверка через metadata-запрос.
        for attempt := 0; attempt < 10; attempt++ {
            conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
            if err == nil {
                conn.Close()
                return w, nil
            }

            delay := time.Duration(rand.Int63n(int64(
                time.Second * time.Duration(1<<uint(attempt)))))
            if delay > 30*time.Second {
                delay = 30 * time.Second
            }

            log.Printf("kafka not ready, retry in %v: %v", delay, err)
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }

        return nil, errors.New("kafka unreachable")
    }

  ПРИ ОТПРАВКЕ:
    Writer сам ретраит. Но если хочешь контролировать — оберни
    WriteMessages в свой retry.

    ВАЖНО: у Kafka есть семантика at-least-once. Продюсер
    может отправить одно сообщение дважды. Consumer должен
    быть идемпотентным.

  8. RETRY ДЛЯ HTTP-ЗАПРОСОВ
  Внешние API. Stripe, Twilio, любые REST-сервисы.

  КОГДА РЕТРАИТЬ:
    • 5xx — серверная ошибка. Retry.
    • 429 — rate limit. Retry с Retry-After.
    • 408 — timeout. Retry.
    • Сетевые ошибки. Retry.

  КОГДА НЕ РЕТРАИТЬ:
    • 4xx кроме 408, 429. Ошибка клиента, повтор не поможет.
    • Неидемпотентные операции без idempotency key.

  ПРИМЕР:
    func doWithRetry(ctx context.Context, client *http.Client,
        req *http.Request) (*http.Response, error) {

        var lastErr error
        for attempt := 0; attempt < 5; attempt++ {
            if attempt > 0 {
                delay := time.Duration(rand.Int63n(int64(
                    time.Second * time.Duration(1<<uint(attempt)))))
                if delay > 30*time.Second {
                    delay = 30 * time.Second
                }
                select {
                case <-time.After(delay):
                case <-ctx.Done():
                    return nil, ctx.Err()
                }
            }

            resp, err := client.Do(req.Clone(ctx))
            if err != nil {
                lastErr = err
                continue
            }

            if resp.StatusCode >= 500 {
                resp.Body.Close()
                lastErr = fmt.Errorf("status: %d", resp.StatusCode)
                continue
            }

            if resp.StatusCode == http.StatusTooManyRequests {
                resp.Body.Close()
                // Уважай Retry-After.
                if ra := resp.Header.Get("Retry-After"); ra != "" {
                    if sec, err := strconv.Atoi(ra); err == nil {
                        time.Sleep(time.Duration(sec) * time.Second)
                    }
                }
                lastErr = errors.New("rate limited")
                continue
            }

            return resp, nil
        }

        return nil, lastErr
    }

  ВАЖНО:
    • Клонируй запрос через req.Clone(ctx). Тело запроса нельзя
      использовать дважды.
    • Уважай Retry-After для 429.
    • Ограничивай общее время через ctx.

  9. ОБЩАЯ БИБЛИОТЕКА RETRY
  Для повторяющихся паттернов — вынеси в библиотеку.

  ПРОСТАЯ РЕАЛИЗАЦИЯ:
    type RetryConfig struct {
        MaxAttempts int
        InitialDelay time.Duration
        MaxDelay    time.Duration
    }

    func Do(ctx context.Context, cfg RetryConfig, fn func() error) error {
        var lastErr error

        for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
            if err := ctx.Err(); err != nil {
                return err
            }

            err := fn()
            if err == nil {
                return nil
            }
            lastErr = err

            if attempt == cfg.MaxAttempts-1 {
                break
            }

            base := cfg.InitialDelay * time.Duration(1<<uint(attempt))
            if base > cfg.MaxDelay {
                base = cfg.MaxDelay
            }
            delay := time.Duration(rand.Int63n(int64(base)))

            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        return fmt.Errorf("after %d attempts: %w", cfg.MaxAttempts, lastErr)
    }

  ИСПОЛЬЗОВАНИЕ:
    err := Do(ctx, RetryConfig{
        MaxAttempts:  5,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     30 * time.Second,
    }, func() error {
        return db.PingContext(ctx)
    })

  ГОТОВЫЕ БИБЛИОТЕКИ:
    • github.com/cenkalti/backoff/v4 — самая популярная.
    • github.com/avast/retry-go — простая.
    • github.com/sethvargo/go-retry — минималистичная.

  РЕКОМЕНДАЦИЯ: github.com/cenkalti/backoff/v4.

    Exponential backoff, jitter, контекст, интеграция с
    retryablehttp.

  10. КОНТЕКСТ И ОТМЕНА
  Retry ДОЛЖЕН уважать контекст. Иначе при shutdown сервис
  висит в retry-цикле, не давая процессу завершиться.

  ПЛОХО:
    for {
        err := connect()
        if err == nil {
            return nil
        }
        time.Sleep(delay)
    }

    Вечный цикл. Не реагирует на SIGTERM.

  ХОРОШО:
    for {
        err := connect()
        if err == nil {
            return nil
        }
        select {
        case <-time.After(delay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    При отмене ctx — выходит из retry.

  СВЯЗЬ С SHUTDOWN:
    signal.NotifyContext → ctx отменяется
    → retry выходит
    → приложение завершается
    → Compose видит exit
    → контейнер останавливается чисто

  БЕЗ ЭТОГО:
    SIGTERM приходит.
    Retry продолжает попытки.
    stop_grace_period истекает.
    SIGKILL.
  ПРАВИЛО: любой retry-цикл должен слушать ctx.Done().

  11. WAIT-FOR-IT.SH И ЕГО МЕСТО
  До появления depends_on с condition: service_healthy
  использовали скрипт wait-for-it.sh.

  ЧТО ДЕЛАЕТ:
    Ждёт, пока TCP-порт сервиса станет доступен.
    Потом запускает основную команду.

  ПРИМЕР:
    #!/bin/bash
    host="$1"
    port="$2"
    shift 2

    until nc -z "$host" "$port"; do
      echo "waiting for $host:$port..."
      sleep 1
    done

    exec "$@"

  ИСПОЛЬЗОВАНИЕ В COMPOSE:
    services:
      app:
        command: ["/wait-for-it.sh", "postgres:5432", "--", "/app"]

  ПРОБЛЕМЫ:
    • Проверяет только TCP-порт, не готовность сервиса.
    • Требует nc, bash, sh — не работает на distroless.
    • Не учитывает shutdown.
    • Устаревает — depends_on с condition лучше.

  КОГДА ВСЁ-ТАКИ НУЖЕН:
    • Compose v1 (устаревший).
    • Проверка чего-то, что не покрывается healthcheck.
    • Редкие случаи, когда другой механизм не работает.

  ПРАВИЛО: для новых проектов — depends_on с
  condition: service_healthy. Retry в коде — для runtime.

  12. ИДЕМПОТЕНТНОСТЬ ПРИ RETRY
  Если операция неидемпотентная — retry может выполнить
  её дважды.

  ПРИМЕР:
    POST /payments — списать 100 рублей.
    Запрос ушёл, ответ потерялся.
    Retry → списали 200.

  РЕШЕНИЕ: idempotency key.

  РЕАЛИЗАЦИЯ:
    Клиент:
      POST /payments
      Idempotency-Key: <uuid>

    Сервер:
      1. Проверить ключ в Redis/БД.
      2. Если есть — вернуть прошлый ответ.
      3. Если нет — выполнить, сохранить результат с ключом.

  В GO:
    func (s *Service) Charge(ctx context.Context,
        idempotencyKey string, amount float64) (*ChargeResponse, error) {

        // Проверка.
        if cached, err := s.redis.Get(ctx,
            "idem:"+idempotencyKey).Result(); err == nil {
            var resp ChargeResponse
            json.Unmarshal([]byte(cached), &resp)
            return &resp, nil
        }

        // Выполняем.
        resp, err := s.doCharge(ctx, amount)
        if err != nil {
            return nil, err
        }

        // Сохраняем результат с TTL.
        data, _ := json.Marshal(resp)
        s.redis.Set(ctx, "idem:"+idempotencyKey, data, 24*time.Hour)

        return resp, nil
    }

  ПРАВИЛО: для неидемпотентных операций с retry —
  обязателен idempotency key.

  13. СВЯЗЬ С CIRCUIT BREAKER
  Retry и circuit breaker работают вместе.

  RETRY:
    Повторяет временные ошибки.

  CIRCUIT BREAKER:
    Прекращает слать запросы, если сервис «известно-плохой».

  СХЕМА:
    [app] → [circuit breaker] → [retry] → [http client]

  Если сервис упал:
    1. Retry пытается N раз — все падают.
    2. Circuit breaker открывается.
    3. Следующие запросы не идут в retry — сразу ошибка.
    4. Через timeout CB переходит в half-open.
    5. Один запрос идёт в retry. Если успех — CB закрыт.

  БЕЗ CIRCUIT BREAKER:
    Retry долбит упавший сервис. Каждый клиент шлёт
    5 попыток. Нагрузка умножается.

  С CIRCUIT BREAKER:
    После N ошибок CB открыт. Запросы падают мгновенно.
    Сервис получает передышку.

  ГОТОВЫЕ БИБЛИОТЕКИ:
    • github.com/sony/gobreaker — circuit breaker.
    • Вместе с cenkalti/backoff — полный стек.

  14. АНТИПАТТЕРНЫ
  14.1. RETRY БЕЗ BACKOFF.
    Мгновенные повторы. Добивают сервис.
  14.2. RETRY БЕЗ JITTER.
    Thundering herd. Все клиенты бьют одновременно.
  14.3. RETRY НЕИДЕМПОТЕНТНЫХ ОПЕРАЦИЙ.
    POST без idempotency key. Двойное списание.
  14.4. ВЕЧНЫЙ RETRY.
    Без максимального числа попыток. Сервис никогда не
    сдаётся.
  14.5. RETRY БЕЗ КОНТЕКСТА.
    Не реагирует на SIGTERM. Сервис висит при shutdown.
  14.6. RETRY НА ВСЕ ОШИБКИ.
    Повторяет 400, 401, 404. Бессмысленно и вредно.
  14.7. RETRY БЕЗ ЛОГИРОВАНИЯ.
    Не видно, что происходит. Отладка невозможна.
  14.8. WAIT-FOR-IT.SH ВМЕСТО CONDITION.
    Устаревший подход. depends_on с condition лучше.
  14.9. RETRY + CIRCUIT BREAKER НЕ ВМЕСТЕ.
    Только retry — сервис добивается. Только CB — нет
    толерантности к временным сбоям.
  14.10. RETRY СЛИШКОМ АГРЕССИВНЫЙ.
    100 попыток за секунду. Не даёт сервису подняться.
  14.11. BACKOFF БЕЗ МАКСИМУМА.
    delay растёт до минут. Клиент висит.
  14.12. RETRY КАЖДОГО ЗАПРОСА.
    Каждый HTTP-запрос ретраится 5 раз. При нагрузке
    умножает трафик в 5 раз.

  15. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  Healthcheck + depends_on не панацея. Нужен retry в коде
      для runtime-проблем.
  2.  Retry для временных ошибок: connection refused, 5xx,
      429, timeout.
  3.  Retry НЕ для бизнес-ошибок: 400, 401, 404, 422.
  4.  Exponential backoff: delay = initial * 2^attempt.
      Максимум — 30s.
  5.  Full jitter обязателен. delay = random(0, base).
      Без jitter — thundering herd.
  6.  Retry для подключения к БД, Kafka, Redis, HTTP.
  7.  Контекст обязателен. Retry должен реагировать на SIGTERM.
  8.  Idempotency key для неидемпотентных операций с retry.
  9.  wait-for-it.sh устарел. depends_on с condition:
      service_healthy — стандарт.
  10. Retry + circuit breaker вместе. Один защищает от
      временных сбоев, другой — от постоянных.
      свой велосипед.
  12. Логируй каждую попытку. Иначе отладка невозможна.
  13. Антипаттерны: retry без backoff, без jitter, вечный
      retry, без контекста, на все ошибки, без логов,
      слишком агрессивный.
*/
