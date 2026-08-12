package main

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
УРОК 7.4. RATE LIMITING
0. ВВЕДЕНИЕ: ПОЧЕМУ RATE LIMITING?

Rate Limiting — один из самых востребованных паттернов в распределённых системах.
Он защищает сервисы от перегрузки, обеспечивает справедливость между клиентами
и предотвращает злоупотребления. Без Rate Limiting ваш сервис может быть легко
положен одним недобросовестным клиентом, который отправит миллион запросов.

Redis — идеальный инструмент для Rate Limiting благодаря:
- Атомарным операциям (INCR, Lua).
- Встроенным механизмам TTL для автоматической очистки.
- Высокой производительности (десятки тысяч операций в секунду).
- Поддержке сложных структур данных (ZSET для скользящих окон).

1. ЧЕТЫРЕ ОСНОВНЫХ АЛГОРИТМА (ПОДРОБНО)

1.1. FIXED WINDOW (ФИКСИРОВАННОЕ ОКНО)
    - Принцип: подсчёт запросов в фиксированном временном интервале.
    - Реализация: INCR ключа, TTL = длина окна.
    - Пример: 10 запросов в минуту. Ключ "rate:user:123" живёт 60 секунд.
    - Проблема: "края" окон. Если сделать 10 запросов на 59-й секунде и 10 на 60-й,
      то за 2 секунды будет 20 запросов.
    - Атомарность: INCR + EXPIRE (через Lua) чтобы избежать гонок.
    - Память: O(1) — один ключ на пользователя/интервал.

    Когда использовать:
    - Простые API, где пиковые нагрузки не критичны.
    - Быстрая реализация.
    - Допустима небольшая погрешность.

1.2. SLIDING WINDOW (СКОЛЬЗЯЩЕЕ ОКНО)
    - Принцип: учитываются запросы за последние N секунд (точное окно).
    - Реализация: ZSET с временными метками как скорами.
    - Алгоритм (запрос):
      1. ZADD key timestamp "req_id"
      2. ZREMRANGEBYSCORE key 0 now-window  // удаляем старые
      3. ZCARD key  // считаем количество
      4. Если count > limit → отказ; иначе → разрешить
    - Преимущество: нет проблемы краёв, точное ограничение.
    - Недостаток: память — O(N) (хранит все запросы в окне).
    - Оптимизация: можно использовать агрегацию (например, хранить по 1 записи в секунду).

    Когда использовать:
    - API с биллингом (важна точность).
    - Строгие SLA.
    - Если допустимо небольшое потребление памяти.

1.3. TOKEN BUCKET (БАКЕТ С ТОКЕНАМИ)
    - Принцип: бакет с токенами, пополняемыми с постоянной скоростью.
    - Состояние: количество токенов (tokens) и время последнего пополнения (last_refill).
    - Алгоритм (Lua):
      1. Вычисляем delta = now - last_refill
      2. refill = delta * rate (токенов в секунду)
      3. tokens = min(capacity, tokens + refill)
      4. Если tokens >= 1 → tokens--, возвращаем разрешено
      5. Иначе → возвращаем отклонено
      6. Сохраняем tokens и last_refill
    - Преимущества:
      * Позволяет небольшие всплески (burst) до capacity.
      * Сглаживает нагрузку.
      * Гибкая настройка.
    - Недостаток: сложнее в реализации, нужно хранить состояние.

    Когда использовать:
    - Системы, где допустимы всплески (например, веб-серверы).
    - Платёжные шлюзы, где важна средняя скорость.
    - Сценарии с неравномерной нагрузкой.

1.4. LEAKY BUCKET (ДЫРЯВЫЙ БАКЕТ)
    - Принцип: запросы поступают в очередь, обрабатываются с постоянной скоростью.
    - Реализация: список (List) в Redis + фоновый воркер.
    - Алгоритм:
      1. Запрос → RPUSH в очередь (если длина < capacity).
      2. Воркер каждые interval секунд → LPOP и обработка.
      3. Если очередь полна → отказ.
    - Преимущества:
      * Гарантирует постоянную скорость обработки.
      * Простая реализация.
    - Недостатки:
      * Задержки, если очередь заполнена.
      * Нужен фоновый процесс.

    Когда использовать:
    - Отправка email, SMS (важна постоянная скорость).
    - Обработка задач в очередях.
    - Системы, где задержка допустима.

