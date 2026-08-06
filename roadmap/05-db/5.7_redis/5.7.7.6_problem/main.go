package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

/*
УРОК 7.6. ТИПИЧНЫЕ ПРОБЛЕМЫ REDIS И ИХ РЕШЕНИЯ
0. ВВЕДЕНИЕ: ПОЧЕМУ ВОЗНИКАЮТ ПРОБЛЕМЫ?

Redis — это высокопроизводительное in-memory хранилище, но в продакшн-окружении
оно сталкивается с множеством вызовов. Большинство проблем связано с
неправильным использованием команд, неэффективными структурами данных или
непредвиденной нагрузкой. Понимание этих проблем и умение их решать — ключевой
навык опытного разработчика.

В этом разделе мы разберём наиболее частые проблемы, с которыми вы столкнётесь
в реальных проектах, и покажем, как их детектировать, анализировать и исправлять.

1. ГОРЯЧИЕ КЛЮЧИ (HOT KEYS)

1.1. Что такое горячие ключи?
Горячие ключи — это ключи, которые запрашиваются или обновляются чрезвычайно часто
(тысячи или миллионы раз в секунду). Они создают дисбаланс нагрузки: весь трафик
идёт на один узел (в кластере) или перегружает один экземпляр Redis.

1.2. Как обнаружить горячие ключи?
- redis-cli --hotkeys (сканирует ключи и показывает самые часто запрашиваемые).
- Мониторинг latency: команда `redis-cli --latency` или встроенные инструменты.
- Анализ трафика: если вы видите, что один ключ генерирует много запросов в логах.
- Использование мониторинга Redis (например, Redis Insight, Grafana + Prometheus).
- В кластере: проверка неравномерного распределения запросов по узлам.

1.3. Решения для горячих ключей:
a) Клиентское кэширование (client-side caching):
   - Приложение хранит популярные данные в локальной памяти (например, map с TTL).
   - Это снижает нагрузку на Redis до 90% для горячих ключей.
   - Важно: использовать TTL для инвалидации, чтобы данные не устаревали.

б) Шардирование ключа (key sharding):
   - Разбить один горячий ключ на несколько (например, "user:1:0" ... "user:1:9").
   - Клиенты читают случайный или хешированный шард, распределяя нагрузку.
   - При обновлении нужно обновлять все шарды (или использовать брокер).

в) Использование реплик (replica reads):
   - Направить часть запросов на реплики, разгружая мастер.
   - Но нужно учитывать репликационную задержку (см. ниже).

г) Redis Cluster с хеш-тегами:
   - Если горячий ключ — это один слот, можно использовать хеш-теги для перераспределения.

2. БОЛЬШИЕ ЗНАЧЕНИЯ (BIG KEYS)

2.1. Что такое big keys?
Это ключи, которые занимают много памяти или содержат много элементов:
- Строка > 10 КБ.
- Список > 10 000 элементов.
- Хеш > 1000 полей.
- Множество > 1000 элементов.
- ZSET > 1000 элементов.

2.2. Почему big keys опасны?
- Операции чтения/записи становятся медленными (HGETALL, SMEMBERS, LRANGE).
- Удаление (DEL) может блокировать Redis на секунды.
- Увеличивают размер RDB/AOF и время восстановления.
- Вызывают фрагментацию памяти.
- В кластере: перемещение таких ключей при решардинге занимает много времени.

2.3. Как обнаружить big keys?
- redis-cli --bigkeys (сканирует все ключи и выводит топ по каждому типу).
- MEMORY USAGE key (размер конкретного ключа в байтах).
- SCAN + MEMORY USAGE (программный способ, как в примере 2).

2.4. Решения для big keys:
а) Разбивка (sharding):
   - Разбить большой ключ на несколько мелких (например, по диапазонам ID).
   - Для списков: разбить на части по 1000 элементов и хранить отдельные ключи.
   - Для хешей: разбить по полям (например, user:1:profile, user:1:settings).

б) Сжатие данных:
   - Перед записью сжимать данные (gzip, zstd) на клиенте.
   - Это уменьшает размер, но увеличивает CPU на клиенте.

в) Вынос данных из Redis:
   - Хранить большие данные во внешнем хранилище (S3, файловая система),
     а в Redis хранить только ссылку (URL, ключ).

г) Использование правильных структур:
   - Для объектов с множеством полей использовать хеши вместо строк с JSON.
   - Для множеств использовать битовые карты или HyperLogLog.

3. THUNDERING HERD (ЭФФЕКТ СТАИ)

3.1. Что это?
Эффект стаи возникает, когда множество клиентов одновременно запрашивают один
и тот же ключ в момент, когда кэш-промах (например, TTL истек). Все они бросаются
загружать данные из БД, создавая огромную нагрузку на бэкенд.

3.2. Почему это проблема?
- БД или внешний сервис перегружаются, что может привести к каскадному отказу.
- Запросы дублируются, увеличивая время ответа.
- В Redis создаётся большое количество одновременных запросов на один ключ.

3.3. Решения:
а) Singleflight (пакет golang.org/x/sync/singleflight):
   - Гарантирует, что только одна горутина выполнит загрузку данных из БД,
     остальные получат тот же результат (будут ждать).
   - Простая и эффективная реализация (см. пример 3).

б) Блокировка обновления через SET NX:
   - Первый клиент, который обнаружил кэш-промах, устанавливает блокировку
     на ключ (SET resource NX EX 5). Остальные ждут, пока блокировка не будет снята.
   - После обновления кэша блокировка удаляется.
   - Недостаток: нужно управлять временем блокировки и обработкой ошибок.

в) Использование "stale" (устаревших) данных:
   - При кэш-промахе возвращать старые данные (если они есть) и асинхронно обновлять кэш.
   - Клиенты не ждут обновления, что снижает нагрузку.

г) Предварительная загрузка (warm-up):
   - Заранее загружать горячие ключи при старте приложения.

4. РЕПЛИКАЦИОННАЯ ЗАДЕРЖКА (REPLICATION LAG)

4.1. Что это?
Репликационная задержка — это отставание реплики от мастера. Она возникает из-за
асинхронной природы репликации: мастер подтверждает запись клиенту до того,
как команда передана на реплику.

4.2. Как измерить задержку?
- INFO replication показывает master_repl_offset, slave_repl_offset и master_last_io_seconds_ago.
- Разница между offset и время последнего I/O дают представление о задержке.

4.3. Почему это проблема?
- Чтение с реплик может давать устаревшие данные.
- При падении мастера данные, не переданные на реплику, теряются.

4.4. Решения:
а) Чтение с мастера для критичных данных:
   - Для операций, где важна актуальность (баланс счёта), читать только с мастера.
   - Для некритичных (список товаров) — с реплик.

б) Использование WAIT:
   - Команда WAIT numreplicas timeout заставляет мастер ждать подтверждения от N реплик.
   - Это синхронизирует запись, но снижает производительность.

в) Оптимизация сети и оборудования:
   - Использовать быстрые сети и SSD для реплик.
   - Увеличить repl-backlog-size для уменьшения полных синхронизаций.

г) Использование Redis Cluster с репликами:
   - В кластере реплики автоматически синхронизируются, и задержка обычно меньше.

5. МЕДЛЕННЫЕ ЗАПРОСЫ (SLOW QUERIES)

5.1. Что это?
Команды, выполняющиеся дольше заданного порога (slowlog-log-slower-than).
Они блокируют Redis, увеличивая задержки для всех клиентов.

5.2. Как обнаружить?
- SLOWLOG GET N — получить N последних медленных команд.
- Мониторинг метрик: latency, CPU, количество запросов.
- Анализ INFO commandstats для поиска команд с высоким временем выполнения.

5.3. Типичные причины медленных запросов:
- Использование KEYS * (полный перебор).
- LRANGE с большим диапазоном (O(N)).
- HGETALL на большом хеше (O(N)).
- SMEMBERS на большом множестве.
- ZRANGE с большим диапазоном.
- DEL/UNLINK больших коллекций.
- Lua-скрипты, выполняющие много операций.

5.4. Решения:
а) Заменить KEYS на SCAN.
б) Ограничить размер возвращаемых данных (LRANGE с ограничением).
в) Для больших коллекций использовать SSCAN/HSCAN/ZSCAN.
г) Для удаления больших ключей использовать UNLINK (асинхронное удаление).
д) Оптимизировать Lua-скрипты (уменьшить количество операций).
е) Использовать пайплайн для массовых операций.
ж) При необходимости реструктурировать данные (разбивка).

6. KEYS vs SCAN

6.1. Проблема команды KEYS:
- Блокирует Redis на всё время выполнения.
- При миллионе ключей может занять секунды, вызывая таймауты.
- Возвращает все ключи сразу, что может привести к OOM.

6.2. Альтернатива — SCAN:
- Итеративный обход без блокировки.
- Возвращает порции ключей (пачки) и курсор для продолжения.
- Не блокирует, но может создавать нагрузку при большом количестве ключей.

6.3. Правильное использование SCAN:
- Всегда проверять курсор на 0 для завершения.
- Использовать COUNT для контроля размера пачки (например, 100-1000).
- Помнить, что SCAN может возвращать дубликаты (если данные меняются во время обхода).
- Для множеств, хешей и ZSET — SSCAN, HSCAN, ZSCAN.

7. ФРАГМЕНТАЦИЯ ПАМЯТИ (MEMORY FRAGMENTATION)

7.1. Что это?
Фрагментация возникает, когда память выделяется и освобождается множеством мелких
блоков, что приводит к неэффективному использованию. Redis использует jemalloc,
который может фрагментироваться при интенсивных операциях SET/DEL.

7.2. Как измерить?
- INFO memory показывает mem_fragmentation_ratio = used_memory_rss / used_memory.
- Норма: 1.0–1.2.
- > 1.5 — высокая фрагментация, стоит вмешаться.

7.3. Причины высокой фрагментации:
- Частые операции SET/DEL с переменным размером данных.
- Использование большого количества мелких ключей.
- Работа с большими значениями.

7.4. Решения:
а) Активная дефрагментация (Redis 4.0+):
   - CONFIG SET activedefrag yes
   - Настройки: active-defrag-threshold-lower, active-defrag-ignore-bytes, и др.
   - В фоне перемещает объекты, уменьшая фрагментацию.

б) Периодический перезапуск Redis (восстанавливает память).
в) Использование MEMORY PURGE (очистка буферов).
г) Оптимизация данных: уменьшение размера ключей, объединение мелких ключей в хеши.

8. OOM (OUT OF MEMORY) И ПРЕДОТВРАЩЕНИЕ

8.1. Что это?
Когда Redis использует всю доступную память (maxmemory) и не может выделить новую.
Результат — ошибки записи, падение приложения.

8.2. Как предотвратить?
а) Установить maxmemory и maxmemory-policy:
   - Выбрать политику вытеснения (allkeys-lru, volatile-lru и т.д.).
   - Следить за использованием памяти через INFO memory.

б) Мониторинг и алерты:
   - Установить порог > 80% для отправки алерта.
   - Использовать Prometheus + Grafana для визуализации.

в) Резервирование памяти:
   - Зарезервировать память для системы и других процессов.
   - Использовать контейнеры с ограничениями.

г) Fallback стратегии:
   - При ошибке записи в Redis использовать БД или другие хранилища.
   - В коде обрабатывать ошибки и делать ретраи с экспоненциальной задержкой.

9. ОБЩИЕ РЕКОМЕНДАЦИИ ПО МОНИТОРИНГУ

9.1. Что мониторить:
- использованную память (used_memory),
- пиковую память (used_memory_peak),
- фрагментацию (mem_fragmentation_ratio),
- количество вытесненных ключей (evicted_keys),
- задержки (latency),
- медленные запросы (SLOWLOG),
- размер очередей и пропускную способность.

9.2. Инструменты:
- Redis CLI (INFO, CONFIG, SLOWLOG).
- Redis Insight (графический интерфейс).
- Prometheus + redis_exporter.
- ELK / Loki для логов.
- Системы оповещения (PagerDuty, Slack).

10. ИТОГИ

Понимание типичных проблем Redis и умение их решать — это то, что отличает
опытного разработчика от новичка. Вы должны не только знать команды, но и
уметь диагностировать проблемы в реальном времени, анализировать метрики
и принимать решения по оптимизации. В этом разделе мы рассмотрели основные
проблемы и способы их решения. В реальном проекте вы будете сталкиваться
с ними постоянно, и теперь у вас есть теоретическая база для их устранения.
*/

