package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

/*
  БЛОК 3.1: CONFLUENT-KAFKA-GO
  confluent-kafka-go — официальный Go-клиент для Kafka от компании Confluent.
  Это не чистый Go, а обёртка над высокопроизводительной C++-библиотекой
  librdkafka.

  1.  В ДВУХ СЛОВАХ: ЧТО ЭТО И ЗАЧЕМ
  confluent-kafka-go — это самый быстрый и самый "полный" клиент для Kafka.
  Если тебе нужны:
    • Максимальная производительность (> 100k msg/s)
    • Транзакции и exactly-once (EOS)
    • 100% совместимость с Confluent Cloud
    • Все фичи Kafka (Admin API, все версии, все типы сжатия)
  — бери confluent-kafka-go.

  Если тебе НЕ нужна максимальная производительность — возьми sarama
  (чистый Go, проще сборка) или kafka-go (ещё проще, но нет транзакций).

  2.  КЛЮЧЕВОЕ: CGO (ПОЧЕМУ ЭТО ВАЖНО)
  confluent-kafka-go использует CGO — Go-код вызывает C++-код.

  ЧТО ЭТО ДАЁТ:
    + Скорость: librdkafka (C++) — одна из самых быстрых реализаций.
    + Надёжность: код проверен миллионами продакшен-систем.
    + Полнота: все фичи Kafka доступны "из коробки".

  ЧТО ЭТО УСЛОЖНЯЕТ:
    - Сборка: нужен C-компилятор (GCC/Clang).
    - Alpine Linux: нужно ставить пакеты g++ и librdkafka-dev.
    - Отладка: ошибки могут быть в C-коде, сложнее трассировать.
    - Нет context: не работает привычный context.WithTimeout().

  ВЫВОД: CGO — это компромисс. Ты получаешь скорость и полную поддержку
  фич, но платишь сложностью сборки и отсутствием context.

  3.  СРАВНЕНИЕ С ДРУГИМИ БИБЛИОТЕКАМИ (ЧТО СПРОСЯТ)
  ┌─────────────────────┬──────────────────┬──────────────────┬──────────────────┐
  │ Характеристика      │confluent-kafka-go│ sarama           │ kafka-go         │
  │                     │                  │ (IBM)            │ (segmentio)      │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Реализация          │ CGO (librdkafka) │ Pure Go          │ Pure Go          │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Производительность  │ Самая высокая    │ Высокая          │ Средняя          │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Транзакции / EOS    │ Да               │ Да               │ Нет              │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Поддержка context   │ Нет              │ Нет              │ Да               │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Сложность сборки    │ Высокая          │ Низкая           │ Низкая           │
  ├─────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Когда использовать  │ High-load,       │ Enterprise,      │ Простые сценарии,│
  │                     │ Enterprise       │ транзакции       │ без транзакций   │
  └─────────────────────┴──────────────────┴──────────────────┴──────────────────┘

  4.  ОСНОВНЫЕ КОМПОНЕНТЫ (ЧТО ТЫ БУДЕШЬ ИСПОЛЬЗОВАТЬ)

  4.1. PRODUCER — отправляет сообщения
    p, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "acks": "all",
        "enable.idempotence": true,
    })
    // Обязательно закрывай! defer p.Close()

    // Асинхронная отправка
    p.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
        Value: []byte("hello"),
    }, nil)

    // Обработка событий доставки
    for e := range p.Events() {
        switch ev := e.(type) {
        case *kafka.Message:
            if ev.TopicPartition.Error != nil {
                // ошибка доставки
            } else {
                // успешно доставлено
            }
        }
    }

    // Дождаться доставки всех сообщений
    p.Flush(15 * 1000) // таймаут 15 секунд

  4.2. CONSUMER — читает сообщения
    c, err := kafka.NewConsumer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "group.id": "my-group",
        "auto.offset.reset": "earliest",
    })
    // Обязательно закрывай! defer c.Close()

    c.SubscribeTopics([]string{"my-topic"}, nil)

    for {
        msg, err := c.ReadMessage(-1) // ждём бесконечно
        if err != nil {
            continue
        }
        // обрабатываем msg
    }

  4.3. ADMIN CLIENT — управление топиками
    admin, err := kafka.NewAdminClient(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
    })
    defer admin.Close()

    // Создание топика
    admin.CreateTopics(context.Background(), []kafka.TopicSpecification{{
        Topic: "new-topic",
        NumPartitions: 3,
        ReplicationFactor: 1,
    }}, nil)

  5.  ТРАНЗАКЦИИ (ДЛЯ EXACTLY-ONCE)
  Если тебе нужна exactly-once семантика (без дублей и без потерь):

    // 1. Настрой продюсера
    p, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "enable.idempotence": true,
        "transactional.id": "my-txn-id", // уникальный ID
    })

    // 2. Инициализация транзакций (делается один раз)
    p.InitTransactions(nil)

    // 3. Транзакция: начало → отправка → коммит/откат
    p.BeginTransaction()
    p.Produce(msg1, nil)
    p.Produce(msg2, nil)
    p.CommitTransaction(nil) // или p.AbortTransaction(nil)

  Консюмер должен быть настроен на чтение только закоммиченных транзакций:
    isolation.level = read_committed

  6.  ГЛАВНЫЕ ГРАБЛИ (ЧТО МОЖЕТ УБИТЬ)

  6.1. НЕТ CONTEXT
    В confluent-kafka-go нет поддержки context.Context.

    // ЭТО НЕ РАБОТАЕТ
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    producer.Produce(msg, nil) // context игнорируется

    // ТАК НАДО — через каналы и time.After
    done := make(chan bool)
    go func() {
        producer.Produce(msg, nil)
        done <- true
    }()
    select {
    case <-done:
        // успех
    case <-time.After(5 * time.Second):
        // таймаут
    }

  6.2. УТЕЧКИ ПАМЯТИ
    Если не обрабатывать события из p.Events(), C-память не освобождается.

    // ПЛОХО — память утекает
    p.Produce(msg, nil)

    // ХОРОШО — обрабатываем все события
    go func() {
        for e := range p.Events() {
            // даже если не нужны отчёты, прочитай их из канала
            _ = e
        }
    }()
    // Или отключи отчёты о доставке:
    configMap.SetKey("go.delivery.reports", false)

  6.3. БЛОКИРОВКА ПРИ ПЕРЕПОЛНЕНИИ
    Если очередь продюсера заполнена, Produce() блокируется.
    // Решение: увеличить очередь или использовать асинхронную отправку
    configMap.SetKey("queue.buffering.max.messages", 100000)

  6.4. CGO В ALPINE
    В Alpine Linux (минималистичный Docker) нужно ставить пакеты:
      apk add g++ librdkafka-dev

  7.  КОГДА ВЫБИРАТЬ CONFLUENT-KAFKA-GO

  ВЫБИРАЙ, ЕСЛИ:
    + Тебе нужна максимальная производительность.
    + Ты используешь Confluent Cloud.
    + Тебе нужны транзакции и exactly-once.
    + Ты готов к CGO и усложнённой сборке.
  НЕ ВЫБИРАЙ, ЕСЛИ:
    - У тебя простая нагрузка (< 10k msg/s).
    - Ты работаешь в окружении без C-компилятора (Alpine без GCC).
    - Тебе нужна поддержка Go context.
    - Ты хочешь минимизировать зависимости.

 9.  ПРОДВИНУТЫЕ НАСТРОЙКИ ПРОИЗВОДИТЕЛЬНОСТИ

  9.1. БАТЧИНГ И ЗАДЕРЖКИ
    // Для максимальной пропускной способности:
    configMap.SetKey("batch.size", 1000000)      // 1MB
    configMap.SetKey("linger.ms", 10)            // ждём 10мс для накопления

    // Для минимальной задержки:
    configMap.SetKey("batch.size", 1)            // минимальный батч
    configMap.SetKey("linger.ms", 0)             // отправляем сразу

    // Сжатие для уменьшения трафика:
    configMap.SetKey("compression.type", "snappy")

  9.2. НАСТРОЙКИ ДЛЯ HIGH-LOAD
    configMap.SetKey("queue.buffering.max.messages", 100000)   // очередь 100k
    configMap.SetKey("queue.buffering.max.ms", 1000)           // макс. время в очереди
    configMap.SetKey("max.in.flight.requests.per.connection", 5) // параллельные запросы

  9.3. НАСТРОЙКИ ДЛЯ НАДЁЖНОСТИ
    configMap.SetKey("enable.idempotence", true)               // защита от дублей
    configMap.SetKey("acks", "all")                            // ждём все реплики
    configMap.SetKey("retries", 2147483647)                    // бесконечные ретраи
    configMap.SetKey("retry.backoff.ms", 100)                  // 100мс между ретраями

  10. ПРОДВИНУТЫЕ НАСТРОЙКИ КОНСЮМЕРА (ДОПОЛНЕНИЕ)

  10.1. УПРАВЛЕНИЕ LAG (ОТСТАВАНИЕМ)
    // Увеличиваем fetch размер для большей пропускной способности
    configMap.SetKey("fetch.min.bytes", 1024*1024)    // 1MB
    configMap.SetKey("fetch.max.bytes", 50*1024*1024) // 50MB
    configMap.SetKey("max.partition.fetch.bytes", 10*1024*1024) // 10MB на партицию

    // Увеличиваем интервал между опросами
    configMap.SetKey("max.poll.interval.ms", 300000)  // 5 минут
    configMap.SetKey("session.timeout.ms", 45000)     // 45 секунд

  10.2. РУЧНОЙ КОММИТ СМЕЩЕНИЙ
    configMap.SetKey("enable.auto.commit", false)
    // В цикле обработки:
    msg, err := consumer.ReadMessage(-1)
    // обработка
    // после успешной обработки:
    consumer.Commit() // или CommitMessage(msg)

  10.3. ОБРАБОТКА РЕБАЛАНСИРОВКИ
    // При ребалансировке нужно коммитить смещения
    consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "group.id": "my-group",
        "auto.offset.reset": "earliest",
        "enable.auto.commit": false,
        "go.events.channel.enable": true,
    })
    for {
        select {
        case ev := <-consumer.Events():
            switch e := ev.(type) {
            case kafka.AssignedPartitions:
                consumer.Assign(e.Partitions)
            case kafka.RevokedPartitions:
                consumer.Commit()
                consumer.Unassign()
            case *kafka.Message:
                // обработка
                consumer.Commit()
            }
        }
    }

  11. ЗАГОЛОВКИ И ТРАССИРОВКА

  11.1. ДОБАВЛЕНИЕ ЗАГОЛОВКОВ
    msg := &kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
        Value: []byte("hello"),
        Headers: []kafka.Header{
            {Key: "trace_id", Value: []byte(traceID)},
            {Key: "span_id", Value: []byte(spanID)},
            {Key: "user_id", Value: []byte(userID)},
        },
    }

  11.2. ЧТЕНИЕ ЗАГОЛОВКОВ В КОНСЮМЕРЕ

    for _, h := range msg.Headers {
        switch string(h.Key) {
        case "trace_id":
            traceID = string(h.Value)
        case "user_id":
            userID = string(h.Value)
        }
    }

  12. ИНТЕГРАЦИЯ СО SCHEMA REGISTRY
  Для работы с Avro/Protobuf/JSON схемами используй:
    • github.com/confluentinc/confluent-kafka-go/schemaregistry
    • Сериализаторы/десериализаторы для Avro, Protobuf, JSON

  ПРИМЕР С AVRO:

    import "github.com/confluentinc/confluent-kafka-go/schemaregistry/serde/avro"

    // Создаём сериализатор
    ser, err := avro.NewSerializer(srClient, avro.NewSerializerConfig())

    // Сериализуем сообщение
    payload, err := ser.Serialize(topic, &message)

    // Отправляем в Kafka
    p.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
        Value: payload,
    }, nil)

  13. МОНИТОРИНГ И МЕТРИКИ

  13.1. ПОЛУЧЕНИЕ СТАТИСТИКИ
    // Получить статистику через событие
    for e := range producer.Events() {
        if stats, ok := e.(kafka.Stats); ok {
            fmt.Printf("Статистика: %s\n", stats)
        }
    }

  13.2. КЛЮЧЕВЫЕ МЕТРИКИ ДЛЯ МОНИТОРИНГА
    • queue_buffering_msgs — количество сообщений в очереди
    • outbuf_msgs — сообщения в исходящем буфере
    • waitresp_msgs — сообщения, ожидающие ответа от брокера
    • txmsgs — всего отправлено сообщений
    • txbytes — всего отправлено байт
    • rxmsgs — всего получено сообщений
    • rxbytes — всего получено байт
    • errs — количество ошибок

  13.3. ВКЛЮЧЕНИЕ ЛОГИРОВАНИЯ
    configMap.SetKey("debug", "all") // "protocol", "metadata", "broker", "topic"

  14.  КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ
  1.  confluent-kafka-go — официальный клиент, основан на C++-библиотеке
      librdkafka. Самый быстрый и полный.
  2.  Главный минус: CGO — сложнее сборка, нет context, возможны утечки
      памяти при неправильной обработке событий.
  3.  Выбирай для high-load и enterprise-проектов, где нужны транзакции
      и максимальная производительность.
  4.  Для простых проектов — sarama (чистый Go) или kafka-go (ещё проще).
  5.  Всегда обрабатывай Events() у продюсера, иначе память утекает.
  6.  Для транзакций: transactional.id + isolation.level=read_committed.
  7.  В Alpine Linux нужны пакеты: g++ и librdkafka-dev.
  8.  Вопрос на собеседовании: "какую библиотеку используешь и почему?"
      Ответ: "confluent-kafka-go, потому что нам нужна максимальная
      производительность и транзакции. Да, сборка сложнее из-за CGO,
      но это окупается надёжностью и скоростью."
*/

