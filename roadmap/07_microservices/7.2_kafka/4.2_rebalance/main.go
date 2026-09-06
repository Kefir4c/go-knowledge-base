package rebalance

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
  БЛОК 4.2: CONSUMER REBALANCE
  Consumer Rebalance — это механизм перераспределения партиций между
  консюмерами в одной группе. Это критически важная часть Kafka,
  которая обеспечивает масштабируемость, отказоустойчивость и балансировку
  нагрузки.

  В этой теории мы разберём:
    1. Что такое Consumer Rebalance и когда он происходит
    2. Протокол ребалансировки: JoinGroup, SyncGroup, Heartbeat
    3. Типы ребалансировки: Eager vs Cooperative
    4. Стратегии распределения партиций (Range, RoundRobin, Sticky, Cooperative)
    5. Лидер группы — кто он и зачем нужен
    6. Static Membership — как уменьшить количество ребалансировок
    7. Жизненный цикл ребалансировки (под капотом)
    8. Влияние на производительность и как его минимизировать
    9. Ребалансировка и коммиты смещений — как не потерять данные
    10. Обработка ребалансировки в коде (ConsumerRebalanceListener)
    11. Проблемы: rebalance timeout, stuck rebalance, rebalance storm
    12. Настройка параметров для production (рекомендации)
    13. Частые проблемы и их решения
    14. Ключевые выводы для собеседования

  1.  ЧТО ТАКОЕ CONSUMER REBALANCE
  Consumer Rebalance — это процесс, в ходе которого партиции топика
  перераспределяются между консюмерами в одной consumer group.

  Когда происходит ребалансировка?
    • Добавление нового консюмера в группу.
    • Удаление консюмера из группы (по инициативе или из-за сбоя).
    • Изменение подписки на топики (добавление/удаление топиков).
    • Изменение количества партиций в топике.
    • Истечение сессии консюмера (session.timeout.ms).
    • Изменение стратегии распределения (если конфигурация изменилась).

  ЗАЧЕМ НУЖНА РЕБАЛАНСИРОВКА?
    • Равномерное распределение нагрузки между консюмерами.
    • Обеспечение отказоустойчивости (при падении консюмера его партиции
      переходят к другим).
    • Масштабируемость — можно добавлять консюмеры для увеличения
      пропускной способности.

  2.  ПРОТОКОЛ РЕБАЛАНСИРОВКИ: JOINGROUP, SYNCGROUP, HEARTBEAT
  Ребалансировка основана на трёх основных RPC-запросах:

  2.1. HEARTBEAT (СЕРДЦЕБИЕНИЕ)
    • Консюмер периодически отправляет heartbeat координатору группы.
    • Если координатор не получает heartbeat в течение session.timeout.ms,
      консюмер считается мёртвым и исключается из группы.
    • Частота heartbeat: heartbeat.interval.ms (обычно 1/3 от session.timeout).
    • Heartbeat также используется для обнаружения изменений в группе
      (например, когда нужно начать ребалансировку).

  2.2. JOINGROUP (ПРИСОЕДИНЕНИЕ К ГРУППЕ)
    • Когда консюмер хочет присоединиться к группе (при старте или после
      выхода), он отправляет JoinGroup запрос координатору.
    • Координатор собирает все JoinGroup запросы от консюмеров в группе.
    • Выбирается лидер группы (первый консюмер, приславший JoinGroup).
    • Лидер получает список всех консюмеров и их подписки.

  2.3. SYNCGROUP (СИНХРОНИЗАЦИЯ НАЗНАЧЕНИЙ)
    • После выбора лидера, лидер вычисляет новый план распределения
      партиций (на основе стратегии).
    • Лидер отправляет SyncGroup запрос с планом.
    • Координатор рассылает план всем консюмерам.
    • Консюмеры получают свои назначения и начинают чтение.

  ВАЖНО: Вся ребалансировка синхронизируется через координатора группы,
  который хранит состояние группы и смещения.

  3.  EAGER VS COOPERATIVE REBALANCE — КЛЮЧЕВОЕ ОТЛИЧИЕ

  3.1. EAGER REBALANCE (ТРАДИЦИОННЫЙ ПОДХОД)
    Это классический механизм ребалансировки, который использовался
    в Kafka до версии 2.4.

    КАК ЭТО РАБОТАЕТ:
      1. Все консюмеры в группе останавливают чтение (stop-the-world).
      2. Каждый консюмер отзывает свои партиции (Revoke).
      3. Происходит перераспределение всех партиций заново.
      4. Консюмеры получают новые назначения (Assign).
      5. Чтение возобновляется.

    ПРОБЛЕМЫ:
      • Stop-the-world: все консюмеры перестают обрабатывать сообщения
        на время ребалансировки.
      • При большом количестве партиций и консюмеров ребалансировка
        может занимать десятки секунд.
      • Частые ребалансировки приводят к "кушению" (thrashing) —
        консюмеры постоянно перераспределяются.

  3.2. COOPERATIVE REBALANCE (INCREMENTAL COOPERATIVE REBALANCING)
    Появился в Kafka 2.4 (KIP-429) и стал стандартом в современных версиях.

    КЛЮЧЕВОЕ ОТЛИЧИЕ:
      • Не все партиции перераспределяются, а только те, которые реально
        меняют владельца.
      • Консюмеры, которые не теряют партиции, продолжают читать.

    КАК ЭТО РАБОТАЕТ:
      1. Координатор определяет, какие партиции нужно переместить.
      2. Только консюмеры, отдающие или получающие партиции, участвуют
         в ребалансировке.
      3. Остальные консюмеры продолжают читать без остановки.

    ПРЕИМУЩЕСТВА:
      • Минимальное время простоя.
      • Лучшая производительность при частых ребалансировках.
      • Меньше накладных расходов на координацию.

  4.  СТРАТЕГИИ РАСПРЕДЕЛЕНИЯ ПАРТИЦИЙ (PARTITION ASSIGNMENT STRATEGIES)
  4.1. RANGE (ПО УМОЛЧАНИЮ В СТАРЫХ ВЕРСИЯХ)
    Как работает:
      • Партиции делятся на диапазоны для каждого консюмера.
      • Для каждого топика отдельно.

    Пример (топик с 6 партициями, 2 консюмера):
      • Консюмер 1 → партиции 0-2
      • Консюмер 2 → партиции 3-5

    Проблема: при разном количестве партиций в разных топиках
    распределение может быть неравномерным.

  4.2. ROUNDROBIN
    Как работает:
      • Партиции распределяются по кругу между консюмерами.
      • Учитываются все топики вместе.

    Пример (2 топика по 3 партиции, 2 консюмера):
      • Консюмер 1 → P0, P2, P4
      • Консюмер 2 → P1, P3, P5

    Преимущество: более равномерное распределение при нескольких топиках.

  4.3. STICKY (РЕКОМЕНДУЕТСЯ ДЛЯ EAGER)
    Как работает:
      • Минимизирует перемещение партиций при ребалансировке.
      • Сохраняет существующие назначения, насколько это возможно.
      • Перемещает только те партиции, которые необходимы для балансировки.

    Преимущество: уменьшает время ребалансировки и снижает нагрузку.

  4.4. COOPERATIVE STICKY (РЕКОМЕНДУЕТСЯ ДЛЯ PRODUCTION)
    Как работает:
      • Сочетает Sticky стратегию с Cooperative ребалансировкой.
      • Минимизирует перемещение партиций И позволяет продолжать чтение
        во время ребалансировки.

    Это лучшая стратегия для большинства production-сценариев.

  5.  ЛИДЕР ГРУППЫ — КТО ОН И ЗАЧЕМ НУЖЕН
  Лидер группы — это один из консюмеров в группе, который берёт на себя
  ответственность за вычисление нового плана распределения партиций.

  КАК ВЫБИРАЕТСЯ ЛИДЕР:
    • Первый консюмер, приславший JoinGroup запрос, становится лидером.
    • Если лидер уходит из группы, новый лидер выбирается из оставшихся.

  РОЛЬ ЛИДЕРА:
    • Получает список всех консюмеров в группе и их подписки.
    • Применяет стратегию распределения (Range, RoundRobin, Sticky, Cooperative).
    • Вычисляет, какие партиции должны быть назначены каждому консюмеру.
    • Отправляет план координатору через SyncGroup.

  ПОЧЕМУ ЭТО ВАЖНО:
    • Лидер выполняет вычисления, разгружая координатора.
    • Это масштабируемо — вычисления распределяются между консюмерами.

  6.  STATIC MEMBERSHIP — УМЕНЬШЕНИЕ РЕБАЛАНСИРОВОК
  Static Membership (KIP-345) — это механизм, который позволяет консюмеру
  сохранять своё членство в группе при перезапуске.

  БЕЗ STATIC MEMBERSHIP:
    • При перезапуске консюмер уходит из группы, происходит ребалансировка.
    • При повторном подключении — ещё одна ребалансировка.
    • Если консюмер часто перезапускается (например, rolling update),
      ребалансировки происходят постоянно.

  С STATIC MEMBERSHIP:
    • Консюмер имеет уникальный ID (group.instance.id).
    • При перезапуске координатор узнаёт консюмера по ID и сохраняет
      его партиции на время перезапуска.
    • Ребалансировка происходит только если консюмер не вернулся
      в течение session.timeout.ms.

  НАСТРОЙКА:
    config.Consumer.Group.InstanceId = "consumer-1"

  ПРЕИМУЩЕСТВА:
    • Значительно уменьшает количество ребалансировок.
    • Полезно при rolling updates.
    • Улучшает стабильность групп с большим количеством партиций.

  ВАЖНО: group.instance.id должен быть уникальным для каждого консюмера
  в группе и статичным на протяжении жизни консюмера.

  7.  ЖИЗНЕННЫЙ ЦИКЛ РЕБАЛАНСИРОВКИ (ПОД КАПОТОМ)

  1. ИНИЦИАЦИЯ (внешнее событие)
     • Добавление/удаление консюмера.
     • Изменение подписки.

  2. ОБНАРУЖЕНИЕ КООРДИНАТОРОМ
     • Координатор группы (один из брокеров) замечает изменение.

  3. ПРИНЯТИЕ РЕШЕНИЯ О РЕБАЛАНСИРОВКЕ
     • Координатор отправляет уведомление всем консюмерам.

  4. ПОДГОТОВКА К РЕБАЛАНСИРОВКЕ
     • Консюмеры выполняют Cleanup (если реализован).
     • Revoke партиций (для Eager — все, для Cooperative — только
       те, которые будут переданы).

  5. ВЫБОР ЛИДЕРА ГРУППЫ
     • Один из консюмеров становится лидером группы.
     • Лидер получает список всех консюмеров и партиций.

  6. РАСПРЕДЕЛЕНИЕ ПАРТИЦИЙ
     • Лидер применяет стратегию распределения.
     • Формирует новый план назначения.

  7. ОТПРАВКА ПЛАНА КООРДИНАТОРУ
     • Координатор утверждает план.

  8. НАЗНАЧЕНИЕ ПАРТИЦИЙ
     • Каждый консюмер получает свои новые партиции.
     • Выполняется Setup (если реализован).
     • Начинается чтение.

  8.  РЕБАЛАНСИРОВКА И КОММИТЫ СМЕЩЕНИЙ — КАК НЕ ПОТЕРЯТЬ ДАННЫЕ
  Критически важно: во время ребалансировки консюмеры должны корректно
  коммитить смещения, чтобы не потерять данные и не обработать их повторно.

  ПРОБЛЕМА:
    • При ребалансировке партиции переходят от одного консюмера к другому.
    • Если смещения не закоммичены, новый консюмер начнёт с последнего
      закоммиченного смещения, что может привести к повторной обработке
      или потере сообщений.

  РЕШЕНИЕ:
    • Использовать ручной коммит (enable.auto.commit=false).
    • Коммитить смещения в Cleanup() перед тем, как консюмер отдаёт
      свои партиции.
    • В Setup() новый консюмер начинает с последнего закоммиченного
      смещения.

  В SARAMA:
    type handler struct{}

    func (h *handler) Cleanup(session sarama.ConsumerGroupSession) error {
        // Коммитим все смещения перед ребалансировкой
        session.Commit()
        return nil
    }

    func (h *handler) Setup(session sarama.ConsumerGroupSession) error {
        // Получаем назначенные партиции
        for topic, partitions := range session.Claims() {
            log.Printf("Назначены партиции: %v", partitions)
        }
        return nil
    }
  ВАЖНО: Если вы используете автоматический коммит, он может произойти
  в неподходящий момент, и данные могут быть потеряны. Ручной коммит
  даёт полный контроль.

  9.  ОБРАБОТКА РЕБАЛАНСИРОВКИ В КОДЕ (CONSUMERREBALANCELISTENER)
  В Java есть ConsumerRebalanceListener, который позволяет выполнять
  действия при ребалансировке. В Go (sarama) это реализовано через методы
  Setup и Cleanup интерфейса ConsumerGroupHandler.

  ПРИМЕР В SARAMA:

    type MyHandler struct{}

    func (h *MyHandler) Setup(session sarama.ConsumerGroupSession) error {
        // Вызывается при назначении партиций (после ребалансировки)
        log.Println("Setup: партиции назначены")
        return nil
    }

    func (h *MyHandler) Cleanup(session sarama.ConsumerGroupSession) error {
        // Вызывается перед ребалансировкой (перед отзывом партиций)
        log.Println("Cleanup: ребалансировка, коммитим смещения")
        session.Commit()
        return nil
    }

    func (h *MyHandler) ConsumeClaim(session sarama.ConsumerGroupSession,
        claim sarama.ConsumerGroupClaim) error {
        // Обработка сообщений
        for msg := range claim.Messages() {
            // обработка
            session.MarkMessage(msg, "")
        }
        return nil
    }

  ЭТО КЛЮЧЕВОЙ ПАТТЕРН ДЛЯ НАДЁЖНОЙ ОБРАБОТКИ В ПРОДАКШЕНЕ.

  10. ПРОБЛЕМЫ: REBALANCE TIMEOUT, STUCK REBALANCE, REBALANCE STORM

  10.1. REBALANCE TIMEOUT
    Симптом: ребалансировка не завершается, консюмеры не получают
    назначения, группа "зависает".

    Причина: один из консюмеров не отвечает на heartbeat (сетевая проблема,
    долгая обработка, GC).

    Решение:
      • Увеличить session.timeout.ms.
      • Уменьшить heartbeat.interval.ms.
      • Оптимизировать обработку (чтобы не превышать max.poll.interval.ms).

  10.2. STUCK REBALANCE (ЗАВИСШАЯ РЕБАЛАНСИРОВКА)
    Симптом: ребалансировка началась, но не завершается, хотя все консюмеры
    живы.

    Причина: лидер группы не может вычислить план (например, из-за ошибки
    в стратегии или из-за несовместимых подписок).

    Решение:
      • Проверить логи лидера группы.
      • Убедиться, что все консюмеры используют одинаковую стратегию.
      • Перезапустить группу (пересоздать консюмеров).

  10.3. REBALANCE STORM (ШТОРМ РЕБАЛАНСИРОВОК)
    Симптом: ребалансировки происходят постоянно, каждые несколько секунд.

    Причина:
      • Консюмеры постоянно падают и переподключаются.
      • Слишком маленький session.timeout.ms.
      • Проблемы с сетью.

    Решение:
      • Увеличить session.timeout.ms.
      • Использовать Static Membership.
      • Проверить стабильность сети и GC.

  11. НАСТРОЙКА ПАРАМЕТРОВ ДЛЯ PRODUCTION (РЕКОМЕНДАЦИИ CONFLUENT)

  11.1. ВЫБОР СТРАТЕГИИ
    • Для большинства сценариев используйте CooperativeStickyAssignor.
    • Если у вас старые версии Kafka (< 2.4), используйте StickyAssignor.

  11.2. ТАЙМАУТЫ
    • session.timeout.ms = 45000 (45 секунд) — стандарт.
    • heartbeat.interval.ms = 3000 (3 секунды) — 1/3 от session.timeout.
    • max.poll.interval.ms = 300000 (5 минут) — для долгой обработки.

  11.3. STATIC MEMBERSHIP
    • group.instance.id = уникальный идентификатор (например, hostname).
    • Используйте для rolling updates.

  11.4. В SARAMA (GO)
    config := sarama.NewConfig()
    config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
        sarama.NewBalanceStrategyCooperativeSticky(),
    }
    config.Consumer.Group.InstanceId = "consumer-1"
    config.Consumer.Group.Session.Timeout = 45 * time.Second
    config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

  12. ПОДВОДНЫЕ КАМНИ И ЧАСТЫЕ ОШИБКИ

  12.1. "Я ИСПОЛЬЗУЮ РУЧНОЙ КОММИТ, НО РЕБАЛАНСИРОВКА ТЕРЯЕТ ДАННЫЕ"
    Причина: коммит не выполняется в Cleanup.
    Решение: всегда коммитить в Cleanup.

  12.2. "РЕБАЛАНСИРОВКА ПРОИСХОДИТ КАЖДЫЙ РАЗ, КОГДА Я ПЕРЕЗАПУСКАЮ КОНСЮМЕР"
    Причина: нет Static Membership.
    Решение: включить group.instance.id.

  12.3. "ГРУППА НЕ МОЖЕТ ЗАВЕРШИТЬ РЕБАЛАНСИРОВКУ"
    Причина: один из консюмеров не отвечает на heartbeat.
    Решение: проверить сеть, увеличить таймауты.

  12.4. "ПОСЛЕ РЕБАЛАНСИРОВКИ КОНСЮМЕРЫ НАЧИНАЮТ ЧИТАТЬ С НАЧАЛА"
    Причина: auto.offset.reset = earliest, а смещений нет.
    Решение: убедиться, что смещения закоммичены перед ребалансировкой.

  12.5. "РЕБАЛАНСИРОВКА ДЛИТСЯ ОЧЕНЬ ДОЛГО"
    Причина: большое количество партиций и консюмеров, Eager стратегия.
    Решение: использовать Cooperative Sticky стратегию.

  13. КЛЮЧЕВЫЕ ВЫВОДЫ
  1.  Consumer Rebalance — перераспределение партиций между консюмерами
      при изменении состава группы.
  2.  Eager Rebalance — stop-the-world, все консюмеры останавливаются.
      Cooperative Rebalance — частичная, только изменяемые партиции.
  3.  Стратегии распределения:
      • Range — диапазоны (был по умолчанию).
      • RoundRobin — по кругу.
      • Sticky — минимизирует перемещения (рекомендуется).
      • Cooperative Sticky — Sticky + Cooperative (лучшая).
  4.  Static Membership (group.instance.id) — консюмер сохраняет ID
      при перезапуске, уменьшает ребалансировки.
  5.  Влияние на производительность: ребалансировка вызывает паузу
      в потреблении (особенно Eager).
  6.  Решения: Cooperative + Sticky + Static Membership + правильные
      таймауты.
  7.  В Go (sarama): config.Consumer.Group.Rebalance.GroupStrategies
      и config.Consumer.Group.InstanceId.
  8.  Таймауты: session.timeout.ms, heartbeat.interval.ms,
      max.poll.interval.ms — критичны для стабильности.
  9.  Если обработка долгая — увеличивайте max.poll.interval.ms
      и используйте ручной коммит.
  10. Static Membership — обязателен для rolling updates без простоя.
  11. Всегда коммитьте смещения в Cleanup() для защиты от потери данных.
  12. Мониторьте ребалансировки (частота, задержка) для обнаружения проблем.
  13. Кооперативная ребалансировка — это будущее, используйте её
      в новых проектах.
