package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
0. ВВЕДЕНИЕ: ПОЧЕМУ БОЛЬШИЕ КЛЮЧИ — ЭТО ПРОБЛЕМА?

Redis — это in-memory база данных, и её производительность напрямую зависит
от эффективности использования памяти. Большие ключи (big keys) — это ключи,
которые занимают значительный объём памяти или содержат большое количество
элементов. Они создают множество проблем:

1. Блокировка Redis: операции, которые работают со всем ключом (HGETALL, SMEMBERS,
   LRANGE 0 -1), выполняются медленно и блокируют другие запросы (Redis однопоточный).
2. Увеличение задержек: даже простые операции (GET, DEL) на больших строках
   могут занимать больше времени из-за копирования данных.
3. Нагрузка на память и фрагментация: большие ключи могут вызывать фрагментацию
   памяти, увеличивать RSS и приводить к OOM.
4. Медленная репликация: передача больших ключей по сети замедляет синхронизацию.
5. Проблемы с бэкапами: RDB/AOF становятся больше, восстановление дольше.
6. Увеличение времени выполнения SCAN: при наличии больших ключей итератор
   может работать медленнее.

Понимание того, как Redis использует память, и умение находить и оптимизировать
большие ключи — обязательный навык для любого разработчика, работающего с Redis
в продакшн-окружении.

1. ВНУТРЕННЕЕ УСТРОЙСТВО ПАМЯТИ REDIS

1.1. Аллокатор памяти (jemalloc)
    - Redis использует jemalloc (по умолчанию) для управления памятью.
    - jemalloc выделяет память блоками (аренами) и уменьшает фрагментацию.
    - Уровни выделения: small, large, huge.
    - Команда INFO memory показывает used_memory (выделено jemalloc) и
      used_memory_rss (выделено ОС).
    - Фрагментация = used_memory_rss / used_memory. Норма 1.0–1.2.

1.2. Структура redisObject
    - Каждый ключ и значение представлены структурой redisObject (~24 байта).
    - Содержит: тип, кодировку, LRU, счётчик ссылок, указатель на данные.
    - Для маленьких строк используется кодировка embstr (объект и данные в одном блоке).
    - Для больших строк — raw (раздельное хранение).

1.3. Внутренние кодировки
    - ziplist: компактный список для небольших коллекций (list, hash, zset).
      Используется при числе элементов < hash-max-ziplist-entries (512) и
      размере элементов < hash-max-ziplist-value (64 байта).
    - intset: компактное множество целых чисел (set) при всех элементах типа int.
    - hashtable: стандартная хеш-таблица (для больших коллекций).
    - skiplist: для сортированных множеств (zset) при большом количестве элементов.

1.4. Overhead
    - Каждый ключ имеет накладные расходы (redisObject + ключ-строка).
    - Для коллекций добавляется overhead самой структуры (hashtable, ziplist и т.д.).
    - MEMORY USAGE учитывает всё это, что позволяет оценить реальное потребление.

2. КОМАНДЫ ДЛЯ АНАЛИЗА ПАМЯТИ (ПОДРОБНО)

2.1. MEMORY USAGE key [SAMPLES count]
    - Возвращает количество байт, занимаемых ключом (включая внутренние структуры).
    - SAMPLES — количество образцов для коллекций (по умолчанию 5).
      Больше образцов = точнее, но медленнее.
    - Для строк — точный размер. Для коллекций — приблизительный.
    - Важно: не учитывает внешнюю фрагментацию.
    - Пример:
        > SET mykey "hello"
        > MEMORY USAGE mykey
        (integer) 56
    - Для больших коллекций SAMPLES помогает оценить реальный размер,
      так как Redis может не хранить точную информацию о размере всех элементов.

2.2. MEMORY STATS
    - Возвращает детальную статистику внутренних аллокаций.
    - Основные поля:
        * peak.allocated — пиковое выделение памяти (байт).
        * total.allocated — общее выделение (байт).
        * startup.allocated — память при старте (байт).
        * replication — память для репликации.
        * clients.slaves, clients.normal — память клиентов.
        * cluster.links — память кластерных соединений.
        * db.0 — память для БД 0 (и для каждой БД отдельно).
        * overhead.* — служебные расходы (hashtable, keys, values и т.д.).
    - Используется для глубокого анализа, когда INFO memory не хватает.

