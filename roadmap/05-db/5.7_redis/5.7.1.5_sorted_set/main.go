package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 5. СОРТИРОВАННЫЕ МНОЖЕСТВА (SORTED SET) В REDIS

1. ЧТО ТАКОЕ СОРТИРОВАННОЕ МНОЖЕСТВО?
   - Sorted Set — это коллекция уникальных элементов (строк), каждый из которых
     связан с числовым "скором" (score). Элементы автоматически сортируются по скору.
   - Если два элемента имеют одинаковый скор, они сортируются лексикографически.
   - Сложность операций добавления/удаления/поиска по скору — O(log N), что очень быстро.

2. ГЛАВНОЕ ПРИМЕНЕНИЕ:
   - Лидерборды / рейтинги (игры, спортивные турниры).
   - Рейтинг популярности статей / товаров (по лайкам, просмотрам).
   - Очереди с приоритетом (задачи с разным приоритетом).
   - Таймеры / отложенные задачи (исполнение в определенное время).
   - Аналитика: скользящие окна, топ-N за период.
   - Индексация по времени (храним события с timestamp в качестве скора).

3. ОСНОВНЫЕ КОМАНДЫ И ИХ ДЕЙСТВИЕ (ПОДРОБНО)
   -------------------------------------------------------------------------------
   ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
   - Добавляет элементы с указанными скорами.
   - Если элемент уже существует, обновляет его скор (по умолчанию).
   - Возвращает количество добавленных элементов (не обновлённых).
   - CLI: ZADD leaderboard 100 "Alice" 95 "Bob"
   - Go: added, err := rdb.ZAdd(ctx, "leaderboard",
            redis.Z{Score: 100, Member: "Alice"},
            redis.Z{Score: 95, Member: "Bob"}).Result()

   ZRANK key member
   - Возвращает позицию элемента в порядке возрастания скора (0-based).
   - Если элемента нет, возвращает nil.
   - CLI: ZRANK leaderboard "Alice" → 1 (если Bob на 0 месте)
   - Go: rank, err := rdb.ZRank(ctx, "leaderboard", "Alice").Result() // int64

   ZREVRANK key member
   - Возвращает позицию элемента в порядке убывания скора (0-based).
   - CLI: ZREVRANK leaderboard "Alice" → 0 (если Alice на первом месте)
   - Go: rank, err := rdb.ZRevRank(ctx, "leaderboard", "Alice").Result()

   ZRANGE key start stop [WITHSCORES]
   - Возвращает элементы в порядке возрастания скора, в диапазоне индексов.
   - WITHSCORES — также возвращает скоры.
   - CLI: ZRANGE leaderboard 0 2 WITHSCORES
   - Go: members, err := rdb.ZRange(ctx, "leaderboard", 0, 2).Result() // []string
   - Go (со скорами): members, err := rdb.ZRangeWithScores(ctx, "leaderboard", 0, 2).Result()
        // []redis.Z

   ZREVRANGE key start stop [WITHSCORES]
   - То же, но в порядке убывания скора (обратный порядок).
   - CLI: ZREVRANGE leaderboard 0 2 WITHSCORES
   - Go: members, err := rdb.ZRevRange(ctx, "leaderboard", 0, 2).Result()
   - Go (со скорами): members, err := rdb.ZRevRangeWithScores(ctx, "leaderboard", 0, 2).Result()

   ZREM key member [member ...]
   - Удаляет элементы из сортированного множества.
   - Возвращает количество удалённых элементов.
   - CLI: ZREM leaderboard "Alice"
   - Go: removed, err := rdb.ZRem(ctx, "leaderboard", "Alice").Result()

   ZSCORE key member
   - Возвращает скор элемента.
   - Если элемента нет, возвращает nil.
   - CLI: ZSCORE leaderboard "Alice" → "100"
   - Go: score, err := rdb.ZScore(ctx, "leaderboard", "Alice").Result() // float64

   ZINCRBY key increment member
   - Атомарно увеличивает скор элемента на заданную величину (может быть отрицательной).
   - Если элемента нет, создаёт его со скором increment.
   - Возвращает новый скор (float64).
   - CLI: ZINCRBY leaderboard 5 "Alice" → 105
   - Go: newScore, err := rdb.ZIncrBy(ctx, "leaderboard", 5, "Alice").Result()

   ZCOUNT key min max
   - Возвращает количество элементов со скором в диапазоне [min, max].
   - CLI: ZCOUNT leaderboard 90 100 → 2
   - Go: count, err := rdb.ZCount(ctx, "leaderboard", "90", "100").Result() // int64

   Дополнительные команды (не в основном списке, но полезны):
   - ZREVRANGEBYSCORE — получить элементы по диапазону скоров в обратном порядке.
   - ZREMRANGEBYRANK — удалить элементы по диапазону позиций (например, для усечения).
   - ZREMRANGEBYSCORE — удалить элементы по диапазону скоров.
   - ZUNIONSTORE / ZINTERSTORE — объединение/пересечение нескольких сортированных множеств.