*/

//КОНФИГУРАЦИЯ

var (
	mode       = flag.String("mode", "consumer", "Режим: consumer, producer")
	broker     = flag.String("broker", "localhost:9092", "Адрес брокера")
	topic      = flag.String("topic", "rebalance-test", "Топик")
	groupID    = flag.String("group", "test-group", "ID группы консюмеров")
	consumerID = flag.String("id", "", "Уникальный ID консюмера (для Static Membership)")
)

//PRODUCE

func runProducer() error {
	log.Println("Запуск продюсера для отправки тестовых сообщений")

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Idempotent = true

	producer, err := sarama.NewSyncProducer([]string{*broker}, config)
	if err != nil {
		return fmt.Errorf("ошибка создания продюсера: %w", err)
	}
	defer producer.Close()

	// Отправляем 100 сообщений с разными ключами
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("message-%d", i)
		msg := &sarama.ProducerMessage{
			Topic: *topic,
			Key:   sarama.StringEncoder(key),
			Value: sarama.StringEncoder(value),
		}
		_, _, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
		}
		if i%10 == 0 {
			log.Printf("Отправлено %d сообщений", i+1)
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.Println("Все сообщения отправлены")
	return nil
}

// CONSUMER HANDLER
type ConsumerHandler struct {
	consumerID string
	groupID    string
	ready      chan bool
	mu         sync.Mutex
}

