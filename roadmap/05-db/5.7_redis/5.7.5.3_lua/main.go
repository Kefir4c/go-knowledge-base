package lua

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 5.3. LUA-СКРИПТЫ В REDIS
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖНЫ LUA-СКРИПТЫ?

Redis — это высокопроизводительное in-memory хранилище, но часто требуется
выполнить несколько операций атомарно, с условиями и логикой, которые зависят
от текущих данных. Это можно сделать на стороне клиента с помощью транзакций
(MULTI/EXEC) и WATCH, но такой подход имеет недостатки:
- Множество сетевых вызовов (GET, WATCH, MULTI, EXEC).
- Обработка конфликтов требует повторных попыток на клиенте.
- Нет возможности выполнить сложную логику (циклы, условия).

Lua-скрипты позволяют перенести эту логику на сервер Redis, обеспечивая:
- Атомарность: скрипт выполняется как одна неделимая операция.
- Сокращение сетевых вызовов: клиент отправляет только вызов скрипта.
- Производительность: скрипт работает с данными напрямую в памяти сервера.
- Гибкость: можно реализовать любую логику на Lua (условия, циклы, математика).

На собеседованиях часто спрашивают: «Зачем нужны Lua-скрипты, если есть транзакции?»
И правильный ответ: транзакции — это простой способ атомарно выполнить несколько
команд, но Lua даёт больше возможностей (условная логика) и лучше работает
в высоконагруженных системах, снижая сетевые задержки.

1. БАЗОВЫЙ СИНТАКСИС И КОМАНДЫ

1.1. EVAL — выполнение скрипта
    EVAL <script> <numkeys> <key1> <key2> ... <arg1> <arg2> ...
    - script — строка с Lua-кодом.
    - numkeys — количество ключей, которые будут переданы (число).
    - Остальные аргументы делятся на ключи (первые numkeys) и аргументы (остальные).

    Пример:
        EVAL "return redis.call('GET', KEYS[1])" 1 mykey

1.2. EVALSHA — выполнение по SHA1-хешу
    EVALSHA <sha1> <numkeys> <key1> ... <arg1> ...
    - Если скрипт с данным хешем был загружен ранее (через SCRIPT LOAD), он выполняется.
    - В противном случае возвращается ошибка NOSCRIPT.

1.3. SCRIPT LOAD — загрузка скрипта без выполнения
    SCRIPT LOAD <script>
    - Возвращает SHA1-хеш скрипта, который можно использовать для EVALSHA.

1.4. SCRIPT EXISTS — проверка существования скрипта
    SCRIPT EXISTS <sha1> [sha1 ...]
    - Возвращает массив из 1 (есть) или 0 (нет) для каждого хеша.

1.5. SCRIPT FLUSH — удаление всех кэшированных скриптов
    SCRIPT FLUSH [ASYNC|SYNC]
    - Очищает кэш скриптов. В Redis 7.0+ можно использовать ASYNC для неблокирующего удаления.

2. ПЕРЕДАЧА ПАРАМЕТРОВ: KEYS И ARGV

Очень важно понимать разницу между KEYS и ARGV:
- KEYS — это ключи, которые скрипт будет использовать. Redis использует их для:
  * Проверки, что все ключи принадлежат одному слоту (в кластере).
  * Репликации: изменения ключей реплицируются на реплики.
  * Оповещения о ключевых событиях (keyspace notifications).
- ARGV — это остальные аргументы (значения, параметры). Они не участвуют в репликации и кластерных ограничениях.

Почему это важно:
- В Redis Cluster скрипт может обращаться только к ключам из одного слота.
- Redis использует KEYS для определения слотов и репликации изменений.
- Если скрипт использует ключи, они должны быть переданы через KEYS, даже если они не используются напрямую.

Пример правильного скрипта:
    -- KEYS[1] = user:123, KEYS[2] = user:123:balance
    -- ARGV[1] = 50 (сумма)
    local balance = redis.call('GET', KEYS[2])
    if tonumber(balance) >= tonumber(ARGV[1]) then
        redis.call('DECRBY', KEYS[2], ARGV[1])
        redis.call('INCRBY', KEYS[1] .. ':total', ARGV[1])
        return 1
    else
        return 0
    end

3. ВЗАИМОДЕЙСТВИЕ С REDIS: redis.call() и redis.pcall()

