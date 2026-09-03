package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
)

/*
  БЛОК 2.5: TRANSACTIONAL OUTBOX
  Transactional Outbox — это архитектурный паттерн, решающий одну из самых
  сложных проблем в распределённых системах: как гарантировать консистентность
  между локальной БД и внешней системой (Kafka) без использования
  распределённых транзакций (которых нет и не будет).

  Суть паттерна предельно проста: вместо того чтобы отправлять сообщение
  в Kafka прямо из бизнес-логики, мы сохраняем его в таблицу outbox в ТОЙ ЖЕ
  ACID-транзакции, что и бизнес-данные. Отдельный асинхронный процесс читает
  outbox и отправляет сообщения в Kafka.

  Этот паттерн используется в Netflix, Uber, Airbnb и многих других
  high-load проектах. Без него вы не можете гарантировать exactly-once
  в распределённой системе.

  1.  ПРОБЛЕМА: DUAL-WRITE PROBLEM (ПРОБЛЕМА ДВОЙНОЙ ЗАПИСИ)

  В любой распределённой системе рано или поздно возникает ситуация, когда
  нужно изменить состояние в одной системе и отправить событие в другую.
  Например: создать заказ в PostgreSQL и отправить событие в Kafka.

  ПРОБЛЕМА: это ДВЕ НЕЗАВИСИМЫЕ ОПЕРАЦИИ. Они не могут быть выполнены
  атомарно. Если одна из них упадёт, система станет несогласованной.

   ПЛОХОЙ ПОДХОД:
    func CreateOrder(order Order) error {
        // Шаг 1: сохраняем в БД
        db.Save(&order)

        // Шаг 2: отправляем в Kafka
        kafka.Send("orders", order) //  Может упасть

        // Если Kafka упала — заказ сохранён, но события нет.
        // Downstream-сервисы никогда не узнают о заказе.
    }

   ЕЩЁ ХУЖЕ:
    func CreateOrder(order Order) error {
        // Шаг 1: отправляем в Kafka
        kafka.Send("orders", order) //  Может упасть

        // Шаг 2: сохраняем в БД
        db.Save(&order) //  Может упасть

        // Если БД упала — событие ушло, но заказа нет.
        // Фантомное событие.
    }

   РЕШЕНИЕ:
    func CreateOrder(order Order) error {
        // Шаг 1: сохраняем в БД + outbox в ОДНОЙ транзакции
        tx := db.Begin()
        tx.Save(&order)
        tx.Save(&OutboxEvent{Payload: order}) // ← таблица outbox
        tx.Commit() // либо всё, либо ничего

        // Шаг 2: отдельный процесс прочитает outbox и отправит в Kafka
        // Если Kafka упала — событие останется в outbox и будет отправлено позже
    }

  2.  ПОЧЕМУ OUTBOX РАБОТАЕТ
  Ключевой принцип: мы используем БД как очередь (буфер). Запись в outbox
  происходит в той же транзакции, что и основная бизнес-операция. Это даёт:

    • АТОМАРНОСТЬ: либо всё записано, либо ничего.
    • ИЗОЛЯЦИЯ: запись в outbox видна только после коммита.
    • НАДЁЖНОСТЬ: даже если Kafka недоступна, события сохраняются в БД.

  3.  ДВА ПОДХОДА К РЕАЛИЗАЦИИ OUTBOX PUBLISHER

  3.1. ПОДХОД 1: ПОЛЛИНГ (OUTBOX PUBLISHER НА GO)
    Отдельный воркер периодически опрашивает таблицу outbox и отправляет
    сообщения в Kafka.

    ПЛЮСЫ:
      • Простота реализации.
      • Полный контроль над отправкой.
      • Можно реализовать retry и backoff.
      • Работает с любой БД.

    МИНУСЫ:
      • Нагрузка на БД (постоянные SELECT).
      • Задержка между записью и отправкой (polling interval).
      • Нужно синхронизировать несколько экземпляров воркера (SKIP LOCKED).

  3.2. ПОДХОД 2: CDC (CHANGE DATA CAPTURE) С DEBEZIUM
    Debezium читает WAL (Write-Ahead Log) PostgreSQL и отправляет изменения
    напрямую в Kafka.

    ПЛЮСЫ:
      • Минимальная задержка (streaming, а не polling).
      • Нет нагрузки на БД (читает WAL).
      • Гарантирует exactly-once.
      • Не нужно писать воркер на Go.

    МИНУСЫ:
      • Сложная настройка.
      • Дополнительная инфраструктура (Debezium, Kafka Connect).
      • При изменении схемы outbox нужно обновлять конфигурацию Debezium.

  4.  СТРУКТУРА TABLICA OUTBOX

  4.1. МИНИМАЛЬНАЯ СТРУКТУРА
    CREATE TABLE outbox (
        id UUID PRIMARY KEY,
        aggregate_type VARCHAR(255) NOT NULL,   -- например: "order", "payment"
        aggregate_id VARCHAR(255) NOT NULL,     -- ключ для партиционирования
        event_type VARCHAR(255) NOT NULL,       -- например: "ORDER_CREATED"
        payload JSONB NOT NULL,                 -- данные события
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        processed_at TIMESTAMP,                 -- NULL = не отправлено
        version INT DEFAULT 1                   -- для оптимистичной блокировки
    );

    CREATE INDEX idx_outbox_processed_at ON outbox(processed_at)
        WHERE processed_at IS NULL;

  4.2. РАСШИРЕННАЯ СТРУКТУРА (ДЛЯ ПРОДАКШЕНА)
    CREATE TABLE outbox (
        id UUID PRIMARY KEY,
        aggregate_type VARCHAR(255) NOT NULL,
        aggregate_id VARCHAR(255) NOT NULL,
        event_type VARCHAR(255) NOT NULL,
        payload JSONB NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        processed_at TIMESTAMP,                 -- NULL = не отправлено
        retry_count INT DEFAULT 0,
        last_error TEXT,
        version INT DEFAULT 1,
        trace_id VARCHAR(255),                  -- для распределённой трассировки
        event_id UUID UNIQUE NOT NULL           -- для идемпотентности на стороне консюмера
    );

  5.  РЕАЛИЗАЦИЯ OUTBOX PUBLISHER НА GO (ПОЛНЫЙ КОД)

  5.1. СТРУКТУРА PUBLISHER

    type OutboxPublisher struct {
        db        *sql.DB
        producer  sarama.SyncProducer
        interval  time.Duration
        batchSize int
        logger    *slog.Logger
    }

    func NewOutboxPublisher(db *sql.DB, producer sarama.SyncProducer) *OutboxPublisher {
        return &OutboxPublisher{
            db:        db,
            producer:  producer,
            interval:  1 * time.Second,
            batchSize: 100,
            logger:    slog.Default(),
        }
    }

  5.2. ОСНОВНОЙ ЦИКЛ ПУБЛИШЕРА
    func (p *OutboxPublisher) Run(ctx context.Context) {
        ticker := time.NewTicker(p.interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                p.logger.Info("Outbox publisher stopped")
                return
            case <-ticker.C:
                if err := p.publishBatch(ctx); err != nil {
                    p.logger.Error("Failed to publish batch", "error", err)
                }
            }
        }
    }

  5.3. ПУБЛИКАЦИЯ БАТЧА (С SELECT ... FOR UPDATE SKIP LOCKED)
    func (p *OutboxPublisher) publishBatch(ctx context.Context) error {
        tx, err := p.db.BeginTx(ctx, nil)
        if err != nil {
            return err
        }
        defer tx.Rollback()

        // 🔥 КЛЮЧЕВАЯ ФИЧА: SELECT ... FOR UPDATE SKIP LOCKED
        // SKIP LOCKED гарантирует, что несколько инстансов воркера
        // не вычитают одни и те же записи.
        query := `
            SELECT id, aggregate_type, aggregate_id, event_type, payload, version
            FROM outbox
            WHERE processed_at IS NULL
            ORDER BY created_at ASC
            LIMIT $1
            FOR UPDATE SKIP LOCKED
        `
        rows, err := tx.QueryContext(ctx, query, p.batchSize)
        if err != nil {
            return err
        }
        defer rows.Close()

        var events []OutboxEvent
        for rows.Next() {
            var e OutboxEvent
            if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID,
                &e.EventType, &e.Payload, &e.Version); err != nil {
                return err
            }
            events = append(events, e)
        }

        if len(events) == 0 {
            return nil
        }
        // Отправляем в Kafka
        for _, e := range events {
            topic := e.AggregateType // например, "order"
            key := e.AggregateID

            msg := &sarama.ProducerMessage{
                Topic: topic,
                Key:   sarama.StringEncoder(key),
                Value: sarama.ByteEncoder(e.Payload),
                Headers: []sarama.RecordHeader{
                    {Key: []byte("event_type"), Value: []byte(e.EventType)},
                    {Key: []byte("event_id"), Value: []byte(e.ID.String())},
                },
            }
            _, _, err := p.producer.SendMessage(msg)
            if err != nil {
                // Обновляем retry_count и last_error
                p.updateError(tx, e.ID, err)
                continue
            }

            // Помечаем как отправленное
            if err := p.markProcessed(tx, e.ID); err != nil {
                return err
            }
        }

        return tx.Commit()
    }

  5.4. ФУНКЦИИ ОБНОВЛЕНИЯ СТАТУСА
    func (p *OutboxPublisher) markProcessed(tx *sql.Tx, id UUID) error {
        _, err := tx.Exec(`
            UPDATE outbox
            SET processed_at = NOW(), version = version + 1
            WHERE id = $1 AND processed_at IS NULL
        `, id)
        return err
    }
    func (p *OutboxPublisher) updateError(tx *sql.Tx, id UUID, err error) error {
        _, err = tx.Exec(`
            UPDATE outbox
            SET retry_count = retry_count + 1,
                last_error = $2,
                version = version + 1
            WHERE id = $1
        `, id, err.Error())
        return err
    }

  6.  CDC С DEBEZIUM (АЛЬТЕРНАТИВНЫЙ ПОДХОД)
  Debezium — это платформа для Change Data Capture. Она читает WAL (Write-Ahead
  Log) PostgreSQL и отправляет изменения в Kafka.

  6.1. КАК ЭТО РАБОТАЕТ
    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
    │ PostgreSQL  │───▶│  WAL        │───▶│  Debezium   │───▶│  Kafka      │
    │             │    │  (Write-    │    │  (CDC)      │    │             │
    │  INSERT     │    │   Ahead     │    │             │    │             │
    │  INTO outbox│    │   Log)      │    │             │    │             │
    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘

  6.2. КОНФИГУРАЦИЯ DEBEZIUM (пример для PostgreSQL)
    {
      "name": "outbox-connector",
      "config": {
        "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
        "database.hostname": "postgres",
        "database.port": "5432",
        "database.user": "user",
        "database.password": "password",
        "database.dbname": "mydb",
        "table.include.list": "public.outbox",
        "plugin.name": "pgoutput",
        "transforms": "unwrap",
        "transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
        "transforms.unwrap.drop.tombstones": "false",
        "transforms.unwrap.delete.handling.mode": "drop"
      }
    }

  6.3. ПРЕИМУЩЕСТВА DEBEZIUM
    • Минимальная задержка: события отправляются в Kafka почти мгновенно.
    • Нет нагрузки на БД: читает WAL, а не делает SELECT.
    • Гарантирует at-least-once: если Kafka недоступна, Debezium сохраняет
      позицию в WAL и продолжит с того же места.
    • Не требует дополнительного кода на Go.

  6.4. НЕДОСТАТКИ DEBEZIUM
    • Сложная настройка (Kafka Connect, конфигурация коннектора).
    • Требует понимания WAL и репликации PostgreSQL.
    • При изменении схемы outbox нужно обновлять конфигурацию.
    • Дополнительная инфраструктура (Kafka Connect).

  7.  СРАВНЕНИЕ: ПОЛЛИНГ VS CDC
  ┌─────────────────────┬──────────────────────┬────────────────────────────┐
  │ Критерий            │ Polling (Go)         │ CDC (Debezium)             │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Задержка            │ 1-5 секунд           │ < 100 мс                   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Нагрузка на БД      │ Высокая (SELECT)     │ Низкая (WAL)               │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Сложность           │ Низкая               │ Высокая                    │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Инфраструктура      │ Только Go            │ Kafka Connect + Debezium   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Гарантии            │ At-least-once        │At-least-once / Exactly-once│
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Контроль            │ Полный               │ Ограниченный               │
  └─────────────────────┴──────────────────────┴────────────────────────────┘

  8.  ПРОБЛЕМЫ И ПОДВОДНЫЕ КАМНИ

  8.1. СИНХРОНИЗАЦИЯ НЕСКОЛЬКИХ ВОРКЕРОВ
    Если запущено несколько экземпляров outbox-publisher, они могут пытаться
    обработать одни и те же записи.

    РЕШЕНИЕ: Использовать SELECT ... FOR UPDATE SKIP LOCKED.

    • SELECT ... FOR UPDATE — блокирует выбранные строки.
    • SKIP LOCKED — пропускает уже заблокированные строки.
    • Это гарантирует, что каждый воркер обрабатывает уникальный набор записей.

  8.2. ЗАВИСШИЕ ТРАНЗАКЦИИ
    Если транзакция с outbox не завершилась, запись видна только внутри
    транзакции. Воркер не увидит её до коммита.

    РЕШЕНИЕ: Использовать READ COMMITTED изоляцию для воркера.

  8.3. ДУБЛИРОВАНИЕ СООБЩЕНИЙ
    Если воркер отправил сообщение, но упал до обновления processed_at,
    сообщение будет отправлено повторно.

    РЕШЕНИЕ: Использовать идемпотентный продюсер + уникальные event_id
    для дедупликации на стороне консюмера.

  8.4. НАКОПЛЕНИЕ ДАННЫХ
    Если Kafka долго недоступна, outbox может переполниться.

    РЕШЕНИЕ:
      • Мониторинг размера outbox.
      • Алерты при превышении порога.
      • Автоматическая очистка старых записей (processed_at IS NOT NULL).

  9.  BEST PRACTICES

  9.1. ДЛЯ ОБОИХ ПОДХОДОВ
    • Всегда используйте уникальный event_id в outbox для идемпотентности.
    • Храните в outbox все данные, необходимые для отправки (topic, key, headers).
    • Мониторьте размер outbox и время обработки.
    • Удаляйте или архивируйте обработанные записи (processed_at IS NOT NULL).
    • Используйте индексы для быстрого поиска необработанных записей.

  9.2. ДЛЯ POLLING (GO)
    • Используйте SKIP LOCKED для синхронизации воркеров.
    • Настройте batch size и интервал опроса для баланса производительности.
    • Добавьте retry с экспоненциальной задержкой при ошибках отправки.
    • Используйте контекст для graceful shutdown.

  9.3. ДЛЯ CDC (DEBEZIUM)
    • Настройте правильный plugin.name (pgoutput для PostgreSQL 10+).
    • Используйте transforms для извлечения данных из outbox.
    • Настройте мониторинг Debezium (Lag, ошибки).
    • Обновляйте конфигурацию при изменении схемы.

  10. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ

  1.  Transactional Outbox решает проблему двойной записи (dual-write problem)
      между БД и Kafka.
  2.  Суть: запись события в outbox в той же ACID-транзакции, что и
      бизнес-данные.
  3.  Отдельный воркер (outbox publisher) читает outbox и отправляет в Kafka.
  4.  Два подхода: Polling (на Go) и CDC (Debezium).
  5.  Polling:
      • SELECT ... FOR UPDATE SKIP LOCKED для синхронизации воркеров.
      • Простота, полный контроль.
      • Нагрузка на БД.
  6.  CDC (Debezium):
      • Читает WAL, минимальная задержка.
      • Сложная настройка, дополнительная инфраструктура.
  7.  Для идемпотентности используйте уникальный event_id и идемпотентный
      продюсер.
  8.  Мониторинг outbox (размер, время обработки) — обязателен.
  9.  SKIP LOCKED — ключевой механизм для масштабирования нескольких
      воркеров.
  10. Это архитектурный паттерн, который используется в Netflix, Uber,
      Airbnb для обеспечения консистентности в распределённых системах.
*/

