package leaderboards

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 7.5. ЛИДЕРБОРДЫ (РЕЙТИНГИ)
0. ВВЕДЕНИЕ: ПОЧЕМУ ЛИДЕРБОРДЫ В REDIS?

Лидерборды — это неотъемлемая часть игровых платформ, спортивных приложений,
социальных сетей и систем мотивации. Redis с его Sorted Set (ZSET) предоставляет
идеальный инструмент для построения высокопроизводительных рейтинговых систем:

- Сортировка по очкам за O(log N).
- Быстрое добавление/обновление очков.
- Получение топ-N игроков за O(log N + M).
- Получение ранга игрока за O(log N).
- Атомарность операций (без гонок).
- Поддержка сценариев с пагинацией, временными периодами и агрегацией.

Лидерборды могут быть простыми (топ-10) или сложными (многомерные рейтинги,
еженедельные сбросы, комбинированные скоринговые модели). В этом разделе
мы рассмотрим все аспекты разработки и эксплуатации лидербордов на Redis.

1. ОСНОВНЫЕ КОМАНДЫ И ИХ СЛОЖНОСТЬ (ПОДРОБНО)

1.1. ZADD key score member [score member ...]
    - Добавляет элементы или обновляет их очки.
    - Сложность: O(log N) для каждого добавленного элемента.
    - Если элемент уже существует, его скор обновляется.
    - Возвращает количество добавленных новых элементов.

1.2. ZINCRBY key increment member
    - Атомарно увеличивает очки элемента на increment (может быть отрицательным).
    - Если элемента нет, он создаётся с increment.
    - Сложность: O(log N).
    - Возвращает новое значение очков.

1.3. ZREVRANGE key start stop [WITHSCORES] (устаревшая версия)
    - Возвращает элементы в порядке убывания очков.
    - Сложность: O(log N + M), где M = stop - start + 1.
    - Рекомендуется использовать ZRANGE с опцией REV.

1.4. ZRANGE key start stop [WITHSCORES] [REV]
    - Универсальная команда: по умолчанию по возрастанию, с REV — по убыванию.
    - Сложность: O(log N + M).
    - В go-redis используйте ZRangeArgs с параметром Rev: true.

1.5. ZRANK key member
    - Возвращает ранг элемента (индекс) в порядке возрастания очков (0-based).
    - Сложность: O(log N).
    - Возвращает nil, если элемента нет.

1.6. ZREVRANK key member
    - Ранг в порядке убывания очков (0-based = позиция в топе).
    - Сложность: O(log N).

1.7. ZSCORE key member
    - Возвращает очки элемента.
    - Сложность: O(1).
    - Возвращает nil, если элемента нет.

1.8. ZREM key member [member ...]
    - Удаляет элементы.
    - Сложность: O(M log N), где M — число удаляемых элементов.

1.9. ZREMRANGEBYRANK key start stop
    - Удаляет элементы по диапазону рангов (например, всё после 100-го места).
    - Сложность: O(log N + M).
    - Полезно для ограничения размера лидерборда.

1.10. ZREMRANGEBYSCORE key min max
    - Удаляет элементы по диапазону очков.
    - Сложность: O(log N + M).

1.11. ZCOUNT key min max
    - Количество элементов с очками в диапазоне [min, max].
    - Сложность: O(log N + M).

1.12. ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
    - Возвращает элементы по диапазону очков, с поддержкой пагинации.
    - Сложность: O(log N + M).

1.13. ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight] [AGGREGATE SUM|MIN|MAX]
    - Объединяет несколько ZSET-ов в один. Очень мощная для агрегации периодов.
    - Сложность: O(N + K log K), где N — суммарное число элементов, K — число уникальных.

1.14. ZINTERSTORE destination numkeys key [key ...] [WEIGHTS weight] [AGGREGATE SUM|MIN|MAX]
    - Пересечение нескольких ZSET-ов.

2. ПРОДВИНУТЫЕ ПАТТЕРНЫ ЛИДЕРБОРДОВ

2.1. **Многомерные лидерборды**
    - Хранение нескольких метрик в одном ZSET с использованием составных скоров.
    - Например, очки = wins*1000 + kills — для сортировки сначала по победам, потом по убийствам.
    - Реализация: скор = wins * 1000000 + kills (достаточно большой множитель).

2.2. **Рейтинг с «эластичностью» (decay)**
    - Очки уменьшаются со временем, чтобы стимулировать активность.
    - Используйте фоновый процесс, который периодически применяет ZINCRBY с отрицательным значением ко всем игрокам (или только к топу).