4. ВАЖНЫЕ СВОЙСТВА И ПРАВИЛА:
   - Все операции — O(log N) (кроме массовых вставок, которые O(N*logN)).
   - Скоры — это числа с плавающей точкой (double), можно использовать как целые.
   - Если элементы имеют одинаковый скор, они сортируются лексикографически по строке.
   - Можно использовать дробные скоры для точного упорядочивания.

5. КОГДА НЕ ИСПОЛЬЗОВАТЬ:
   - Если нужен только уникальный элемент без сортировки → Set.
   - Если нужен упорядоченный список, но без весов → List.
   - Если элементы должны хранить много полей → Hash.

6. СВЯЗЬ С GO (go-redis/v9):
   - ZAdd(ctx, key, ...Z) — *IntCmd
   - ZRank(ctx, key, member) — *IntCmd (возвращает -1, если нет, в go-redis — redis.Nil)
   - ZRevRank(ctx, key, member) — *IntCmd
   - ZRange(ctx, key, start, stop) — *StringSliceCmd
   - ZRangeWithScores(ctx, key, start, stop) — *ZSliceCmd
   - ZRevRange(ctx, key, start, stop) — *StringSliceCmd
   - ZRevRangeWithScores(ctx, key, start, stop) — *ZSliceCmd
   - ZRem(ctx, key, members...) — *IntCmd
   - ZScore(ctx, key, member) — *FloatCmd
   - ZIncrBy(ctx, key, increment, member) — *FloatCmd
   - ZCount(ctx, key, min, max) — *IntCmd

7. ТИПИЧНЫЕ ОШИБКИ:
   - redis.Nil при ZScore/ZRank на несуществующем элементе.
   - Ошибка, если попытаться использовать некорректные границы диапазона в ZCOUNT.

8. ПРОДВИНУТЫЕ ПАТТЕРНЫ:
   - Лидерборд с ежемесячным сбросом: используйте ключи с датой (leaderboard:2024-01).
   - Система голосования: скор = количество голосов, ZINCRBY для добавления голоса.
   - Очередь с приоритетом: скор = приоритет (чем выше, тем раньше обрабатывается).
   - Отложенные задачи: скор = timestamp выполнения, периодически забираем ZRANGEBYSCORE.
   - Топ-N по просмотрам за последний час: скор = просмотры, элементы обновляются через ZINCRBY.
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
	fmt.Println("=== ПРАКТИКА ПО ZSET ===\n")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
}

// 1. ИГРОВОЙ ЛИДЕРБОРД С ОБНОВЛЕНИЕМ РЕЙТИНГА
func primer1() {
	fmt.Println("--- 1. Лидерборд: обновление очков, получение ранга и топа ---")
	key := "game:scores"

	// Начальные очки
	rdb.ZAdd(ctx, key,
		redis.Z{Score: 1500, Member: "player1"},
		redis.Z{Score: 2300, Member: "player2"},
		redis.Z{Score: 1800, Member: "player3"},
	)

	// Игрок 1 получает +200 очков
	newScore, _ := rdb.ZIncrBy(ctx, key, 200, "player1").Result()
	fmt.Printf("player1 новый скор: %.0f\n", newScore) // 1700

	// Топ-3
	top, _ := rdb.ZRevRangeWithScores(ctx, key, 0, 3).Result()
	fmt.Println("Топ-3 игрока:")
	for i, z := range top {
		fmt.Printf("  %d. %s — %.0f\n", i+1, z.Member, z.Score)
	}
	// Ожидаем: player2 (2300), player3 (1800), player1 (1700)

	// Ранг player1 (ZREVRANK)
	rank, _ := rdb.ZRevRank(ctx, key, "player1").Result()
	fmt.Printf("player1 на %d месте (с 1)\n", rank+1)

	// Удаляем игрока, который вышел (ZREM)
	rdb.ZRem(ctx, key, "player3")
	fmt.Println("player3 удалён")
}

