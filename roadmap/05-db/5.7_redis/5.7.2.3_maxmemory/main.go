package maxmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 2.3. MAXMEMORY OVERHEAD & INVALIDATION

ТЕОРИЯ: ЧТО ТАКОЕ OVERHEAD?

Когда вы думаете о памяти в Redis, вы, скорее всего, представляете только
сами данные: строку "Hello", число 100500, JSON-объект. Но на самом деле
Redis тратит память на МНОГО служебных структур, которые обеспечивают
работу базы данных. Это и есть overhead (накладные расходы).

Реальное потребление памяти = размер данных + overhead.

Overhead состоит из:
1. Структура ключа (redisObject) — ~16-24 байта на ключ.
2. Структура значения (redisObject) — ~16-24 байта на значение.
3. Срок жизни (TTL) — если установлен, хранится отдельно.
4. Внутренние указатели, ссылки, счётчики.
5. Хеш-таблицы, деревья, списки — внутренние структуры данных.
6. Фрагментация памяти (memory fragmentation).

Типичный overhead для маленьких ключей и значений может быть БОЛЬШЕ,
чем сами данные! Например, ключ длиной 5 байт может занимать 50-60 байт
в Redis из-за служебных структур.

1. СТРУКТУРА redisObject (ключевой объект)

Каждый ключ и значение в Redis представлены структурой redisObject:

    struct redisObject {
        unsigned type:4;        // тип данных (string, list, hash и т.д.)
        unsigned encoding:4;    // способ хранения (raw, int, ziplist и т.д.)
        unsigned lru:LRU_BITS;  // для LRU/LFU
        int refcount;           // счётчик ссылок (для совместного использования)
        void *ptr;              // указатель на фактические данные
    };

Размер: ~24 байта (зависит от архитектуры и компилятора).
Это ДЛЯ КАЖДОГО ключа и ДЛЯ КАЖДОГО значения.

Итого минимум: 48 байт на каждую пару ключ-значение + сами данные.

2. КАК РАЗНЫЕ ТИПЫ ДАННЫХ ВЛИЯЮТ НА OVERHEAD

┌────────────────┬────────────────────────────────────────────────────────────┐
│ ТИП ДАННЫХ     │ ОСОБЕННОСТИ OVERHEAD                                       │
├────────────────┼────────────────────────────────────────────────────────────┤
│ Строка (String)│ Самый простой случай. Данные хранятся как массив байт.     │
│                │ Overhead: ~24 байта (redisObject) + длина ключа + длина    │
│                │ значения.                                                  │
├────────────────┼────────────────────────────────────────────────────────────┤
│ Список (List)  │ Если элементов мало (< 512) и они маленькие —              │
│                │ используется ziplist (компактный). Если много —            │
│                │ linked list, где каждый узел имеет свой redisObject.       │
│                │ Overhead: каждый элемент — свой redisObject.               │
├────────────────┼────────────────────────────────────────────────────────────┤
│ Хеш (Hash)     │ Если полей мало (< 512) и они маленькие — ziplist.         │
│                │ Если много — хеш-таблица. Каждое поле и значение —         │
│                │ отдельные redisObject + overhead хеш-таблицы.              │
├────────────────┼────────────────────────────────────────────────────────────┤
│ Множество (Set)│ Аналогично хешу: intset для маленьких, хеш-таблица         │
│                │ для больших. Каждый элемент — redisObject.                 │
├────────────────┼────────────────────────────────────────────────────────────┤
│ Sorted Set     │ Самый "дорогой". Два представления: ziplist (маленькие)    │
│                │ или skiplist + хеш-таблица. Элементы имеют скор и член,    │
│                │ что увеличивает overhead.                                  │
└────────────────┴────────────────────────────────────────────────────────────┘

3. ЭНКОДИНГИ (способ хранения данных внутри)

Redis использует разные кодировки для оптимизации памяти:

┌─────────────────────┬────────────────────────────────────────────────────┐
│ ЭНКОДИНГ            │ ОПИСАНИЕ                                           │
├─────────────────────┼────────────────────────────────────────────────────┤
│ raw                 │ Простой массив байт (для строк > 44 байт)          │
├─────────────────────┼────────────────────────────────────────────────────┤
│ embstr              │ Строка, умещающаяся вместе с redisObject           │
│                     │ (< 44 байт) — экономит одно выделение памяти       │
├─────────────────────┼────────────────────────────────────────────────────┤
│ int                 │ Целое число (8, 16, 32, 64 бит) — компактно        │
├─────────────────────┼────────────────────────────────────────────────────┤
│ ziplist             │ Компактный список для маленьких элементов          │
│                     │ (list, hash, zset)                                 │
├─────────────────────┼────────────────────────────────────────────────────┤
│ intset              │ Компактный набор целых чисел (для множеств)        │
├─────────────────────┼────────────────────────────────────────────────────┤
│ hashtable           │ Полноценная хеш-таблица (для больших коллекций)    │
├─────────────────────┼────────────────────────────────────────────────────┤
│ skiplist + dict     │ Двойное представление для больших sorted set       │
└─────────────────────┴────────────────────────────────────────────────────┘

Важно: Redis автоматически переключает кодировку, когда размер коллекции
превышает порог (настраивается: hash-max-ziplist-entries и др.).

4. ФРАГМЕНТАЦИЯ ПАМЯТИ

Фрагментация — это ситуация, когда память разбита на маленькие занятые
и свободные куски. Redis выделяет память через jemalloc (или glibc malloc).
После многих операций SET/DEL память может фрагментироваться.

Фрагментация считается как:
    fragmentation = used_memory_rss / used_memory

Нормальное значение: 1.1–1.2 (10-20% overhead).
Высокое значение (> 1.5) — проблема, требует вмешательства.

Что делать при высокой фрагментации:
1. Перезапустить Redis (но это потеря данных).
2. Использовать CONFIG SET activedefrag yes (активная дефрагментация).
3. Использовать MEMORY PURGE (очистка буферов).

5. КОМАНДА MEMORY USAGE (как измерить overhead)

Команда MEMORY USAGE возвращает количество байт, которое занимает ключ
со всеми служебными структурами.

    MEMORY USAGE key [SAMPLES count]

Пример:
    127.0.0.1:6379> SET mykey "hello"
    127.0.0.1:6379> MEMORY USAGE mykey
    (integer) 56   # 56 байт включая overhead

Важно: MEMORY USAGE не учитывает:
- Память, выделенную через jemalloc, но не используемую (внутренняя фрагментация).
- Память буферов репликации, AOF и др.

Для точной оценки используйте комбинацию MEMORY USAGE и INFO memory.

6. INVALIDATION (ИНВАЛИДАЦИЯ КЭША)

Инвалидация — это процесс удаления или пометки устаревших данных в кэше.

Основные стратегии инвалидации:
1. TTL-based invalidation — данные автоматически удаляются по истечении TTL.
2. Write-Through — при обновлении данных в БД обновляется и кэш.
3. Cache-Aside — при обновлении данных в БД удаляется ключ из кэша.
4. Write-Behind — обновление кэша асинхронно.

При выборе TTL важно учитывать overhead: слишком маленький TTL приводит
к частому пересозданию ключей и росту фрагментации.
Слишком большой TTL — к хранению устаревших данных.

Оптимальный TTL для кэша обычно составляет 5-15 минут, но зависит от сценария.

7. ПРАКТИЧЕСКИЙ СОВЕТ: КАК ОЦЕНИВАТЬ ПАМЯТЬ

1. Используйте MEMORY USAGE key для каждого типа ключа.
2. Умножайте на количество ключей (приблизительно).
3. Добавьте 20% на фрагментацию и служебные буферы.
4. Учитывайте peak-нагрузки (used_memory_peak).
5. Всегда оставляйте ~20-30% свободной памяти выше maxmemory.