var rdb *redis.Client
var ctx = context.Background()
var logger = log.New(os.Stdout, "[REDIS] ", log.LstdFlags|log.Lshortfile)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     20,
		MinIdleConns: 5,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

func keysPattern(pattern string, count int) []string {
	keys := make([]string, 0, count)
	iter := rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys
}

func main() {
	fmt.Println("=== REDIS: РЕШЕНИЕ ПРОБЛЕМ===\n")

	// Для демонстрации запускаем все примеры по очереди.
	// В реальном проекте каждый пример — отдельный модуль.
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
	primer9()
}

// 1. ГОРЯЧИЕ КЛЮЧИ (HOT KEYS) — КЛИЕНТСКОЕ КЭШИРОВАНИЕ + ШАРДИРОВАНИЕ
func primer1() {
	fmt.Println("--- 1. Горячие ключи (клиентский кэш + шардирование) ---")

	// Проблема: ключ "user:1" запрашивается 100k раз в секунду.
	// Решение: клиентский кэш с TTL + шардирование на 10 частей.

	// 1.1. Локальный кэш с TTL
	type LocalCache struct {
		mu     sync.RWMutex
		data   map[string]string
		expiry map[string]time.Time
		ttl    time.Duration
	}

	cache := &LocalCache{
		data:   make(map[string]string),
		expiry: make(map[string]time.Time),
		ttl:    1 * time.Second,
	}

	// 1.2. Шардирование: генерируем шард на основе хеша ключа
	shardCount := 10
	getShardKey := func(key string, shard int) string {
		return fmt.Sprintf("%s:%d", key, shard)
	}

	// 1.3. Функция получения данных (сначала локальный кэш, потом Redis)
	getHotData := func(key string) (string, error) {
		// Проверяем локальный кэш
		cache.mu.RLock()
		if val, ok := cache.data[key]; ok {
			if time.Now().Before(cache.expiry[key]) {
				cache.mu.RUnlock()
				logger.Printf("Горячий ключ %s из локального кэша", key)
				return val, nil
			}
		}
		cache.mu.RUnlock()

		// Загружаем из Redis (читаем с нескольких шардов и агрегируем)
		// В простом случае читаем с первого шарда, но для балансировки можно выбирать случайный.
		shard := int(time.Now().UnixNano()%int64(shardCount)) % shardCount
		shardKey := getShardKey(key, shard)
		val, err := rdb.Get(ctx, shardKey).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return "", err
		}
		// Если ключа нет, можно загрузить из БД и записать во все шарды.
		if errors.Is(err, redis.Nil) {
			logger.Printf("Кэш-промах для %s, загружаем из БД", key)
			val = "loaded_from_db" // имитация
			// Записываем во все шарды (для отказоустойчивости)
			for i := 0; i < shardCount; i++ {
				rdb.Set(ctx, getShardKey(key, i), val, 10*time.Minute)
			}
		}
		// Обновляем локальный кэш
		cache.mu.Lock()
		cache.data[key] = val
		cache.expiry[key] = time.Now().Add(cache.ttl)
		cache.mu.Unlock()

		logger.Printf("Горячий ключ %s загружен из Redis (шард %d)", key, shard)
		return val, nil
	}

	// Тестируем
	for i := 0; i < 5; i++ {
		val, _ := getHotData("user:1")
		fmt.Printf("Получено: %s\n", val)
		time.Sleep(200 * time.Millisecond)
	}
	// Очистка тестовых ключей
	for i := 0; i < shardCount; i++ {
		rdb.Del(ctx, getShardKey("user:1", i))
	}
	fmt.Println()
}