//КОНФИГУРАЦИЯ

const (
	defaultBroker   = "localhost:9092"
	defaultTopic    = "test-topic"
	consumerGroupID = "test-group"
)

var (
	mode            = flag.String("mode", "producer", "Режим: producer, consumer, transactional")
	broker          = flag.String("broker", defaultBroker, "Адрес брокера Kafka")
	topic           = flag.String("topic", defaultTopic, "Топик для отправки/чтения")
	transactionalID = flag.String("txn-id", "my-txn-id", "ID транзакционного продюсера")
)

//ОБЩИЕ УТИЛИТЫ

// createProducer создаёт продюсера с заданной конфигурацией.
func createProducer(configMap *kafka.ConfigMap) (*kafka.Producer, error) {
	return kafka.NewProducer(configMap)
}

// createConsumer создаёт консюмера с заданной конфигурацией.
func createConsumer(configMap *kafka.ConfigMap) (*kafka.Consumer, error) {
	return kafka.NewConsumer(configMap)
}

// waitForShutdown ожидает сигнала SIGINT или SIGTERM.
func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	<-sigCh
	log.Println("Получен сигнал завершения...")
}

// ПРОДЮСЕР
func runProducer() {
	log.Println("Запуск продюсера (асинхронный)")

	config := kafka.ConfigMap{
		"bootstrap.servers":                     *broker,
		"acks":                                  "all",
		"enable.idempotence":                    true,
		"retries":                               2147483647,
		"retry.backoff.ms":                      100,
		"batch.size":                            16384,
		"linger.ms":                             5,
		"compression.type":                      "snappy",
		"max.in.flight.requests.per.connection": 5,
		"queue.buffering.max.messages":          100000,
		"queue.buffering.max.ms":                1000,
		"go.events.channel.enable":              true,
		"go.delivery.reports":                   true,
		"debug":                                 "broker,topic,msg", // для отладки (опционально)
	}

	producer, err := createProducer(&config)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	// Канал для обработки событий доставки
	deliveryChan := producer.Events()
	// Канал для graceful shutdown
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	// Горутина для обработки событий доставки
	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-deliveryChan:
				switch ev := e.(type) {
				case *kafka.Message:
					if ev.TopicPartition.Error != nil {
						log.Printf("Ошибка доставки: %v", ev.TopicPartition.Error)
					} else {
						log.Printf("Доставлено: topic=%s, partition=%d, offset=%d",
							*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
					}
				case kafka.Error:
					log.Printf("Ошибка Kafka: %v", ev)
				}
			case <-done:
				log.Println("Остановка обработчика событий")
				return
			}
		}
	}()

	// Отправляем несколько сообщений
	messages := []string{
		"Hello from confluent-kafka-go",
		"Message 2",
		"Message 3 with key",
	}

	for i, msgText := range messages {
		key := fmt.Sprintf("key-%d", i)
		msg := &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: topic, Partition: kafka.PartitionAny},
			Key:            []byte(key),
			Value:          []byte(msgText),
			Headers: []kafka.Header{
				{Key: "source", Value: []byte("go-producer")},
				{Key: "sequence", Value: []byte(fmt.Sprintf("%d", i))},
			},
		}
		// Асинхронная отправка (неблокирующая)
		if err := producer.Produce(msg, nil); err != nil {
			log.Printf("Ошибка отправки: %v", err)
		} else {
			log.Printf("Отправлено: key=%s, value=%s", key, msgText)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Ждём доставки всех сообщений (с таймаутом)
	log.Println("Ожидание доставки...")
	producer.Flush(15 * 1000)

	// Останавливаем обработчик событий
	close(done)
	wg.Wait()

	log.Println("Продюсер завершён")
}