2. СРАВНЕНИЕ АЛГОРИТМОВ
┌───────────────────────────┬────────────────┬────────────────┬────────────────┬────────────────┐
│ ХАРАКТЕРИСТИКА            │ FIXED WINDOW   │ SLIDING WINDOW │ TOKEN BUCKET   │ LEAKY BUCKET   │
├───────────────────────────┼────────────────┼────────────────┼────────────────┼────────────────┤
│ Точность ограничений      │ Низкая         │ Высокая        │ Средняя        │ Средняя        │
│ Память на пользователя    │ O(1)           │ O(N)           │ O(1)           │ O(1) (очередь) │
│ Сложность реализации      │ Очень низкая   │ Средняя        │ Высокая        │ Средняя        │
│ Поддержка burst(всплесков)│ Да (на границе)│ Нет            │ Да(до capacity)│ Нет (очередь)  │
│ Равномерность нагрузки    │ Нет            │ Да             │ Да             │ Да             │
│ Требования к TTL          │ TTL на ключ    │ Удаление старых│ Хранение time  │ TTL не нужен   │
│ Подходит для              │ Простые API    │ Биллинг, SLA   │ Веб-серверы    │ Очереди задач  │
└───────────────────────────┴────────────────┴────────────────┴────────────────┴────────────────┘

3. АТОМАРНОСТЬ И ГОНКИ

Rate Limiting подвержен гонкам данных (race conditions), особенно при высоких нагрузках.
Например, два запроса одновременно проверяют лимит и видят, что он ещё не достигнут,
оба увеличивают счётчик и оба проходят.

Решение: использовать Lua-скрипты для атомарного выполнения операций
(INCR + проверка + установка TTL). Все операции выполняются на сервере
без возможности вмешательства других запросов.

Для Sliding Window также важен Lua: ZADD + ZREMRANGEBYSCORE + ZCARD.

4. ПРОБЛЕМЫ И ПОДВОДНЫЕ КАМНИ

4.1. "Эффект края" (Fixed Window)
    - Описано выше. Решение: Sliding Window.

4.2. "Голод" (Starvation) в Sliding Window
    - Если окно большое, старые записи могут долго не удаляться.
    - Решение: периодическая очистка через ZREMRANGEBYSCORE.

4.3. "Горячие" ключи в Cluster
    - Ключи всех пользователей могут попасть на один узел, если не использовать хеш-теги.
    - Решение: {user:123}:rate для равномерного распределения.

4.4. Точность времени в Sliding Window
    - Используйте миллисекунды для высокоточных окон.
    - Учитывайте разницу времени на разных серверах (NTP).

4.5. Ошибка TTL в Fixed Window
    - Если EXPIRE выполняется отдельно от INCR, может возникнуть гонка.
    - Решение: объединить в Lua.

5. РЕАЛИЗАЦИЯ В REDIS: ПАТТЕРНЫ И АНТИПАТТЕРНЫ

5.1. Фиксированное окно (правильно):
    local current = redis.call('INCR', key)
    if current == 1 then
        redis.call('EXPIRE', key, window)
    end
    return current <= limit

5.2. Фиксированное окно (неправильно — гонка):
    local current = redis.call('GET', key)
    if current and current >= limit then
        return 0
    end
    redis.call('INCR', key)  -- гонка: два запроса могут прочитать одно и то же значение
    return 1

5.3. Скользящее окно (правильно):
    local now = tonumber(ARGV[1])
    local window = tonumber(ARGV[2])
    local limit = tonumber(ARGV[3])
    local member = ARGV[4]
    redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
    redis.call('ZADD', key, now, member)
    local count = redis.call('ZCARD', key)
    if count > limit then
        redis.call('ZREM', key, member)
        return 0
    end
    return 1

5.4. Token Bucket (правильно):
    local tokens = redis.call('HGET', key, 'tokens') or capacity
    local last_refill = redis.call('HGET', key, 'last_refill') or now
    local delta = now - last_refill
    local refill = delta * rate
    tokens = math.min(capacity, tokens + refill)
    if tokens >= 1 then
        tokens = tokens - 1
        redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
        return 1
    else
        redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
        return 0
    end

6. МАСШТАБИРОВАНИЕ И ПРОИЗВОДИТЕЛЬНОСТЬ

- Fixed Window: 1 ключ на пользователя. Быстро.
- Sliding Window: может хранить тысячи записей в ZSET. Ограничьте размер через ZREMRANGEBYSCORE.
- Token Bucket: 1 ключ на пользователя (HASH). Быстро.
- Leaky Bucket: 1 список на пользователя. Используйте ограничение длины.

Для кластера используйте хеш-теги для пользователей.

7. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

