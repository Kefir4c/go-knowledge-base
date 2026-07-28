package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 4. МНОЖЕСТВА (SET) В REDIS

1. ЧТО ТАКОЕ МНОЖЕСТВО?
   - Множество (Set) — это неупорядоченная коллекция уникальных строк.
   - Элементы не повторяются, порядок не гарантируется.
   - Операции добавления, удаления, проверки существования выполняются за O(1).
   - Максимальное количество элементов: 2³² - 1 (более 4 миллиардов).

2. ГЛАВНОЕ ПРИМЕНЕНИЕ:
   - Хранение уникальных идентификаторов (например, ID пользователей, IP-адресов).
   - Теги / категории для объектов (например, теги товаров).
   - Проверка принадлежности (например, является ли пользователь подписчиком).
   - Пересечения и объединения (например, общие друзья, рекомендации).
   - Удаление дубликатов из потока данных.
   - Реализация систем голосования (лайки/дизлайки).

3. ОСНОВНЫЕ КОМАНДЫ И ИХ ДЕЙСТВИЕ (ПОДРОБНО)
   -------------------------------------------------------------------------------
   SADD key member [member ...]
   - Добавляет один или несколько элементов в множество.
   - Если элемент уже существует, игнорируется.
   - Возвращает количество добавленных элементов (новых).
   - CLI: SADD tags "golang" "redis"
   - Go: added, err := rdb.SAdd(ctx, "tags", "golang", "redis").Result()

   SREM key member [member ...]
   - Удаляет один или несколько элементов из множества.
   - Возвращает количество удалённых элементов.
   - CLI: SREM tags "redis"
   - Go: removed, err := rdb.SRem(ctx, "tags", "redis").Result()

   SMEMBERS key
   - Возвращает все элементы множества (как слайс строк).
   - Для больших множеств может быть дорого, используйте с осторожностью.
   - CLI: SMEMBERS tags
   - Go: members, err := rdb.SMembers(ctx, "tags").Result() // []string

   SISMEMBER key member
   - Проверяет, является ли элемент членом множества.
   - Возвращает true/false.
   - CLI: SISMEMBER tags "golang" → 1
   - Go: exists, err := rdb.SIsMember(ctx, "tags", "golang").Result() // bool

   SCARD key
   - Возвращает количество элементов в множестве (cardinality).
   - CLI: SCARD tags → 2
   - Go: count, err := rdb.SCard(ctx, "tags").Result() // int64

   SUNION key [key ...]
   - Возвращает объединение нескольких множеств (все элементы из всех множеств).
   - CLI: SUNION set1 set2
   - Go: union, err := rdb.SUnion(ctx, "set1", "set2").Result() // []string

   SINTER key [key ...]
   - Возвращает пересечение множеств (элементы, присутствующие во всех).
   - CLI: SINTER set1 set2
   - Go: inter, err := rdb.SInter(ctx, "set1", "set2").Result() // []string

   SDIFF key [key ...]
   - Возвращает разность множеств (элементы первого множества, отсутствующие в остальных).
   - CLI: SDIFF set1 set2
   - Go: diff, err := rdb.SDiff(ctx, "set1", "set2").Result() // []string

   Дополнительные команды (не в основном списке, но полезны):
   - SPOP key [count] — удаляет и возвращает случайный элемент (или несколько).
   - SRANDMEMBER key [count] — возвращает случайный элемент без удаления.
   - SMOVE source destination member — перемещает элемент между множествами.
   - SSCAN — итерация по элементам множества (безопасно для больших множеств).

4. ВАЖНЫЕ СВОЙСТВА И ПРАВИЛА:
   - Все операции с множествами (кроме SMEMBERS) O(1).
   - Элементы уникальны — повторное добавление игнорируется.
   - Неупорядоченность — нельзя полагаться на порядок элементов.
   - Мощные операции объединения/пересечения/разности могут быть затратны для больших множеств.

5. КОГДА НЕ ИСПОЛЬЗОВАТЬ МНОЖЕСТВА:
   - Если нужен упорядоченный список → List или Sorted Set.
   - Если нужна сортировка по числовому значению → Sorted Set.
   - Если элементы должны храниться с полями → Hash.

