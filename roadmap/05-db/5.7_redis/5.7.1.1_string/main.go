package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 1. СТРОКИ (STRING) В REDIS

1. ЧТО ТАКОЕ СТРОКА?
  - Простейший тип данных в Redis: ключ → значение.
  - Значение — бинарно-безопасная строка (может хранить текст, числа, JSON, изображения,
    сериализованные объекты, любые байты).
  - Максимальный размер: 512 МБ (на практике редко превышает несколько килобайт,
    исключение — кэширование больших объектов).

2. ГЛАВНОЕ ПРИМЕНЕНИЕ:

  - Кэширование результатов БД, HTML-страниц, API-ответов.

  - Счётчики (просмотры, лайки, онлайн-пользователи) — через атомарные INCR/DECR.

  - Временные данные: сессии, OTP-коды, токены сброса пароля — через EXPIRE/TTL.

  - Распределённые блокировки — через SET NX EX.

  - Хранение конфигураций, флагов функций.

    3. ОСНОВНЫЕ КОМАНДЫ И ИХ ДЕЙСТВИЕ (ПОДРОБНО)
    -------------------------------------------------------------------------------
    SET key value [EX seconds] [PX milliseconds] [NX|XX]

  - Устанавливает значение для ключа.

  - Если ключ уже существует, перезаписывает значение (по умолчанию).

  - Опции:
    EX seconds     — установить время жизни в секундах.
    PX milliseconds — установить время жизни в миллисекундах.
    NX             — установить, только если ключа нет (Not eXists).
    XX             — установить, только если ключ уже существует (eXists).

  - Возвращает: "OK" при успехе, nil при ошибке.

  - CLI пример: SET user:1 "Alice" EX 60 NX

  - Go: rdb.Set(ctx, key, value, expiration).Err()
    (expiration = 0 для бессрочного)

    -------------------------------------------------------------------------------
    GET key

  - Возвращает значение, связанное с ключом.

  - Если ключ не существует, возвращает (nil) в CLI, в Go — ошибка redis.Nil.

  - CLI пример: GET user:1  → "Alice"

  - Go: val, err := rdb.Get(ctx, key).Result()
    -------------------------------------------------------------------------------
    DEL key [key ...]

  - Удаляет один или несколько ключей.

  - Возвращает количество удалённых ключей.

  - CLI пример: DEL user:1 session:abc

  - Go: deleted, err := rdb.Del(ctx, key1, key2).Result()
    -------------------------------------------------------------------------------
    EXISTS key [key ...]

  - Проверяет существование одного или нескольких ключей.

  - Возвращает количество существующих ключей (не значение).

  - CLI пример: EXISTS user:1  → 1 (если есть)

  - Go: exists, err := rdb.Exists(ctx, key).Result()
    -------------------------------------------------------------------------------
    EXPIRE key seconds

  - Устанавливает время жизни ключа в секундах.

  - Если ключ не существует, возвращает 0 (не удалось).

  - Возвращает 1 при успехе, 0 если ключ не существует или TTL не установлен.

  - CLI пример: EXPIRE user:1 60

  - Go: ok, err := rdb.Expire(ctx, key, 60*time.Second).Result()
    -------------------------------------------------------------------------------
    TTL key

  - Возвращает оставшееся время жизни ключа в секундах.

  - Возвращает:

  - положительное число — оставшиеся секунды,

  - -1 — ключ существует, но без TTL,

  - -2 — ключ не существует.

  - CLI пример: TTL user:1  → 45

  - Go: ttl, err := rdb.TTL(ctx, key).Result() // time.Duration
    -------------------------------------------------------------------------------
    INCR key

  - Атомарно увеличивает значение ключа на 1 (значение должно быть числом).

  - Если ключ не существует, сначала устанавливает 0, затем увеличивает.

  - Возвращает новое значение (int64).

  - CLI пример: INCR counter  → 11

  - Go: newVal, err := rdb.Incr(ctx, key).Result()
    -------------------------------------------------------------------------------
    DECR key

  - Атомарно уменьшает значение ключа на 1.

  - Аналогично INCR, но в обратную сторону.

  - CLI пример: DECR counter → 10

  - Go: newVal, err := rdb.Decr(ctx, key).Result()
    -------------------------------------------------------------------------------
    INCRBY key increment

  - Атомарно увеличивает значение на заданное целое число (может быть отрицательным).

  - Возвращает новое значение.

  - CLI пример: INCRBY counter 5 → 15

  - Go: newVal, err := rdb.IncrBy(ctx, key, 5).Result()
    -------------------------------------------------------------------------------
    APPEND key value

  - Добавляет переданное значение в конец существующей строки.

  - Если ключа нет, создаёт его со значением value (как SET).

  - Возвращает новую длину строки (в байтах).

  - CLI пример: APPEND greeting ", World!" → 13 (новая длина)

  - Go: newLen, err := rdb.Append(ctx, key, value).Result()
    -------------------------------------------------------------------------------
    STRLEN key

  - Возвращает длину строки, хранящейся по ключу (в байтах).

  - Если ключа нет, возвращает 0 (ошибки нет, просто 0).

  - CLI пример: STRLEN greeting → 13

  - Go: length, err := rdb.StrLen(ctx, key).Result()
    -------------------------------------------------------------------------------
    ДОПОЛНИТЕЛЬНО (используются реже, но полезны):

  - SETRANGE key offset value  — заменяет часть строки, начиная с offset.

  - GETRANGE key start end    — возвращает подстроку.

  - MSET / MGET               — массовое чтение/запись нескольких ключей.

  - INCRBYFLOAT               — инкремент с плавающей точкой.

  - SETBIT / GETBIT           — работа с битами (используется для компактных флагов).

