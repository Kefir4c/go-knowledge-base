package monitoringmetrics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
  БЛОК 4.4: MONITORING AND METRICS
  Мониторинг Kafka — это не просто "посмотреть графики". Это фундаментальная
  практика, которая позволяет обнаруживать проблемы до того, как они затронут
  пользователей, и предотвращать катастрофические сбои в продакшене.

  В этой теории мы разберём:
    1. Consumer Lag — главный показатель здоровья консюмера
    2. Метрики кластера: Under-replicated partitions, Offline partitions
    3. ISR shrink/expand — индикаторы проблем с репликацией
    4. Метрики брокеров: CPU, память, сеть, диски
    5. Инструменты мониторинга: Kafka Manager, Burrow, Prometheus + JMX exporter
    6. Настройка алертов и best practices
    7. Расширенные темы: Distributed Tracing, логирование
    8. Ключевые выводы для собеседования

  1.  CONSUMER LAG — ГЛАВНЫЙ ПОКАЗАТЕЛЬ ЗДОРОВЬЯ КОНСЮМЕРА
  Consumer Lag (задержка потребителя) — это разница между последним
  сообщением, записанным в партицию (Log End Offset), и последним
  сообщением, которое консюмер успешно обработал и закоммитил
  (Current Committed Offset).

  1.1. ФОРМУЛА
    Lag = Log End Offset - Current Committed Offset

  1.2. ЧТО ОЗНАЧАЕТ LAG
    • Lag = 0 — консюмер полностью синхронизирован, всё ок.
    • Lag > 0 — консюмер отстаёт от продюсера.
    • Lag растёт — консюмер не успевает обрабатывать сообщения.
    • Lag падает — консюмер догоняет (возможно, нагрузка снизилась).

  1.3. ПОЧЕМУ LAG — ЭТО ВАЖНО
    Consumer lag — это первая линия обороны. Если вы не мониторите lag,
    вы узнаете о проблеме только когда пользователи начнут жаловаться.

    Lag показывает:
      • Успевает ли консюмер обрабатывать поступающие сообщения.
      • Есть ли проблемы с производительностью консюмера.
      • Нужно ли масштабировать консюмеры (добавлять партиции или консюмеров).
      • Не упал ли консюмер (lag растёт бесконечно).

  1.4. КАК ИЗМЕРЯЕТСЯ LAG
    В Kafka lag измеряется через JMX-метрики, которые экспортируются
    консюмерами и брокерами.

    Основные метрики:
      • records-lag-max — максимальный lag среди всех партиций для консюмера.
        (MBean: kafka.consumer:type=consumer-fetch-manager-metrics,client-id=*)
      • records-lag — lag для каждой конкретной партиции.

    В Prometheus (через kafka_exporter) используется метрика:
      • kafka_consumergroup_lag — lag для каждой партиции в группе.

  1.5. КАК НАСТРАИВАТЬ АЛЕРТЫ ПО LAG
    Это частый вопрос на собеседовании. Простое правило "алерт при lag > X"
    работает плохо, потому что lag зависит от пропускной способности.

    ЛУЧШАЯ ПРАКТИКА:
      • Используйте не абсолютное значение, а скорость изменения (velocity).
      • Алерт на "lag растёт быстрее, чем консюмер успевает обрабатывать".
      • Учитывайте retention: если lag превышает retention, сообщения будут
        потеряны — это критический алерт.

    ПРАКТИЧЕСКИЙ СОВЕТ:
      • Настройте несколько уровней алертов:
        - WARNING: lag > 100 000 сообщений (или 10 минут задержки).
        - CRITICAL: lag > 1 000 000 сообщений (или 1 час задержки).
        - При retention=7 дней: lag > retention — CRITICAL.
      • Всегда смотрите на тренд, а не на абсолютное значение.

  1.6. КАК УМЕНЬШИТЬ LAG
    • Увеличить количество партиций (если консюмеров больше, чем партиций).
    • Увеличить количество консюмеров в группе.
    • Оптимизировать обработку (снизить время на обработку сообщения).
    • Увеличить fetch.max.bytes и max.poll.records.
    • Увеличить max.poll.interval.ms (если обработка долгая).

  2.  МЕТРИКИ КЛАСТЕРА — ЗДОРОВЬЕ БРОКЕРОВ И ТОПИКОВ

  2.1. UNDER-REPLICATED PARTITIONS (URP)
    Это количество партиций, у которых количество реплик в ISR меньше
    чем replication.factor.

    • В здоровом кластере URP должно быть 0.
    • URP > 0 означает, что одна или несколько реплик отстают от лидера.
    • Если URP > 0 длительное время — данные находятся под угрозой потери.

    ПРИЧИНЫ URP > 0:
      • Брокер упал или недоступен.
      • Сетевые проблемы между брокерами.
      • Один брокер перегружен и не успевает реплицировать.
      • Диск на брокере заполнен.

    КАК ПРОВЕРИТЬ URP:
      • Команда: kafka-topics.sh --describe --under-replicated-partitions
      • JMX метрика: kafka.server:type=ReplicaManager,name=UnderReplicatedPartitions

  2.2. OFFLINE PARTITIONS COUNT
    • Партиции, у которых нет доступного лидера.
    • Это критическая ситуация — чтение и запись в такие партиции невозможны.
    • Должно быть 0 всегда.

  2.3. ACTIVE CONTROLLER COUNT
    • В кластере всегда должен быть ровно один активный контроллер.
    • Если 0 — кластер не работает.
    • Если > 1 — проблема с выбором контроллера (split brain).
    • Должно быть 1 всегда.

  2.4. UNCLEAN LEADER ELECTIONS
    • Выбор лидера из реплики, которая НЕ входит в ISR.
    • Это означает, что данные могут быть потеряны при выборах.
    • Должно быть 0 в здоровом кластере.

  2.5. REQUEST HANDLER IDLE PERCENT
    • Показывает, насколько заняты обработчики запросов.
    • Низкое значение (< 10%) означает, что брокер перегружен.
    • Высокое значение (> 50%) означает, что брокер простаивает.
    • Должно быть около 30-50% в нормальном режиме.

  3.  ISR SHRINK / EXPAND — ИНДИКАТОРЫ ПРОБЛЕМ С РЕПЛИКАЦИЕЙ

  3.1. ЧТО ЭТО
    • ISR Shrink (IsrShrinksPerSec) — скорость, с которой реплики покидают ISR.
    • ISR Expand (IsrExpandsPerSec) — скорость, с которой реплики возвращаются в ISR.

  3.2. ПОЧЕМУ ЭТО ВАЖНО
    • В здоровом кластере shrink и expand должны быть близки к 0.
    • Если shrink > 0 без соответствующего expand — это проблема.
    • Частые shrink/expand (flapping) — признак нестабильности:
      - Сетевые проблемы между брокерами.
      - Брокер постоянно падает и восстанавливается.
      - Перегрузка брокера, он не успевает реплицировать.

  3.3. МЕТРИКИ
    • IsrShrinksPerSec — kafka.server:type=ReplicaManager,name=IsrShrinksPerSec
    • IsrExpandsPerSec — kafka.server:type=ReplicaManager,name=IsrExpandsPerSec

  4.  МЕТРИКИ БРОКЕРОВ (ИНФРАСТРУКТУРНЫЕ)
  Помимо Kafka-специфичных метрик, нужно мониторить инфраструктуру брокеров:

    • CPU usage — перегрузка CPU замедляет обработку запросов.
      - Норма: < 70% в пиковые часы.
      - Алерт: > 80% в течение 5 минут.

    • Memory usage — утечки памяти или недостаток памяти для Page Cache.
      - Норма: < 80% от выделенной памяти.
      - Алерт: > 90% в течение 5 минут.

    • Disk usage — заполнение диска приводит к остановке брокера.
      - Норма: < 80% от общего объёма.
      - Алерт: > 85% (предупреждение), > 90% (критический).

    • Disk I/O — высокая задержка диска замедляет запись и чтение.
      - Норма: < 50 мс для записи, < 20 мс для чтения.
      - Алерт: > 100 мс для записи, > 50 мс для чтения.

    • Network I/O — перегрузка сети приводит к задержкам репликации.
      - Норма: < 80% от пропускной способности.
      - Алерт: > 90% в течение 5 минут.

    • JVM GC — частые или долгие GC приводят к паузам в работе брокера.
      - Норма: GC паузы < 100 мс.
      - Алерт: GC паузы > 1 секунды.

  5.  ИНСТРУМЕНТЫ МОНИТОРИНГА

  5.1. KAFKA MANAGER (CMAK — CLUSTER MANAGER FOR APACHE KAFKA)
    • Веб-интерфейс для управления и мониторинга кластеров Kafka.
    • Позволяет просматривать топики, партиции, консюмер-группы.
    • Отображает информацию о брокерах и их состоянии.
    • Поддерживает несколько кластеров в одном интерфейсе.
    • Можно управлять топиками (создание, удаление, изменение конфигурации).
    • Показывает потребительские группы и их смещения.

    Плюсы:
      + Простой веб-интерфейс.
      + Подходит для быстрой диагностики.
      + Не требует установки дополнительных агентов.

    Минусы:
      - Не подходит для автоматического мониторинга и алертинга.
      - Ограниченные возможности визуализации трендов.

  5.2. BURROW (ОТ LINKEDIN)
    • Специализированный инструмент для мониторинга consumer lag.
    • Автоматически обнаруживает все consumer groups в кластере.
    • Вычисляет lag на основе трендов, а не абсолютных значений.
    • Предоставляет HTTP API для интеграции с системами мониторинга.

    КЛЮЧЕВЫЕ ОСОБЕННОСТИ:
      • Не использует JMX — читает данные напрямую из Kafka.
      • Автоматически определяет "отстающие" группы.
      • Позволяет настроить окна оценки (evaluation windows).
      • Поддерживает фильтрацию уведомлений по приоритету групп.

    ПЛЮСЫ:
      + Точно определяет, есть ли проблема с lag.
      + Учитывает тренды, а не абсолютные значения.
      + Не требует сложной настройки.

    МИНУСЫ:
      - Только lag, не показывает другие метрики.
      - Требует отдельной установки и настройки.

  5.3. PROMETHEUS + JMX EXPORTER + GRAFANA (ЗОЛОТОЙ СТАНДАРТ)
    Это наиболее распространённый и мощный подход к мониторингу Kafka
    в современных инфраструктурах.

    АРХИТЕКТУРА:
      ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
      │  Kafka Broker   │───▶│   JMX Exporter  │────▶│   Prometheus    │
      │  (JMX Metrics)  │     │  (HTTP /metrics)│     │   (Storage)     │
      └─────────────────┘     └─────────────────┘     └─────────────────┘
                                                               │
                                                               ▼
                                                        ┌─────────────────┐
                                                        │    Grafana      │
                                                        │  (Dashboards)   │
                                                        └─────────────────┘

    КОМПОНЕНТЫ:
      • JMX Exporter — экспортирует JMX-метрики Kafka в формате Prometheus.
      • Prometheus — собирает и хранит метрики как временные ряды.
      • Grafana — визуализирует метрики в виде дашбордов.

    КАКИЕ МЕТРИКИ СОБИРАЕТ:
      • Consumer lag для всех групп.
      • Under-replicated partitions.
      • ISR shrink/expand.
      • Производительность брокеров (запросы/сек, байты/сек).
      • JVM-метрики (GC, память).
      • Сетевые метрики.

    ПЛЮСЫ:
      + Полное покрытие всех метрик.
      + Исторические данные (можно анализировать тренды).
      + Гибкие дашборды и алерты.
      + Интеграция с другими системами (Kubernetes, облака).

    МИНУСЫ:
      - Требует настройки (JMX Exporter, Prometheus, Grafana).
      - Больше инфраструктуры для поддержки.

  5.4. KAFKA EXPORTER (ОТ PROMETHEUS)
    • Специальный экспортер для Kafka, который читает метрики напрямую
      из брокеров и consumer groups.
    • Не требует JMX.
    • Отлично работает с Prometheus.

    ПЛЮСЫ:
      + Простая настройка.
      + Хорошо интегрируется с Prometheus.
      + Собирает основные метрики (lag, URP, метрики брокеров).

    МИНУСЫ:
      - Не все метрики доступны (требуется JMX для глубоких метрик).

  5.5. ДРУГИЕ ИНСТРУМЕНТЫ
    • Confluent Control Center — коммерческий инструмент с продвинутыми
      возможностями мониторинга и управления.
    • Kafka Eagle — веб-интерфейс для мониторинга и управления.
    • KMinion — лёгкий инструмент для мониторинга lag от Confluent.

  6.  НАСТРОЙКА АЛЕРТОВ И BEST PRACTICES

  6.1. ЧТО МОНИТОРИТЬ (МИНИМАЛЬНЫЙ НАБОР)
    По рекомендации Confluent, обязательно мониторить:
      • ActiveControllerCount — должно быть = 1.
      • OfflinePartitionsCount — должно быть = 0.
      • UnderReplicatedPartitions — должно быть = 0.
      • UncleanLeaderElectionsPerSec — должно быть = 0.
      • Consumer Lag — алерты по тренду.

  6.2. КАК НАСТРАИВАТЬ АЛЕРТЫ
    • UnderReplicatedPartitions > 0 в течение 5 минут → CRITICAL.
    • OfflinePartitionsCount > 0 → CRITICAL (немедленно).
    • Consumer Lag: не по абсолютному значению, а по скорости роста.
    • ISR shrink/expand: если shrink > 0 без expand → WARNING.
    • Брокер недоступен → CRITICAL.

  6.3. BEST PRACTICES
    • Мониторьте тренды, а не абсолютные значения.
    • Используйте несколько уровней алертов (WARNING, CRITICAL).
    • Настраивайте время на устранение (cool-down) чтобы избежать спама.
    • Логируйте все алерты для пост-мортем анализа.
    • Интегрируйте мониторинг с системой оповещений (PagerDuty, Slack).
    • Регулярно проверяйте дашборды на актуальность.

  6.4. ПРИМЕР ПРАВИЛ ДЛЯ PROMETHEUS (ALERTMANAGER)
    groups:
      - name: kafka_alerts
        rules:
          - alert: KafkaUnderReplicatedPartitions
            expr: kafka_server_ReplicaManager_UnderReplicatedPartitions > 0
            for: 5m
            annotations:
              summary: "Under-replicated partitions in Kafka cluster"

          - alert: KafkaConsumerGroupLag
            expr: kafka_consumergroup_lag > 100000
            for: 5m
            annotations:
              summary: "Consumer group lag is high"

          - alert: KafkaOfflinePartitions
            expr: kafka_server_ReplicaManager_OfflinePartitionsCount > 0
            for: 0m
            annotations:
              summary: "Offline partitions in Kafka cluster"

  7.  РАСШИРЕННЫЕ ТЕМЫ: DISTRIBUTED TRACING И ЛОГИРОВАНИЕ

  7.1. DISTRIBUTED TRACING (ОТ SLA JEAGER/OPENTELEMETRY)

    Для распределённых систем важно отслеживать запросы через все сервисы.
    OpenTelemetry позволяет передавать trace_id через Kafka.
    КАК ЭТО РАБОТАЕТ:
      • Продюсер добавляет trace_id в заголовки сообщения.
      • Консюмер извлекает trace_id и продолжает трассировку.
      • Это позволяет увидеть полный путь запроса.

    ПРИМЕР В GO:
      // Продюсер
      headers = append(headers, kafka.Header{
          Key:   "trace_id",
          Value: []byte(traceID),
      })

      // Консюмер
      traceID = extractTraceID(msg.Headers)

  7.2. ЛОГИРОВАНИЕ
    Логирование в Kafka должно быть структурированным (JSON) и включать:
      • trace_id — для связи с distributed tracing.
      • consumer_group — для идентификации группы.
      • topic, partition, offset — для локализации сообщения.
      • latency — для отслеживания задержек.
      • error — для быстрого обнаружения проблем.

    ПРИМЕР СТРУКТУРИРОВАННОГО ЛОГА:
      {
        "level": "info",
        "time": "2024-01-01T12:00:00Z",
        "msg": "Message processed",
        "trace_id": "abc-123",
        "consumer_group": "order-processor",
        "topic": "orders",
        "partition": 0,
        "offset": 42,
        "latency_ms": 150
      }

  8.  ЧАСТЫЕ ОШИБКИ
    • Алерт на абсолютное значение lag без учёта пропускной способности.
    • Игнорирование under-replicated partitions (URP > 0).
    • Нет мониторинга OfflinePartitionsCount.
    • Не мониторятся ISR shrink/expand (пропускаются проблемы с репликацией).
    • Алерты слишком чувствительны → усталость от оповещений.
    • Нет исторических данных → невозможно анализировать тренды.
    • Не логируются ошибки консюмеров.
    • Нет мониторинга JVM GC.
    • Не мониторятся диски (заполнение диска → остановка брокера).

  9.  КЛЮЧЕВЫЕ ВЫВОДЫ

  1.  Consumer Lag — главный показатель здоровья консюмера.
      Lag = Log End Offset - Current Committed Offset.
  2.  Мониторить нужно не абсолютное значение lag, а его тренд и скорость  роста.
  3.  UnderReplicatedPartitions (URP) — количество партиций с отставшими
      репликами. Должно быть 0.
  4.  OfflinePartitionsCount — партиции без лидера. Должно быть 0.
  5.  ISR shrink/expand — индикаторы проблем с репликацией. Норма — близки к 0.
  6.  Kafka Manager (CMAK) — веб-интерфейс для управления и быстрой диагностики.
  7.  Burrow — специализированный инструмент для мониторинга consumer lag
      с анализом трендов.
  8.  Prometheus + JMX Exporter + Grafana — золотой стандарт для
      production-мониторинга.
  9.  Confluent рекомендует мониторить: ActiveControllerCount,
      OfflinePartitionsCount, UnderReplicatedPartitions,
      UncleanLeaderElectionsPerSec.
  10. Без мониторинга вы узнаёте о проблеме только когда пользователи
      начинают жаловаться.
  11. Всегда настраивайте алерты на основе трендов, а не абсолютных значений.
  12. Используйте структурированное логирование с trace_id для отладки.
  13. Мониторьте инфраструктуру брокеров: CPU, память, диски, сеть.
  14. GC паузы > 1 секунды — повод для беспокойства.
  15. Under-replicated partitions — это обычно проблема с сетью или диском.
