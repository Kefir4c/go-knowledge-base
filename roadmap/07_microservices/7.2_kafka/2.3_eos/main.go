package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

/*
  БЛОК 2.3: EXACTLY-ONCE SEMANTICS
  Exactly-once semantics (EOS) — это высшая форма гарантий доставки в Kafka,
  которая обеспечивает, что каждое сообщение будет обработано ровно один раз,
  без потерь и без дублей. Это сочетание идемпотентного продюсера и транзакций,
  согласованное на всех уровнях системы.

  1.  ПОЧЕМУ EOS — ЭТО СЛОЖНО
  В распределённых системах достичь exactly-once принципиально сложно,
  потому что:
    • Сеть ненадёжна — пакеты могут теряться, дублироваться, приходить
      не в том порядке.
    • Компоненты могут падать в любой момент.
    • Нет единого глобального состояния.

  Kafka решает эту проблему через комбинацию механизмов:
    1. Идемпотентный продюсер — защита от дублей при повторных отправках.
    2. Транзакции — атомарность при записи в несколько партиций.
    3. Правильная настройка консюмера — чтение только закоммиченных данных.
    4. Согласованная работа всех компонентов.

  2.  ТРИ ГАРАНТИИ EOS В KAFKA
  В Kafka EOS достигается на трёх уровнях:

  2.1. ПРОДЮСЕР → KAFKA (ПРОИЗВОДСТВО)
    • Идемпотентный продюсер (enable.idempotence=true).
    • Транзакции (transactional.id).
    • acks=all, retries>0.

    Гарантирует: каждое сообщение будет записано в партицию ровно один раз.

  2.2. ВНУТРИ KAFKA (ХРАНЕНИЕ)
    • Репликация с ISR.
    • min.insync.replicas.
    • Транзакционный лог.

    Гарантирует: данные не теряются при сбоях брокеров.

  2.3. KAFKA → КОНСЮМЕР (ПОТРЕБЛЕНИЕ)
    • isolation.level = read_committed.
    • Ручной коммит смещения.
    • Коммит смещения в той же транзакции, что и отправка результата.

    Гарантирует: консюмер видит только закоммиченные данные и не читает
    "черновики" (uncommitted).

  3.  КОМБИНАЦИЯ ИДЕМПОТЕНТНОСТИ И ТРАНЗАКЦИЙ
  Идемпотентность и транзакции — это два взаимодополняющих механизма:

  3.1. ИДЕМПОТЕНТНОСТЬ (ПЕРВЫЙ УРОВЕНЬ)
    • Защищает от дублей в пределах одной партиции.
    • Использует PID + Sequence Number.
    • Включена enable.idempotence=true.

    Что даёт: даже если продюсер повторно отправляет одно и то же сообщение,
    брокер его не запишет повторно.

  3.2. ТРАНЗАКЦИИ (ВТОРОЙ УРОВЕНЬ)
    • Защищают от дублей между партициями.
    • Дают атомарность записи в несколько партиций.
    • Используют transactional.id.

    Что даёт: запись в несколько партиций происходит атомарно — либо все
    сообщения записаны, либо ни одного.

  3.3. ВМЕСТЕ
    Идемпотентность + транзакции дают exactly-once:
      • Нет дублей (идемпотентность).
      • Атомарность (транзакции).
      • Нет потерь (replication + acks=all).

  4.  КОНСЮМЕР С ISOLATION.LEVEL = READ_COMMITTED
  Это ключевая настройка для EOS на стороне консюмера.

  4.1. ЧТО ЭТО ДАЁТ
    Консюмер с isolation.level = read_committed читает только те сообщения,
    которые принадлежат закоммиченным транзакциям.

    • Не видит сообщения из незавершённых транзакций.
    • Не видит сообщения из откатанных (aborted) транзакций.

  4.2. КАК ЭТО РАБОТАЕТ
    Когда продюсер начинает транзакцию, сообщения помечаются Transaction
    Marker (begin/commit/abort). Консюмер с read_committed читает только
    те, у которых есть commit.

  4.3. КОГДА ЭТО ВАЖНО
    • При откате транзакции консюмер не увидит откатанные данные.
    • При чтении незавершённой транзакции консюмер блокируется и ждёт
      commit или abort.

  5.  END-TO-END EXACTLY-ONCE (E2E EOS)
  E2E EOS — это сквозная гарантия от начала до конца: от первого продюсера
  до финального консюмера.

  5.1. СХЕМА РАБОТЫ
    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
    │ Producer│──▶│  Kafka  │───▶│ Consumer│──▶│ Producer│
    │ (input) │    │ (topicA)│    │         │    │ (output)│
    └─────────┘    └─────────┘    └─────────┘    └─────────┘
         │              │              │              │
         │   enable.idempotence=true   │   transactional.id
         │   acks=all                  │   read_committed
         │   retries>0                 │   manual commit
         └─────────────────────────────┴──────────────┘

  5.2. ПОШАГОВО
    1. Продюсер-источник отправляет сообщение в Kafka (topic A) с идемпотентностью.
    2. Консюмер читает сообщение из topic A с read_committed.
    3. Консюмер обрабатывает сообщение (бизнес-логика).
    4. Продюсер-приёмник (той же консюмер) начинает транзакцию.
    5. Продюсер отправляет результат в topic B.
    6. Продюсер коммитит смещение консюмера в той же транзакции.
    7. Продюсер коммитит транзакцию.

  5.3. АТОМАРНОСТЬ
    Шаги 4-7 выполняются атомарно: либо все выполнены, либо ни один.
    Это гарантирует, что консюмер не закоммитит смещение, если не отправил
    результат.

  6.  КАК ЭТО РАБОТАЕТ ПОД КАПОТОМ

  6.1. TRANSACTION COORDINATOR
    • Специальный брокер, отвечающий за конкретный transactional.id.
    • Управляет состоянием транзакции (begin, commit, abort).
    • Хранит состояние в топике __transaction_state.

  6.2. PRODUCER ID (PID) И EPOCH
    • PID — уникальный идентификатор продюсера.
    • Epoch — счётчик версий PID.
    • При сбое продюсера новый экземпляр получает новый epoch.
    • Это позволяет брокеру отличать старые сообщения от новых.

  6.3. SEQUENCE NUMBER
    • Порядковый номер сообщения в партиции.
    • Используется для обнаружения дублей.
    • При смене epoch sequence number сбрасывается.

  6.4. ДВУХФАЗНЫЙ КОММИТ (2PC)
    При коммите транзакции Coordinator выполняет двухфазный коммит:
      1. PREPARE — все брокеры подтверждают готовность.
      2. COMMIT — все брокеры фиксируют транзакцию.

  7.  ПРОИЗВОДИТЕЛЬНОСТЬ И КОМПРОМИССЫ

  7.1. ВЛИЯНИЕ НА ПРОИЗВОДИТЕЛЬНОСТЬ
      Идемпотентность: -5-10% пропускной способности.
      Транзакции: -30-50% пропускной способности.
      read_committed: +10-20% задержки.

  7.2. КОГДА ИСПОЛЬЗОВАТЬ EOS
      Финансовые транзакции (платежи, переводы).
      Бухгалтерские системы (учёт, баланс).
      Системы заказов (создание, оплата, доставка).
      Любые системы, где дубли или потери недопустимы.

  7.3. КОГДА НЕ ИСПОЛЬЗОВАТЬ
      Высоконагруженные системы (>100k msg/sec).
      Системы мониторинга и метрик.
      Логи и трейсы.
      Системы, где допустимы дубли (аналитика, рекомендации).

  8.  ОГРАНИЧЕНИЯ И ПОДВОДНЫЕ КАМНИ

  8.1. EXACTLY-ONCE НЕ РАСПРОСТРАНЯЕТСЯ НА ВНЕШНИЕ СИСТЕМЫ
    EOS работает только в пределах Kafka. Если после обработки сообщения
    вы обновляете внешнюю базу данных (PostgreSQL) — EOS не гарантируется.
    Решение: паттерн Transactional Outbox или идемпотентность внешних вызовов.

  8.2. ПРОИЗВОДИТЕЛЬНОСТЬ
    EOS значительно снижает производительность. Для high-load систем
    часто выбирают at-least-once с идемпотентной обработкой.

  8.3. УНИКАЛЬНОСТЬ TRANSACTIONAL.ID
    transactional.id должен быть уникальным в кластере. Два продюсера
    с одинаковым transactional.id не могут работать одновременно.

  8.4. ВОССТАНОВЛЕНИЕ ПОСЛЕ СБОЕВ
    При сбое продюсера транзакция может остаться незавершённой.
    Coordinator автоматически завершает транзакцию после таймаута
    (transaction.timeout.ms).

  9.  ПОЛНАЯ КОНФИГУРАЦИЯ ДЛЯ EOS

  9.1. ПРОДЮСЕР (ВХОДНОЙ)
    enable.idempotence = true
    acks = all
    retries = Integer.MAX_VALUE
    max.in.flight.requests.per.connection = 5

  9.2. БРОКЕР
    min.insync.replicas = 2
    default.replication.factor = 3
    transaction.state.log.replication.factor = 3
    transaction.state.log.min.isr = 2

  9.3. ПРОДЮСЕР (ТРАНЗАКЦИОННЫЙ)
    enable.idempotence = true
    acks = all
    transactional.id = "my-transactional-id"
    retries = Integer.MAX_VALUE

  9.4. КОНСЮМЕР
    enable.auto.commit = false
    isolation.level = read_committed
    partition.assignment.strategy = cooperative-sticky

  10. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  EOS — это комбинация идемпотентного продюсера + транзакций +
      read_committed консюмера.
  2.  Идемпотентность — защита от дублей внутри одной партиции.
      PID + Sequence Number.
  3.  Транзакции — атомарная запись в несколько партиций.
      transactional.id + Coordinator.
  4.  read_committed — консюмер читает только закоммиченные транзакции.
  5.  End-to-end EOS — связка коммита смещения консюмера с транзакцией
      продюсера через SendOffsetsToTxn.
  6.  EOS работает только внутри Kafka. Для внешних систем нужен
      Transactional Outbox или идемпотентность.
  7.  EOS снижает производительность на 30-50%.
  8.  EOS требует согласованной работы продюсера, консюмера и брокеров.
  9.  transactional.id должен быть уникальным в кластере.
  10. Для high-load систем выбирают at-least-once с идемпотентной
      обработкой вместо full EOS.
*/

