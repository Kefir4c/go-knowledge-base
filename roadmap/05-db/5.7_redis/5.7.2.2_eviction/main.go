package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 2.2. ПОЛИТИКИ ВЫТЕСНЕНИЯ (EVICTION)
1. ЧТО ТАКОЕ ВЫТЕСНЕНИЕ (EVICTION)?

Вытеснение — это процесс автоматического удаления ключей из Redis,
когда использование памяти достигает установленного лимита maxmemory.

Redis — это in-memory база данных, и память не бесконечна. Когда кэш заполняется,
он должен либо отказать в записи новых данных, либо удалить часть старых,
чтобы освободить место.

Процесс вытеснения запускается при КАЖДОЙ команде записи (SET, LPUSH, HSET и т.д.):
1. Клиент отправляет команду, которая добавляет данные.
2. Redis проверяет использование памяти.
3. Если память превышает maxmemory, Redis запускает вытеснение.
4. Redis удаляет ключи согласно выбранной политике до тех пор, пока память не опустится ниже лимита.

Важно: maxmemory — это НЕ точный лимит. Redis может временно превысить его
при выполнении больших команд (например, SINTERSTORE), но затем запускает
вытеснение, чтобы вернуться под лимит.

2. maxmemory: НАСТРОЙКА ЛИМИТА ПАМЯТИ

maxmemory задаётся в redis.conf или через CONFIG SET:
    maxmemory 1gb

Особенности:
- maxmemory = 0 — лимит отключён (по умолчанию для 64-битных систем).
- 32-битные системы имеют неявный лимит 3 ГБ.
- При использовании репликации или AOF буферы НЕ учитываются в maxmemory,
  иначе может возникнуть порочный круг: вытеснение → новые команды репликации →
  занятие памяти → ещё больше вытеснения.

Рекомендация: оставлять ~20% свободной памяти сверх maxmemory для буферов
и пиковых нагрузок.

3. ВСЕ ПОЛИТИКИ ВЫТЕСНЕНИЯ (8 штук)

Все политики делятся на 3 категории:

А. БЕЗ ВЫТЕСНЕНИЯ (1 политика)
Б. СЛУЧАЙНОЕ ВЫТЕСНЕНИЕ (2 политики)
В. АЛГОРИТМИЧЕСКОЕ ВЫТЕСНЕНИЕ (5 политик)

3.1. noeviction (ПО УМОЛЧАНИЮ)

Поведение: при достижении maxmemory Redis НЕ удаляет никакие ключи.
Любая команда записи возвращает ошибку OOM (Out Of Memory).
Команды чтения (GET) и удаления (DEL) продолжают работать.

Когда использовать:
- Данные критичны и НЕЛЬЗЯ терять (финансовые транзакции, пользовательские данные).
- Вы предпочитаете получить ошибку, чем потерять данные.

Риски:
- Приложение может упасть, если не обрабатывает ошибки записи.
- Требуется внешний мониторинг и ручное вмешательство при заполнении памяти.

3.2. allkeys-lru (САМАЯ ПОПУЛЯРНАЯ ДЛЯ КЭША)

Поведение: удаляет ключи, которые использовались РЕЖЕ ВСЕГО (Least Recently Used)
среди ВСЕХ ключей, независимо от наличия TTL.

Когда использовать:
- Чистый кэш, где данные можно потерять и восстановить из БД.
- Доступ к данным подчиняется степенному распределению (80/20).
- Все данные равны по важности.

Пример: кэш товаров интернет-магазина.

3.3. volatile-lru (ДЛЯ СМЕШАННЫХ ДАННЫХ)

Поведение: удаляет редко используемые ключи ТОЛЬКО среди тех,
у которых есть TTL. Ключи без TTL НИКОГДА не удаляются.

Когда использовать:
- Есть данные, которые НЕЛЬЗЯ удалять (конфигурация, справочники).
- Есть кэшируемые данные с TTL, которые можно удалять.
- Нужно защитить "вечные" ключи от вытеснения.

Пример: сессии пользователей (TTL) + системные настройки (без TTL).

3.4. allkeys-random (ПРОСТОЙ И БЫСТРЫЙ)

