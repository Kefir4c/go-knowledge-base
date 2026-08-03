package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 5.1. ПАЙПЛАЙН (PIPELINE)
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖЕН ПАЙПЛАЙН?

Redis — это высокопроизводительная база данных, но основная задержка при работе
с ней часто связана не с выполнением команд, а с сетевыми round-trip (RTT).
Каждая команда требует отдельного запроса и ответа. При большом количестве команд
задержка RTT суммируется, и даже быстрый Redis может показаться медленным.

Пайплайн позволяет объединить несколько команд в одну сетевую операцию,
что радикально снижает накладные расходы и увеличивает пропускную способность.

1. КАК РАБОТАЕТ ПАЙПЛАЙН (ВНУТРЕННЕЕ УСТРОЙСТВО)

1.1. Сетевой уровень:
    - Клиент (например, go-redis) буферизует команды в локальном буфере.
    - Все команды отправляются на сервер одним пакетом (или несколькими, если буфер большой).
    - Сервер Redis обрабатывает команды в том же порядке, в котором они были отправлены.
    - Ответы отправляются обратно клиенту в том же порядке.
    - Клиент получает все ответы и связывает их с командами по порядку.

1.2. Буферизация:
    - go-redis использует внутренний буфер записи (write buffer) для накопления команд.
    - При вызове `Exec()` буфер сбрасывается на сервер.
    - Это позволяет минимизировать количество системных вызовов `write()`.

1.3. Обработка на сервере:
    - Redis получает команды из пайплайна и помещает их в очередь выполнения.
    - Команды выполняются последовательно, но между ними могут вклиниться команды от других клиентов.
    - Результаты записываются в буфер ответов и отправляются клиенту одним пакетом (или по мере готовности).

2. ПРОИЗВОДИТЕЛЬНОСТЬ: ПАЙПЛАЙН VS ПОСЛЕДОВАТЕЛЬНЫЕ КОМАНДЫ

2.1. Пример:
    - 1000 команд SET.
    - Без пайплайна: 1000 сетевых round-trip.
    - С пайплайном: 1 или несколько round-trip (зависит от размера буфера).
    - Ускорение может быть в 10-100 раз, особенно при высокой задержке сети.

2.2. Факторы, влияющие на производительность:
    - Размер пачки: слишком большая пачка может перегрузить память клиента и сервера.
    - Сложность команд: тяжёлые команды (LRANGE с большим диапазоном) могут замедлить всю пачку.
    - Сеть: пайплайн даёт наибольший выигрыш при высокой задержке.
    - Количество одновременных клиентов: пайплайн позволяет утилизировать сеть и сервер эффективнее.

2.3. Рекомендуемый размер пачки:
    - Обычно 100-1000 команд на пачку.
    - Для очень больших объёмов (сотни тысяч) разбивайте на пакеты по 1000-5000 команд,
      чтобы избежать переполнения памяти на клиенте и сервере.

3. СРАВНЕНИЕ ПАЙПЛАЙНА С ДРУГИМИ ПОДХОДАМИ

3.1. Пайплайн против MGET/MSET:
    - MGET/MSET — специальные команды для массового чтения/записи.
    - Они более эффективны, чем пайплайн с отдельными GET/SET, т.к. выполняются атомарно и с меньшим оверхедом.
    - Используйте MGET/MSET, когда нужно работать с несколькими ключами в одной команде.
    - Пайплайн полезен, когда команды разнородны (SET + GET + DEL + INCR).

3.2. Пайплайн против транзакций (MULTI/EXEC):
    - Транзакции обеспечивают атомарность (все команды выполняются как единое целое).
    - Пайплайн НЕ атомарен: другие клиенты могут вклиниться между командами.
    - Если нужна атомарность — используйте транзакции или Lua.
    - Транзакции также отправляют все команды одной пачкой, т.е. дают тот же выигрыш в RTT.

3.3. Пайплайн против Lua:
    - Lua-скрипт выполняется атомарно на сервере и может содержать логику.
    - Пайплайн — это просто набор команд, без логики.
    - Используйте Lua, когда нужно выполнить сложную логику (условия, циклы) на стороне сервера.

4. ОШИБКИ В ПАЙПЛАЙНЕ

4.1. Поведение Redis при ошибке:
    - Если одна команда в пайплайне вызывает ошибку (например, INCR на строке),
      эта команда вернёт ошибку, но остальные команды будут выполнены.
    - Redis не прерывает выполнение пайплайна при ошибке.
    - Ответы возвращаются в том же порядке, и ошибка содержится в конкретной команде.

