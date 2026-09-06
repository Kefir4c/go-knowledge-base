package isr

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

/*
  БЛОК 4.3: ISR (IN-SYNC REPLICAS) И MIN.INSYNC.REPLICAS
  ISR (In-Sync Replicas) и min.insync.replicas — это фундаментальные
  механизмы Kafka, обеспечивающие баланс между надёжностью данных
  (durability) и доступностью (availability). Без глубокого понимания
  этих концепций невозможно проектировать отказоустойчивые системы.

  В этой теории мы разберём:
    1. Что такое ISR и как он работает
    2. Как реплики попадают и выходят из ISR
    3. min.insync.replicas — защита от скрытых потерь данных
    4. Влияние на надёжность (durability)
    5. Влияние на доступность (availability)
    6. Связь с acks=all и replication.factor
    7. Компромиссы: надёжность vs доступность
    8. Подводные камни и частые ошибки
    9. Настройка для production
    10. Мониторинг ISR
    11. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ ISR (IN-SYNC REPLICAS)
  ISR (In-Sync Replicas) — это набор реплик партиции, которые полностью
  синхронизированы с лидером. Только реплики из ISR могут стать лидером
  при сбое текущего лидера.

  КЛЮЧЕВЫЕ ХАРАКТЕРИСТИКИ:
    • ISR — это динамический набор реплик.
    • Реплика входит в ISR, если она не отстаёт от лидера больше чем
      на replica.lag.time.max.ms (по умолчанию 10 секунд).
    • Реплика покидает ISR, если она перестаёт получать данные от лидера
      или отстаёт больше допустимого.
    • Лидер всегда входит в ISR.
    • Только реплики из ISR участвуют в подтверждении записей при acks=all.

  ЗАЧЕМ НУЖЕН ISR:
    • Гарантирует, что данные не будут потеряны при сбое лидера.
    • Позволяет гибко управлять уровнем надёжности.
    • Обеспечивает баланс между долговечностью и доступностью.

  2.  КАК РЕПЛИКИ ПОПАДАЮТ И ВЫХОДЯТ ИЗ ISR

  2.1. ПОПАДАНИЕ В ISR
    Реплика становится частью ISR, когда:
      • Она успешно присоединилась к кластеру.
      • Она догнала лидера (её смещение не меньше, чем у лидера).
      • Она продолжает получать данные от лидера без задержек.
      • Её отставание не превышает replica.lag.time.max.ms.

  2.2. ВЫХОД ИЗ ISR (REPLICA LAG)
    Реплика выходит из ISR, если:
      • Она перестала получать данные от лидера (сбой сети, падение брокера).
      • Она отстаёт от лидера больше чем на replica.lag.time.max.ms.
      • Она не может догнать лидера из-за высокой нагрузки.

  2.3. УПРАВЛЕНИЕ ISR
    • Лидер периодически проверяет состояние реплик.
    • Если реплика отстала, она исключается из ISR.
    • Если реплика догнала, она снова включается в ISR.
    • ISR динамически меняется, что влияет на поведение acks=all.

  3.  MIN.INSYNC.REPLICAS — ЗАЩИТА ОТ СКРЫТЫХ ПОТЕРЬ ДАННЫХ
  min.insync.replicas — это настройка на уровне топика (или брокера),
  которая определяет МИНИМАЛЬНОЕ количество реплик в ISR, необходимое
  для подтверждения записи при acks=all.

  3.1. ПОЧЕМУ ЭТО НУЖНО
    Без min.insync.replicas, acks=all может давать ложное чувство
    безопасности. Если ISR состоит только из лидера (например,
    при падении двух брокеров из трёх), то acks=all фактически
    эквивалентен acks=1, так как лидер получает подтверждение сам от себя.
    Это приводит к потере данных при сбое лидера.

  3.2. КАК ЭТО РАБОТАЕТ
    • При acks=all лидер ждёт подтверждения от ВСЕХ реплик в ISR.
    • НО если ISR состоит из 1 реплики (только лидер), то это acks=1.
    • min.insync.replicas устанавливает порог: если размер ISR меньше
      этого значения, запись отклоняется с ошибкой NotEnoughReplicasException.

  3.3. ПРИМЕР
    ┌─────────────────────────────────────────────────────────────────────────┐
    │  Топик: orders                                                          │
    │  replication.factor = 3                                                 │
    │  min.insync.replicas = 2                                                │
    │  acks = all                                                             │
    ├─────────────────────────────────────────────────────────────────────────┤
    │  Ситуация 1: 3 брокера работают                                         │
    │    ISR = [broker1, broker2, broker3]                                    │
    │    Размер ISR = 3 ≥ 2 → запись разрешена                                │
    │    Продюсер ждёт подтверждения от всех 3 реплик                         │
    ├─────────────────────────────────────────────────────────────────────────┤
    │  Ситуация 2: 1 брокер упал (broker3)                                    │
    │    ISR = [broker1, broker2]                                             │
    │    Размер ISR = 2 ≥ 2 → запись разрешена                                │
    │    Продюсер ждёт подтверждения от broker1 и broker2                     │
    ├─────────────────────────────────────────────────────────────────────────┤
    │  Ситуация 3: 2 брокера упали (broker2, broker3)                         │
    │    ISR = [broker1]                                                      │
    │    Размер ISR = 1 < 2 → запись ОТКЛОНЕНА!                               │
    │    Продюсер получает NotEnoughReplicasException                         │
    └─────────────────────────────────────────────────────────────────────────┘

  4.  ВЛИЯНИЕ НА НАДЁЖНОСТЬ (DURABILITY)

  4.1. НАДЁЖНОСТЬ ПРИ РАЗНЫХ ЗНАЧЕНИЯХ MIN.INSYNC.REPLICAS

    ┌─────────────────────────────────────────────────────────────────────────┐
    │  min.insync.replicas = 1 (по умолчанию)                                 │
    │                                                                         │
    │  → Достаточно ОДНОЙ реплики (лидера) в ISR                              │
    │  → acks=all фактически = acks=1, если ISR = 1                           │
    │  → При сбое лидера до репликации → данные теряются                      │
    │  → Скрытая уязвимость: "безопасность" только на бумаге                  │
    └─────────────────────────────────────────────────────────────────────────┘

    ┌─────────────────────────────────────────────────────────────────────────┐
    │  min.insync.replicas = 2 (РЕКОМЕНДУЕТСЯ)                                │
    │                                                                         │
    │  → Нужно минимум 2 реплики в ISR                                        │
    │  → Можно потерять 1 брокер без потери данных                            │
    │  → Рекомендуется для production                                         │
    │  → Требует replication.factor ≥ 3                                       │
    └─────────────────────────────────────────────────────────────────────────┘

    ┌─────────────────────────────────────────────────────────────────────────┐
    │  min.insync.replicas = 3 (максимальная надёжность)                      │
    │                                                                         │
    │  → Нужно 3 реплики в ISR                                                │
    │  → Можно потерять 2 брокера без потери данных                           │
    │  → Но доступность снижается (при сбое 2 брокеров запись невозможна)     │
    │  → Требует replication.factor ≥ 4                                       │
    └─────────────────────────────────────────────────────────────────────────┘

  5.  ВЛИЯНИЕ НА ДОСТУПНОСТЬ (AVAILABILITY)
  min.insync.replicas — это не только защита от потери данных, но и
  потенциальное ограничение доступности.

  5.1. КОМПРОМИСС
    • Чем выше min.insync.replicas, тем надёжнее хранение данных.
    • Чем выше min.insync.replicas, тем ниже доступность (availability).
    • При сбое части брокеров запись может стать невозможной.

  5.2. ФОРМУЛА ДОСТУПНОСТИ
    Доступность = min.insync.replicas / replication.factor
    Пример:
      • replication.factor = 3
      • min.insync.replicas = 2 → доступность = 2/3 = 66% (можно потерять 1 брокера)
      • min.insync.replicas = 1 → доступность = 1/3 = 33% (можно потерять 2 брокеров)
      • min.insync.replicas = 3 → доступность = 3/3 = 100% (нельзя терять брокеров)

  5.3. ПРАВИЛО ВЫБОРА
    replication.factor >= min.insync.replicas + 1
    Это гарантирует, что при потере одного брокера система останется
    доступной для записи.

  6.  СВЯЗЬ С ACKS=ALL И REPLICATION.FACTOR

  6.1. ACKS=ALL
    • Продюсер ждёт подтверждения от ВСЕХ реплик в текущем ISR.
    • Без min.insync.replicas это может быть просто подтверждение от лидера.
    • С min.insync.replicas > 1 это даёт реальную защиту.

  6.2. REPLICATION.FACTOR (RF)
    • RF — общее количество реплик для партиции.
    • RF должно быть >= min.insync.replicas.
    • Рекомендуется RF = 3, min.insync.replicas = 2.

  6.3. НАСТРОЙКИ ДЛЯ PRODUCTION
    replication.factor = 3
    min.insync.replicas = 2
    acks = all
    Это "золотой стандарт" Confluent.

  7.  ПОДВОДНЫЕ КАМНИ И ЧАСТЫЕ ОШИБКИ

  7.1. "У МЕНЯ REPLICATION.FACTOR=3, ЗНАЧИТ ДАННЫЕ НЕ ПОТЕРЯЮТСЯ"
    НЕПРАВДА! Если min.insync.replicas = 1, то при падении 2 брокеров
    запись всё равно будет accepted, но данные будут только на 1 брокере.
    Потеря данных гарантирована при падении этого последнего брокера.
    Решение: всегда устанавливать min.insync.replicas >= 2.

  7.2. "Я УСТАНОВИЛ MIN.INSYNC.REPLICAS=2, ЗНАЧИТ МОЖНО ТЕРЯТЬ 2 БРОКЕРА"
    НЕПРАВДА! При RF=3 и min.insync.replicas=2, можно потерять ТОЛЬКО
    1 брокера. При потере 2 брокеров ISR = 1 < 2, и запись становится
    невозможной (NotEnoughReplicas).

  7.3. "ISR ВСЕГДА РАВЕН REPLICATION.FACTOR"
    НЕТ! ISR динамический. При сбоях или задержках реплики могут
    выходить из ISR. Это нормально, но требует мониторинга.

  7.4. "NOTENOUGHREPLICAS — ЭТО ОШИБКА, КОТОРАЯ НЕ ДОЛЖНА ПРОИСХОДИТЬ"
    На самом деле, это защитный механизм. Если вы получаете эту ошибку,
    значит вы достигли границы доступности. Это сигнал к тому, чтобы
    проверить состояние кластера или увеличить RF.

  8.  НАСТРОЙКА ДЛЯ PRODUCTION (РЕКОМЕНДАЦИИ CONFLUENT)

  8.1. РЕКОМЕНДУЕМАЯ КОНФИГУРАЦИЯ
    replication.factor = 3
    min.insync.replicas = 2
    acks = all
    default.replication.factor = 3 (для всех новых топиков)
    unclean.leader.election.enable = false (защита от выбора не-ISR лидера)

  8.2. ДЛЯ КРИТИЧНЫХ ДАННЫХ (ФИНАНСЫ, ПЛАТЕЖИ)
    replication.factor = 5
    min.insync.replicas = 3
    acks = all

  8.3. ДЛЯ НЕКРИТИЧНЫХ ДАННЫХ (ЛОГИ, МЕТРИКИ)
    replication.factor = 2
    min.insync.replicas = 1
    acks = all (или 1)

  9.  МОНИТОРИНГ ISR

  9.1. КЛЮЧЕВЫЕ МЕТРИКИ
    • Under-replicated partitions (URP) — партиции, у которых количество
      реплик меньше RF (т.е. не все реплики в ISR).
    • ISR shrink rate — частота уменьшения ISR (проблемы с репликацией).
    • ISR expand rate — частота увеличения ISR (восстановление реплик).
    • Replica lag — отставание реплик от лидера.

  9.2. ИНСТРУМЕНТЫ
    • kafka-topics.sh --describe --topic my-topic — показывает ISR для каждой партиции.
    • kafka-replica-verification.sh — проверка репликации.
    • Prometheus + JMX exporter — мониторинг метрик.
    • Burrow — мониторинг consumer lag, но не ISR.

  9.3. ПРЕДУПРЕЖДАЮЩИЕ СИГНАЛЫ
    • Увеличение under-replicated partitions.
    • Частые ISR shrink (реплики постоянно выпадают и возвращаются).
    • Ошибки NotEnoughReplicas в логах продюсеров.

  10. СВОДНАЯ ТАБЛИЦА КОНФИГУРАЦИЙ
  ┌────────────────────────┬──────────────────┬──────────────────┬──────────────────┐
  │ Параметр               │ Некритичные      │ Стандартные      │ Критичные        │
  │                        │ данные           │ данные           │ данные           │
  ├────────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ replication.factor     │ 2                │ 3                │ 5                │
  ├────────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ min.insync.replicas    │ 1                │ 2                │ 3                │
  ├────────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ acks                   │ 1                │ all              │ all              │
  ├────────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Теряем брокеров (no    │ 1                │ 0                │ 0                │
  │ data loss)             │                  │                  │                  │
  ├────────────────────────┼──────────────────┼──────────────────┼──────────────────┤
  │ Теряем брокеров (still │ 0                │ 1                │ 2                │
  │ available for writes)  │                  │                  │                  │
  └────────────────────────┴──────────────────┴──────────────────┴──────────────────┘

  11. КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  ISR (In-Sync Replicas) — реплики, синхронизированные с лидером.
      Только они могут стать новым лидером.
  2.  min.insync.replicas — минимальное количество реплик в ISR для
      подтверждения записи при acks=all.
  3.  acks=all + min.insync.replicas=2 + replication.factor=3 — золотой
      стандарт для production.
  4.  Без min.insync.replicas > 1, acks=all не даёт реальной защиты
      от потери данных.
  5.  min.insync.replicas защищает от "скрытой" потери данных, когда
      ISR состоит только из лидера.
  6.  Компромисс: увеличение min.insync.replicas повышает надёжность,
      но снижает доступность.
  7.  При недостаточном количестве реплик в ISR запись отклоняется
      с ошибкой NotEnoughReplicas.
  8.  always следить за under-replicated partitions.
  9.  Настройки: replication.factor, min.insync.replicas, acks —
      должны быть согласованы.
  10. Для критичных данных используйте RF=5, min.insync=3.
*/

