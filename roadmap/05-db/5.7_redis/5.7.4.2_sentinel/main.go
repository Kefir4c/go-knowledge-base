package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 4.2. SENTINEL — ВЫСОКАЯ ДОСТУПНОСТЬ И АВТОМАТИЧЕСКИЙ FAILOVER
0. ВВЕДЕНИЕ: ЗАЧЕМ SENTINEL?

Master-Replica репликация даёт нам масштабирование чтения и базовую отказоустойчивость,
НО если мастер падает, нам нужно вручную переключаться на реплику. Это требует
времени и может привести к даунтайму. Sentinel автоматизирует этот процесс.

Sentinel — это распределённая система, которая:
- Мониторит мастеров и реплики (проверяет их доступность).
- Автоматически выполняет failover (переключение на новую реплику).
- Уведомляет клиентов о новом мастере (через механизм подписки или опроса).
- Предоставляет API для управления и мониторинга.

1. АРХИТЕКТУРА И ВНУТРЕННЕЕ УСТРОЙСТВО

1.1. Компоненты:
    - Мастер (master) — основной Redis, принимает запись.
    - Реплики (replicas) — копии мастера, обслуживают чтение (если разрешено).
    - Sentinel'ы (обычно 3 или 5) — процессы, которые следят за мастером и репликами.
      Они не являются прокси, а только предоставляют информацию клиентам.

1.2. Как Sentinel'ы взаимодействуют друг с другом:
    - Они обмениваются сообщениями через Pub/Sub каналы Redis.
    - Каждый Sentinel подписывается на каналы других Sentinel'ов,
      чтобы получать обновления о состоянии мастеров.
    - Используется протокол «gossip» для распространения информации.
    - Sentinel'ы также проверяют доступность друг друга (но это не критично,
      так как система остаётся работоспособной, пока есть кворум).

1.3. Обнаружение сбоя (Failure Detection):
    - Каждый Sentinel периодически (каждую секунду) отправляет PING мастеру и репликам.
    - Если ответ не получен в течение down-after-milliseconds (обычно 5-10 секунд),
      Sentinel помечает мастера как S_DOWN (subjectively down) — подозреваемый.
    - Другие Sentinel'ы также проверяют доступность мастера.
    - Если N (кворум) Sentinel'ов подтверждают недоступность, мастер переводится
      в состояние O_DOWN (objectively down) — объективно недоступен.
    - Только после O_DOWN запускается процесс failover.

1.4. Кворум и роль большинства:
    - Кворум (quorum) — минимальное количество Sentinel'ов, которые должны
      согласиться, что мастер недоступен. Например, при 5 Sentinel'ах кворум = 2.
    - Для запуска failover требуется также большинство (majority) Sentinel'ов.
      При 5 процессах — 3, при 3 — 2.
    - Эти механизмы предотвращают ложные переключения при сетевых сбоях.

1.5. Алгоритм выбора нового мастера (leader election):
    1. Когда мастер переходит в O_DOWN, Sentinel'ы запускают выборы лидера.
    2. Лидером становится Sentinel, который первым обнаружил сбой (или голосованием).
    3. Лидер выбирает лучшую реплику для повышения до мастера по критериям:
        - Максимальное смещение репликации (slave_repl_offset) — наиболее актуальные данные.
        - Приоритет реплики (replica-priority, по умолчанию 100).
        - Если приоритеты равны — выбирается с самым большим offset.
        - Если и offset равны — выбирается та, которая первой ответила на PING.
        - Если ни одна реплика не подходит (все отстали) — ошибка.
    4. Выбранная реплика повышается до мастера (REPLICAOF NO ONE).
    5. Остальные реплики перенастраиваются на нового мастера (REPLICAOF <new_master>).
    6. В конфигурацию мастера записывается новый адрес, чтобы после перезапуска
       он стал репликой нового мастера (если не настроено иное).

1.6. Время failover:
    - Зависит от down-after-milliseconds, failover-timeout и количества реплик.
    - В среднем: 10–30 секунд.
    - В это время запись недоступна (мастер не принимает запросы).
    - Клиенты, использующие Sentinel, могут получить ошибку, но затем
      автоматически переключиться на нового мастера.

2. КОНФИГУРАЦИЯ SENTINEL (ПОДРОБНО)