/*
  TRANSACTIONAL OUTBOX
  ЭТОТ ПРИМЕР ПОКАЗЫВАЕТ ПРОДАКШЕН-РЕАЛИЗАЦИЮ OUTBOX PATTERN:
    • Чёткое разделение: репозиторий, сервис, воркер.
    • Транзакции + SELECT FOR UPDATE SKIP LOCKED.
    • Масштабирование: несколько воркеров (горутин) с синхронизацией.
    • Graceful shutdown с контекстом.
    • Логирование, метрики (счётчики).
    • Конфигурация через структуру.
  В ФАЙЛЕ ДВА РЕЖИМА:
    1. polling — запускает outbox publisher и имитирует бизнес-операции.
    2. cdc — запускает консюмера для топика, куда пишет Debezium.
  СТРУКТУРА (в одном файле для удобства, но логически разделена):
    • Config — настройки.
    • OutboxRepository — работа с БД.
    • OutboxPublisher — воркер (polling).
    • OutboxService — бизнес-логика (запись в outbox).
    • CDC Consumer — консюмер для CDC-режима.
  ВНИМАНИЕ: Для CDC нужен Debezium. Конфигурация приведена в комментариях.

*/

// КОНФИГУРАЦИЯ
type Config struct {
	PostgresDSN     string
	KafkaBrokers    []string
	OutboxTopic     string
	PollInterval    time.Duration
	BatchSize       int
	WorkerCount     int
	ConsumerGroupID string
}