// 1. КОНФИГУРАЦИЯ ПРОДЮСЕРА И КОНСЬЮМЕРА

const (
	brokers       = "localhost:9092"
	topicFirst    = "orders"
	topicSecond   = "payments"
	consumerGroup = "transactional-group"
)

// producerConfig возвращает настройки для продюсера уровня Middle.
func producerConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0

	// --- Идемпотентность и надёжность ---
	cfg.Producer.Idempotent = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll // acks = all
	cfg.Producer.Retry.Max = int(^uint(0) >> 1)   // retries = MAX (бесконечные ретраи)
	cfg.Producer.Retry.Backoff = 100 * time.Millisecond

	// --- Транзакции ---
	cfg.Producer.Transaction.Timeout = 60 * time.Second
	cfg.Producer.Transaction.ID = "producerTnx" // уникальный ID для транзакций

	// --- Производительность ---
	cfg.Net.MaxOpenRequests = 5
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true

	return cfg
}

// consumerConfig возвращает настройки для консьюмера уровня Middle.
func consumerConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0

	// --- Читаем только закоммиченные транзакции ---
	cfg.Consumer.IsolationLevel = sarama.ReadCommitted

	// --- Ручной коммит офсетов ---
	cfg.Consumer.Offsets.AutoCommit.Enable = false
	cfg.Consumer.Offsets.Retry.Max = 5

	// --- Стратегия перебалансировки (cooperative-sticky) ---
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyCooperativeSticky(),
	}

	// --- Таймауты ---
	cfg.Consumer.MaxWaitTime = 500 * time.Millisecond
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	return cfg
}