Поведение: удаляет СЛУЧАЙНЫЕ ключи среди ВСЕХ.
Не анализирует, не считает — просто берёт и удаляет.

Когда использовать:
- Все ключи примерно одинаково важны.
- Доступ к данным равномерный (нет "горячих" ключей).
- Нужна минимальная нагрузка на CPU при вытеснении.

Когда НЕ использовать:
- Если есть ключи, которые нельзя терять (лучше volatile-random).
- Если есть явные "горячие" данные (лучше LRU/LFU).

3.5. volatile-random

Поведение: удаляет СЛУЧАЙНЫЕ ключи ТОЛЬКО среди ключей с TTL.
Ключи без TTL не трогает.

Когда использовать:
- Нужно защитить ключи без TTL.
- Все ключи с TTL примерно одинаково важны.
- Простота важнее эффективности.

3.6. volatile-ttl (УДАЛЯЕТ ТО, ЧТО СКОРО УМРЁТ)

Поведение: удаляет ключи с TTL, у которых осталось МЕНЬШЕ ВСЕГО времени до истечения
(сортировка по возрастанию TTL).

Важно: когда все ключи с TTL удалены, а память всё ещё заполнена,
Redis переходит в режим noeviction и возвращает OOM ошибку.

Когда использовать:
- Много ключей с разными TTL, и вы хотите удалять те, что скоро истекут сами.
- Хотите максимально продлить жизнь ключам с большим TTL.

Когда НЕ использовать:
- Если у всех ключей примерно одинаковый TTL — эффекта не будет.
- Если нужно удалять редко используемые ключи (лучше volatile-lru).

3.7. allkeys-lfu (НОВИНКА С REDIS 4.0)

Поведение: удаляет ключи, которые используются РЕЖЕ ВСЕГО по ЧАСТОТЕ обращений
(Least Frequently Used) среди ВСЕХ ключей.

Отличие от LRU:
- LRU смотрит на ВРЕМЯ последнего доступа.
- LFU смотрит на ЧАСТОТУ обращений за всё время.

Когда использовать LFU вместо LRU[reference:30]:
- Нагрузка ПРЕДСКАЗУЕМАЯ: есть явные "популярные" и "непопулярные" данные.
- Нужно сохранять в кэше то, что запрашивают ЧАСТО, даже если давно.
- Пример: бестселлеры, которые продаются годами.

Когда лучше LRU[reference:32]:
- Нагрузка ДИНАМИЧЕСКАЯ: тренды меняются быстро.
- Важна СВЕЖЕСТЬ данных, а не частота.
- Пример: пользовательские сессии, дашборды.

LFU использует вероятностный счётчик (Morris counter) для оценки частоты,
что позволяет хранить информацию о частоте в нескольких битах на объект.

3.8. volatile-lfu (С REDIS 4.0)

Поведение: удаляет редко используемые по частоте ключи ТОЛЬКО среди ключей с TTL.
Ключи без TTL не трогает.

Когда использовать:
- Те же сценарии, что и volatile-lru, но с LFU вместо LRU.

4. КАК РАБОТАЮТ ПРИБЛИЖЁННЫЕ АЛГОРИТМЫ (LRU / LFU)

Redis НЕ использует точные алгоритмы LRU/LFU с глобальным списком всех ключей
— это было бы слишком дорого по памяти и CPU.

Вместо этого Redis использует ПРИБЛИЖЁННЫЕ (probabilistic) алгоритмы:

1. Redis выбирает СЛУЧАЙНУЮ ВЫБОРКУ из N ключей (по умолчанию N=5).
2. Из этих N ключей выбирается ЛУЧШИЙ КАНДИДАТ для вытеснения.
3. Для LRU: ключ с самым старым временем последнего доступа.
4. Для LFU: ключ с наименьшей частотой обращений.
5. Этот ключ удаляется.

Этот процесс повторяется, пока память не освободится.

Настройка точности: параметр maxmemory-samples (по умолчанию 5).
- Больше выборок = точнее приближение, но выше нагрузка на CPU.
- Меньше выборок = быстрее, но менее точное вытеснение.

Почему приближение:
- Точный LRU требует хранения глобального списка всех ключей (дополнительная память).
- Точный LRU требует обновления списка при КАЖДОМ обращении (нагрузка на CPU).
- Приближение даёт "достаточно хороший" результат с минимальными затратами.