2.3. INFO memory
    - Классическая команда, доступная всегда.
    - Ключевые поля:
        * used_memory — общее использование памяти (байт).
        * used_memory_human — в человекочитаемом виде.
        * used_memory_rss — память, выделенная ОС.
        * used_memory_peak — пик использования.
        * mem_fragmentation_ratio — коэффициент фрагментации.
        * maxmemory — лимит памяти (если установлен).
        * maxmemory_policy — политика вытеснения.
        * total_system_memory — общая память системы.
    - Мониторинг этих метрик позволяет вовремя заметить проблемы.

2.4. redis-cli --bigkeys
    - Встроенная утилита для сканирования всех ключей и поиска больших.
    - Использует SCAN для безопасного обхода.
    - Выводит топ-5 ключей по типу (string, list, hash, set, zset).
    - Пример: redis-cli --bigkeys
    - Важно: может создавать нагрузку при большом количестве ключей,
      но обычно безопасна.

2.5. MEMORY PURGE (редко используется)
    - Очищает буферы памяти (malloc_trim).
    - Может помочь снизить RSS при высокой фрагментации, но не рекомендуется для частого использования.

2.6. CONFIG SET activedefrag yes
    - Включает активную дефрагментацию (Redis 4.0+).
    - Автоматически перемещает объекты для уменьшения фрагментации.
    - Настройка: activedefrag yes, а также пороги (active-defrag-*).
    - Помогает снизить mem_fragmentation_ratio.

3. ИНСТРУМЕНТЫ ДЛЯ ПОИСКА БОЛЬШИХ КЛЮЧЕЙ

3.1. redis-cli --bigkeys
    - Самый простой способ, но даёт только общее представление.
    - Показывает топ-5 ключей по каждому типу.
    - Не показывает точные размеры строк (только количество элементов).

3.2. SCAN + MEMORY USAGE (программный способ)
    - Более гибкий и точный.
    - Можно написать скрипт на Go, Python, Shell.
    - Позволяет фильтровать по паттерну и порогу.
    - Даёт полный контроль над процессом.

3.3. Сторонние утилиты (redis-rdb-tools, rdb-tools)
    - Анализируют RDB-файл без подключения к работающему Redis.
    - Позволяют выгрузить метаданные о ключах.
    - Полезны для офлайн-анализа и миграций.

4. КАК ИНТЕРПРЕТИРОВАТЬ РЕЗУЛЬТАТЫ

4.1. Фрагментация (mem_fragmentation_ratio)
    - 1.0–1.2 — норма.
    - 1.2–1.5 — допустимо, но стоит обратить внимание.
    - > 1.5 — высокая фрагментация, рекомендуется включить activedefrag или перезапустить Redis.
    - Если ratio > 2.0 — возможны проблемы с памятью.

4.2. Overhead
    - MEMORY STATS показывает overhead.total.
    - Большой overhead может быть вызван большим количеством мелких ключей.
    - Оптимизация: объединять ключи в хеши.

4.3. Размер ключей
    - Для строк: размер данных + overhead (обычно 40–60 байт).
    - Для коллекций: overhead зависит от количества элементов и их размера.
    - MEMORY USAGE позволяет точно определить, сколько занимает каждый ключ.

4.4. Использование пиковой памяти (used_memory_peak)
    - Показывает максимальное использование за время жизни Redis.
    - Важно для планирования maxmemory.

5. СТРАТЕГИИ ОПТИМИЗАЦИИ

5.1. Разбивка больших ключей
    - Строки > 10 КБ: сжимайте (gzip, zstd) или храните внешне (S3).
    - Списки > 1000 элементов: разбивайте на части (чанки).
    - Хеши > 1000 полей: разделяйте по пространству (например, по диапазонам ID).
    - Множества > 1000 элементов: используйте битовые карты или HyperLogLog.

5.2. Выбор правильной структуры данных
    - Вместо строк с большим JSON → хеш (разбивка по полям).
    - Вместо списка для истории → Streams (если нужна хронология).
    - Вместо множества для подсчёта уникальных → HyperLogLog.
    - Вместо сортированного множества для рейтинга → используйте битовые карты, если скоры ограничены.

5.3. Использование компактных кодировок
    - Настройки: hash-max-ziplist-entries, hash-max-ziplist-value,
      list-max-ziplist-size, set-max-intset-entries, zset-max-ziplist-entries,
      zset-max-ziplist-value.
    - Если данные укладываются в пороги, Redis использует ziplist/intset,
      что экономит память.
    - Увеличение порогов может улучшить использование памяти, но повысить CPU при операциях.