4. ВАЖНЫЕ СВОЙСТВА И ПРАВИЛА:
  - Атомарность: INCR, DECR, APPEND, SET с условиями (NX/XX) выполняются как одна
    операция — безопасно при конкурентных запросах.
  - Опции SET:
  - NX — установить, только если ключа нет (используется для блокировок).
  - XX — установить, только если ключ уже существует.
  - EX/PX — установить TTL одновременно с записью (экономит round-trip).
  - Числа: INCR автоматически преобразует строку в число; если значение не число —
    возвращается ошибка.
  - GET отсутствующего ключа возвращает (nil) в CLI, в go-redis — ошибка redis.Nil.
  - TTL возвращает:
  - -2 — ключ не существует,
  - -1 — ключ существует, но без TTL,
  - положительное число — оставшиеся секунды.
  - Все основные команды работают за O(1).

5. КОГДА НЕ ИСПОЛЬЗОВАТЬ СТРОКИ:
  - Для хранения структуры с множеством полей — лучше Hash (HSET/HGET).
  - Для уникальных элементов — лучше Set (SADD/SMEMBERS).
  - Для сортированных данных — лучше Sorted Set (ZADD/ZRANGE).
  - Для работы с битами, картами, гиперлогами — есть специализированные типы.

6. СВЯЗЬ С GO (библиотека go-redis/v9):
  - Подключение: redis.NewClient(&redis.Options{Addr, Password, DB})
  - Контекст: все методы принимают context.Context для таймаутов и отмены.
  - Основные методы:
  - Set(ctx, key, value, expiration) — *StatusCmd (ошибка, если не удалось).
  - Get(ctx, key) — *StringCmd. Значение через .Result(); при отсутствии ключа
    возвращает redis.Nil (проверяем через errors.Is(err, redis.Nil)).
  - Del(ctx, keys...) — *IntCmd (количество удалённых ключей).
  - Exists(ctx, keys...) — *IntCmd (количество существующих ключей).
  - Expire(ctx, key, duration) — *BoolCmd (true, если TTL установлен).
  - TTL(ctx, key) — *DurationCmd (возвращает time.Duration; может быть -1 или -2).
  - Incr(ctx, key) — *IntCmd (новое значение int64).
  - Decr(ctx, key) — *IntCmd.
  - IncrBy(ctx, key, increment) — *IntCmd.
  - Append(ctx, key, value) — *IntCmd (новая длина строки).
  - StrLen(ctx, key) — *IntCmd (длина строки).
  - SetNX(ctx, key, value, expiration) — *BoolCmd (true — установлено, false — ключ уже есть).
  - Особенности:
  - Для SET с опцией XX используйте SetArgs: rdb.SetArgs(ctx, key, value,
    redis.SetArgs{XX: true, TTL: time.Second}).
  - Для больших объёмов данных используйте Pipeline для пакетной отправки.