// 2. АНАЛИТИКА ПО ВРЕМЕННЫМ ОКНАМ (скользящее окно)
func primer2() {
	fmt.Println("\n--- 2. Скользящее окно: события за последнюю минуту ---")
	key := "events:minute"

	// Добавляем события с текущим временем (timestamp)
	now := time.Now().Unix()
	for i := 0; i < 5; i++ {
		ts := now - int64(i*5) // каждые 5 секунд
		rdb.ZAdd(ctx, key, redis.Z{Score: float64(ts), Member: fmt.Sprintf("event_%d", i)})
	}

	// Подсчитываем события за последние 10 секунд (ZCOUNT)
	threshold := now - 10
	count, _ := rdb.ZCount(ctx, key, fmt.Sprintf("%d", threshold), "+inf").Result()
	fmt.Printf("Событий за последние 10 секунд: %d\n", count)

	// Удаляем события старше 60 секунд (ZREMRANGEBYSCORE)
	oldThreshold := now - 60
	removed, _ := rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", oldThreshold)).Result()
	fmt.Printf("Удалено старых событий: %d\n", removed)

	// Оставшиеся элементы
	remaining, _ := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	fmt.Println("Оставшиеся события:")
	for _, z := range remaining {
		fmt.Printf("  %s (время: %d)\n", z.Member, int64(z.Score))
	}
}

// 3. ОБЪЕДИНЕНИЕ И ПЕРЕСЕЧЕНИЕ ЛИДЕРБОРДОВ (ZUNIONSTORE / ZINTERSTORE)
func primer3() {
	fmt.Println("\n--- 3. Объединение и пересечение лидербордов ---")
	key1, key2, destUnion, destInter := "leaderboard:month1", "leaderboard:month2", "combined:union", "combined:inter"

	// Данные за два месяца
	rdb.ZAdd(ctx, key1,
		redis.Z{Score: 100, Member: "Alice"},
		redis.Z{Score: 90, Member: "Bob"},
		redis.Z{Score: 80, Member: "Charlie"},
	)
	rdb.ZAdd(ctx, key2,
		redis.Z{Score: 95, Member: "Alice"},
		redis.Z{Score: 85, Member: "Bob"},
		redis.Z{Score: 70, Member: "Diana"},
	)

	// Объединение (сумма очков)
	// ZUNIONSTORE destUnion 2 key1 key2 WEIGHTS 1 1
	err := rdb.ZUnionStore(ctx, destUnion, &redis.ZStore{
		Keys:      []string{key1, key2},
		Weights:   []float64{1, 1},
		Aggregate: "SUM",
	}).Err()
	if err != nil {
		panic(err)
	}
	union, _ := rdb.ZRevRangeWithScores(ctx, destUnion, 0, -1).Result()
	fmt.Println("Объединение (сумма):")
	for _, z := range union {
		fmt.Printf("  %s — %.0f\n", z.Member, z.Score)
	}
	// Пересечение (минимальный скор)
	err = rdb.ZInterStore(ctx, destInter, &redis.ZStore{
		Keys:      []string{key1, key2},
		Weights:   []float64{1, 1},
		Aggregate: "MIN",
	}).Err()
	if err != nil {
		panic(err)
	}
	inter, _ := rdb.ZRevRangeWithScores(ctx, destInter, 0, -1).Result()
	fmt.Println("Пересечение (MIN):")
	for _, z := range inter {
		fmt.Printf("  %s — %.0f\n", z.Member, z.Score)
	}
	// Ожидаем: Alice (95), Bob (85) — Charlie и Diana отсутствуют
}

// 4. СИСТЕМА РЕЙТИНГА ФИЛЬМОВ (голосование)
func primer4() {
	fmt.Println("\n--- 4. Рейтинг фильмов (голосование) ---")
	key := "movies:rating"

	// Фильмы с начальным рейтингом (средний балл)
	rdb.ZAdd(ctx, key,
		redis.Z{Score: 4.5, Member: "Inception"},
		redis.Z{Score: 4.2, Member: "Interstellar"},
		redis.Z{Score: 3.8, Member: "Dunkirk"},
	)

	// Пользователь ставит оценку 5 для "Inception" — обновляем средний балл
	// В реальности мы бы хранили отдельно количество голосов, но здесь упростим:
	// просто прибавим 0.1 (имитация)
	rdb.ZIncrBy(ctx, key, 0.1, "Inception")

	// Топ-2 фильма
	top, _ := rdb.ZRangeWithScores(ctx, key, 0, 1).Result()
	fmt.Println("Топ-2 фильма:")
	for i, z := range top {
		fmt.Printf("  %d. %s — %.1f\n", i+1, z.Member, z.Score)
	}
}

