package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 2.1. ВРЕМЯ ЖИЗНИ (TTL) В REDIS

1. ЗАЧЕМ НУЖЕН TTL?
   - Redis — это in-memory база, память не резиновая.
   - Если данные не удалять, они будут занимать место вечно, и рано или поздно Redis упрётся в лимит.
   - TTL (Time To Live) позволяет автоматически удалять ключи через заданный промежуток времени.
   - Это основа для кэширования, хранения сессий, OTP-кодов, временных токенов.

2. КАК ЭТО РАБОТАЕТ ВНУТРИ?
   - У каждого ключа есть поле expire (время истечения) в миллисекундах.
   - Когда время истекает, ключ становится «мёртвым» и удаляется при следующей попытке доступа (ленивое удаление) или в фоновом процессе (активное удаление).
   - Механизм:
     1. Ленивое удаление (Lazy expiration):
        - Когда ты пытаешься получить ключ (GET, EXISTS, TTL), Redis проверяет, не истёк ли он.
        - Если истёк — удаляет и возвращает nil.
     2. Активное удаление (Active expiration):
        - Redis каждые 100 мс запускает фоновый процесс, который проверяет случайные ключи с TTL.
        - Если ключ истёк — удаляет.
   - Почему не удаляются сразу?
     - Потому что Redis не хранит список ключей по времени истечения — это было бы слишком дорого.
     - Активное удаление + ленивое — компромисс между производительностью и освобождением памяти.

3. КОМАНДЫ ДЛЯ РАБОТЫ С TTL:
   - EXPIRE key seconds         — установить TTL в секундах.
   - PEXPIRE key milliseconds   — установить TTL в миллисекундах.
   - EXPIREAT key timestamp     — установить TTL по абсолютной временной метке (Unix time в секундах).
   - PEXPIREAT key timestamp    — аналогично, но в миллисекундах.
   - TTL key                    — получить оставшееся время в секундах.
   - PTTL key                   — получить оставшееся время в миллисекундах.
   - PERSIST key                — снять TTL (сделать ключ вечным).

4. ВОЗВРАЩАЕМЫЕ ЗНАЧЕНИЯ TTL / PTTL:
   - > 0 — оставшееся время.
   - -1 — ключ существует, но TTL не установлен (бессрочный).
   - -2 — ключ не существует.

5. ВОЗВРАЩАЕМЫЕ ЗНАЧЕНИЯ EXPIRE / PEXPIRE:
   - 1 — TTL успешно установлен.
   - 0 — ключ не существует или TTL не установлен (например, отрицательное значение).
   - В go-redis это возвращается как bool: true — установлен, false — не удалось.

6. ОСОБЕННОСТИ И ПОДВОДНЫЕ КАМНИ:
   6.1. EXPIRE перезаписывает существующий TTL.
        - Если у ключа уже есть TTL, то EXPIRE заменит его на новое значение.
        - Это работает, даже если новый TTL больше старого.

   6.2. Если ключ удалён, TTL для него больше не существует.
        - DEL удаляет ключ и его TTL.
        - Если ключ истёк — он удаляется автоматически.

   6.3. SET с опцией EX или PX устанавливает TTL атомарно.
        - SET key value EX 10 — установить ключ и TTL за одну операцию.
        - Это сокращает число round-trip и гарантирует, что ключ не будет создан без TTL.

   6.4. TTL не сбрасывается при обновлении значения через SET.
        - SET key new_value не влияет на TTL.
        - Если хочешь обновить и TTL, нужно либо использовать SET с EX/PX, либо отдельно EXPIRE.

   6.5. TTL для хешей, списков, множеств.
        - TTL устанавливается только на весь ключ, а не на отдельные поля/элементы.
        - Это важно помнить: нельзя поставить TTL на одно поле в хеше.

   6.6. TTL при использовании RENAME.
        - Если переименовать ключ, TTL сохраняется у нового ключа.

   6.7. PERSIST
        - Удаляет TTL, делая ключ вечным.
        - Если у ключа нет TTL, PERSIST вернёт 0 (ничего не делает).

   6.8. TTL и репликация.
        - TTL корректно реплицируется на реплики.
        - Если мастер удаляет ключ по истечении TTL, команда DEL реплицируется.

7. КАК ИСПОЛЬЗОВАТЬ TTL В РАЗНЫХ СЦЕНАРИЯХ:

   7.1. Кэширование результатов БД
        - Храним данные 5 минут.
        - Если ключ истёк, следующий запрос перезагрузит данные из БД.

   7.2. Сессии пользователей
        - Сессия живёт 24 часа.
        - Каждый раз, когда пользователь активен, продлеваем TTL (EXPIRE).

   7.3. Rate Limiting
        - Счётчик запросов для пользователя.
        - TTL = 1 минута, INCR увеличивает счётчик.
        - Если TTL истёк — счётчик сбрасывается.

   7.4. OTP-коды и токены
        - Одноразовые пароли живут 5 минут.
        - По истечении — автоматически удаляются, и код нельзя использовать.

   7.5. Отложенные задачи (через список)
        - Можно хранить задачи с временем выполнения, и удалять их по истечении TTL.