Пример оценки:
    100 000 ключей по 200 байт (с overhead) = 20 МБ.
    + 20% фрагментация = 24 МБ.
    + пиковая нагрузка 150% = 36 МБ.
    maxmemory = 50 МБ (с запасом).

8. СВЯЗЬ С GO

В go-redis есть метод MemoryUsage:
    used, err := rdb.MemoryUsage(ctx, "mykey").Result()

Он возвращает примерный размер ключа в байтах (int64).

Также можно использовать Info для общего анализа:
    info, _ := rdb.InfoMap(ctx, "memory").Result()
    usedMemory := info["memory"]["used_memory"]
    rssMemory := info["memory"]["used_memory_rss"]
    fragmentation := rssMemory / usedMemory  // приблизительно

9. ИТОГИ

- Overhead — это реальность, с которой нужно считаться.
- RedisObject (24 байта) есть у каждого ключа и значения.
- Разные типы данных имеют разный overhead.
- Используйте MEMORY USAGE для измерения.
- При планировании памяти закладывайте overhead и фрагментацию.
- Invalidation влияет на производительность — выбирайте стратегию осознанно.
*/

var rdb *redis.Client
var ctx = context.Background()

func init() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic("Redis не отвечает: " + err.Error())
	}
}

func main() {
	fmt.Println("=== ПРОДАКШН-КЕЙСЫ: OVERHEAD & INVALIDATION ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// КЕЙС 1: Сессии пользователей — хеш vs строка
func primer1() {
	fmt.Println("--- 1. Сессии: хеш vs строка для 100 000 сессий ---")

	// Сценарий: у нас 100 000 активных сессий.
	// Каждая сессия: user_id, ip, user_agent, last_activity, expires_at.

	// ВАРИАНТ А: отдельные строки (плохо)
	rdb.FlushDB(ctx)
	for i := 0; i < 1000; i++ {
		prefix := fmt.Sprintf("session:%d", i)
		rdb.Set(ctx, prefix+":user_id", fmt.Sprintf("user_%d", i), 0)
		rdb.Set(ctx, prefix+":ip", "192.168.1.1", 0)
		rdb.Set(ctx, prefix+":user_agent", "Mozilla/5.0", 0)
		rdb.Set(ctx, prefix+":last_activity", time.Now().String(), 0)
	}

	info, _ := rdb.InfoMap(ctx, "memory").Result()
	usedStrings := info["memory"]["used_memory"]
	fmt.Printf("Отдельные строки: %s\n", usedStrings)

	// ВАРИАНТ Б: один хеш на сессию (хорошо)
	rdb.FlushDB(ctx)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("session:%d", i)
		rdb.HSet(ctx, key,
			"user_id", fmt.Sprintf("user_%d", i),
			"ip", "192.168.1.1",
			"user_agent", "Mozilla/5.0",
			"last_activity", time.Now().String(),
		)
	}
	info, _ = rdb.InfoMap(ctx, "memory").Result()
	usedHash := info["memory"]["used_memory"]
	fmt.Printf("Хеш на сессию: %s (экономия ~40%%)\n", usedHash)

	fmt.Println("Вывод: для сессий используйте хеши — они экономят память.")
}

// КЕЙС 2: Кэш товаров с инвалидацией при обновлении
type Product struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	UpdatedAt int64   `json:"updated_at"`
}

