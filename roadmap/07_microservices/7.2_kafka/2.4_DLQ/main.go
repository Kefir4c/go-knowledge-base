package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

/*
  БЛОК 2.4: ОБРАБОТКА ОШИБОК И DLQ
  Мы углубляемся в детали реализации DLQ, Retry-механизмов,
  управления задержками и мониторинга, а также рассматриваем продвинутые
  стратегии обработки ошибок в распределённых системах на Kafka.

  1.  АРХИТЕКТУРА DLQ И RETRY В РЕАЛЬНЫХ ПРОДАКШЕН-СИСТЕМАХ
  В крупных системах обработка ошибок — это отдельный архитектурный слой,
  который включает:

    1.1. БАЗОВАЯ СХЕМА (ОСНОВНОЙ ПОТОК)
      Топик: orders (основной поток)
        → Consumer (основной) → Обработка
          → Успех: коммит смещения
          → Ошибка: переход в Retry-механизм

    1.2. RETRY-МЕХАНИЗМ (С ЗАДЕРЖКОЙ)
      Топики: orders-retry-1s, orders-retry-10s, orders-retry-60s
        → Consumer для каждого топика
          → Перед обработкой: задержка (time.Sleep)
          → Обработка → Успех: коммит смещения
          → Ошибка: переход в следующий Retry-топик или DLQ

    1.3. DLQ (DEAD LETTER QUEUE)
      Топик: orders-dlq
        → Consumer для DLQ
          → Логирование, алертинг
          → Ручное вмешательство или автоматическое исправление

  2.  РЕАЛИЗАЦИЯ RETRY С ЗАДЕРЖКОЙ В KAFKA (ПОДРОБНО)

  2.1. ПРОБЛЕМА ЗАДЕРЖКИ В KAFKA
    Kafka не имеет нативной поддержки отложенной доставки (как RabbitMQ
    с Dead Letter Exchange). Это создаёт вызовы:
      • Нет встроенной задержки между попытками.
      • Приходится реализовывать задержку на стороне потребителя.

  2.2. ПОДХОДЫ К РЕАЛИЗАЦИИ ЗАДЕРЖКИ
    А. TIME.SLEEP В КОНСЮМЕРЕ (ПРОСТОЙ)
      • Консюмер читает сообщение из retry-топика и перед обработкой
        засыпает на заданное время.
      • Плюс: простота реализации.
      • Минус: блокируется поток, снижается пропускная способность.

    Б. ОТДЕЛЬНЫЕ ТОПИКИ С РАЗНЫМИ ЗАДЕРЖКАМИ (РАСПРОСТРАНЁННЫЙ)
      • Создаются несколько топиков (например, orders-retry-1s,
        orders-retry-10s, orders-retry-60s).
      • Консюмер для каждого топика обрабатывает сообщения после
        задержки (с помощью time.Sleep или планировщика).
      • Плюс: простота, предсказуемость.
      • Минус: нужно управлять множеством топиков и консюмеров.

    В. ИСПОЛЬЗОВАНИЕ ЗАГОЛОВКОВ И ПЛАНИРОВЩИКА
      • В заголовках сообщения хранится время, когда его можно
        обрабатывать (например, timestamp).
      • Консюмер проверяет это время и, если оно ещё не наступило,
        откладывает обработку (например, с помощью time.Sleep
        до указанного времени) или использует паузу/возобновление.
      • Плюс: гибкость.
      • Минус: сложность реализации.

  2.3. РЕКОМЕНДУЕМЫЙ ПАТТЕРН ДЛЯ ПРОДАКШЕНА
    • Использовать отдельные топики с разными задержками,
      контролируемыми через retention и управление консюмерами.
    • Для каждой задержки создаётся отдельный топик и отдельный
      консюмер (или пул консюмеров).
    • Задержка реализуется через time.Sleep или, для лучшей
      производительности, через использование паузы и возобновления
      на уровне consumer group (но это сложнее).
    • В большинстве real-world проектов используют комбинацию
      отдельного сервиса-планировщика (например, на основе Redis
      или Debezium) для управления задержкой.

  3.  ХРАНЕНИЕ МЕТАДАННЫХ ОШИБКИ В ЗАГОЛОВКАХ СООБЩЕНИЯ
  Критически важно передавать информацию об ошибке вместе с сообщением
  при пересылке в Retry-топик или DLQ.

  3.1. КАКИЕ ДАННЫЕ ХРАНИТЬ
    • retry_count — количество попыток обработки.
    • last_error — текст последней ошибки.
    • last_error_time — время последней ошибки.
    • original_topic — исходный топик (для DLQ).
    • original_partition — исходная партиция.
    • original_offset — исходное смещение.

  3.2. РЕАЛИЗАЦИЯ В GO (С ИСПОЛЬЗОВАНИЕМ ЗАГОЛОВКОВ)
    // Создаём заголовки для сообщения
    headers := []sarama.RecordHeader{
        {Key: []byte("retry_count"), Value: []byte("1")},
        {Key: []byte("last_error"), Value: []byte("user not found")},
        {Key: []byte("last_error_time"), Value: []byte(time.Now().Format(time.RFC3339))},
        {Key: []byte("original_topic"), Value: []byte("orders")},
    }

    // Создаём сообщение для retry-топика
    retryMsg := &sarama.ProducerMessage{
        Topic:    "orders-retry-10s",
        Key:      msg.Key,
        Value:    msg.Value,
        Headers:  headers,
    }

  3.3. ПРЕИМУЩЕСТВА
    • Позволяет отслеживать историю ошибок.
    • Упрощает отладку.
    • Позволяет принимать решения о дальнейшей обработке (например,
      если ошибка повторяется, можно отправить в DLQ).

  4.  РЕАЛИЗАЦИЯ DLQ С ОТДЕЛЬНЫМ КОНСЮМЕРОМ И РУЧНЫМ АНАЛИЗОМ

  4.1. АРХИТЕКТУРА DLQ
    • DLQ-топик: orders-dlq
    • Отдельный консюмер (или группа консюмеров) для чтения из DLQ.
    • При чтении из DLQ консюмер логирует ошибку, отправляет алерт,
      и, возможно, сохраняет сообщение в БД для ручного анализа.

  4.2. ОБРАБОТКА DLQ НА ПРАКТИКЕ
    • В большинстве систем DLQ обрабатывается вручную: инженеры
      анализируют сообщения, исправляют ошибки и повторно отправляют
      их в основной топик.
    • В некоторых автоматизированных системах DLQ может быть
      интегрирован с системой поддержки (Jira, ServiceNow) для
      автоматического создания тикетов.

  4.3. РЕАЛИЗАЦИЯ В GO (КОНЦЕПТУАЛЬНО)
    // Консюмер для DLQ
    func consumeDLQ() {
        for {
            msg := <-dlqConsumer.Messages()
            log.Printf("DLQ: %s, error: %v", string(msg.Value), getHeader(msg, "last_error"))
            // Отправляем алерт
            sendAlert(msg)
            // Сохраняем в БД для ручного анализа
            saveToDatabase(msg)
            // Не коммитим смещение — сообщение останется в DLQ до ручного вмешательства
            // или коммитим после сохранения.
        }
    }

  5.  МЕТРИКИ И МОНИТОРИНГ DLQ И RETRY
  Без мониторинга DLQ и Retry-топиков вы не узнаете о проблемах, пока
  они не станут критическими.

  5.1. КЛЮЧЕВЫЕ МЕТРИКИ
    • Количество сообщений в каждом Retry-топике (по уровням задержки).
    • Количество сообщений в DLQ.
    • Скорость роста DLQ (сообщений/мин).
    • Среднее время обработки сообщения (для выявления задержек).
    • Количество ошибок по типам (для выявления частых проблем).

  5.2. ИНСТРУМЕНТЫ МОНИТОРИНГА
    • Prometheus + Grafana для визуализации метрик.
    • Burrow (LinkedIn) для мониторинга consumer lag.
    • Консольные утилиты (kafka-consumer-groups.sh) для ручной проверки.

  5.3. АЛЕРТИНГ
    • Настройте алерты при превышении порогового значения количества
      сообщений в DLQ (например, > 100 за 5 минут).
    • Алерты на рост consumer lag для Retry-топиков.

  6.  СРАВНЕНИЕ ПОДХОДОВ: DLQ VS RETRY WITH BACKOFF VS SKIP AND LOG
  ┌─────────────────────────┬─────────────────────────────────────────────────┐
  │ Подход                  │ Характеристики                                  │
  ├─────────────────────────┼─────────────────────────────────────────────────┤
  │ DLQ                     │ Сохраняет все данные; требует ручного           │
  │                         │ вмешательства; подходит для критичных данных.   │
  ├─────────────────────────┼─────────────────────────────────────────────────┤
  │ Retry with Backoff      │ Автоматически повторяет; подходит для           │
  │                         │ временных ошибок; может быть сложным в          │
  │                         │ реализации.                                     │
  ├─────────────────────────┼─────────────────────────────────────────────────┤
  │ Skip and Log            │ Простота; риск потери данных; подходит для      │
  │                         │ некритичных данных.                             │
  └─────────────────────────┴─────────────────────────────────────────────────┘

  7.  ПРОДВИНУТЫЕ СТРАТЕГИИ: CONDITIONAL RETRY, CIRCUIT BREAKER, FALLBACK

  7.1. CONDITIONAL RETRY (УСЛОВНЫЙ RETRY)
    • Повторять только для определённых типов ошибок (например,
      временных сетевых ошибок, а не для ошибок валидации).
    • Реализуется через анализ текста ошибки или кода ошибки.

  7.2. CIRCUIT BREAKER (ПРЕДОХРАНИТЕЛЬ)
    • Если количество ошибок превышает порог, временно прекращать
      попытки обработки, чтобы дать системе восстановиться.
    • Реализуется с помощью библиотек (например, gobreaker) или
      вручную с помощью состояния.

  7.3. FALLBACK (ЗАПАСНОЙ ВАРИАНТ)
    • Если обработка не удалась, использовать альтернативную логику
      (например, записать в кэш или отправить уведомление).
    • Подходит для систем, где допустима частичная обработка.

  8.  ПРИМЕР ПОЛНОЙ РЕАЛИЗАЦИИ НА GO (КОНЦЕПТУАЛЬНЫЙ КОД)
  // Основной консюмер
  func consumeMain() {
      for {
          msg := <-mainConsumer.Messages()
          err := processMessage(msg)
          if err == nil {
              mainConsumer.MarkMessage(msg, "")
              continue
          }
          // Ошибка
          retryCount := getRetryCount(msg) // из заголовков
          if retryCount >= maxRetries {
              sendToDLQ(msg, err)
              mainConsumer.MarkMessage(msg, "")
              continue
          }
          // Отправляем в retry с задержкой
          delay := getDelayForRetry(retryCount)
          sendToRetryTopic(msg, delay)
          // Не коммитим смещение (оно останется для повторного чтения, если retry не сработает)
          // Но для избежания бесконечного цикла, можно коммитить после отправки в retry,
          // если retry-обработка гарантирована.
      }
  }
  // Консюмер для retry (с задержкой)
  func consumeRetry(delay time.Duration) {
      for {
          msg := <-retryConsumer.Messages()
          time.Sleep(delay)
          err := processMessage(msg)
          if err == nil {
              retryConsumer.MarkMessage(msg, "")
              continue
          }
          // Ошибка в retry — увеличиваем счётчик и отправляем дальше
          incrementRetryCount(msg)
          sendToNextRetryOrDLQ(msg, err)
      }
  }
  // Функция отправки в DLQ
  func sendToDLQ(msg *sarama.ConsumerMessage, err error) {
      headers := []sarama.RecordHeader{
          {Key: []byte("last_error"), Value: []byte(err.Error())},
          {Key: []byte("original_topic"), Value: []byte("orders")},
      }
      producer.SendMessage(&sarama.ProducerMessage{
          Topic:   "orders-dlq",
          Key:     msg.Key,
          Value:   msg.Value,
          Headers: headers,
      })
  }

  9.  ЧАСТЫЕ ОШИБКИ ПРИ РЕАЛИЗАЦИИ DLQ И RETRY

  9.1. НЕ ПРОВЕРЯТЬ IDEMPOTENTNOST
    Если retry повторяет обработку, необходимо убедиться, что операция
    идемпотентна (повторное выполнение не меняет результат).

  9.2. НЕ ОГРАНИЧИВАТЬ КОЛИЧЕСТВО ПОПЫТОК
    Бесконечные retry могут привести к бесконечному циклу и
    переполнению топиков.

  9.3. НЕ МОНИТОРИТЬ DLQ
    Если не мониторить DLQ, вы можете потерять данные без уведомления.

  9.4. НЕ ИСПОЛЬЗОВАТЬ ЗАГОЛОВКИ ДЛЯ МЕТАДАННЫХ
    Без заголовков невозможно определить количество попыток и причину
    ошибки, что затрудняет отладку.

  10. КЛЮЧЕВЫЕ ВЫВОДЫ ДЛЯ СОБЕСЕДОВАНИЯ

  1.  DLQ и Retry — обязательные компоненты production-систем на Kafka
      для обработки ошибок и предотвращения потери данных.
  2.  Retry-механизм должен включать экспоненциальную задержку и
      ограничение количества попыток.
  3.  Используйте заголовки сообщений для хранения метаданных ошибки
      (количество попыток, текст ошибки, время).
  4.  Мониторинг DLQ и Retry-топиков — критически важен для быстрого
      обнаружения проблем.
  5.  Выбор стратегии (DLQ, retry, skip) зависит от бизнес-требований
      и характера ошибок.
  6.  Conditional Retry, Circuit Breaker и Fallback — продвинутые
      стратегии, которые могут значительно повысить устойчивость
      системы.
  7.  Всегда проверяйте идемпотентность операций при retry, чтобы
      избежать дублей.
  8.  Для реализации задержки в Kafka используйте отдельные топики
      с разными задержками или внешний планировщик.
  9.  DLQ должен обрабатываться отдельным консюмером с системой
      алертинга и ручного вмешательства.
  10. Тестируйте сценарии ошибок и убедитесь, что DLQ и retry работают
      корректно.
*/