// 2. БОЛЬШИЕ ЗНАЧЕНИЯ (BIG KEYS) — ОБНАРУЖЕНИЕ И РАЗБИВКА
func primer2() {
	fmt.Println("--- 2. Большие значения (обнаружение и разбивка) ---")

	// 2.1. Создаём большой ключ (имитация)
	bigKey := "big:data"
	rdb.Set(ctx, bigKey, string(make([]byte, 10*1024*1024)), 0) // 10 МБ

	// 2.2. Обнаружение через MEMORY USAGE
	size, err := rdb.MemoryUsage(ctx, bigKey).Result()
	if err != nil {
		logger.Printf("Ошибка MEMORY USAGE: %v", err)
	} else {
		fmt.Printf("Размер ключа %s: %d байт (%.2f МБ)\n", bigKey, size, float64(size)/(1024*1024))
	}

	// 2.3. Решение: разбивка на чанки по 1 МБ
	chunkSize := 1 * 1024 * 1024
	data := string(make([]byte, 10*1024*1024))
	chunkCount := (len(data) + chunkSize - 1) / chunkSize
	for i := 0; i < chunkCount; i++ {
		chunkKey := fmt.Sprintf("big:chunk:%d", i)
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := rdb.Set(ctx, chunkKey, data[start:end], 0).Err(); err != nil {
			logger.Printf("Ошибка записи чанка %d: %v", i, err)
		}
	}
	rdb.Del(ctx, bigKey)
	fmt.Printf("Большой ключ разбит на %d чанков по 1 МБ\n", chunkCount)

	// 2.4. Чтение: собираем чанки
	var result []byte
	for i := 0; i < chunkCount; i++ {
		chunkKey := fmt.Sprintf("big:chunk:%d", i)
		chunk, err := rdb.Get(ctx, chunkKey).Result()
		if err != nil {
			logger.Printf("Ошибка чтения чанка %d: %v", i, err)
			continue
		}
		result = append(result, chunk...)
	}
	fmt.Printf("Собрано данных: %d байт\n", len(result))
	// Очистка
	for i := 0; i < chunkCount; i++ {
		rdb.Del(ctx, fmt.Sprintf("big:chunk:%d", i))
	}
	fmt.Println()
}