Внутри скрипта можно вызывать любые команды Redis через:
- redis.call(command, ...) — выполняет команду. В случае ошибки генерирует исключение,
  которое прерывает выполнение скрипта и откатывает все изменения.
- redis.pcall(command, ...) — выполняет команду. В случае ошибки возвращает таблицу
  с полем err, но скрипт продолжает выполняться.

Когда использовать:
- redis.call() — для большинства случаев, когда ошибка означает фатальный сбой.
- redis.pcall() — для обработки ошибок внутри скрипта (например, проверка существования ключа).

Пример с pcall:
    local status, result = pcall(redis.call, 'GET', 'nonexistent')
    if not status then
        -- обработать ошибку
        return nil
    end
    return result

Возвращаемые значения:
- Redis преобразует ответы в типы Lua:
  * INTEGER → number (целое)
  * BULK STRING → string
  * MULTI BULK → таблица (массив)
  * STATUS → OK или ошибка
  * NIL → nil

4. ОБРАБОТКА ОШИБОК И ВОЗВРАТ ЗНАЧЕНИЙ

4.1. Генерация ошибок:
    error('сообщение об ошибке') — выбрасывает исключение, которое отменяет скрипт.

4.2. Возврат значений:
    return value — возвращает значение клиенту.
    Можно возвращать:
    - числа (integer/float)
    - строки
    - таблицы (массивы) — превращаются в MULTI BULK ответ
    - булевы значения (true/false) → 1/0

4.3. Обработка ошибок на клиенте (Go):
    - Если скрипт вызывает error(), go-redis возвращает ошибку типа redis.Error
      с текстом из скрипта.
    - Если скрипт возвращает nil, это эквивалентно redis.Nil.

5. АТОМАРНОСТЬ И ИЗОЛЯЦИЯ

Одно из главных свойств Lua-скриптов: они выполняются атомарно и изолированно.
- Атомарность: весь скрипт выполняется как одна команда — никакие другие команды
  не могут вклиниться между операциями скрипта.
- Изоляция: скрипт не видит изменений, сделанных другими клиентами, пока он выполняется.

Это означает, что скрипт не подвержен гонкам данных и не требует использования WATCH.
Также все изменения, сделанные в скрипте, либо применяются полностью (если скрипт
завершился без ошибок), либо откатываются (если возникла ошибка).
Исключение: ошибки в redis.call() откатывают только изменения, сделанные до ошибки,
а не весь скрипт. Но скрипт прерывается, и изменения до ошибки фиксируются.
Поэтому рекомендуется выполнять все проверки до изменения данных, чтобы избежать
частичных изменений.

6. КЭШИРОВАНИЕ СКРИПТОВ (EVALSHA)

6.1. Как работает кэширование:
    - При первом вызове EVAL Redis парсит и компилирует скрипт, вычисляет SHA1-хеш
      и сохраняет его в кэше (in-memory).
    - При последующих вызовах можно использовать EVALSHA <sha1>, передавая только хеш,
      что экономит трафик (не нужно отправлять весь скрипт).

6.2. Когда использовать EVALSHA:
    - Всегда, когда скрипт не меняется в процессе работы приложения.
    - Для часто вызываемых скриптов — это значительно сокращает сетевой трафик
      и уменьшает задержки.

6.3. Обработка NOSCRIPT в go-redis:
    - go-redis предоставляет redis.NewScript(), который автоматически загружает
      скрипт при первом вызове и использует EVALSHA для всех последующих.
    - Если сервер возвращает ошибку NOSCRIPT (скрипт был удалён из кэша),
      go-redis повторно загружает скрипт и выполняет его через EVAL.
    - Это удобно и не требует ручного управления.

6.4. Управление кэшем:
    - SCRIPT FLUSH — очищает кэш (например, после развёртывания нового скрипта).
    - SCRIPT EXISTS — проверяет, загружен ли скрипт.

7. ОГРАНИЧЕНИЯ И ПРОИЗВОДИТЕЛЬНОСТЬ

7.1. Ограничения в Redis Cluster:
    - Скрипт может обращаться только к ключам из одного слота.
    - Если скрипт обращается к нескольким ключам, они должны иметь одинаковый хеш-тег.
    - Исключение: если скрипт не использует ключи (только ARGV), он может выполняться на любом узле.

7.2. Время выполнения:
    - Redis имеет глобальный таймаут для скриптов: lua-time-limit (по умолчанию 5000 мс).
    - Если скрипт выполняется дольше, Redis начинает отвечать на другие команды ошибкой BUSY.
    - Рекомендация: скрипты должны выполняться менее 1 мс. Для объёмных операций используйте пайплайн.

