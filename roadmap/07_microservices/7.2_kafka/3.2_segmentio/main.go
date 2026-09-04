package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

/*
  БЛОК 3.2: SEGMENTIO/KAFKA-GO
  segmentio/kafka-go — это чистая Go-реализация клиента для Apache Kafka.
  Она не использует CGO, что делает её простой в установке и сборке.
  API максимально простой и идиоматичный для Go, хорошо подходит для
  асинхронных сценариев и приложений, где не нужны транзакции.

  1.  В ДВУХ СЛОВАХ: ЧТО ЭТО И ЗАЧЕМ
  segmentio/kafka-go — это чистый Go клиент для Kafka.

  ОСОБЕННОСТИ:
    • Нет CGO — легко собирается, нет зависимости от C-компилятора.
    • Простой и идиоматичный API.
    • Поддержка Go context.Context (в отличие от confluent-kafka-go).
    • Подходит для асинхронных сценариев.
    • Хорошая производительность для большинства задач.

  ОГРАНИЧЕНИЯ:
    • Нет поддержки транзакций.
    • Нет exactly-once семантики (EOS).
    • Нет идемпотентного продюсера.
    • Производительность ниже, чем у confluent-kafka-go и sarama.

  2.  СРАВНЕНИЕ С ДРУГИМИ БИБЛИОТЕКАМИ (ЧТО СПРОСЯТ)
  ┌─────────────────────┬──────────────────┬──────────────────┬──────────────────┐
  │ Характеристика      │ kafka-go         │ sarama           │ confluent        │
  │                     │ (segmentio)      │ (IBM)            │ (CGO)            │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Реализация          │ Pure Go          │ Pure Go          │ CGO (librdkafka) │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Производительность  │ Средняя          │ Высокая          │ Самая высокая    │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Транзакции / EOS    │ Нет              │ Да               │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Поддержка context   │ Да               │ Нет              │ Нет              │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Сложность сборки    │ Низкая           │ Низкая           │ Высокая          │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Простота API        │ Очень простая    │ Средняя          │ Сложная          │
  └─────────────────────┴──────────────────┴──────────────────┴──────────────────┘

  3.  ПОДДЕРЖКА CONTEXT — ГЛАВНОЕ ПРЕИМУЩЕСТВО
  В отличие от confluent-kafka-go и sarama, kafka-go поддерживает
  Go context.Context из коробки.

  //РАБОТАЕТ — таймаут через context
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  // Чтение с таймаутом
  msg, err := reader.ReadMessage(ctx)

  // Отправка с таймаутом
  err := writer.WriteMessages(ctx, msg)

  // Отмена запроса
  ctx, cancel := context.WithCancel(context.Background())
  go func() {
      msg, err := reader.ReadMessage(ctx)
      // ...
  }()
  cancel() // отменяем чтение

  4.  ПРОСТОТА API — ГЛАВНАЯ ФИЧА
  API kafka-go максимально простой и интуитивно понятный.

  4.1. ПРОДЮСЕР (WRITER)
    writer := &kafka.Writer{
        Addr:  kafka.TCP("localhost:9092"),
        Topic: "my-topic",
        Balancer: &kafka.LeastBytes{}, // стратегия балансировки
        BatchSize: 100,               // размер батча
        BatchTimeout: 10 * time.Millisecond, // таймаут батча
        Async: false,                 // синхронная отправка
        RequiredAcks: kafka.RequireAll, // acks=all
        Compression: kafka.Snappy,    // сжатие
    }
    defer writer.Close()

    // Отправка одного сообщения
    err := writer.WriteMessages(ctx,
        kafka.Message{
            Key:   []byte("key"),
            Value: []byte("value"),
        },
    )

    // Отправка нескольких сообщений (батч)
    err := writer.WriteMessages(ctx,
        kafka.Message{Key: []byte("k1"), Value: []byte("v1")},
        kafka.Message{Key: []byte("k2"), Value: []byte("v2")},
    )

  4.2. КОНСЮМЕР (READER)
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:   []string{"localhost:9092"},
        Topic:     "my-topic",
        GroupID:   "my-group",
        MinBytes:  10e3, // 10KB
        MaxBytes:  10e6, // 10MB
        MaxWait:   1 * time.Second,
        CommitInterval: 0, // 0 = ручной коммит
        StartOffset: kafka.FirstOffset, // читаем с начала
    })
    defer reader.Close()

    // Чтение одного сообщения
    msg, err := reader.ReadMessage(ctx)

    // Чтение в цикле (с ручным коммитом)
    for {
        msg, err := reader.FetchMessage(ctx)
        if err != nil {
            break
        }
        // обработка
        reader.CommitMessages(ctx, msg) // ручной коммит
    }

  4.3. CONSUMER GROUP
    // kafka-go поддерживает consumer groups "из коробки"
    // через Reader с указанием GroupID

    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"localhost:9092"},
        Topic:   "my-topic",
        GroupID: "my-group", // <- consumer group
        // ...
    })

  5.  ОСНОВНЫЕ КОМПОНЕНТЫ

  5.1. WRITER — продюсер
    writer := &kafka.Writer{
        Addr:  kafka.TCP("localhost:9092"),
        Topic: "my-topic",
        Balancer: &kafka.LeastBytes{},  // распределение по партициям
        BatchSize: 100,                 // размер батча
        BatchTimeout: 10 * time.Millisecond,
        RequiredAcks: kafka.RequireAll, // acks
        Async: false,                   // синхронный или асинхронный
        Compression: kafka.Snappy,      // сжатие
    }

  5.2. READER — консюмер
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:   []string{"localhost:9092"},
        Topic:     "my-topic",
        GroupID:   "my-group",
        MinBytes:  10e3,
        MaxBytes:  10e6,
        MaxWait:   1 * time.Second,
        CommitInterval: 0,
        StartOffset: kafka.FirstOffset,
    })

  5.3. CONN — низкоуровневое соединение
    conn, err := kafka.DialLeader(context.Background(), "tcp", "localhost:9092", "my-topic", 0)

    // Отправка через conn
    conn.WriteMessages(...)

    // Чтение через conn
    conn.ReadMessage(...)

  6.  БАЛАНСИРОВКА (PARTITION BALANCER)
  kafka-go предлагает несколько стратегий распределения по партициям:

    • kafka.LeastBytes{} — сообщение отправляется в партицию с наименьшим
      размером данных (по умолчанию)
    • kafka.RoundRobin{} — по кругу
    • kafka.Hash{} — по хэшу ключа
    • kafka.CRC32Balancer{} — по CRC32 хэшу ключа

  writer := &kafka.Writer{
      // ...
      Balancer: &kafka.Hash{}, // сообщения с одинаковым ключом попадают в одну партицию
  }

  7.  СЖАТИЕ (COMPRESSION)
  Поддерживаются все основные алгоритмы сжатия:
    • kafka.Snappy   — хороший баланс скорость/размер
    • kafka.Gzip     — лучшее сжатие, медленнее
    • kafka.Lz4      — быстро, хорошее сжатие
    • kafka.Zstd     — лучшее сжатие, медленнее (требуется Kafka 2.1+)

  writer := &kafka.Writer{
      // ...
      Compression: kafka.Snappy,
  }

  8.  АСИНХРОННАЯ ОТПРАВКА
  kafka-go поддерживает асинхронную отправку через Async=true.

  writer := &kafka.Writer{
      // ...
      Async: true, // асинхронная отправка
  }

  // Отправка не блокируется
  err := writer.WriteMessages(ctx, msg1, msg2)

  // Для получения отчётов о доставке используйте Completion
  ch := make(chan error, 1)
  writer.WriteMessages(ctx, msg, &kafka.Completion{ch})
  err := <-ch // блокируется до доставки

  9.  КОНФИГУРАЦИЯ ЧЕРЕЗ КОНФИГ-СТРУКТУРЫ
  В отличие от других библиотек, kafka-go использует структуры для
  конфигурации, а не мапы.

  9.1. ДЛЯ WRITER
    config := kafka.WriterConfig{
        Addr:  kafka.TCP("localhost:9092"),
        Topic: "my-topic",
        Balancer: &kafka.LeastBytes{},
        BatchSize: 100,
        BatchTimeout: 10 * time.Millisecond,
        RequiredAcks: kafka.RequireAll,
        Async: false,
        Compression: kafka.Snappy,
        Logger: log.Default(), // кастомный логгер
        ErrorLogger: log.Default(),
    }
    writer := kafka.NewWriter(config)

  9.2. ДЛЯ READER
    config := kafka.ReaderConfig{
        Brokers: []string{"localhost:9092"},
        Topic: "my-topic",
        GroupID: "my-group",
        MinBytes: 10e3,
        MaxBytes: 10e6,
        MaxWait: 1 * time.Second,
        CommitInterval: 0,
        StartOffset: kafka.FirstOffset,
        Logger: log.Default(),
        ErrorLogger: log.Default(),
    }
    reader := kafka.NewReader(config)

  10.  ГЛАВНЫЕ ГРАБЛИ (ЧТО МОЖЕТ УБИТЬ)

  10.1. НЕТ ТРАНЗАКЦИЙ
    Если тебе нужна exactly-once семантика — kafka-go не подходит.
    Транзакции и идемпотентность не поддерживаются.

  10.2. ПРОИЗВОДИТЕЛЬНОСТЬ НИЖЕ
    По сравнению с confluent-kafka-go и sarama производительность
    kafka-go ниже. Для high-load (>50k msg/s) лучше выбрать другую
    библиотеку.

  10.3. ПАМЯТЬ ПРИ БОЛЬШИХ БАТЧАХ
    При использовании больших батчей (MaxBytes) и медленных консюмеров
    возможен рост памяти.

    // Решение: уменьшить MaxBytes или увеличить MaxWait
    config.MaxBytes = 1 * 1024 * 1024 // 1MB
    config.MaxWait = 5 * time.Second

  10.4. COMMIT INTERVAL
    При использовании CommitInterval > 0 автоматический коммит может
    привести к потере сообщений при падении консюмера.

    // Решение: использовать ручной коммит
    config.CommitInterval = 0

  10.5. ЗАВИСИМОСТИ
    В отличие от confluent-kafka-go, у kafka-go нет зависимостей.
    Всё, что нужно — один пакет.

  11.  КОГДА ВЫБИРАТЬ SEGMENTIO/KAFKA-GO

  ВЫБИРАЙ, ЕСЛИ:
    + Тебе нужна простота и идиоматичный Go API.
    + Ты не используешь транзакции и EOS.
    + Тебе нужна поддержка Go context.
    + У тебя средняя нагрузка (< 50k msg/s).
    + Ты не хочешь возиться с CGO.

  НЕ ВЫБИРАЙ, ЕСЛИ:
    - Тебе нужны транзакции и exactly-once.
    - Тебе нужна максимальная производительность (> 50k msg/s).
    - Ты работаешь с Confluent Cloud.
    - Тебе нужна идемпотентность продюсера.

  12.  ПРИМЕР ПОЛНОЙ КОНФИГУРАЦИИ ДЛЯ ПРОДАКШЕНА

  PRODUCER:
    writer := &kafka.Writer{
        Addr:  kafka.TCP("broker1:9092", "broker2:9092", "broker3:9092"),
        Topic: "my-topic",
        Balancer: &kafka.LeastBytes{},
        BatchSize: 100,
        BatchTimeout: 10 * time.Millisecond,
        RequiredAcks: kafka.RequireAll,
        Async: false,
        Compression: kafka.Snappy,
        MaxAttempts: 3,                 // повторные попытки
        WriteTimeout: 10 * time.Second, // таймаут записи
        ReadTimeout: 10 * time.Second,  // таймаут чтения
        Logger: log.Default(),
    }

  CONSUMER:
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"broker1:9092", "broker2:9092", "broker3:9092"},
        Topic: "my-topic",
        GroupID: "my-group",
        MinBytes: 10e3,
        MaxBytes: 10e6,
        MaxWait: 5 * time.Second,
        CommitInterval: 0,              // ручной коммит
        StartOffset: kafka.FirstOffset,
        ReadTimeout: 10 * time.Second,  // таймаут чтения
        Logger: log.Default(),
    })

  13.  ОТВЕТ
  Вопрос: "Какую библиотеку для Kafka в Go ты используешь и почему?"

  Ответ для kafka-go:
  "Мы используем segmentio/kafka-go. Это чистый Go клиент, который легко
  собирать и деплоить. Он поддерживает context, что упрощает управление
  таймаутами. API очень простой и идиоматичный. Для наших сценариев
  (логи, метрики, событийность с нагрузкой до 30k msg/s) его производительности
  достаточно. Если бы нам понадобились транзакции, мы бы рассмотрели sarama."

  14.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  segmentio/kafka-go — чистый Go клиент без CGO.
  2.  Простой и идиоматичный API.
  3.  Поддерживает Go context.
  4.  Нет транзакций и EOS.
  5.  Производительность ниже, чем у confluent и sarama.
  6.  Выбирай для простых сценариев и средней нагрузки.
  7.  Не выбирай для high-load и транзакций.
  8.  В продакшене настраивай таймауты и ручной коммит.
*/