2.3. **Сезонные / периодические лидерборды**
    - Ключи с суффиксами дат: leaderboard:game:season:2024-01.
    - TTL на каждый сезон для автоматического удаления.

2.4. **Лидерборды с разбивкой на группы (по странам, уровням)**
    - leaderboard:game:region:US, leaderboard:game:region:EU.

2.5. **Лидерборды с скрытым рейтингом (MMR)**
    - Используется в системах подбора игроков. Скор обновляется по формуле Эло.
    - Redis ZSET отлично подходит для хранения текущего MMR.

2.6. **Лидерборды с несколькими весами**
    - Можно использовать ZUNIONSTORE с весами для комбинирования разных метрик.

3. МАСШТАБИРОВАНИЕ И ПРОИЗВОДИТЕЛЬНОСТЬ

3.1. **Ограничение размера лидерборда**
    - Для экономии памяти удаляйте элементы после определённого ранга с помощью ZREMRANGEBYRANK.
    - Например, хранить только топ-10 000 игроков.

3.2. **Пагинация и кэширование**
    - Для частых запросов топа кэшируйте результат в памяти приложения на 1-5 секунд.
    - Используйте ZRANGE с LIMIT для эффективной пагинации.
    - При больших offset (например, страница 1000) операция может быть медленной.
      Решение: используйте ZRANGEBYSCORE с курсором или храните топ в отдельных ключах.

3.3. **Кластеризация**
    - В Redis Cluster ZSET работает только в пределах одного слота.
    - Для лидербордов используйте хеш-теги, чтобы ключи с суффиксами дат попадали в один слот.
    - Пример: {leaderboard:game}:daily:2024-01-15.

3.4. **Высокая частота обновлений**
    - При высоких RPS используйте Pipeline для пакетных обновлений.
    - Используйте ZINCRBY вместо ZADD, чтобы избежать лишних проверок.

3.5. **Мониторинг размера**
    - Используйте ZCARD для контроля количества элементов.
    - При превышении порога уведомляйте администратора.

4. ОПТИМИЗАЦИЯ ПАМЯТИ

- ZSET в Redis использует две структуры данных: ziplist (для маленьких коллекций)
  и skiplist + dict (для больших). Переключение происходит автоматически.
- Настройки: zset-max-ziplist-entries (512) и zset-max-ziplist-value (64 байта).
- Если ваши элементы (члены) короткие и их мало (до 512), они хранятся компактно.
- Для больших лидербордов используйте короткие идентификаторы (числовые ID вместо строк).

5. АТОМАРНОСТЬ И ОБРАБОТКА КОНКУРЕНТНОСТИ

- Все операции ZADD, ZINCRBY, ZREM и чтения атомарны.
- Нет необходимости в WATCH при обновлении очков одного игрока.
- Для сложных операций (например, перевод очков между игроками) используйте Lua-скрипты.

6. ПРИМЕРЫ СЛОЖНЫХ ЗАПРОСОВ

6.1. **Топ-10 игроков с очками > 1000**
    ZRANGEBYSCORE leaderboard 1000 +inf WITHSCORES LIMIT 0 10

6.2. **Игроки на позициях с 51 по 100**
    ZREVRANGE leaderboard 50 99 WITHSCORES

6.3. **Суммарные очки за две недели**
    ZUNIONSTORE week_sum 7 leaderboard:day1 ... leaderboard:day7

6.4. **Пересечение двух лидербордов (игроки, попавшие в оба)**
    ZINTERSTORE common 2 lb1 lb2

6.5. **Обновление очков с проверкой минимального порога**
    local new = redis.call('ZINCRBY', key, delta, member)
    if new < threshold then
        redis.call('ZREM', key, member)
    end

7. ТИПИЧНЫЕ ОШИБКИ И РЕШЕНИЯ

«Использую KEYS для поиска лидербордов» → вместо этого используйте SCAN или отдельный ключ-справочник.

«Не ограничиваю размер ZSET» → память может переполниться. Используйте ZREMRANGEBYRANK.

«Обновляю очки через ZADD вместо ZINCRBY» → ZADD заменяет весь скор, а не увеличивает. Используйте ZINCRBY для атомарного увеличения.

«Не учитываю пагинацию при больших offset» → кэшируйте топ или используйте SCAN.

«Храню длинные строки в качестве членов» → используйте короткие идентификаторы (ID игроков).

«Не использую WITHSCORES при выводе» → возвращаются только имена, без очков.

8. МОНИТОРИНГ И МЕТРИКИ

Метрики для лидерборда:
- Размер ZSET (ZCARD).
- Количество обновлений в секунду.
- Время выполнения операций ZRANGE.
- Процент попаданий в кэш топа.

