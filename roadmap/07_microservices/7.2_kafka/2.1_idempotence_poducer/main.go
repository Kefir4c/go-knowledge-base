package idempotencepoducer

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

/*
  БЛОК 2.1: ИДЕМПОТЕНТНЫЙ ПРОДЮСЕР
  Идемпотентный продюсер — это механизм Kafka, который гарантирует, что
  каждое сообщение будет записано в партицию ровно один раз, даже при
  повторных попытках отправки (retries). Это первый шаг к exactly-once
  семантике и база для построения надёжных систем.

  1.  ПРОБЛЕМА: ПОЧЕМУ НУЖЕН ИДЕМПОТЕНТНЫЙ ПРОДЮСЕР
  Kafka по умолчанию предоставляет "at-least-once" семантику доставки.
  Это означает, что сообщение может быть доставлено один или несколько раз
  (то есть возможны дубли).

  ПРИЧИНЫ ПОЯВЛЕНИЯ ДУБЛЕЙ:
    • Сетевые ошибки: продюсер отправил сообщение, но не получил
      подтверждение от брокера из-за обрыва сети. Продюсер повторяет отправку,
      а сообщение уже было записано.
    • Сбой брокера: брокер записал сообщение, но упал до отправки ответа.
      Продюсер повторяет запрос, и сообщение может быть записано дважды.
    • Ошибки при репликации: при сбое лидера новая реплика может не знать
      о уже записанных сообщениях.

  Эта проблема решается идемпотентным продюсером.

  2.  ЧТО ТАКОЕ ИДЕМПОТЕНТНЫЙ ПРОДЮСЕР
  Идемпотентный продюсер гарантирует, что отправка одного и того же
  сообщения несколько раз приведёт к тому, что в Kafka будет записана
  только одна копия.

  КЛЮЧЕВЫЕ ХАРАКТЕРИСТИКИ:
    • Гарантирует отсутствие дублей в партиции при повторных отправках.
    • Работает на уровне одной партиции (не между разными партициями).
    • Требует правильной настройки продюсера.
    • Является основой для exactly-once семантики.

  3.  КАК ЭТО РАБОТАЕТ: PID, SEQUENCE NUMBER И EPOCH
  Kafka использует три ключевых механизма для обеспечения идемпотентности:

  3.1. PRODUCER ID (PID)
  Каждый экземпляр идемпотентного продюсера при инициализации получает
  уникальный идентификатор (Producer ID) от брокера.
    • PID назначается прозрачно для пользователя.
    • PID действует в течение всей сессии продюсера.
    • PID используется для отслеживания последовательности сообщений.

  3.2. SEQUENCE NUMBER (ПОРЯДКОВЫЙ НОМЕР)
  Каждое сообщение получает порядковый номер для каждой комбинации
  "продюсер-топик-партиция".
    • Нумерация начинается с 0 для каждой комбинации.
    • Номер увеличивается на 1 для каждого отправленного сообщения.
    • Брокер использует этот номер для обнаружения дублей.

  3.3. PRODUCER EPOCH (ЭПОХА ПРОДЮСЕРА)
  Epoch — это счётчик, который увеличивается при каждом изменении PID
  (например, при перезапуске продюсера).
    • Позволяет брокеру отличать нового продюсера от старого с тем же PID.
    • При получении OutOfOrderSequenceException продюсер увеличивает epoch.
    • После смены epoch сбрасывается sequence number.

  3.4. КАК ЭТО РАБОТАЕТ ВМЕСТЕ
    1. Продюсер инициализируется и получает PID от брокера.
    2. При отправке сообщения продюсер добавляет PID и sequence number.
    3. Брокер проверяет последний записанный sequence number для этого PID:
       - Если новый sequence number = last + 1 → записывает сообщение.
       - Если sequence number уже был записан → отбрасывает дубликат и
         возвращает успех.
       - Если sequence number > last + 1 → выбрасывает
         OutOfOrderSequenceException.
    4. При ошибке OutOfOrderSequenceException продюсер увеличивает epoch.

  4.  НАСТРОЙКА ИДЕМПОТЕНТНОГО ПРОДЮСЕРА

  4.1. ОСНОВНЫЕ НАСТРОЙКИ
    enable.idempotence = true  # Включает идемпотентность

  При включении идемпотентности Kafka автоматически устанавливает:
    • retries = Integer.MAX_VALUE — бесконечные повторные попытки
    • max.in.flight.requests.per.connection = 5 — максимум 5 запросов
      одновременно (сохраняет порядок)
    • acks = all — подтверждение от всех реплик в ISR

  4.2. ДЕФОЛТНОЕ ПОВЕДЕНИЕ В KAFKA 3.0+
  Начиная с Kafka 3.0, идемпотентный продюсер включён ПО УМОЛЧАНИЮ.

  Версии Kafka и значение по умолчанию enable.idempotence:
    • До Kafka 3.0: false
    • Kafka 3.0+: true

  4.3. ПОЛНАЯ КОНФИГУРАЦИЯ ДЛЯ ПРОДАКШЕНА
    enable.idempotence = true
    acks = all
    retries = Integer.MAX_VALUE
    max.in.flight.requests.per.connection = 5
    delivery.timeout.ms = 120000
    compression.type = snappy
    batch.size = 32768
    linger.ms = 5

  4.4. ОГРАНИЧЕНИЯ НАСТРОЕК
    • max.in.flight.requests.per.connection должен быть ≤ 5 для сохранения
      порядка сообщений.
    • retries должен быть > 0.
    • acks должен быть all.

  5.  ОГРАНИЧЕНИЯ И ПОДВОДНЫЕ КАМНИ

  5.1. ТОЛЬКО В ПРЕДЕЛАХ ОДНОЙ ПАРТИЦИИ
  Идемпотентный продюсер гарантирует отсутствие дублей только в пределах
  одной партиции. Для атомарной записи в несколько партиций
  нужны транзакции.

  5.2. НЕ ЗАЩИЩАЕТ ОТ ДУБЛЕЙ МЕЖДУ СЕССИЯМИ
  При перезапуске продюсера он получает новый PID. Сообщения, отправленные
  до перезапуска, не могут быть дедуплицированы относительно новых,
  так как PID изменился.

  5.3. ПАМЯТЬ НА БРОКЕРЕ
  Брокер должен хранить состояние последовательности для каждого PID
  и партиции. При большом количестве продюсеров это создаёт
  дополнительную нагрузку на память.

  5.4. ОШИБКА OUTOFORDERSEQUENCEEXCEPTION
  Если продюсер получает эту ошибку, это означает, что порядок сообщений
  нарушен. Продюсер должен увеличить epoch и повторить отправку.

  6.  ПРОИЗВОДИТЕЛЬНОСТЬ И КОМПРОМИССЫ
  Идемпотентный продюсер добавляет небольшие накладные расходы:
    • Память на брокере для хранения состояния последовательностей.
    • Небольшое увеличение задержки из-за проверки sequence number.
    • Ограничение max.in.flight.requests.per.connection = 5.

  Компромиссы:
    • Без идемпотентности, retries=0 → самая высокая пропускная способность,
      но возможна потеря данных.
    • Без идемпотентности, retries>0 → высокая пропускная способность,
      но возможны дубли.
    • Идемпотентный продюсер → средняя-высокая пропускная способность,
      без дублей.

  Включение идемпотентности добавляет нулевые накладные расходы на пропускную
  способность (сравнимо с acks=all) и требуется для exactly-once семантики.

  7.  КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  Идемпотентный продюсер гарантирует, что каждое сообщение будет
      записано в партицию ровно один раз, даже при повторных отправках.
  2.  Два ключевых механизма: Producer ID (PID) и Sequence Number.
  3.  PID назначается брокером при инициализации продюсера.
  4.  Sequence Number увеличивается для каждого сообщения в рамках
      одной партиции.
  5.  Брокер проверяет sequence number: ожидаемый → запись, дубликат → игнор,
      вне очереди → OutOfOrderSequenceException.
  6.  Epoch позволяет отличать нового продюсера от старого с тем же PID.
  7.  Включается: enable.idempotence=true.
  8.  С Kafka 3.0+ включён по умолчанию.
  9.  Требует: acks=all, retries>0, max.in.flight.requests.per.connection≤5.
  10. Работает только в пределах одной партиции.
  11. Для exactly-once между партициями нужны транзакции.
  12. Идемпотентность — первый шаг к exactly-once и EOS.
*/