//КОНФИГУРАЦИЯ

const (
	defaultBroker  = "localhost:9092"
	defaultTopic   = "orders" // топик для заказов
	defaultGroupID = "order-processor"
	batchSize      = 100             // размер пачки для обработки
	commitInterval = 5 * time.Second // интервал коммита
)

var (
	mode    = flag.String("mode", "producer", "Режим: producer, consumer")
	broker  = flag.String("broker", defaultBroker, "Адрес брокера")
	topic   = flag.String("topic", defaultTopic, "Топик")
	groupID = flag.String("group", defaultGroupID, "ID группы")
)

//МЕТРИКИ

type Metrics struct {
	MessagesProcessed int64
	MessagesFailed    int64
	LastOffset        int64
	mu                sync.Mutex
}

func (m *Metrics) IncProcessed() {
	atomic.AddInt64(&m.MessagesProcessed, 1)
}

func (m *Metrics) IncFailed() {
	atomic.AddInt64(&m.MessagesFailed, 1)
}

func (m *Metrics) SetLastOffset(offset int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastOffset = offset
}

func (m *Metrics) Print() {
	processed := atomic.LoadInt64(&m.MessagesProcessed)
	failed := atomic.LoadInt64(&m.MessagesFailed)
	m.mu.Lock()
	offset := m.LastOffset
	m.mu.Unlock()
	log.Printf("Метрики: обработано=%d, ошибок=%d, последний offset=%d",
		processed, failed, offset)
}