2.1. Базовые настройки (sentinel.conf):
    port 26379                          # порт, на котором слушает Sentinel
    sentinel monitor mymaster 127.0.0.1 6379 2   # имя мастера, адрес, порт, кворум
    sentinel down-after-milliseconds mymaster 5000   # время до объявления S_DOWN
    sentinel failover-timeout mymaster 60000      # таймаут на failover
    sentinel parallel-syncs mymaster 1            # сколько реплик синхронизировать одновременно
    sentinel auth-pass mymaster mypassword        # пароль (если мастер защищён)

2.2. Дополнительные настройки:
    sentinel notification-script mymaster /path/to/script.sh   # скрипт для уведомлений
    sentinel client-reconfig-script mymaster /path/to/script.sh # скрипт для клиентов
    sentinel announce-ip 192.168.1.10        # IP, который будет объявляться клиентам
    sentinel announce-port 26379             # порт для объявления
    sentinel resolve-hostnames yes           # разрешать DNS имена (для Kubernetes)

2.3. Динамическое изменение:
    SENTINEL SET mymaster down-after-milliseconds 3000
    SENTINEL SET mymaster quorum 3

2.4. Команды управления:
    SENTINEL masters                                    # список всех мастеров
    SENTINEL master <master_name>                       # информация о мастере
    SENTINEL slaves <master_name>                       # список реплик
    SENTINEL sentinels <master_name>                    # список Sentinel'ов
    SENTINEL get-master-addr-by-name <master_name>      # адрес мастера
    SENTINEL failover <master_name>                     # принудительный failover
    SENTINEL flushconfig                                # применить изменения к конфигу

3. ПОВЕДЕНИЕ КЛИЕНТА (GO-REDIS) С SENTINEL

3.1. Как работает go-redis с Sentinel:
    - Клиент подключается к одному из Sentinel'ов (по списку адресов).
    - При создании клиента он опрашивает Sentinel и получает адрес текущего мастера.
    - Этот адрес кэшируется и используется для всех операций.
    - Периодически (или при ошибке) клиент перепроверяет мастер через Sentinel.
    - Если мастер меняется (после failover), клиент получает новый адрес
      и автоматически переключается на нового мастера.

3.2. Важные параметры клиента:
    MasterName: "mymaster"                    # имя мастера из sentinel.conf
    SentinelAddrs: []string{"localhost:26379", "localhost:26380", "localhost:26381"}
    MaxRetries: 3                             # количество попыток при ошибках
    MinRetryBackoff: 100 * time.Millisecond   # задержка между попытками
    ReadTimeout: 3 * time.Second              # таймаут чтения
    WriteTimeout: 3 * time.Second             # таймаут записи
    PoolSize: 10                              # размер пула соединений
    MinIdleConns: 2                           # минимальное число idle соединений

3.3. Обработка ошибок:
    - Если мастер недоступен, клиент получает ошибку (обычно `redis: connection refused`).
    - При следующем запросе клиент опрашивает Sentinel и обновляет адрес мастера.
    - Это происходит автоматически, без перезапуска приложения.
    - Можно настроить Retry для автоматического повторения команд.

3.4. Работа с репликами (чтение с реплик):
    - FailoverClient по умолчанию всегда обращается к мастеру для всех команд.
    - Для чтения с реплик нужно создать отдельный клиент, получающий список реплик
      через Sentinel и подключающийся к ним.
    - Альтернативно: использовать ClusterClient с ReadOnly для кластера.

4. СРАВНЕНИЕ SENTINEL И REDIS CLUSTER

┌──────────────────────────┬──────────────────────────┬──────────────────────────┐
│ ХАРАКТЕРИСТИКА           │ SENTINEL                 │ REDIS CLUSTER            │
├──────────────────────────┼──────────────────────────┼──────────────────────────┤
│ Назначение               │ Высокая доступность      │ Масштабирование + HA     │
│ Репликация               │ Асинхронная              │ Асинхронная              │
│ Автоматический failover  │ Да (через Sentinel)      │ Да (встроенный)          │
│ Шардирование             │ Нет                      │ Да (16384 слота)         │
│ Максимальный размер      │ Ограничен памятью        │ Масштабируется           │
│ Сложность настройки      │ Средняя                  │ Высокая (кластер сложнее)│
│ Поддержка клиентов       │ Широкая (через Sentinel) │ Широкая (через Cluster)  │
│ Когда использовать       │ До 100 ГБ данных         │ > 100 ГБ, большое кол-во │
│                          │                          │ ключей, шардирование     │
└──────────────────────────┴──────────────────────────┴──────────────────────────┘