//КОНФИГУРАЦИЯ

var (
	topic         = "idempotest-test"
	broker        = "localhost:9092"
	messageToSend = 3
)

var (
	enableIdempotence = flag.Bool("idempotent", false, "Включить идемпотентность")
	retries           = flag.Int("retries", 3, "Количество повторных попыток")
)

//ПРОДЮСЕР

type Producer struct {
	client sarama.SyncProducer
}

func NewProducer(enableIdempotence bool, retries int) (*Producer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0 // Kafka 3.0+ (поддерживает KRaft)

	// Базовые настройки
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = retries

	//КЛЮЧЕВАЯ НАСТРОЙКА — ВКЛЮЧАЕМ ИДЕМПОТЕНТНОСТЬ
	config.Producer.Idempotent = enableIdempotence

	if enableIdempotence {
		// При включении идемпотентности обязательно:
		config.Producer.RequiredAcks = sarama.WaitForAll
		// Максимум 5 запросов одновременно (сохраняет порядок, но допускает параллелизм)
		log.Println("Идемпотентность ВКЛЮЧЕНА")
	} else {
		// Без идемпотентности можно использовать acks=1 для скорости
		config.Producer.RequiredAcks = sarama.WaitForLocal
		log.Println("Идемпотентность ВЫКЛЮЧЕНА")
	}

	client, err := sarama.NewSyncProducer([]string{broker}, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) Close() error {
	return p.client.Close()
}

// SendMessage отправляет сообщение с имитацией сбоя (повторные попытки)
func (p *Producer) SendMessage(key, value string, failCount int) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(value),
	}

	for attempt := 0; attempt <= failCount; attempt++ {
		if attempt > 0 {
			log.Printf("Повторная попытка #%d для key=%s", attempt, key)
			time.Sleep(500 * time.Millisecond)
		}
		partition, offset, err := p.client.SendMessage(msg)
		if err != nil {
			log.Printf("Ошибка (попытка %d): %v", attempt+1, err)
			continue
		}
		log.Printf("Отправлено: key=%s, value=%s, partition=%d, offset=%d", key, value, partition, offset)
		return nil
	}
	return fmt.Errorf("failed to send after %d attempts", failCount+1)
}