var defaultConfig = Config{
	PostgresDSN:     "postgres://user:password@localhost:5432/mydb?sslmode=disable",
	KafkaBrokers:    []string{"localhost:9092"},
	OutboxTopic:     "outbox-events",
	PollInterval:    2 * time.Second,
	BatchSize:       20,
	WorkerCount:     3, // количество конкурентных воркеров
	ConsumerGroupID: "outbox-consumer",
}

// МОДЕЛЬ
type OutboxEvent struct {
	ID            int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	ProcessedAt   sql.NullTime
	RetryCount    int
	LastError     sql.NullString
	Version       int
}

// РЕПОЗИТОРИЙ
type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Insert сохраняет событие в outbox.
func (r *OutboxRepository) Insert(ctx context.Context, aggregateType, aggregateID, eventType string, payload []byte) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`, aggregateType, aggregateID, eventType, payload)
	return err
}

// FetchUnprocessed возвращает необработанные события с блокировкой.
func (r *OutboxRepository) FetchUnprocessed(ctx context.Context, limit int) ([]OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload
		FROM outbox
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// MarkProcessed помечает событие как обработанное.
func (r *OutboxRepository) MarkProcessed(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE outbox
		SET processed_at = NOW(), version = version + 1
		WHERE id = $1 AND processed_at IS NULL
	`, id)
	return err
}