4.2. Обработка в go-redis:
    - `Exec()` вернёт ошибку, если хотя бы одна команда завершилась с ошибкой.
    - Но даже если `Exec()` вернул ошибку, некоторые команды могли выполниться успешно.
    - Чтобы проверить каждую команду, нужно итерировать по `cmds` и проверять `cmd.Err()`.

4.3. Ошибки сети:
    - Если соединение разорвано во время выполнения пайплайна, часть команд могла выполниться,
      часть — нет. go-redis автоматически повторит попытку в зависимости от настроек Retry.

5. ПАЙПЛАЙН В CLUSTER

5.1. Ограничения:
    - В кластере ключи распределены по слотам.
    - Если команды в пайплайне обращаются к ключам из разных слотов,
      go-redis автоматически разбивает пайплайн на несколько пачек (по слотам).
    - Это снижает эффективность, т.к. пачек становится больше.

5.2. Рекомендации:
    - Для максимальной производительности группируйте ключи в одном слоте
      с помощью хеш-тегов (например, `{user:123}.field`).
    - Если невозможно сгруппировать, используйте отдельные пайплайны для каждого слота
      или выполняйте команды последовательно.

5.3. Пример:
    - Пайплайн с 1000 ключами, разбросанными по 10 слотам.
    - go-redis разобьёт их на 10 отдельных сетевых запросов.
    - Ускорение будет меньше, чем в случае одного слота.

6. ПАЙПЛАЙН И ПАМЯТЬ

6.1. Клиент:
    - Все команды и их аргументы буферизируются в памяти клиента.
    - При большом количестве команд (сотни тысяч) буфер может занять много памяти.
    - Рекомендуется разбивать на пакеты, чтобы избежать переполнения памяти.

6.2. Сервер (Redis):
    - Команды из пайплайна помещаются в очередь на сервере.
    - Если пачка очень большая, Redis может временно хранить все команды в буфере,
      что увеличивает потребление памяти.
    - Также большая пачка может заблокировать другие операции (команды других клиентов
      будут ждать, пока пайплайн не завершится).

7. ПАЙПЛАЙН И БЛОКИРУЮЩИЕ ОПЕРАЦИИ

7.1. Блокирующие команды (BLPOP, BRPOP, WAIT):
    - Их использование в пайплайне не рекомендуется.
    - Если добавить BLPOP в пайплайн, он заблокирует выполнение всех последующих команд
      до получения элемента.
    - Пайплайн не может быть "частично" выполнен — он выполняется целиком.
    - В go-redis блокирующие команды в пайплайне также будут работать, но это неэффективно.

8. ЛУЧШИЕ ПРАКТИКИ ИСПОЛЬЗОВАНИЯ ПАЙПЛАЙНА

1. Используйте пайплайн для массовых операций (вставка, обновление, чтение).
2. Держите размер пачки в пределах 100-1000 команд.
3. Для однотипных операций используйте MGET/MSET (они эффективнее).
4. Для разнотипных операций используйте пайплайн.
5. В кластере группируйте ключи через хеш-теги.
6. Проверяйте ошибки каждой команды, а не только общую ошибку `Exec()`.
7. Не смешивайте блокирующие команды с пайплайном.
8. Используйте `TxPipeline()` для атомарных операций (аналог MULTI/EXEC).
9. При работе с большими объёмами (сотни тысяч) разбивайте на пакеты и следите за памятью.
10. В go-redis используйте `Pipeline()` для обычных операций и `TxPipeline()` для транзакций.

9. ПАЙПЛАЙН В GO-REDIS: ПОДРОБНОСТИ РЕАЛИЗАЦИИ

9.1. Типы команд:
    - `*redis.StatusCmd` — для команд, возвращающих статус (SET, EXPIRE).
    - `*redis.StringCmd` — для команд, возвращающих строку (GET).
    - `*redis.IntCmd` — для команд, возвращающих целое число (INCR, DEL).
    - `*redis.SliceCmd` — для команд, возвращающих массив (LRANGE, MGET).

9.2. Использование `Pipelined`:
    - Упрощённая форма: `cmds, err := client.Pipelined(ctx, func(pipe Pipeliner) error { ... })`.
    - Все команды в колбэке добавляются в пайплайн, выполнение происходит автоматически.
    - Результаты возвращаются в том же порядке.