// 3. THUNDERING HERD — ЗАЩИТА ЧЕРЕЗ SINGLEFLIGHT
func primer3() {
	fmt.Println("--- 3. Thundering Herd (singleflight) ---")

	var sf singleflight.Group
	cacheKey := "cahce:thundering"
	rdb.Del(ctx, cacheKey)

	// 3.1. Имитация загрузки из БД (долго)
	loadFromDB := func() (string, error) {
		logger.Println("Загрузка из БД (долгая операция)...")
		time.Sleep(2 * time.Second)
		return "data_from_db", nil
	}

	// 3.2. Функция получения данных с защитой
	getData := func() (string, error) {
		// Проверяем кэш
		val, err := rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			return val, nil
		}
		if !errors.Is(err, redis.Nil) {
			return "", err
		}
		// Кэш-промах: используем singleflight
		result, err, _ := sf.Do(cacheKey, func() (any, error) {
			return loadFromDB()
		})
		if err != nil {
			return "", err
		}
		data := result.(string)
		_ = rdb.Set(ctx, cacheKey, data, time.Minute).Err()
		return data, nil
	}
	// 3.3. 10 одновременных запросов
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := getData()
			if err != nil {
				logger.Printf("Ошибка: %v", err)
			} else {
				fmt.Printf("Получено: %s\n", data)
			}
		}()
	}
	wg.Wait()
	rdb.Del(ctx, cacheKey)
	fmt.Println()
}