// ПРОДЮСЕР (С АСИНХРОННОЙ ОТПРАВКОЙ)
func runProducer() {
	log.Println("Запуск продюсера (асинхронный)")

	// Конфигурация Writer
	w := &kafka.Writer{
		Addr:         kafka.TCP(*broker),
		Topic:        *topic,
		Balancer:     &kafka.LeastBytes{}, // балансировка по наименьшей нагрузке
		BatchSize:    100,                 // размер батча
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll, // acks=all
		Async:        true,             // асинхронная отправка
		Compression:  kafka.Snappy,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Logger:       log.Default(),
	}
	defer w.Close()

	// Канал для получения completion-отчётов
	complectionCh := make(chan error, 100)
	defer close(complectionCh)

	// Горутина для обработки отчётов
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range complectionCh {
			if err != nil {
				log.Printf("Ошибка доставки: %v", err)
			}
			log.Printf("Сообщение доставлено")
		}
	}()

	// Контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Отправляем 1000 сообщений (имитация заказов)
	log.Println("Отправка 1000 сообщений...")
	for i := 0; i < 1000; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		msg := kafka.Message{
			Key:   []byte(orderID),
			Value: []byte(fmt.Sprintf(`{"id":"%s","amount":%.2f}`, orderID, float64(i*100))),
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte("ORDER_CREATED")},
				{Key: "source", Value: []byte("producer")},
			},
		}
		// Асинхронная отправка (без аргумента Completion)
		if err := w.WriteMessages(ctx, msg); err != nil {
			log.Printf("Ошибка отправки: %v", err)
			continue
		}
		if i%100 == 0 {
			log.Printf("Отправлено %d сообщений", i)
		}
	}

	// Ждём доставки (закрываем Writer, чтобы получить все completion)
	w.Close()
	wg.Wait()
	log.Println("Продюсер завершён")
}

