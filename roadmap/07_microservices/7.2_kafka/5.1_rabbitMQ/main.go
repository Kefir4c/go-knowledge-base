package rabbitmq

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

/*
  БЛОК 5.1: RABBITMQ
  RabbitMQ — это самый популярный open-source брокер сообщений, реализующий
  протокол AMQP 0-9-1. В отличие от Kafka, которая построена как распределённый
  лог, RabbitMQ — это классическая очередь сообщений с богатой маршрутизацией.

  В этой расширенной теории мы разберём:
    1. Что такое AMQP 0-9-1 и его модель
    2. Ключевые концепции: Exchange, Queue, Binding, Routing Key
    3. Четыре типа Exchange в деталях (Direct, Fanout, Topic, Headers)
    4. Виртуальные хосты (Virtual Hosts) и их роль
    5. Подтверждения (Acknowledgements) и Publisher Confirms
    6. Транзакции в RabbitMQ vs Publisher Confirms
    7. Протоколы: AMQP 1.0, MQTT, STOMP
    8. Библиотека amqp091-go для Go — архитектура и паттерны
    9. Настройка для продакшена: memory, disk, high availability
    10. Мониторинг и управление
    11. RabbitMQ vs Kafka — глубокое сравнение
    12. Ключевые выводы для собеседования

  1.  AMQP 0-9-1 — МОДЕЛЬ И ПРОТОКОЛ
  AMQP (Advanced Message Queuing Protocol) 0-9-1 — это бинарный сетевой
  протокол, который определяет модель обмена сообщениями.

  КЛЮЧЕВЫЕ ХАРАКТЕРИСТИКИ:
    • Бинарный протокол — эффективная сериализация.
    • Программируемый — клиенты сами создают очереди и обменники.
    • Поддержка подтверждений и транзакций.
    • Разделение на четыре модели: Exchange, Queue, Binding, Routing Key.

  Модель AMQP 0-9-1 отличается от многих других систем:
    • Продюсер публикует сообщение в Exchange.
    • Exchange маршрутизирует сообщение в одну или несколько Queue.
    • Консюмер читает из Queue и подтверждает обработку.

  Это разделение позволяет создавать гибкие схемы маршрутизации без изменения
  продюсеров и консюмеров.

  2.  EXCHANGE, QUEUE, BINDING, ROUTING KEY — ГЛУБОКИЙ РАЗБОР

  2.1. EXCHANGE (ОБМЕННИК)
  Обменник — это точка входа для сообщений. Продюсеры публикуют сообщения
  в обменник, а не напрямую в очередь.

  ХАРАКТЕРИСТИКИ:
    • Обменник не хранит сообщения — он только маршрутизирует.
    • Обменники могут быть долговременными (durable) или временными.
    • Тип обменника определяет логику маршрутизации.
    • Существует встроенный default exchange (amq.direct).

  ТИПЫ:
    • Direct — точное совпадение routing key.
    • Fanout — рассылка всем.
    • Topic — совпадение по шаблону.
    • Headers — совпадение по заголовкам.

  2.2. QUEUE (ОЧЕРЕДЬ)
  Очередь — это буфер, в котором хранятся сообщения.

  ХАРАКТЕРИСТИКИ:
    • FIFO (первым пришёл — первым ушёл), но порядок может быть нарушен
      при повторных попытках или приоритетах.
    • Очереди могут быть durable (переживают перезапуск брокера).
    • Очереди могут быть exclusive (доступны только одному соединению).
    • Очереди могут быть auto-delete (удаляются, когда нет консюмеров).
    • Очереди могут иметь TTL (time-to-live) для сообщений.

  2.3. BINDING (ПРИВЯЗКА)
  Привязка — это правило, которое соединяет обменник и очередь.

  • Привязка выполняется на стороне очереди.
  • Каждая привязка содержит routing key.
  • Одна очередь может иметь множество привязок.
  • Один обменник может иметь множество привязок.

  2.4. ROUTING KEY (КЛЮЧ МАРШРУТИЗАЦИИ)
  Routing key — это строка, которую продюсер указывает при публикации.

  • Может быть любой строкой (максимум 255 байт).
  • Формат: слова разделённые точками (например, "user.created").
  • Используется только для маршрутизации (не для хранения).
  • Для Headers exchange не используется.

  3.  ЧЕТЫРЕ ТИПА EXCHANGE В ДЕТАЛЯХ

  3.1. DIRECT EXCHANGE (ПРЯМОЙ ОБМЕННИК)
    Сообщение направляется в очередь, если routing key сообщества ТОЧНО
    совпадает с routing key привязки.

    ПРИМЕР:
      • Продюсер публикует сообщение с routing key = "error".
      • Очередь A привязана с routing key = "error" → получает сообщение.
      • Очередь B привязана с routing key = "warning" → не получает.

    ИСПОЛЬЗОВАНИЕ:
      • Задачи (Task Queues) с распределением по типам.
      • RPC-вызовы (ответы в отдельную очередь).
      • Логирование по уровням (error, warning, info).
      • Когда нужна простая маршрутизация.

  3.2. FANOUT EXCHANGE (ВЕЕРООБРАЗНЫЙ ОБМЕННИК)
    Сообщение рассылается ВО ВСЕ очереди, привязанные к обменнику.
    Routing key ИГНОРИРУЕТСЯ.

    ПРИМЕР:
      • Продюсер публикует сообщение в fanout-обменник.
      • Очереди A, B, C привязаны к обменнику → все получают копию.

    ИСПОЛЬЗОВАНИЕ:
      • Широковещательные уведомления (broadcast).
      • События, которые должны получить все сервисы.
      • Обновление кэша во всех экземплярах.

  3.3. TOPIC EXCHANGE (ТЕМАТИЧЕСКИЙ ОБМЕННИК)
    Сообщение направляется в очереди, если routing key сообщества СООТВЕТСТВУЕТ
    шаблону routing key привязки.

    ПАТТЕРНЫ:
      • "*" (звёздочка) — заменяет ровно ОДНО слово.
      • "#" (решётка) — заменяет НОЛЬ или БОЛЬШЕ слов.

    ПРИМЕР:
      • Routing key = "user.created" → шаблон "user.*" подходит.
      • Routing key = "user.created.123" → шаблон "user.#" подходит.
      • Routing key = "order.paid" → шаблон "user.*" НЕ подходит.

    ИСПОЛЬЗОВАНИЕ:
      • Сложная маршрутизация событий по категориям.
      • Фильтрация логов по компонентам и уровням.
      • Системы с различными типами событий (user.*, order.*, payment.*).

  3.4. HEADERS EXCHANGE (ОБМЕННИК НА ОСНОВЕ ЗАГОЛОВКОВ)
    Маршрутизация происходит не по routing key, а по заголовкам сообщения.

    • Привязка указывает набор заголовков (key-value).
    • Сообщение попадает в очередь, если его заголовки совпадают.
    • Поддерживает логические операции: "any" и "all".

    ИСПОЛЬЗОВАНИЕ:
      • Сложная маршрутизация по множеству атрибутов.
      • Когда routing key не подходит (например, несколько измерений).

  4.  ВИРТУАЛЬНЫЕ ХОСТЫ (VIRTUAL HOSTS)
  Виртуальный хост (vhost) — это логическая изоляция внутри одного брокера.

  • Каждый vhost имеет свои очереди, обменники, привязки и права доступа.
  • Используется для разделения окружений (dev, test, prod) или клиентов.
  • При подключении клиент указывает vhost в URL: amqp://user:pass@host:5672/ vhost.
  • По умолчанию используется "/".

  5.  ПОДТВЕРЖДЕНИЯ (ACKNOWLEDGEMENTS) И PUBLISHER CONFIRMS

  5.1. CONSUMER ACKNOWLEDGEMENTS
    Подтверждения консюмеров гарантируют, что сообщение не будет потеряно
    при сбое консюмера.

    • AUTO-ACK: сообщение подтверждается сразу после доставки.
      Если консюмер упадёт, сообщение будет потеряно (at-most-once).

    • MANUAL ACK: консюмер явно вызывает ack() после обработки.
      Если консюмер упадёт, сообщение вернётся в очередь (at-least-once).

    • NACK: отрицательное подтверждение.
      Сообщение может быть возвращено в очередь (requeue) или отправлено в DLQ.

  5.2. PUBLISHER CONFIRMS (ПОДТВЕРЖДЕНИЯ ПРОДЮСЕРА)
    Publisher confirms — это механизм, позволяющий продюсеру знать,
    что сообщение получено и записано брокером.

    • Включается на канале: channel.Confirm()
    • Продюсер ждёт подтверждения: channel.Publish(...) + channel.WaitForConfirms()
    • Аналог acks=all в Kafka.
    • Гарантирует at-least-once доставку от продюсера к брокеру.

    Пример:
      ch.Confirm(false)
      ch.Publish(...)
      if err := ch.WaitForConfirms(); err != nil {
          // сообщение не доставлено
      }

  6.  ТРАНЗАКЦИИ В RABBITMQ VS PUBLISHER CONFIRMS
  RabbitMQ поддерживает транзакции, но они редко используются из-за
  производительности.

  6.1. ТРАНЗАКЦИИ (AMQP TX)
    • Включаются: channel.Tx()
    • Коммит: channel.TxCommit()
    • Откат: channel.TxRollback()
    • Медленные, так как блокируют операции на канале.
    • Рекомендуется использовать Publisher Confirms вместо транзакций.

  6.2. PUBLISHER CONFIRMS (РЕКОМЕНДУЕТСЯ)
    • Быстрее и легче.
    • Дают ту же гарантию at-least-once.
    • Поддерживаются в amqp091-go.

  7.  ПРОТОКОЛЫ: AMQP 1.0, MQTT, STOMP
  RabbitMQ поддерживает несколько протоколов:

    • AMQP 0-9-1 — основной протокол (родной для RabbitMQ).
    • AMQP 1.0 — более современный, но менее функциональный в RabbitMQ.
    • MQTT — лёгкий протокол для IoT (используется плагином).
    • STOMP — простой текстовый протокол для веб-приложений.
  Это делает RabbitMQ универсальным брокером для разных сценариев.

  8.  БИБЛИОТЕКА AMQP091-GO — АРХИТЕКТУРА И ПАТТЕРНЫ

  8.1. КОМПОНЕНТЫ
    • amqp.Connection — TCP-соединение с брокером.
    • amqp.Channel — логический канал внутри соединения.
    • amqp.Delivery — структура полученного сообщения.
    • amqp.Publishing — структура отправляемого сообщения.

  8.2. BEST PRACTICES
    • ОДНО соединение на всё приложение.
    • ОДИН канал на горутину или на задачу.
    • Не использовать один канал в нескольких горутинах без синхронизации.
    • Всегда закрывать каналы и соединения.

  8.3. ПАТТЕРН РЕКОННЕКТА
    • При обрыве соединения нужно пересоздавать всё.
    • Используйте готовые обёртки или пишите свою логику.

  9.  НАСТРОЙКА ДЛЯ ПРОДАКШЕНА

  9.1. ПАМЯТЬ И ДИСК
    • vm_memory_high_watermark — порог памяти (по умолчанию 40% от RAM).
    • disk_free_limit — минимальное свободное место на диске.
    • При превышении порогов брокер блокирует публикацию.

  9.2. HIGH AVAILABILITY (КЛАСТЕРИЗАЦИЯ)
    • RabbitMQ поддерживает кластеризацию для отказоустойчивости.
    • Очереди могут быть зеркальными (mirrored queues) — реплицируются
      на несколько узлов.
    • HA policy: ha-mode = all / exactly / nodes.
    • В версии 3.8+ используется Quorum Queues (Raft-based).

  9.3. ПЕРСИСТЕНТНОСТЬ
    • Для важных данных делайте очереди durable и сообщения persistent.
    • Это гарантирует, что сообщения не потеряются при перезапуске брокера.

  9.4. PREFETCH (QOS)
    • Настройка prefetch определяет, сколько сообщений может быть доставлено
      консюмеру без подтверждения.
    • Prefetch = 1 — гарантирует, что один консюмер не загрузит себя,
      но снижает пропускную способность.
    • Prefetch = N — повышает пропускную способность, но увеличивает
      риск потери при сбое консюмера.

  10. МОНИТОРИНГ И УПРАВЛЕНИЕ

  10.1. MANAGEMENT UI
    • Встроенный веб-интерфейс на порту 15672.
    • Показывает очереди, обменники, соединения, каналы.
    • Отображает ключевые метрики (queue depth, unacked, publish rate).

  10.2. PROMETHEUS + GRAFANA
    • Используйте rabbitmq-prometheus для экспорта метрик.
    • Ключевые метрики:
      - rabbitmq_queue_messages_ready — сообщения в очереди.
      - rabbitmq_queue_messages_unacked — неподтверждённые.
      - rabbitmq_connections — количество соединений.
      - rabbitmq_channels — количество каналов.

  10.3. АЛЕРТЫ
    • Длина очереди растёт → консюмер не успевает.
    • Количество неподтверждённых сообщений > threshold → возможная проблема.
    • Соединения падают → сетевые проблемы.

  11. RABBITMQ VS KAFKA — ГЛУБОКОЕ СРАВНЕНИЕ
  ┌─────────────────────┬──────────────────────┬────────────────────────────┐
  │ Характеристика      │ RabbitMQ             │ Kafka                      │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Модель              │ Очередь (Queue)      │ Лог (Log)                  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Хранение            │ Временное (до ack)   │ Долгосрочное (retention)   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Маршрутизация       │ Сложная (4 типа)     │ Простая (по ключу)         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Порядок             │ Не гарантируется     │ Гарантируется в партиции   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Задержка            │ Очень низкая         │ Средняя/высокая            │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Replay              │ Нет                  │ Да                         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Exactly-once        │ Нет (только at-     │ Да (транзакции)             │
  │                     │ least-once)          │                            │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Когда использовать  │ Задачи, RPC, сложная │ Стриминг, аналитика,       │
  │                     │ маршрутизация,       │ event sourcing             │
  │                     │ low-latency          │                            │
  └─────────────────────┴──────────────────────┴────────────────────────────┘

  12. ЧАСТЫЕ ОШИБКИ В ПРОДАКШЕНЕ
  1. Prefetch слишком большой → консюмер перегружен, ack не отправляются.
  2. Нет подтверждений (auto-ack) → потеря сообщений при падении консюмера.
  3. Не durable очереди → потеря данных при перезапуске.
  4. Не persistent сообщения → потеря данных при перезапуске брокера.
  5. Нет обработки ошибок → канал или соединение падает.
  6. Использование одного канала в нескольких горутинах → race condition.
  7. Нет мониторинга → не видно, когда очередь переполняется.

  13. КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  RabbitMQ — брокер сообщений на AMQP 0-9-1 с богатой маршрутизацией.
  2.  Четыре типа обменников:
      • Direct — точное совпадение routing key.
      • Fanout — рассылка всем (broadcast).
      • Topic — сопоставление по шаблону (*, #).
      • Headers — маршрутизация по заголовкам.
  3.  Binding — связь между обменником и очередью с routing key.
  4.  Подтверждения (ack) — гарантируют at-least-once доставку.
  5.  Publisher Confirms — гарантируют, что сообщение получено брокером.
  6.  Виртуальные хосты (vhost) — изоляция окружений.
  7.  amqp091-go — официальный Go-клиент, поддерживаемый RabbitMQ team.
  8.  Best practices: одно соединение, канал на горутину, ручные ack для
      критичных данных, автоматическое переподключение.
  9.  RabbitMQ — для задач, RPC, сложной маршрутизации и low-latency.
      Kafka — для стриминга, аналитики, event sourcing.
  10. На собеседовании: «RabbitMQ выбираем для сложной маршрутизации и задач,
      Kafka — для стриминга и долгосрочного хранения.»
*/