func primer2() {
	fmt.Println("--- 2. Кэш товаров: инвалидация при обновлении ---")

	// Функция получения из БД (имитация)
	getProductFromDB := func(id int) *Product {
		return &Product{
			ID:        id,
			Name:      fmt.Sprintf("Product %d", id),
			Price:     99.99,
			Stock:     100,
			UpdatedAt: time.Now().Unix(),
		}
	}

	// Функция получения с кэшом
	getProduct := func(id int) (*Product, error) {
		key := fmt.Sprintf("product:%d", id)
		// Пытаемся получить из кэша
		data, err := rdb.Get(ctx, key).Result()
		if err == nil {
			var p Product
			if err := json.Unmarshal([]byte(data), &p); err == nil {
				return &p, nil
			}
		}

		// Кэш-промах: загружаем из БД
		p := getProductFromDB(id)
		jsonData, _ := json.Marshal(p)
		rdb.Set(ctx, key, jsonData, 10*time.Minute)
		return p, nil
	}

	// Функция обновления товара (инвалидация кэша)
	updateProduct := func(id int, newPrice float64) {
		// Обновляем в БД (имитация)
		fmt.Printf("  Обновлён товар %d: цена %.2f\n", id, newPrice)
		// Инвалидируем кэш
		key := fmt.Sprintf("product:%d", id)
		rdb.Del(ctx, key)
		fmt.Printf("  Кэш для товара %d удалён\n", id)
		// В реальности можно записать новый кэш синхронно
		// или оставить до первого запроса (Cache-Aside)
	}

	// Симуляция
	prod, _ := getProduct(1)
	fmt.Printf("Товар 1: %s, цена %.2f\n", prod.Name, prod.Price)

	updateProduct(1, 89.99)

	// Следующий запрос подтянет новый данные из БД
	prod, _ = getProduct(1)
	fmt.Printf("Товар 1 после обновления: %s, цена %.2f\n", prod.Name, prod.Price)

	rdb.Del(ctx, "product:1")
}

// КЕЙС 3: Rate Limiter с очисткой старых ключей
func primer3() {
	fmt.Println("--- 3. Rate Limiter: очистка старых ключей ---")

	// Проблема: при большом количестве пользователей ключи rate:* занимают
	// много памяти из-за overhead. Их нужно чистить, если пользователь неактивен.

	rateLimitKey := func(userID int) string {
		return fmt.Sprintf("rate:%d", userID)
	}

	// Проверка лимита
	checkRateLimit := func(userID int, limit int, window time.Duration) bool {
		key := rateLimitKey(userID)
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			return false
		}
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}
		return count <= int64(limit)
	}

	cleanupInactiveRateKeys := func(threshold time.Duration) {
		iter := rdb.Scan(ctx, 0, "rate:*", 100).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			ttl, _ := rdb.TTL(ctx, key).Result()
			// Если TTL <= 0 или ключ вот-вот истечёт, не трогаем
			if ttl > threshold {
				// Если пользователь не был активен (TTL большой), удаляем
				rdb.Del(ctx, key)
				fmt.Printf("  Удалён неактивный ключ: %s (TTL: %v)\n", key, ttl)
			}
		}
	}

	// Симуляция: 10 пользователей делают запросы
	for i := 0; i < 10; i++ {
		userID := rand.Intn(5) + 1
		ok := checkRateLimit(userID, 3, 10*time.Second)
		fmt.Printf("Пользователь %d: %v\n", userID, ok)
	}

	// Очистка: удаляем ключи, у которых TTL > 5 секунд
	cleanupInactiveRateKeys(5 * time.Second)

	// Проверяем, сколько осталось
	keys, _ := rdb.Keys(ctx, "rate:*").Result()
	fmt.Printf("Осталось ключей rate:*: %d\n", len(keys))

	rdb.Del(ctx, keys...)
}

// КЕЙС 4: История заказов с ограничением по памяти (LTRIM + TTL)
func primer4() {
	fmt.Println("--- 4. История заказов: ограничение размера списка ---")

	// Сценарий: храним последние 100 заказов пользователя.
	// При добавлении нового — удаляем самый старый, если их > 100.

	addOrder := func(userID int, orderID string) {
		key := fmt.Sprintf("orders:user:%d", userID)
		// Добавляем заказ в начало списка
		rdb.LPush(ctx, key, orderID)
		// Оставляем только 100 последних
		rdb.LTrim(ctx, key, 0, 99)
		// Устанавливаем TTL, чтобы история не висела вечно
		rdb.Expire(ctx, key, 30*24*time.Hour) // 30 дней
	}

	getOrders := func(userID int) []string {
		key := fmt.Sprintf("orders:user:%d", userID)
		orders, _ := rdb.LRange(ctx, key, 0, -1).Result()
		return orders
	}

	// Симуляция: пользователь делает 150 заказов
	for i := 1; i <= 150; i++ {
		addOrder(1, fmt.Sprintf("ORD-%05d", i))
	}

	orders := getOrders(1)
	fmt.Printf("История заказов пользователя 1: %d записей (последние 100)\n", len(orders))
	fmt.Printf("Последние 3: %v\n", orders[:3])

	// Проверяем размер в памяти
	mem, _ := rdb.MemoryUsage(ctx, "order:user:1").Result()
	fmt.Printf("Занимает памяти: %d байт\n", mem)

	rdb.Del(ctx, "orders:user:1")
}

