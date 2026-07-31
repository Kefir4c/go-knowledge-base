package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 4.3. REDIS CLUSTER
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖЕН CLUSTER?

Redis Cluster решает две основные проблемы:
  - Горизонтальное масштабирование (шардирование) — позволяет хранить данные,
    объёмом больше, чем может вместить один сервер, и увеличивать пропускную способность.
  - Высокая доступность — автоматическое переключение на реплики при падении мастера,
    без необходимости внешнего компонента (как Sentinel).

Cluster — это распределённая система, которая объединяет несколько узлов Redis
в единое логическое хранилище. Клиенты взаимодействуют с кластером как с единым целым,
но запросы автоматически направляются на нужный узел.

1. АРХИТЕКТУРА CLUSTER

1.1. Узлы (Nodes):
  - Каждый узел — это экземпляр Redis, работающий в кластерном режиме.
  - Узлы делятся на мастер-узлы (принимают запись) и реплики (копии мастеров).
  - Минимальное количество мастеров для рабочего кластера — 3 (для отказоустойчивости).
  - Обычно используется 6 узлов: 3 мастера + 3 реплики.

1.2. Слоты (Slots):
  - Данные распределяются по 16384 логическим сегментам — хеш-слотам.
  - Каждый мастер-узел отвечает за определённый диапазон слотов (например, 0–5460, 5461–10922, 10923–16383).
  - Ключ привязывается к слоту через хеш-функцию: HASH_SLOT = CRC16(key) % 16384.
  - Это позволяет равномерно распределять данные между узлами.

1.3. Репликация внутри кластера:
  - Каждый мастер может иметь одну или несколько реплик.
  - Репликация асинхронная (как в обычной Master-Replica).
  - Если мастер падает, одна из его реплик автоматически становится мастером (failover).
  - Клиент при этом получает MOVED-редирект и переключается на новый мастер.

1.4. Коммуникация между узлами:
  - Узлы общаются друг с другом через gossip-протокол (по портам +10000 от Redis-порта).
  - Они обмениваются информацией о состоянии, слотах, репликации, недоступных узлах.
  - Каждый узел имеет полную картину кластера (метаданные).

2. ХЕШ-ФУНКЦИЯ И ХЕШ-ТЕГИ

2.1. Стандартное хеширование:
  - CRC16(key) % 16384.
  - Хеш считается от полного имени ключа (байты).
  - Это гарантирует, что один и тот же ключ всегда попадает в один и тот же слот.

2.2. Хеш-теги (Hash Tags):
  - Если в ключе есть фигурные скобки {}, то хеш считается только от содержимого скобок.
  - Например:
  - user:123:profile → хешируется как "user:123:profile"
  - {user:123}.profile → хешируется как "user:123"
  - {user:123}.orders → хешируется как "user:123"
  - Это позволяет группировать несколько ключей в одном слоте, чтобы выполнять
    многоключевые операции (MGET, MSET, транзакции) в пределах одного слота.
  - Важно: хеш-теги чувствительны к регистру.

2.3. Рекомендации по использованию хеш-тегов:
  - Используйте для группировки связанных данных (например, данные пользователя).
  - Не злоупотребляйте, чтобы не нарушить балансировку (все ключи могут оказаться на одном узле).

3. ПРОТОКОЛ КОММУНИКАЦИИ И РЕДИРЕКТЫ

3.1. Клиентское взаимодействие:
  - Клиент (например, go-redis) вычисляет слот для каждого ключа.
  - Клиент кэширует карту слотов (какой узел отвечает за какой диапазон).
  - Запрос направляется на соответствующий узел.

3.2. Редиректы:
  - Если клиент отправил запрос не на тот узел (устаревшая карта), узел возвращает:
  - MOVED <slot> <ip:port> — постоянный редирект. Клиент должен обновить свою карту.
  - ASK <slot> <ip:port> — временный редирект (во время миграции слотов).
    Клиент должен выполнить запрос на указанном узле, но не обновлять карту.
  - go-redis автоматически обрабатывает оба типа редиректов.