// КОНФИГУРАЦИЯ
var (
	mode       = flag.String("mode", "producer", "Режим: producer, consumer")
	exchange   = flag.String("exchange", "direct", "Тип exchange: direct, fanout, topic")
	routingKey = flag.String("routing-key", "info", "Routing key (для direct/topic)")
	queueName  = flag.String("queue", "my-queue", "Имя очереди (для direct)")
	rabbitURL  = flag.String("rabbit-url", "amqp://guest:guest@localhost:5672/", "URL подключения")
)

// PRODUCER
func runProducer(ctx context.Context) error {
	conn, err := amqp.Dial(*rabbitURL)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("ошибка создания канала: %w", err)
	}
	defer ch.Close()

	// Объявляем exchange
	var exchangeType string
	switch *exchange {
	case "direct":
		exchangeType = "direct"
	case "fanout":
		exchangeType = "fanout"
	case "topic":
		exchangeType = "topic"
	default:
		return fmt.Errorf("неизвестный exchange: %s", *exchange)
	}

	err = ch.ExchangeDeclare(
		*exchange,
		exchangeType,
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("ошибка объявления exchange: %w", err)
	}

	//Включаем Publisher Confirms (через канал подтверждений)
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("ошибка включения confirms: %w", err)
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	log.Println("Publisher Confirms включены")

	log.Printf("Producer запущен (exchange=%s, routing-key=%s)", *exchange, *routingKey)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	counter := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("Producer остановлен")
			return nil
		case <-ticker.C:
			counter++
			msg := fmt.Sprintf("Сообщение #%d от %s", counter, time.Now().Format("15:04:05"))

			// Публикуем сообщение
			err := ch.PublishWithContext(ctx,
				*exchange,
				*routingKey,
				false, // mandatory
				false, // immediate
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         []byte(msg),
					DeliveryMode: amqp.Persistent,
				},
			)
			if err != nil {
				log.Printf("Ошибка публикации: %v", err)
				continue
			}

			//Ждём подтверждение через канал
			select {
			case confirm := <-confirms:
				if confirm.Ack {
					log.Printf("Подтверждено: %s", msg)
				} else {
					log.Printf("Не подтверждено: %s", msg)
				}
			case <-time.After(5 * time.Second):
				log.Printf("Таймаут подтверждения: %s", msg)
			}
		}
	}
}