5.4. Сжатие данных на клиенте
    - Для строк > 1 КБ используйте сжатие (gzip, zstd) перед записью.
    - Это уменьшает размер и снижает нагрузку на память, но требует CPU на клиенте.

5.5. TTL и удаление устаревших данных
    - Используйте EXPIRE для автоматической очистки.
    - Для больших ключей, которые не нужны, удаляйте их явно.

5.6. Мониторинг и алерты
    - Настройте проверку MEMORY USAGE для известных больших ключей.
    - Установите пороги (например, > 1 МБ) и отправляйте алерты.
    - Регулярно запускайте redis-cli --bigkeys для общего обзора.

6. МОНИТОРИНГ ПАМЯТИ В ПРОДАКШНЕ

6.1. Метрики для сбора
    - used_memory, used_memory_rss, used_memory_peak.
    - mem_fragmentation_ratio.
    - evicted_keys (сколько ключей вытеснено).
    - hit_rate (попадания в кэш).
    - Количество ключей по типам (INFO keyspace).

6.2. Алерты
    - used_memory > 80% от maxmemory.
    - mem_fragmentation_ratio > 1.5.
    - evicted_keys > 0 (значит память заполнена, идёт вытеснение).
    - Появление нового большого ключа (> порога).

6.3. Инструменты мониторинга
    - Prometheus + Grafana (с использованием redis_exporter).
    - ELK для логов SLOWLOG и анализа ключей.
    - Встроенный мониторинг в облачных провайдерах (AWS ElastiCache, GCP Memorystore).

7. АНАЛИЗ ПАМЯТИ С ПОМОЩЬЮ GO

7.1. Использование MemoryUsage
    - rdb.MemoryUsage(ctx, key).Result() — возвращает размер в байтах.
    - Комбинируйте с SCAN для поиска больших ключей.

7.2. Использование InfoMap
    - rdb.InfoMap(ctx, "memory").Result() — получает все метрики.
    - Парсите и используйте для мониторинга и алертов.

7.3. Использование MemoryStats (если доступно)
    - rdb.MemoryStats(ctx).Result() — даёт больше деталей, но не во всех версиях.

7.4. Практический паттерн:
    - Периодически (раз в час) запускайте SCAN + MemoryUsage для поиска ключей > порога.
    - Логируйте их и отправляйте в систему мониторинга.
    - При обнаружении критических ключей генерируйте алерт.

8. ТИПИЧНЫЕ ОШИБКИ И ИХ РЕШЕНИЯ

"Большие ключи — это нормально, если их мало."
Один большой ключ может блокировать Redis. Оптимизируйте даже один ключ.

"MEMORY USAGE показывает точный размер."
Для коллекций это приближение. Используйте SAMPLES для точности.

"Фрагментация — это не проблема."
Высокая фрагментация приводит к OOM, даже если used_memory ниже maxmemory.

"Я использую только строки, большие ключи мне не страшны."
Большие строки (JSON, бинарные данные) тоже могут быть проблемой.

"Я проверяю ключи только при запуске."
Данные растут. Нужен регулярный мониторинг.

"SCAN + MEMORY USAGE — слишком медленно."
Если у вас миллионы ключей, это действительно может быть дорого. Используйте redis-cli --bigkeys в периоды низкой нагрузки.

9. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

1. Установите порог для размера ключей (например, 1 МБ) и проверяйте ключи на превышение.
2. Используйте redis-cli --bigkeys еженедельно в непиковые часы.
3. Настройте activedefrag, если фрагментация > 1.2.
4. Включайте сжатие данных (на стороне клиента) для больших строк.
5. Используйте хеши для группировки мелких ключей (экономия overhead).
6. Настройте TTL для устаревающих данных.
7. Мониторьте used_memory и алертьте при > 80% maxmemory.
8. Проводите аудит ключей ежемесячно.
9. Используйте MEMORY STATS для глубокого анализа, если стандартных метрик не хватает.
10. В Go реализуйте периодическую проверку больших ключей и логирование.

10. ПРИМЕРЫ

- Кэш товаров интернет-магазина: отдельные ключи для каждого товара → большой хеш с полями для товаров (экономия памяти).
- История действий пользователя: список с 100k элементов → разбивка по месяцам.
- Сессии пользователей: JSON-строка с полями → хеш с отдельными полями (быстрее обновление отдельных атрибутов).
- Рейтинг игроков: сортированное множество с миллионами элементов → периодическая очистка устаревших игроков.

