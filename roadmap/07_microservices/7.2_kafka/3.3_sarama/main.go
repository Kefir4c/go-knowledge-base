package sarama

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

/*
  БЛОК 3.3: IBM/SARAMA
  IBM/sarama (ранее Shopify/sarama) — это одна из старейших и наиболее
  зрелых Go-библиотек для работы с Apache Kafka. Она существует с 2013 года
  и прошла проверку тысячами продакшен-систем.

  1.  В ДВУХ СЛОВАХ: ЧТО ЭТО И ОТКУДА
  • Разработана компанией Shopify для своих внутренних нужд.
  • Сейчас активно поддерживается IBM (перешла под их крыло).
  • Чистый Go (без CGO) — легко собирается, нет зависимости от C-компилятора.
  • Одна из самых производительных Pure Go-реализаций.
  • Поддерживает все ключевые фичи Kafka, включая транзакции и идемпотентность.
  • Имеет как низкоуровневый, так и высокоуровневый API.

  2.  КЛЮЧЕВЫЕ ОСОБЕННОСТИ (ПОЧЕМУ ВЫБИРАЮТ SARAMA)

  2.1. ЧИСТЫЙ GO, БЕЗ CGO
    • Нет зависимости от C-компилятора (GCC/Clang).
    • Сборка и деплой просты, работает везде, где работает Go.
    • Не требует установки librdkafka.

  2.2. ПОДДЕРЖКА ТРАНЗАКЦИЙ И ИДЕМПОТЕНТНОСТИ
    • В отличие от segmentio/kafka-go, sarama поддерживает транзакции.
    • Поддерживает идемпотентный продюсер (enable.idempotence=true).
    • Можно строить exactly-once семантику (EOS).

  2.3. ДВА УРОВНЯ API
    • Низкоуровневый (sarama) — полный контроль, но сложнее.
    • Высокоуровневый (sarama-cluster, теперь встроен) — упрощает работу
      с consumer groups, управление смещениями, ребалансировку.

  2.4. ВЫСОКАЯ ПРОИЗВОДИТЕЛЬНОСТЬ
    • Бенчмарки показывают, что sarama уступает только CGO-реализациям
      (confluent-kafka-go) и обходит segmentio/kafka-go.
    • Хорошо подходит для нагрузки 50k-100k msg/s.

  3.  СРАВНЕНИЕ С ДРУГИМИ БИБЛИОТЕКАМИ (ДЛЯ СОБЕСЕДОВАНИЯ)
  ┌─────────────────────┬──────────────────┬──────────────────┬──────────────────┐
  │ Характеристика      │ sarama           │ confluent        │ kafka-go         │
  │                     │ (IBM)            │ (CGO)            │ (segmentio)      │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Реализация          │ Pure Go          │ CGO (librdkafka) │ Pure Go          │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Производительность  │ Высокая          │ Самая высокая    │ Средняя          │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Транзакции / EOS    │ Да               │ Да               │ Нет              │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Поддержка context   │ Нет (только      │ Нет              │ Да               │
  │                     │ через каналы)    │                  │                  │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Сложность API       │ Средняя/Высокая  │ Высокая          │ Низкая           │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Зрелость            │ Очень высокая    │ Высокая          │ Средняя          │
  └─────────────────────┴──────────────────┴──────────────────┴──────────────────┘

  4.  КОГДА ВЫБИРАТЬ SARAMA (И КОГДА НЕТ)

  ВЫБИРАЙ, ЕСЛИ:
    + Тебе нужен чистый Go без CGO.
    + Нужны транзакции и exactly-once семантика.
    + Проект enterprise-уровня (за ним стоит IBM).
    + Нужна высокая производительность (50k–100k msg/s).
    + Ты готов к более сложному API (но оно того стоит).

  НЕ ВЫБИРАЙ, ЕСЛИ:
    - Тебе нужна поддержка Go context (тогда бери kafka-go).
    - Тебе нужна максимальная производительность (>100k msg/s) — тогда confluent.
    - Тебе нужен максимально простой API — тогда kafka-go.
    - Нет требований к транзакциям — kafka-go проще.

  5.  ОСНОВНЫЕ КОМПОНЕНТЫ И ИХ API

  5.1. PRODUCER
    syncProducer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
    // или asyncProducer, err := sarama.NewAsyncProducer(...)

    // SyncProducer — отправка с блокировкой до получения подтверждения.
    partition, offset, err := syncProducer.SendMessage(&sarama.ProducerMessage{
        Topic: "my-topic",
        Key:   sarama.StringEncoder("key"),
        Value: sarama.StringEncoder("value"),
    })

    // AsyncProducer — асинхронная отправка через каналы.
    asyncProducer.Input() <- &sarama.ProducerMessage{...}
    // Читаем успехи/ошибки из каналов asyncProducer.Successes() / Errors()

  5.2. CONSUMER
    consumer, err := sarama.NewConsumer([]string{"localhost:9092"}, config)
    partitionConsumer, err := consumer.ConsumePartition("my-topic", 0, sarama.OffsetNewest)
    for msg := range partitionConsumer.Messages() {
        // обрабатываем msg
    }

  5.3. CONSUMER GROUP (ВЫСОКОУРОВНЕВЫЙ API)
    // Создаём consumer group
    group, err := sarama.NewConsumerGroup([]string{"localhost:9092"}, "my-group", config)
    // Реализуем интерфейс ConsumerGroupHandler
    type handler struct{}

    func (h *handler) Setup(session sarama.ConsumerGroupSession) error   { return nil }
    func (h *handler) Cleanup(session sarama.ConsumerGroupSession) error { return nil }
    func (h *handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
        for msg := range claim.Messages() {
            // обрабатываем
            session.MarkMessage(msg, "")
        }
        return nil
    }

    // Запускаем потребление
    group.Consume(ctx, []string{"my-topic"}, &handler{})

  5.4. ADMIN CLIENT
    admin, err := sarama.NewClusterAdmin([]string{"localhost:9092"}, config)
    // Создание топика
    admin.CreateTopic("new-topic", &sarama.TopicDetail{
        NumPartitions:     3,
        ReplicationFactor: 1,
    }, false)

  6.  КОНФИГУРАЦИЯ ДЛЯ ПРОДАКШЕНА

  6.1. ПРОДЮСЕР (с идемпотентностью и транзакциями)
    config := sarama.NewConfig()
    config.Version = sarama.V3_0_0_0
    config.Producer.Idempotent = true
    config.Producer.RequiredAcks = sarama.WaitForAll
    config.Producer.Retry.Max = 5
    config.Producer.Return.Successes = true
    config.Producer.Return.Errors = true
    config.Producer.MaxInFlightRequests = 5  // ≤ 5 для порядка при идемпотентности
    config.Producer.Transaction.ID = "my-txn-id" // для транзакций

  6.2. КОНСЮМЕР (consumer group)
    config := sarama.NewConfig()
    config.Version = sarama.V3_0_0_0
    config.Consumer.Offsets.Initial = sarama.OffsetOldest
    config.Consumer.Offsets.AutoCommit.Enable = false // ручной коммит
    config.Consumer.Return.Errors = true
    config.Consumer.IsolationLevel = sarama.ReadCommitted // для EOS

  7.  ТРАНЗАКЦИИ В SARAMA (ДЕТАЛИ)
  Sarama поддерживает транзакции практически так же, как и confluent.

    // 1. Настройка продюсера с transactional.id
    config.Producer.Transaction.ID = "my-txn"

    // 2. Инициализация транзакций (получение PID)
    producer, _ := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
    producer.InitTransactions()

    // 3. Начало транзакции
    producer.BeginTransaction()

    // 4. Отправка сообщений
    producer.SendMessage(msg1)
    producer.SendMessage(msg2)

    // 5. Коммит или откат
    producer.CommitTransaction()
    // или producer.AbortTransaction()

    // Важно: потребитель должен быть с isolation.level=read_committed.

  8.  ГЛАВНЫЕ ГРАБЛИ (ПОДВОДНЫЕ КАМНИ)

  8.1. НЕТ CONTEXT
    Sarama не поддерживает Go context. Для таймаутов и отмены используйте
    каналы, select и time.After.

    //НЕ РАБОТАЕТ (context игнорируется)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    producer.SendMessage(msg)

    //РАБОТАЕТ — через каналы
    done := make(chan error, 1)
    go func() {
        _, _, err := producer.SendMessage(msg)
        done <- err
    }()
    select {
    case err := <-done:
        // обработка
    case <-time.After(5 * time.Second):
        // таймаут
    }

  8.2. СЛОЖНЫЙ API
    Sarama имеет низкоуровневый API, который требует понимания внутреннего
    устройства Kafka. Для новичков это может быть сложно.

  8.3. УПРАВЛЕНИЕ ПАМЯТЬЮ
    При использовании AsyncProducer нужно обязательно читать из каналов
    Successes() и Errors(), иначе произойдёт утечка памяти.

  8.4. ВЕРСИИ
    Убедитесь, что config.Version соответствует версии вашего кластера Kafka.
    Для Kafka 3.0+ используйте sarama.V3_0_0_0.

  9. ВОПРОС
  Вопрос: "Какую библиотеку для Kafka в Go ты используешь и почему?"

  Ответ:
  "Мы используем IBM/sarama. Это чистый Go-клиент, поэтому у нас нет
  проблем с CGO и сборкой. Он поддерживает транзакции и идемпотентность,
  что критично для нашего проекта (нам нужна exactly-once семантика).
  Производительность sarama достаточно высокая для нашей нагрузки (~50k msg/s).
  Да, API сложнее, чем у kafka-go, но это даёт нам гибкость. Если бы нам
  не нужны были транзакции, мы бы рассмотрели kafka-go."

  10. КЛЮЧЕВЫЕ ВЫВОДЫ
  1.  Sarama — чистый Go, без CGO.
  2.  Поддерживает транзакции и идемпотентность.
  3.  Два уровня API: низкоуровневый и высокоуровневый (consumer group).
  4.  Нет поддержки context (нужно использовать каналы).
  5.  Выбирай для enterprise-проектов с транзакциями.
  6.  Не выбирай для простых задач без транзакций.
  7.  Производительность высокая (уступает только confluent).
  8.  Активно поддерживается IBM.
*/