// SendDuplicate демонстрирует дублирование — повторная отправка того же сообщения
func (p *Producer) SendDuplicate(key, value string) {
	log.Printf("[1] Отправка: key=%s, value=%s", key, value)
	if err := p.SendMessage(key, value, 0); err != nil {
		log.Printf("Ошибка: %v", err)
		return
	}
	log.Printf("[2] Повторная отправка (имитация дубля): key=%s, value=%s", key, value)
	if err := p.SendMessage(key, value, 0); err != nil {
		log.Printf("Ошибка: %v", err)
	}
}

//КОНСЮМЕР

type Consumer struct {
	client sarama.Consumer
}

func NewConsumer() (*Consumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumer([]string{broker}, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	return &Consumer{client: client}, nil
}

func (c *Consumer) Close() error {
	return c.client.Close()
}

// ReadAllMessages читает все сообщения из топика и возвращает их
func (c *Consumer) ReadAllMessages() ([]string, error) {
	partitions, err := c.client.Partitions(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}

	var messages []string
	for _, partition := range partitions {
		pc, err := c.client.ConsumePartition(topic, partition, sarama.OffsetOldest)
		if err != nil {
			return nil, fmt.Errorf("failed to consume partition %d: %w", partition, err)
		}
		defer pc.Close()

		for {
			select {
			case msg := <-pc.Messages():
				messages = append(messages, fmt.Sprintf("key=%s, value=%s, offset=%d, partition=%d",
					string(msg.Key), string(msg.Value), msg.Offset, msg.Partition))
			default:
				goto nextPartition
			}
		}
	nextPartition:
	}
	return messages, nil
}

// УТИЛИТЫ
func createTopicIfNotExists() error {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	admin, err := sarama.NewClusterAdmin([]string{broker}, config)
	if err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}
	defer admin.Close()

	topics, err := admin.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}
	if _, exist := topics[topic]; exist {
		log.Printf("Топик %s уже существует", topic)
		if err := admin.DeleteTopic(topic); err != nil {
			log.Printf("Не удалось удалить топик: %v", err)
		} else {
			log.Printf("Топик %s удалён", topic)
		}
		time.Sleep(2 * time.Second)
	}

	err = admin.CreateTopic(topic, &sarama.TopicDetail{
		NumPartitions:     3,
		ReplicationFactor: 1,
	}, false)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}
	log.Printf("✅ Топик %s создан", topic)
	return nil
}

// MAIN
func main() {
	flag.Parse()

	log.Printf("Запуск с идемпотентностью: %v", *enableIdempotence)
	log.Printf("Повторных попыток: %d", *retries)

	if err := createTopicIfNotExists(); err != nil {
		log.Fatalf("❌ Ошибка создания топика: %v", err)
	}

	producer, err := NewProducer(*enableIdempotence, *retries)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	log.Println("Отправка уникальных сообщений с имитацией сбоев...")
	var wg sync.WaitGroup
	keys := []string{"user-1", "user-2", "user-3"}
	values := []string{"event-A", "event-B", "event-C"}

	for i := 0; i < len(keys); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := producer.SendMessage(keys[idx], values[idx], 2); err != nil {
				log.Printf("Ошибка отправки: %v", err)
			}
		}(i)
	}
	wg.Wait()

	log.Println("\nДемонстрация дублирования (отправка одного сообщения дважды):")
	producer.SendDuplicate("duplicate-key", "duplicate-value")

	time.Sleep(2 * time.Second)

	log.Println("\n📥 Чтение всех сообщений...")
	consumer, err := NewConsumer()
	if err != nil {
		log.Fatalf("Ошибка создания консюмера: %v", err)
	}
	defer consumer.Close()

	messages, err := consumer.ReadAllMessages()
	if err != nil {
		log.Fatalf("Ошибка чтения: %v", err)
	}

	log.Printf("Всего сообщений: %d", len(messages))
	for _, msg := range messages {
		log.Printf("  %s", msg)
	}

	if *enableIdempotence {
		log.Println("Идемпотентность ВКЛЮЧЕНА")
	} else {
		log.Println("Идемпотентность ВЫКЛЮЧЕНА")
	}

	seen := make(map[string]int)
	for _, msg := range messages {
		seen[msg]++
	}
	duplicates := 0
	for key, count := range seen {
		if count > 1 {
			log.Printf("ДУБЛИКАТ: '%s' встретилось %d раз", key, count)
			duplicates++
		}
	}
	if duplicates == 0 {
		log.Println("Дублей не найдено")
	} else {
		log.Printf("Найдено %d дубликатов", duplicates)
	}
}
