package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/labstack/gommon/log"
)

/*
  БЛОК 2.2: ТРАНЗАКЦИИ В KAFKA
  Транзакции в Kafka позволяют выполнять атомарные операции записи в несколько
  партиций и/или топиков. Это ключевой механизм для достижения exactly-once
  семантики (EOS) в распределённых системах.

  1.  ПРОБЛЕМА: ПОЧЕМУ НУЖНЫ ТРАНЗАКЦИИ
  Идемпотентный продюсер решает проблему дублей в пределах одной партиции.
  Но он не решает проблему атомарности при записи в несколько партиций.

  ПРИМЕР ПРОБЛЕМЫ:
    • Платёжная система: нужно одновременно записать:
      1. Списание средств со счёта пользователя (партиция 0)
      2. Зачисление средств на счёт получателя (партиция 1)
    • Если запись в партицию 0 прошла, а в партицию 1 упала — система
      становится несогласованной (деньги списались, но не зачислились).
    • Без транзакций эту проблему решить невозможно — нужен механизм
      "всё или ничего".

  2.  КОГДА НУЖНЫ ТРАНЗАКЦИИ
  Транзакции в Kafka нужны в следующих сценариях:

  2.1. АТОМАРНАЯ ЗАПИСЬ В НЕСКОЛЬКО ПАРТИЦИЙ
    • Запись данных в несколько топиков или партиций как единое целое.
    • При сбое все изменения откатываются.

  2.2. END-TO-END EXACTLY-ONCE (СВЯЗКА С КОНСЮМЕРОМ)
    • Консюмер читает сообщение из топика A.
    • Обрабатывает его.
    • Отправляет результат в топик B.
    • Коммитит смещение (offset) в топике A.
    • Все эти операции должны быть атомарными: либо все выполнены,
      либо ни одна.

  2.3. ЧТЕНИЕ ТОЛЬКО ЗАКОММИЧЕННЫХ ДАННЫХ
    • Некоторые консюмеры должны видеть только полностью обработанные
      транзакции.
    • Это предотвращает чтение "черновиков" (uncommitted data).

  3.  КЛЮЧЕВЫЕ КОМПОНЕНТЫ ТРАНЗАКЦИЙ

  3.1. TRANSACTIONAL.ID
    • Уникальный идентификатор транзакционного продюсера.
    • Должен быть уникальным в рамках всего кластера Kafka.
    • Используется для восстановления после сбоев.
    • Если продюсер упал и перезапустился с тем же transactional.id,
      он может продолжить незавершённые транзакции.
    • Без transactional.id транзакции невозможны.

  3.2. PRODUCER ID (PID)
    • Уникальный идентификатор, который назначается продюсеру брокером.
    • В транзакционном режиме PID привязывается к transactional.id.
    • Используется для отслеживания последовательности сообщений.

  3.3. PRODUCER EPOCH
    • Счётчик, который увеличивается при каждом изменении PID.
    • Позволяет брокеру отличать новый экземпляр продюсера от старого
      с тем же transactional.id.
    • При сбое и перезапуске продюсер получает новый epoch.

  4.  TRANSACTION COORDINATOR И TRANSACTION LOG

  4.1. TRANSACTION COORDINATOR
    • Специальный брокер в кластере Kafka, который управляет транзакциями.
    • Для каждого transactional.id определяется свой coordinator
      (по хэшу transactional.id).
    • Coordinator отвечает за:
      - Назначение PID.
      - Управление состоянием транзакции.
      - Координацию коммита/отката.

  4.2. TRANSACTION LOG
    • Внутренний топик __transaction_state.
    • Хранит состояние всех транзакций в кластере.
    • Используется для восстановления после сбоев.

  5.  ЖИЗНЕННЫЙ ЦИКЛ ТРАНЗАКЦИИ

  5.1. ИНИЦИАЛИЗАЦИЯ (INITTRANSACTIONS)
    • Продюсер вызывает initTransactions().
    • Coordinator проверяет transactional.id.
    • Назначается PID и epoch.
    • Продюсер готов к началу транзакции.

  5.2. НАЧАЛО ТРАНЗАКЦИИ (BEGINTRANSACTION)
    • Продюсер вызывает beginTransaction().
    • Начинается новая транзакция.
    • Все последующие отправки будут частью этой транзакции.

  5.3. ОТПРАВКА СООБЩЕНИЙ (SEND)
    • Продюсер отправляет сообщения в рамках транзакции.
    • Каждое сообщение помечается:
      - transactional.id
      - PID
      - Sequence number (в пределах партиции)
      - Transaction marker

  5.4. КОММИТ ТРАНЗАКЦИИ (COMMITTRANSACTION)
    • Продюсер вызывает commitTransaction().
    • Coordinator выполняет двухфазный коммит (2PC):
      1. Подготовка (prepare) — все брокеры подтверждают готовность.
      2. Фиксация (commit) — все брокеры записывают транзакцию.
    • Все сообщения становятся видимыми для консюмеров
      (если isolation.level = read_committed).

  5.5. ОТКАТ ТРАНЗАКЦИИ (ABORTTRANSACTION)
    • Продюсер вызывает abortTransaction().
    • Coordinator откатывает транзакцию.
    • Все сообщения транзакции удаляются.

  6.  ИЗОЛЯЦИЯ: READ_COMMITTED VS READ_UNCOMMITTED

  6.1. READ_UNCOMMITTED (ПО УМОЛЧАНИЮ)
    • Консюмер читает все сообщения, включая незакоммиченные.
    • Может увидеть "черновики" данных.
    • Выше производительность.
    • Используется в большинстве систем.

  6.2. READ_COMMITTED
    • Консюмер читает только закоммиченные транзакции.
    • Не видит незакоммиченные данные.
    • Ниже производительность.
    • Требуется для exactly-once семантики.

  7.  СВЯЗКА С КОНСЮМЕРОМ: SENDOFFSETSTOTRANSACTION
  Ключевой механизм для end-to-end exactly-once:

  7.1. ПРОБЛЕМА
    • Консюмер прочитал сообщение из топика A.
    • Обработал его.
    • Отправил результат в топик B.
    • Закоммитил смещение в топике A.
    • Если одна из операций упала — система становится несогласованной.

  7.2. РЕШЕНИЕ
    • Все операции выполняются в рамках одной транзакции.
    • Консюмер передаёт смещения продюсеру.
    • Продюсер включает коммит смещений в транзакцию.

  7.3. КАК ЭТО РАБОТАЕТ
    1. Консюмер читает сообщение (с ручным коммитом).
    2. Продюсер начинает транзакцию.
    3. Продюсер отправляет результат в топик B.
    4. Продюсер вызывает sendOffsetsToTransaction(offsets, groupId).
    5. Продюсер коммитит транзакцию.
    6. Все операции атомарны: либо все выполнены, либо ни одна.

  8.  END-TO-END EXACTLY-ONCE (ПОЛНЫЙ ЦИКЛ)
  Для достижения exactly-once от начала до конца нужно:

  8.1. НА СТОРОНЕ ПРОДЮСЕРА (ВХОДНОЙ)
    • enable.idempotence = true
    • acks = all
    • retries = Integer.MAX_VALUE
    • max.in.flight.requests.per.connection = 5

  8.2. НА СТОРОНЕ БРОКЕРА
    • min.insync.replicas = 2 (минимум 2 реплики в ISR)
    • acks = all (продюсер ждёт подтверждения от всех реплик)

  8.3. НА СТОРОНЕ КОНСЮМЕРА
    • enable.auto.commit = false (ручной коммит)
    • isolation.level = read_committed (читать только закоммиченные транзакции)

  8.4. НА СТОРОНЕ ПРОДЮСЕРА (ВЫХОДНОЙ)
    • transactional.id = "unique-id"
    • enable.idempotence = true
    • acks = all

  9.  НАСТРОЙКИ ДЛЯ ПРОДАКШЕНА

  9.1. ПРОДЮСЕР
    // Конфигурация транзакционного продюсера
    config.Producer.Idempotent = true
    config.Producer.RequiredAcks = sarama.WaitForAll
    config.Producer.Retry.Max = 5
    config.Producer.MaxInFlightRequests = 5 // sarama сама установит
    config.Producer.Transaction.ID = "my-transactional-id"

  9.2. КОНСЮМЕР
    // Конфигурация консюмера для exactly-once
    config.Consumer.Offsets.Initial = sarama.OffsetOldest
    config.Consumer.Offsets.AutoCommit.Enable = false
    config.Consumer.IsolationLevel = sarama.ReadCommitted

  10.  ОГРАНИЧЕНИЯ И ПОДВОДНЫЕ КАМНИ

  10.1. ПРОИЗВОДИТЕЛЬНОСТЬ
    • Транзакции снижают пропускную способность на 30-50%.
    • Требуют дополнительных сетевых обменов (2PC).
    • Не подходят для high-load систем.

  10.2. ТРЕБОВАНИЯ К ИДЕМПОТЕНТНОСТИ
    • Транзакции требуют идемпотентного продюсера.
    • Без enable.idempotence=true транзакции не работают.

  10.3. УНИКАЛЬНОСТЬ TRANSACTIONAL.ID
    • transactional.id должен быть уникальным в кластере.
    • Два продюсера с одинаковым transactional.id не могут работать
      одновременно.

  10.4. ВОССТАНОВЛЕНИЕ ПОСЛЕ СБОЕВ
    • При сбое продюсера транзакция может остаться незавершённой.
    • Coordinator автоматически завершает транзакцию после таймаута
      (transaction.timeout.ms).

  10.5. ИЗОЛЯЦИЯ READ_COMMITTED
    • Консюмеры с isolation.level=read_committed читают только закоммиченные
      сообщения, что увеличивает задержку.

  11.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  Транзакции решают проблему атомарной записи в несколько партиций.
  2.  Ключевые компоненты: transactional.id, PID, Epoch, Transaction Coordinator.
  3.  Transaction Coordinator управляет состоянием транзакций.
  4.  Жизненный цикл: initTransactions() → beginTransaction() → send() →
      commitTransaction() / abortTransaction().
  5.  sendOffsetsToTransaction() связывает коммит смещения консюмера
      с транзакцией продюсера.
  6.  isolation.level = read_committed позволяет читать только закоммиченные транзакции.
  7.  Транзакции требуют идемпотентного продюсера (enable.idempotence=true).
  8.  transactional.id должен быть уникальным в кластере.
  9.  Транзакции снижают производительность на 30-50%.
  10. Для end-to-end exactly-once нужно согласовать продюсера, консюмера
      и брокера.
*/