Алерты:
- Размер ZSET > порога.
- Время выполнения ZRANGE > 10 мс.

9. СВЯЗЬ С GO (go-redis) — ОБЗОР МЕТОДОВ

- ZAdd, ZAddNX, ZAddXX — добавление с опциями.
- ZIncrBy — увеличение очков.
- ZRangeArgs — универсальный ZRANGE (с поддержкой REV, BYSCORE, BYLEX).
- ZRevRange — устаревшая, но всё ещё работает.
- ZRank, ZRevRank — получение ранга.
- ZScore — получение очков.
- ZRem — удаление.
- ZRemRangeByRank — удаление по рангу.
- ZRemRangeByScore — удаление по очкам.
- ZCard — количество элементов.
- ZCount — количество в диапазоне.
- ZRangeByScore — с пагинацией по очкам.
- ZUnionStore, ZInterStore — объединение/пересечение.

10. ИТОГИ

Лидерборды на Redis — это мощный инструмент, который может работать с миллионами игроков
с низкой задержкой. Правильное проектирование включает выбор ключей, управление размером,
кэширование топа, атомарные обновления и мониторинг. В следующем разделе мы покажем
реальные продакшн-примеры с кодом на Go.
*/

var rdb *redis.Client
var ctx = context.Background()
var logger = log.Default()

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     10,
		MinIdleConns: 5,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

func main() {
	fmt.Println("=== ЛИДЕРБОРДЫ: ПРОДАКШН-ПРИМЕРЫ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. БАЗОВЫЙ ЛИДЕРБОРД (ТОП-N + РАНГ)
type Leaderboard struct {
	client *redis.Client
	key    string
}

func (lb *Leaderboard) AddPlayer(ctx context.Context, player string, score int64) error {
	return lb.client.ZAdd(ctx, lb.key, redis.Z{Score: float64(score), Member: player}).Err()
}

func (lb *Leaderboard) GetTopN(ctx context.Context, n int64) ([]redis.Z, error) {
	// Используем ZRangeArgs с Rev: true для порядка убывания
	res, err := lb.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   lb.key,
		Start: 0,
		Stop:  n - 1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, err
	}
	// Конвертируем плоский слайс в []redis.Z
	zs := make([]redis.Z, 0, n)
	for i := 0; i < len(res); i += 2 {
		member := res[i]
		score, _ := strconv.ParseFloat(res[i+1], 64)
		zs = append(zs, redis.Z{Score: score, Member: member})
	}
	return zs, nil
}

func (lb *Leaderboard) GetRank(ctx context.Context, player string) (int64, error) {
	// Ранг в порядке убывания (0 = первое место)
	rank, err := lb.client.ZRevRank(ctx, lb.key, player).Result()
	if err == redis.Nil {
		return -1, nil // игрок не найден
	}
	return rank, err
}

func primer1() {
	fmt.Println("--- 1. Базовый лидерборд ---")

	// Использование
	key := "lb:game1"
	lb := &Leaderboard{client: rdb, key: key}
	rdb.Del(ctx, key)

	// Добавляем игроков
	players := map[string]int64{
		"Alice": 1500, "Bob": 2300, "Charlie": 1800,
		"Diana": 1200, "Eve": 2100,
	}
	for name, score := range players {
		lb.AddPlayer(ctx, name, score)
	}

	// Получаем топ-3
	top, _ := lb.GetTopN(ctx, 3)
	fmt.Println("Топ-3 игроков:")
	for i, z := range top {
		fmt.Printf("  %d. %s: %.0f\n", i+1, z.Member, z.Score)
	}

	// Ранг Alice
	rank, _ := lb.GetRank(ctx, "Alice")
	fmt.Printf("Ранг Alice: %d (с 1)\n", rank+1)

	rdb.Del(ctx, key)
	fmt.Println()
}

// 2. ПАГИНАЦИЯ С КЭШИРОВАНИЕМ ТОПА
type CachedLeaderboard struct {
	client   *redis.Client
	key      string
	cacheKey string
	cacheTTL time.Duration
}

func (lb *CachedLeaderboard) GetTopN(ctx context.Context, n int64) ([]redis.Z, error) {
	// Пытаемся получить из кэша (в отдельном ключе)
	cached, err := lb.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   lb.cacheKey,
		Start: 0,
		Stop:  n - 1,
		Rev:   true,
	}).Result()
	if err == nil && len(cached) > 0 {
		zs := make([]redis.Z, 0, n)
		for i := 0; i < len(cached); i++ {
			member := cached[i]
			score, _ := strconv.ParseFloat(cached[i+1], 64)
			zs = append(zs, redis.Z{Score: score, Member: member})
		}
		return zs, nil
	}
	// Кэш промах — загружаем из основного лидерборда
	res, err := lb.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   lb.key,
		Start: 0,
		Stop:  n - 1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, err
	}
	zs := make([]redis.Z, 0, n)
	for i := 0; i < len(res); i += 2 {
		member := res[i]
		score, _ := strconv.ParseFloat(res[i+1], 64)
		zs = append(zs, redis.Z{Score: score, Member: member})
	}
	// Сохраняем в кэш (используем ZADD для простоты, можно хранить сериализованный JSON)
	pipe := lb.client.Pipeline()
	for _, z := range zs {
		pipe.ZAdd(ctx, lb.key, z)
	}
	pipe.Expire(ctx, lb.cacheKey, lb.cacheTTL)
	_, _ = pipe.Exec(ctx)
	return zs, nil
}