// CONSUMER
func runConsumer(ctx context.Context) error {
	conn, err := amqp.Dial(*rabbitURL)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("ошибка создания канала: %w", err)
	}
	defer ch.Close()

	// === 1. Объявляем exchange ===
	var exchangeType string
	switch *exchange {
	case "direct":
		exchangeType = "direct"
	case "fanout":
		exchangeType = "fanout"
	case "topic":
		exchangeType = "topic"
	default:
		return fmt.Errorf("неизвестный exchange: %s", *exchange)
	}

	err = ch.ExchangeDeclare(
		*exchange,    // name
		exchangeType, // type
		true,         // durable
		false,        // auto-delete
		false,        // internal
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("ошибка объявления exchange: %w", err)
	}

	// === 2. Объявляем очередь (уникальную для fanout) ===
	var queue string
	if *exchange == "fanout" {
		// Для fanout используем временную очередь (auto-delete)
		q, err := ch.QueueDeclare(
			"",    // name (пусто = уникальное имя)
			false, // durable
			false, // auto-delete
			true,  // exclusive
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			return fmt.Errorf("ошибка объявления очереди: %w", err)
		}
		queue = q.Name
		log.Printf("Создана временная очередь: %s (fanout)", queue)
	} else {
		// Для direct/topic используем указанную очередь
		q, err := ch.QueueDeclare(
			*queueName, // name
			true,       // durable
			false,      // auto-delete
			false,      // exclusive
			false,      // no-wait
			nil,        // args
		)
		if err != nil {
			return fmt.Errorf("ошибка объявления очереди: %w", err)
		}
		queue = q.Name
		log.Printf("Очередь: %s", queue)
	}

	// === 3. Привязываем очередь к exchange ===
	err = ch.QueueBind(
		queue,       // queue name
		*routingKey, // routing key
		*exchange,   // exchange
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("ошибка привязки очереди: %w", err)
	}
	log.Printf("Привязка: queue=%s, exchange=%s, routing-key=%s", queue, *exchange, *routingKey)

	// === 4. QoS (prefetch) ===
	err = ch.Qos(
		1,     // prefetch count (1 сообщение за раз)
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("ошибка настройки QoS: %w", err)
	}
	log.Println("Prefetch = 1 (manual ACK)")

	// === 5. Подписываемся ===
	msgs, err := ch.Consume(
		queue, // queue
		"",    // consumer
		false, // auto-ack (false = manual)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("ошибка подписки: %w", err)
	}

	log.Printf("Consumer запущен, ожидание сообщений...")

	for {
		select {
		case <-ctx.Done():
			log.Println("⏳ Consumer остановлен")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				log.Println("Канал сообщений закрыт")
				return nil
			}
			log.Printf("Получено: %s", msg.Body)

			// Имитация обработки
			time.Sleep(200 * time.Millisecond)

			// Ручное подтверждение
			if err := msg.Ack(false); err != nil {
				log.Printf("Ошибка ACK: %v", err)
			} else {
				log.Printf("ACK отправлен")
			}
		}
	}
}

//MAIN

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Получен сигнал завершения...")
		cancel()
	}()

	var err error
	switch *mode {
	case "producer":
		err = runProducer(ctx)
	case "consumer":
		err = runConsumer(ctx)
	default:
		err = fmt.Errorf("неизвестный режим: %s", *mode)
	}

	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