// ─── КОНСЮМЕР С ГРУППОЙ И РУЧНОЙ РЕБАЛАНСИРОВКОЙ
type ConsumerHandler struct {
	reader  *kafka.Reader
	metrics *Metrics
	batch   []kafka.Message
	batchMu sync.Mutex
	done    chan struct{}
}

func NewConsumerHandler(reader *kafka.Reader) *ConsumerHandler {
	return &ConsumerHandler{
		reader:  reader,
		metrics: &Metrics{},
		batch:   make([]kafka.Message, 0, batchSize),
		done:    make(chan struct{}),
	}
}

func (h *ConsumerHandler) HandleMessage(msg kafka.Message) error {
	// Имитация бизнес-логики
	log.Printf("Обработка: key=%s, value=%s, offset=%d",
		string(msg.Key), string(msg.Value), msg.Offset)
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (h *ConsumerHandler) ProcessBatch(batch []kafka.Message) error {
	log.Printf("Обработка пачки из %d сообщений", len(batch))
	for _, msg := range batch {
		if err := h.HandleMessage(msg); err != nil {
			return err
		}
		h.metrics.IncProcessed()
		h.metrics.SetLastOffset(msg.Offset)
	}
	return nil
}

func (h *ConsumerHandler) Run(ctx context.Context) {
	defer close(h.done)

	commitTicker := time.NewTicker(commitInterval)
	defer commitTicker.Stop()

	metricsTicker := time.NewTicker(10 * time.Second)
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Завершение консюмера...")
			h.commitBatch()
			return
		case <-metricsTicker.C:
			h.metrics.Print()
		case <-commitTicker.C:
			h.commitBatch()
		default:
			msg, err := h.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					continue
				}
				log.Printf("Ошибка чтения: %v", err)
				h.metrics.IncFailed()
				continue
			}

			h.batchMu.Lock()
			h.batch = append(h.batch, msg)
			h.batchMu.Unlock()

			if len(h.batch) >= batchSize {
				if err := h.processAndCommit(); err != nil {
					log.Printf("Ошибка обработки пачки: %v", err)
					h.metrics.IncFailed()
				}
			}
		}
	}
}