//ОБЩИЕ КОНСТАНТЫ И ПЕРЕМЕННЫЕ

const (
	broker        = "localhost:9092"
	inputTopic    = "orders"
	retryTopic    = "orders-retry"
	dlqTopic      = "orders-dlq"
	consumerGroup = "order-processor"
)

var (
	mode       = flag.String("mode", "retry", "Режим: retry (базовый) или advanced (продвинутый)")
	maxRetries = flag.Int("retries", 3, "Максимальное количество попыток")
)

// ОБЩИЕ УТИЛИТЫ
func createTopicsIfNotExist() error {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	admin, err := sarama.NewClusterAdmin([]string{broker}, config)
	if err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}
	defer admin.Close()

	topics := []string{inputTopic, retryTopic, dlqTopic}
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

func getHeader(msg *sarama.ConsumerMessage, key string) string {
	for _, h := range msg.Headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func getRetryCount(msg *sarama.ConsumerMessage) int {
	v := getHeader(msg, "retry_count")
	if v == "" {
		return 0
	}
	var count int
	fmt.Sscanf(v, "%d", &count)
	return count
}

func createProducer() (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	return sarama.NewSyncProducer([]string{broker}, config)
}

// ПРИМЕР 1: БАЗОВЫЙ DLQ + RETRY
func runRetryMode() error {
	log.Println("Запуск режима: DLQ + Retry")

	if err := createTopicsIfNotExist(); err != nil {
		return err
	}

	producer, err := createProducer()
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer producer.Close()

	// Запускаем консюмер для основного топика
	go consumeMain(producer)
	// Запускаем консюмер для retry-топика
	go consumeRetry(producer)

	// Ожидаем сигнала
	waitForShutdown()
	return nil
}

func consumeMain(producer sarama.SyncProducer) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup([]string{broker}, consumerGroup+"-main", config)
	if err != nil {
		log.Fatalf("Ошибка создания консюмера: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &MainHandler{producer: producer}
	go func() {
		for {
			if err := client.Consume(ctx, []string{inputTopic}, handler); err != nil {
				log.Printf("Ошибка консюмера main: %v", err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	<-ctx.Done()
}

type MainHandler struct {
	producer sarama.SyncProducer
}

func (h *MainHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *MainHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *MainHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Получено: %s", string(msg.Value))
		err := processMessage(msg)
		if err == nil {
			session.MarkMessage(msg, "")
			log.Printf("Обработано: offset=%d", msg.Offset)
			continue
		}

		retryCount := getRetryCount(msg) + 1
		if retryCount > *maxRetries {
			log.Printf("Превышено количество попыток (%d), отправка в DLQ", *maxRetries)
			if sendErr := sendToDLQ(h.producer, msg, err); sendErr != nil {
				log.Printf("Ошибка отправки в DLQ: %v", sendErr)
				continue
			}
			session.MarkMessage(msg, "")
			continue
		}

		if sendErr := sendToRetry(h.producer, msg, err, retryCount); sendErr != nil {
			log.Printf("Ошибка отправки в retry: %v", sendErr)
			continue
		}
		log.Printf("🔄 Отправлено в retry (попытка %d)", retryCount)
		session.MarkMessage(msg, "")
	}
	return nil
}

func sendToRetry(producer sarama.SyncProducer, msg *sarama.ConsumerMessage, err error, retryCount int) error {
	headers := []sarama.RecordHeader{
		{Key: []byte("retry_count"), Value: []byte(fmt.Sprintf("%d", retryCount))},
		{Key: []byte("last_error"), Value: []byte(err.Error())},
		{Key: []byte("last_error_time"), Value: []byte(time.Now().Format(time.RFC3339))},
	}
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic:   retryTopic,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
	return err
}

func sendToDLQ(producer sarama.SyncProducer, msg *sarama.ConsumerMessage, err error) error {
	headers := []sarama.RecordHeader{
		{Key: []byte("last_error"), Value: []byte(err.Error())},
		{Key: []byte("original_topic"), Value: []byte(inputTopic)},
		{Key: []byte("original_offset"), Value: []byte(fmt.Sprintf("%d", msg.Offset))},
		{Key: []byte("time"), Value: []byte(time.Now().Format(time.RFC3339))},
	}
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic:   dlqTopic,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
	return err
}

func consumeRetry(producer sarama.SyncProducer) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup([]string{broker}, consumerGroup+"-retry", config)
	if err != nil {
		log.Fatalf("Ошибка создания консюмера retry: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &RetryHandler{producer: producer}
	go func() {
		for {
			if err := client.Consume(ctx, []string{retryTopic}, handler); err != nil {
				log.Printf("Ошибка консюмера retry: %v", err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	<-ctx.Done()
}

type RetryHandler struct {
	producer sarama.SyncProducer
}

func (h *RetryHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *RetryHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *RetryHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		delay := time.Duration(getRetryCount(msg)) * 5 * time.Second
		log.Printf("Retry: задержка %v перед обработкой", delay)
		time.Sleep(delay)

		log.Printf("Retry: обработка %s", string(msg.Value))
		err := processMessage(msg)
		if err == nil {
			session.MarkMessage(msg, "")
			log.Printf("Retry успешен: offset=%d", msg.Offset)
			continue
		}

		retryCount := getRetryCount(msg) + 1
		if retryCount > *maxRetries {
			log.Printf("Превышено количество попыток в retry (%d), отправка в DLQ", *maxRetries)
			if sendErr := sendToDLQ(h.producer, msg, err); sendErr != nil {
				log.Printf("Ошибка отправки в DLQ: %v", sendErr)
				continue
			}
			session.MarkMessage(msg, "")
			continue
		}
		if sendErr := sendToRetry(h.producer, msg, err, retryCount); sendErr != nil {
			log.Printf("Ошибка повторной отправки в retry: %v", sendErr)
			continue
		}
		session.MarkMessage(msg, "")
		log.Printf("Повторная отправка в retry (попытка %d)", retryCount)
	}
	return nil
}

// ─── ПРИМЕР 2: ADVANCED (CONDITIONAL RETRY + CIRCUIT BREAKER + FALLBACK) ──

func runAdvancedMode() error {
	log.Println("🚀 Запуск режима: Conditional Retry + Circuit Breaker + Fallback")

	if err := createTopicsIfNotExist(); err != nil {
		return err
	}

	producer, err := createProducer()
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer producer.Close()

	go consumeAdvanced(producer)

	waitForShutdown()
	return nil
}

// Circuit Breaker состояние
type CircuitBreaker struct {
	mu           sync.Mutex
	failures     int
	threshold    int
	timeWindow   time.Duration
	lastFailTime time.Time
	state        string // "CLOSED", "OPEN", "HALF_OPEN"
}

func NewCircuitBreaker(threshold int, window time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:  threshold,
		timeWindow: window,
		state:      "CLOSED",
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	now := time.Now()
	if now.Sub(cb.lastFailTime) > cb.timeWindow {
		cb.failures = 0
	}
	cb.failures++
	cb.lastFailTime = now
	if cb.failures >= cb.threshold {
		cb.state = "OPEN"
		log.Printf("Circuit Breaker открыт (failures=%d)", cb.failures)
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == "OPEN" || cb.state == "HALF_OPEN" {
		cb.failures = 0
		cb.state = "CLOSED"
		log.Printf("Circuit Breaker закрыт")
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == "CLOSED" {
		return true
	}
	if cb.state == "OPEN" {
		if time.Since(cb.lastFailTime) > cb.timeWindow {
			cb.state = "HALF_OPEN"
			log.Printf("Circuit Breaker в HALF_OPEN")
			return true
		}
		return false
	}
	// HALF_OPEN
	return true
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

var cb = NewCircuitBreaker(3, 10*time.Second)

func consumeAdvanced(producer sarama.SyncProducer) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup([]string{broker}, consumerGroup+"-advanced", config)
	if err != nil {
		log.Fatalf("Ошибка создания консюмера advanced: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &AdvancedHandler{producer: producer}
	go func() {
		for {
			if err := client.Consume(ctx, []string{inputTopic}, handler); err != nil {
				log.Printf("Ошибка консюмера advanced: %v", err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	<-ctx.Done()
}

type AdvancedHandler struct {
	producer sarama.SyncProducer
}

func (h *AdvancedHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *AdvancedHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *AdvancedHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Advanced: получено %s", string(msg.Value))

		if !cb.Allow() {
			log.Printf("⚠Circuit Breaker открыт, fallback")
			h.sendFallback(msg)
			session.MarkMessage(msg, "")
			continue
		}

		err := processMessageAdvanced(msg)
		if err == nil {
			cb.RecordSuccess()
			session.MarkMessage(msg, "")
			log.Printf("Успешно обработано")
			continue
		}

		if isTemporaryError(err) {
			log.Printf("Временная ошибка: %v", err)
			retryCount := getRetryCount(msg) + 1
			if retryCount <= *maxRetries {
				h.sendToConditionalRetry(msg, err, retryCount)
				session.MarkMessage(msg, "")
				continue
			}
		}

		cb.RecordFailure()
		log.Printf("Постоянная ошибка или превышены попытки: %v", err)
		h.sendToDLQAdvanced(msg, err)
		session.MarkMessage(msg, "")
	}
	return nil
}

func isTemporaryError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "connection") || strings.Contains(msg, "temporary")
}

func (h *AdvancedHandler) sendToConditionalRetry(msg *sarama.ConsumerMessage, err error, retryCount int) {
	headers := []sarama.RecordHeader{
		{Key: []byte("retry_count"), Value: []byte(fmt.Sprintf("%d", retryCount))},
		{Key: []byte("last_error"), Value: []byte(err.Error())},
		{Key: []byte("type"), Value: []byte("conditional_retry")},
	}
	_, _, _ = h.producer.SendMessage(&sarama.ProducerMessage{
		Topic:   retryTopic,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
	log.Printf("Условный retry (попытка %d)", retryCount)
}

func (h *AdvancedHandler) sendToDLQAdvanced(msg *sarama.ConsumerMessage, err error) {
	headers := []sarama.RecordHeader{
		{Key: []byte("last_error"), Value: []byte(err.Error())},
		{Key: []byte("circuit_breaker_state"), Value: []byte(cb.State())},
		{Key: []byte("time"), Value: []byte(time.Now().Format(time.RFC3339))},
	}
	_, _, _ = h.producer.SendMessage(&sarama.ProducerMessage{
		Topic:   dlqTopic,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
}

func (h *AdvancedHandler) sendFallback(msg *sarama.ConsumerMessage) {
	log.Printf("⚠Fallback для сообщения: %s", string(msg.Value))
	headers := []sarama.RecordHeader{
		{Key: []byte("fallback"), Value: []byte("true")},
		{Key: []byte("time"), Value: []byte(time.Now().Format(time.RFC3339))},
	}
	_, _, _ = h.producer.SendMessage(&sarama.ProducerMessage{
		Topic:   "orders-fallback",
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder([]byte(`{"status":"fallback","original":` + string(msg.Value) + `}`)),
		Headers: headers,
	})
}

//ОБЩАЯ ЛОГИКА ОБРАБОТКИ

func processMessage(msg *sarama.ConsumerMessage) error {
	if string(msg.Value) == `{"order":"bad"}` || string(msg.Value) == `{"order":"fail"}` {
		return fmt.Errorf("processing error")
	}
	return nil
}

func processMessageAdvanced(msg *sarama.ConsumerMessage) error {
	val := string(msg.Value)
	if val == `{"order":"timeout"}` {
		return fmt.Errorf("timeout error")
	}
	if val == `{"order":"connection"}` {
		return fmt.Errorf("connection refused")
	}
	if val == `{"order":"invalid"}` {
		return fmt.Errorf("validation error")
	}
	if val == `{"order":"fail"}` {
		return fmt.Errorf("permanent error")
	}
	return nil
}

// ─── ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ─────────────────────────────────────────────────

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Завершение...")
}

func main() {
	flag.Parse()

	var err error
	switch *mode {
	case "retry":
		err = runRetryMode()
	case "advanced":
		err = runAdvancedMode()
	default:
		log.Fatalf("Неверный режим: %s. Используйте -mode=retry или -mode=advanced", *mode)
	}
	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
