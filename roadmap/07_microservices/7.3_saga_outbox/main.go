package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

/*
  УРОК 8.3: SAGA И OUTBOX
  СОДЕРЖАНИЕ:
    1.  Проблема распределённых транзакций (детально)
    2.  Паттерн Saga — концепция, история, виды
    3.  Choreography vs Orchestration — глубокое сравнение
    4.  Компенсирующие транзакции — паттерны и подводные камни
    5.  Saga на практике: паттерны реализации
    6.  Transactional Outbox — полный разбор
    7.  Реализация Outbox: Polling vs CDC vs LISTEN/NOTIFY
    8.  Идемпотентность и Idempotency Keys — глубоко
    9.  Гарантии доставки и семантика в Saga
    10. Мониторинг, отладка и troubleshooting
    11. Паттерны тестирования Saga
    12. Антипаттерны и как их избежать
    13. Ключевые выводы для собеседования

  1.  ПРОБЛЕМА РАСПРЕДЕЛЁННЫХ ТРАНЗАКЦИЙ (ДЕТАЛЬНО)
  1.1. ACID В МОНОЛИТЕ
    Классическая транзакция в одной БД:
      BEGIN;
      UPDATE accounts SET balance = balance - 100 WHERE id = 1;
      UPDATE accounts SET balance = balance + 100 WHERE id = 2;
      COMMIT;

    Гарантии ACID:
      • Atomicity — всё или ничего.
      • Consistency — переход из одного валидного состояния в другое.
      • Isolation — параллельные транзакции не мешают друг другу.
      • Durability — после COMMIT данные сохранены.

  1.2. ЧТО МЕНЯЕТСЯ В МИКРОСЕРВИСАХ
    В микросервисной архитектуре данные распределены:
      • Сервис A → БД A (PostgreSQL)
      • Сервис B → БД B (MongoDB)
      • Сервис C → БД C (Redis)

    Транзакция, охватывающая несколько сервисов, — РАСПРЕДЕЛЁННАЯ.
    ACID не работает без 2PC. А 2PC не подходит (см. ниже).

  1.3. ПОЧЕМУ 2PC (TWO-PHASE COMMIT) НЕ ПОДХОДИТ
    2PC — классический протокол для распределённых транзакций.

    ФАЗЫ:
      1. PREPARE — координатор спрашивает всех участников: "Готовы?".
      2. COMMIT/ROLLBACK — если все готовы → COMMIT, иначе → ROLLBACK.

    ПРОБЛЕМЫ:
    • БЛОКИРОВКИ НА ВСЁ ВРЕМЯ ТРАНЗАКЦИИ:
      Участники блокируют ресурсы (строки, таблицы) с момента PREPARE
      до COMMIT. Если координатор завис, блокировки держатся часами.
    • ЕДИНАЯ ТОЧКА ОТКАЗА:
      Если координатор упал после PREPARE, но до COMMIT — участники
      остаются в подвешенном состоянии (in-doubt transaction).
      Требуется ручное вмешательство.
    • НЕ МАСШТАБИРУЕТСЯ:
      Каждый участник должен поддерживать 2PC. Многие NoSQL и облачные
      БД не поддерживают.
    • НИЗКАЯ ПРОИЗВОДИТЕЛЬНОСТЬ:
      2PC требует 2 сетевых round-trip'а. Это в разы медленнее
      локальных транзакций.
    • ХРУПКОСТЬ:
      Любая ошибка сети или сбой участника → вся транзакция зависает.
    ВЫВОД: 2PC — антипаттерн для современных микросервисов.

  1.4. АЛЬТЕРНАТИВЫ 2PC
    • Saga — последовательность локальных транзакций с компенсациями.
    • TCC (Try-Confirm-Cancel) — резервирование ресурсов, подтверждение
      или отмена.
    • Event Sourcing — сохранение всех событий вместо состояния.
    • Outbox — надёжная публикация событий.

  2.  ПАТТЕРН SAGA — КОНЦЕПЦИЯ, ИСТОРИЯ, ВИДЫ
  2.1. ИСТОРИЯ ПАТТЕРНА
    Паттерн Saga был впервые описан в 1987 году в статье Hector Garcia-Molina
    и Kenneth Salem "Sagas" (ACM SIGMOD). Изначально он использовался в
    контексте длительных транзакций в БД.

    В 2010-х, с ростом микросервисов, паттерн переосмыслили для
    распределённых систем: Chris Richardson (microservices.io) популяризировал
    Saga как способ управления распределёнными транзакциями.

  2.2. ФОРМАЛЬНОЕ ОПРЕДЕЛЕНИЕ
    Saga — это последовательность локальных транзакций T1, T2, ..., Tn,
    где каждая Ti выполняется в одном сервисе. Для каждой Ti определена
    компенсирующая транзакция Ci. Если Ti падает, выполняются
    Cn-1, ..., C2, C1 в обратном порядке.

    Свойства Saga:
      • S = {T1, T2, ..., Tn, C1, C2, ..., Cn} — множество транзакций.
      • Либо все Ti выполняются успешно.
      • Либо для некоторого i выполняется Ci, и тогда выполняются
        C_{i-1}, ..., C1.

  2.3. ТИПЫ ТРАНЗАКЦИЙ В SAGA
    a) COMPENSABLE (компенсируемые)
       Могут быть отменены компенсирующей транзакцией.
       Пример: создать заказ → отменить заказ.

    b) PIVOT (поворотные)
       Точка невозврата. После успешного выполнения PIVOT-транзакции
       предыдущие шаги больше не нужно компенсировать, даже если
       последующие шаги упадут.
       Пример: подтвердить платёж (деньги уже списаны).

    c) RETRIABLE (повторяемые)
       Идут после PIVOT. Всегда выполняются до успеха (повторяются
       при сбоях). Компенсации для них не нужны.
       Пример: отправить email, обновить read-model.

    d) IRREVERSIBLE (необратимые)
       Не могут быть отменены.
       Пример: отправить sms. Решение: выполнять после PIVOT.

  2.4. АКСИОМА: "SEMANTIC LOCK"
    Ключевая идея Saga: вместо технических блокировок (2PC) мы
    используем "семантические блокировки" на уровне приложения.

    Пример:
      • 2PC: заблокировать строку в БД до конца транзакции.
      • Saga: пометить заказ как "PENDING" (семантическая блокировка),
        затем после успеха → "CONFIRMED", после сбоя → "CANCELLED".

  3.  CHOREOGRAPHY VS ORCHESTRATION — ГЛУБОКОЕ СРАВНЕНИЕ

  3.1. CHOREOGRAPHY — ДЕЦЕНТРАЛИЗОВАННЫЙ ПОДХОД
    АРХИТЕКТУРА:
      ┌──────────────┐   event "OrderCreated"    ┌──────────────┐
      │ OrderService │──────────────────────────▶│Kafka / broker│
      └──────────────┘                            └──────┬───────┘
                                                         │
                                          ┌──────────────┼──────────────┐
                                          ▼              ▼              ▼
                                    ┌──────────┐  ┌──────────┐  ┌──────────┐
                                    │ Payment  │  │ Stock    │  │ Shipping │
                                    │ Service  │  │ Service  │  │ Service  │
                                    └────┬─────┘  └────┬─────┘  └────┬─────┘
                                         │             │             │
                                    event "PaymentDone" event "StockReserved"
                                         │             │             │
                                         └─────────────┴─────────────┘
                                                       │
                                                       ▼
                                          (cascading events)

    ПОТОК:
      1. OrderService → публикует "OrderCreated".
      2. PaymentService слушает → списывает деньги → "PaymentProcessed".
      3. StockService слушает "PaymentProcessed" → резервирует товар.
      4. ShippingService слушает "StockReserved" → создаёт доставку.
      5. Если StockService падает → публикует "StockReservationFailed".
      6. PaymentService слушает → возвращает деньги.
      7. OrderService слушает → отменяет заказ.

    ПЛЮСЫ:
      + Нет центрального координатора → нет единой точки отказа.
      + Слабая связанность: каждый сервис знает только события.
      + Просто масштабировать: добавить новый сервис → просто подписаться.

    МИНУСЫ:
      - Сложно отследить поток (не видно всей картины).
      - Неявное распределённое состояние.
      - Компенсации "размазаны" по сервисам.
      - Сложно тестировать end-to-end.
      - Циклические зависимости: сервис A слушает B, B слушает A.

  3.2. ORCHESTRATION — ЦЕНТРАЛИЗОВАННЫЙ ПОДХОД
    АРХИТЕКТУРА:
                            ┌──────────────────┐
                            │   Orchestrator   │
                            │ (state machine)  │
                            └────────┬─────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │ Command        │ Command        │ Command
                    ▼                ▼                ▼
              ┌──────────┐     ┌──────────┐     ┌──────────┐
              │ Payment  │     │ Stock    │     │ Shipping │
              │ Service  │     │ Service  │     │ Service  │
              └────┬─────┘     └────┬─────┘     └────┬─────┘
                   │                │                │
                   └────────────────┴────────────────┘
                                     │ Result / Event
                                     ▼
                            ┌──────────────────┐
                            │   Orchestrator   │
                            │   next step      │
                            └──────────────────┘

    ПОТОК:
      1. Orchestrator получает "Начать оформление заказа".
      2. Orchestrator → "Выполни платёж" → PaymentService.
      3. PaymentService → "Платёж проведён" → Orchestrator.
      4. Orchestrator → "Зарезервируй товар" → StockService.
      5. StockService → "Резерв сделан" → Orchestrator.
      6. Если что-то падает → Orchestrator управляет компенсациями.

    ПЛЮСЫ:
      + Вся логика в одном месте — видно workflow.
      + Легче отлаживать и тестировать.
      + Центральный обзор состояния.
      + Легко добавлять таймауты, retry, отмену.
      + Работает с длительными процессами (дни, недели).

    МИНУСЫ:
      - Orchestrator — потенциальная единая точка отказа.
      - Может превратиться в "god service", если не следить.
      - Дополнительная связанность (все сервисы знают оркестратор).

  3.3. ГИБРИДНЫЙ ПОДХОД
    В реальных системах часто комбинируют оба подхода:
      • Orchestration для критичных потоков (платежи).
      • Choreography для остального (уведомления, аналитика).

  3.4. ТАБЛИЦА СРАВНЕНИЯ (ПОЛНАЯ)
    ┌─────────────────────┬──────────────────────┬────────────────────────────┐
    │ Аспект              │ Choreography         │ Orchestration              │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Управление          │ События              │ Central state machine      │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Связанность         │ Слабая               │ Средняя                    │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Точка отказа        │ Нет единой           │ Есть (оркестратор)         │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Видимость потока    │ Сложно               │ Легко                      │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Тестирование        │ Сложно               │ Легко                      │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Таймауты            │ Per-service          │ Централизованные           │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Длительные процессы │ Плохо                │ Хорошо                     │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Масштабирование     │ Легко                │ Требует HA orchestrator    │
    ├─────────────────────┼──────────────────────┼────────────────────────────┤
    │ Когда использовать  │ Простые потоки 2-5   │ Сложные потоки > 5 шагов,  │
    │                     │ шагов                │ таймауты, retry, условия   │
    └─────────────────────┴──────────────────────┴────────────────────────────┘

  4.  КОМПЕНСИРУЮЩИЕ ТРАНЗАКЦИИ — ПАТТЕРНЫ И ПОДВОДНЫЕ КАМНИ

  4.1. ЧТО ТАКОЕ КОМПЕНСАЦИЯ
    Компенсирующая транзакция — это БИЗНЕС-ОПЕРАЦИЯ, которая семантически
    отменяет эффект предыдущей операции. Это НЕ rollback БД.

    ПРИМЕР:
      Forward: "Списать 100 рублей с карты"
      Compensation: "Вернуть 100 рублей на карту" (refund)

     НЕЛЬЗЯ сделать rollback: транзакция уже закоммичена.
     НУЖНО сделать новую операцию с противоположным эффектом.

  4.2. СВОЙСТВА КОМПЕНСАЦИИ
    a) ИДЕМПОТЕНТНОСТЬ
       Повторное выполнение не должно менять результат.
       Пример: "Вернуть 100 рублей" — если вызвать дважды, вернётся 200?
       НЕТ! Должно вернуться 100. Используем idempotency key.

    b) КОММУТАТИВНОСТЬ (в некоторых случаях)
       Порядок компенсаций может быть важен (LIFO — Last In First Out).

    c) НАДЁЖНОСТЬ
       Компенсация сама может упасть. Нужны retry + DLQ.

  4.3. ПАТТЕРНЫ КОМПЕНСАЦИИ
    a) FORWARD RECOVERY (движение вперёд)
       Вместо отката пытаемся дойти до конца, повторяя шаги.
       Пример: retry платежа до успеха.
       Используется для retriable-транзакций.
    b) BACKWARD RECOVERY (движение назад)
       Откатываем предыдущие шаги в обратном порядке.
       Используется для compensable-транзакций.
    c) MIXED RECOVERY
       Комбинация: compensation до PIVOT, forward — после.

  4.4. ПОДВОДНЫЕ КАМНИ
       НЕКОМПЕНСИРУЕМЫЕ ОПЕРАЦИИ
       Отправка email нельзя "отменить". Решение: делать после PIVOT
       или использовать "корректирующие" операции (извинительное письмо).

       ЧАСТИЧНАЯ КОМПЕНСАЦИЯ
       Если компенсация упала на середине — система несогласована.
       Нужен retry + ручное вмешательство.

       КАСКАДНЫЕ КОМПЕНСАЦИИ
       Одна компенсация может вызвать компенсацию в другом сервисе.
       Нужно контролировать глубину.

       БИЗНЕС-ЛОГИКА В КОМПЕНСАЦИИ
       Компенсация — не технический откат, а БИЗНЕС-ОПЕРАЦИЯ.
       Требует согласования с бизнесом.

  5.  SAGA НА ПРАКТИКЕ: ПАТТЕРНЫ РЕАЛИЗАЦИИ

  5.1. SAGA LOG (ЖУРНАЛ САГИ)
    Все шаги и их результаты сохраняются в отдельную таблицу saga_log.

    CREATE TABLE saga_log (
        saga_id UUID NOT NULL,
        step INT NOT NULL,
        step_name VARCHAR(255) NOT NULL,
        status VARCHAR(50) NOT NULL,  -- PENDING, SUCCESS, FAILED, COMPENSATED
        payload JSONB,
        created_at TIMESTAMP DEFAULT NOW(),
        PRIMARY KEY (saga_id, step)
    );

    ПЛЮСЫ:
      + Можно восстановить состояние после сбоя.
      + Легко отлаживать.
      + Аудит всех операций.

  5.2. STATE MACHINE ДЛЯ ORCHESTRATOR
    Оркестратор — это конечный автомат (state machine):
      • Состояния: INIT, PAYMENT_PENDING, PAYMENT_DONE, STOCK_PENDING, ...
      • Переходы: по событиям или командам.
      • Persist state в БД (например, saga_state).

  5.3. TIMEOUTS И DEADLINES
    Каждый шаг саги имеет таймаут. Если шаг не завершился за N секунд:
      • Отправить команду компенсации.
      • Пометить сагу как FAILED.

  5.4. ИДЕМПОТЕНТНОСТЬ ВСЕХ ШАГОВ
    Каждый шаг и каждая компенсация должны быть идемпотентными.
    Используем idempotency keys.

  5.5. КОРРЕЛЯЦИЯ
    Все события и команды содержат saga_id (correlation_id).
    Это позволяет:
      • Проследить весь поток по логам.
      • Связать события разных сервисов.
      • Использовать в Distributed Tracing.

  6.  TRANSACTIONAL OUTBOX — ПОЛНЫЙ РАЗБОР

  6.1. ПРОБЛЕМА DUAL-WRITE
    После выполнения локальной транзакции нужно опубликовать событие в Kafka.
    Но это ДВЕ РАЗНЫЕ операции в разных системах.
    Возможные исходы:
      • Успех БД + Успех Kafka → всё хорошо.
      • Успех БД + Ошибка Kafka → событие потеряно.
      • Ошибка БД + Успех Kafka → фантомное событие.
      • Ошибка БД + Ошибка Kafka → всё откатывается.
    Мы не можем гарантировать атомарность между БД и Kafka.

  6.2. РЕШЕНИЕ: OUTBOX
    Записываем событие в таблицу outbox В ТОЙ ЖЕ ТРАНЗАКЦИИ, что и
    бизнес-данные. Затем отдельный воркер (relay) читает outbox и
    отправляет события в Kafka.

    ПРЕИМУЩЕСТВА:
      • Атомарность: либо и данные, и событие сохранены, либо ничего.
      • Надёжность: событие не потеряется при сбое Kafka.
      • At-least-once: событие будет отправлено минимум один раз.

  6.3. СХЕМА ТАБЛИЦЫ OUTBOX (ПОЛНАЯ)
    CREATE TABLE outbox (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        -- Метаданные для маршрутизации
        aggregate_type VARCHAR(255) NOT NULL,
        aggregate_id VARCHAR(255) NOT NULL,
        event_type VARCHAR(255) NOT NULL,
        -- Полезная нагрузка
        payload JSONB NOT NULL,
        headers JSONB,
        -- Метаданные для отслеживания
        event_id UUID UNIQUE NOT NULL,
        correlation_id UUID,
        trace_id VARCHAR(255),
        -- Управление обработкой
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        processed_at TIMESTAMP,
        retry_count INT DEFAULT 0,
        last_error TEXT,
        next_retry_at TIMESTAMP
    );
    -- Индекс для быстрого поиска необработанных
    CREATE INDEX idx_outbox_unprocessed
        ON outbox (created_at)
        WHERE processed_at IS NULL;

  6.4. ПОЛНЫЙ ЦИКЛ РАБОТЫ
    1. Бизнес-транзакция:
       BEGIN;
         INSERT INTO orders (...) VALUES (...);
         INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
         VALUES ('order', '123', 'OrderCreated', '{...}');
       COMMIT;

    2. Relay (воркер):
       BEGIN;
         SELECT * FROM outbox
         WHERE processed_at IS NULL
         ORDER BY created_at
         LIMIT 100
         FOR UPDATE SKIP LOCKED;

         -- Для каждого события:
         sendToKafka(event);

         UPDATE outbox SET processed_at = NOW() WHERE id = event.id;
       COMMIT;

  6.5. ГАРАНТИИ И ОГРАНИЧЕНИЯ
    • At-least-once: событие будет отправлено как минимум один раз.
    • Возможны дубли: если relay упал после sendToKafka, но до UPDATE,
      событие отправится повторно.
    • Решение: идемпотентный продюсер + уникальные event_id.

  7.  РЕАЛИЗАЦИЯ OUTBOX: POLLING VS CDC VS LISTEN/NOTIFY

  7.1. POLLING (РЕЛЕЙ НА GO)
    Классический подход: relay периодически опрашивает таблицу outbox.

    ПРИМЕР SQL:
      SELECT * FROM outbox
      WHERE processed_at IS NULL
      ORDER BY created_at ASC
      LIMIT 100
      FOR UPDATE SKIP LOCKED;

    КЛЮЧЕВОЙ ЭЛЕМЕНТ: FOR UPDATE SKIP LOCKED
      • FOR UPDATE — блокирует выбранные строки.
      • SKIP LOCKED — пропускает уже заблокированные.
      • Гарантирует, что несколько relay не обработают одни и те же записи.

    ПЛЮСЫ:
      + Простота.
      + Работает с любой БД.
      + Полный контроль.

    МИНУСЫ:
      - Нагрузка на БД (постоянные SELECT).
      - Задержка (polling interval 1-5 сек).
      - Нужно синхронизировать relay.

    ОПТИМИЗАЦИЯ:
      • Использовать индекс на processed_at.
      • Настроить интервал опроса.
      • Batch size 100-1000.

  7.2. CDC (DEBEZIUM)
    Debezium читает WAL PostgreSQL и отправляет изменения в Kafka напрямую.

    АРХИТЕКТУРА:
      App → PostgreSQL WAL (outbox) → Debezium → Kafka Connect → Kafka

    ПЛЮСЫ:
      + Минимальная задержка (< 100 мс).
      + Нет нагрузки на БД (читает WAL).
      + Не требует кода на Go.

    МИНУСЫ:
      - Сложная настройка (Kafka Connect, Debezium).
      - Дополнительная инфраструктура.
      - Схема таблицы должна быть стабильной.

  7.3. LISTEN/NOTIFY (POSTGRESQL)
    PostgreSQL поддерживает механизм NOTIFY/LISTEN для pub-sub.

    ПОДХОД:
      1. Триггер AFTER INSERT ON outbox вызывает NOTIFY.
      2. Relay слушает через LISTEN.
      3. При получении уведомления — читает outbox.

    ПЛЮСЫ:
      + Реальное время (задержка < 10 мс).
      + Без polling — нет нагрузки.

    МИНУСЫ:
      - Работает только с PostgreSQL.
      - Требует триггеров.
      - NOTIFY не работает при репликации (streaming replication).

  7.4. СРАВНЕНИЕ
    ┌─────────────────────┬──────────────┬──────────────┬──────────────┐
    │ Аспект              │ Polling      │ CDC          │ LISTEN       │
    ├─────────────────────┼──────────────┼──────────────┼──────────────┤
    │ Задержка            │ 1-5 сек      │ < 100 мс     │ < 10 мс      │
    ├─────────────────────┼──────────────┼──────────────┼──────────────┤
    │ Нагрузка на БД      │ Высокая      │ Низкая       │ Низкая       │
    ├─────────────────────┼──────────────┼──────────────┼──────────────┤
    │ Сложность           │ Низкая       │ Высокая      │ Средняя      │
    ├─────────────────────┼──────────────┼──────────────┼──────────────┤
    │ Работа с любой БД   │ Да           │ Зависит      │ Только PG    │
    └─────────────────────┴──────────────┴──────────────┴──────────────┘

  8.  ИДЕМПОТЕНТНОСТЬ И IDEMPOTENCY KEYS

  8.1. ЧТО ТАКОЕ ИДЕМПОТЕНТНОСТЬ
    Операция идемпотентна, если её повторное выполнение даёт тот же
    результат, что и однократное.

    ПРИМЕРЫ:
      • Идемпотентные: "Установить баланс = 100" (SET).
      • НЕ идемпотентные: "Увеличить баланс на 10" (INCREMENT).

  8.2. ПОЧЕМУ ИДЕМПОТЕНТНОСТЬ ВАЖНА
    В распределённых системах невозможно гарантировать exactly-once
    доставку. Можно только at-least-once. Значит, консюмер получит
    сообщение МИНИМУМ один раз, но может получить несколько раз.

    Чтобы достичь эффекта exactly-once, консюмер должен быть
    ИДЕМПОТЕНТНЫМ.

  8.3. IDEMPOTENCY KEY
    Idempotency key — уникальный идентификатор операции. При получении
    сообщения с уже обработанным ключом → игнорируем.

    РЕАЛИЗАЦИЯ:
      1. У каждого события есть уникальный event_id.
      2. Консюмер перед обработкой проверяет: есть ли event_id в таблице
         processed_events.
      3. Если нет → обработать + сохранить event_id (в одной транзакции).
      4. Если есть → пропустить.

    СХЕМА ТАБЛИЦЫ:
      CREATE TABLE processed_events (
          event_id UUID PRIMARY KEY,
          processed_at TIMESTAMP DEFAULT NOW(),
          consumer_group VARCHAR(255)
      );

  8.4. ТИПЫ IDEMPOTENCY KEYS
    a) UUIDv4 — случайный. Требует хранения всех ключей.
       Плюс: не нужен центральный генератор.
       Минус: БД растёт бесконечно (нужна очистка).

    b) UUIDv7 / ULID — содержит timestamp.
       Плюс: можно удалять старые ключи (по времени).
       Минус: требует поддержки генерации.

    c) MONOTONIC SEQUENCE — монотонно возрастающий.
       Плюс: хранить только последний.
       Минус: требует центрального генератора.

    d) NATURAL KEY — естественный ключ (например, order_id + version).
       Плюс: не требует хранения.
       Минус: подходит только для конкретных сценариев.

  8.5. ПАТТЕРН OUTBOX + IDEMPOTENCY
    Комбинация:
      1. Продюсер пишет событие в outbox с уникальным event_id.
      2. Relay отправляет в Kafka с этим event_id.
      3. Консюмер при получении проверяет event_id в processed_events.
      4. Если новое → обработать.
      5. Если уже было → пропустить.
    Это даёт end-to-end exactly-once.

  9.  ГАРАНТИИ ДОСТАВКИ И СЕМАНТИКА В SAGA

  9.1. ГАРАНТИИ В РАЗНЫХ ЧАСТЯХ САГИ
    • Бизнес-операция + запись в outbox: АТОМАРНО (ACID).
    • Relay → Kafka: AT-LEAST-ONCE.
    • Kafka → консюмер: AT-LEAST-ONCE.
    • Итог: at-least-once на всём пути + идемпотентность = exactly-once.

  9.2. ГАРАНТИИ В КОМПЕНСАЦИЯХ
    • Компенсации тоже могут падать → retry.
    • Если retry исчерпан → DLQ для ручного вмешательства.
    • Компенсации должны быть идемпотентными.

  9.3. SEMANTIC LOCK
    Вместо технических блокировок — семантические:
      • Статус "PENDING" — ресурс зарезервирован, но не подтверждён.
      • Статус "CONFIRMED" — операция завершена успешно.
      • Статус "CANCELLED" — операция отменена.
    Преимущество: нет блокировок БД, система масштабируется.

  10. МОНИТОРИНГ, ОТЛАДКА И TROUBLESHOOTING

  10.1. КЛЮЧЕВЫЕ МЕТРИКИ
    • SAGA_STARTED — количество начатых саг.
    • SAGA_COMPLETED — успешно завершённых.
    • SAGA_FAILED — упавших.
    • SAGA_COMPENSATED — откатанных.
    • SAGA_DURATION — время выполнения саги.
    • STEP_DURATION — время каждого шага.
    • OUTBOX_LAG — размер необработанного outbox.
    • OUTBOX_ERRORS — количество ошибок при отправке.

  10.2. ЛОГИРОВАНИЕ
    Обязательно логировать:
      • saga_id (correlation_id).
      • step (номер шага).
      • status (PENDING/SUCCESS/FAILED).
      • timestamp.
      • payload (для отладки).

  10.3. TRACING
    Использовать OpenTelemetry:
      • Span на всю сагу.
      • Child spans на каждый шаг.
      • Propagate trace_id через Kafka headers.

  10.4. TROUBLESHOOTING
    Частые проблемы:
      • Зависшие саги → нужен timeout + reconciliation job.
      • Пропущенные компенсации → audit log.
      • Дубли событий → idempotency.
      • Outbox переполнен → увеличить relay capacity.

  11. ПАТТЕРНЫ ТЕСТИРОВАНИЯ SAGA

  11.1. UNIT-ТЕСТЫ
    • Тестировать каждый шаг отдельно.
    • Мокать внешние вызовы.
    • Проверять идемпотентность.

  11.2. INTEGRATION-ТЕСТЫ
    • Поднимать Kafka, БД через Testcontainers.
    • Проверять полный поток саги.
    • Проверять компенсации.

  11.3. CHAOS ENGINEERING
    • Убивать сервисы во время саги.
    • Проверять, что компенсации выполняются.
    • Проверять восстановление.

  11.4. СИМУЛЯЦИЯ СБОЕВ
    • Проверить сбой после каждого шага.
    • Убедиться, что система консистентна.

  12. АНТИПАТТЕРНЫ И КАК ИХ ИЗБЕЖАТЬ

  12.1. GOD TOPIC
    Все сервисы слушают всё → хаос.
    РЕШЕНИЕ: Строгая иерархия топиков. Например:
      • order.created
      • order.paid
      • order.shipped

  12.2. SYNCHRONOUS SAGA
    Блокирующая HTTP-цепочка под видом "async".
    РЕШЕНИЕ: Только события или команды через брокер.

  12.3. MISSING COMPENSATION
    Forward-only потоки в денежных доменах.
    РЕШЕНИЕ: Все критичные шаги должны иметь компенсации.

  12.4. INFINITE RETRY
    Бесконечные повторы бизнес-ошибок.
    РЕШЕНИЕ: Retry с лимитом + DLQ.

  12.5. ORCHESTRATOR IN DB
    Хранимые процедуры управляют кросс-сервисным потоком.
    РЕШЕНИЕ: Оркестратор — отдельный сервис.

  12.6. NO IDEMPOTENCY
    Обработчики не идемпотентны → дубли.
    РЕШЕНИЕ: Idempotency keys для всех операций.

  12.7. SAGA WITHOUT TIMEOUT
    Саги могут висеть вечно.
    РЕШЕНИЕ: Timeout для каждого шага + reconciliation.

  13. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ

  1.  Saga — паттерн для распределённых транзакций без 2PC.
      Последовательность локальных транзакций + компенсации.
  2.  2PC не подходит из-за блокировок, единой точки отказа и плохой
      масштабируемости.
  3.  Два подхода: Choreography (децентрализованный) и Orchestration
      (централизованный).
  4.  Компенсирующие транзакции — бизнес-операции, а не rollback БД.
      Должны быть идемпотентными.
  5.  Типы транзакций в Saga: Compensable, Pivot, Retriable, Irreversible.
  6.  Transactional Outbox решает проблему dual-write между БД и Kafka.
      Запись в outbox в той же ACID-транзакции.
  7.  Реализация Outbox: Polling (просто), CDC (Debezium, быстро),
      LISTEN/NOTIFY (только PostgreSQL).
  8.  Идемпотентность — ключ к exactly-once в распределённых системах.
      Idempotency keys позволяют консюмерам распознавать дубликаты.
  9.  На собеседовании: «Saga — это последовательность локальных транзакций
      с компенсациями. Мы используем Choreography для простых потоков и
      Orchestration для сложных. Outbox гарантирует атомарность между БД и
      Kafka.»
  10. Не используйте 2PC. Не делайте forward-only потоков в денежных
      доменах. Всегда обрабатывайте сбои.
  11. Мониторьте размер outbox, lag, количество саг. Логируйте все шаги
      с correlation_id.
  12. Тестируйте компенсации. Chaos engineering — обязателен для
      критичных саг.
*/