func NewConsumerHandler(consumerID, groupID string) *ConsumerHandler {
	return &ConsumerHandler{
		consumerID: consumerID,
		groupID:    groupID,
		ready:      make(chan bool),
	}
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("🔧 [%s] Setup: назначены партиции", h.consumerID)
	for topic, partitions := range session.Claims() {
		log.Printf("   Топик: %s, партиции: %v", topic, partitions)
	}

	// Сигнализируем, что консюмер готов
	close(h.ready)
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[%s] Cleanup: ребалансировка, коммитим смещения", h.consumerID)
	// Коммитим все смещения, чтобы новый консюмер продолжил с правильного места
	session.Commit() //Commit() не возвращает ошибку
	log.Printf("[%s] Смещения закоммичены", h.consumerID)
	return nil
}

// ConsumeClaim — основной метод обработки сообщений.
func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	log.Printf("[%s] Начало обработки партиций: %v", h.consumerID, claim.Partition())

	var count int

	for msg := range claim.Messages() {
		count++
		log.Printf("[%s] Получено: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
			h.consumerID, msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))

		// Имитация обработки (например, запись в БД)
		time.Sleep(50 * time.Millisecond)

		// 🔥 Ручной отметка смещения после успешной обработки
		session.MarkMessage(msg, "")

		// Периодический коммит для уменьшения нагрузки
		if count%10 == 0 {
			session.Commit() // Commit() не возвращает ошибку
			log.Printf("[%s] Закоммичено %d сообщений", h.consumerID, count)
		}
	}

	log.Printf("[%s] Завершена обработка партиции, обработано %d сообщений", h.consumerID, count)
	return nil
}