11. ИТОГИ

Анализ памяти и управление большими ключами — критически важные навыки для
поддержания стабильной работы Redis в продакшне. Правильное использование
команд MEMORY USAGE, MEMORY STATS, INFO memory, а также инструментов вроде
redis-cli --bigkeys, позволяет своевременно выявлять и решать проблемы.
Оптимизация данных (разбивка, сжатие, выбор правильных структур) помогает
снизить нагрузку на память и повысить производительность. Регулярный мониторинг
и автоматизация процессов обнаружения больших ключей обеспечивают долгосрочную
стабильность системы.
*/

var (
	rdb    *redis.Client
	ctx    = context.Background()
	logger = log.New(os.Stdout, "[REDIS-MEMORY] ", log.LstdFlags|log.Lshortfile)
)

const (
	// Порог для больших ключей (в байтах)
	BigKeyThreshold = 1024 * 1024 // 1 МБ
	// Размер пачки для SCAN
	ScanBatchSize = 100
)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     20,
		MinIdleConns: 5,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ПАРСИНГА
func parseBytes(s string) (int64, error) {
	var val int64
	fmt.Sscanf(s, "%d", &val)
	return val, nil
}
func parseFloat(s string) (float64, error) {
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val, nil
}

func main() {
	fmt.Println("=== АНАЛИЗ ПАМЯТИ ===\n")

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

// 1. БАЗОВОЕ СКАНИРОВАНИЕ БОЛЬШИХ КЛЮЧЕЙ С ОТЧЁТОМ
func primer1() {
	fmt.Println("--- 1. Сканирование больших ключей (SCAN + MEMORY USAGE) ---")

	// Создаём тестовые данные
	for i := 0; i < 100; i++ {
		rdb.Set(ctx, fmt.Sprintf("big:test:%d", i), string(make([]byte, 1024*10)), 0) // 10 КБ
	}
	for i := 0; i < 1000; i++ {
		rdb.Set(ctx, fmt.Sprintf("small:test:%d", i), "small", 0)
	}

	type KeyInfo struct {
		Key  string
		Size int64
		Type string
	}
	var bigKeys []KeyInfo

	// Используем SCAN с таймаутом
	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(scanCtx, cursor, "*", ScanBatchSize).Result()
		if err != nil {
			logger.Printf("SCAN ошибка: %v", err)
			break
		}
		for _, key := range keys {
			// Пропускаем служебные ключи, если нужно
			size, err := rdb.MemoryUsage(scanCtx, key).Result()
			if err != nil {
				continue
			}
			if size > BigKeyThreshold {
				typ, _ := rdb.Type(scanCtx, key).Result()
				bigKeys = append(bigKeys, KeyInfo{Key: key, Size: size, Type: typ})
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	// Сортируем по размеру
	sort.Slice(bigKeys, func(i, j int) bool { return bigKeys[i].Size > bigKeys[j].Size })
	fmt.Printf("Найдено %d больших ключей (> %d байт):\n", len(bigKeys), BigKeyThreshold)
	for i, k := range bigKeys {
		if i >= 10 {
			fmt.Printf("  ... и ещё %d ключей\n", len(bigKeys)-10)
			break
		}
		fmt.Printf("  %s [%s]: %d байт (%.2f КБ)\n", k.Key, k.Type, k.Size, float64(k.Size)/1024)
	}
	// Очистка
	rdb.Del(ctx, "big:test:*", "small:test:*")
}

// 2. АСИНХРОННЫЙ АНАЛИЗ С WORKER POOL (ДЛЯ БОЛЬШИХ ОБЪЁМОВ)
func primer2() {
	fmt.Println("\n--- 2. Асинхронный анализ с worker pool ---")

	// Создаём 5000 ключей
	for i := 0; i < 5000; i++ {
		rdb.Set(ctx, fmt.Sprintf("async:%d", i), string(make([]byte, 1024*2)), 0)
	}

	type Result struct {
		Key  string
		Size int64
		Type string
		Err  error
	}

	const numWorkers = 10
	jobs := make(chan string, 1000)
	results := make(chan Result, 1000)

	var wg sync.WaitGroup
	// Запускаем воркеры
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				size, err := rdb.MemoryUsage(ctx, key).Result()
				typ, _ := rdb.Type(ctx, key).Result()
				results <- Result{Key: key, Size: size, Type: typ, Err: err}
			}
		}()
	}

	// Заполняем канал с ключами через SCAN
	go func() {
		var cursor uint64
		for {
			keys, nextCursor, err := rdb.Scan(ctx, cursor, "async:*", 100).Result()
			if err != nil {
				logger.Printf("SCAN ошибка: %v", err)
				close(jobs)
				return
			}
			for _, k := range keys {
				jobs <- k
			}
			cursor = nextCursor
			if cursor == 0 {
				close(jobs)
				return
			}
		}
	}()
	// Ждём завершения воркеров и закрываем results
	go func() {
		wg.Wait()
		close(results)
	}()

	var bigKeys []Result
	for res := range results {
		if res.Err != nil {
			continue
		}
		if res.Size > 1024*1024 { // 1 МБ
			bigKeys = append(bigKeys, res)
		}
	}
	fmt.Printf("Асинхронно найдено %d больших ключей (>1 МБ)\n", len(bigKeys))
	rdb.Del(ctx, "async:*")
}