const (
	dsn        = "postgres://app:app@localhost:5433/app?sslmode=disable"
	kafkaAddr  = "localhost:9092"
	kafkaTopic = "app.events"
)

// SCHEMA
const schema = `
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY, user_id TEXT, amount NUMERIC, status TEXT);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY, order_id UUID, amount NUMERIC, status TEXT);

CREATE TABLE IF NOT EXISTS stock_reservations (
    id UUID PRIMARY KEY, order_id UUID, sku TEXT, quantity INT, status TEXT);

CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ);
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (created_at) WHERE status IN ('PENDING','FAILED');

CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID NOT NULL,
    consumer_group TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, consumer_group));

CREATE TABLE IF NOT EXISTS saga_state (
    saga_id UUID PRIMARY KEY,
    saga_type TEXT NOT NULL,
    status TEXT NOT NULL,
    step INT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL,
    last_error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
`

// OUTBOX
type Event struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

// insertOutbox ВСЕГДА принимает tx — это и есть суть паттерна:
// запись в outbox атомарна с бизнес-операцией.
func insertOutbox(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.AggregateType, e.AggregateID, e.EventType, e.Payload)
	return err
}

// RELAY: outbox -> kafka
type Relay struct {
	pool *pgxpool.Pool
	send func(ctx context.Context, eventID uuid.UUID, key string, payload []byte) error
	log  *slog.Logger
}