// 4. РЕПЛИКАЦИОННАЯ ЗАДЕРЖКА — СТРАТЕГИЯ ЧТЕНИЯ
func primer4() {
	fmt.Println("--- 4. Репликационная задержка (чтение с мастера/реплик) ---")
	// 4.1. Подключаемся к реплике (предполагаем, что она на порту 6380)
	replica := redis.NewClient(&redis.Options{Addr: "localhost:6380"})
	defer replica.Close()

	// 4.2. Функция определения, актуальны ли данные на реплике
	isReplicaToDate := func() bool {
		info, err := rdb.InfoMap(ctx, "replication").Result()
		if err != nil {
			return false
		}
		lag, _ := strconv.Atoi(info["replication"]["master_last_io_seconds_ago"])
		// Если задержка более 1 секунды, считаем, что реплика отстаёт
		return lag <= 1
	}
	// 4.3. Чтение с fallback на мастер
	readData := func(key string, critical bool) (string, error) {
		if !critical && isReplicaToDate() {
			// Некритичные данные и реплика актуальна — читаем с реплики
			val, err := replica.Get(ctx, key).Result()
			if err == nil || errors.Is(err, redis.Nil) {
				return val, nil
			}
			// Ошибка реплики — fallback на мастер
			logger.Printf("Реплика недоступна, fallback на мастер")
		}
		// В остальных случаях читаем с мастера
		return rdb.Get(ctx, key).Result()
	}

	// Тестим
	rdb.Set(ctx, "test:critical", "master_value", 0)
	val, _ := readData("test:critical", true)
	fmt.Printf("Критичные данные (с мастера): %s\n", val)

	val, _ = readData("test:noncritical", false)
	fmt.Printf("Некритичные данные (с реплики или мастера): %s\n", val)
	rdb.Del(ctx, "test:critical")
	fmt.Println()
}