// 5. РЕКОМЕНДАТЕЛЬНАЯ СИСТЕМА НА ОСНОВЕ ПЕРЕСЕЧЕНИЙ (INTERSTORE)
func primer5() {
	fmt.Println("\n--- 5. Рекомендации: общие интересы пользователей ---")
	// У каждого пользователя есть взвешенный набор интересов (ZSET)
	user1 := "user:1:interests"
	user2 := "user:2:interests"
	dest := "common:interests"

	rdb.ZAdd(ctx, user1,
		redis.Z{Score: 10, Member: "golang"},
		redis.Z{Score: 8, Member: "redis"},
		redis.Z{Score: 6, Member: "docker"},
	)
	rdb.ZAdd(ctx, user2,
		redis.Z{Score: 9, Member: "golang"},
		redis.Z{Score: 7, Member: "redis"},
		redis.Z{Score: 5, Member: "python"},
	)

	// Общие интересы с минимальным весом (пересечение)
	err := rdb.ZInterStore(ctx, dest, &redis.ZStore{
		Keys:      []string{user1, user2},
		Weights:   []float64{1, 1},
		Aggregate: "MIN",
	}).Err()
	if err != nil {
		panic(err)
	}
	common, _ := rdb.ZRangeWithScores(ctx, dest, 0, -1).Result()
	fmt.Println("Общие интересы (с минимальным весом):")
	for _, z := range common {
		fmt.Printf("  %s — %.0f\n", z.Member, z.Score)
	}
	// Ожидаем: golang (9), redis (7)
}

// 6. ОЧЕРЕДЬ ЗАДАЧ С ДИНАМИЧЕСКИМ ИЗМЕНЕНИЕМ ПРИОРИТЕТА
func primer6() {
	fmt.Println("\n--- 6. Очередь с динамическим приоритетом ---")
	key := "tasks:priority"

	// Добавляем задачи с приоритетом (скор)
	rdb.ZAdd(ctx, key,
		redis.Z{Score: 5, Member: "task:low"},
		redis.Z{Score: 8, Member: "task:medium"},
		redis.Z{Score: 10, Member: "task:high"},
	)

	// Задача "task:medium" становится критичной — увеличиваем приоритет до 12
	newScore, _ := rdb.ZIncrBy(ctx, key, 4, "task:medium").Result() // стало 12
	fmt.Printf("Новый приоритет task:medium: %.0f\n", newScore)

	// Теперь самая приоритетная — task:medium (12), затем task:high (10)
	top, _ := rdb.ZRangeWithScores(ctx, key, 0, 0).Result()
	fmt.Printf("Самая приоритетная задача: %s (скор: %.0f)\n", top[0].Member, top[0].Score)

	// Забираем задачу с наивысшим приоритетом (ZRANGE ... REV + ZREM)
	tasks, err := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   key,
		Start: 0,
		Stop:  0,
		Rev:   true, // Этот параметр заменяет старый ZREVRANGE
	}).Result()
	if err != nil {
		panic(err)
	}
	if len(tasks) > 0 {
		_, err = rdb.ZRem(ctx, key, tasks[0]).Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Забрана задача: %s\n", tasks[0])
	}
}

// 7. ЭКСПОНЕНЦИАЛЬНОЕ ЗАТУХАНИЕ ПОПУЛЯРНОСТИ (алгоритм горячих новостей)
func primer7() {
	fmt.Println("\n--- 7. Алгоритм горячих новостей (затухание с возрастом) ---")
	key := "news:hotness"

	// Добавляем новости с очками (лайки) и временем публикации
	now := time.Now().Unix()
	// Скор = лайки - (возраст в секундах) / 100
	// Чем свежее и больше лайков, тем выше скор
	rdb.ZAdd(ctx, key,
		redis.Z{Score: 100 - float64(now-1000)/100, Member: "news1"}, // старый
		redis.Z{Score: 80 - float64(now-100)/100, Member: "news2"},   // новый
	)

	// Добавляем ещё одну новость с большим числом лайков, но очень свежую
	rdb.ZAdd(ctx, key, redis.Z{Score: 200 - 0, Member: "news3"}) // только что

	// Получаем топ новостей по "горячести"
	hot, _ := rdb.ZRevRangeWithScores(ctx, key, 0, 2).Result()
	fmt.Println("Горячие новости:")
	for i, z := range hot {
		fmt.Printf("  %d. %s — %.2f\n", i+1, z.Member, z.Score)
	}
	// Ожидаем: news3 (200), news2 (~80), news1 (~90) — но может отличаться
}