9.3. Обработка ошибок:
    - Если `Exec()` возвращает ошибку, это не значит, что все команды провалились.
    - Проверяйте `cmd.Err()` для каждой команды.

9.4. Контекст и таймауты:
    - Пайплайн использует контекст, переданный в `Exec()` или `Pipelined()`.
    - Если контекст завершится по таймауту, пайплайн будет прерван.

10. ТИПИЧНЫЕ ОШИБКИ ПРИ ИСПОЛЬЗОВАНИИ ПАЙПЛАЙНА

 "Пайплайн даёт атомарность" — НЕТ! Команды выполняются последовательно,
   но не атомарно. Другие клиенты могут вклиниться.

 "Если одна команда упала, все откатываются" — НЕТ! Redis выполняет все команды,
   даже если некоторые завершились ошибкой.

 "Пайплайн всегда быстрее" — НЕТ! Для маленького количества команд (1-2) оверхед
   пайплайна может быть выше, чем отдельных команд.

 "Пайплайн работает одинаково в кластере" — НЕТ! В кластере пайплайн может
   разбиваться на несколько пачек, если ключи в разных слотах.

11. РЕЗЮМЕ: КОГДА ИСПОЛЬЗОВАТЬ ПАЙПЛАЙН

 Когда нужно выполнить много команд (десятки, сотни, тысячи).
 Когда команды разнородны (нельзя использовать MGET/MSET).
 Когда сеть имеет высокую задержку (пайплайн уменьшает RTT).
 Когда нужна высокая пропускная способность (пакетная обработка).

 Когда нужно атомарное выполнение (используйте транзакции или Lua).
 Когда команд мало (1-5) — оверхед пайплайна не даст выигрыша.
 Когда в пайплайне есть блокирующие команды.
 Когда ключи разбросаны по разным слотам в кластере (эффективность снижается).
*/
// Глобальный клиент (для простоты)
var (
	rdb *redis.Client
	ctx = context.Background()
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
		log.Fatalf("Redis не отвечает: %v", err)
	}
}

func main() {
	fmt.Println("=== ПАЙПЛАЙН ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. МАССОВАЯ ВСТАВКА С ОБРАБОТКОЙ ОШИБОК И ЧАСТИЧНЫМ УСПЕХОМ
func primer1() {
	fmt.Println("--- 1. Массовая вставка (100k записей) с контролем ошибок ---")

	type User struct {
		ID   int
		Name string
		Age  int
	}

	// Генерируем данные
	users := make([]User, 100000)
	for i := 0; i < 100000; i++ {
		users[i] = User{
			ID:   i,
			Name: fmt.Sprintf("User%d", i),
			Age:  20 + i%30,
		}
	}

	const batchSize = 1000
	var wg sync.WaitGroup
	errorsCh := make(chan error, 10)

	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[i:end]

		wg.Add(1)
		go func(bath []User) {
			defer wg.Done()
			pipe := rdb.Pipeline()
			for _, u := range bath {
				key := fmt.Sprintf("user:%d", u.ID)
				pipe.HSet(ctx, key,
					"name", u.Name,
					"age", u.Age,
				)
			}
			// Выполняем пачку
			cmds, err := pipe.Exec(ctx)
			if err != nil {
				// Проверяем, какие команды упали
				for _, cmd := range cmds {
					if cmd.Err() != nil {
						errorsCh <- fmt.Errorf("команда %v ошибка: %w", cmd, cmd.Err())
					}
				}
				// Также логируем общую ошибку
				log.Printf("Ошибка выполнения пачки: %v", err)
				return
			}
			log.Printf("Пачка из %d записей успешно вставлена", len(batch))
		}(batch)
	}
	wg.Wait()
	close(errorsCh)

	// Вывод ошибок (если есть)
	for err := range errorsCh {
		fmt.Printf("%v\n", err)
	}
}

// 2. ПАКЕТНОЕ ЧТЕНИЕ С ОБРАБОТКОЙ НЕСУЩЕСТВУЮЩИХ КЛЮЧЕЙ
func primer2() {
	fmt.Println("\n--- 2. Пакетное чтение 1000 ключей с обработкой redis.Nil ---")

	// Подготовим тестовые данные (1000 ключей)
	for i := 0; i < 1000; i++ {
		rdb.Set(ctx, fmt.Sprintf("read:%d", i), fmt.Sprintf("value_%d", i), 0)
	}

	// Читаем пачками
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = fmt.Sprintf("read:%d", i)
	}

	pipe := rdb.Pipeline()
	for _, key := range keys {
		pipe.Get(ctx, key)
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		log.Printf("Ошибка выполнения пайплайна: %v", err)
		return
	}

	// Разбираем результаты
	var missingKeys []string
	var values []string
	for i, cmd := range cmds {
		if getCmd, ok := cmd.(*redis.StringCmd); ok {
			val, err := getCmd.Result()
			if errors.Is(err, redis.Nil) {
				missingKeys = append(missingKeys, keys[i])
			} else if err != nil {
				log.Printf("Ошибка чтения ключа %s: %v", keys[i], err)
			} else {
				values = append(values, val)
			}
		}
	}
	fmt.Printf("Прочитано %d значений, пропущено (нет ключей): %d\n", len(values), len(missingKeys))
	if len(missingKeys) > 0 {
		fmt.Printf("Отсутствующие ключи (первые 5): %v\n", missingKeys[:5])
	}

	// Очистка
	rdb.Del(ctx, keys...)
}