func strPtr(s string) *string {
	return &s
}

//КОНФИГУРАЦИЯ

var (
	mode       = flag.String("mode", "producer", "Режим: create-topic, producer, check-isr")
	broker     = flag.String("broker", "localhost:9092", "Адрес брокера")
	topic      = flag.String("topic", "isr-test", "Топик")
	rf         = flag.Int("rf", 3, "Replication Factor")
	minISR     = flag.Int("min-isr", 2, "min.insync.replicas")
	partitions = flag.Int("partitions", 3, "Количество партиций")
)

//ADMIN CLIENT (УПРАВЛЕНИЕ ТОПИКАМИ И КОНФИГАМИ)

// createTopicWithConfig создаёт топик с заданными replication.factor
// и min.insync.replicas.
func createTopicWithConfig() error {
	log.Printf("Создание топика: %s, rf=%d, min.isr=%d", *topic, *rf, *minISR)

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	admin, err := sarama.NewClusterAdmin([]string{*broker}, config)
	if err != nil {
		return fmt.Errorf("ошибка создания admin client: %w", err)
	}
	defer admin.Close()

	// Проверяем, существует ли топик
	topics, err := admin.ListTopics()
	if err != nil {
		return fmt.Errorf("ошибка получения списка топиков: %w", err)
	}
	if _, exists := topics[*topic]; exists {
		log.Printf("ℹТопик %s уже существует, удаляем...", *topic)
		if err := admin.DeleteTopic(*topic); err != nil {
			return fmt.Errorf("ошибка удаления топика: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Создаём топик с конфигом
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     int32(*partitions),
		ReplicationFactor: int16(*rf),
		ConfigEntries: map[string]*string{
			"min.insync.replicas":            strPtr(fmt.Sprintf("%d", *minISR)),
			"unclean.leader.election.enable": strPtr("false"),
		},
	}

	err = admin.CreateTopic(*topic, topicDetail, false)
	if err != nil {
		return fmt.Errorf("ошибка создания топика: %w", err)
	}
	log.Printf("Топик %s создан", *topic)

	// используем DescribeConfigs с правильными аргументами
	configResources := []*sarama.ConfigResource{
		{
			Type: sarama.TopicResource,
			Name: *topic,
		},
	}
	configResults, err := admin.DescribeConfigs(configResources, sarama.DescribeConfigsOptions{})
	if err != nil {
		return fmt.Errorf("ошибка получения конфига: %w", err)
	}
	for _, res := range configResults {
		if res.ErrorMsg != "" {
			return fmt.Errorf("ошибка в результате DescribeConfigs: %s", res.ErrorMsg)
		}
	}
	return nil
}

// PRODUCER
func runProducer() error {
	log.Printf("Запуск продюсера с acks=all, топик: %s", *topic)

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	// КРИТИЧЕСКИЕ НАСТРОЙКИ ДЛЯ ДЕМОНСТРАЦИИ
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Idempotent = true
	config.Producer.Retry.Max = 3
	config.Producer.Retry.Backoff = 1 * time.Second
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	producer, err := sarama.NewSyncProducer([]string{*broker}, config)
	if err != nil {
		return fmt.Errorf("ошибка создания продюсера: %w", err)
	}
	defer producer.Close()

	log.Println("Отправка 10 сообщений...")

	for i := 0; i < 10; i++ {
		msg := &sarama.ProducerMessage{
			Topic: *topic,
			Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
			Value: sarama.StringEncoder(fmt.Sprintf("msg-%d", i)),
		}

		_, _, err := producer.SendMessage(msg)
		if err != nil {
			if strings.Contains(err.Error(), "NotEnoughReplicas") {
				log.Printf("NotEnoughReplicas: ISR меньше min.insync.replicas (сообщение %d)", i)
				log.Println("Это защитный механизм! Данные не записаны.")
			} else {
				log.Printf("Ошибка отправки (сообщение %d): %v", i, err)
			}
			continue
		}
		log.Printf("Сообщение %d отправлено", i)
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("Продюсер завершён")
	return nil
}

// CHECK ISR
func checkISR() error {
	log.Printf("Проверка ISR для топика: %s", *topic)

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	admin, err := sarama.NewClusterAdmin([]string{*broker}, config)
	if err != nil {
		return fmt.Errorf("ошибка создания admin client: %w", err)
	}
	defer admin.Close()

	// Получаем метаданные топика
	metadata, err := admin.DescribeTopics([]string{*topic})
	if err != nil {
		return fmt.Errorf("ошибка получения метаданных: %w", err)
	}
	if len(metadata) == 0 {
		return fmt.Errorf("топик %s не найден", *topic)
	}

	topicMeta := metadata[0]
	log.Printf("Топик: %s", topicMeta.Name)
	log.Printf("Партиций: %d", len(topicMeta.Partitions))

	for _, p := range topicMeta.Partitions {
		isr := p.Isr
		replicas := p.Replicas
		leader := p.Leader

		log.Printf("Партиция %d:", p.ID)
		log.Printf("Лидер: %d", leader)
		log.Printf("Реплики: %v", replicas)
		log.Printf("ISR: %v", isr)
		log.Printf("Размер ISR: %d", len(isr))

		if len(isr) < len(replicas) {
			log.Printf("Не все реплики в ISR (under-replicated)")
		}
		if len(isr) < *minISR {
			log.Printf("ISR меньше min.insync.replicas (%d < %d)", len(isr), *minISR)
			log.Printf("Записи с acks=all будут отклоняться!")
		}
	}
	return nil
}

func main() {
	flag.Parse()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		log.Println("⏳ Получен сигнал завершения...")
		cancel()
	}()

	var err error
	switch *mode {
	case "create-topic":
		err = createTopicWithConfig()
	case "producer":
		err = runProducer()
	case "check-isr":
		err = checkISR()
	default:
		err = fmt.Errorf("неизвестный режим: %s", *mode)
	}

	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
