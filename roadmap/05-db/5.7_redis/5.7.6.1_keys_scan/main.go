package keysscan

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 6.1.
0. ВВЕДЕНИЕ: ПОЧЕМУ KEYS — ЭТО ПЛОХО?

Redis — однопоточный сервер. Это означает, что любая команда, выполняющаяся
долго, блокирует все остальные операции. Команда KEYS — классический пример
такой «тяжёлой» операции. Когда вы выполняете KEYS pattern, Redis сканирует
все ключи (O(N) по количеству ключей) и возвращает все совпавшие.

Проблемы KEYS:
- Блокировка: во время выполнения KEYS Redis не отвечает на другие запросы.
- Длительность: при миллионе ключей может занять несколько секунд.
- Непредсказуемость: время выполнения растёт линейно с ростом ключей.
- Риск падения: при большом количестве ключей может вызвать таймауты клиентов
  и даже OOM (если возвращается слишком много ключей).

В производственных системах KEYS категорически запрещён.

1. SCAN — БЕЗОПАСНАЯ АЛЬТЕРНАТИВА

SCAN — это итератор, который возвращает ключи порциями (пачками). Он не блокирует
Redis надолго, так как каждая итерация обрабатывает только небольшую часть данных.

Синтаксис:
    SCAN cursor [MATCH pattern] [COUNT count]

Где:
- cursor — число, указывающее позицию в сканировании. Начинается с 0.
- MATCH — паттерн для фильтрации ключей (опционально).
- COUNT — подсказка серверу, сколько элементов вернуть (по умолчанию 10).

Команда возвращает два значения:
- Новый курсор (0 означает конец итерации).
- Массив ключей (может быть пустым).

2. ВНУТРЕННЕЕ УСТРОЙСТВО SCAN

SCAN использует хеш-таблицу Redis (словарь) для итерации. Каждый ключ хранится
в одном из множества бакетов (buckets). Итератор проходит по бакетам,
возвращая ключи из каждого. Курсор кодирует номер бакета и позицию внутри него.

Курсор не является простым числом — это битовая маска, которая изменяется
по алгоритму, гарантирующему, что каждый бакет будет посещён ровно один раз
при условии, что таблица не меняется.

Если во время итерации хеш-таблица изменяется (добавление/удаление ключей),
возможны ситуации:
- Ключ может быть возвращён дважды.
- Ключ может быть пропущен.
- Ключ может быть возвращён, даже если он был удалён.

Однако Redis гарантирует, что каждый ключ, который существовал непрерывно
с начала и до конца итерации, будет возвращён хотя бы один раз.

3. ГАРАНТИИ SCAN

3.1. Полнота: каждый ключ, существующий от начала до конца итерации,
     будет возвращён минимум один раз.
3.2. Уникальность: не гарантируется — ключи могут дублироваться.
3.3. Актуальность: ключи, добавленные во время итерации, могут быть
     возвращены или пропущены.
3.4. Порядок: не гарантируется.
3.5. Блокировка: не блокирует Redis на длительное время.
3.6. Атомарность: каждая итерация атомарна, но между итерациями могут
     выполняться другие команды.

4. СРАВНЕНИЕ ПРОИЗВОДИТЕЛЬНОСТИ

Параметр                  | KEYS                          | SCAN
--------------------------|-------------------------------|-------------------------------
Блокировка                | Полная (весь Redis)           | Минимальная (на время итерации)
Время выполнения          | O(N), пропорционально N       | O(N), но распределено по вызовам
Сетевой трафик            | 1 запрос, 1 ответ (много данных)| N запросов, N ответов (мало данных)
Нагрузка на CPU           | Высокая (вся сразу)           | Равномерная (распределённая)
Подходит для продакшна    | Нет                           | Да
Использование памяти      | Много (все ключи в ответе)    | Мало (порция ключей)

5. КОГДА ИСПОЛЬЗОВАТЬ SCAN

5.1. Удаление ключей по паттерну: SCAN + DEL (вместо KEYS + DEL).
5.2. Экспорт данных для бэкапа/миграции.
5.3. Подсчёт количества ключей с определённым префиксом.
5.4. Инвентаризация ключей (например, для мониторинга).
5.5. Очистка устаревших ключей с определённым префиксом.
5.6. Массовое обновление ключей (с пайплайном).