7. ТИПИЧНЫЕ ОШИБКИ И ИХ ОБРАБОТКА:
  - redis.Nil — не является фатальной ошибкой, её нужно отдельно обрабатывать.
  - Ошибка при INCR на нечисловом значении: ошибка типа *redis.Error, можно проверить
    через errors.Is(err, redis.Err) или по тексту.
  - Таймауты: передавайте контекст с deadline, чтобы не вешать горутины.
  - Не копируйте клиент (под капотом есть пул соединений), используйте один экземпляр.

8. ПАТТЕРНЫ ИСПОЛЬЗОВАНИЯ (в контексте Go):
  - Cache-Aside: сначала GET, при redis.Nil — загружаем из БД и делаем SET.
  - Счётчик с TTL: INCR + EXPIRE, например, для ограничения запросов (rate limiting).
  - Блокировка: SetNX с TTL, проверка результата, при успехе — выполняем работу,
    потом DEL.
  - Атомарное обновление: использование WATCH + MULTI/EXEC (это уже выходит за рамки
    строк, но упоминаем).
*/
var (
	rdb *redis.Client
	ctx = context.Background()
)

func init() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic("Redis no response: " + err.Error())
	}
}

func main() {
	fmt.Println("ПРАКТИКА")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. SET и GET
func primer1() {
	fmt.Println("--- 1. SET / GET ---")
	key := "user:1"

	// Установка
	err := rdb.Set(ctx, key, "Kolya", 0).Err()
	if err != nil {
		panic(err)
	}
	fmt.Println("SET user:1 = Alice")

	// Получение
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		panic(err)
	}
	fmt.Printf("GET user:1 = %s\n", val)

	// Перезапись
	rdb.Set(ctx, key, "Bobka", 0).Err()
	val, _ = rdb.Get(ctx, key).Result()
	fmt.Printf("После перезаписи: %s\n", val)

	rdb.Del(ctx, key)
	fmt.Println()
}

// 2. DEL и EXISTS
func primer2() {
	fmt.Println("--- 2. DEL / EXISTS ---")
	key := "temp"
	rdb.Set(ctx, key, "value", 0)

	// Проверяем существование
	exists, _ := rdb.Exists(ctx, key).Result()
	fmt.Printf("EXISTS temp = %d (1 - есть)\n", exists)

	// Удаляем
	deleted, _ := rdb.Del(ctx, key).Result()
	fmt.Printf("DEL temp -> удалено %d ключей\n", deleted)

	// Проверяем снова
	exists, _ = rdb.Exists(ctx, key).Result()
	fmt.Printf("EXISTS после DEL = %d (0 - нет)\n", exists)

	fmt.Println()
}

// 3. EXPIRE и TTL
func primer3() {
	fmt.Println("--- 3. EXPIRE / TTL ---")
	key := "session:abc"
	rdb.Set(ctx, key, "active", 0)

	// Устанавливаем TTL = 5 секунд
	ok, _ := rdb.Expire(ctx, key, 5*time.Second).Result()

	fmt.Printf("EXPIRE session:abc 5s -> %v\n", ok)

	// Проверяем TTL
	ttl, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL = %v\n", ttl)

	// Ждём 6 секунд и проверяем
	time.Sleep(6 * time.Second)
	_, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("Ключ истёк и удалён")
	}
	fmt.Println("Ключ всё ещё существует (ошибка)")
}

