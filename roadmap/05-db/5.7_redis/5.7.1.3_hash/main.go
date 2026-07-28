package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 3. ХЕШИ (HASH) В REDIS

1. ЧТО ТАКОЕ ХЕШ?
   - Хеш — это структура данных, похожая на map или объект: ключ → (поля и значения).
   - Поля — это строки, значения — тоже строки (бинарно-безопасные).
   - Хеш идеален для хранения объектов с множеством атрибутов (профиль пользователя, настройки и т.д.).
   - Максимальное количество полей в хеше: 2³² - 1 (более 4 миллиардов).

2. ГЛАВНОЕ ПРИМЕНЕНИЕ:
   - Хранение профилей пользователей (имя, email, возраст, настройки).
   - Конфигурации и настройки приложения.
   - Кэширование объектов из БД, где нужно часто обновлять отдельные поля.
   - Счётчики по категориям (например, просмотры по дням).
   - Хранение временных данных с TTL для всего объекта (ключ удаляется целиком).

3. ОСНОВНЫЕ КОМАНДЫ И ИХ ДЕЙСТВИЕ (ПОДРОБНО)
   -------------------------------------------------------------------------------
   HSET key field value [field value ...]
   - Устанавливает одно или несколько полей в хеше.
   - Если поле уже существует, перезаписывает значение.
   - Возвращает количество новых полей (которые были добавлены, а не обновлены).
   - CLI: HSET user:1 name "Alice" age 30
   - Go: result, err := rdb.HSet(ctx, "user:1", "name", "Alice", "age", 30).Result()

   HGET key field
   - Возвращает значение поля.
   - Если поля нет, возвращает (nil) в CLI, в Go — redis.Nil.
   - CLI: HGET user:1 name → "Alice"
   - Go: val, err := rdb.HGet(ctx, "user:1", "name").Result()

   HMSET key field value [field value ...] (устаревшая, но ещё используется)
   - То же, что HSET, но без возврата количества новых полей (только ошибка).
   - В новых версиях Redis рекомендуется использовать HSET.
   - Go: err := rdb.HMSet(ctx, "user:1", "name", "Alice", "age", 30).Err()

   HGETALL key
   - Возвращает все поля и значения хеша в виде плоского списка.
   - В Go возвращает map[string]string.
   - CLI: HGETALL user:1 → ["name", "Alice", "age", "30"]
   - Go: data, err := rdb.HGetAll(ctx, "user:1").Result() // map[string]string

   HDEL key field [field ...]
   - Удаляет одно или несколько полей из хеша.
   - Возвращает количество удалённых полей.
   - CLI: HDEL user:1 age
   - Go: deleted, err := rdb.HDel(ctx, "user:1", "age").Result()

   HEXISTS key field
   - Проверяет существование поля в хеше.
   - Возвращает 1, если поле есть, иначе 0.
   - CLI: HEXISTS user:1 name → 1
   - Go: exists, err := rdb.HExists(ctx, "user:1", "name").Result() // bool

   HINCRBY key field increment
   - Атомарно увеличивает числовое значение поля на заданное целое число.
   - Если поля нет, создаёт его со значением 0 и затем инкрементит.
   - Возвращает новое значение (int64).
   - CLI: HINCRBY user:1 age 1 → 31
   - Go: newVal, err := rdb.HIncrBy(ctx, "user:1", "age", 1).Result()

   HKEYS key
   - Возвращает все поля хеша (только ключи, без значений).
   - CLI: HKEYS user:1 → ["name", "age"]
   - Go: keys, err := rdb.HKeys(ctx, "user:1").Result() // []string

   HVALS key
   - Возвращает все значения хеша (только значения, без ключей).
   - Порядок не гарантируется.
   - CLI: HVALS user:1 → ["Alice", "30"]
   - Go: vals, err := rdb.HVals(ctx, "user:1").Result() // []string

   Дополнительные команды (не в основном списке, но полезны):
   - HINCRBYFLOAT — инкремент с плавающей точкой.
   - HLEN — количество полей в хеше.
   - HSTRLEN — длина значения поля (в байтах).
   - HSETNX — установить поле, только если его нет.

4. ВАЖНЫЕ СВОЙСТВА И ПРАВИЛА:
   - Все операции с хешами O(1) — очень быстрые.
   - Хеш — это полноценный объект, и TTL устанавливается только на ключ целиком, а не на отдельные поля.
   - HSET с несколькими парами полей атомарно.
   - Если нужно хранить вложенные структуры, лучше использовать JSONB (в Redis Module) или сериализовать в строку.
   - Хеши экономят память при хранении большого количества мелких полей по сравнению со строковыми ключами (т.к. общий префикс ключа хранится один раз).

5. КОГДА НЕ ИСПОЛЬЗОВАТЬ ХЕШИ:
   - Для хранения больших бинарных данных (картинки, видео) — лучше строки.
   - Если нужны сортировка или уникальность значений — другие структуры.
   - Если поля часто меняются все вместе, возможно, проще сериализовать объект в строку.