//КОНФИГУРАЦИЯ

const (
	defaultBroker  = "localhost:9092"
	defaultTopic   = "orders"
	defaultGroupID = "order-processor"
)

var (
	mode    = flag.String("mode", "producer-sync", "Режим: producer-sync, producer-async, producer-txn, consumer")
	broker  = flag.String("broker", defaultBroker, "Адрес брокера")
	topic   = flag.String("topic", defaultTopic, "Топик")
	groupID = flag.String("group", defaultGroupID, "ID группы")
)

//ОБЩИЙ КОНФИГ

func baseConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	return config
}

// SYNCPRODUCER (С ИДЕМПОТЕНТНОСТЬЮ)
func runProducerSync() {
	log.Println("Запуск SyncProducer с идемпотентностью")

	config := baseConfig()
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Compression = sarama.CompressionSnappy

	producer, err := sarama.NewSyncProducer([]string{*broker}, config)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	// Отправляем 10 сообщений
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("order-%d", i)
		msg := &sarama.ProducerMessage{
			Topic: *topic,
			Key:   sarama.StringEncoder(key),
			Value: sarama.StringEncoder(value),
		}
		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			continue
		}
		log.Printf("Отправлено: key=%s, value=%s, partition=%d, offset=%d",
			key, value, partition, offset)
		time.Sleep(200 * time.Millisecond)
	}
	log.Println("SyncProducer завершён")
}