// 2. ТРАНЗАКЦИОННЫЙ ПРОДЮСЕР

type TransactionalProducer struct {
	producer sarama.SyncProducer
}

// NewTransactionalProducer создаёт и инициализирует продюсера.
// ВАЖНО: при работе с транзакциями нужно вызвать InitProducerID().
func NewTransactionalProducer() (*TransactionalProducer, error) {
	cfg := producerConfig()
	producer, err := sarama.NewSyncProducer([]string{brokers}, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	// Инициализация PID (Producer ID) для транзакций
	// Это обязательный шаг перед использованием BeginTxn()
	err = producer.InitProducerID()
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("failed to init producer ID: %w", err)
	}
	return &TransactionalProducer{producer: producer}, nil
}

// SendAtomic отправляет два сообщения в одну транзакцию.
// Либо оба сообщения будут доставлены, либо ни одного.
func (tp *TransactionalProducer) SendAtomic(ctx context.Context, key, payload1, payload2 string) error {
	// Начинаем транзакцию
	if err := tp.producer.BeginTxn(); err != nil {
		return fmt.Errorf("begin txn: %w", err)
	}
	defer func() {
		// Если произойдёт паника или ошибка, транзакция будет отменена
		if r := recover(); r != nil {
			tp.producer.AbortTxn()
			panic(r)
		}
	}()

	// Сообщение 1 → топик orders
	msg1 := &sarama.ProducerMessage{
		Topic: topicFirst,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(payload1),
	}
	// Сообщение 2 → топик payments
	msg2 := &sarama.ProducerMessage{
		Topic: topicSecond,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(payload2),
	}

	// Отправляем сообщения в рамках транзакции
	if _, _, err := tp.producer.SendMessage(msg1); err != nil {
		tp.producer.AbortTxn()
		return fmt.Errorf("failed to send msg1: %w", err)
	}
	if _, _, err := tp.producer.SendMessage(msg2); err != nil {
		tp.producer.AbortTxn()
		return fmt.Errorf("failed to send msg2: %w", err)
	}

	// Фиксируем транзакцию
	if err := tp.producer.CommitTxn(); err != nil {
		tp.producer.AbortTxn()
		return fmt.Errorf("commit txn: %w", err)
	}
	return nil
}