// UpdateError обновляет ошибку и увеличивает счётчик попыток.
func (r *OutboxRepository) UpdateError(ctx context.Context, tx *sql.Tx, id int64, errMsg string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE outbox
		SET retry_count = retry_count + 1,
			last_error = $2,
			version = version + 1
		WHERE id = $1
	`, id, errMsg)
	return err
}

// OUTBOX PUBLISHER (ВОРКЕР)
type OutboxPublisher struct {
	repo      *OutboxRepository
	producer  sarama.SyncProducer
	cfg       Config
	processed int64
	failed    int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

func NewOutboxPublisher(repo *OutboxRepository, producer sarama.SyncProducer, cfg Config) *OutboxPublisher {
	return &OutboxPublisher{
		repo:     repo,
		producer: producer,
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
}

// Run запускает несколько воркеров (горутин) для параллельной обработки.
func (p *OutboxPublisher) Run(ctx context.Context) {
	for i := 0; i < p.cfg.BatchSize; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	log.Printf("Outbox publisher запущен с %d воркерами", p.cfg.WorkerCount)

	<-p.stopCh
	p.wg.Wait()
	log.Println("Outbox publisher остановлен")
}

func (p *OutboxPublisher) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.processBatch(ctx, id); err != nil {
				log.Printf("❌ [воркер %d] ошибка обработки: %v", id, err)
			}
		}
	}
}

func (p *OutboxPublisher) processBatch(ctx context.Context, workerID int) error {
	tx, err := p.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	events, err := p.repo.FetchUnprocessed(ctx, p.cfg.BatchSize)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	log.Printf("[воркер %d] обработано %d событий", workerID, len(events))

	for _, e := range events {
		msg := &sarama.ProducerMessage{
			Topic: p.cfg.OutboxTopic,
			Key:   sarama.StringEncoder(e.AggregateID),
			Value: sarama.ByteEncoder(e.Payload),
			Headers: []sarama.RecordHeader{
				{Key: []byte("event_type"), Value: []byte(e.EventType)},
				{Key: []byte("aggregate_type"), Value: []byte(e.AggregateType)},
			},
		}
		_, _, err := p.producer.SendMessage(msg)
		if err != nil {
			atomic.AddInt64(&p.failed, 1)
			if updErr := p.repo.UpdateError(ctx, tx, e.ID, err.Error()); updErr != nil {
				log.Printf("[воркер %d] ошибка обновления ошибки: %v", workerID, updErr)
			}
			continue
		}
		if err := p.repo.MarkProcessed(ctx, tx, e.ID); err != nil {
			atomic.AddInt64(&p.failed, 1)
			continue
		}
		atomic.AddInt64(&p.processed, 1)
	}
	return tx.Commit()
}

func (p *OutboxPublisher) Stop() {
	close(p.stopCh)
}

func (p *OutboxPublisher) Stats() (processed, failed int64) {
	return atomic.LoadInt64(&p.processed), atomic.LoadInt64(&p.failed)
}

// БИЗНЕС-СЛУЖБА (ЗАПИСЬ В OUTBOX)
type OutboxService struct {
	repo *OutboxRepository
}

func NewOutboxService(repo *OutboxRepository) *OutboxService {
	return &OutboxService{repo: repo}
}

// CreateOrder — пример бизнес-операции, которая записывает в outbox.
func (s *OutboxService) CreateOrder(ctx context.Context, orderID string, amount float64) error {
	payload := []byte(fmt.Sprintf(`{"id":"%s","amount":%.2f}`, orderID, amount))

	// В реальном проекте здесь была бы запись в таблицу orders в той же транзакции.
	// Для простоты пишем только в outbox.
	return s.repo.Insert(ctx, "order", orderID, "ORDER_CREATED", payload)
}

//CDC КОНСЮМЕР

type CDCConsumer struct {
	consumer sarama.ConsumerGroup
	ready    chan bool
}

func NewCDCConsumer(cfg Config) (*CDCConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(cfg.KafkaBrokers, cfg.ConsumerGroupID, config)
	if err != nil {
		return nil, err
	}
	return &CDCConsumer{consumer: client, ready: make(chan bool)}, nil
}

func (c *CDCConsumer) Run(ctx context.Context, topic string) {
	handler := &CDCHandler{ready: c.ready}
	go func() {
		for {
			if err := c.consumer.Consume(ctx, []string{topic}, handler); err != nil {
				log.Printf("❌ Ошибка консюмера: %v", err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	<-c.ready
	log.Println("✅ CDC консюмер готов")
	<-ctx.Done()
	c.consumer.Close()
}

type CDCHandler struct {
	ready chan bool
}

func (h *CDCHandler) Setup(session sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}
func (h *CDCHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}
func (h *CDCHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("📨 CDC: событие: key=%s, value=%s", string(msg.Key), string(msg.Value))
		session.MarkMessage(msg, "")
	}
	return nil
}

//INIT DB

func initDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS outbox (
			id BIGSERIAL PRIMARY KEY,
			aggregate_type VARCHAR(255) NOT NULL,
			aggregate_id VARCHAR(255) NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMP,
			retry_count INT DEFAULT 0,
			last_error TEXT,
			version INT DEFAULT 1
		);
		CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox(processed_at) WHERE processed_at IS NULL;
	`)
	return db, err
}