5. ПОДВОДНЫЕ КАМНИ И РЕШЕНИЯ

5.1. Ложные срабатывания (false positive)
    - Сетевые задержки могут привести к ошибочному объявлению мастера недоступным.
    - Решение: увеличить down-after-milliseconds (например, 10-15 секунд).
    - Использовать несколько Sentinel'ов для подтверждения.

5.2. Сетевые разделения (split-brain)
    - Если сеть разделилась, могут появиться два мастера (старый и новый).
    - Sentinel решает это через кворум: новые записи могут быть потеряны,
      но старый мастер не будет принимать запись, если он не может связаться
      с большинством Sentinel'ов.
    - В некоторых версиях Redis реализована защита от split-brain.

5.3. Асинхронная репликация и потеря данных
    - При failover часть данных, записанных на старый мастер, может не успеть
      синхронизироваться с новой репликой.
    - Для минимизации используйте WAIT, но это снижает производительность.
    - Или используйте синхронную репликацию (но это нестандартно).

5.4. Время восстановления
    - При большом количестве данных полная синхронизация реплик может занять время.
    - Используйте repl-diskless-sync для ускорения.
    - Настраивайте parallel-syncs для одновременной синхронизации нескольких реплик.

5.5. Обновление конфигурации
    - После изменения конфигурации Sentinel нужно перезагрузить (или использовать SENTINEL SET).
    - Убедитесь, что все Sentinel'ы имеют одинаковую конфигурацию.

6. МОНИТОРИНГ И ДИАГНОСТИКА SENTINEL

6.1. Логи:
    - Sentinel логирует все события (доступность мастеров, failover).
    - Просмотр: tail -f /var/log/redis/sentinel.log.
    - Уровень логирования можно настроить в sentinel.conf: loglevel debug.

6.2. Метрики через INFO:
    - INFO sentinel — общая информация.
    - INFO sentinel.sentinels — список Sentinel'ов.

6.3. Команды мониторинга:
    - SENTINEL master mymaster — статус мастера (флаги, количество реплик).
    - SENTINEL slaves mymaster — состояние реплик (подключены ли, отставание).
    - SENTINEL sentinels mymaster — список других Sentinel'ов.

6.4. Алертинг:
    - Настраивайте оповещения при failover (script).
    - Используйте скрипты для отправки уведомлений в Telegram, Email, Slack.

7. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА (ЧЕК-ЛИСТ)

1. Запускайте минимум 3 Sentinel'а на разных хостах (для кворума).
2. Убедитесь, что все Sentinel'ы имеют синхронизированные часы (NTP).
3. Настройте down-after-milliseconds не менее 5 секунд (например, 10 секунд).
4. failover-timeout должен быть достаточным для синхронизации реплик (обычно 60 секунд).
5. Включите auth-pass, если мастер защищён паролем.
6. Установите announce-ip, если Sentinel работает в NAT или Docker.
7. Настройте script для оповещения о событиях.
8. Регулярно тестируйте failover в тестовой среде (SENTINEL failover).
9. Мониторьте логи и метрики Sentinel.
10. Рассмотрите использование Redis Cluster для больших систем.
11. В коде (go-redis) настройте таймауты и ретраи.
12. Проверяйте, что клиент корректно переключается на нового мастера.

8. ПРИМЕР РАБОЧЕЙ КОНФИГУРАЦИИ SENTINEL (sentinel.conf)

port 26379
sentinel monitor mymaster 192.168.1.10 6379 2
sentinel down-after-milliseconds mymaster 10000
sentinel failover-timeout mymaster 60000
sentinel parallel-syncs mymaster 1
sentinel auth-pass mymaster mypassword
sentinel announce-ip 192.168.1.10
sentinel resolve-hostnames yes
logfile /var/log/redis/sentinel.log
loglevel notice

9. СВЯЗЬ С GO: КЛИЕНТСКАЯ ЧАСТЬ (РЕЗЮМЕ)