// 3. ОБНОВЛЕНИЕ МНОЖЕСТВА СЧЁТЧИКОВ (INCR) С АТОМАРНЫМ TTL
func primer3() {
	fmt.Println("\n--- 3. Обновление 500 счётчиков (INCR) с установкой TTL ---")

	const numCounters = 500
	pipe := rdb.Pipeline()
	for i := 0; i < numCounters; i++ {
		key := fmt.Sprintf("counter:%d", i)
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Hour)
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Ошибка выполнения: %v", err)
		return
	}

	// Проверяем, что все INCR прошли успешно
	success := 0
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			log.Printf("Ошибка: %v", cmd.Err())
		} else {
			success++
		}
	}
	fmt.Printf("Обновлено %d счётчиков, ошибок: %d\n", success/2, numCounters-success/2)

	// Очистка
	keys := make([]string, numCounters)
	for i := 0; i < numCounters; i++ {
		keys[i] = fmt.Sprintf("counter:%d", i)
	}
	rdb.Del(ctx, keys...)
}

// 4. УДАЛЕНИЕ КЛЮЧЕЙ ПО ПАТТЕРНУ (SCAN + PIPELINE) БЕЗ KEYS
func primer4() {
	fmt.Println("\n--- 4. Удаление ключей по паттерну через SCAN + Pipeline ---")

	// Создаём тестовые ключи
	for i := 0; i < 1000; i++ {
		rdb.Set(ctx, fmt.Sprintf("session:user:%d", i), "active", 0)
	}

	var cursor uint64
	var totalDeleted int
	for {
		var keys []string
		var err error
		keys, cursor, err := rdb.Scan(ctx, cursor, "session:user:*", 100).Result()
		if err != nil {
			log.Printf("Ошибка SCAN: %v", err)
			break
		}
		if len(keys) == 0 {
			break
		}

		// Удаляем через пайплайн
		pipe := rdb.Pipeline()
		for _, key := range keys {
			pipe.Del(ctx, key)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Ошибка удаления: %v", err)
			// Проверяем частичный успех
			for _, cmd := range cmds {
				if cmd.Err() != nil {
					log.Printf("Ошибка DEL %v", cmd.Err())
				}
			}
		}
		totalDeleted += len(keys)
		if cursor == 0 {
			break
		}
	}
	fmt.Printf("Удалено %d ключей\n", totalDeleted)
}

// 5. СБОР МЕТРИК С ТАЙМАУТОМ
func primer5() {
	fmt.Println("\n--- 5. Сбор метрик с таймаутом (100 GET) ---")

	metricsKeys := []string{
		"metrics:requests",
		"metrics:errors",
		"metrics:latency_avg",
		"metrics:latency_p99",
		"metrics:connections",
		"metrics:cache_hits",
		"metrics:cache_misses",
		"metrics:memory_used",
	}
	// Заполним некоторые значения
	for _, k := range metricsKeys {
		rdb.Set(ctx, k, rand.Intn(1000), 0)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	pipe := rdb.Pipeline()
	for _, k := range metricsKeys {
		pipe.Get(ctx, k)
	}
	cmds, err := pipe.Exec(ctxTimeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("Таймаут при сборе метрик")
		} else {
			log.Printf("Ошибка: %v", err)
		}
		return
	}

	fmt.Println("Собранные метрики:")
	for i, k := range metricsKeys {
		if getCmd, ok := cmds[i].(*redis.StringCmd); ok {
			val, err := getCmd.Result()
			if err != nil {
				fmt.Printf("  %s: ошибка (%v)\n", k, err)
			} else {
				fmt.Printf("  %s: %s\n", k, val)
			}
		}
	}
	rdb.Del(ctx, metricsKeys...)
}