//КОНФИГУРАЦИЯ

const (
	broker        = "localhost:9092"
	inputTopic    = "orders-input"
	outputTopic   = "orders-output"
	consumerGroup = "order-processor"
)

var (
	mode        = flag.String("mode", "producer", "Режим: producer или consumer")
	shouldAbort = flag.Bool("abort", false, "Для producer: откатить транзакцию вместо коммита")
)

//ОБЩИЕ УТИЛИТЫ

func createTopicsIfNotExist() error {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	admin, err := sarama.NewClusterAdmin([]string{broker}, config)
	if err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}
	defer admin.Close()

	topics := []string{inputTopic, outputTopic}
	for _, topic := range topics {
		exists := false
		list, _ := admin.ListTopics()
		if _, ok := list[topic]; ok {
			exists = true
		}
		if !exists {
			err := admin.CreateTopic(topic, &sarama.TopicDetail{
				NumPartitions:     1,
				ReplicationFactor: 1,
			}, false)
			if err != nil {
				return fmt.Errorf("failed to create topic %s: %w", topic, err)
			}
			log.Printf("Топик %s создан", topic)
		} else {
			log.Printf("Топик %s уже существует", topic)
		}
	}
	return nil
}

// ПРОДЮСЕР
type TransactionalProducer struct {
	client sarama.SyncProducer
}