func primer2() {
	fmt.Println("--- 2. Пагинация с кэшированием топа ---")

	key := "lb:game2"
	cacheKey := "lb:game2:cache:top10"
	lb := &CachedLeaderboard{client: rdb, key: key, cacheKey: cacheKey, cacheTTL: 10 * time.Second}
	rdb.Del(ctx, key, cacheKey)

	// Добавляем 100 игроков
	for i := 1; i <= 100; i++ {
		rdb.ZAdd(ctx, key, redis.Z{Score: float64(i * 10), Member: fmt.Sprintf("Player_%d", i)})
	}

	// Первый запрос — кэш пуст, загружаем и сохраняем
	top, _ := lb.GetTopN(ctx, 5)
	fmt.Println("Топ-5 (из кэша после первого запроса):")
	for i, z := range top {
		fmt.Printf("  %d. %s: %.0f\n", i+1, z.Member, z.Score)
	}

	// Второй запрос — из кэша
	top2, _ := lb.GetTopN(ctx, 5)
	fmt.Println("Топ-5 (из кэша):")
	for i, z := range top2 {
		fmt.Printf("  %d. %s: %.0f\n", i+1, z.Member, z.Score)
	}

	rdb.Del(ctx, key, cacheKey)
	fmt.Println()
}

// 3. ОБНОВЛЕНИЕ ОЧКОВ С ВАЛИДАЦИЕЙ
func primer3() {
	fmt.Println("--- 3. Обновление очков с валидацией ---")

	// Сценарий: игрок может получить очки только если они положительные
	// и не превышают максимум. Используем Lua для атомарной проверки.
	updateScript := redis.NewScript(`
		local key = KEYS[1]
		local player = ARGV[1]
		local delta = tonumber(ARGV[2])
		local max_score = tonumber(ARGV[3])

		local current = redis.call('ZSCORE', key, player)
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		local new_score = current + delta
		if new_score < 0 or new_score > max_score then
			return {err = "invalid score"}
		end
		redis.call('ZADD', key, new_score, player)
		return {ok = new_score}
	`)

	key := "lb:game3"
	rdb.Del(ctx, key)
	rdb.ZAdd(ctx, key, redis.Z{Score: 100, Member: "Alice"})

	// Функция обновления
	updatePlayer := func(player string, delta int64, maxScore int64) (int64, error) {
		res, err := updateScript.Run(ctx, rdb, []string{key}, player, delta, maxScore).Result()
		if err != nil {
			return 0, err
		}
		resultMap := res.(map[interface{}]interface{})
		if errMsg, ok := resultMap["err"]; ok {
			return 0, fmt.Errorf("%v", errMsg)
		}
		newScore := resultMap["ok"].(int64)
		return newScore, nil
	}

	// Успешное обновление
	newScore, err := updatePlayer("Alice", 50, 200)
	if err != nil {
		logger.Printf("Ошибка: %v", err)
	} else {
		fmt.Printf("Новые очки Alice: %d\n", newScore)
	}

	// Попытка превысить максимум
	_, err = updatePlayer("Alice", 100, 200)
	if err != nil {
		fmt.Printf("Ошибка (ожидаемо): %v\n", err)
	}

	rdb.Del(ctx, key)
	fmt.Println()
}