func (r *Relay) Run(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = r.tick(ctx)
		}
	}
}

func (r *Relay) tick(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// ВАЖНО: берём и PENDING, и FAILED — иначе упавшее событие
	// застрянет навсегда. SKIP LOCKED — параллельные релеи не мешают.
	rows, err := tx.Query(ctx, `
		WITH batch AS (
			SELECT id FROM outbox
			WHERE status IN ('PENDING','FAILED')
			ORDER BY created_at
			LIMIT 100
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox o SET status='PROCESSING'
		FROM batch WHERE o.id = batch.id
		RETURNING o.id, o.aggregate_id, o.payload`)
	if err != nil {
		return err
	}

	type row struct {
		ID      uuid.UUID
		Key     string
		Payload []byte
	}
	var batch []row
	for rows.Next() {
		var b row
		if err := rows.Scan(&b.ID, &b.Key, &b.Payload); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, b)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	for _, b := range batch {
		if err := r.send(ctx, b.ID, b.Key, b.Payload); err != nil {
			r.log.Error("kafka send failed", "id", b.ID, "err", err)
			_, _ = r.pool.Exec(ctx, `
				UPDATE outbox SET status='FAILED', retry_count=retry_count+1,
				  last_error=$2 WHERE id=$1`, b.ID, err.Error())
			continue
		}
		r.log.Info("relay → kafka", "event_id", b.ID)
		_, _ = r.pool.Exec(ctx,
			`UPDATE outbox SET status='SENT', sent_at=NOW() WHERE id=$1`, b.ID)
	}
	return nil
}

