package list

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 2. СПИСКИ (LIST) В REDIS

1. ЧТО ТАКОЕ СПИСОК?
   - Список — это упорядоченная коллекция строк, где элементы хранятся в порядке добавления.
   - Каждый элемент имеет индекс (0 — первый, -1 — последний).
   - Максимальная длина списка: 2³² - 1 (более 4 миллиардов элементов).
   - Операции вставки/удаления с обоих концов выполняются за O(1), а доступ по индексу — O(N).

2. ГЛАВНОЕ ПРИМЕНЕНИЕ:
   - Очереди (FIFO): RPUSH + LPOP или LPUSH + RPOP.
   - Стеки (LIFO): LPUSH + LPOP или RPUSH + RPOP.
   - Логи, хронологические события (новые добавляются в конец).
   - Брокеры сообщений (с блокирующими командами для ожидания).
   - Хранение последних N действий пользователя (например, история).

3. ОСНОВНЫЕ КОМАНДЫ И ИХ ДЕЙСТВИЕ (ПОДРОБНО)
   -------------------------------------------------------------------------------
   LPUSH key value [value ...]
   - Добавляет одно или несколько значений в начало списка (слева).
   - Если ключа нет, создаёт новый список.
   - Возвращает новую длину списка.
   - CLI: LPUSH mylist "a" "b" → список: [b, a] (так как добавляем по очереди слева)
   - Go: newLen, err := rdb.LPush(ctx, key, "a", "b").Result()

   RPUSH key value [value ...]
   - Добавляет значения в конец списка (справа).
   - Возвращает новую длину.
   - CLI: RPUSH mylist "c" → [b, a, c]
   - Go: newLen, err := rdb.RPush(ctx, key, "c").Result()

   LPOP key
   - Удаляет и возвращает первый элемент списка (слева).
   - Если список пуст, возвращает (nil) в CLI, в Go — redis.Nil.
   - CLI: LPOP mylist → "b"
   - Go: val, err := rdb.LPop(ctx, key).Result()

   RPOP key
   - Удаляет и возвращает последний элемент списка (справа).
   - Аналогичен LPOP, но с другого конца.
   - Go: val, err := rdb.RPop(ctx, key).Result()

   LRANGE key start stop
   - Возвращает элементы списка в диапазоне от start до stop (включительно).
   - Индексы могут быть отрицательными (-1 — последний элемент).
   - CLI: LRANGE mylist 0 -1 → все элементы
   - Go: vals, err := rdb.LRange(ctx, key, 0, -1).Result() // []string

   LLEN key
   - Возвращает длину списка.
   - CLI: LLEN mylist → 2
   - Go: length, err := rdb.LLen(ctx, key).Result()

   LINDEX key index
   - Возвращает элемент по индексу (0 — первый, -1 — последний).
   - Если индекса нет, возвращает (nil).
   - CLI: LINDEX mylist 1 → "a"
   - Go: val, err := rdb.LIndex(ctx, key, 1).Result()

   BLPOP key [key ...] timeout
   - Блокирующая команда: удаляет и возвращает первый элемент из первого непустого списка.
   - Если все списки пусты, ждёт до истечения timeout (в секундах) или появления элемента.
   - Возвращает пару (ключ, элемент). При timeout возвращает (nil).
   - CLI: BLPOP mylist 0 → блокирует бесконечно, пока не появится элемент
   - Go: result, err := rdb.BLPop(ctx, timeout, "mylist1", "mylist2").Result()
          result — это []string длиной 2: [ключ, значение]

   Дополнительные команды (не в основном списке, но полезны):
   - BRPOP — как BLPOP, но с правого конца.
   - RPOPLPUSH — перемещает элемент из одного списка в другой (атомарно).
   - LREM — удаляет элементы по значению.
   - LINSERT — вставляет элемент перед/после указанного.
   - LSET — устанавливает значение по индексу.
   - LPOS — возвращает индекс элемента.

4. ВАЖНЫЕ СВОЙСТВА И ПРАВИЛА:
   - Все операции вставки/удаления с головы/хвоста O(1).
   - Доступ к середине списка (LINDEX, LRANGE с большим диапазоном) O(N), поэтому для частого доступа к середине лучше использовать другие структуры.
   - Списки поддерживают блокирующие операции (BLPOP/BRPOP) — идеально для очередей с ожиданием.
   - Если список пуст, LPOP/RPOP возвращают nil, а не ошибку (в Go — redis.Nil).
   - Можно использовать несколько ключей в BLPOP — берётся первый непустой.

5. КОГДА НЕ ИСПОЛЬЗОВАТЬ СПИСКИ:
   - Для частого доступа по индексу — лучше использовать массив в памяти.
   - Для множеств уникальных элементов — лучше Set.
   - Для сортировки по score — Sorted Set.