8. МОНИТОРИНГ TTL:
   - Можно посмотреть все ключи с TTL через SCAN и проверку TTL.
   - INFO stats показывает expired_keys — сколько ключей истекло за всё время.
   - Это полезно, чтобы понять, эффективно ли работает активное удаление.

9. ПРАКТИЧЕСКИЕ СОВЕТЫ:
   - Всегда устанавливай TTL на кэшируемые данные.
   - Используй SET EX вместо SET + EXPIRE для атомарности.
   - Для сессий продлевай TTL при каждом обращении (Expire).
   - Помни, что TTL не точный до миллисекунды — активное удаление может задерживаться.

10. СВЯЗЬ С GO (go-redis/v9):
    - Expire(ctx, key, duration) — *BoolCmd.
    - PExpire(ctx, key, duration) — *BoolCmd.
    - TTL(ctx, key) — *DurationCmd (возвращает time.Duration).
    - PTTL(ctx, key) — *DurationCmd (но можно использовать TTL).
    - Persist(ctx, key) — *BoolCmd.
    - При SET можно передать expiration: rdb.Set(ctx, key, value, 10*time.Second).
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
	fmt.Println("=== TTL ===\n")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. Rate Limiter (ограничение запросов) на базе TTL
func primer1() {
	fmt.Println("--- 1. Rate Limiter (лимит 10 запросов в минуту) ---")
	key := "rate:user:123"
	limit := 10

	// Проверяем текущее количество
	count, err := rdb.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		count = 0
	} else if err != nil {
		panic(err)
	}

	if count >= int64(limit) {
		fmt.Printf("Лимит превышен: %d/%d\n", count, limit)
		return
	}

	// Увеличиваем счётчик
	newCount, _ := rdb.Incr(ctx, key).Result()

	// Устанавливаем TTL, если ключ новый
	if count == 0 {
		rdb.Expire(ctx, key, time.Second)
	}
	fmt.Printf("✅ Запрос разрешён: %d/%d (TTL = %v)\n", newCount, limit, time.Second)
}

// 2. Кэш с "плавающим" TTL (обновление при доступе)
func primer2() {
	fmt.Println("--- 2. Кэш с продлением TTL при обращении (Sliding Expiration) ---")
	key := "cache:user:42"
	ttl := 10 * time.Second

	// Имитация загрузки из БД
	getFromDB := func() string {
		fmt.Println("  -> Загружаем из БД (дорого)")
		return "user_data_from_db"
	}

	// Получаем данные (с кэшем)
	val, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Кэш-промах
		val = getFromDB()
		rdb.Set(ctx, key, val, ttl)
		fmt.Printf("Данные закешированы на %v\n", ttl)
	} else if err != nil {
		panic(err)
	} else {
		// Кэш-хит — продлеваем TTL
		rdb.Expire(ctx, key, ttl)
		fmt.Printf("Кэш-хит, TTL продлён до %v\n", ttl)
	}

	// Проверяем TTL
	remaining, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("Оставшееся время: %v\n", remaining)
}

// 3. Распределённая блокировка с авто-продлением (auto-refresh)
func primer3() {
	fmt.Println("--- 3. Блокировка с авто-продлением для долгих операций ---")
	lockKey := "lock:resource"
	lockValue := "worker-1"
	ttl := 5 * time.Second

	// Пытаемся захватить блокировку
	ok, err := rdb.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		panic(err)
	}
	if !ok {
		fmt.Println("Блокировка уже занята")
		return
	}
	fmt.Printf("Блокировка захвачена на %v\n", ttl)

	// Запускаем горутину для продления блокировки
	stopRefresh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(ttl / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Проверяем, что блокировка всё ещё наша (сравниваем значение)
				currentVal, err := rdb.Get(ctx, lockKey).Result()
				if err != nil || currentVal != lockValue {
					fmt.Println("Блокировка потеряна, прекращаем продление")
					return
				}
				// Продлеваем
				rdb.Expire(ctx, lockKey, ttl)
				fmt.Printf("Блокировка продлена до %v\n", ttl)
			case <-stopRefresh:
				return
			}
		}
	}()

	// Имитация долгой работы (10 секунд)
	fmt.Println("Выполняем работу... (10 сек)")
	time.Sleep(10 * time.Second)

	// Останавливаем продление и освобождаем блокировку
	close(stopRefresh)
	wg.Wait()
	rdb.Del(ctx, lockKey)
	fmt.Println("Блокировка освобождена")
}

// 4. Очередь отложенных задач (с TTL для времени выполнения)
func primer4() {
	fmt.Println("--- 4. Очередь отложенных задач (timestamp как TTL) ---")
	// Добавляем задачу, которая должна выполниться через 3 секунды
	executeAt := time.Now().Add(3 * time.Second)
	taskID := "task:123"
	// Сохраняем задачу с TTL = время до выполнения + запас
	err := rdb.Set(ctx, taskID, "send_email", 4*time.Second)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Задача %s запланирована на %s (TTL = %v)\n", taskID, executeAt.Format("15:04:05"), 4*time.Second)
}

