package nats

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

/*
  БЛОК 5.2: NATS
  NATS (Neural Autonomic Transport System) — это лёгкий, высокопроизводительный
  и облачно-ориентированный брокер сообщений с открытым исходным кодом.
  Он создан для микросервисов, IoT и low-latency систем, где скорость и
  простота важнее сложной маршрутизации и долгосрочного хранения.

  В этой расширенной теории мы разберём:
    1. Архитектура NATS: Core NATS vs JetStream
    2. Кластеризация и федерация NATS
    3. Протокол NATS — как это работает под капотом
    4. JetStream в деталях: Stream, Consumer, RAFT-консенсус
    5. Детальный разбор Ack'ов и гарантий доставки
    6. Безопасность: аутентификация, авторизация, TLS, JWT
    7. Масштабирование и производительность
    8. NATS vs Kafka vs RabbitMQ — глубокое сравнение
    9. Реальные сценарии использования в продакшене
    10. Ключевые выводы для собеседования

  1.  АРХИТЕКТУРА NATS — КАК ЭТО РАБОТАЕТ
  NATS состоит из двух основных режимов работы: Core NATS и JetStream.
  Они могут работать как вместе, так и по отдельности.

  1.1. CORE NATS — БЫСТРЫЙ И ЛЁГКИЙ
    Core NATS — это оригинальная реализация NATS без постоянного хранения.
    Он работает по принципу "запомнил и отправил" (in-memory).

    КЛЮЧЕВЫЕ ХАРАКТЕРИСТИКИ:
      • In-memory сообщения (не хранятся на диске).
      • At-most-once доставка (fire-and-forget).
      • Поддержка Queue Groups (конкурирующие потребители).
      • Поддержка Request-Reply (RPC).
      • Максимальная скорость (< 1 мс задержки).

  1.2. JETSTREAM — С ХРАНЕНИЕМ И ГАРАНТИЯМИ
    JetStream — это встроенная система хранения и потоковой обработки.

    КЛЮЧЕВЫЕ ХАРАКТЕРИСТИКИ:
      • Сохранение сообщений на диске (FileStorage) или в памяти (MemoryStorage).
      • At-least-once и Exactly-once (с транзакциями) гарантии.
      • Поддержка Replay (переигрывание) сообщений.
      • Поддержка Consumer Groups с Ack-подтверждениями.
      • Поддержка Stream (логическое хранилище) и Consumer (способ чтения).

  2.  КЛАСТЕРИЗАЦИЯ И ФЕДЕРАЦИЯ NATS

  2.1. КЛАСТЕР NATS
    • Несколько серверов NATS объединяются в кластер.
    • Сообщения маршрутизируются между серверами.
    • Если один сервер падает, другие продолжают работу.
    • Использует протокол NATS для синхронизации состояния.

  2.2. ФЕДЕРАЦИЯ NATS (GATEWAY)
    • Соединение двух независимых кластеров NATS.
    • Позволяет обмениваться сообщениями между ними.
    • Используется для multi-cloud или multi-region архитектур.

  2.3. LEAF NODE (ЛИСТОВОЙ УЗЕЛ)
    • Лёгкий узел, который подключается к кластеру.
    • Не хранит состояние.
    • Используется для IoT-устройств или периферийных систем.

  2.4. СУПЕРКЛАСТЕР
    • Флагманская функция NATS — создание единого кластера из разных
      географических регионов.
    • Автоматическая маршрутизация сообщений между регионами.

  3.  ПРОТОКОЛ NATS — КАК ЭТО РАБОТАЕТ ПОД КАПОТОМ

  NATS использует простой текстовый протокол поверх TCP.

  3.1. КОМАНДЫ ПРОТОКОЛА
    • CONNECT { "verbose": false, "user": "u", "pass": "p" } — подключение.
    • PUB subject 10\r\nHello\r\n — публикация сообщения.
    • SUB subject 1\r\n — подписка на subject.
    • UNSUB 1\r\n — отписка.
    • MSG subject 10\r\nHello\r\n — доставка сообщения (клиенту).

  3.2. ВЕРСИИ ПРОТОКОЛА
    • Протокол v1.0 — базовый (Core NATS).
    • Протокол v2.0 — с поддержкой JetStream (добавлены новые команды).

  3.3. ПРИМЕР ПРОТОКОЛА
    // Клиент подключается
    CONNECT {"verbose":false,"user":"u","pass":"p"}

    // Клиент публикует сообщение
    PUB user.created 5\r\nHello

    // Сервер подтверждает
    +OK

    // Клиент подписывается
    SUB user.* 1

    // Сервер отправляет сообщение
    MSG user.created 1 5\r\nHello

  4.  JETSTREAM В ДЕТАЛЯХ

  4.1. STREAM (ПОТОК)
    Stream — это хранилище сообщений в JetStream.

    КОНФИГУРАЦИЯ STREAM:
      • Name — уникальное имя потока.
      • Subjects — список subject, которые попадают в поток.
      • Retention — политика хранения:
        - Limits — по лимитам (макс. сообщений, размер, время).
        - Interest — хранить только если есть подписчики.
        - WorkQueue — обрабатывать каждое сообщение один раз.
      • Storage — тип хранения:
        - FileStorage — на диске (persistent).
        - MemoryStorage — в памяти (быстрее, но теряется при перезапуске).
      • Replicas — количество реплик (для отказоустойчивости).
      • MaxAge — максимальное время жизни сообщения (TTL).

  4.2. CONSUMER (ПОТРЕБИТЕЛЬ)
    Consumer определяет, как читать сообщения из Stream.

    КОНФИГУРАЦИЯ CONSUMER:
      • Durable — постоянный потребитель (сохраняет смещение).
      • AckPolicy — политика подтверждений:
        - AckExplicit — явное подтверждение (ручное).
        - AckNone — без подтверждений.
        - AckAll — подтверждение всех сообщений сразу.
      • DeliverPolicy — политика доставки:
        - All — все сообщения.
        - Last — последнее сообщение.
        - ByStartTime — с определённого времени.
        - ByStartSequence — с определённой последовательности.
      • ReplayPolicy — политика переигрывания:
        - Instant — мгновенная доставка.
        - Original — с задержкой, как в оригинале.

  4.3. RAFT-КОНСЕНСУС В JETSTREAM
    • JetStream использует RAFT для репликации данных.
    • Каждый Stream имеет несколько реплик (Replicas).
    • Лидер обрабатывает запросы, фолловеры реплицируют данные.
    • При сбое лидера выбирается новый.

  4.4. ХРАНЕНИЕ НА ДИСКЕ
    • Сообщения хранятся в файлах на диске.
    • Используется эффективное хранилище (LSM-tree).
    • Поддерживает сжатие данных.

  5.  ДЕТАЛЬНЫЙ РАЗБОР ACK'ОВ И ГАРАНТИЙ ДОСТАВКИ

  5.1. ТИПЫ ACK
    • ACK (Acknowledgment) — сообщение успешно обработано.
    • NAK (Negative Acknowledgment) — ошибка, нужно повторить.
    • TERM (Terminal Acknowledgment) — фатальная ошибка, не повторять.
    • PROGRESS — обработка продолжается (для долгих операций).

  5.2. ACK-ПОЛИТИКИ
    • AckExplicit — консюмер явно подтверждает каждое сообщение.
      Гарантирует at-least-once.

    • AckNone — подтверждения не отправляются.
      Сообщение считается доставленным сразу. At-most-once.

    • AckAll — подтверждение всех сообщений сразу.
      Используется для батчевой обработки.

  5.3. ГАРАНТИИ ДОСТАВКИ
    • AT-MOST-ONCE (Core NATS):
      - Нет подтверждений.
      - Сообщение может быть потеряно.
      - Максимальная скорость.

    • AT-LEAST-ONCE (JetStream + AckExplicit):
      - Консюмер подтверждает обработку.
      - Если Ack не получен, сообщение переотправляется.
      - Возможны дубли.

    • EXACTLY-ONCE (JetStream + Ack + идемпотентность):
      - Требует идемпотентной обработки на стороне консюмера.
      - Использует уникальные ID сообщений (msg ID).

  6.  БЕЗОПАСНОСТЬ: АУТЕНТИФИКАЦИЯ, АВТОРИЗАЦИЯ, TLS, JWT

  6.1. АУТЕНТИФИКАЦИЯ
    • Username/Password — базовая аутентификация.
    • Token — простой токен.
    • TLS Certificate — аутентификация по сертификату.
    • JWT (NGS) — JSON Web Token (декодируемые токены).

  6.2. АВТОРИЗАЦИЯ (PERMISSIONS)
    • Ограничение на публикацию (publish).
    • Ограничение на подписку (subscribe).
    • Ограничение на доступ к subject.

  6.3. TLS (ШИФРОВАНИЕ)
    • Включается сертификат для шифрования.
    • Поддерживает mutual TLS (взаимная аутентификация).

  7.  МАСШТАБИРОВАНИЕ И ПРОИЗВОДИТЕЛЬНОСТЬ

  7.1. ПРОИЗВОДИТЕЛЬНОСТЬ
    • Core NATS: > 10 млн сообщений/сек (в одном узле).
    • Задержка: < 1 мс (в среднем 0.5 мс).
    • JetStream: > 100 тыс. сообщений/сек.

  7.2. МАСШТАБИРОВАНИЕ
    • Горизонтальное масштабирование за счёт кластеризации.
    • Добавление узлов увеличивает пропускную способность.
    • Поддерживает до 1000 узлов в кластере.

  7.3. ОПТИМИЗАЦИЯ
    • Batch-отправка (публикация нескольких сообщений вместе).
    • Сжатие (gzip, snappy).
    • Использование JetStream для постоянных данных.

  8.  NATS VS KAFKA VS RABBITMQ — ГЛУБОКОЕ СРАВНЕНИЕ (РАСШИРЕННОЕ)
  ┌─────────────────────┬──────────────────────┬────────────────────────────┬────────────────────────────┐
  │ Характеристика      │ NATS (Core)          │ NATS (JetStream)           │ Kafka                      │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Модель              │ Pub/Sub              │ Pub/Sub + Queue            │ Лог                        │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Хранение            │ Нет                  │ Да (диск/память)           │ Да (диск)                  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Порядок             │ Не гарантируется     │ Не гарантируется           │ Гарантируется в партиции   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Replay              │ Нет                  │ Да                         │ Да                         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Задержка            │ < 1 мс               │ < 5 мс                     │ 5-20 мс                    │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Гарантии            │ At-most-once         │ At-least-once              │ Exactly-once (транзакции)  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Маршрутизация       │ Простая (subject)    │ Простая (subject)          │ Сложная (по ключу)         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┼────────────────────────────┤
  │ Когда использовать  │ Микросервисы, RPC,   │ Event-driven, задачи,      │ Стриминг, аналитика,       │
  │                     │ IoT, low-latency     │ системы с гарантиями       │ event sourcing             │
  └─────────────────────┴──────────────────────┴────────────────────────────┴────────────────────────────┘

  ┌─────────────────────┬──────────────────────┬────────────────────────────┐
  │ Характеристика      │ NATS (JetStream)     │ RabbitMQ                   │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Модель              │ Pub/Sub + Queue      │ Queue                      │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Маршрутизация       │ Простая (subject)    │ Сложная (4 типа exchange)  │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Порядок             │ Не гарантируется     │ Не гарантируется           │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Задержка            │ < 5 мс               │ < 5 мс                     │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Репликация          │ Да                   │ Да (Quorum Queues)         │
  ├─────────────────────┼──────────────────────┼────────────────────────────┤
  │ Когда использовать  │ Простые события,     │ Сложная маршрутизация,     │
  │                     │ задачи, low-latency  │ задачи, RPC                │
  └─────────────────────┴──────────────────────┴────────────────────────────┘

  9.  РЕАЛЬНЫЕ СЦЕНАРИИ ИСПОЛЬЗОВАНИЯ В ПРОДАКШЕНЕ

  9.1. МИКРОСЕРВИСНАЯ АРХИТЕКТУРА
    • Сервис A публикует событие "order.created".
    • Сервис B подписан на "order.*" и обрабатывает заказы.
    • NATS обеспечивает быструю доставку (< 1 мс).

  9.2. RPC (REQUEST-RESPONSE)
    • Клиент отправляет запрос в subject "user.get".
    • Сервер обрабатывает и отвечает.
    • NATS автоматически направляет ответ клиенту.

  9.3. IOT (ИНТЕРНЕТ ВЕЩЕЙ)
    • Устройства публикуют данные в subject "sensor.*".
    • Агрегатор подписывается на "sensor.>" и собирает данные.
    • NATS лёгкий и работает на устройствах с ограниченными ресурсами.

  9.4. EVENT-DRIVEN АРХИТЕКТУРА С ГАРАНТИЯМИ
    • JetStream хранит все события.
    • Consumer гарантирует доставку at-least-once.
    • При сбое консюмера сообщения переотправляются.

  10. ЧАСТЫЕ ОШИБКИ В ПРОДАКШЕНЕ
  1. Не использовать JetStream для критичных данных.
  2. Не настраивать Ack для важных сообщений (потеря данных).
  3. Слишком глубокие subject (снижают производительность).
  4. Не использовать Queue Group для конкурирующих потребителей.
  5. Не мониторить размер Stream (переполнение диска).
  6. Использовать один Conn для всех операций (без пула).
  7. Не настраивать авторизацию (доступ всем).
  8. Не использовать TLS для шифрования (данные в открытом виде).

  11. КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  NATS — лёгкий, высокопроизводительный брокер для микросервисов и IoT.
  2.  Core NATS — at-most-once, без хранения, задержка < 1 мс.
  3.  JetStream — at-least-once, с хранением и replay.
  4.  Subject — тема с wildcard (*, >).
  5.  Queue Group — конкурирующие потребители (аналог Kafka Consumer Group).
  6.  Stream — хранилище в JetStream (аналог топика в Kafka).
  7.  Consumer — способ чтения из Stream (Push/Pull).
  8.  Ack — подтверждение обработки (ACK, NAK, TERM).
  9.  nats.go — официальный Go-клиент с поддержкой Core и JetStream.
  10. NATS — для простоты и скорости. Kafka — для сложных стримингов
      и долгосрочного хранения. RabbitMQ — для сложной маршрутизации.
  11. Кластеризация, федерация и суперкластер — для масштабирования.
  12. Протокол NATS прост и эффективен (текстовый, поверх TCP).
  13. JetStream использует RAFT-консенсус для надёжности.
  14. Всегда настраивайте аутентификацию и авторизацию в продакшене.
  15. На собеседовании: «Мы используем NATS для микросервисов и RPC,
      Kafka для стриминга данных, RabbitMQ для сложных задач.»
*/