6. СВЯЗЬ С GO (go-redis/v9):
   - HSet(ctx, key, values...) — *IntCmd (возвращает количество добавленных полей)
   - HGet(ctx, key, field) — *StringCmd
   - HMSet(ctx, key, values...) — *BoolCmd (устаревшая)
   - HGetAll(ctx, key) — *StringStringMapCmd (возвращает map[string]string)
   - HDel(ctx, key, fields...) — *IntCmd
   - HExists(ctx, key, field) — *BoolCmd
   - HIncrBy(ctx, key, field, incr) — *IntCmd
   - HKeys(ctx, key) — *StringSliceCmd
   - HVals(ctx, key) — *StringSliceCmd

7. ТИПИЧНЫЕ ОШИБКИ:
   - redis.Nil при HGet несуществующего поля (нужно обрабатывать).
   - HIncrBy на нечисловом значении вызывает ошибку.
   - Попытка использовать TTL на отдельных полях — не поддерживается, TTL только на весь хеш.

8. ПАТТЕРНЫ ИСПОЛЬЗОВАНИЯ:
   - Хранение пользовательских профилей: HSET user:42 name "Bob" email "bob@ex.com".
   - Кэш объектов БД: каждый объект хранится в хеше, поля соответствуют колонкам.
   - Счётчики метрик: HINCRBY stats:2024 "page_views" 1, HINCRBY stats:2024 "unique_visitors" 1.
   - Конфигурация приложения: HSET config "db_host" "localhost" "db_port" "5432".
*/

var rdb *redis.Client
var ctx = context.Background()

func init() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic("Redis no response: " + err.Error())
	}
}

func main() {
	fmt.Println("ПРАКТИКА ПО ХЕШАМ")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. HSET, HGET
func primer1() {
	fmt.Println("--- 1. HSET, HGET ---")
	key := "user:1"
	rdb.Del(ctx, key)

	// Устанавливаем поля
	added, err := rdb.HSet(ctx, key, "name", "Kolya", "age", 25).Result()
	if err != nil {
		panic(err)
	}
	fmt.Printf("HSET: добавлено %d новых полей\n", added)

	// Получаем поля
	name, _ := rdb.HGet(ctx, key, "name").Result()
	age, _ := rdb.HGet(ctx, key, "age").Result()
	fmt.Printf("HGET: name=%s, age=%s\n", name, age)

	// Попытка получить несуществующее поле
	_, err = rdb.HGet(ctx, key, "email").Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("HGET email -> redis.Nil (поля нет)")
	}

	// Обновляем существующее поле (HSET перезаписывает)
	rdb.HSet(ctx, key, "age", 26)
	age, _ = rdb.HGet(ctx, key, "age").Result()
	fmt.Printf("После обновления age=%s\n", age)

	rdb.Del(ctx, key)
}

// 2. HGETALL — получить все поля
func primer2() {
	fmt.Println("--- 2. HGETALL ---")
	key := "user:2"
	rdb.Del(ctx, key)

	rdb.HSet(ctx, key, "name", "Bob", "email", "bob@mail.com", "city", "NYC")
	data, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		panic(err)
	}
	fmt.Printf("HGETALL: %v\n", data) // map[string]string

	// Если хеш пуст, возвращает пустую map
	emptyKey := "empty"
	rdb.Del(ctx, emptyKey)
	emptyData, _ := rdb.HGetAll(ctx, emptyKey).Result()
	fmt.Printf("HGETALL на пустом хеше: %v (пустая map)\n", emptyData)

	rdb.Del(ctx, key)
}

// 3. HDEL и HEXISTS
func primer3() {
	fmt.Println("--- 3. HDEL, HEXISTS ---")
	key := "user:3"
	rdb.Del(ctx, key)
	rdb.HSet(ctx, key, "name", "Charlie", "age", 30, "city", "LA")

	// Проверяем существование поля
	exists, _ := rdb.HExists(ctx, key, "city").Result()
	fmt.Printf("HEXISTS city -> %v\n", exists)

	// Удаляем поле
	deleted, _ := rdb.HDel(ctx, key, "city").Result()
	fmt.Printf("HDEL city -> удалено %d полей\n", deleted)

	// Проверяем снова
	exists, _ = rdb.HExists(ctx, key, "city").Result()
	fmt.Printf("HEXISTS после HDEL -> %v\n", exists)

	// Удаляем несколько полей
	deleted, _ = rdb.HDel(ctx, key, "name", "age").Result()
	fmt.Printf("HDEL name age -> удалено %d полей\n", deleted)

	rdb.Del(ctx, key)
}