// ASYNCPRODUCER
func runProducerAsync() {
	log.Println("Запуск AsyncProducer")

	config := baseConfig()
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 5
	config.Producer.Compression = sarama.CompressionSnappy

	producer, err := sarama.NewAsyncProducer([]string{*broker}, config)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	// Обработка успехов и ошибок
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for msg := range producer.Successes() {
			log.Printf("Доставлено: topic=%s, partition=%d, offset=%d",
				msg.Topic, msg.Partition, msg.Offset)
		}
	}()

	go func() {
		defer wg.Done()
		for err := range producer.Errors() {
			log.Printf("Ошибка доставки: %v", err)
		}
	}()

	// Отправляем 100 сообщений асинхронно
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("async-key-%d", i)
		value := fmt.Sprintf("async-order-%d", i)
		producer.Input() <- &sarama.ProducerMessage{
			Topic: *topic,
			Key:   sarama.StringEncoder(key),
			Value: sarama.StringEncoder(value),
		}
		if i%10 == 0 {
			log.Printf("Отправлено %d сообщений", i)
		}
	}
	// Закрываем продюсера (дожидается доставки)
	producer.AsyncClose()
	wg.Wait()
	log.Println("AsyncProducer завершён")
}

//ТРАНЗАКЦИОННЫЙ ПРОДЮСЕР