Клиент в go-redis создаётся через:
    client := redis.NewFailoverClient(&redis.FailoverOptions{
        MasterName:    "mymaster",
        SentinelAddrs: []string{"localhost:26379", "localhost:26380", "localhost:26381"},
        Password:      "mypassword",
        DB:            0,
        MaxRetries:    3,
        PoolSize:      10,
    })

После создания клиент автоматически получает адрес мастера от Sentinel.
При изменении мастера клиент переключается без перезапуска.

10. ЗАКЛЮЧЕНИЕ

Sentinel — это надёжный механизм обеспечения высокой доступности для Redis.
Он не требует изменений в приложении (клиенты должны поддерживать Sentinel,
но go-redis это делает). Основные преимущества: автоматический failover,
минимальное время простоя, лёгкость настройки.

Однако, Sentinel не является решением для масштабирования — для этого используйте Cluster.
Выбор между Sentinel и Cluster зависит от объёма данных и требований к масштабируемости.
Для большинства проектов Sentinel (с 3-мя экземплярами) — это золотая середина.
*/

// 0. Глобальные клиенты и настройки

var (
	// Основной клиент с поддержкой failover
	failoverClient *redis.Client

	// Клиент для управления Sentinel (мониторинг, команды)
	sentinelClient *redis.Client

	// Контекст с отменой для graceful shutdown
	ctx, cancelFunc = context.WithCancel(context.Background())

	// Логгер (можно заменить на любой структурированный)
	logger = log.Default()
)
var SentinelAddrs = []string{"localhost:26379", "localhost:26380", "localhost:26381"}

// Настройки для продакшна
const (
	MasterName      = "mymaster"
	MaxRetries      = 3
	MinRetryBackoff = 100 * time.Millisecond
	MaxRetryBackoff = 2 * time.Second
	ReadTimeout     = 3 * time.Second
	WriteTimeout    = 3 * time.Second
	PoolSize        = 20
	MinIdleConns    = 5
	ConnMaxLifetime = 30 * time.Minute
	ConnMaxIdleTime = 10 * time.Minute
	DialTimeout     = 5 * time.Second
)

// Инициализация клиентов
func init() {
	// Клиент для управления Sentinel
	sentinelClient = redis.NewClient(&redis.Options{
		Addr:         SentinelAddrs[0],
		DialTimeout:  DialTimeout,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		PoolSize:     5,
	})

	// Основной failover-клиент
	failoverClient = redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:      MasterName,
		SentinelAddrs:   SentinelAddrs,
		Password:        "", // если есть пароль
		DB:              0,
		MaxRetries:      MaxRetries,
		MinRetryBackoff: MinRetryBackoff,
		MaxRetryBackoff: MaxRetryBackoff,
		DialTimeout:     DialTimeout,
		ReadTimeout:     ReadTimeout,
		WriteTimeout:    WriteTimeout,
		PoolSize:        PoolSize,
		MinIdleConns:    MinIdleConns,
		ConnMaxLifetime: ConnMaxLifetime,
		ConnMaxIdleTime: ConnMaxIdleTime,
		PoolTimeout:     4 * time.Second,
	})

	// Проверяем соединение
	if err := failoverClient.Ping(ctx).Err(); err != nil {
		logger.Printf("⚠️  Ошибка подключения к Redis через Sentinel: %v", err)
	} else {
		logger.Println("Клиент Sentinel успешно инициализирован")
	}
}