6. СВЯЗЬ С GO (go-redis/v9):
   - Основные методы:
      * LPush(ctx, key, values...) — *IntCmd
      * RPush(ctx, key, values...) — *IntCmd
      * LPop(ctx, key) — *StringCmd (возвращает redis.Nil при пустом списке)
      * RPop(ctx, key) — *StringCmd
      * LRange(ctx, key, start, stop) — *StringSliceCmd
      * LLen(ctx, key) — *IntCmd
      * LIndex(ctx, key, index) — *StringCmd
      * BLPop(ctx, timeout, keys...) — *StringSliceCmd (возвращает [ключ, значение])
      * BRPop(ctx, timeout, keys...) — аналогично.
   - Для BLPop timeout задаётся как time.Duration, например, 5*time.Second.

7. ТИПИЧНЫЕ ОШИБКИ:
   - redis.Nil при LPop/RPop на пустом списке (нужно обрабатывать).
   - BLPop без timeout (0) блокирует бесконечно — нужно предусмотреть контекст с таймаутом.
   - Большие списки: LRANGE 0 -1 может быть дорогим, используйте ограничение.

8. ПАТТЕРНЫ ИСПОЛЬЗОВАНИЯ:
   - Очередь задач: RPUSH (добавить задачу), LPOP (забрать задачу) — с BLPOP для ожидания.
   - Стек: LPUSH + LPOP или RPUSH + RPOP.
   - История действий пользователя: LPUSH + LTRIM (ограничить размер).
   - Планировщик: список задач с временем выполнения, проверка каждую минуту.
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

// 1. LPUSH, RPUSH, LRANGE (добавление и просмотр)
func primer1() {
	fmt.Println("--- 1. LPUSH, RPUSH, LRANGE ---")
	key := "mylist"
	rdb.Del(ctx, key) // очищаем перед примером

	// Добавляем слева (в начало)
	len1, _ := rdb.LPush(ctx, key, "a", "b", "c").Result()
	fmt.Printf("LPUSH a b -> длина %d\n", len1)

	// Добавляем справа (в конец)
	len2, _ := rdb.RPush(ctx, key, "r", "e").Result()
	fmt.Printf("RPUSH c d -> длина %d\n", len2)

	// Получаем все элементы
	vals, _ := rdb.LRange(ctx, key, 0, -1).Result()
	fmt.Printf("LRANGE 0 -1: %v\n", vals) // Ожидаем: [b, a, c, d]

	rdb.Del(ctx, key)
}

// 2. LPOP и RPOP (извлечение)
func primer2() {
	fmt.Println("--- 2. LPOP, RPOP ---")
	key := "stack"
	rdb.Del(ctx, key)

	rdb.RPush(ctx, key, "one", "two", "three")
	vals, _ := rdb.LRange(ctx, key, 0, -1).Result()
	fmt.Printf("Начальный список: %v\n", vals)

	// Извлекаем слева
	left, _ := rdb.LPop(ctx, key).Result()
	fmt.Printf("LPOP -> %s\n", left)

	// Извлекаем справа
	right, _ := rdb.RPop(ctx, key).Result()
	fmt.Printf("RPOP -> %s\n", right)

	// Оставшиеся элементы
	remaining, _ := rdb.LRange(ctx, key, 0, -1).Result()
	fmt.Printf("Осталось: %v\n", remaining) // Ожидаем: [two]

	// Попытка извлечь из пустого списка
	rdb.LPop(ctx, key) // удаляем последний
	_, err := rdb.LPop(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("LPop пустого списка -> redis.Nil")
	}

	rdb.Del(ctx, key)

}

// 3. LLEN и LINDEX
func primer3() {
	fmt.Println("--- 3. LLEN, LINDEX ---")
	key := "numbers"
	rdb.Del(ctx, key)
	rdb.RPush(ctx, key, 10, 20, 30, 40)

	// Длина
	length, _ := rdb.LLen(ctx, key).Result()
	fmt.Printf("LLEN = %d\n", length)

	// Получение по индексу
	for i := int64(0); i < length; i++ {
		val, _ := rdb.LIndex(ctx, key, i).Result()
		fmt.Printf("LINDEX %d = %s\n", i, val)
	}

	// Отрицательный индекс
	last, _ := rdb.LIndex(ctx, key, -1).Result()
	fmt.Printf("LINDEX -1 = %s\n", last)

	// Несуществующий индекс
	_, err := rdb.LIndex(ctx, key, 100).Result()
	if errors.Is(err, redis.Nil) {
		fmt.Println("LINDEX 100 -> redis.Nil")
	}

	rdb.Del(ctx, key)
}