1. Для простых API используйте Fixed Window (INCR + Lua).
2. Для точного биллинга — Sliding Window.
3. Для систем со всплесками — Token Bucket.
4. Для очередей с постоянной скоростью — Leaky Bucket.
5. Всегда используйте Lua для атомарности.
6. Устанавливайте TTL для ключей, чтобы не накапливать данные.
7. Для Sliding Window удаляйте старые записи.
8. Для кластера используйте хеш-теги.
9. Мониторьте количество отклонённых запросов.
10. Возвращайте Retry-After (TTL ключа) для информирования клиентов.
11. Для высоких нагрузок используйте Pipeline для пакетных операций.
12. Используйте контекст с таймаутом для предотвращения зависаний.

8. СВЯЗЬ С GO (go-redis)

- INCR: rdb.Incr(ctx, key).Result()
- EXPIRE: rdb.Expire(ctx, key, window).Result()
- Lua: script.Run(ctx, rdb, []string{key}, args...).Result()
- ZSET: rdb.ZAdd, rdb.ZRemRangeByScore, rdb.ZCard
- Pipeline: для массовых операций (проверка нескольких пользователей)
- Контекст: для таймаутов

9. ИТОГИ

Rate Limiting — критически важный компонент любой распределённой системы.
Выбор алгоритма зависит от требований:
- Точность (Fixed Window vs Sliding Window)
- Всплески (Token Bucket)
- Равномерная скорость (Leaky Bucket)

Redis предоставляет все необходимые инструменты для реализации любого из них.
Главное — правильно использовать Lua для атомарности и следить за памятью.
*/

var rdb *redis.Client
var ctx = context.Background()
var logger = log.Default()

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

func main() {
	fmt.Println("=== RATE LIMITING: ПРОДАКШН-ПРИМЕРЫ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
}

// 1. FIXED WINDOW (INCR + TTL) — ПРОСТОЙ И БЫСТРЫЙ
type FixedWindowLimiter struct {
	client *redis.Client
	key    string
	limit  int64
	window time.Duration
}

func (l *FixedWindowLimiter) Allow(ctx context.Context) (bool, int64, error) {
	// Lua-скрипт: INCR, TTL при первом запросе, возврат TTL для Retry-After
	script := redis.NewScript(`
	local key = KEYS[1]
	local limit = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local current = redis.call("INCR", key)
	if current == 1 then
		redis.call("EXPIRE", key, window)
	end
	local ttl = redis.call("TTL", key)
	if current > limit then
		return {0, ttl}	
	end
	return{1,ttl}	
	`)
	res, err := script.Run(ctx, l.client, []string{l.key}, l.limit, int(l.window.Seconds())).Result()
	if err != nil {
		return false, 0, err
	}
	arr := res.([]interface{})
	ok := arr[0].(int64) == 1
	ttl := arr[1].(int64)
	return ok, ttl, nil
}

func primer1() {
	fmt.Println("--- 1. Fixed Window Limiter ---")

	limiter := &FixedWindowLimiter{
		client: rdb,
		key:    "rate:fixed:user:123",
		limit:  5,
		window: 10 * time.Second,
	}
	rdb.Del(ctx, limiter.key)

	for i := 0; i < 7; i++ {
		ok, ttl, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён, попробуйте через %d сек\n", i+1, ttl)
		}
	}
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}

// 2. SLIDING WINDOW (ZSET) — ТОЧНОЕ ОГРАНИЧЕНИЕ
type SlidingWindowLimiter struct {
	client *redis.Client
	key    string
	limit  int64
	window time.Duration
}

func (l *SlidingWindowLimiter) Allow(ctx context.Context) (bool, error) {
	now := time.Now().UnixMilli()
	windowMs := l.window.Milliseconds()
	member := fmt.Sprintf("%d", now) // уникальный ID

	script := redis.NewScript(`
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local member = ARGV[4]

	-- Удаляем старые записи
	redis.call("ZREMRANGEBYSCORE", key, 0, now - window)
	-- Добавляем текущий
	redis.call('ZADD', key, now, member)
	-- Считаем количество
	local count = redis.call('ZCARD', key)
	if count > limit then
		redis.call("ZREM", key, member)
		return 0
	end	
	-- Устанавливаем TTL на ключ (окно + 1 секунда)
	redic.call("EXPIRE", key, math.ceil(window/1000)+1)
	return 1
	`)

	ok, err := script.Run(ctx, l.client, []string{l.key}, now, windowMs, l.limit, member).Result()
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}