// 3. МОНИТОРИНГ ПАМЯТИ И ФРАГМЕНТАЦИИ С АЛЕРТАМИ
func primer3() {
	fmt.Println("\n--- 3. Мониторинг памяти и фрагментации ---")

	type MemoryMetrics struct {
		UsedMemory         int64
		UsedMemoryRSS      int64
		UsedMemoryPeak     int64
		FragmentationRatio float64
		EvictedKeys        int64
		MaxMemory          int64
		UsedMemoryPercent  float64
	}

	getMetrics := func() (*MemoryMetrics, error) {
		info, err := rdb.InfoMap(ctx, "memoty").Result()
		if err != nil {
			return nil, err
		}
		mem := info["memory"]
		var m MemoryMetrics
		m.UsedMemory, _ = parseBytes(mem["used_memory"])
		m.UsedMemoryRSS, _ = parseBytes(mem["used_memory_rss"])
		m.UsedMemoryPeak, _ = parseBytes(mem["used_memory_peak"])
		m.FragmentationRatio, _ = parseFloat(mem["mem_fragmentation_ratio"])
		m.EvictedKeys, _ = parseBytes(mem["evicted_keys"])
		m.MaxMemory, _ = parseBytes(mem["maxmemory"])
		if m.MaxMemory > 0 {
			m.UsedMemoryPercent = float64(m.UsedMemory) / float64(m.MaxMemory) * 100
		}
		return &m, nil
	}

	metrics, err := getMetrics()
	if err != nil {
		logger.Printf("Ошибка получения метрик: %v", err)
		return
	}
	fmt.Printf("used_memory: %d MB\n", metrics.UsedMemory/(1024*1024))
	fmt.Printf("used_memory_rss: %d MB\n", metrics.UsedMemoryRSS/(1024*1024))
	fmt.Printf("mem_fragmentation_ratio: %.2f\n", metrics.FragmentationRatio)
	fmt.Printf("evicted_keys: %d\n", metrics.EvictedKeys)

	// Алерты
	if metrics.FragmentationRatio > 1.5 {
		fmt.Println("ВЫСОКАЯ ФРАГМЕНТАЦИЯ! (>1.5)")
	}
	if metrics.UsedMemoryPercent > 80 {
		fmt.Printf("ПАМЯТЬ ПЕРЕПОЛНЕНА! Использовано %.1f%% от maxmemory\n", metrics.UsedMemoryPercent)
	}
}