5. ПРОЦЕСС ВЫТЕСНЕНИЯ: ПОШАГОВО

Шаг 1: Клиент отправляет команду записи (SET, LPUSH, HSET и т.д.).
Шаг 2: Redis добавляет данные, память может временно превысить maxmemory.
Шаг 3: Redis проверяет: used_memory > maxmemory?
Шаг 4: Если ДА, запускается цикл вытеснения:
    a) Выбирается случайная выборка ключей (размер = maxmemory-samples).
    b) Согласно политике, выбирается "лучший" ключ для удаления.
    c) Ключ удаляется (и синхронизируется с репликами через DEL-команды).
    d) Проверяется память: если всё ещё > maxmemory → повторяем.
Шаг 5: Когда память < maxmemory, команда завершается.
Шаг 6: Клиент получает ответ.

Важно: при большой команде (например, сохранение 10 МБ данных) Redis может
временно превысить maxmemory на 10 МБ, а затем запустить вытеснение.

6. МОНИТОРИНГ ВЫТЕСНЕНИЯ

Основные метрики через INFO stats:

    redis-cli INFO stats | grep evicted_keys

- evicted_keys — общее количество ключей, вытесненных за всё время работы.
- Если evicted_keys растёт → память заканчивается, нужно что-то делать.
- Если evicted_keys = 0 → вытеснения не было (либо памяти достаточно,
  либо политика noeviction).

Метрики памяти через INFO memory:

    redis-cli INFO memory | grep used_memory

- used_memory — текущее использование памяти.
- used_memory_peak — пиковое использование памяти.
- used_memory_human — человекочитаемый формат.

Рекомендация: мониторить used_memory и бить тревогу при 90% от maxmemory.

7. КАК ВЫБРАТЬ ПОЛИТИКУ: ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ

┌─────────────────────────────────┬─────────────────────────────────────────────┐
│ СИТУАЦИЯ                        │ РЕКОМЕНДУЕМАЯ ПОЛИТИКА                     │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Чистый кэш, данные можно потерять│ allkeys-lru (или allkeys-lfu)             │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Кэш + вечные настройки          │ volatile-lru (или volatile-lfu)            │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Данные критичны, нельзя терять  │ noeviction + масштабирование / мониторинг  │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Все ключи одинаково важны       │ allkeys-random                             │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Много TTL, хотим удалять        │ volatile-ttl                               │
│ скоро истекшие                  │                                             │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Есть явные "популярные" ключи   │ allkeys-lfu / volatile-lfu                 │
├─────────────────────────────────┼─────────────────────────────────────────────┤
│ Динамические тренды, свежесть   │ allkeys-lru / volatile-lru                 │
│ важнее частоты                  │                                             │
└─────────────────────────────────┴─────────────────────────────────────────────┘

8. СВЯЗЬ С GO

В Go-коде политика НЕ настраивается — это задача DevOps / администратора.

Но вы ДОЛЖНЫ:
1. ЗНАТЬ, какая политика выбрана на сервере (через CONFIG GET).
2. ПРОЕКТИРОВАТЬ кэширование с учётом политики.
3. МОНИТОРИТЬ evicted_keys через INFO.

Пример проверки политики из Go:
    val, _ := rdb.ConfigGet(ctx, "maxmemory-policy").Result()
    fmt.Println(val["maxmemory-policy"])