*/

//КОНФИГУРАЦИЯ

var (
	broker        = flag.String("broker", "localhost:9092", "Адрес брокера")
	topic         = flag.String("topic", "monitoring-test", "Топик")
	groupID       = flag.String("group", "monitoring-group", "ID группы")
	metricsPort   = flag.String("metrics-port", "8080", "Порт для метрик")
	producerRate  = flag.Int("producer-rate", 100, "Сообщений в секунду (producer)")
	consumerDelay = flag.Int("consumer-delay", 50, "Задержка обработки в мс (для lag)")
)

//МЕТРИКИ PROMETHEUS

var (
	// Consumer Lag для каждой партиции
	consumerLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Consumer lag per partition",
		},
		[]string{"group", "topic", "partition"},
	)

	// Подсчёт отправленных сообщений
	messagesProduced = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_messages_produced_total",
			Help: "Total messages produced",
		},
	)

	// Подсчёт потреблённых сообщений
	messagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_consumed_total",
			Help: "Total messages consumed per consumer",
		},
		[]string{"consumer"},
	)

	// Время обработки сообщений (гистограмма)
	processingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_consumer_processing_time_seconds",
			Help:    "Consumer processing time",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"consumer"},
	)

	// Количество активных консюмеров
	activeConsumers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_group_members",
			Help: "Number of active consumers in group",
		},
	)

	// Метрики кластера (заглушки для примера)
	underReplicatedPartitions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kafka_under_replicated_partitions",
			Help: "Under-replicated partitions count",
		},
	)

	offlinePartitions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kafka_offline_partitions",
			Help: "Offline partitions count",
		},
	)
)