3.3. Миграция слотов (resharding):
  - При добавлении или удалении узлов слоты перераспределяются.
  - Во время миграции ключи перемещаются между узлами.
  - Клиенты могут получать ASK-редиректы, пока миграция не завершится.
  - Процесс управляется командой CLUSTER SETSLOT.

4. ОГРАНИЧЕНИЯ CLUSTER (ВАЖНО ДЛЯ ПРОЕКТИРОВАНИЯ)

4.1. Многоключевые операции:
  - Команды, работающие с несколькими ключами (MGET, MSET, DEL, UNLINK, RPOPLPUSH и др.),
    работают только если все ключи принадлежат одному слоту.
  - Если ключи из разных слотов, Redis вернёт ошибку "CROSSSLOT".
  - Исключение: некоторые команды, такие как MGET, могут работать с ключами из одного слота.

4.2. Транзакции (MULTI/EXEC):
  - Все команды в транзакции должны относиться к одному слоту.
  - Ошибка "CROSSSLOT" при попытке смешивания слотов.

4.3. Lua-скрипты:
  - Скрипты могут обращаться только к ключам из одного слота.
  - Если скрипт обращается к разным слотам, Redis вернёт ошибку.
  - Обход: используйте хеш-теги для всех ключей в скрипте.

4.4. Выборка ключей (KEYS, SCAN):
  - KEYS не поддерживается в кластере (возвращает ошибку).
  - SCAN работает только на одном узле (не сканирует весь кластер).
  - Для сканирования всех ключей нужно выполнять SCAN на каждом мастер-узле отдельно.

4.5. Горячие ключи:
  - Если один ключ очень популярен (например, счетчик), весь трафик идёт на один узел.
  - Решение: использовать хеш-теги с суффиксами (например, `counter:{user:123}:1`, `counter:{user:123}:2`)
    и распределять нагрузку через приложение.

5. ОТКАЗОУСТОЙЧИВОСТЬ (FAILOVER)

5.1. Обнаружение сбоя:
  - Узлы обмениваются PING/PONG сообщениями через gossip-протокол.
  - Если узел не отвечает в течение cluster-node-timeout (по умолчанию 15 сек),
    он помечается как "недоступный" (PFAIL).
  - Когда другой узел подтверждает недоступность (через gossip), состояние становится FAIL.

5.2. Выбор нового мастера:
  - Когда мастер переходит в состояние FAIL, его реплики начинают выборы.
  - Реплика с наибольшим смещением репликации (наиболее актуальная) становится мастером.
  - После выбора новый мастер рассылает обновлённую карту слотов.

5.3. Время failover:
  - Зависит от cluster-node-timeout. Обычно 15-30 секунд.
  - В это время слоты, принадлежащие упавшему мастеру, недоступны для записи.
  - Чтение может продолжаться с реплик (если они не упали).

5.4. Автоматическое переключение:
  - Происходит без внешнего вмешательства.
  - Клиенты получают MOVED-редиректы и обновляют карту.

6. СРАВНЕНИЕ CLUSTER И SENTINEL

┌──────────────────────────┬──────────────────────────┬──────────────────────────┐
│ ХАРАКТЕРИСТИКА           │ CLUSTER                  │ SENTINEL                 │
├──────────────────────────┼──────────────────────────┼──────────────────────────┤
│ Масштабирование          │ Горизонтальное (шарды)   │ Вертикал(один мастер)    │
│ Максимальный объём       │ Неограничен (добавление  │ Ограничен памятью мастера│
│                          │ узлов)                   │                          │
│ Автоматический failover  │ Встроенный               │ Через Sentinel           │
│ Транзакции               │ Только в пределах слота  │ Работают на всём наборе  │
│ Сложность настройки      │ Высокая                  │ Средняя                  │
│ Поддержка клиентов       │ Широкая                  │ Широкая                  │
│ Когда использовать       │ > 100 ГБ, высокая        │ До 100 ГБ, простота      │
│                          │ нагрузка, шардирование   │ важнее масштабирования   │
└──────────────────────────┴──────────────────────────┴──────────────────────────┘

7. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

 1. Минимум 3 мастера и 3 реплики (6 узлов) для production.
 2. Размещайте узлы на разных физических хостах (или хотя бы разных зонах доступности).
 3. Настройте cluster-node-timeout адекватно (обычно 10-20 секунд) — слишком маленькое
    значение может вызвать ложные переключения.
 4. Включайте cluster-require-full-coverage yes, чтобы кластер не принимал запросы,
    если не все слоты покрыты (защита от потери данных при частичном отказе).
 5. Используйте хеш-теги для группировки связанных ключей.
 6. Для чтения с реплик включите ReadOnly в клиенте (разгружает мастер).
 7. Мониторьте состояние кластера через CLUSTER INFO, CLUSTER NODES.
 8. Периодически проверяйте равномерность распределения слотов.
 9. При добавлении узлов делайте решардинг в часы низкой нагрузки.
 10. Используйте клиентские библиотеки с поддержкой редиректов (go-redis поддерживает).

8. СВЯЗЬ С GO (go-redis)

8.1. Создание клиента:

	client := redis.NewClusterClient(&redis.ClusterOptions{
	    Addrs:        []string{"localhost:7000", "localhost:7001", "localhost:7002"},
	    Password:     "",
	    PoolSize:     10,
	    MaxRetries:   3,
	    ReadOnly:     false, // true — для чтения с реплик
	})

8.2. Особенности go-redis для кластера:
  - Автоматическое вычисление слота для каждого ключа.
  - Кэширование карты слотов (обновляется при MOVED).
  - Автоматическая обработка ASK-редиректов.
  - Поддержка хеш-тегов.
  - Транзакции (MULTI/EXEC) работают только в пределах одного слота.
  - Для многоключевых операций ключи должны быть в одном слоте.

8.3. Чтение с реплик:
  - Установите ReadOnly: true.
  - Клиент будет направлять GET-запросы на реплики, если они доступны.
  - Запись всегда идёт на мастер.

8.4. Обработка ошибок:
  - При MOVED-редиректе клиент обновляет карту и повторяет запрос.
  - При ASK-редиректе клиент перенаправляет запрос на указанный узел.
  - Все стандартные ошибки (таймаут, соединение) обрабатываются через механизмы retry.
*/
var (
	clusterClient *redis.ClusterClient
	ctx           = context.Background()
	logger        = log.Default()
)
var ClusterAddrs = []string{"localhost:7000", "localhost:7001", "localhost:7002"}

const (
	ClusterPassword = ""
	PoolSize        = 50
	MaxRetries      = 3
	ReadTimeout     = 5 * time.Second
	WriteTimeout    = 5 * time.Second
)

func init() {
	clusterClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           ClusterAddrs,
		Password:        ClusterPassword,
		PoolSize:        PoolSize,
		MinIdleConns:    10,
		MaxRetries:      MaxRetries,
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 2 * time.Second,
		ReadTimeout:     ReadTimeout,
		WriteTimeout:    WriteTimeout,
		PoolTimeout:     4 * time.Second,
		MaxRedirects:    5,
		ReadOnly:        false,
	})
}