func primer2() {
	fmt.Println("--- 2. Sliding Window Limiter ---")
	limiter := &SlidingWindowLimiter{
		client: rdb,
		key:    "rate:sliding:user:123",
		limit:  5,
		window: 10 * time.Second,
	}
	rdb.Del(ctx, limiter.key)

	for i := 0; i < 7; i++ {
		ok, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён\n", i+1)
		}
	}
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}

// 3. TOKEN BUCKET (HASH) — ВСПЛЕСКИ И СГЛАЖИВАНИЕ
type TokenBucketLimiter struct {
	client     *redis.Client
	key        string
	capacity   int64
	refillRate float64 // токенов в секунду
}

func (l *TokenBucketLimiter) Allow(ctx context.Context) (bool, error) {
	now := time.Now().Unix()

	script := redis.NewScript(`
	local key = KEYS[1]
			local capacity = tonumber(ARGV[1])
			local rate = tonumber(ARGV[2])
			local now = tonumber(ARGV[3])

			local data = redis.call('HMGET', key, 'tokens', 'last_refill')
			local tokens = tonumber(data[1]) or capacity
			local last = tonumber(data[2]) or now

			local delta = math.max(0, now - last)
			local refill = delta * rate
			tokens = math.min(capacity, tokens + refill)

			if tokens >= 1 then
				tokens = tokens - 1
				redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
				redis.call('EXPIRE', key, 3600)
				return 1
			else
				redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
				redis.call('EXPIRE', key, 3600)
				return 0
			end
	`)
	ok, err := script.Run(ctx, l.client, []string{l.key}, l.capacity, l.refillRate, now).Int()
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}

func primer3() {
	fmt.Println("--- 3. Token Bucket Limiter ---")

	limiter := &TokenBucketLimiter{
		client:     rdb,
		key:        "rate:token:user:123",
		capacity:   10,
		refillRate: 1.0, // 1 токен/сек
	}
	rdb.Del(ctx, limiter.key)

	for i := 0; i < 15; i++ {
		ok, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён\n", i+1)
		}
		if i == 9 {
			fmt.Println("  (пауза 3 сек для пополнения)")
			time.Sleep(3 * time.Second)
		}
	}
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}

// 4. LEAKY BUCKET (LIST) — ПОСТОЯННАЯ СКОРОСТЬ ОБРАБОТКИ
type LeakyBucketLimiter struct {
	client   *redis.Client
	queueKey string
	capacity int64
	rate     time.Duration // интервал между обработкой
}

func (l *LeakyBucketLimiter) Allow(ctx context.Context) (bool, error) {
	script := redis.NewScript(`
		local queue = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local member = ARGV[2]
		local len = redis.call("LLEN",queue)
		if len >= capacity then
			return 0
		end
		redis.call("RPUSH", queue, number)
			return 1	
	`)
	member := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := script.Run(ctx, l.client, []string{l.queueKey}, l.capacity, member).Int()
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}

func (l *LeakyBucketLimiter) StartWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(l.rate)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				val, err := l.client.LPop(ctx, l.queueKey).Result()
				if errors.Is(err, redis.Nil) {
					continue
				}
				if err != nil {
					logger.Printf("Ошибка LPOP: %v", err)
					continue
				}
				logger.Printf("Обработан запрос: %s", val)
			}
		}
	}()
}