// КЕЙС 5: Инвалидация кэша по паттерну (префиксу)
func primer5() {
	fmt.Println("--- 5. Инвалидация кэша по префиксу (паттерну) ---")

	// Сценарий: нужно удалить все ключи, начинающиеся с "cache:user:*"
	// при обновлении пользователя.

	// Популярная ошибка: KEYS cache:user:* (блокирует сервер!)
	// Правильный способ: SCAN + DEL

	invalidateByPrefix := func(prefix string) {
		iter := rdb.Scan(ctx, 0, prefix+"*", 100).Iterator()
		keysToDelete := []string{}
		for iter.Next(ctx) {
			keysToDelete = append(keysToDelete, iter.Val())
		}
		if err := iter.Err(); err != nil {
			panic(err)
		}
		if len(keysToDelete) > 0 {
			rdb.Del(ctx, keysToDelete...)
			fmt.Printf("  Удалено %d ключей с префиксом '%s'\n", len(keysToDelete), prefix)
		}
	}

	// Создаём тестовые ключи
	for i := 0; i < 10; i++ {
		rdb.Set(ctx, fmt.Sprintf("cache:user:%d", i), "data", 0)
		rdb.Set(ctx, fmt.Sprintf("cache:product:%d", i), "data", 0)
	}

	fmt.Println("Создано 10 пользовательских и 10 продуктовых кэшей")

	// Инвалидируем все пользовательские кэши
	invalidateByPrefix("cache:user")

	// Проверяем, что остались только продуктовые
	keys, _ := rdb.Keys(ctx, "cache:*").Result()
	fmt.Printf("Осталось: %d ключей (только продуктовые)\n", len(keys))

	rdb.Del(ctx, keys...)
}

// КЕЙС 6: Сессии с TTL и защита от вытеснения
func primer6() {
	// Проблема: если Redis использует allkeys-lru, сессии с TTL
	// могут быть вытеснены раньше истечения срока.
	// Решение: использовать volatile-lru или явно контролировать память.

	// Устанавливаем политику volatile-lru (защищает ключи без TTL)
	setConfig("maxmemory-policy", "volatile-lru")
	setConfig("maxmemory", "10mb")

	// Создаём сессии с TTL
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("session:%d", i)
		rdb.HSet(ctx, key,
			"user_id", fmt.Sprintf("user_%d", i),
			"created", time.Now().String(),
		)
		rdb.Expire(ctx, key, 10*time.Minute)
	}

	// Создаём вечные ключи (настройки, конфиги) — их нельзя удалять!
	rdb.Set(ctx, "config:app_name", "MyApp", 0)
	rdb.Set(ctx, "config:version", "1.0.0", 0)

	fmt.Println("Создано 1000 сессий с TTL и 2 вечных ключа")

	// Проверяем, что вечные ключи сохранились
	exists, _ := rdb.Exists(ctx, "config:app_name").Result()
	if exists == 1 {
		fmt.Println("Вечные ключи сохранились (volatile-lru защищает)")
	} else {
		fmt.Println("Вечные ключи удалены (политика не та)")
	}

	// Очистка
	rdb.FlushDB(ctx)
	resetConfig()
}