func main() {
	fmt.Println("=== ПРОДАКШН-ПРИМЕРЫ SENTINEL ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
}

// 1. НАСТРОЙКА КЛИЕНТА С ОПТИМАЛЬНЫМИ ПАРАМЕТРАМИ
func primer1() {
	fmt.Println("--- 1. Клиент с оптимальными параметрами для продакшна ---")

	// Мы уже создали failoverClient в init.
	// Здесь только показываем, как можно кастомизировать.
	// В реальном проекте параметры выносятся в конфиг.

	fmt.Printf("MasterName: %s\n", MasterName)
	fmt.Printf("SentinelAddrs: %v\n", SentinelAddrs)
	fmt.Printf("PoolSize: %d, MinIdleConns: %d\n", PoolSize, MinIdleConns)
	fmt.Printf("ReadTimeout: %v, WriteTimeout: %v\n", ReadTimeout, WriteTimeout)
	fmt.Printf("MaxRetries: %d, RetryBackoff: %v - %v\n", MaxRetries, MinRetryBackoff, MaxRetryBackoff)

	// Проверяем, что клиент работает
	pong, err := failoverClient.Ping(ctx).Result()
	if err != nil {
		logger.Printf("Ошибка PING: %v", err)
	} else {
		fmt.Printf("PING ответ: %s\n", pong)
	}
	fmt.Println()
}

// 2. БЕЗОПАСНОЕ ВЫПОЛНЕНИЕ КОМАНД С РЕТРАЯМИ

// SafeExecutor — обёртка для выполнения команд с автоматическим ретраем и логированием
type SafeExecutor struct {
	client *redis.Client
	logger *log.Logger
}

func NewSafeExecutor(client *redis.Client) *SafeExecutor {
	return &SafeExecutor{client: client, logger: logger}
}

// Execute выполняет функцию с ретраями при ошибках (кроме redis.Nil)
func (e *SafeExecutor) Execute(ctx context.Context, op func(ctx context.Context) error) error {
	var lastErr error
	backoff := MinRetryBackoff
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// Экспоненциальная задержка с джиттером
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			sleepDuration := backoff + jitter
			if sleepDuration > MaxRetryBackoff {
				sleepDuration = MaxRetryBackoff
			}
			e.logger.Printf("Повторная попытка %d/%d через %v", attempt, MaxRetries, sleepDuration)
			select {
			case <-time.After(sleepDuration):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
			if backoff > MaxRetryBackoff {
				backoff = MaxRetryBackoff
			}
		}

		err := op(ctx)
		if err == nil {
			return nil
		}
		// Если ключ не найден — не повторяем
		if errors.Is(err, redis.Nil) {
			return err
		}
		lastErr = err
		e.logger.Printf("Ошибка выполнения (попытка %d): %v", attempt+1, err)
	}
	return fmt.Errorf("исчерпаны все попытки (%d): %w", MaxRetries+1, lastErr)
}

func primer2() {
	fmt.Println("--- 2. Безопасное выполнение команд с ретраями ---")

	executor := NewSafeExecutor(failoverClient)
	key := "safe:test"

	// Пример записи
	err := executor.Execute(ctx, func(ctx context.Context) error {
		return failoverClient.Set(ctx, key, "value", 10*time.Second).Err()
	})
	if err != nil {
		fmt.Printf("Ошибка записи: %v\n", err)
	} else {
		fmt.Println("Запись выполнена успешно")
	}

	// Пример чтения
	var val string
	err = executor.Execute(ctx, func(ctx context.Context) error {
		var e error
		val, e = failoverClient.Get(ctx, key).Result()
		return e
	})
	if errors.Is(err, redis.Nil) {
		fmt.Println("Ключ не найден")
	} else if err != nil {
		fmt.Printf("Ошибка чтения: %v\n", err)
	} else {
		fmt.Printf("Прочитано: %s\n", val)
	}

	// Очистка
	failoverClient.Del(ctx, key)
}

// 3. МОНИТОРИНГ СОСТОЯНИЯ МАСТЕРА И РЕПЛИК
func primer3() {
	fmt.Println("--- 3. Мониторинг состояния мастера и реплик ---")

	// Получаем информацию о мастере через Sentinel
	masterInfo, err := sentinelClient.Do(ctx, "SENTINEL", "master", MasterName).Result()
	if err != nil {
		fmt.Printf("Ошибка получения информации о мастере: %v\n", err)
		return
	}

	// Разбираем ответ (массив ключ-значение)
	data := masterInfo.([]interface{})
	master := make(map[string]interface{})
	for i := 0; i < len(data); i += 2 {
		key, _ := data[i].(string)
		val := data[i+1]
		master[key] = val
	}

	fmt.Printf("Мастер: %s:%s (flags: %s, реплик: %s)\n",
		master["ip"], master["port"], master["flags"], master["num-slaves"])

	// Получаем список реплик
	slavesRes, err := sentinelClient.Do(ctx, "SENTINEL", "slaves", MasterName).Result()
	if err != nil {
		fmt.Printf("Ошибка получения списка реплик: %v\n", err)
		return
	}
	slavesList := slavesRes.([]interface{})
	fmt.Printf("Найдено %d реплик:\n", len(slavesList))
	for i, s := range slavesList {
		slaveData := s.([]interface{})
		slave := make(map[string]interface{})
		for j := 0; j < len(slaveData); j += 2 {
			key, _ := slaveData[j].(string)
			val := slaveData[j+1]
			slave[key] = val
		}
		fmt.Printf("  Реплика %d: %s:%s (flags: %s, offset: %s)\n",
			i+1, slave["ip"], slave["port"], slave["flags"], slave["slave-repl-offset"])
	}

	// Проверяем статус подключения мастера к Sentinel
	// Можно также получить информацию из INFO replication
	info, err := failoverClient.InfoMap(ctx, "replication").Result()
	if err != nil {
		fmt.Printf("Ошибка INFO replication: %v\n", err)
	} else {
		role := info["replication"]["role"]
		connectedSlaves := info["replication"]["connected_slaves"]
		fmt.Printf("Роль текущего инстанса: %s, подключено реплик: %s\n", role, connectedSlaves)
	}
	fmt.Println()
}