func initMetrics() {
	prometheus.MustRegister(consumerLag)
	prometheus.MustRegister(messagesProduced)
	prometheus.MustRegister(messagesConsumed)
	prometheus.MustRegister(processingTime)
	prometheus.MustRegister(activeConsumers)
	prometheus.MustRegister(underReplicatedPartitions)
	prometheus.MustRegister(offlinePartitions)

	// Инициализируем метрики кластера (в реальности они обновляются периодически)
	underReplicatedPartitions.Set(0)
	offlinePartitions.Set(0)
}

// PRODUCER
func runProducer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Idempotent = true

	producer, err := sarama.NewSyncProducer([]string{*broker}, config)
	if err != nil {
		log.Fatalf("Ошибка создания продюсера: %v", err)
	}
	defer producer.Close()

	log.Printf("Producer запущен, скорость: %d msg/s", *producerRate)

	ticker := time.NewTicker(time.Second / time.Duration(*producerRate))
	defer ticker.Stop()

	var counter int64

	for {
		select {
		case <-ctx.Done():
			log.Println("Producer остановлен")
			return
		case <-ticker.C:
			counter++
			key := fmt.Sprintf("key-%d", counter%10) // 10 разных ключей
			value := fmt.Sprintf(`{"id":%d,"time":"%s"}`, counter, time.Now().Format(time.RFC3339))

			msg := &sarama.ProducerMessage{
				Topic: *topic,
				Key:   sarama.StringEncoder(key),
				Value: sarama.StringEncoder(value),
			}

			if _, _, err := producer.SendMessage(msg); err != nil {
				log.Printf("Ошибка отправки: %v", err)
				continue
			}
			messagesProduced.Inc()

			if counter%1000 == 0 {
				log.Printf("Отправлено %d сообщений", counter)
			}
		}
	}
}