// КЕЙС 7: Write-Behind кэш (пакетное обновление)
func primer7() {
	fmt.Println("--- 7. Write-Behind: пакетное обновление кэша ---")
	fmt.Println("--- 7. Write-Behind: пакетное обновление кэша ---")

	// Сценарий: много запросов на обновление счётчика просмотров.
	// Вместо каждого инкремента пишем в Redis, а потом раз в N секунд
	// синхронизируем с БД.

	type ViewStats struct {
		mu       sync.Mutex
		buffer   map[string]int
		lastSync time.Time
	}

	stats := &ViewStats{
		buffer:   make(map[string]int),
		lastSync: time.Now(),
	}

	// Добавление просмотра (быстро, без блокировок БД)
	addView := func(pageID string) {
		stats.mu.Lock()
		defer stats.mu.Unlock()
		stats.buffer[pageID]++
		fmt.Printf("  +1 просмотр для %s (буфер: %d)\n", pageID, stats.buffer[pageID])
	}

	// Синхронизация с БД (раз в N секунд)
	syncToDB := func() {
		stats.mu.Lock()
		defer stats.mu.Unlock()
		if len(stats.buffer) == 0 {
			return
		}
		fmt.Printf("  📤 Синхронизация с БД: %v\n", stats.buffer)
		// В реальности здесь был бы запрос к БД:
		// for page, count := range stats.buffer {
		//     db.Exec("UPDATE page_views SET views = views + ? WHERE page_id = ?", count, page)
		// }
		// Теперь обновляем Redis (инвалидация кэша)
		for page := range stats.buffer {
			rdb.Del(ctx, "cache:page:"+page)
		}
		// Очищаем буфер
		stats.buffer = make(map[string]int)
		stats.lastSync = time.Now()
	}

	// Симуляция: пользователи смотрят страницы
	for i := 0; i < 10; i++ {
		page := fmt.Sprintf("page_%d", rand.Intn(3)+1)
		addView(page)
	}

	// Синхронизация
	syncToDB()

	// Проверяем, что кэш инвалидирован
	keys, _ := rdb.Keys(ctx, "cache:page:*").Result()
	fmt.Printf("Кэш инвалидирован, осталось ключей: %d\n", len(keys))

	rdb.Del(ctx, keys...)
}

// КЕЙС 8: 1 хеш vs 1000 строк для профилей пользователей
func primer8() {
	fmt.Println("--- 8. Профили пользователей: 1 хеш vs 1000 строк ---")

	// Сценарий: храним профили 1000 пользователей.
	// Вариант А: 1 хеш с ключом user_id и значением JSON.
	// Вариант Б: отдельная строка на пользователя.

	// ВАРИАНТ А: 1 хеш
	rdb.FlushDB(ctx)
	for i := 0; i < 1000; i++ {
		profile := fmt.Sprintf(`{"id":%d,"name":"User %d","email":"user%d@ex.com"}`, i, i, i)
		rdb.HSet(ctx, "users:hash", fmt.Sprintf("user:%d", i), profile)
	}
	info, _ := rdb.InfoMap(ctx, "memory").Result()
	hashMem := info["memory"]["used_memory"]

	// ВАРИАНТ Б: отдельные строки
	rdb.FlushDB(ctx)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("user:%d:profile", i)
		profile := fmt.Sprintf(`{"id":%d,"name":"User %d","email":"user%d@ex.com"}`, i, i, i)
		rdb.Set(ctx, key, profile, 0)
	}
	info, _ = rdb.InfoMap(ctx, "memory").Result()
	stringMem := info["memory"]["used_memory"]

	fmt.Printf("1 хеш (1000 полей): %s\n", hashMem)
	fmt.Printf("1000 отдельных строк: %s\n", stringMem)

	fmt.Println("Вывод: хеш экономит память и упрощает инвалидацию.")
	rdb.FlushDB(ctx)
}

func setConfig(param, value string) {
	rdb.ConfigSet(ctx, param, value)
}

func resetConfig() {
	rdb.ConfigSet(ctx, "maxmemory", "0")
	rdb.ConfigSet(ctx, "maxmemory-policy", "noeviction")
}