func runProducerTxn() {
	log.Println("Запуск транзакционного продюсера")

	config := baseConfig()
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Transaction.ID = "my-txn-producer"

	producer, err := sarama.NewAsyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Fatalf("❌ Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	// Обработка успехов и ошибок (обязательно!)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range producer.Successes() {
			// логируем успехи, если нужно
		}
	}()

	go func() {
		defer wg.Done()
		for err := range producer.Errors() {
			log.Printf("❌ Ошибка: %v", err)
		}
	}()

	// Начинаем транзакцию
	if err := producer.BeginTxn(); err != nil {
		log.Fatalf("❌ Ошибка начала транзакции: %v", err)
	}
	log.Println("🔓 Транзакция начата")

	// Отправляем сообщения в транзакции
	for i := 0; i < 5; i++ {
		producer.Input() <- &sarama.ProducerMessage{
			Topic: "my-topic",
			Key:   sarama.StringEncoder("key"),
			Value: sarama.StringEncoder("value"),
		}
	}

	// Коммитим транзакцию
	if err := producer.CommitTxn(); err != nil {
		producer.AbortTxn()
		log.Fatalf("Ошибка коммита: %v", err)
	}
	log.Println("Транзакция закоммичена")

	producer.AsyncClose()
	wg.Wait()
}

// CONSUMER GROUP
type ConsumerHandler struct {
	ready  chan bool
	mu     sync.Mutex
	closed bool
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	log.Println("Setup: назначены партиции")
	close(h.ready)
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	log.Println("Cleanup: ребалансировка или завершение")
	session.Commit()
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Получено: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))

		// Имитация обработки
		time.Sleep(100 * time.Millisecond)

		// Ручной коммит смещения
		session.MarkMessage(msg, "")
		log.Printf("Закоммичено offset %d", msg.Offset)
	}
	return nil
}

func runConsumer() {
	log.Println("Запуск Consumer Group")

	config := baseConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategySticky(), // приоритетная стратегия
	}
	config.Consumer.Group.Rebalance.Timeout = 60 * time.Second
	config.Consumer.Group.Rebalance.Retry.Max = 4
	config.Consumer.Group.Rebalance.Retry.Backoff = 2 * time.Second

	config.Consumer.Group.Session.Timeout = 30 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 5 * time.Second

	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false // ручной коммит
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup([]string{*broker}, *groupID, config)
	if err != nil {
		log.Fatalf("❌ Ошибка создания consumer group: %v", err)
	}
	defer client.Close()

	// Канал для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	go func() {
		<-sigCh
		log.Println("⏳ Получен сигнал завершения...")
		cancel()
	}()

	handler := &ConsumerHandler{ready: make(chan bool)}
	go func() {
		for {
			if err := client.Consume(ctx, []string{*topic}, handler); err != nil {
				log.Printf("Ошибка консюмера: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	<-handler.ready
	log.Println("Консюмер готов, ожидание сообщений...")

	<-ctx.Done()
	log.Println("Консюмер завершён")
}

func main() {
	flag.Parse()

	switch *mode {
	case "producer-sync":
		runProducerSync()
	case "producer-async":
		runProducerAsync()
	case "producer-txn":
		runProducerTxn()
	case "consumer":
		runConsumer()
	default:
		log.Fatalf("Неизвестный режим: %s. Используйте: producer-sync, producer-async, producer-txn, consumer", *mode)
	}
}