// 5. МЕДЛЕННЫЕ ЗАПРОСЫ — SLOWLOG И ОПТИМИЗАЦИЯ
func primer5() {
	fmt.Println("--- 5. Медленные запросы (SLOWLOG и оптимизация) ---")

	// 5.1. Настраиваем SLOWLOG на 1 мс (только для демонстрации)
	_ = rdb.ConfigSet(ctx, "slowlog-log-skiver-then", "1000")

	// 5.2. Выполняем потенциально медленную команду (HGETALL большого хеша)
	bigHashKey := "slow:hash"
	for i := 0; i < 5000; i++ {
		rdb.HSet(ctx, bigHashKey, fmt.Sprintf("f_%d", i), fmt.Sprintf("v_%d", i))
	}

	// 5.3. Выполняем HGETALL (может попасть в SLOWLOG)
	start := time.Now()
	all, _ := rdb.HGetAll(ctx, bigHashKey).Result()
	fmt.Printf("HGETALL: %d полей, время: %v\n", len(all), time.Since(start))

	// 5.4. Читаем SLOWLOG
	entries, _ := rdb.SlowLogGet(ctx, 10).Result()
	fmt.Println("Последние записи SLOWLOG:")
	for _, e := range entries {
		if len(e.Args) > 0 && e.Args[0] == "HGETALL" {
			fmt.Printf("  Команда: %v, Время: %d мкс\n", e.Args, e.Duration)
		}
		// 5.5. Оптимизация: использовать HSCAN
		start := time.Now()
		var cursor uint64
		count := 0
		for {
			fields, nextCursor, err := rdb.HScan(ctx, bigHashKey, cursor, "", 100).Result()
			if err != nil {
				break
			}
			count += len(fields) / 2
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
		fmt.Printf("HSCAN: %d полей, время: %v (оптимально)\n", count, time.Since(start))
		rdb.Del(ctx, bigHashKey)
		_ = rdb.ConfigSet(ctx, "slowlog-log-slower-than", "10000") // вернуть по умолчанию
		fmt.Println()
	}
}

// 6. ФРАГМЕНТАЦИЯ ПАМЯТИ — МОНИТОРИНГ И АКТИВНАЯ ДЕФРАГМЕНТАЦИЯ
func primer6() {
	fmt.Println("--- 6. Фрагментация памяти (мониторинг) ---")

	info, err := rdb.InfoMap(ctx, "memory").Result()
	if err != nil {
		logger.Printf("Ошибка INFO memory: %v", err)
		return
	}
	mem := info["memory"]
	used := mem["used_memory"]
	rss := mem["used_memory_rss"]
	frag := mem["mem_fragmentation_ratio"]

	fmt.Printf("used_memory: %s, used_memory_rss: %s\n", used, rss)
	fmt.Printf("mem_fragmentation_ratio: %s\n", frag)

	fragFloat, _ := strconv.ParseFloat(frag, 64)
	if fragFloat > 1.5 {
		fmt.Println("Высокая фрагментация (>1.5). Применяем меры:")
		// Включаем активную дефрагментацию
		err := rdb.ConfigSet(ctx, "activedefrag", "yes").Err()
		if err != nil {
			logger.Printf("Ошибка включения activedefrag: %v", err)
		} else {
			fmt.Println("Активная дефрагментация включена.")
		}
		// Можно также вызвать MEMORY PURGE
		_ = rdb.Do(ctx, "MEMORY", "PURGE").Err()
		fmt.Println("MEMORY PURGE выполнен.")
	} else {
		fmt.Println("Фрагментация в норме.")
	}
	fmt.Println()
}

// 7. OOM ПРЕДОТВРАЩЕНИЕ — FALLBACK И МОНИТОРИНГ
func primer7() {
	fmt.Println("--- 7. OOM Prevention (fallback) ---")

	// Получаем maxmemory
	maxMemStr, err := rdb.ConfigGet(ctx, "maxmemory").Result()
	if err != nil {
		logger.Printf("Ошибка ConfigGet: %v", err)
		return
	}
	maxMemory, _ := strconv.ParseInt(maxMemStr["maxmemory"], 10, 64)
	if maxMemory == 0 {
		fmt.Println("maxmemory не установлен, пропускаем")
		return
	}

	// Получаем used_memory
	info, _ := rdb.InfoMap(ctx, "memory").Result()
	usedMemory, _ := strconv.ParseInt(info["memory"]["used_memory"], 10, 64)
	usagePercent := float64(usedMemory) / float64(maxMemory) * 100
	fmt.Printf("Использовано памяти: %.2f%%\n", usagePercent)

	// Fallback-функция: если память > 80%, использовать БД
	writeToRedis := func(key, value string) error {
		if usagePercent > 80 {
			logger.Printf("Память переполнена (%.2f%%), fallback на БД", usagePercent)
			return errors.New("fallback to DB")
		}
		return rdb.Set(ctx, key, value, 0).Err()
	}
	err = writeToRedis("fallback:test", "value")
	if err != nil {
		fmt.Printf("Запись отклонена (fallback): %v\n", err)
	} else {
		fmt.Println("Запись выполнена в Redis")
		rdb.Del(ctx, "fallback:test")
	}
	fmt.Println()
}

// 8. ИСТОЩЕНИЕ ПУЛА СОЕДИНЕНИЙ (CONNECTION POOL EXHAUSTION)
func primer8() {
	fmt.Println("--- 8. Истощение пула соединений (решение) ---")

	// Проблема: много горутин создают клиенты, не закрывая их.
	// Решение: использовать один клиент с настройками пула.

	// Настройки пула в init: PoolSize=20, MinIdleConns=5.
	// В этом примере мы просто покажем, как проверить количество активных соединений.

	// Получаем статистику клиентов
	info, err := rdb.InfoMap(ctx, "clients").Result()
	if err != nil {
		logger.Printf("Ошибка INFO clients: %v", err)
		return
	}
	connectedClients := info["clients"]["connected_clients"]
	fmt.Printf("Подключенных клиентов: %s\n", connectedClients)

	// Симуляция: создаём 1000 временных клиентов (плохая практика)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Плохо: создаём новый клиент на каждый запрос
			client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
			defer client.Close()
			_, _ = rdb.Ping(ctx).Result()
		}()
	}
	wg.Wait()

	// Проверяем снова
	info2, _ := rdb.InfoMap(ctx, "clients").Result()
	connectedClients2 := info2["clients"]["connected_clients"]
	fmt.Printf("Подключенных клиентов после 10 временных: %s\n", connectedClients2)
	fmt.Println("Рекомендация: используйте один клиент с пулом, не создавайте новые подключения на каждый запрос.")
	fmt.Println()
}