//MAIN

func main() {
	cfg := defaultConfig
	mode := flag.String("mode", "polling", "режим: polling или cdc")
	flag.Parse()

	db, err := initDB(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()
	log.Println("БД готова")

	producer, err := sarama.NewSyncProducer(cfg.KafkaBrokers, sarama.NewConfig())
	if err != nil {
		log.Fatalf("Ошибка продюсера: %v", err)
	}
	defer producer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *mode == "polling" {
		runPolling(ctx, db, producer, cfg)
	} else if *mode == "cdc" {
		runCDC(ctx, cfg)
	} else {
		log.Fatalf("Неверный режим: %s", *mode)
	}
}

func runPolling(ctx context.Context, db *sql.DB, producer sarama.SyncProducer, cfg Config) {
	repo := NewOutboxRepository(db)
	service := NewOutboxService(repo)
	publisher := NewOutboxPublisher(repo, producer, cfg)

	// Запускаем publisher
	go publisher.Run(ctx)

	// Имитация бизнес-операций
	go func() {
		for i := 1; i <= 10; i++ {
			orderID := fmt.Sprintf("order-%d", i)
			if err := service.CreateOrder(ctx, orderID, float64(i*100)); err != nil {
				log.Printf("Ошибка создания заказа: %v", err)
			} else {
				log.Printf("Создан заказ %s", orderID)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Остановка...")
	publisher.Stop()
	time.Sleep(1 * time.Second)
	processed, failed := publisher.Stats()
	log.Printf("📊 Статистика: processed=%d, failed=%d", processed, failed)

}

func runCDC(ctx context.Context, cfg Config) {
	consumer, err := NewCDCConsumer(cfg)
	if err != nil {
		log.Fatalf("Ошибка консюмера: %v", err)
	}
	go consumer.Run(ctx, cfg.OutboxTopic)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("⏳ Остановка консюмера...")
}