// 4. HINCRBY — атомарное увеличение числовых полей
func primer4() {
	fmt.Println("--- 4. HINCRBY ---")
	key := "stats:page"
	rdb.Del(ctx, key)

	// Увеличиваем счётчик просмотров
	newVal, _ := rdb.HIncrBy(ctx, key, "views", 1).Result()
	fmt.Printf("HINCRBY views +1 -> %d\n", newVal)

	// Увеличиваем на 5
	newVal, _ = rdb.HIncrBy(ctx, key, "views", 5).Result()
	fmt.Printf("HINCRBY views +5 -> %d\n", newVal)

	// Увеличиваем несуществующее поле — создаётся со значением 0 и инкрементится
	newVal, _ = rdb.HIncrBy(ctx, key, "likes", 1).Result()
	fmt.Printf("HINCRBY likes (новое) +1 -> %d\n", newVal)

	// Попытка инкрементировать нечисловое поле
	rdb.HSet(ctx, key, "bad", "abc")
	_, err := rdb.HIncrBy(ctx, key, "bad", 1).Result()
	if err != nil {
		fmt.Printf("HINCRBY на 'abc' -> ошибка: %v\n", err)
	}
	rdb.Del(ctx, key)
}

// 5. HKEYS и HVALS
func primer5() {
	fmt.Println("--- 5. HKEYS, HVALS ---")
	key := "user:4"
	rdb.Del(ctx, key)
	rdb.HSet(ctx, key, "name", "Diana", "age", "28", "city", "SF")

	// Получаем ключи (поля)
	keys, _ := rdb.HKeys(ctx, key).Result()
	fmt.Printf("HKEYS: %v\n", keys)

	// Получаем значения
	vals, _ := rdb.HVals(ctx, key).Result()
	fmt.Printf("HVALS: %v\n", vals)

	// Обратите внимание: порядок не гарантируется, но в пределах одного вызова он одинаков
	rdb.Del(ctx, key)
}

// 6. Хеш как объект — профиль пользователя с настройками
func primer6() {
	fmt.Println("--- 6. Хеш как объект (профиль пользователя) ---")
	userKey := "user:123"
	rdb.Del(ctx, userKey)

	// Сохраняем профиль
	rdb.HSet(ctx, userKey,
		"id", "123",
		"name", "John Doe",
		"email", "john@example.com",
		"preferences", `{"theme":"dark","notifications":true}`,
		"created_at", time.Now().Format(time.RFC3339),
	)

	// Получаем весь профиль
	profile, _ := rdb.HGetAll(ctx, userKey).Result()
	fmt.Println("Профиль пользователя:")
	for k, v := range profile {
		fmt.Printf("  %s: %s\n", k, v)
	}

	// Обновляем только email
	rdb.HSet(ctx, userKey, "email", "kent.k@mail.ru")
	newEmail, _ := rdb.HGet(ctx, userKey, "email").Result()
	fmt.Printf("Обновлён email: %s\n", newEmail)

	rdb.Del(ctx, userKey)
}

// 7. Счётчики по категориям — использование HINCRBY
func primer7() {
	fmt.Println("--- 7. Счётчики по категориям ---")
	key := "stats:2024-01"
	rdb.Del(ctx, key)

	// Увеличиваем счётчики для разных категорий
	// Увеличиваем счётчики для разных категорий
	rdb.HIncrBy(ctx, key, "views", 10)
	rdb.HIncrBy(ctx, key, "views", 5) // теперь 15
	rdb.HIncrBy(ctx, key, "clicks", 3)
	rdb.HIncrBy(ctx, key, "clicks", 2) // теперь 5

	stats, _ := rdb.HGetAll(ctx, key).Result()
	fmt.Println("Статистика:")
	for category, count := range stats {
		fmt.Printf("  %s: %s\n", category, count)
	}

	// Получаем конкретную метрику
	views, _ := rdb.HGet(ctx, key, "views").Result()
	fmt.Printf("Всего просмотров: %s\n", views)

	rdb.Del(ctx, key)
}

// 8. HSETNX (установить поле, только если его нет) и TTL
func primer8() {
	fmt.Println("--- 8. HSETNX и TTL для хеша ---")
	key := "config"
	rdb.Del(ctx, key)

	// HSETNX — устанавливает поле, если его нет
	ok, _ := rdb.HSetNX(ctx, key, "mode", "prod").Result()
	fmt.Printf("HSETNX mode (первый раз) -> %v\n", ok)

	// Повторная попытка — вернёт false
	ok, _ = rdb.HSetNX(ctx, key, "mode", "dev").Result()
	fmt.Printf("HSETNX mode (второй раз) -> %v\n", ok)

	// Проверяем, что значение не изменилось
	mode, _ := rdb.HGet(ctx, key, "mode").Result()
	fmt.Printf("Текущее mode: %s\n", mode)

	// Устанавливаем TTL на весь хеш (ключ будет удалён через 5 секунд)
	rdb.Expire(ctx, key, 5*time.Second)
	ttl, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL для хеша: %v\n", ttl)

	time.Sleep(6 * time.Second)
	_, err := rdb.HGetAll(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("Хеш истёк и удалён целиком")
	} else {
		fmt.Println("Хеш всё ещё существует (ошибка)")
	}
	rdb.Del(ctx, key)
}