7.3. Блокировка Redis:
    - Долгие скрипты блокируют Redis, так как они выполняются в основном потоке.
    - Это увеличивает задержки для всех клиентов и может вызвать таймауты.

7.4. Использование памяти:
    - Скрипты могут создавать большие таблицы Lua, что потребляет память сервера.
    - Старайтесь ограничивать объём данных, обрабатываемых в скрипте.

8. СРАВНЕНИЕ С ТРАНЗАКЦИЯМИ (MULTI/EXEC) И ПАЙПЛАЙНОМ

┌────────────────┬──────────────────┬──────────────────┬────────────────────┐
│ ХАРАКТЕРИСТИКА │ LUA-СКРИПТ       │ ТРАНЗАКЦИЯ       │ ПАЙПЛАЙН           │
├────────────────┼──────────────────┼──────────────────┼────────────────────┤
│ Атомарность    │ Да (весь скрипт) │ Да               │ Нет                │
│ Изоляция       │ Да               │ Да               │ Нет                │
│ Условная логика│ Да               │ WATCH + ретраи   │ Нет                │
│ Сложность      │ Высокая          │ Средняя          │ Простая            │
│ Производительность│ Высокая       │ Средняя (ретраи)│ Высокая             │
│ Сетевой трафик │ Низкий (1 вызов) │ Средний (WATCH)  │ Низкий (пачка)     │
│ Обработка      │ Только финальный │ Массив результат │ Индексы команд     │
│ результатов    │ результат        │                  │                    │
│ Откат при      │ Нет (если не     │ Нет (только до   │ Нет                │
│ ошибке         │ прервать через   │ EXEC)            │                    │
│                │ error())         │                  │                    │
└────────────────┴──────────────────┴──────────────────┴────────────────────┘

Вывод: Lua — самый мощный инструмент для сложной логики, требующей атомарности.
Транзакции хороши для простых обновлений без условий. Пайплайн — для массовых
операций, где атомарность не нужна.

9. ЛУЧШИЕ ПРАКТИКИ И РЕКОМЕНДАЦИИ

1. Всегда передавайте ключи через KEYS, аргументы через ARGV.
2. Проверяйте существование ключей перед изменением.
3. Используйте redis.call() для большинства команд, redis.pcall() — только когда нужна обработка ошибок.
4. Избегайте больших циклов и операций с большим объёмом данных (более 1000 элементов).
5. Возвращайте структурированные ответы (например, таблицу с полями ok/err) для удобства клиента.
6. Используйте redis.NewScript() в go-redis для автоматического кэширования.
7. Тестируйте скрипты через redis-cli --eval перед внедрением.
8. Для кластера используйте хеш-теги для группировки ключей.
9. Устанавливайте lua-time-limit адекватно (не слишком большое, чтобы не блокировать Redis).
10. Логируйте ошибки скриптов на стороне клиента.

10. ЧАСТЫЕ ВОПРОСЫ НА СОБЕСЕДОВАНИЯХ

Вопрос: «В чём отличие Lua-скриптов от транзакций?»
Ответ: Lua даёт условную логику, меньше сетевых вызовов, атомарность без WATCH.

Вопрос: «Как гарантировать атомарность в Lua?»
Ответ: Скрипт выполняется как одна команда, никакие другие операции не вмешиваются.

Вопрос: «Что такое EVALSHA и зачем он нужен?»
Ответ: EVALSHA позволяет выполнить скрипт по его SHA1-хешу, экономя трафик, если скрипт уже загружен.

Вопрос: «Как обрабатывать ошибки в Lua-скриптах?»
Ответ: Использовать redis.pcall() для перехвата ошибок или error() для генерации исключений.

Вопрос: «Можно ли использовать Lua-скрипты в кластере?»
Ответ: Да, но только если скрипт обращается к ключам из одного слота (с хеш-тегами).

Вопрос: «Как предотвратить долгое выполнение скрипта?»
Ответ: Настроить lua-time-limit и писать эффективные скрипты.
*/

var (
	rdb    *redis.Client
	ctx    = context.Background()
	logger = log.Default()
)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     20,
		MinIdleConns: 5,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