6. СВЯЗЬ С GO (go-redis/v9):
   - SAdd(ctx, key, members...) — *IntCmd
   - SRem(ctx, key, members...) — *IntCmd
   - SMembers(ctx, key) — *StringSliceCmd
   - SIsMember(ctx, key, member) — *BoolCmd
   - SCard(ctx, key) — *IntCmd
   - SUnion(ctx, keys...) — *StringSliceCmd
   - SInter(ctx, keys...) — *StringSliceCmd
   - SDiff(ctx, keys...) — *StringSliceCmd

7. ТИПИЧНЫЕ ОШИБКИ:
   - SMembers на огромных множествах может вызвать задержки (лучше использовать SSCAN для итерации).
   - Попытка выполнить SINTER/SUNION/SDIFF с несуществующими ключами — они интерпретируются как пустые множества.

8. ПАТТЕРНЫ ИСПОЛЬЗОВАНИЯ:
   - Теги: SADD article:123 "golang" "redis"; SMEMBERS article:123 → список тегов.
   - Подписки: SADD subscribers:user:456 "user:789" → подписчики.
   - Рекомендации: SINTER интересов пользователей для поиска общих интересов.
   - Черные списки: SADD blacklist "ip1" "ip2"; SISMEMBER blacklist ip1 → проверка.
   - Уникальные посетители за день: SADD visitors:2024-01-01 userID.
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
	fmt.Println("ПРАКТИКА ПО МНОЖЕСТВАМ (SET)")
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. SADD и SMEMBERS
func primer1() {
	fmt.Println("--- 1. SADD, SMEMBERS ---")
	key := "tags"
	rdb.Del(ctx, key)

	// Добавляем элементы
	added, _ := rdb.SAdd(ctx, key, "golang", "redis", "docker", "golang").Result()
	fmt.Printf("SADD: добавлено %d элементов (повторные игнорируются)\n", added)

	// Получаем все элементы
	members, _ := rdb.SMembers(ctx, key).Result()
	fmt.Printf("SMEMBERS: %v (порядок не гарантирован)\n", members)

	rdb.Del(ctx, key)
}

// 2. SREM и SISMEMBER
func primer2() {
	fmt.Println("--- 2. SREM, SISMEMBER ---")
	key := "admins"
	rdb.Del(ctx, key)
	rdb.SAdd(ctx, key, "alice", "bob", "charlie")

	// Проверяем наличие
	exists, _ := rdb.SIsMember(ctx, key, "alice").Result()
	fmt.Printf("SISMEMBER alice -> %v\n", exists)

	// Удаляем элемент
	removed, _ := rdb.SRem(ctx, key, "bob").Result()
	fmt.Printf("SREM bob -> удалено %d элементов\n", removed)

	// Проверяем снова
	exists, _ = rdb.SIsMember(ctx, key, "bob").Result()
	fmt.Printf("SISMEMBER bob после удаления -> %v\n", exists)

	rdb.Del(ctx, key)
}

// 3. SCARD — количество элементов
func primer3() {
	fmt.Println("--- 3. SCARD ---")
	key := "set"
	rdb.Del(ctx, key)
	rdb.SAdd(ctx, key, "a", "b", "c")

	count, _ := rdb.SCard(ctx, key).Result()
	fmt.Printf("SCARD: %d элементов\n", count)

	rdb.Del(ctx, key)
}