// 9. ОПТИМИЗАЦИЯ LUA-СКРИПТОВ — ПАРТИОННОЕ ВЫПОЛНЕНИЕ
func primer9() {
	fmt.Println("--- 10. Оптимизация Lua-скриптов (пакетная обработка) ---")

	// Проблема: Lua-скрипт, который обрабатывает слишком много ключей за раз.
	// Решение: разбить на пачки и использовать Pipeline.

	// Создаём 1000 ключей
	for i := 0; i < 1000; i++ {
		rdb.Set(ctx, fmt.Sprintf("lua:key:%d", i), i, 0)
	}

	// Плохо: один скрипт на 1000 ключей (может блокировать)
	badScript := redis.NewScript(`
		for i=1, #KEYS do
			redis.call('INCR', KEYS[i])
		end
		return 'done'
	`)
	start := time.Now()
	_, err := badScript.Run(ctx, rdb, keysPattern("lua:key:*", 1000)).Result()
	if err != nil {
		logger.Printf("Ошибка скрипта: %v", err)
	} else {
		fmt.Printf("Один скрипт на 1000 ключей: %v\n", time.Since(start))
	}

	// Хорошо: разбиваем на пачки по 100 ключей
	start = time.Now()
	batchSize := 100
	keys := keysPattern("lua:key:*", 1000)
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]
		// Можно использовать пайплайн вместо скрипта для простых операций
		pipe := rdb.Pipeline()
		for _, k := range batch {
			pipe.Incr(ctx, k)
		}
		_, err := pipe.Exec(ctx)
		if err != nil {
			logger.Printf("Ошибка пайплайна: %v", err)
		}
	}
	fmt.Printf("Пайплайн пачками по 100 ключей: %v (быстрее и не блокирует)\n", time.Since(start))

	// Очистка
	for _, k := range keys {
		rdb.Del(ctx, k)
	}
	fmt.Println()
}