// КОНСЮМЕР (CONSUMER GROUP)
func runConsumer() {
	log.Println("Запуск консюмера (consumer group)")

	config := &kafka.ConfigMap{
		"bootstrap.servers":         *broker,
		"group.id":                  consumerGroupID,
		"auto.offset.reset":         "earliest",
		"enable.auto.commit":        false, // ручной коммит
		"fetch.min.bytes":           1024,
		"fetch.max.bytes":           50 * 1024 * 1024,
		"max.partition.fetch.bytes": 10 * 1024 * 1024,
		"max.poll.interval.ms":      300000,
		"session.timeout.ms":        45000,
		"heartbeat.interval.ms":     3000,
		"go.events.channel.enable":  true,
		"debug":                     "consumer,cgrp",
	}

	consumer, err := createConsumer(config)
	if err != nil {
		log.Fatalf("Ошибка создания консюмера: %v", err)
	}
	defer consumer.Close()

	// Подписываемся на топик
	err = consumer.SubscribeTopics([]string{*topic}, nil)
	if err != nil {
		log.Fatalf("Ошибка подписки: %v", err)
	}
	log.Printf("Подписка на топик %s", *topic)

	// Канал для завершения
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	// Горутина для обработки событий (сообщения + ребалансировка)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				// Используем ReadMessage с таймаутом 1 секунда для возможности выхода по сигналу
				msg, err := consumer.ReadMessage(1000 * time.Millisecond)
				if err != nil {
					if err.(kafka.Error).Code() == kafka.ErrTimedOut {
						continue
					}
					log.Printf("Ошибка чтения: %v", err)
					continue
				}
				// Обработка сообщения
				log.Printf("Получено: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
					*msg.TopicPartition.Topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset,
					string(msg.Key), string(msg.Value))

				// Имитация обработки
				time.Sleep(100 * time.Millisecond)

				// Ручной коммит смещения (после успешной обработки)
				if _, err := consumer.CommitMessage(msg); err != nil {
					log.Printf("Ошибка коммита: %v", err)
				} else {
					log.Printf("Закоммичено offset %d", msg.TopicPartition.Offset)
				}
			}
		}
	}()
	// Ожидаем сигнала завершения
	waitForShutdown()
	close(done)
	wg.Wait()

	log.Println("Консюмер завершён")
}