// 4. SUNION — объединение множеств
func primer4() {
	fmt.Println("--- 4. SUNION: рекомендации по интересам ---")

	// Статья 1: теги
	rdb.SAdd(ctx, "article:1:tags", "golang", "redis", "docker")
	// Статья 2: теги
	rdb.SAdd(ctx, "article:2:tags", "golang", "kubernetes", "redis")

	// Получаем все теги, которые есть у пользователя в интересах
	// (можно закешировать, но здесь просто для примера)
	// Для каждой статьи проверяем пересечение с интересами пользователя
	for _, article := range []string{"article:1", "article:2"} {
		// SINTER с интересами
		common, _ := rdb.SInter(ctx, article+":tags", "user:1:interests").Result()
		if len(common) > 0 {
			fmt.Printf("Статья %s рекомендуется (общие теги: %v)\n", article, common)
		}
	}
	// А вообще SUNION используется для объединения тегов нескольких статей
	// Например, для поискового индекса
	allTags, _ := rdb.SUnion(ctx, "article:1:tags", "article:2:tags").Result()
	fmt.Printf("Все уникальные теги в библиотеке: %v\n", allTags)

	rdb.Del(ctx, "article:1:tags", "article:2:tags", "user:1:interests")
}

// 5. SINTER — поиск общих подписчиков (соцсеть)
func primer5() {
	fmt.Println("--- 5. SINTER: общие подписчики (соцсеть) ---")

	// Пользователи и их подписчики
	rdb.SAdd(ctx, "user:alice:followers", "bob", "charlie", "dave", "eve")
	rdb.SAdd(ctx, "user:bob:followers", "alice", "charlie", "eve")

	// Найти общих подписчиков у Alice и Bob
	commonFollowers, _ := rdb.SInter(ctx, "user:alice:followers", "user:bob:followers").Result()
	fmt.Printf("Общие подписчики у Alice и Bob: %v\n", commonFollowers)

	// Рекомендовать пользователям, на которых подписан Alice, но не подписан Bob
	diff, _ := rdb.SDiff(ctx, "user:alice:followers", "user:bob:followers").Result()
	fmt.Printf("Alice подписана на %v, а Bob — нет\n", diff) // [dave]
}

// 6. SDIFF — разность множеств
func primer6() {
	// Все IP-адреса, которые заходили сегодня
	rdb.SAdd(ctx, "ips:today", "192.168.1.1", "192.168.1.2", "192.168.1.3", "10.0.0.1")
	// Чёрный список
	rdb.SAdd(ctx, "ips:blacklist", "192.168.1.2", "10.0.0.1")

	// Найти чистые IP (которые в today, но не в blacklist)
	cleanIPs, _ := rdb.SDiff(ctx, "ips:today", "ips:blacklist").Result()
	fmt.Printf("Чистые IP-адреса: %v\n", cleanIPs)

	rdb.Del(ctx, "ips:today", "ips:blacklist")
}

// 7. SADD + SCARD + SISMEMBER — система лайков
func primer7() {
	fmt.Println("--- 7. SADD + SCARD + SISMEMBER: лайки постов ---")
	postKey := "post:42:likes"

	// Пользователи лайкают пост
	rdb.SAdd(ctx, postKey, "user1", "user2", "user3", "user2") // дубли игнорируются

	// Количество лайков
	likes, _ := rdb.SCard(ctx, postKey).Result()
	fmt.Printf("У поста %d лайков\n", likes) // 3

	// Проверяем, лайкнул ли конкретный пользователь
	liked, _ := rdb.SIsMember(ctx, postKey, "user1").Result()
	fmt.Printf("user1 лайкнул? %v\n", liked)

	// Убираем лайк
	rdb.SRem(ctx, postKey, "user3")
	likes, _ = rdb.SCard(ctx, postKey).Result()
	fmt.Printf("У поста %d лайков\n", likes) // 2
}

// 8. Использование SPOP — случайный выбор (розыгрыш)
func primer8() {
	fmt.Println("--- 8. SPOP: случайный выбор победителя ---")

	participantsKey := "contest:participants"
	rdb.SAdd(ctx, participantsKey, "user1", "user2", "user3", "user4", "user5")

	// Выбираем случайного победителя (удаляем из множества)
	winner, err := rdb.SPop(ctx, participantsKey).Result()
	if err == nil {
		fmt.Printf("Победитель: %s\n", winner)
	}

	// Сколько осталось участников после удаления победителя
	remaining, _ := rdb.SCard(ctx, participantsKey).Result()
	fmt.Printf("Осталось участников: %d\n", remaining)
}