func main() {
	fmt.Println("=== LUA-СКРИПТЫ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. RATE LIMITER (TOKEN BUCKET) — ОГРАНИЧЕНИЕ ЗАПРОСОВ
func primer1() {
	fmt.Println("--- 1. Rate Limiter (Token Bucket) ---")

	script := redis.NewScript(`
	local key = KEYS[1]
		local limit = tonumber(ARGV[1])      -- максимум запросов
		local window = tonumber(ARGV[2])     -- окно в секундах
		local now = tonumber(ARGV[3])
		local member = ARGV[4]               -- уникальный идентификатор запроса

		-- Удаляем записи старше окна
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

		-- Добавляем текущий запрос с временем
		redis.call('ZADD', key, now, member)

		-- Подсчитываем количество в окне
		local count = redis.call('ZCARD', key)

		if count > limit then
		-- Откатываем добавление
		redis.call('ZREM', key, number)
		return {err = "rate limit exceeded", limit = limit, current = count - 1}
		end

		-- Устанавливаем TTL для автоматической очистки
		redis.call('EXPIRE', key, window)
		return {ok = 'allowed', remaining = limit - count}
	`)

	key := "{rate}:user:1234"
	limit := 5
	window := 10

	for i := 0; i < 7; i++ {
		now := time.Now().Unix()
		member := fmt.Sprintf("req_%d_%d", i, now)

		result, err := script.Run(ctx, rdb, []string{key}, limit, window, now, member).Result()
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		// Результат — Lua таблица, в Go map[interface{}]interface{}
		resMap := result.(map[interface{}]interface{})
		if errMsg, ok := resMap["err"]; ok {
			fmt.Printf("Запрос %d отклонён: %v (лимит %v, текущий %v)\n", i+1, errMsg, resMap["limit"], resMap["current"])
		} else {
			fmt.Printf("Запрос %d разрешён, осталось: %v\n", i+1, resMap["remaining"])
		}
	}
	rdb.Del(ctx, key)
}

// 2. ПЕРЕВОД СРЕДСТВ С ЖУРНАЛОМ
func primer2() {
	fmt.Println("\n--- 2. Перевод средств между счетами ---")

	script := redis.NewScript(`
	local from = KEYS[1]
	local to = KEYS[2]
	local logKey = KEYS[3]
	local amount = tonumber(ARGV[1])

	local fromBal = redic.call('GET', from)
	if not fromBal then
		return {err = "from account amount not found"}
	end
	fromBal = tonumber(fromBal)
	if fromBal < amount then
		return {err = "insufficient funds", balance = fromBal, required = amount}
	end
	
	local toBal = redis.call('GET', to)
	if not toBal then
			toBal = 0
		else
			toBal = tonumber(toBal)
		end

		-- Выполняем перевод
		redis.call('SET',from, fromBal - amount)
		redis.call('SET', toBal + amounto)

		-- Записываем в журнал (LPush с временем)
		redis.call('LPUSH', logKey, string.format("%s -> %s: %d at %d", from, to, amount, redis.call('TIME')[1]))

		return {ok = "success", from_balance = fromBal - amount, to_balance = toBal + amount}
	`)

	fromKey := "{acc}:A"
	toKey := "{acc}:B"
	logKey := "{acc}:transfers:log"
	rdb.Set(ctx, fromKey, 100, 0)
	rdb.Set(ctx, toKey, 50, 0)

	result, err := script.Run(ctx, rdb, []string{fromKey, toKey, logKey}, 30).Result()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		resMap := result.(map[interface{}]interface{})
		if errMsg, ok := resMap["err"]; ok {
			fmt.Printf("Перевод не удался: %v (баланс %v, требуется %v)\n", errMsg, resMap["balance"], resMap["required"])
		} else {
			fmt.Printf("Перевод успешен: from=%v, to=%v\n", resMap["from_balance"], resMap["to_balance"])
		}
	}
	// Очистка
	rdb.Del(ctx, fromKey, toKey, logKey)
}

// 3. РЕЗЕРВИРОВАНИЕ ТОВАРА С ВОЗВРАТОМ
func primer3() {
	fmt.Println("\n--- 3. Резервирование товара ---")

	script := redis.NewScript(`
		local stock = KEYS[1]
		local reserved = KEYS[2]
		local action = ARGV[1]               -- "reserve" или "release"
		local qty = tonumber(ARGV[2])

		local stockVal = redis.call('GET', stock)
		if not stockVal then
			return {err = "stock key not found"}
		end
		stockVal = tonumber(stockVal)

		local reservedVal = redis.call('GET', reserved)
		if not reservedVal then
			reservedVal = 0
		else
			reservedVal = tonumber(reservedVal)
		end

		local available = stockVal - reservedVal

		if action == "reserve" then
			if available < qty then
				return {err = "not enough stock", available = available, requested = qty}
			end
			redis.call('SET', reserved, reservedVal + qty)
			return {ok = "reserved", available = available - qty}
		elseif action == "release" then
			if reservedVal < qty then
				return {err = "cannot release more than reserved", reserved = reservedVal}
			end
			redis.call('SET', reserved, reservedVal - qty)
			return {ok = "released", available = stockVal - (reservedVal - qty)}
		else
			return {err = "invalid action"}
		end
	`)

	stockKey := "{inv}:product:123:stock"
	reservedKey := "{inv}:product:123:reserved"
	rdb.Set(ctx, stockKey, 10, 0)
	rdb.Set(ctx, reservedKey, 2, 0)

	// Резервируем 3
	result, err := script.Run(ctx, rdb, []string{stockKey, reservedKey}, "reserve", 3).Result()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		resMap := result.(map[interface{}]interface{})
		if errMsg, ok := resMap["err"]; ok {
			fmt.Printf("Резервирование не удалось: %v (доступно %v)\n", errMsg, resMap["available"])
		} else {
			fmt.Printf("Зарезервировано, осталось доступно: %v\n", resMap["available"])
		}
	}
	rdb.Del(ctx, stockKey, reservedKey)
}