// 4. ЧТЕНИЕ С РЕПЛИК С БАЛАНСИРОВКОЙ И FALLBACK
// ReplicaReader управляет чтением с реплик
type ReplicaReader struct {
	sentinelClient *redis.Client
	masterName     string
	mu             sync.RWMutex
	replicas       []string // адреса реплик "ip:port"
	logger         *log.Logger
}

func NewReplicaReader(sentinel *redis.Client, masterName string) *ReplicaReader {
	return &ReplicaReader{
		sentinelClient: sentinel,
		masterName:     masterName,
		logger:         logger,
	}
}

// UpdateReplicas обновляет список реплик из Sentinel
func (r *ReplicaReader) UpdateReplicas(ctx context.Context) error {
	res, err := r.sentinelClient.Do(ctx, "SENTINEL", "slaves", r.masterName).Result()
	if err != nil {
		return fmt.Errorf("не удалось получить реплики: %w", err)
	}
	slaves := res.([]interface{})
	var addrs []string
	for _, s := range slaves {
		data := s.([]interface{})
		var ip, port string
		for i := 0; i < len(data); i += 2 {
			key, _ := data[i].(string)
			val := data[i+1]
			if key == "ip" {
				ip = val.(string)
			} else if key == "port" {
				port = val.(string)
			}
		}
		if ip != "" && port != "" {
			addrs = append(addrs, ip+":"+port)
		}
	}
	if len(addrs) == 0 {
		return fmt.Errorf("нет доступных реплик")
	}
	r.mu.Lock()
	r.replicas = addrs
	r.mu.Unlock()
	return nil
}

// GetReplica возвращает случайную реплику (round-robin можно реализовать отдельно)
func (r *ReplicaReader) GetReplica() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.replicas) == 0 {
		return ""
	}
	// Простая балансировка — случайный выбор
	return r.replicas[rand.Intn(len(r.replicas))]
}

// ReadFromReplica выполняет чтение с реплики, при ошибке пробует другие реплики и мастер
func (r *ReplicaReader) ReadFromReplica(ctx context.Context, key string) (string, error) {
	// Обновляем список реплик (можно периодически в фоне)
	if err := r.UpdateReplicas(ctx); err != nil {
		r.logger.Printf("Ошибка обновления реплик: %v", err)
	}

	r.mu.RLock()
	replicas := make([]string, len(r.replicas))
	copy(replicas, r.replicas)
	r.mu.RUnlock()

	if len(replicas) == 0 {
		r.logger.Println("Нет реплик, читаем с мастера")
		return failoverClient.Get(ctx, key).Result()
	}

	// Перемешиваем, чтобы балансировать нагрузку
	rand.Shuffle(len(replicas), func(i, j int) {
		replicas[i], replicas[j] = replicas[j], replicas[i]
	})

	// Пробуем каждую реплику
	for _, addr := range replicas {
		client := redis.NewClient(&redis.Options{Addr: addr})
		val, err := client.Get(ctx, key).Result()
		client.Close()
		if err == nil {
			return val, nil
		}
		if errors.Is(err, redis.Nil) {
			return "", err // ключа нет ни на одной реплике
		}
		r.logger.Printf("Ошибка чтения с реплики %s: %v", addr, err)
	}

	// Все реплики недоступны — fallback на мастер
	r.logger.Println("Все реплики недоступны, читаем с мастера")
	return failoverClient.Get(ctx, key).Result()
}