Пример проверки evicted_keys из Go:
    info, _ := rdb.InfoMap(ctx, "stats").Result()
    evicted := info["stats"]["evicted_keys"]
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
	fmt.Println("=== ПРИМЕРЫ ВЫТЕСНЕНИЯ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// ПРИМЕР 1: allkeys-lru для кэша товаров интернет-магазина
func primer1() {
	fmt.Println("--- 1. allkeys-lru: кэш товаров (LRU) ---")

	// Настраиваем небольшой лимит для демонстрации
	setConfig("maxmemory", "10mb")
	setConfig("maxmemory-policy", "allkeys-lru")

	// Создаём 500 товаров с разным "рейтингом" (имитация)
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("product:%d", i)
		val := make([]byte, 1024) // 1 KB
		rand.Read(val)
		rdb.Set(ctx, key, val, 0)

		// Имитируем частоту обращений: первые 100 товаров запрашиваются часто
		if i < 100 {
			for j := 0; j < 10; j++ {
				rdb.Get(ctx, key)
			}
		}
	}
	fmt.Println("Создано 500 товаров, первые 100 часто запрашивались")

	time.Sleep(2 * time.Second)

	// Проверяем, сколько товаров осталось
	keys, _ := rdb.Keys(ctx, "product:*").Result()
	fmt.Printf("Осталось товаров: %d\n", len(keys))

	// Проверяем, остались ли часто запрашиваемые (первые 100)
	survived := 0
	for i := 0; i < 100; i++ {
		exists, _ := rdb.Exists(ctx, fmt.Sprintf("product:%d", i)).Result()
		if exists == 1 {
			survived++
		}
	}
	fmt.Printf("Из первых 100 часто запрашиваемых выжило: %d\n", survived)
	resetConfig()
}

// ПРИМЕР 2: volatile-lru для сессий + вечные настройки
func primer2() {
	fmt.Println("--- 2. volatile-lru: сессии (TTL) + вечные настройки ---")

	setConfig("maxmemory", "8mb")
	setConfig("maxmemory-policy", "volatile-lru")

	// Вечные настройки (без TTL) — не должны удаляться
	rdb.Set(ctx, "config:app_name", "MyApp", 0)
	rdb.Set(ctx, "config:version", "1.2.3", 0)

	// Сессии пользователей (с TTL)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("session:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		rdb.Set(ctx, key, val, 30*time.Second)
	}
	fmt.Println("Создано 1000 сессий (TTL 30с) и 2 вечные настройки")

	time.Sleep(2 * time.Second)

	// Проверяем, что настройки сохранились
	_, err := rdb.Get(ctx, "config:app_name").Result()
	if err == nil {
		fmt.Println("Настройки сохранились (не удалены)")
	} else {
		fmt.Println("Настройки удалены (ошибка)")
	}

	// Смотрим, сколько сессий осталось
	keys, _ := rdb.Keys(ctx, "session:*").Result()
	fmt.Printf("Осталось сессий: %d\n", len(keys))

	resetConfig()
}