// 5. Семафор для ограничения одновременных операций
func primer5() {
	fmt.Println("--- 5. Семафор с TTL (ограничение до 2 одновременных) ---")
	semKey := "semaphore:workers"
	max := 2
	ttl := 5 * time.Second

	// Пытаемся занять слот
	// Используем INCR + EXPIRE, но INCR не атомарен с EXPIRE, поэтому используем Lua
	script := redis.NewScript(`
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local current = redis.call('INCR', key)
		if current == 1 then
			redis.call('EXPIRE', key, ttl)
		end
		if current > max then
			redis.call('DECR', key)
			return 0
		end
		return current
	`)

	// Имитация 5 конкурентных запросов
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			val, err := script.Run(ctx, rdb, []string{semKey}, max, int(ttl.Seconds())).Int()
			if err != nil {
				fmt.Printf("  Горутина %d: ошибка\n", id)
				return
			}
			if val == 0 {
				fmt.Printf("  Горутина %d: ❌ не удалось занять слот (лимит)\n", id)
			} else {
				fmt.Printf("  Горутина %d: ✅ заняла слот (всего %d)\n", id, val)
				// Имитация работы
				time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
				// Освобождаем
				rdb.Decr(ctx, semKey)
			}
		}(i)
	}
	wg.Wait()
	rdb.Del(ctx, semKey)
	fmt.Println()
}

// 6. Сессия с обновлением активности (rolling TTL)
func primer6() {
	fmt.Println("--- 6. Сессия пользователя с обновлением TTL при активности ---")
	sessionKey := "session:user:456"
	ttl := 10 * time.Second

	// Создаём сессию при логине
	rdb.Set(ctx, sessionKey, "active", ttl)
	fmt.Printf("Сессия создана, TTL = %v\n", ttl)

	// Имитация активности: каждые 2 секунды обновляем TTL
	for i := 0; i < 4; i++ {
		time.Sleep(2 * time.Second)
		// Проверяем, жива ли сессия
		_, err := rdb.Get(ctx, sessionKey).Result()
		if errors.Is(err, redis.Nil) {
			fmt.Println("  ❌ Сессия истекла (пользователь не активен)")
			break
		}
		// Обновляем TTL
		rdb.Expire(ctx, sessionKey, ttl)
		fmt.Printf("  🔄 Активность, TTL обновлён до %v\n", ttl)
	}

	// После цикла сессия жива
	rdb.Del(ctx, sessionKey)
}

// 7. Ограничение попыток с блокировкой по TTL
func primer7() {
	fmt.Println("--- 7. Ограничение попыток входа (блокировка на TTL) ---")
	user := "alice"
	failKey := "login:fail:" + user
	blockKey := "login:block:" + user
	maxAttempts := 3
	blockDuration := 15 * time.Second

	// Симуляция неудачных попыток
	for attempt := 1; attempt <= 5; attempt++ {
		// Проверяем, не заблокирован ли
		_, err := rdb.Get(ctx, blockKey).Result()
		if err == nil {
			fmt.Printf("❌ Пользователь %s заблокирован на %v\n", user, blockDuration)
			break
		}

		// Увеличиваем счётчик неудач
		fails, _ := rdb.Incr(ctx, failKey).Result()
		rdb.Expire(ctx, failKey, 60*time.Second) // сбрасываем через минуту

		if fails >= int64(maxAttempts) {
			// Блокируем
			rdb.Set(ctx, blockKey, "blocked", blockDuration)
			fmt.Printf("🔒 Пользователь %s заблокирован на %v (превышено попыток)\n", user, blockDuration)
			break
		}
		fmt.Printf("  Попытка %d неудачна (%d/%d)\n", attempt, fails, maxAttempts)
	}

	// Очистка
	rdb.Del(ctx, failKey, blockKey)
}

// 8. Адаптивное TTL для горячих данных
func primer8() {
	fmt.Println("--- 8. Адаптивное TTL в зависимости от частоты запросов ---")
	key := "hot:data:123"
	baseTTL := 10 * time.Second
	hotTTL := 60 * time.Second

	// Счётчик обращений
	hitsKey := "hits:" + key

	// Симуляция 5 запросов
	for i := 0; i <= 5; i++ {
		hits, _ := rdb.Incr(ctx, key).Result()
		rdb.Expire(ctx, key, 10*time.Second)

		// Если запросов больше 3 за окно, увеличиваем TTL
		var ttl time.Duration
		if hits > 3 {
			ttl = baseTTL
		} else {
			ttl = hotTTL
		}
		rdb.Set(ctx, key, "data", ttl)
		fmt.Printf("Запрос %d: TTL установлен = %v (хитов: %d)\n", i+1, ttl, hits)
		time.Sleep(2 * time.Second)
	}
	// Проверяем финальный TTL
	finalTTL, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("Финальный TTL: %v\n", finalTTL)

	rdb.Del(ctx, key, hitsKey)
}