// CONSUMER GROUP
func runConsumer() error {
	if *consumerID == "" {
		return fmt.Errorf("для консюмера необходимо указать -id (для Static Membership)")
	}

	log.Printf("Запуск консюмера: id=%s, group=%s", *consumerID, *groupID)

	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0

	//НАСТРОЙКИ РЕБАЛАНСИРОВКИ

	// 1. Стратегия: Cooperative Sticky (современная, минимальный downtime)
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyCooperativeSticky(),
	}

	// 2. Static Membership — консюмер сохраняет ID при перезапуске
	config.Consumer.Group.InstanceId = *consumerID

	// 3. Таймауты (рекомендованные Confluent)
	config.Consumer.Group.Session.Timeout = 45 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	// 4. Ручной коммит (обязательно для контроля)
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	// Создаём Consumer Group
	client, err := sarama.NewConsumerGroup([]string{*broker}, *groupID, config)
	if err != nil {
		return fmt.Errorf("ошибка создания consumer group: %w", err)
	}
	defer client.Close()

	// Обработчик
	handler := NewConsumerHandler(*consumerID, *groupID)

	// Контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Перехват сигналов
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	go func() {
		<-sigCh
		log.Printf("[%s] Получен сигнал завершения...", *consumerID)
		cancel()
	}()

	// Запуск потребления в горутине
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := client.Consume(ctx, []string{*topic}, handler); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					log.Printf("ℹ[%s] Контекст отменён, завершаем", *consumerID)
					return
				}
				log.Printf("[%s] Ошибка консюмера: %v", *consumerID, err)
				time.Sleep(1 * time.Second)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Ждём готовности консюмера (первая ребалансировка)
	<-handler.ready
	log.Printf("[%s] Консюмер готов, начал обработку", *consumerID)

	// Ждём завершения
	wg.Wait()
	log.Printf("[%s] Консюмер завершён", *consumerID)
	return nil
}

func main() {
	flag.Parse()

	var err error
	switch *mode {
	case "producer":
		err = runProducer()
	case "consumer":
		err = runConsumer()
	default:
		err = fmt.Errorf("неизвестный режим: %s. Используйте consumer или producer", *mode)
	}

	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