// 6. ТРАНЗАКЦИЯ С ПАЙПЛАЙНОМ (TxPipeline) — АТОМАРНОСТЬ
func primer6() {
	fmt.Println("\n--- 6. Атомарное обновление баланса (TxPipeline) ---")

	key := "account:123"
	initialBalance := 100
	rdb.Set(ctx, key, initialBalance, 0)

	// Функция перевода средств
	transfer := func(amount int) error {
		// Используем WATCH для оптимистической блокировки
		err := rdb.Watch(ctx, func(tx *redis.Tx) error {
			balance, err := tx.Get(ctx, key).Int64()
			if err != nil && err != redis.Nil {
				return err
			}
			if balance < int64(amount) {
				return fmt.Errorf("недостаточно средств: %d < %d", balance, amount)
			}
			// Транзакция с пайплайном
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, balance-int64(amount), 0)
				// Можно добавить ещё команды (логирование, история)
				return nil
			})
			return err
		}, key)
		return err
	}

	// Выполняем перевод
	err := transfer(30)
	if err != nil {
		fmt.Printf("Перевод не удался: %v\n", err)
	} else {
		fmt.Println("Перевод успешен")
	}

	// Проверяем баланс
	balance, _ := rdb.Get(ctx, key).Int64()
	fmt.Printf("Баланс: %d\n", balance)
	rdb.Del(ctx, key)
}

// 7. ПАЙПЛАЙН В КЛАСТЕРЕ (С ХЕШ-ТЕГАМИ)
func primer7() {
	fmt.Println("\n--- 7. Пайплайн в кластере с группировкой ключей ---")

	// Создаём кластерный клиент (предположим, что кластер запущен)
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
	})
	defer clusterClient.Close()

	// Группируем ключи в одном слоте через хеш-тег {user:123}
	pipe := clusterClient.Pipeline()
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("{user:123}.field:%d", i)
		pipe.Set(ctx, key, fmt.Sprintf("value_%d", i), 0)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Ошибка кластерного пайплайна: %v\n", err)
	} else {
		fmt.Println("100 ключей записаны в одном слоте")
	}

	// Очистка
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("{user:123}.field:%d", i)
	}
	clusterClient.Del(ctx, keys...)
}

// 8. ОБНОВЛЕНИЕ КЭША И ИНВАЛИДАЦИЯ (ПАЙПЛАЙН ДЛЯ КОМПЛЕКСНЫХ ОПЕРАЦИЙ)
func primer8() {
	fmt.Println("\n--- 8. Обновление кэша и инвалидация (комплексный пайплайн) ---")

	// Предположим, мы обновляем профиль пользователя и инвалидируем связанные ключи
	userID := "123"
	profileKey := fmt.Sprintf("user:%s:profile", userID)
	sessionKey := fmt.Sprintf("user:%s:session", userID)
	cacheKey := fmt.Sprintf("user:%s:cache", userID)

	// Подготавливаем старые данные
	rdb.Set(ctx, profileKey, "old_profile", 0)
	rdb.Set(ctx, sessionKey, "old_session", 0)
	rdb.Set(ctx, cacheKey, "old_cache", 0)

	// Обновление: пишем новые данные и удаляем связанные кэши
	pipe := rdb.Pipeline()
	pipe.Set(ctx, profileKey, "new_profile", 0)
	pipe.Set(ctx, sessionKey, "new_session", 0)
	pipe.Del(ctx, cacheKey)
	// Добавляем логирование (например, сохраняем в историю)
	pipe.LPush(ctx, "user:history", time.Now().String())

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Ошибка обновления: %v\n", err)
		return
	}

	// Проверяем результат
	success := 0
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			fmt.Printf("Ошибка: %v\n", cmd.Err())
		} else {
			success++
		}
	}
	fmt.Printf("Обновлено %d из %d команд\n", success, len(cmds))

	// Проверяем, что cache удалён
	_, err = rdb.Get(ctx, cacheKey).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("Кэш успешно инвалидирован")
	}

	// Очистка
	rdb.Del(ctx, profileKey, sessionKey, cacheKey, "user:history")
}