// CONSUMER
type ConsumerHandler struct {
	consumerID string
	delay      time.Duration
	processed  int64
}

func NewConsumerHandler(id string, delay time.Duration) *ConsumerHandler {
	return &ConsumerHandler{
		consumerID: id,
		delay:      delay,
	}
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	log.Printf("[%s] Setup: назначены партиции", h.consumerID)
	activeConsumers.Inc()
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	log.Printf("[%s] Cleanup", h.consumerID)
	activeConsumers.Dec()
	session.Commit()
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	log.Printf("[%s] Начало обработки партиции: %d", h.consumerID, claim.Partition())

	for msg := range claim.Messages() {
		start := time.Now()

		// Имитация обработки с задержкой
		time.Sleep(h.delay)

		// Обновляем метрики
		atomic.AddInt64(&h.processed, 1)
		messagesConsumed.WithLabelValues(h.consumerID).Inc()
		processingTime.WithLabelValues(h.consumerID).Observe(time.Since(start).Seconds())

		// Коммитим смещение
		session.MarkMessage(msg, "")

		// Обновляем consumer lag (для демонстрации)
		// В реальном проекте lag читается из Kafka, здесь мы имитируем
		if msg.Offset%100 == 0 {
			consumerLag.WithLabelValues(*groupID, *topic, strconv.Itoa(int(msg.Partition))).Set(float64(msg.Offset % 1000))
		}
	}
	return nil
}