// 4. ОПТИМИЗАЦИЯ БОЛЬШОГО ХЕША (РАЗБИВКА НА ЧАСТИ)
func primer4() {
	fmt.Println("\n--- 4. Оптимизация большого хеша (разбивка на чанки) ---")

	bigHashKey := "big:hash:optimize"
	// Создаём хеш с 10 000 полей
	for i := 0; i < 10000; i++ {
		rdb.HSet(ctx, bigHashKey, fmt.Sprintf("f_%d", i), fmt.Sprintf("v_%d", i))
	}
	sizeBefore, _ := rdb.MemoryUsage(ctx, bigHashKey).Result()
	fmt.Printf("Размер хеша до оптимизации: %d байт (%.2f КБ)\n", sizeBefore, float64(sizeBefore)/1024)

	// Разбиваем на чанки по 1000 полей
	const chunkSize = 1000
	totalFields, _ := rdb.HLen(ctx, bigHashKey).Result()
	chunkKeys := []string{}
	pipe := rdb.Pipeline()
	for i := int64(0); i < totalFields; i += chunkSize {
		chunkKey := fmt.Sprintf("big:hash:chunk:%d", i/chunkSize)
		chunkKeys = append(chunkKeys, chunkKey)
		// Получаем поля и значения (в реальности это делается через HSCAN)
		fields, err := rdb.HGetAll(ctx, bigHashKey).Result()
		if err != nil {
			logger.Printf("HGetAll ошибка: %v", err)
			return
		}
		// Вставляем в чанк (здесь упрощённо, в реальности используем HSCAN)
		for k, v := range fields {
			if i%chunkSize == 0 && i < int64(len(fields)) {
				pipe.HSet(ctx, chunkKey, k, v)
			}
		}
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Printf("Pipeline ошибка при создании чанков: %v", err)
	}
	rdb.Del(ctx, bigHashKey) // Удаляем старый ключ

	// Суммарный размер чанков
	var totalSize int64
	for _, ck := range chunkKeys {
		s, _ := rdb.MemoryUsage(ctx, ck).Result()
		totalSize += s
	}
	fmt.Printf("Суммарный размер чанков: %d байт (%.2f КБ)\n", totalSize, float64(totalSize)/1024)
	fmt.Println("Теперь операции над отдельными полями быстрее, и можно обрабатывать части независимо.")
	rdb.Del(ctx, chunkKeys...)
}

// 5. СЖАТИЕ БОЛЬШИХ СТРОК ПЕРЕД ЗАПИСЬЮ
func primer5() {
	fmt.Println("\n--- 5. Сжатие больших строк (gzip) ---")

	// Имитируем сжатие (используем простое сжатие, но в реальности можно использовать gzip/zstd)
	compress := func(data []byte) []byte {
		// В реальном проекте используйте compress/gzip или github.com/klauspost/compress/zstd
		// Для демонстрации просто возвращаем те же данные + метку
		return append([]byte("compressed:"), data...)
	}
	decompress := func(data []byte) []byte {
		return data[11:] // убираем префикс
	}

	originalData := string(make([]byte, 10*1024))
	key := "bif:string:compressed"

	// Записываем сжатые данные
	compressed := compress([]byte(originalData))
	err := rdb.Set(ctx, key, compressed, 0).Err()
	if err != nil {
		logger.Printf("Ошибка записи: %v", err)
		return
	}
	size, _ := rdb.MemoryUsage(ctx, key).Result()
	fmt.Printf("Размер сжатого ключа: %d байт (оригинал ~10 КБ)\n", size)

	// Читаем и распаковываем
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		logger.Printf("Ошибка чтения: %v", err)
	} else {
		restored := string(decompress([]byte(val)))
		fmt.Printf("Восстановлен размер: %d байт\n", len(restored))
	}
	rdb.Del(ctx, key)
}

// 6. ЭКСПОРТ ОТЧЁТА О БОЛЬШИХ КЛЮЧАХ В JSON
func primer6() {
	fmt.Println("\n--- 6. Экспорт отчёта о больших ключах в JSON ---")
	type KeyDetail struct {
		Key  string `json:"key"`
		Size int64  `json:"size_bytes"`
		Type string `json:"type"`
		TTL  int64  `json:"ttl_seconds"`
	}

	type Report struct {
		Timestamp string      `json:"timestamp"`
		Keys      []KeyDetail `json:"keys"`
	}

	// Создаём несколько ключей
	for i := 0; i < 5; i++ {
		rdb.Set(ctx, fmt.Sprintf("report:big:%d", i), string(make([]byte, 2*1024*1024)), 0)
	}
	for i := 0; i < 50; i++ {
		rdb.Set(ctx, fmt.Sprintf("report:small:%d", i), "small", 0)
	}

	var report Report
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)

	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "report:*", 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			size, _ := rdb.MemoryUsage(ctx, k).Result()
			if size > 1024*1024 { // >1 МБ
				typ, _ := rdb.Type(ctx, k).Result()
				ttl, _ := rdb.TTL(ctx, k).Result()
				report.Keys = append(report.Keys, KeyDetail{
					Key:  k,
					Size: size,
					Type: typ,
					TTL:  int64(ttl.Seconds()),
				})
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		logger.Printf("Ошибка маршалинга JSON: %v", err)
	} else {
		fmt.Printf("Отчёт экспортирован (JSON): %s\n", string(jsonData))
	}
	rdb.Del(ctx, "report:*")
}