// 4. Блокирующая BLPOP (ожидание элемента)
func primer4() {
	fmt.Println("--- 4. BLPOP (блокирующая) ---")
	key := "queue"
	rdb.Del(ctx, key)

	// Запускаем потребитель в отдельной горутине, который ждёт элемент
	go func() {
		fmt.Println("  Потребитель ждёт элемент...")
		// Блокируем на 5 секунд (если элемента нет, вернёт nil)
		result, err := rdb.BLPop(ctx, 5*time.Second, key).Result()
		if err != nil {
			fmt.Println("Ошибка BLPOP:", err)
			return
		}
		if len(result) == 0 {
			fmt.Println("Таймаут, элемента не было")
		} else {
			fmt.Printf("Получено: ключ=%s, значение=%s\n", result[0], result[1])
		}
	}()

	// Даём потребителю время запуститься
	time.Sleep(1 * time.Second)

	// Производитель добавляет элемент
	fmt.Println("Производитель добавляет 'task1'")
	rdb.RPush(ctx, key, "task1")

	// Даём время на обработку
	time.Sleep(2 * time.Second)

	rdb.Del(ctx, key)
}

// 5. Паттерн: очередь задач (RPUSH + LPOP / BLPOP)
func primer5() {
	fmt.Println("--- 5. Паттерн: очередь задач ---")
	queueKey := "tasks"
	rdb.Del(ctx, queueKey)

	// Добавляем задачи в очередь
	for i := 1; i <= 5; i++ {
		rdb.RPush(ctx, queueKey, fmt.Sprintf("task%d", i))
	}
	fmt.Println("Добавлены задачи")

	for {
		val, err := rdb.LPop(ctx, queueKey).Result()
		if errors.Is(err, redis.Nil) {
			fmt.Println("Очередь пуста, завершаем")
			break
		} else if err != nil {
			panic(err)
		}
		fmt.Printf("Обработка: %s\n", val)
	}

	// Теперь используем BLPOP в реальном сценарии (имитация ожидания)
	go func() {
		// Блокируем на 3 секунды, ждём новую задачу
		res, _ := rdb.BLPop(ctx, 3*time.Second, queueKey).Result()
		if len(res) > 0 {
			fmt.Printf("Доставлена новая задача: %s\n", res[1])
		} else {
			fmt.Println("Новая задача не появилась за 3 секунды")
		}
	}()

	time.Sleep(1 * time.Second)
	rdb.RPush(ctx, queueKey, "task4")
	time.Sleep(2 * time.Second)

	rdb.Del(ctx, queueKey)
}

// 6. Паттерн: стек (LPUSH + LPOP)
func primer6() {
	fmt.Println("--- 6. Паттерн: стек (LIFO) ---")
	key := "stack"
	rdb.Del(ctx, key)

	// Кладём элементы в стек (в начало)
	rdb.LPush(ctx, key, "first")
	rdb.LPush(ctx, key, "second")
	rdb.LPush(ctx, key, "third")

	// Извлекаем LIFO — должны получить third, second, first
	for {
		val, err := rdb.LPop(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			break
		}
		fmt.Printf("Извлечено: %s\n", val)
	}
	rdb.Del(ctx, key)
}

// 7. Ограничение истории через LTRIM (сохраняем последние N элементов)
func primer7() {
	fmt.Println("--- 7. LTRIM для ограничения истории ---")
	key := "history:user:42"
	rdb.Del(ctx, key)

	// Добавляем 10 действий
	for i := 1; i <= 10; i++ {
		rdb.LPush(ctx, key, fmt.Sprintf("action%d", i))
	}
	fmt.Println("Добавлено 10 действий")

	// Оставляем только последние 5 (самые свежие) — с начала списка
	rdb.LTrim(ctx, key, 0, 4)
	remaining, _ := rdb.LRange(ctx, key, 0, -1).Result()
	fmt.Printf("После LTRIM (последние 5): %v\n", remaining)

	rdb.Del(ctx, key)
}

// 8. BRPOP с таймаутом и обработка
func primer8() {
	fmt.Println("--- 8. BRPOP с таймаутом ---")
	key := "mailbox"
	rdb.Del(ctx, key)

	// Ожидаем сообщение, но не дожидаемся (таймаут 2 секунды)
	start := time.Now()
	result, err := rdb.BRPop(ctx, 2*time.Second, key).Result()
	if err != nil {
		fmt.Println("Ошибка:", err)
	} else if len(result) == 0 {
		fmt.Printf("Таймаут, прошло %v\n", time.Since(start))
	} else {
		fmt.Printf("Получено: %s\n", result[1])
	}

	// Теперь добавляем сообщение и ждём с таймаутом 5 секунд
	go func() {
		time.Sleep(1 * time.Second)
		rdb.RPush(ctx, key, "hello")
	}()
	result, _ = rdb.BRPop(ctx, 5*time.Second, key).Result()
	if len(result) > 0 {
		fmt.Printf("Успешно получено: %s (за %v)\n", result[1], time.Since(start))
	}
	rdb.Del(ctx, key)
}