// ТРАНЗАКЦИОННЫЙ ПРОДЮСЕР
func runTransactional() {
	log.Println("Запуск транзакционного продюсера")

	config := &kafka.ConfigMap{
		"bootstrap.servers":                     *broker,
		"enable.idempotence":                    true,
		"transactional.id":                      *transactionalID,
		"acks":                                  "all",
		"retries":                               2147483647,
		"retry.backoff.ms":                      100,
		"max.in.flight.requests.per.connection": 5,
		"go.events.channel.enable":              true,
	}

	producer, err := createProducer(config)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	// Инициализация транзакций
	if err := producer.InitTransactions(nil); err != nil {
		log.Fatalf("Ошибка инициализации транзакций: %v", err)
	}
	log.Println("Транзакции инициализированы")

	// Канал для событий доставки (для транзакций тоже нужно обрабатывать)
	events := producer.Events()
	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-events:
				switch ev := e.(type) {
				case *kafka.Message:
					if ev.TopicPartition.Error != nil {
						log.Printf("Ошибка доставки в транзакции: %v", ev.TopicPartition.Error)
					}
					log.Printf("✅ Доставлено в транзакции: offset=%d", ev.TopicPartition.Offset)
				case kafka.Error:
					log.Printf("Ошибка Kafka: %v", ev)
				}
			case <-done:
				return
			}
		}
	}()

	// Начинаем транзакцию
	if err := producer.BeginTransaction(); err != nil {
		log.Fatalf("Ошибка начала транзакции: %v", err)
	}
	log.Println("Транзакция начата")

	// Отправляем несколько сообщений в транзакции
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("txn-key-%d", i)
		value := fmt.Sprintf("txn-msg-%d", i)
		msg := &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: topic, Partition: kafka.PartitionAny},
			Key:            []byte(key),
			Value:          []byte(value),
		}
		if err := producer.Produce(msg, nil); err != nil {
			log.Printf("Ошибка отправки в транзакции: %v", err)
			// Откатываем транзакцию при ошибке
			producer.AbortTransaction(nil)
			log.Println("Транзакция откатана")
			return
		}
		log.Printf("Отправлено (txn): key=%s, value=%s", key, value)
	}

	// Коммитим транзакцию
	if err := producer.CommitTransaction(nil); err != nil {
		log.Printf("❌ Ошибка коммита транзакции: %v", err)
		producer.AbortTransaction(nil)
		log.Println("Транзакция откатана")
	}
	log.Println("✅ Транзакция закоммичена")

	// Даём время на доставку
	producer.Flush(10 * 1000)

	// Останавливаем обработчик событий
	close(done)
	wg.Wait()

	log.Println("✅ Транзакционный продюсер завершён")
}

func main() {
	flag.Parse()

	switch *mode {
	case "producer":
		runProducer()
	case "consumer":
		runConsumer()
	case "transactional":
		runTransactional()
	default:
		log.Fatalf("Неизвестный режим: %s. Используйте: producer, consumer, transactional", *mode)
	}
}