// 7. ИНТЕГРАЦИЯ С МЕТРИКАМИ (ИМИТАЦИЯ PROMETHEUS)
func primer7() {
	fmt.Println("\n--- 7. Экспорт метрик для Prometheus (имитация) ---")

	// Собираем метрики в структуру
	type Metrics struct {
		UsedMemoryMB       float64
		UsedMemoryRSSMB    float64
		FragmentationRatio float64
		EvictedKeys        int64
		TotalKeys          int64
	}

	info, _ := rdb.InfoMap(ctx, "memory").Result()
	mem := info["memory"]
	used, _ := parseBytes(mem["used_memory"])
	rss, _ := parseBytes(mem["used_memory_rss"])
	evicted, _ := parseBytes(mem["evicted_keys"])
	frag, _ := parseFloat(mem["mem_fragmentation_ratio"])

	// Количество ключей (INFO keyspace)
	infoKeys, _ := rdb.InfoMap(ctx, "keyspace").Result()
	var totalKeys int64
	for _, v := range infoKeys["keyspace"] {
		// парсим строку вида "db0: keys=100, expires=10"
		var keys int64
		fmt.Sscanf(v, "keys=%d", &keys)
		totalKeys += keys
	}

	metrics := Metrics{
		UsedMemoryMB:       float64(used) / (1024 * 1024),
		UsedMemoryRSSMB:    float64(rss) / (1024 * 1024),
		FragmentationRatio: frag,
		EvictedKeys:        evicted,
		TotalKeys:          totalKeys,
	}
	fmt.Printf("Метрики для Prometheus:\n")
	fmt.Printf("  redis_used_memory_mb %f\n", metrics.UsedMemoryMB)
	fmt.Printf("  redis_used_memory_rss_mb %f\n", metrics.UsedMemoryRSSMB)
	fmt.Printf("  redis_mem_fragmentation_ratio %f\n", metrics.FragmentationRatio)
	fmt.Printf("  redis_evicted_keys_total %d\n", metrics.EvictedKeys)
	fmt.Printf("  redis_db_keys_total %d\n", metrics.TotalKeys)
}

// 8. АВТОМАТИЧЕСКАЯ ДЕФРАГМЕНТАЦИЯ И УПРАВЛЕНИЕ ФРАГМЕНТАЦИЕЙ
func primer8() {
	fmt.Println("\n--- 8. Управление фрагментацией (активная дефрагментация) ---")

	// Проверяем текущий ratio
	info, _ := rdb.InfoMap(ctx, "memory").Result()
	fragStr := info["memory"]["mem_fragmentation_ratio"]
	frag, _ := parseFloat(fragStr)
	fmt.Printf("Текущая фрагментация: %.2f\n", frag)

	if frag > 1.3 {
		fmt.Println("Фрагментация > 1.3, включаем активную дефрагментацию (если выключена)")
		err := rdb.ConfigSet(ctx, "activedefrag", "yes").Err()
		if err != nil {
			logger.Printf("Ошибка включения activedefrag: %v", err)
		} else {
			fmt.Println("✅ activedefrag включена")
		}
	} else {
		fmt.Println("Фрагментация в норме, активная дефрагментация не требуется.")
	}
}

// 9. МИГРАЦИЯ СТРОК В ХЕШИ ДЛЯ ЭКОНОМИИ ПАМЯТИ
func primer9() {
	fmt.Println("\n--- 9. Проверка ключей на превышение порога (алерт) ---")

	// Проверяем ключ, который потенциально может быть большим
	keysToCheck := []string{"user:profile:123", "cache:big:1"}
	for _, k := range keysToCheck {
		size, err := rdb.MemoryUsage(ctx, k).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			logger.Printf("Ошибка получения размера %s: %v", k, err)
			continue
		}
		if size > 10*1024*1024 { // 10 МБ
			// В реальном проекте здесь был бы вызов alerting (Slack, Email, PagerDuty)
			fmt.Printf("АЛЕРТ: Ключ %s превышает 10 МБ! (размер: %d байт)\n", k, size)
		} else {
			fmt.Printf("Ключ %s в норме: %d байт\n", k, size)
		}
	}
}