// Close закрывает продюсера.
func (tp *TransactionalProducer) Close() error {
	return tp.producer.Close()
}

// 3. КОНСЬЮМЕР С РУЧНЫМ КОММИТОМ
// MiddleConsumerHandler — обработчик сообщений уровня Middle.
type MiddleConsumerHandler struct {
	wg sync.WaitGroup
}

// Setup — вызывается перед началом потребления.
func (h *MiddleConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error {
	log.Println("🔧 Consumer setup: starting")
	return nil
}

// Cleanup — вызывается после завершения потребления.
func (h *MiddleConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	log.Println("🧹 Consumer cleanup: finished")
	return nil
}

// ConsumeClaim — основная логика обработки сообщений.
func (h *MiddleConsumerHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Увеличиваем счётчик горутин (для graceful shutdown)
	h.wg.Add(1)
	defer h.wg.Done()

	for msg := range claim.Messages() {
		// --- 1. Обрабатываем сообщение ---
		log.Printf(" Received: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))

		// Имитация обработки (например, сохранение в БД)
		if err := processMessage(msg); err != nil {
			log.Printf("Processing error: %v", err)
			// В реальном проекте здесь должна быть логика retry или DLQ.
			// Для примера мы просто пропускаем и не коммитим офсет,
			// чтобы сообщение было прочитано снова при следующем запуске.
			continue
		}
		// --- 2. Ручной коммит офсета ---
		// Отмечаем сообщение как обработанное.
		// MarkMessage() отмечает offset, но не коммитит его сразу.
		// Коммит произойдёт при вызове sess.Commit() или по таймеру.
		sess.MarkMessage(msg, "")

		log.Printf("Offset %d marked for commit", msg.Offset)
	}
	return nil
}

// processMessage — бизнес-логика обработки сообщения.
func processMessage(msg *sarama.ConsumerMessage) error {
	// Имитация успешной обработки.
	// В реальном проекте здесь может быть запись в БД или вызов внешнего API.
	if string(msg.Key) == "bad" {
		return fmt.Errorf("simulated processing error")
	}
	return nil
}

// runConsumer запускает консьюмер-группу.
func runConsumer(ctx context.Context) error {
	cfg := consumerConfig()
	consumerGroup, err := sarama.NewConsumerGroup([]string{brokers}, consumerGroup, cfg)
	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	defer consumerGroup.Close()

	handler := &MiddleConsumerHandler{}

	// Запускаем потребление в отдельной горутине.
	go func() {
		for {
			// Consume() блокируется до тех пор, пока не произойдёт ошибка
			// или не будет отменён контекст.
			if err := consumerGroup.Consume(ctx, []string{topicFirst, topicSecond}, handler); err != nil {
				log.Printf("Consumer error: %v", err)
				// В случае ошибки делаем паузу перед повторной попыткой.
				time.Sleep(1 * time.Second)
			}
			// Проверяем, не отменён ли контекст.
			if ctx.Err() != nil {
				log.Println("🛑 Consumer context cancelled")
				return
			}
		}
	}()
	// Ждём, пока все сообщения будут обработаны перед завершением.
	handler.wg.Wait()
	return nil
}

// 4. MAIN — ЗАПУСК И GRACEFUL SHUTDOWN
func main() {
	//1. Инициализация продюсера
	producer, err := NewTransactionalProducer()
	if err != nil {
		log.Fatalf("❌ Producer init: %v", err)
	}
	defer producer.Close()
	log.Println("✅ Transactional producer ready")

	// 2. Отправка двух сообщений в одной транзакции
	ctx := context.Background()
	key := "order-123"
	payload1 := `{"order_id": "123", "user": "Alice", "amount": 100}`
	payload2 := `{"payment_id": "456", "order_id": "123", "amount": 100}`

	if err := producer.SendAtomic(ctx, key, payload1, payload2); err != nil {
		log.Fatalf("Failed to send atomic messages: %v", err)
	}
	log.Println("Transaction committed successfully")

	// 3. Запуск консьюмера
	// Создаём контекст с отменой для graceful shutdown
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()

	go func() {
		if err := runConsumer(consumerCtx); err != nil {
			log.Printf("Consumer error: %v", err)
		}
	}()

	// 4. Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("🛑 Shutting down gracefully...")
	cancelConsumer()

	// Даём время завершить обработку
	time.Sleep(2 * time.Second)
	log.Println("✅ Shutdown complete")
}