func main() {
	fmt.Println("=== REDIS CLUSTER: ПРОДАКШН-РЕШЕНИЯ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. РАСПРЕДЕЛЁННЫЙ СЧЁТЧИК С АТОМАРНЫМ ИНКРЕМЕНТОМ И ЛИМИТОМ
type RateLimiter struct {
	client *redis.ClusterClient
	key    string
	limit  int64
	window time.Duration
}

func (rl *RateLimiter) allow(ctx context.Context) (bool, error) {
	// Атомарный инкремент
	count, err := rl.client.Incr(ctx, rl.key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		// Устанавливаем TTL, если ключ новый
		if err := rl.client.Expire(ctx, rl.key, rl.window).Err(); err != nil {
			// Откатываем инкремент при ошибке TTL
			rl.client.Decr(ctx, rl.key)
			return false, err
		}
	}
	if count > rl.limit {
		// Откатываем, чтобы не засорять память
		rl.client.Decr(ctx, rl.key)
		return false, nil
	}
	return true, nil
}

func primer1() {
	fmt.Println("--- 1. Распределённый счётчик (лимит запросов) ---")

	rl := &RateLimiter{
		client: clusterClient,
		key:    "rate:user:123",
		limit:  10,
		window: time.Minute,
	}

	allow, err := rl.allow(ctx)
	if err != nil {
		fmt.Printf("Ошибка проверки лимита: %v\n", err)
		return
	}
	if allow {
		fmt.Println("Запрос разрешён")
	} else {
		fmt.Println("Лимит превышен")
	}
}

// 2. КЭШИРОВАНИЕ С АВТОМАТИЧЕСКОЙ ИНВАЛИДАЦИЕЙ И ОБНОВЛЕНИЕМ
type UserCache struct {
	client  *redis.ClusterClient
	baseTTL time.Duration
	hotTTL  time.Duration
	hotHits int64
	mu      sync.Mutex
}

func (c *UserCache) invalidate(ctx context.Context, key string) {
	c.client.Del(ctx, key)
}

func (c *UserCache) get(ctx context.Context, key string, loader func(string) string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == nil {
		// Увеличиваем счётчик хитов для адаптивного TTL
		c.mu.Lock()
		c.hotHits++
		hits := c.hotHits
		c.mu.Unlock()

		// Если ключ горячий, продлеваем TTL
		if hits > 1000 {
			c.client.Expire(ctx, key, c.hotTTL)
		} else {
			c.client.Expire(ctx, key, c.baseTTL)
		}
		return val, nil
	}
	if !errors.Is(err, redis.Nil) {
		return "", err
	}
	// Кэш-промах: загружаем из БД
	data := loader("42")
	err = c.client.Set(ctx, key, data, c.baseTTL).Err()
	if err != nil {
		return "", err
	}
	return data, nil
}

func primer2() {
	fmt.Println("--- 2. Кэширование с инвалидацией при обновлении ---")

	cache := &UserCache{
		client:  clusterClient,
		baseTTL: 5 * time.Minute,
		hotTTL:  30 * time.Minute,
	}

	userID := "42"
	key := fmt.Sprintf("{user:%s}.profile", userID)

	// Функция загрузки из БД (имитация)
	loadFromDB := func(id string) string {
		return fmt.Sprintf(`{"id":%s,"name":"John","email":"john@example.com"}`, id)
	}

	// Получение с кэшем
	val, err := cache.get(ctx, key, loadFromDB)
	if err != nil {
		fmt.Printf("Ошибка получения: %v\n", err)
	} else {
		fmt.Printf("Данные пользователя: %s\n", val)
	}

	// Обновление пользователя — инвалидация
	cache.invalidate(ctx, key)
	fmt.Println("Кэш инвалидирован")
}

// 3. УПРАВЛЕНИЕ СЕССИЯМИ С ХЕШ-ТЕГАМИ (ГАРАНТИЯ СОГЛАСОВАННОСТИ)
func primer3() {
	fmt.Println("--- 3. Управление сессиями с хеш-тегами ---")

	type SessionManager struct {
		client *redis.ClusterClient
	}

	sm := &SessionManager{client: clusterClient}

	sessionID := "abc123"
	userID := "user:452"

	// Создаём сессию с хеш-тегом для группировки данных пользователя
	prefix := fmt.Sprintf("{%s}", userID)
	sessionKey := prefix + ".session" + sessionID
	userKey := prefix + ".user"

	// Сохраняем сессию и данные пользователя в одном слоте
	err := sm.client.Watch(ctx, func(tx *redis.Tx) error {
		pipe := tx.Pipeline()
		pipe.HSet(ctx, sessionKey,
			"user_id", userID,
			"created", time.Now().Unix(),
			"expires", time.Now().Add(1*time.Hour).Unix(),
		)
		pipe.HSet(ctx, userKey,
			"name", "Alice",
			"email", "alice@example.com",
		)
		pipe.Expire(ctx, sessionKey, time.Hour)
		pipe.Expire(ctx, userKey, time.Hour)
		_, err := pipe.Exec(ctx)
		return err
	}, sessionKey, sessionID)

	if err != nil {
		fmt.Printf("Ошибка создания сессии: %v\n", err)
	} else {
		fmt.Println("Сессия создана с группировкой в одном слоте")
	}

	// Получаем сессию
	sessionData, _ := sm.client.HGetAll(ctx, sessionKey).Result()
	fmt.Printf("Данные сессии: %+v\n", sessionData)

	// Очистка
	sm.client.Del(ctx, sessionKey, userKey)
}

// 4. ОБРАБОТКА ОЧЕРЕДИ ЗАДАЧ С РАСПРЕДЕЛЕНИЕМ ПО СЛОТАМ
func primer4() {
	fmt.Println("--- 4. Очередь задач с группировкой по пользователям ---")

	type TaskQueue struct {
		client *redis.ClusterClient
	}

	tq := &TaskQueue{client: clusterClient}

	// Добавляем задачи для разных пользователей (группируем по хеш-тегам)
	users := []string{"alice", "bob", "charlie"}
	for _, u := range users {
		key := fmt.Sprintf("{%s}:tasks", u)
		for i := 0; i < 5; i++ {
			task := fmt.Sprintf("task:%d", i)
			tq.client.LPush(ctx, key, task)
		}
		fmt.Printf("Добавлено 5 задач для %s\n", u)
	}
	// Обработчик — забирает задачи для конкретного пользователя
	processUserTasks := func(user string) {
		key := fmt.Sprintf("{%s}:tasks", user)
		for {
			task, err := tq.client.LPop(ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				break
			}
			if err != nil {
				logger.Printf("Ошибка получения задачи для %s: %v", user, err)
				break
			}
			fmt.Printf("  Обработка задачи %s для %s\n", task, user)
			// Имитация обработки
			time.Sleep(50 * time.Millisecond)
		}
	}
	// Обрабатываем задачи для каждого пользователя
	for _, u := range users {
		processUserTasks(u)
	}

	// Очистка
	for _, u := range users {
		tq.client.Del(ctx, fmt.Sprintf("{%s}:tasks", u))
	}
}

// 5. МОНИТОРИНГ И АЛЕРТИНГ НА ОСНОВЕ МЕТРИК КЛАСТЕРА
func primer5() {
	fmt.Println("--- 5. Мониторинг кластера ---")

	type ClusterMonitor struct {
		client *redis.ClusterClient
	}

	cm := &ClusterMonitor{client: clusterClient}

	// Получаем информацию о кластере
	nodesInfo, err := cm.client.Do(ctx, "CLUSTER", "NODES").Result()
	if err != nil {
		fmt.Printf("Ошибка получения узлов: %v\n", err)
		return
	}
	nodesStr, _ := nodesInfo.(string)
	fmt.Printf("Узлы кластера:\n%s\n", nodesStr)

	// Более структурированно через CLUSTER SLOTS
	slotsInfo, err := cm.client.Do(ctx, "CLUSTER", "SLOTS").Result()
	if err != nil {
		fmt.Printf("Ошибка получения слотов: %v\n", err)
	} else {
		slots, _ := slotsInfo.([]interface{})
		fmt.Printf("Всего мастер-узлов: %d\n", len(slots))
	}

	// Проверяем использование памяти и состояние
	info, err := cm.client.InfoMap(ctx, "memory").Result()
	if err != nil {
		fmt.Printf("Ошибка INFO memory: %v\n", err)
	} else {
		used, _ := strconv.ParseInt(info["memory"]["used_memory"], 10, 64)
		max, _ := strconv.ParseInt(info["memory"]["maxmemory"], 10, 64)
		if max > 0 {
			percent := float64(used) / float64(max) * 100
			fmt.Printf("Использование памяти: %.2f%% (%d / %d MB)\n", percent, used/(1024*1024), max/(1024*1024))
			if percent > 80 {
				fmt.Println("⚠️  КРИТИЧЕСКОЕ: память > 80%")
			}
		}
	}
	fmt.Println()
}

// 6. GRACEFUL SHUTDOWN С СОХРАНЕНИЕМ СОСТОЯНИЯ
func primer6() {
	fmt.Println("--- 6. Graceful shutdown ---")

	// Эмуляция завершения работы
	shutdown := func() {
		logger.Println("Получен сигнал завершения, закрываем клиенты...")
		if err := clusterClient.Close(); err != nil {
			logger.Printf("Ошибка закрытия кластерного клиента: %v", err)
		} else {
			logger.Println("Клиент закрыт успешно")
		}
	}
	// В реальном приложении вызывается при сигнале SIGTERM/SIGINT
	shutdown()
}

// 7. ПАКЕТНАЯ ОБРАБОТКА С АВТОМАТИЧЕСКИМ РЕТРАЕМ ПРИ MOVED/ASK
func primer7() {
	fmt.Println("--- 7. Пакетная обработка с ретраем при редиректах ---")

	type BatchProcessor struct {
		client *redis.ClusterClient
	}

	bp := &BatchProcessor{client: clusterClient}

	// Группируем ключи по слотам с помощью хеш-тегов
	keys := []string{
		"{user:1}.name",
		"{user:1}.age",
		"{user:1}.email",
		"{user:2}.name",
		"{user:2}.age",
	}

	// Функция для выполнения пакетной операции с ретраями
	processBatch := func(keys []string, operation func([]string) error) error {
		var lastErr error
		for attempt := 0; attempt <= MaxRetries; attempt++ {
			err := operation(keys)
			if err == nil {
				return nil
			}
			lastErr = err
			// Если ошибка связана с редиректом, ждём и повторяем
			if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
				continue
			}
			// Для других ошибок пробуем ещё
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("исчерпаны попытки: %w", lastErr)
	}

	// Пример: пакетное чтение (MGET) — все ключи в одном слоте или разные?
	// В кластере MGET работает только если все ключи в одном слоте.
	// Для разных слотов нужно разбивать.
	err := processBatch(keys, func(keys []string) error {
		vals, err := bp.client.MGet(ctx, keys...).Result()
		if err != nil {
			return err
		}
		fmt.Printf("Результат MGET: %v\n", vals)
		return nil
	})
	if err != nil {
		fmt.Printf("Ошибка пакетной операции: %v\n", err)
	}
}

// 8. РАСПРЕДЕЛЁННАЯ БЛОКИРОВКА В КЛАСТЕРЕ
type ClusterLock struct {
	client *redis.ClusterClient
	key    string
	ttl    time.Duration
	value  string
}

func primer8() {
	fmt.Println("--- 8. Распределённая блокировка в кластере ---")
	lock := &ClusterLock{
		client: clusterClient,
		key:    "{lock}.resource",
		ttl:    10 * time.Second,
		value:  "client-1",
	}

	// Захват блокировки
	acquired, err := lock.Acquire(ctx)
	if err != nil {
		fmt.Printf("Ошибка захвата блокировки: %v\n", err)
		return
	}
	if !acquired {
		fmt.Println("Не удалось захватить блокировку (занята)")
		return
	}
	fmt.Println("Блокировка захвачена")

	// Имитация работы
	time.Sleep(2 * time.Second)

	// Освобождение
	if err := lock.Release(ctx); err != nil {
		fmt.Printf("Ошибка освобождения: %v\n", err)
	} else {
		fmt.Println("✅ Блокировка освобождена")
	}
}

func (l *ClusterLock) Acquire(ctx context.Context) (bool, error) {
	// Используем SET NX EX для атомарного захвата
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (l *ClusterLock) Release(ctx context.Context) error {
	// Безопасное освобождение: проверяем, что блокировка наша
	val, err := l.client.Get(ctx, l.key).Result()
	if err != nil {
		return err
	}
	if val != l.value {
		return errors.New("блокировка принадлежит другому клиенту")
	}
	return l.client.Del(ctx, l.key).Err()
}