func runConsumer(ctx context.Context, wg *sync.WaitGroup, id string, delay time.Duration) {
	defer wg.Done()

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = false // ручной коммит
	config.Consumer.Return.Errors = true
	config.Consumer.Group.InstanceId = id // Static Membership

	client, err := sarama.NewConsumerGroup([]string{*broker}, *groupID, config)
	if err != nil {
		log.Fatalf("[%s] Ошибка создания consumer group: %v", id, err)
	}
	defer client.Close()

	handler := NewConsumerHandler(id, delay)

	log.Printf("[%s] Consumer запущен, задержка: %v", id, delay)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Consumer остановлен", id)
			return
		default:
			if err := client.Consume(ctx, []string{*topic}, handler); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				log.Printf("[%s] Ошибка консюмера: %v", id, err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}

//METRICS UPDATER (ОБНОВЛЕНИЕ МЕТРИК КЛАСТЕРА)

func updateClusterMetrics(ctx context.Context) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	admin, err := sarama.NewClusterAdmin([]string{*broker}, config)
	if err != nil {
		log.Printf("Ошибка создания admin client: %v", err)
		return
	}
	defer admin.Close()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Получаем метаданные топика
			metadata, err := admin.DescribeTopics([]string{*topic})
			if err != nil {
				log.Printf("Ошибка получения метаданных: %v", err)
				continue
			}

			if len(metadata) == 0 {
				continue
			}

			var urp int
			var offline int

			for _, p := range metadata[0].Partitions {
				if len(p.Isr) < len(p.Replicas) {
					urp++
				}
				if p.Leader == -1 {
					offline++
				}
			}

			underReplicatedPartitions.Set(float64(urp))
			offlinePartitions.Set(float64(offline))

			if urp > 0 {
				log.Printf("Under-replicated partitions: %d", urp)
			}
			if offline > 0 {
				log.Printf("Offline partitions: %d", offline)
			}
		}
	}
}