// 4. INCR и DECR
func primer4() {
	fmt.Println("--- 4. INCR / DECR ---")
	key := "views"

	// Устанавливаем начальное значение
	rdb.Set(ctx, key, 10, 0)

	// Инкремент
	newVal, _ := rdb.Incr(ctx, key).Result()
	fmt.Printf("INCR -> %d\n", newVal)

	// Декремент
	newVal, _ = rdb.Decr(ctx, key).Result()
	fmt.Printf("DECR -> %d\n", newVal)

	// Инкремент на пустом ключе (создаётся со значением 0)
	emptyKey := "newcounter"
	val, _ := rdb.Incr(ctx, emptyKey).Result()
	fmt.Printf("INCR на несуществующем ключе -> %d (установлен 0+1)\n", val)

	// Попытка инкремента на строке
	rdb.Set(ctx, "bad", "abc", 0)
	_, err := rdb.Incr(ctx, "bad").Result()
	if err != nil {
		fmt.Printf("Ошибка INCR на 'abc': %v\n", err)
	}
	rdb.Del(ctx, key, emptyKey, "bad")
}

// 5. APPEND и STRLEN
func primer5() {
	fmt.Println("--- 5. APPEND / STRLEN ---")
	key := "msg"
	rdb.Set(ctx, key, "Hello", 0)

	// Добавляем
	newLen, _ := rdb.Append(ctx, key, "World").Result()
	fmt.Printf("APPEND -> новая длина = %d\n", newLen)

	// Длина строки
	length, _ := rdb.StrLen(ctx, key).Result()
	fmt.Printf("STRLEN msg = %d\n", length)

	rdb.Del(ctx, key)
}

// 6. Комбинированный сценарий: счётчик просмотров с TTL
func primer6() {
	fmt.Println("--- 6. Комбо: счётчик + TTL ---")
	userID := "42"
	key := "views" + userID

	// Увеличиваем счётчик
	newVal, _ := rdb.Incr(ctx, key).Result()
	fmt.Printf("Просмотров: %d\n", newVal)

	// Устанавливаес TTL
	rdb.Expire(ctx, key, 10*time.Second)
	ttl, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL = %v\n", ttl)

	// Получаем значение
	val, _ := rdb.Get(ctx, key).Result()
	fmt.Printf("Текущее значение: %s\n", val)

	// Увеличиваем ещё раз
	rdb.Incr(ctx, key)
	val, _ = rdb.Get(ctx, key).Result()
	fmt.Printf("После ещё одного просмотра: %s\n", val)

	// Очистка
	rdb.Del(ctx, key)
}

// 7. Работа с redis.Nil (обработка отсутствия ключа)
func primer7() {
	fmt.Println("--- 7. Обработка redis.Nil ---")
	key := "unknown"

	val, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("GET unknown -> redis.Nil (ключа нет)")
	} else if err != nil {
		panic(err)
	} else {
		fmt.Printf("Значение: %s\n", val)
	}

	// EXISTS тоже показывает 0
	exists, _ := rdb.Exists(ctx, key).Result()
	fmt.Printf("EXISTS unknown = %d\n", exists)

	// TTL для отсутствующего ключа возвращает -2
	ttl, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL unknown = %v (-2 - ключа нет)\n", ttl)
}

// 8. Проверка TTL после PERSIST (добавим PERSIST, хоть и не в списке,
// но это часть работы с TTL, можно упомянуть)
func primer8() {
	fmt.Println("--- 8. PERSIST (снятие TTL) ---")
	key := "temp"
	rdb.Set(ctx, key, "value", 0).Result()
	rdb.Expire(ctx, key, 10*time.Second)

	ttlBefore, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL до PERSIST: %v\n", ttlBefore)

	// Снимаем TTL
	ok, _ := rdb.Persist(ctx, key).Result()
	fmt.Printf("PERSIST -> %v\n", ok)

	ttlAfter, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("TTL после PERSIST: %v (-1 - бессрочный)\n", ttlAfter)

	rdb.Del(ctx, key)
}