func (h *ConsumerHandler) processAndCommit() error {
	h.batchMu.Lock()
	batch := h.batch
	h.batch = make([]kafka.Message, 0, batchSize)
	h.batchMu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	if err := h.ProcessBatch(batch); err != nil {
		return err
	}
	if err := h.reader.CommitMessages(context.Background(), batch...); err != nil {
		return fmt.Errorf("коммит смещений: %w", err)
	}
	log.Printf("Закоммичено %d сообщений", len(batch))
	return nil
}

func (h *ConsumerHandler) commitBatch() {
	h.batchMu.Lock()
	if len(h.batch) > 0 {
		batch := h.batch
		h.batch = make([]kafka.Message, 0, batchSize)
		h.batchMu.Unlock()
		if err := h.reader.CommitMessages(context.Background(), batch...); err != nil {
			log.Printf("Ошибка коммита: %v", err)
		} else {
			log.Printf("Закоммичено %d сообщений (таймаут)", len(batch))
		}
	} else {
		h.batchMu.Unlock()
	}
}

func runConsumer() {
	log.Println("Запуск консюмера с группой")

	config := kafka.ReaderConfig{
		Brokers:        []string{*broker},
		Topic:          *topic,
		GroupID:        *groupID,
		StartOffset:    kafka.FirstOffset,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        5 * time.Second,
		CommitInterval: 0,
		Logger:         log.Default(),
	}
	reader := kafka.NewReader(config)
	defer reader.Close()

	handler := NewConsumerHandler(reader)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	go func() {
		<-sigCh
		log.Println("Получен сигнал завершения...")
		cancel()
	}()

	handler.Run(ctx)
	log.Println("Консюмер завершён")
}