func NewTransactionalProducer(transactionalID string) (*TransactionalProducer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	// Настройки для транзакций
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Transaction.ID = transactionalID

	client, err := sarama.NewSyncProducer([]string{broker}, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	return &TransactionalProducer{client: client}, nil
}

func (p *TransactionalProducer) Close() error {
	return p.client.Close()
}

// ProcessOrder обрабатывает заказ в рамках транзакции.
func (p *TransactionalProducer) ProcessOrder(orderID string, amount float64, offset int64, partition int32) error {
	// Начинаем транзакцию
	if err := p.client.BeginTxn(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// 1. Отправляем сообщение в output-топик (результат обработки)
	outMsg := &sarama.ProducerMessage{
		Topic: outputTopic,
		Key:   sarama.StringEncoder(orderID),
		Value: sarama.StringEncoder(fmt.Sprintf(`{"order_id":"%s","amount":%.2f,"status":"processed"}`, orderID, amount)),
	}
	if _, _, err := p.client.SendMessage(outMsg); err != nil {
		if abortErr := p.client.AbortTxn(); abortErr != nil {
			return fmt.Errorf("failed to abort transaction: %w", abortErr)
		}
		return fmt.Errorf("failed to send output message: %w", err)
	}

	// 2. Коммитим или откатываем
	if *shouldAbort {
		p.client.AbortTxn()
		log.Printf("⚠️ Транзакция откатана")
		return nil
	}

	if err := p.client.CommitTxn(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	log.Printf("✅ Транзакция закоммичена")
	return nil
}

func runProducer() error {
	log.Print("запуск транзакционного продюсера")

	if err := createTopicsIfNotExist(); err != nil {
		return fmt.Errorf("ошибка создания топиков: %w", err)
	}

	producer, err := NewTransactionalProducer("order-processor-1")
	if err != nil {
		return fmt.Errorf("ошибка создания продюсера: %w", err)
	}
	defer producer.Close()

	// Имитация заказов
	orders := []struct {
		ID     string
		Amount float64
	}{
		{"order-1", 100.50},
		{"order-2", 250.00},
		{"order-3", 75.30},
	}

	for _, order := range orders {
		log.Printf("📦 Обработка заказа: %s (сумма: %.2f)", order.ID, order.Amount)

		// Имитация offset и partition (для демонстрации sendOffsetsToTransaction)
		offset := int64(time.Now().UnixNano())
		partition := int32(0)

		err := producer.ProcessOrder(order.ID, order.Amount, offset, partition)
		if err != nil {
			log.Printf("Ошибка обработки заказа %s: %v", order.ID, err)
		}
		time.Sleep(1 * time.Second)
	}

	log.Print("Продюсер завершил работу")
	return nil
}

//КОНСЮМЕР

type ConsumerHandler struct {
	ready chan bool
}

// Setup запускается при старте консюмера
func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

// Cleanup вызывается при завершении
func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim обрабатывает сообщения из партиции
func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("📨 Получено сообщение: key=%s, value=%s, partition=%d, offset=%d",
			string(msg.Key), string(msg.Value), msg.Partition, msg.Offset)

		// В реальном проекте здесь будет вызов транзакционного продюсера.
		// Для демонстрации мы просто коммитим смещение.
		success := true // симуляция успешной обработки
		if success {
			session.MarkMessage(msg, "")
			log.Printf("Сообщение обработано и закоммичено: offset=%d", msg.Offset)
		} else {
			log.Printf("Ошибка обработки, смещение не коммитится")
		}
	}
	return nil
}

// runConsumer запускает консюмера с read_committed.
func runConsumer() error {
	log.Print("Запуск консюмера с read_committed")

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	//КРИТИЧЕСКИ ВАЖНО: читаем только закоммиченные транзакции
	config.Consumer.IsolationLevel = sarama.ReadCommitted
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup([]string{broker}, consumerGroup, config)
	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &ConsumerHandler{ready: make(chan bool)}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		log.Print("Получен сигнал, завершаем...")
		cancel()
	}()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := client.Consume(ctx, []string{inputTopic}, handler); err != nil {
				log.Printf("Ошибка консюмера: %v", err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	<-handler.ready
	log.Print("Консюмер готов, ожидание сообщений...")

	wg.Wait()
	log.Print("Консюмер остановлен")
	return nil
}

func main() {
	flag.Parse()

	if *mode != "producer" && *mode != "consumer" {
		log.Fatalf("Неверный режим: %s. Используйте -mode=producer или -mode=consumer", *mode)
	}

	var err error
	if *mode == "producer" {
		err = runProducer()
	} else {
		err = runConsumer()
	}

	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