// IDEMPOTENT CONSUMER
func consumeIdempotent(
	ctx context.Context, pool *pgxpool.Pool,
	group string, eventID uuid.UUID,
	business func(ctx context.Context, tx pgx.Tx) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, consumer_group)
		VALUES ($1,$2) ON CONFLICT DO NOTHING`, eventID, group)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // дубль, пропускаем
	}
	if err := business(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SAGA
type SagaStep struct {
	Name       string
	IsPivot    bool
	Forward    func(ctx context.Context, tx pgx.Tx, p []byte) error
	Compensate func(ctx context.Context, tx pgx.Tx, p []byte) error
}

type Orchestrator struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (o *Orchestrator) Run(ctx context.Context, sagaID uuid.UUID, steps []SagaStep) {
	for {
		done, err := o.tick(ctx, sagaID, steps)
		if err != nil {
			o.log.Error("tick", "saga", sagaID, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if done {
			return
		}
	}
}

func (o *Orchestrator) tick(ctx context.Context, id uuid.UUID, steps []SagaStep) (bool, error) {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var (
		status  string
		step    int
		payload []byte
	)
	err = tx.QueryRow(ctx, `
		SELECT status, step, payload FROM saga_state
		WHERE saga_id=$1 FOR UPDATE`, id).Scan(&status, &step, &payload)
	if err != nil {
		return true, err
	}

	switch status {
	case "COMPLETED", "COMPENSATED":
		return true, nil

	case "STARTED":
		if step >= len(steps) {
			_, _ = tx.Exec(ctx,
				`UPDATE saga_state SET status='COMPLETED' WHERE saga_id=$1`, id)
			o.log.Info("saga COMPLETED", "id", id)
			return true, tx.Commit(ctx)
		}
		s := steps[step]
		o.log.Info("saga → forward", "step", s.Name)
		if err := s.Forward(ctx, tx, payload); err != nil {
			o.log.Error("forward failed", "step", s.Name, "err", err)
			if hasPivot(steps, step) {
				// Pivot пройден → retry
				return false, tx.Commit(ctx)
			}
			// До Pivot → компенсация
			_, _ = tx.Exec(ctx, `
				UPDATE saga_state SET status='COMPENSATING', step=step-1,
				  last_error=$2 WHERE saga_id=$1`, id, err.Error())
			return false, tx.Commit(ctx)
		}
		_, _ = tx.Exec(ctx,
			`UPDATE saga_state SET step=step+1 WHERE saga_id=$1`, id)
		return false, tx.Commit(ctx)

	case "COMPENSATING":
		if step < 0 {
			_, _ = tx.Exec(ctx,
				`UPDATE saga_state SET status='COMPENSATED' WHERE saga_id=$1`, id)
			o.log.Info("saga COMPENSATED", "id", id)
			return true, tx.Commit(ctx)
		}
		s := steps[step]
		if s.Compensate != nil {
			o.log.Info("saga ← compensate", "step", s.Name)
			if err := s.Compensate(ctx, tx, payload); err != nil {
				o.log.Error("compensate failed", "step", s.Name, "err", err)
				return false, tx.Commit(ctx)
			}
		}
		_, _ = tx.Exec(ctx,
			`UPDATE saga_state SET step=step-1 WHERE saga_id=$1`, id)
		return false, tx.Commit(ctx)
	}
	return true, nil
}

func hasPivot(steps []SagaStep, upTo int) bool {
	for i := 0; i < upTo; i++ {
		if steps[i].IsPivot {
			return true
		}
	}
	return false
}

// ШАГИ САГИ ЗАКАЗА
// Каждый Forward/Compensate пишет событие в outbox.
func buildOrderSaga() []SagaStep {
	return []SagaStep{
		{
			Name: "reserve_payment",
			Forward: func(ctx context.Context, tx pgx.Tx, p []byte) error {
				var d struct {
					OrderID   uuid.UUID
					PaymentID uuid.UUID
					Amount    float64
				}
				_ = json.Unmarshal(p, &d)
				_, err := tx.Exec(ctx, `
					INSERT INTO payments (id, order_id, amount, status)
					VALUES ($1,$2,$3,'RESERVED')`, d.PaymentID, d.OrderID, d.Amount)
				if err != nil {
					return err
				}
				return insertOutbox(ctx, tx, Event{
					AggregateType: "payment",
					AggregateID:   d.PaymentID.String(),
					EventType:     "PaymentReserved",
					Payload:       p,
				})
			},
			Compensate: func(ctx context.Context, tx pgx.Tx, p []byte) error {
				var d struct{ OrderID uuid.UUID }
				_ = json.Unmarshal(p, &d)
				_, err := tx.Exec(ctx, `
					UPDATE payments SET status='CANCELLED'
					WHERE order_id=$1 AND status='RESERVED'`, d.OrderID)
				if err != nil {
					return err
				}
				return insertOutbox(ctx, tx, Event{
					AggregateType: "payment",
					AggregateID:   d.OrderID.String(),
					EventType:     "PaymentCancelled",
					Payload:       p,
				})
			},
		},
		{
			Name:    "reserve_stock",
			IsPivot: true,
			Forward: func(ctx context.Context, tx pgx.Tx, p []byte) error {
				var d struct {
					OrderID uuid.UUID
					SKU     string
					Qty     int
				}
				_ = json.Unmarshal(p, &d)
				if d.SKU == "out-of-stock" {
					return errors.New("no stock")
				}
				_, err := tx.Exec(ctx, `
					INSERT INTO stock_reservations (id, order_id, sku, quantity, status)
					VALUES ($1,$2,$3,$4,'RESERVED')`,
					uuid.New(), d.OrderID, d.SKU, d.Qty)
				if err != nil {
					return err
				}
				return insertOutbox(ctx, tx, Event{
					AggregateType: "stock",
					AggregateID:   d.OrderID.String(),
					EventType:     "StockReserved",
					Payload:       p,
				})
			},
			Compensate: func(ctx context.Context, tx pgx.Tx, p []byte) error {
				var d struct{ OrderID uuid.UUID }
				_ = json.Unmarshal(p, &d)
				_, err := tx.Exec(ctx, `
					UPDATE stock_reservations SET status='RELEASED'
					WHERE order_id=$1 AND status='RESERVED'`, d.OrderID)
				if err != nil {
					return err
				}
				return insertOutbox(ctx, tx, Event{
					AggregateType: "stock",
					AggregateID:   d.OrderID.String(),
					EventType:     "StockReleased",
					Payload:       p,
				})
			},
		},
		{
			Name: "confirm_order",
			Forward: func(ctx context.Context, tx pgx.Tx, p []byte) error {
				var d struct{ OrderID uuid.UUID }
				_ = json.Unmarshal(p, &d)
				_, err := tx.Exec(ctx, `
					UPDATE orders SET status='CONFIRMED'
					WHERE id=$1 AND status='PENDING'`, d.OrderID)
				if err != nil {
					return err
				}
				return insertOutbox(ctx, tx, Event{
					AggregateType: "order",
					AggregateID:   d.OrderID.String(),
					EventType:     "OrderConfirmed",
					Payload:       p,
				})
			},
			// Pivot пройден → компенсации нет
		},
	}
}

// KAFKA CONSUMER
func runConsumer(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaAddr},
		Topic:    kafkaTopic,
		GroupID:  "demo-consumer",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("kafka read", "err", err)
			continue
		}

		var eventID uuid.UUID
		for _, h := range m.Headers {
			if h.Key == "event_id" {
				eventID, _ = uuid.Parse(string(h.Value))
			}
		}

		log.Info("consumer ← kafka",
			"event_id", eventID,
			"key", string(m.Key),
			"payload", string(m.Value))

		_ = consumeIdempotent(ctx, pool, "demo-consumer", eventID,
			func(ctx context.Context, tx pgx.Tx) error {
				// Здесь была бы бизнес-логика (read-model и т.п.)
				return nil
			})
	}
}

// MAIN
func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Error("pg connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		log.Error("schema", "err", err)
		os.Exit(1)
	}
	log.Info("schema ok")

	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaAddr),
		Topic:        kafkaTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
	defer writer.Close()

	relay := &Relay{
		pool: pool,
		log:  log,
		send: func(ctx context.Context, eventID uuid.UUID, key string, payload []byte) error {
			return writer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(key),
				Value: payload,
				Headers: []kafka.Header{
					{Key: "event_id", Value: []byte(eventID.String())},
				},
			})
		},
	}
	go relay.Run(ctx)

	go runConsumer(ctx, pool, log)

	// Дать Kafka подняться
	time.Sleep(3 * time.Second)

	// Сценарий 1: успешная сага
	runScenario(ctx, pool, log, "sku-1", 100.0)

	// Сценарий 2: падающая сага (out-of-stock → компенсация)
	time.Sleep(2 * time.Second)
	runScenario(ctx, pool, log, "out-of-stock", 200.0)

	time.Sleep(8 * time.Second)

	dumpState(ctx, pool, log)

	log.Info("done. Ctrl+C для выхода.")
	<-ctx.Done()
}

func runScenario(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger,
	sku string, amount float64) {
	orderID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, amount, status)
		VALUES ($1,'u-1',$2,'PENDING')`, orderID, amount)
	if err != nil {
		log.Error("insert order", "err", err)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"OrderID":   orderID,
		"PaymentID": uuid.New(),
		"SKU":       sku,
		"Qty":       1,
		"Amount":    amount,
	})

	sagaID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO saga_state (saga_id, saga_type, status, step, payload)
		VALUES ($1,'order','STARTED',0,$2)`, sagaID, payload)
	if err != nil {
		log.Error("insert saga", "err", err)
		return
	}

	log.Info("scenario started", "order_id", orderID, "sku", sku)
	orch := &Orchestrator{pool: pool, log: log}
	go orch.Run(ctx, sagaID, buildOrderSaga())
}

func dumpState(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	fmt.Println("\n========== ORDERS ==========")
	rows, _ := pool.Query(ctx, `SELECT id, status FROM orders ORDER BY id`)
	for rows.Next() {
		var id uuid.UUID
		var s string
		rows.Scan(&id, &s)
		fmt.Printf("  %s  %s\n", id, s)
	}
	rows.Close()

	fmt.Println("\n========== SAGA STATE ==========")
	rows, _ = pool.Query(ctx, `
		SELECT saga_id, status, step, last_error FROM saga_state
		ORDER BY updated_at`)
	for rows.Next() {
		var id uuid.UUID
		var s string
		var step int
		var e *string
		rows.Scan(&id, &s, &step, &e)
		var errStr string
		if e != nil {
			errStr = *e
		}
		fmt.Printf("  %s  status=%s step=%d err=%s\n", id, s, step, errStr)
	}
	rows.Close()

	fmt.Println("\n========== OUTBOX ==========")
	rows, _ = pool.Query(ctx, `
		SELECT event_type, status, retry_count FROM outbox ORDER BY created_at`)
	for rows.Next() {
		var t, s string
		var rc int
		rows.Scan(&t, &s, &rc)
		fmt.Printf("  %-20s %-6s retry=%d\n", t, s, rc)
	}
	rows.Close()

	fmt.Println("\n========== PROCESSED EVENTS ==========")
	rows, _ = pool.Query(ctx, `
		SELECT event_id, consumer_group, processed_at
		FROM processed_events ORDER BY processed_at`)
	for rows.Next() {
		var id uuid.UUID
		var g string
		var t time.Time
		rows.Scan(&id, &g, &t)
		fmt.Printf("  %s  %s  %s\n", id, g, t.Format("15:04:05.000"))
	}
	rows.Close()
	fmt.Println()
}