func primer4() {
	fmt.Println("--- 4. Leaky Bucket Limiter ---")
	limiter := &LeakyBucketLimiter{
		client:   rdb,
		queueKey: "rate:leaky:queue",
		capacity: 5,
		rate:     500 * time.Millisecond,
	}
	rdb.Del(ctx, limiter.queueKey)

	ctxWorker, cancel := context.WithCancel(ctx)
	limiter.StartWorker(ctxWorker)
	defer cancel()

	for i := 0; i < 10; i++ {
		ok, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d добавлен в очередь\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён (очередь полна)\n", i+1)
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(3 * time.Second)
	rdb.Del(ctx, limiter.queueKey)
	fmt.Println()
}

// 5. МНОГОУРОВНЕВЫЙ ЛИМИТЕР (ГЛОБАЛЬНЫЙ + ПОЛЬЗОВАТЕЛЬСКИЙ)
type MultiLimiter struct {
	client      *redis.Client
	globalKey   string
	userKey     string
	globalLimit int64
	userLimit   int64
	window      time.Duration
}

func (l *MultiLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	userKey := fmt.Sprintf("%s:%s", l.userKey, userID)
	script := redis.NewScript(`
	 	local global_key = KEYS[1]
		local user_key = KEYS[2]
		local global_limit = tonumber(ARGV[1])
		local user_limit = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])

		local g = redis.call("INCR", global_key)
		if g == 1 then
			redis.call("EXPIRE", global_key, window)
		end
		if g > global_limit then
			return 0
		
		local u = redis.call('INCR', user_key)
		if u == 1 then 
			redis.call('EXPIRE', user_key, window) 
		end
		if u > user_limit then 
			return 0 end

		return 1	
	`)
	ok, err := script.Run(ctx, l.client, []string{l.globalKey, userKey},
		l.globalLimit, l.userLimit, int(l.window.Seconds())).Result()

	if err != nil {
		return false, err
	}
	return ok == 1, nil

}

func primer5() {
	fmt.Println("--- 5. Multi-level Limiter ---")
	limiter := &MultiLimiter{
		client:      rdb,
		globalKey:   "rate:global",
		userKey:     "rate:user",
		globalLimit: 100,
		userLimit:   5,
		window:      10 * time.Second,
	}
	rdb.Del(ctx, limiter.globalKey, limiter.userKey+":*")

	for i := 0; i < 10; i++ {
		ok, err := limiter.Allow(ctx, "alice")
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён\n", i+1)
		}
	}
	rdb.Del(ctx, limiter.globalKey, limiter.userKey+":*")
	fmt.Println()
}

// 6. RATE LIMITER С RETRY-AFTER (ВОЗВРАТ ВРЕМЕНИ ОЖИДАНИЯ)
type RetryLimiter struct {
	client *redis.Client
	key    string
	limit  int64
	window time.Duration
}

func (l *RetryLimiter) Allow(ctx context.Context) (bool, int64, error) {
	script := redis.NewScript(`
			local key = KEYS[1]
			local limit = tonumber(ARGV[1])
			local window = tonumber(ARGV[2])
			local current = redis.call('INCR', key)
			if current == 1 then redis.call('EXPIRE', key, window) end
			local ttl = redis.call('TTL', key)
			if current > limit then
				return {0, ttl}
			end
			return {1, ttl}
		`)
	res, err := script.Run(ctx, l.client, []string{l.key}, l.limit, int(l.window.Seconds())).Result()
	if err != nil {
		return false, 0, err
	}
	arr := res.([]interface{})
	ok := arr[0].(int64) == 1
	ttl := arr[1].(int64)
	return ok, ttl, nil
}

func primer6() {
	fmt.Println("--- 6. Rate Limiter с Retry-After ---")
	limiter := &RetryLimiter{
		client: rdb,
		key:    "rate:retry:user:123",
		limit:  3,
		window: 10 * time.Second,
	}
	rdb.Del(ctx, limiter.key)

	for i := 0; i < 5; i++ {
		ok, ttl, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён, попробуйте через %d сек\n", i+1, ttl)
		}
	}
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}

// 7. RATE LIMITER С МЕТРИКАМИ (СБОР СТАТИСТИКИ)
type MetricLimiter struct {
	client   *redis.Client
	key      string
	limit    int64
	window   time.Duration
	mu       sync.Mutex
	allowed  int64
	rejected int64
}

func (l *MetricLimiter) Allow(ctx context.Context) (bool, error) {
	ok, err := l.allow(ctx)
	l.mu.Lock()
	defer l.mu.Unlock()
	if ok {
		l.allowed++
	} else {
		l.rejected++
	}
	return ok, err
}

func (l *MetricLimiter) allow(ctx context.Context) (bool, error) {
	script := redis.NewScript(`
			local key = KEYS[1]
			local limit = tonumber(ARGV[1])
			local window = tonumber(ARGV[2])
			local current = redis.call('INCR', key)
			if current == 1 then redis.call('EXPIRE', key, window) end
			return current <= limit
		`)
	return script.Run(ctx, l.client, []string{l.key}, l.limit, int(l.window.Seconds())).Bool()
}

func (l *MetricLimiter) Stats() (allowed, rejected int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.allowed, l.rejected
}

func primer7() {
	fmt.Println("--- 7. Rate Limiter с метриками ---")
	limiter := &MetricLimiter{
		client: rdb,
		key:    "rate:metrics:user:123",
		limit:  4,
		window: 10 * time.Second,
	}
	rdb.Del(ctx, limiter.key)

	for i := 0; i < 8; i++ {
		ok, _ := limiter.Allow(ctx)
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён\n", i+1)
		}
	}
	allowed, rejected := limiter.Stats()
	fmt.Printf("Статистика: разрешено %d, отклонено %d\n", allowed, rejected)
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}