6. КОГДА KEYS ВСЁ ЖЕ МОЖЕТ БЫТЬ ДОПУСТИМ

KEYS допустим только в следующих случаях:
- Локальная разработка и отладка.
- Одноразовые скрипты на пустом/небольшом Redis (менее 10 000 ключей).
- В окружениях, где блокировка не критична (например, тестовый стенд).
В продакшне — никогда.

7. ВЫБОР ПАРАМЕТРА COUNT

COUNT — это подсказка серверу, сколько элементов вернуть. Redis не гарантирует
точное число, но старается приблизиться к нему.

Рекомендации:
- Для удаления: COUNT = 100–1000 (баланс между количеством итераций и размером пачки).
- Для экспорта: COUNT = 500–1000.
- Для больших ключей (большие значения): COUNT = 10–50 (чтобы не создавать нагрузку на память).
- Слишком маленький COUNT: увеличивает число итераций и RTT.
- Слишком большой COUNT: может создать пиковую нагрузку на память и CPU.

8. SCAN В КЛАСТЕРЕ

В Redis Cluster данные распределены по разным мастер-узлам. Команда SCAN
выполняется на одном узле и возвращает ключи только этого узла.
Чтобы обойти весь кластер, нужно:
1. Получить список мастер-узлов (CLUSTER NODES).
2. Для каждого узла запустить SCAN (подключившись напрямую к его адресу).
3. Объединить результаты.

В go-redis это можно сделать, получив адреса мастеров через ClusterClient
(или выполнив CLUSTER SLOTS) и создавая отдельные клиенты для каждого мастера.

9. SSCAN, HSCAN, ZSCAN

Эти команды аналогичны SCAN, но работают с конкретными структурами:
- SSCAN: итерация по элементам множества (Set).
- HSCAN: итерация по полям хеша (Hash).
- ZSCAN: итерация по элементам сортированного множества (ZSet).

Их синтаксис:
    SSCAN key cursor [MATCH pattern] [COUNT count]
    HSCAN key cursor [MATCH pattern] [COUNT count]
    ZSCAN key cursor [MATCH pattern] [COUNT count]

Возвращают:
- Новый курсор.
- Список элементов (для HSCAN — поля и значения чередуются).

10. ОБРАБОТКА ОШИБОК В SCAN

При использовании SCAN в Go важно обрабатывать ошибки:
- Если ошибка возвращается при вызове SCAN, нужно прервать итерацию.
- Если ошибка происходит в итераторе (iter.Err()), это может быть связано
  с контекстом (таймаут) или сетью.
- Используйте контекст для ограничения времени выполнения SCAN.

11. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ

1. Всегда используйте SCAN вместо KEYS в продакшне.
2. Для удаления ключей используйте SCAN + Pipeline для пакетного DEL.
3. Для экспорта данных используйте SCAN и обрабатывайте дубликаты на клиенте.
4. Устанавливайте COUNT разумно (100–500 для большинства случаев).
5. В кластере выполняйте SCAN на каждом мастер-узле отдельно.
6. Используйте итераторы go-redis для удобства (client.Scan().Iterator()).
7. Всегда проверяйте ошибки и используйте контекст.
8. Если нужно удалить все ключи с префиксом, рассмотрите использование
   Lua-скрипта с SCAN и DEL внутри (это атомарно и безблочно).
9. Мониторьте время выполнения SCAN и корректируйте COUNT при необходимости.

12. СВЯЗЬ С GO (go-redis)

Основные методы:
    // Итератор (удобно)
    iter := client.Scan(ctx, 0, "pattern:*", 100).Iterator()
    for iter.Next(ctx) {
        key := iter.Val()
        // обработать ключ
    }
    if err := iter.Err(); err != nil {
        // ошибка
    }

    // Ручное управление курсором
    var cursor uint64
    for {
        keys, nextCursor, err := client.Scan(ctx, cursor, "pattern:*", 100).Result()
        if err != nil {
            break
        }
        // обработать keys
        cursor = nextCursor
        if cursor == 0 {
            break
        }
    }

    // SSCAN
    iter := client.SScan(ctx, "set_key", 0, "pattern:*", 100).Iterator()
    // аналогично

    // HSCAN
    iter := client.HScan(ctx, "hash_key", 0, "pattern:*", 100).Iterator()
    // возвращает []string — поля и значения чередуются

    // ZSCAN
    iter := client.ZScan(ctx, "zset_key", 0, "pattern:*", 100).Iterator()