//LAG UPDATER

func updateLagMetrics(ctx context.Context) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	admin, err := sarama.NewClusterAdmin([]string{*broker}, config)
	if err != nil {
		log.Printf("Ошибка создания admin client для lag: %v", err)
		return
	}
	defer admin.Close()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Получаем информацию о партициях
			metadata, err := admin.DescribeTopics([]string{*topic})
			if err != nil {
				continue
			}
			if len(metadata) == 0 {
				continue
			}

			for _, p := range metadata[0].Partitions {
				// В реальном проекте здесь нужно читать смещения консюмера
				// Для демонстрации используем имитацию
				lag := float64(time.Now().Unix() % 1000)
				consumerLag.WithLabelValues(*groupID, *topic, strconv.Itoa(int(p.ID))).Set(lag)
			}
		}
	}
}

//HTTP HANDLERS

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

//MAIN

func main() {
	flag.Parse()
	initMetrics()

	// Создаём топик, если его нет
	if err := createTopicIfNotExists(); err != nil {
		log.Fatalf("Ошибка создания топика: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Запускаем producer
	wg.Add(1)
	go runProducer(ctx, &wg)

	// Запускаем два consumer'а с разной скоростью
	wg.Add(1)
	go runConsumer(ctx, &wg, "slow-consumer", time.Duration(*consumerDelay)*time.Millisecond)

	wg.Add(1)
	go runConsumer(ctx, &wg, "fast-consumer", 5*time.Millisecond)

	// Запускаем обновление метрик кластера
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateClusterMetrics(ctx)
	}()

	// Запускаем обновление lag (для демонстрации)
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateLagMetrics(ctx)
	}()

	// Запускаем HTTP сервер для метрик
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", healthHandler)

	go func() {
		log.Printf("Metrics endpoint: http://localhost:%s/metrics", *metricsPort)
		log.Printf("Health: http://localhost:%s/health", *metricsPort)
		if err := http.ListenAndServe(":"+*metricsPort, nil); err != nil {
			log.Fatalf("HTTP сервер ошибка: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Получен сигнал завершения...")
	cancel()

	// Даём время на завершение
	time.Sleep(3 * time.Second)
	wg.Wait()

	log.Println("Программа завершена")
}

//ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

func createTopicIfNotExists() error {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	admin, err := sarama.NewClusterAdmin([]string{*broker}, config)
	if err != nil {
		return fmt.Errorf("ошибка создания admin client: %w", err)
	}
	defer admin.Close()

	topics, err := admin.ListTopics()
	if err != nil {
		return fmt.Errorf("ошибка получения списка топиков: %w", err)
	}
	if _, exists := topics[*topic]; exists {
		log.Printf("Топик %s уже существует", *topic)
		return nil
	}

	err = admin.CreateTopic(*topic, &sarama.TopicDetail{
		NumPartitions:     3,
		ReplicationFactor: 1,
	}, false)
	if err != nil {
		return fmt.Errorf("ошибка создания топика: %w", err)
	}
	log.Printf("Топик %s создан", *topic)
	return nil
}