func primer4() {
	fmt.Println("--- 4. Чтение с реплик с балансировкой и fallback ---")

	reader := NewReplicaReader(sentinelClient, MasterName)
	key := "balance:test"

	// Пишем в мастер
	err := failoverClient.Set(ctx, key, "some_value", 0).Err()
	if err != nil {
		fmt.Printf("Ошибка записи в мастер: %v\n", err)
		return
	}

	// Читаем с реплики
	val, err := reader.ReadFromReplica(ctx, key)
	if err != nil {
		fmt.Printf("Ошибка чтения: %v\n", err)
	} else {
		fmt.Printf("Прочитано с реплики: %s\n", val)
	}

	failoverClient.Del(ctx, key)
	fmt.Println()
}

// 5. ОБРАБОТКА ОШИБОК И АВТОМАТИЧЕСКОЕ ВОССТАНОВЛЕНИЕ
func primer5() {
	fmt.Println("--- 5. Обработка ошибок и автоматическое переключение ---")

	// Пытаемся выполнить команду на мастере
	err := failoverClient.Set(ctx, "failover:test", "value", 0).Err()
	if err != nil {
		fmt.Printf("Ошибка записи: %v\n", err)
		// Если мастер недоступен, клиент автоматически переключится при следующем запросе
	} else {
		fmt.Println("Запись выполнена")
	}

	// Имитируем падение мастера (можно вручную остановить мастер)
	// Здесь мы просто показываем, как клиент переключится.
	fmt.Println("Если мастер упадёт, следующий запрос автоматически пойдёт к новому мастеру.")

	// Проверяем, кто сейчас мастер
	info, err := failoverClient.InfoMap(ctx, "replication").Result()
	if err != nil {
		fmt.Printf("Ошибка получения INFO: %v\n", err)
	} else {
		role := info["replication"]["role"]
		fmt.Printf("Текущий мастер (по клиенту): %s\n", role)
	}
}

// 6. СИМУЛЯЦИЯ FAILOVER И ПРОВЕРКА ПЕРЕКЛЮЧЕНИЯ
func primer6() {
	fmt.Println("--- 6. Симуляция failover и проверка переключения ---")

	// Принудительно запускаем failover через Sentinel
	err := sentinelClient.Do(ctx, "SENTINEL", "failover", MasterName).Err()
	if err != nil {
		fmt.Printf("Ошибка запуска failover: %v\n", err)
		fmt.Println("Возможно, failover уже выполняется или нет реплик.")
	} else {
		fmt.Println("Запущен принудительный failover. Ждём переключения...")
		time.Sleep(5 * time.Second)

		// Проверяем нового мастера
		res, err := sentinelClient.Do(ctx, "SENTINEL", "get-master-addr-by-name", MasterName).Result()
		if err != nil {
			fmt.Printf("Ошибка получения нового мастера: %v\n", err)
		} else {
			addrs := res.([]interface{})
			fmt.Printf("Новый мастер: %s:%s\n", addrs[0], addrs[1])
		}
		// Проверяем, что клиент переключился
		pong, err := failoverClient.Ping(ctx).Result()
		if err != nil {
			fmt.Printf("Ошибка PING нового мастера: %v\n", err)
		} else {
			fmt.Printf("PING нового мастера: %s\n", pong)
		}
	}
}

// 7. GRACEFUL SHUTDOWN
func primer7() {
	fmt.Println("--- 7. Graceful shutdown ---")

	// В реальном приложении вызывается при получении сигнала SIGTERM
	// Мы эмулируем закрытие.

	// Отменяем контекст (если используются долгие операции)
	cancelFunc()

	// Закрываем клиенты
	if err := failoverClient.Close(); err != nil {
		logger.Printf("Ошибка закрытия failover-клиента: %v", err)
	} else {
		fmt.Println("Failover-клиент закрыт")
	}
	if err := sentinelClient.Close(); err != nil {
		logger.Printf("Ошибка закрытия sentinel-клиента: %v", err)
	} else {
		fmt.Println("Sentinel-клиент закрыт")
	}
}