//КОНФИГУРАЦИЯ

var (
	mode     = flag.String("mode", "core-pub", "Режим: core-pub, core-sub, core-queue, req-reply, req-client, js-pub, js-push, js-pull")
	url      = flag.String("url", nats.DefaultURL, "URL NATS сервера (nats://localhost:4222)")
	subject  = flag.String("subject", "test", "Subject для публикации/подписки")
	queue    = flag.String("queue", "workers", "Имя очереди (для queue group)")
	stream   = flag.String("stream", "STREAMS", "Имя JetStream Stream")
	durable  = flag.String("durable", "consumer", "Имя durable consumer")
	msgCount = flag.Int("count", 10, "Количество сообщений")
)

//CORE NATS: PUBLISHER

func corePublisher(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	log.Printf("Core Publisher запущен (subject=%s)", *subject)

	for i := 0; i < *msgCount; i++ {
		select {
		case <-ctx.Done():
			log.Println("Publisher остановлен")
			return nil
		default:
		}

		msg := fmt.Sprintf("Сообщение #%d от %s", i+1, time.Now().Format("15:04:05"))
		if err := nc.Publish(*subject, []byte(msg)); err != nil {
			log.Printf("Ошибка публикации: %v", err)
			continue
		}
		log.Printf("✅ Опубликовано: %s", msg)
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// CORE NATS: SUBSCRIBER
func coreSubscriber(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	log.Printf("Core Subscriber запущен (subject=%s)", *subject)

	sub, err := nc.Subscribe(*subject, func(msg *nats.Msg) {
		log.Printf("Получено: subject=%s, data=%s", msg.Subject, string(msg.Data))
	})
	if err != nil {
		return fmt.Errorf("ошибка подписки: %w", err)
	}
	defer sub.Unsubscribe()

	<-ctx.Done()
	log.Println("Subscriber остановлен")
	return nil
}

//CORE NATS: QUEUE GROUP

func coreQueue(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	queueName := *queue
	log.Printf("Queue Group запущен (queue=%s, subject=%s)", queueName, *subject)

	sub, err := nc.QueueSubscribe(*subject, queueName, func(msg *nats.Msg) {
		log.Printf("[%s] Получено: %s", queueName, string(msg.Data))
		// Имитация обработки
		time.Sleep(100 * time.Millisecond)
	})
	if err != nil {
		return fmt.Errorf("ошибка подписки: %w", err)
	}
	defer sub.Unsubscribe()

	<-ctx.Done()
	log.Printf("[%s] Queue Group остановлен", queueName)
	return nil
}

//REQUEST-REPLY (СЕРВЕР)

func requestReplyServer(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	log.Printf("Request-Reply сервер запущен (subject=%s)", *subject)

	sub, err := nc.Subscribe(*subject, func(msg *nats.Msg) {
		log.Printf("Запрос: %s", string(msg.Data))

		// Обработка запроса
		response := fmt.Sprintf("Ответ на: %s (обработано в %s)", string(msg.Data), time.Now().Format("15:04:05"))

		// Отправка ответа
		if err := nc.Publish(msg.Reply, []byte(response)); err != nil {
			log.Printf("Ошибка отправки ответа: %v", err)
		} else {
			log.Printf("Ответ отправлен: %s", response)
		}
	})
	if err != nil {
		return fmt.Errorf("ошибка подписки: %w", err)
	}
	defer sub.Unsubscribe()

	<-ctx.Done()
	log.Println("Request-Reply сервер остановлен")
	return nil
}

//REQUEST-REPLY (КЛИЕНТ)

func requestReplyClient(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	log.Printf("Request-Reply клиент запущен (subject=%s)", *subject)

	for i := 0; i < *msgCount; i++ {
		select {
		case <-ctx.Done():
			log.Println("Клиент остановлен")
			return nil
		default:
		}

		req := fmt.Sprintf("Запрос #%d", i+1)

		// Отправка запроса с таймаутом 5 секунд
		msg, err := nc.Request(*subject, []byte(req), 5*time.Second)
		if err != nil {
			log.Printf("Ошибка запроса: %v", err)
			continue
		}
		log.Printf("Ответ получен: %s", string(msg.Data))
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

//JETSTREAM: PUBLISHER

func jetStreamPublisher(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("ошибка инициализации JetStream: %w", err)
	}

	// Создаём Stream (если не существует)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      *stream,
		Subjects:  []string{*subject + ".*"},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Replicas:  1,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return fmt.Errorf("ошибка создания Stream: %w", err)
	}

	log.Printf("JetStream Publisher запущен (stream=%s, subject=%s.*)", *stream, *subject)

	for i := 0; i < *msgCount; i++ {
		select {
		case <-ctx.Done():
			log.Println("Publisher остановлен")
			return nil
		default:
		}

		subjectFull := fmt.Sprintf("%s.%d", *subject, i+1)
		msg := fmt.Sprintf("Сообщение #%d от %s", i+1, time.Now().Format("15:04:05"))

		// Публикация с подтверждением (Ack)
		ack, err := js.Publish(subjectFull, []byte(msg))
		if err != nil {
			log.Printf("Ошибка публикации: %v", err)
			continue
		}
		log.Printf("Опубликовано: subject=%s, seq=%d", subjectFull, ack.Sequence)
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

//JETSTREAM: PUSH CONSUMER

func jetStreamPushConsumer(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("ошибка инициализации JetStream: %w", err)
	}

	// Проверяем, существует ли Stream (создаём, если нет)
	_, err = js.StreamInfo(*stream)
	if err != nil {
		return fmt.Errorf("Stream %s не существует, создайте через js-pub", *stream)
	}

	// Создаём Durable Consumer
	_, err = js.AddConsumer(*stream, &nats.ConsumerConfig{
		Durable:       *durable,
		AckPolicy:     nats.AckAllPolicy,
		DeliverPolicy: nats.DeliverAllPolicy,
		ReplayPolicy:  nats.ReplayInstantPolicy,
	})
	if err != nil && err != nats.ErrConsumerNameAlreadyInUse {
		return fmt.Errorf("ошибка создания consumer: %w", err)
	}

	log.Printf("JetStream Push Consumer запущен (stream=%s, durable=%s)", *stream, *durable)

	// Push Consumer — подписка на Stream
	sub, err := js.Subscribe(*subject+".*", func(msg *nats.Msg) {
		log.Printf("Получено: subject=%s, data=%s, seq=%d", msg.Subject, string(msg.Data), msg.Subject)

		// Имитация обработки
		time.Sleep(100 * time.Millisecond)

		// Подтверждение (Ack)
		if err := msg.Ack(); err != nil {
			log.Printf("Ошибка ACK: %v", err)
		} else {
			log.Printf("ACK отправлен для seq=%d", msg.Subject)
		}
	}, nats.Durable(*durable))
	if err != nil {
		return fmt.Errorf("ошибка подписки: %w", err)
	}
	defer sub.Unsubscribe()

	<-ctx.Done()
	log.Println("⏳ Push Consumer остановлен")
	return nil
}

//JETSTREAM: PULL CONSUMER

func jetStreamPullConsumer(ctx context.Context) error {
	nc, err := nats.Connect(*url)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("ошибка инициализации JetStream: %w", err)
	}

	_, err = js.StreamInfo(*stream)
	if err != nil {
		return fmt.Errorf("Stream %s не существует", *stream)
	}

	// Pull Consumer — создаём ephemeral (без durable)
	sub, err := js.Subscribe(*subject+".*", func(msg *nats.Msg) {
		log.Printf("Pull: получено %s", string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("ошибка создания pull consumer: %w", err)
	}
	defer sub.Unsubscribe()

	log.Printf("JetStream Pull Consumer запущен (stream=%s)", *stream)

	// Пул сообщений
	for {
		select {
		case <-ctx.Done():
			log.Println("Pull Consumer остановлен")
			return nil
		default:
			msgs, err := sub.Fetch(5)
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				log.Printf("Ошибка fetch: %v", err)
				continue
			}
			for _, msg := range msgs {
				log.Printf("Pull: subject=%s, data=%s, seq=%d", msg.Subject, string(msg.Data), msg.Subject)
				msg.Ack()
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
	case "core-pub":
		err = corePublisher(ctx)
	case "core-sub":
		err = coreSubscriber(ctx)
	case "core-queue":
		err = coreQueue(ctx)
	case "req-reply":
		err = requestReplyServer(ctx)
	case "req-client":
		err = requestReplyClient(ctx)
	case "js-pub":
		err = jetStreamPublisher(ctx)
	case "js-push":
		err = jetStreamPushConsumer(ctx)
	case "js-pull":
		err = jetStreamPullConsumer(ctx)
	default:
		err = fmt.Errorf("неизвестный режим: %s", *mode)
	}

	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