// 4. CAS-ОБНОВЛЕНИЕ С ВЕРСИОНИРОВАНИЕМ
func primer4() {
	fmt.Println("\n--- 4. CAS-обновление с версией (Compare-And-Set) ---")

	script := redis.NewScript(`
	local key = KEYS[1]
	local versKEy = KEYS[2]
	local expectedVersion = ARGV[1]
	local newValue = ARGV[2]
	local newVersion = ARGV[3]

	local currenVersion = redis.call('GET', versKey)
	if not currentVersion then
		return err{err = "version key not found}
	end
	
	if currentVersion ~= expectedVersion then
		return {err = "version mismatch", expected = expectedVersion, actual = currentVersion}
	end
	
	redis.call('SET', key, newValue)
	redis.call('SET, versKey, newVersion)

	return {ok = "updated", version = newVersion}
	`)

	key := "{user}:123:profile"
	versKey := "{user}:123:profileVersion"
	rdb.Set(ctx, key, "old-data", 0)
	rdb.Set(ctx, versKey, "v1", 0)

	result, err := script.Run(ctx, rdb, []string{key, versKey}, "v1", "new_data", "v2").Result()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	}
	resMap := result.(map[interface{}]interface{})
	if errMsg, ok := resMap["err"]; ok {
		fmt.Printf("Обновление отклонено: %v (ожидалась %v, имеется %v)\n", errMsg, resMap["expected"], resMap["actual"])
	} else {
		fmt.Printf("Обновлено, новая версия: %v\n", resMap["version"])
	}
	rdb.Del(ctx, key, versKey)
}

// 5. ПАКЕТНАЯ ВСТАВКА С ДЕДУПЛИКАЦИЕЙ (ТОЛЬКО УНИКАЛЬНЫЕ)
func primer5() {
	fmt.Println("\n--- 5. Пакетная вставка с дедупликацией (только новые) ---")

	script := redis.NewScript(`
	local prefix = KEYS[1]
	local items = ARGV
	local inserted = 0

	for _, val in ipairs(items) do
		local key = prefix .. val
		if redis.call('SETNX', key,val) == 1 then
			inserted = inserted + 1
			-- Устанавливаем TTL для автоматической очистки
			redis.call('EXPIRE', key, 3600)
		end
	end
	return inserted
	`)

	prefix := "{dedup}:item:"
	values := []interface{}{"A", "B", "C", "A", "D", "B"} // дубликаты A, B
	inserted, err := script.Run(ctx, rdb, []string{prefix}, values...).Result()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	}
	fmt.Printf("Вставлено %d новых элементов (дубликаты пропущены)\n", inserted)
	// Проверяем, что создалось
	keys, _ := rdb.Keys(ctx, prefix+"*").Result()
	fmt.Printf("Создано ключей: %d\n", len(keys))
	rdb.Del(ctx, keys...)
}