// 4. УДАЛЕНИЕ ИГРОКА И ОГРАНИЧЕНИЕ РАЗМЕРА
func primer4() {
	key := "lb:4"
	rdb.Del(ctx, key)

	for i := 1; i <= 10; i++ {
		rdb.ZAdd(ctx, key, redis.Z{Score: float64(i * 10), Member: fmt.Sprintf("P%d", i)})
	}
	// Удаляем игрока P5
	rdb.ZRem(ctx, key, "P5")
	// Оставляем только топ-5 (удаляем ранги 5 и выше)
	rdb.ZRemRangeByRank(ctx, key, 5, -1)

	res, _ := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   key,
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}).Result()
	fmt.Println("Оставшиеся игроки (топ-5):")
	for i := 0; i < len(res); i += 2 {
		fmt.Printf("  %s: %s\n", res[i], res[i+1])
	}
	rdb.Del(ctx, key)
	fmt.Println()
}

// 5. ПОЛУЧЕНИЕ ОКРУЖЕНИЯ ИГРОКА (СОСЕДИ ПО РАНГУ)
func primer5() {
	key := "lb:5"
	rdb.Del(ctx, key)

	for i := 1; i <= 20; i++ {
		rdb.ZAdd(ctx, key, redis.Z{Score: float64(i * 10), Member: fmt.Sprintf("P%d", i)})
	}
	player := "P10"
	rank, _ := rdb.ZRevRank(ctx, key, player).Result()
	radius := int64(3)
	start := rank - radius
	if start < 0 {
		start = 0
	}
	stop := rank + radius

	res, _ := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   key,
		Start: start,
		Stop:  stop,
		Rev:   true,
	}).Result()
	fmt.Printf("Соседи %s (радиус 3):\n", player)
	for i := 0; i < len(res); i += 2 {
		marker := ""
		if res[i] == player {
			marker = " ←"
		}
		fmt.Printf("  %s: %s%s\n", res[i], res[i+1], marker)
	}
	rdb.Del(ctx, key)
	fmt.Println()
}

// 6. СЕЗОННЫЙ ЛИДЕРБОРД (ЕЖЕДНЕВНЫЙ С TTL)
func primer6() {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("lb:daily:%s", today)
	rdb.Del(ctx, key)

	rdb.ZAdd(ctx, key, redis.Z{Score: 100, Member: "Mina"})
	rdb.ZAdd(ctx, key, redis.Z{Score: 300, Member: "Kolya"})
	rdb.Expire(ctx, key, 24*time.Hour)

	ttl, _ := rdb.TTL(ctx, key).Result()
	fmt.Printf("Ключ %s, TTL: %v\n", key, ttl)
	rdb.Del(ctx, key)
	fmt.Println()
}

// 7. ОБЪЕДИНЕНИЕ НЕСКОЛЬКИХ ЛИДЕРБОРДОВ (ZUNIONSTORE)
func primer7() {
	k1, k2, dest := "lb:7a", "lb:7b", "lb:7dest"
	rdb.Del(ctx, k1, k2, dest)

	// День 1
	rdb.ZAdd(ctx, k1, redis.Z{Score: 100, Member: "A"}, redis.Z{Score: 80, Member: "B"})
	// День 2
	rdb.ZAdd(ctx, k2, redis.Z{Score: 90, Member: "A"}, redis.Z{Score: 110, Member: "C"})

	// Объединяем с суммированием
	rdb.ZUnionStore(ctx, dest, &redis.ZStore{
		Keys:      []string{k1, k2},
		Weights:   []float64{1, 1},
		Aggregate: "SUM",
	})

	res, _ := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   dest,
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}).Result()
	fmt.Println("Объединённый лидерборд:")
	for i := 0; i < len(res); i += 2 {
		fmt.Printf("  %s: %s\n", res[i], res[i+1])
	}

	rdb.Del(ctx, k1, k2, dest)
	fmt.Println()
}

// 8. КОМПОЗИТНЫЙ СКОР (МУЛЬТИМЕТРИКИ)
func primer8() {
	key := "lb:8"
	rdb.Del(ctx, key)

	// Составной скор: wins * 1 000 000 + kills
	mult := int64(1_000_000)
	rdb.ZAdd(ctx, key, redis.Z{Score: float64(10*mult + 50), Member: "Alice"})
	rdb.ZAdd(ctx, key, redis.Z{Score: float64(10*mult + 30), Member: "Bob"})
	rdb.ZAdd(ctx, key, redis.Z{Score: float64(8*mult + 100), Member: "Charlie"})

	res, _ := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   key,
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}).Result()
	fmt.Println("Рейтинг (сначала победы, потом убийства):")
	for i := 0; i < len(res); i += 2 {
		score, _ := strconv.ParseInt(res[i+1], 10, 64)
		wins := score / mult
		kills := score % mult
		fmt.Printf("  %s: победы %d, убийства %d\n", res[i], wins, kills)
	}
	rdb.Del(ctx, key)
	fmt.Println()
}