13. ТИПИЧНЫЕ ОШИБКИ И ИХ РЕШЕНИЯ

Использование KEYS в продакшне
   → Заменить на SCAN.

Слишком большой COUNT (например, 10000)
   → Уменьшить до 500–1000.

Неправильная обработка дубликатов
   → Использовать set/map для дедупликации, если это критично.

Игнорирование ошибок в итераторе
   → Всегда проверять iter.Err().

Бесконечный цикл (если курсор не меняется)
   → Всегда проверять, что курсор изменился или не равен предыдущему.

В кластере SCAN выполняется только на одном узле
   → Получить список мастеров и выполнить SCAN на каждом.

14. ИТОГИ
- KEYS — зло, запрещён в продакшне.
- SCAN — безопасная альтернатива для итерации по ключам.
- SCAN требует обработки дубликатов и возможных пропусков.
- Используйте итераторы go-redis для удобства.
- В кластере выполняйте SCAN отдельно на каждом мастер-узле.
- Правильный выбор COUNT важен для производительности.
*/

var (
	rdb    *redis.Client
	ctx    = context.Background()
	logger = log.Default()
)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     10,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

func main() {
	fmt.Println("=== KEYS vs SCAN ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. ДЕМОНСТРАЦИЯ ПРОБЛЕМЫ KEYS
func primer1() {
	fmt.Println("--- 1. KEYS vs SCAN: сравнение (KEYS блокирует!) ---")

	// Создаём 5000 ключей
	for i := 0; i < 5000; i++ {
		rdb.Set(ctx, fmt.Sprintf("test:%d", i), i, 0)
	}

	// KEYS (НЕ ИСПОЛЬЗОВАТЬ В ПРОДАКШНЕ!)
	start := time.Now()
	keys, err := rdb.Keys(ctx, "test:*").Result()
	if err != nil {
		logger.Printf("KEYS ошибка: %v", err)
	} else {
		fmt.Printf("KEYS: найдено %d ключей за %v\n", len(keys), time.Since(start))
	}

	// SCAN (безопасно)
	start = time.Now()
	var cursor uint64
	var scanKeys []string
	for {
		batch, nextCursor, err := rdb.Scan(ctx, cursor, "test:*", 100).Result()
		if err != nil {
			logger.Printf("SCAN ошибка: %v", err)
			break
		}
		scanKeys = append(scanKeys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	fmt.Printf("SCAN: найдено %d ключей за %v\n", len(scanKeys), time.Since(start))

	// Очистка
	rdb.Del(ctx, keys...)
}

// 2. УДАЛЕНИЕ КЛЮЧЕЙ ПО ПАТТЕРНУ (SCAN + PIPELINE)
func primer2() {
	fmt.Println("\n--- 2. Удаление ключей по паттерну (SCAN + Pipeline) ---")

	// Создаём 2000 ключей
	for i := 0; i < 2000; i++ {
		rdb.Set(ctx, fmt.Sprintf("session:user:%d", i), "active", 0)
	}

	pattern := "session:user:*"
	batchSize := 100
	var totalDeleted int64

	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, int64(batchSize)).Result()
		if err != nil {
			logger.Printf("SCAN ошибка: %v", err)
			break
		}
		if len(keys) > 0 {
			// Удаляем пачкой через Pipeline
			pipe := rdb.Pipeline()
			for _, key := range keys {
				pipe.Del(ctx, key)
			}
			cmds, err := pipe.Exec(ctx)
			if err != nil {
				logger.Printf("Pipeline ошибка: %v", err)
				// Частичный успех: проверяем отдельные команды
				for _, cmd := range cmds {
					if cmd.Err() != nil {
						logger.Printf("DEL ошибка: %v", cmd.Err())
					}
				}
			}
			// Считаем удалённые (по количеству команд)
			totalDeleted += int64(len(keys))
			fmt.Printf("Удалено %d ключей (пачка), всего %d\n", len(keys), totalDeleted)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	fmt.Printf("Итого удалено %d ключей\n", totalDeleted)
}

// 3. ЭКСПОРТ ДАННЫХ С ОБРАБОТКОЙ ДУБЛИКАТОВ
func primer3() {
	fmt.Println("\n--- 3. Экспорт данных (SCAN + дедупликация) ---")

	// Создаём тестовые ключи (некоторые дублируются? Нет, ключи уникальны, но мы эмулируем)
	for i := 0; i < 1500; i++ {
		rdb.Set(ctx, fmt.Sprintf("export:%d", i), fmt.Sprintf("value_%d", i), 0)
	}

	// Экспортируем все ключи и значения в map (дедупликация)
	exported := make(map[string]string) // ключ -> значение
	var mu sync.Mutex

	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "export:*", 200).Result()
		if err != nil {
			logger.Printf("SCAN ошибка: %v", err)
			break
		}
		if len(keys) > 0 {
			// Читаем значения через MGET (или Pipeline)
			pipe := rdb.Pipeline()
			for _, key := range keys {
				pipe.Get(ctx, key)
			}
			cmds, err := pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				logger.Printf("Pipeline GET ошибка: %v", err)
			}
			// Разбираем результаты
			for i, cmd := range cmds {
				if getCmd, ok := cmd.(*redis.StringCmd); ok {
					val, err := getCmd.Result()
					if err != nil {
						if errors.Is(err, redis.Nil) {
							continue // ключ мог быть удалён
						}
						logger.Printf("Ошибка GET для %s: %v", keys[i], err)
						continue
					}
					mu.Lock()
					exported[keys[i]] = val
					mu.Unlock()
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	fmt.Printf("Экспортировано %d уникальных ключей\n", len(exported))
	// Очистка
	rdb.Del(ctx, func() []string {
		keys, _ := rdb.Keys(ctx, "export:*").Result()
		return keys
	}()...)
}

// 4. АТОМАРНОЕ УДАЛЕНИЕ ПО ПАТТЕРНУ (LUA + SCAN)
func primer4() {
	fmt.Println("\n--- 4. Атомарное удаление по паттерну (Lua-скрипт) ---")

	// Создаём ключи
	for i := 0; i < 3000; i++ {
		rdb.Set(ctx, fmt.Sprintf("cache:item:%d", i), "data", 0)
	}

	script := redis.NewScript(`
	local pattern = KEYS[1]
	local batchSize = tonumber (ARGV[1])
	local cursor = '0'
	local deleted = 0

	repeat
		local result = redis.call("SCAN", cursor, 'MATCH', pattern, 'COUNT', batchSize)
		cursor = result[1]
		local keys = result[2]
		if #keys > 0 then	
			deleted = deleted + redis.call('DEL', unpack(keys))
		end
	until cursor == '0'

	return deleted
`)

	// Выполняем скрипт
	deleted, err := script.Run(ctx, rdb, []string{}, "cache:item:*", 100).Int()
	if err != nil {
		logger.Printf("Lua ошибка: %v", err)
	} else {
		fmt.Printf("Удалено %d ключей (атомарно)\n", deleted)
	}
}

// 5. SSCAN И HSCAN (ДЛЯ МНОЖЕСТВ И ХЕШЕЙ)
func primer5() {
	fmt.Println("\n--- 5. SSCAN (множество) и HSCAN (хеш) ---")

	// Множество: 100 элементов
	setKey := "myset"
	for i := 0; i < 100; i++ {
		rdb.SAdd(ctx, setKey, fmt.Sprintf("member_%d", i))
	}
	var countSet int
	iterScan := rdb.SScan(ctx, setKey, 0, "", 20).Iterator()
	for iterScan.Next(ctx) {
		countSet++
	}
	if err := iterScan.Err(); err != nil {
		logger.Printf("SSCAN ошибка: %v", err)
	}
	fmt.Printf("Множество: %d элементов\n", countSet)

	// Хеш: 100 полей
	hashKey := "myhash"
	for i := 0; i < 100; i++ {
		rdb.HSet(ctx, hashKey, fmt.Sprintf("field_%d", i), fmt.Sprintf("value_%d", i))
	}
	var countHash int
	iterHash := rdb.HScan(ctx, hashKey, 0, "", 20).Iterator()
	for iterHash.Next(ctx) {
		countHash++
	}
	if err := iterHash.Err(); err != nil {
		logger.Printf("HSCAN ошибка: %v", err)
	}
}

// 6. ПОДСЧЁТ КЛЮЧЕЙ С ПРЕФИКСОМ (МОНИТОРИНГ)
func primer6() {
	fmt.Println("\n--- 6. Подсчёт ключей с префиксом (мониторинг) ---")
	for i := 0; i < 500; i++ {
		if i%2 == 0 {
			rdb.Set(ctx, fmt.Sprintf("typeA:%d", i), i, 0)
		} else {
			rdb.Set(ctx, fmt.Sprintf("typeB:%d", i), i, 0)
		}
	}
	countKeys := func(pattern string) (int64, error) {
		var count int64
		iter := rdb.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			count++
		}
		if err := iter.Err(); err != nil {
			return 0, err
		}
		return count, nil
	}

	countA, _ := countKeys("typeA:*")
	countB, _ := countKeys("typeB:*")
	fmt.Printf("typeA: %d ключей, typeB: %d ключей\n", countA, countB)

	// Очистка
	keysA, _ := rdb.Keys(ctx, "typeA:*").Result()
	keysB, _ := rdb.Keys(ctx, "typeB:*").Result()
	rdb.Del(ctx, append(keysA, keysB...)...)
}

// 7. КОНКУРЕНТНЫЙ SCAN (НЕСКОЛЬКО ГОРУТИН)
func primer7() {
	fmt.Println("\n--- 7. Конкурентный обход ключей (несколько горутин) ---")

	for i := 0; i < 1000; i++ {
		rdb.Set(ctx, fmt.Sprintf("concurrent:%d", i), i, 0)
	}

	var wg sync.WaitGroup
	var totalProcessed int64
	var mu sync.Mutex

	// Обычно SCAN используется последовательно, но если нужно ускорить,
	// можно запустить несколько горутин, каждая со своим курсором.
	// Однако это требует координации, чтобы не обрабатывать одни и те же ключи дважды.
	// Для простоты покажем обработку с разделением по диапазонам (например, по хешу ключа),
	// но здесь используем одну горутину для SCAN (потому что SCAN сам итерирует).
	// Вместо этого демонстрируем обработку каждой пачки в отдельной горутине.

	worker := func(keys []string, wg *sync.WaitGroup) {
		defer wg.Done()
		for _, key := range keys {
			// Симуляция обработки
			mu.Lock()
			totalProcessed++
			fmt.Println(key)
			mu.Unlock()
		}
	}

	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "concurrent:*", 50).Result()
		if err != nil {
			logger.Printf("SCAN ошибка: %v", err)
			break
		}
		if len(keys) > 0 {
			wg.Add(1)
			go worker(keys, &wg)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	wg.Wait()
	fmt.Printf("Обработано %d ключей\n", totalProcessed)

	// Очистка
	keys, _ := rdb.Keys(ctx, "concurrent:*").Result()
	rdb.Del(ctx, keys...)
}

// 8. SCAN С ТАЙМАУТОМ (ИСПОЛЬЗОВАНИЕ КОНТЕКСТА)
func primer8() {
	fmt.Println("\n--- 8. SCAN с таймаутом через контекст ---")

	for i := 0; i < 500; i++ {
		rdb.Set(ctx, fmt.Sprintf("timeout:%d", i), i, 0)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	var count int
	iter := rdb.Scan(ctxTimeout, 0, "timeout:*", 100).Iterator()
	for iter.Next(ctxTimeout) {
		count++
		// Имитация медленной обработки (чтобы вызвать таймаут)
		if count%10 == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if err := iter.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("Таймаут итерации: превышено время ожидания")
		} else {
			logger.Printf("Ошибка: %v", err)
		}
	}
	fmt.Printf("Обработано %d ключей до таймаута\n", count)

	// Очистка
	keys, _ := rdb.Keys(ctx, "timeout:*").Result()
	rdb.Del(ctx, keys...)
}