// 6. ОЧЕРЕДЬ С ПРИОРИТЕТАМИ (АТОМАРНОЕ ИЗВЛЕЧЕНИЕ ИЗ НЕСКОЛЬКИХ)
func primer6() {
	fmt.Println("\n--- 6. Очередь с приоритетами ---")

	script := redis.NewScript(`
		local queues = KEYS
		local result = nil

		for _, q in ipairs(queues) do
			local val = redis.call('LPOP', q)
			if val then
				result = {queue = q, value = val}
				break
			end
		end
		return result
	`)

	// Очереди с приоритетами: сначала высокий, потом средний, потом низкий
	highKey := "{queue}:high"
	medKey := "{queue}:medium"
	lowKey := "{queue}:low"
	rdb.RPush(ctx, highKey, "high_1")
	rdb.RPush(ctx, medKey, "med_1")
	rdb.RPush(ctx, lowKey, "low_1")

	// Извлекаем задачу
	result, err := script.Run(ctx, rdb, []string{highKey, medKey, lowKey}).Result()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else if result == nil {
		fmt.Println("Все очереди пусты")
	} else {
		resMap := result.(map[interface{}]interface{})
		fmt.Printf("Извлечено: очередь=%v, значение=%v\n", resMap["queue"], resMap["value"])
	}
	rdb.Del(ctx, highKey, medKey, lowKey)
}

// 7. РАСПРЕДЕЛЁННАЯ БЛОКИРОВКА (SET NX + TTL + АТОМАРНОЕ ОСВОБОЖДЕНИЕ)
func primer7() {
	fmt.Println("\n--- 7. Распределённая блокировка ---")

	script := redis.NewScript(`
		local key = KEYS[1]
		local token = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local command = ARGV[3]          -- "lock" или "unlock"

		if command == "lock" then
			return redis.call('SET', key, token, 'NX', 'EX', ttl)
		elseif command == "unlock" then
			if redis.call('GET', key) == token then
				return redis.call('DEL', key)
			else
				return 0
			end
		else
			return nil
		end
	`)

	lockKey := "{lock}:resource"
	token := "client-1"
	ttl := 10

	// Захват блокировки
	locked, err := script.Run(ctx, rdb, []string{lockKey}, token, ttl, "lock").Result()
	if err != nil {
		fmt.Printf("Ошибка захвата: %v\n", err)
		return
	}
	if locked == "OK" {
		fmt.Println("Блокировка захвачена")
	} else {
		fmt.Println("Блокировка уже занята")
		return
	}

	// Имитация работы
	time.Sleep(2 * time.Second)

	// Освобождение (только если наш токен)
	unlocked, err := script.Run(ctx, rdb, []string{lockKey}, token, ttl, "unlock").Int()
	if err != nil {
		fmt.Printf("Ошибка освобождения: %v\n", err)
	} else if unlocked == 1 {
		fmt.Println("Блокировка освобождена")
	} else {
		fmt.Println("Не удалось освободить (токен не совпадает или блокировка истекла)")
	}
	rdb.Del(ctx, lockKey)
}

// 8. МАССОВОЕ УДАЛЕНИЕ ПО ПАТТЕРНУ (SCAN + DEL, БЕЗ KEYS)
func primer8() {
	fmt.Println("\n--- 8. Массовое удаление по паттерну (SCAN) ---")

	script := redis.NewScript(`
	local patter = KEYS[1]
	local cursor = '0'
	local deleted = 0

	repeat
		local results = redis.call('SCAN', cursor, 'MATCH', patter, 'COUNT', 100)
		cursor = result[1]
		local keys = result[2]
		if #key > 0 then
			deleted = deleted + redis.call('DEL', unpack(keys))
		end
	until cursor == '0'

	return deleted
	`)

	// Создаём тестовые ключи
	for i := 0; i < 50; i++ {
		rdb.Set(ctx, fmt.Sprintf("session:user:%d", i), "active", 0)
	}

	deleted, err := script.Run(ctx, rdb, []string{"session:user:*"}).Int()
	if err != nil {
		fmt.Printf("Ошибка удаления: %v\n", err)
	} else {
		fmt.Printf("Удалено %d ключей\n", deleted)
	}
	// Проверяем, что все удалены
	remaining, _ := rdb.Keys(ctx, "session:user:*").Result()
	fmt.Printf("Осталось ключей: %d\n", len(remaining))
}