// ПРИМЕР 3: volatile-ttl — удаление ключей с наименьшим TTL
func primer3() {
	fmt.Println("--- 3. volatile-ttl: удаление ключей с наименьшим TTL ---")

	setConfig("maxmemory", "6mb")
	setConfig("maxmemory-policy", "volatile-ttl")

	// Создаём ключи с разным TTL (от 1 до 60 сек)
	for i := 0; i < 600; i++ {
		key := fmt.Sprintf("token:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		ttl := time.Duration(rand.Intn(60)+1) * time.Second
		rdb.Set(ctx, key, val, ttl)
	}
	fmt.Println("Создано 600 токенов с TTL от 1 до 60 сек")

	// Ждём 5 секунд — часть ключей истечёт, но останутся те, у кого TTL большой
	time.Sleep(5 * time.Second)

	keys, _ := rdb.Keys(ctx, "token:*").Result()
	fmt.Printf("Осталось токенов: %d\n", len(keys))

	// Проверяем, у каких токенов маленький TTL остался
	// (они должны быть удалены в первую очередь)
	smallTTL := 0
	for i := 0; i < len(keys); i++ {
		ttl, _ := rdb.TTL(ctx, keys[i]).Result()
		if ttl < 5*time.Second {
			smallTTL++
		}
	}
	fmt.Printf("Из оставшихся, у %d TTL < 5 сек (их должно быть мало)\n", smallTTL)
	resetConfig()
}

// ПРИМЕР 4: allkeys-lfu для часто используемых данных
func primer4() {
	fmt.Println("--- 4. allkeys-lfu: сохранение часто используемых данных ---")

	setConfig("maxmemory", "8mb")
	setConfig("maxmemory-policy", "allkeys-lfu")

	// Создаём 400 ключей, часть из них запрашивается часто
	for i := 0; i < 400; i++ {
		key := fmt.Sprintf("popular:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		rdb.Set(ctx, key, val, 0)

		// 20% ключей запрашиваются очень часто
		if i%5 == 0 {
			for j := 0; j < 100; j++ {
				rdb.Get(ctx, key)
			}
		}
	}
	fmt.Println("Создано 400 ключей, 20% из них — часто используемые")

	time.Sleep(2 * time.Second)

	// Проверяем, сколько из часто используемых сохранились
	survivedPopular := 0
	for i := 0; i < 400; i++ {
		if i%5 == 0 {
			exists, _ := rdb.Exists(ctx, fmt.Sprintf("popular:%d", i)).Result()
			if exists == 1 {
				survivedPopular++
			}
		}
	}
	fmt.Printf("Из часто используемых (80 ключей) сохранилось: %d\n", survivedPopular)

	// Проверяем, сколько сохранилось из редко используемых
	survivedRare := 0
	for i := 0; i < 400; i++ {
		if i%5 != 0 {
			exists, _ := rdb.Exists(ctx, fmt.Sprintf("popular:%d", i)).Result()
			if exists == 1 {
				survivedRare++
			}
		}
	}
	fmt.Printf("Из редко используемых (320 ключей) сохранилось: %d\n", survivedRare)
	resetConfig()
}

// ПРИМЕР 5: noeviction + fallback обработка ошибки
func primer5() {
	fmt.Println("--- 5. noeviction: обработка OOM с fallback ---")

	setConfig("maxmemory", "2mb")
	setConfig("maxmemory-policy", "noeviction")

	// Заполняем память маленькими ключами
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("small:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		rdb.Set(ctx, key, val, 0)
	}
	fmt.Println("Создано 200 маленьких ключей (1 KB каждый)")

	// Пытаемся записать большой ключ (2 MB)
	bigVal := make([]byte, 2*1024*1024)
	err := rdb.Set(ctx, "bigkey", bigVal, 0).Err()
	if err != nil {
		fmt.Printf("Ошибка записи: %v\n", err)
		fmt.Println("Fallback: удаляем 100 случайных ключей и пробуем снова")
		// Удаляем часть ключей
		keys, _ := rdb.Keys(ctx, "small:*").Result()
		if len(keys) >= 100 {
			rdb.Del(ctx, keys[:100]...)
			fmt.Println("  Удалено 100 ключей")
			err = rdb.Set(ctx, "bigkey", bigVal, 0).Err()
			if err == nil {
				fmt.Println("Запись большого ключа удалась после освобождения")
				rdb.Del(ctx, "bigkey")
			} else {
				fmt.Println("Всё равно не хватило памяти")
			}
		} else {
			fmt.Println("Запись удалась без проблем")
		}
	}
	resetConfig()
}

// ПРИМЕР 6: Сравнение cache hit ratio allkeys-lru vs volatile-lru
func primer6() {
	fmt.Println("--- 6. Сравнение hit ratio: allkeys-lru vs volatile-lru ---")

	// Функция для измерения hit ratio
	measureHitRatio := func(policy string) float64 {
		rdb.FlushDB(ctx)
		setConfig("maxmemory", "5mb")
		setConfig("maxmemory-policy", policy)

		// 60% ключей с TTL, 40% без TTL
		for i := 0; i < 600; i++ {
			key := fmt.Sprintf("cache:%d", i)
			val := make([]byte, 1024)
			rand.Read(val)
			if i%3 == 0 {
				rdb.Set(ctx, key, val, 10*time.Second)
			} else {
				rdb.Set(ctx, key, val, 0)
			}
		}

		// Симуляция запросов: 80% к существующим, 20% новые
		hits := 0
		total := 1000
		for i := 0; i < total; i++ {
			key := fmt.Sprintf("cache:%d", rand.Intn(800))
			_, err := rdb.Get(ctx, key).Result()
			if err == nil {
				hits++
			} else {
				// Miss — добавляем новый ключ (нагрузка)
				rdb.Set(ctx, fmt.Sprintf("cache:%d", 600+i), []byte("new"), 0)
			}
		}
		return float64(hits) / float64(total) * 100
	}

	lruHit := measureHitRatio("allkeys-lru")
	volatileHit := measureHitRatio("volatile-lru")

	fmt.Printf("Cache hit ratio (allkeys-lru):   %.1f%%\n", lruHit)
	fmt.Printf("Cache hit ratio (volatile-lru): %.1f%%\n", volatileHit)

	fmt.Println("Вывод: allkeys-lru эффективнее для кэша, т.к. не защищает вечные ключи.")
	resetConfig()
}

// ПРИМЕР 7: Адаптивная стратегия — ручное управление вытеснением
func primer7() {
	fmt.Println("--- 7. Адаптивная стратегия: ручное управление ---")

	setConfig("maxmemory", "3mb")
	setConfig("maxmemory-policy", "noeviction")

	// Добавляем ключи, пока не получим ошибку
	var mu sync.Mutex
	keysAdded := 0
	for i := 0; ; i++ {
		key := fmt.Sprintf("item:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		err := rdb.Set(ctx, key, val, 0).Err()
		if err != nil {
			// Память заполнилась — запускаем адаптивный алгоритм
			fmt.Printf("Память заполнилась на ключе %d\n", i)
			mu.Lock()
			// Удаляем 50% старых ключей
			keys, _ := rdb.Keys(ctx, "item:*").Result()
			if len(keys) > 100 {
				toDelete := len(keys) / 2
				rdb.Del(ctx, keys[:toDelete]...)
				fmt.Printf("  Удалено %d старых ключей\n", toDelete)
				// Пробуем записать снова
				err = rdb.Set(ctx, key, val, 0).Err()
				if err == nil {
					fmt.Printf("Запись ключа %d удалась после очистки\n", i)
					keysAdded++
				} else {
					fmt.Println("Даже после очистки не хватило памяти")
					mu.Unlock()
					break
				}
			} else {
				fmt.Println("Недостаточно ключей для удаления")
				mu.Unlock()
				break
			}
			mu.Unlock()
		} else {
			keysAdded++
		}
		if i > 5000 { // Защита от бесконечного цикла
			break
		}
	}
	fmt.Printf("Всего добавлено ключей: %d\n", keysAdded)

	resetConfig()
}

// ПРИМЕР 8: Мониторинг evicted_keys и алертинг
func primer8() {
	fmt.Println("--- 8. Мониторинг evicted_keys и оповещение ---")

	setConfig("maxmemory", "6mb")
	setConfig("maxmemory-policy", "allkeys-lru")

	// Генерируем нагрузку, вызывающую вытеснение
	for i := 0; i < 2000; i++ {
		key := fmt.Sprintf("monitor:%d", i)
		val := make([]byte, 1024)
		rand.Read(val)
		rdb.Set(ctx, key, val, 0)
	}

	// Собираем статистику
	info, _ := rdb.InfoMap(ctx, "stats").Result()
	evicted, _ := strconv.ParseInt(info["stats"]["evicted_keys"], 10, 64)

	memInfo, _ := rdb.InfoMap(ctx, "memory").Result()
	used, _ := strconv.ParseInt(memInfo["memory"]["used_memory"], 10, 64)

	maxMem, _ := rdb.ConfigGet(ctx, "maxmemory").Result()
	max, _ := strconv.ParseInt(maxMem["maxmemory"], 10, 64)

	fmt.Printf("Статистика памяти:\n")
	fmt.Printf("maxmemory: %d MB\n", max/(1024*1024))
	fmt.Printf("used_memory: %d MB\n", used/(1024*1024))
	fmt.Printf("evicted_keys: %d\n", evicted)

	// Алертинг: если вытеснено > 1000 ключей — бить тревогу
	if evicted > 1000 {
		fmt.Printf("ВНИМАНИЕ: вытеснено более 1000 ключей! (%d)\n", evicted)
		fmt.Println("Рекомендуется увеличить maxmemory или изменить политику.")
	} else {
		fmt.Println("Уровень вытеснения в пределах нормы.")
	}
	resetConfig()
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
func setConfig(param, value string) {
	err := rdb.ConfigSet(ctx, param, value).Err()
	if err != nil {
		fmt.Printf("Не удалось установить %s = %s: %v\n", param, value, err)
	}
}
func resetConfig() {
	// Возвращаем безопасные значения по умолчанию
	rdb.ConfigSet(ctx, "maxmemory", "0")
	rdb.ConfigSet(ctx, "maxmemory-policy", "noeviction")
	// Не флашим — пусть данные останутся для следующих примеров (но они сами чистят)
}
