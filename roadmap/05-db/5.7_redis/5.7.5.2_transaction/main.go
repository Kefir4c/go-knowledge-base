package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 5.2. ТРАНЗАКЦИИ (MULTI / EXEC) — РАСШИРЕННАЯ ТЕОРИЯ
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖНЫ ТРАНЗАКЦИИ?

В распределённых системах часто требуется выполнить несколько операций
как единое целое, чтобы данные оставались согласованными. Например:
- Перевод денег между счетами (дебет одного, кредит другого).
- Резервирование товара и уменьшение остатка.
- Обновление нескольких связанных кэшей.

Redis предоставляет механизм транзакций, который позволяет группировать команды
и выполнять их атомарно и изолированно. Это гарантирует, что либо все команды
будут выполнены, либо ни одна (с учётом особенностей Redis).

1. АТОМАРНОСТЬ: ЧТО ЭТО ЗНАЧИТ В REDIS?

В классических реляционных БД (PostgreSQL, MySQL) транзакции обладают свойством
ACID (Atomicity, Consistency, Isolation, Durability). Redis поддерживает ACID
с некоторыми ограничениями:

- Atomicity: Да, но с оговоркой. Если внутри транзакции возникает ошибка
  (например, синтаксическая), EXEC не выполняется. Если ошибка возникает
  во время выполнения отдельных команд (после EXEC), другие команды продолжают
  выполняться, и транзакция не откатывается. Это поведение отличается от
  реляционных БД, где при ошибке обычно откатываются все изменения.

- Consistency: Redis обеспечивает согласованность данных на уровне своих структур,
  но не проверяет бизнес-логику (например, баланс не может быть отрицательным).
  Это должна делать логика приложения.

- Isolation: Транзакции Redis полностью изолированы — команды из разных клиентов
  не перемешиваются внутри одной транзакции.

- Durability: Redis не гарантирует долговечность данных, если не включён AOF
  с соответствующей настройкой. По умолчанию данные могут быть потеряны
  при падении сервера.

2. ВНУТРЕННЕЕ УСТРОЙСТВО ТРАНЗАКЦИЙ

2.1. Очередь команд (queue):
    - Когда вызывается MULTI, Redis переключает клиента в "режим транзакции".
    - Все последующие команды (кроме EXEC, DISCARD, WATCH, UNWATCH) не выполняются
      сразу, а помещаются в очередь команд.
    - Каждая команда в очереди сохраняется с её аргументами.
    - Ответом на каждую команду является "QUEUED".

2.2. Выполнение транзакции (EXEC):
    - При вызове EXEC Redis выполняет все команды из очереди последовательно.
    - Выполнение происходит атомарно: другие клиенты не могут вклиниться.
    - Результаты всех команд собираются в массив и возвращаются клиенту.

2.3. Отмена транзакции (DISCARD):
    - Очищает очередь команд и выходит из режима транзакции.
    - Все WATCH снимаются.

2.4. Оптимистическая блокировка (WATCH/UNWATCH):
    - WATCH отслеживает изменения ключей (включая истечение TTL).
    - Если после WATCH и до EXEC один из отслеживаемых ключей был изменён
      (SET, DEL, INCR, EXPIRE и т.д.) другим клиентом, EXEC возвращает nil.
    - При этом очередь команд, накопленная после MULTI, всё равно существует,
      но EXEC не выполняется.
    - UNWATCH снимает мониторинг со всех ключей.

3. ОШИБКИ В ТРАНЗАКЦИЯХ (ПОДРОБНО)

3.1. Ошибки до EXEC (синтаксические, неверные аргументы):
    - Если в MULTI после добавления команды возникает синтаксическая ошибка
      (например, "SET key" без значения), Redis не позволяет вызвать EXEC.
    - Вызов EXEC приведёт к ошибке, и транзакция будет отменена.
    - DISCARD может быть вызван для очистки очереди.

3.2. Ошибки во время выполнения (после EXEC):
    - Например, INCR на строке или LPOP на не-списке.
    - Redis продолжает выполнять остальные команды.
    - Ошибка возвращается в массиве ответов для конкретной команды.
    - Это поведение часто неожиданно для разработчиков: "Атомарность" не означает
      "всё или ничего" в смысле отката при ошибках выполнения.

3.3. Ошибки WATCH (конфликт изменения ключа):
    - EXEC возвращает nil (пустой ответ, не ошибка).
    - В go-redis это ошибка redis.TxFailedErr.
    - Транзакция не выполняется, и никакие изменения не применяются.
    - Клиент должен повторить всю логику.

4. ИЗОЛЯЦИЯ И БЛОКИРОВКИ

4.1. Уровень изоляции:
    - Redis обеспечивает изоляцию "сериализуемость" (serializable): транзакции
      выполняются последовательно, без пересечений.
    - Другие клиенты не могут вклиниться между командами транзакции.

4.2. Влияние на производительность:
    - Транзакции могут блокировать другие операции, если они содержат много команд
      или медленные операции (LRANGE большого списка).
    - Рекомендуется делать транзакции короткими и быстрыми.

5. СРАВНЕНИЕ С ДРУГИМИ МЕХАНИЗМАМИ

5.1. Транзакции vs Пайплайн:
    - Пайплайн не атомарен и не изолирован.
    - Транзакции дают атомарность и изоляцию, но с ограничениями по откату.

5.2. Транзакции vs Lua:
    - Lua скрипты выполняются атомарно и могут содержать условную логику.
    - Lua позволяет обрабатывать ошибки внутри скрипта (возвращать ошибку).
    - Lua в целом мощнее и часто предпочтительнее для сложных операций.
    - Однако транзакции более просты для случая простых обновлений.

5.3. Транзакции vs Redis Streams:
    - Streams используются для обработки событий, а не для атомарных обновлений.

6. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

1. Всегда используйте WATCH для защиты от конкурентных изменений.
2. Обрабатывайте redis.TxFailedErr и повторяйте операцию (обычно в цикле).
3. Не делайте слишком большие транзакции (больше 100 команд).
4. Используйте хеш-теги в кластере для группировки ключей в одном слоте.
5. Для сложной логики (условия, циклы) используйте Lua-скрипты вместо транзакций.
6. Всегда проверяйте ошибки после EXEC для каждой команды.
7. Используйте таймауты контекста для предотвращения блокировок.
8. Не смешивайте блокирующие команды (BLPOP, BRPOP) с транзакциями.

7. ЧАСТЫЕ ОШИБКИ ПРИ ИСПОЛЬЗОВАНИИ ТРАНЗАКЦИЙ

 "Если ошибка в транзакции, все откатывается" — НЕТ! Только синтаксические ошибки до EXEC.
 "WATCH защищает от любых изменений" — НЕТ! Только изменения, выполненные другими клиентами.
 "Транзакции работают в кластере как в standalone" — НЕТ! Только в пределах одного слота.
 "Можно использовать WATCH после MULTI" — НЕТ! WATCH должен быть до MULTI.

8. ИТОГ: КОГДА ИСПОЛЬЗОВАТЬ ТРАНЗАКЦИИ

 Когда нужно атомарно обновить несколько ключей.
 Когда нужна изоляция от других клиентов.
 Когда операция простая (без сложной логики).
 Когда вы хотите использовать WATCH для оптимистической блокировки.

 Когда нужен откат при ошибках выполнения — используйте Lua.
 Когда операция затрагивает ключи из разных слотов в кластере.
 Когда транзакция содержит много команд (> 100) или блокирующие операции.
*/

// Глобальный клиент (для простоты, в реальном проекте передаётся через DI)
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
	fmt.Println("=== ТРАНЗАКЦИИ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. CAS (Compare-And-Set) с повторными попытками и backoff
func primer1() {
	fmt.Println("--- 1. CAS с ретраями и экспоненциальной задержкой ---")

	key := "cas:counter"
	rdb.Set(ctx, key, 0, 0)

	type CASOperation struct {
		client      *redis.Client
		key         string
		maxRetries  int
		baseBackoff time.Duration
	}

	op := &CASOperation{
		client:      rdb,
		key:         key,
		maxRetries:  5,
		baseBackoff: 50 * time.Millisecond,
	}

	// Функция атомарного обновления
	update := func(newValue int) error {
		var lastErr error
		for attempt := 0; attempt < op.maxRetries; attempt++ {
			err := op.client.Watch(ctx, func(tx *redis.Tx) error {
				// Читаем текущее значение
				current, err := tx.Get(ctx, key).Int64()
				if err != nil && err != redis.Nil {
					return err
				}
				// Проверяем условие (например, новое значение должно быть больше)
				if current >= int64(newValue) {
					return nil // ничего не делаем
				}
				// Выполняем транзакцию
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.Set(ctx, op.key, newValue, 0)
					return nil
				})
				return err
			}, op.key)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				// Конфликт, делаем backoff и повторяем
				backoff := op.baseBackoff * time.Duration(math.Pow(1<<attempt))
				if backoff > 2*time.Second {
					backoff = 2 * time.Second
				}
				logger.Printf("CAS конфликт, попытка %d, backoff %v", attempt+1, backoff)
				time.Sleep(backoff)
				lastErr = err
				continue
			}
			return err
		}
		return fmt.Errorf("исчерпаны попытки (%d): %w", op.maxRetries, lastErr)
	}
	// Тестируем с конкурентными изменениями
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			if err := update(val); err != nil {
				logger.Printf("Ошибка обновления на %d: %v", val, err)
			} else {
				logger.Printf("Обновление на %d успешно", val)
			}
		}(i*10 + 5)
	}
	wg.Wait()

	final, _ := rdb.Get(ctx, key).Int64()
	fmt.Printf("Финальное значение: %d\n", final)
	rdb.Del(ctx, key)
}

// 2. ПЕРЕВОД СРЕДСТВ МЕЖДУ СЧЁТАМИ (КЛАССИЧЕСКИЙ КЕЙС)
func primer2() {
	fmt.Println("\n--- 2. Перевод средств между счетами ---")

	type Account struct {
		ID      string
		Balance int64
	}

	from := &Account{ID: "A1", Balance: 100}
	to := &Account{ID: "A2", Balance: 50}

	rdb.Set(ctx, "account:"+from.ID, from.Balance, 0)
	rdb.Set(ctx, "account:"+to.ID, to.Balance, 0)

	transfer := func(fromID, toID string, amount int64) error {
		for {
			err := rdb.Watch(ctx, func(tx *redis.Tx) error {
				// Получаем текущие балансы
				fromBal, err := tx.Get(ctx, "account:"+fromID).Int64()
				if err != nil {
					return err
				}
				toBal, err := tx.Get(ctx, "account:"+toID).Int64()
				if err != nil {
					return err
				}
				if fromBal < amount {
					return fmt.Errorf("недостаточно средств: %d < %d", fromBal, amount)
				}
				// Выполняем транзакцию
				_, err = tx.TxPipelined(ctx, func(pipeliner redis.Pipeliner) error {
					pipeliner.Set(ctx, "account:"+fromID, fromBal-amount, 0)
					pipeliner.Set(ctx, "account:"+toID, toBal+amount, 0)
					// Логируем операцию
					pipeliner.LPush(ctx, "transfer:log", fmt.Sprintf("%s -> %s: %d", fromID, toID, amount))
					return nil
				})
				return err
			}, "account:"+fromID, "account:"+toID)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				// Кто-то изменил балансы, повторяем
				continue
			}
			return err
		}
	}
	err := transfer(from.ID, to.ID, 30)
	if err != nil {
		fmt.Printf("шибка перевода: %v\n", err)
	} else {
		fmt.Println("Перевод 30 успешен")
	}
	// Проверяем
	newFrom, _ := rdb.Get(ctx, "account:"+from.ID).Int64()
	newTo, _ := rdb.Get(ctx, "account:"+to.ID).Int64()
	fmt.Printf("from: %d, to: %d\n", newFrom, newTo)
	rdb.Del(ctx, "account:"+from.ID, "account:"+to.ID, "transfers:log")
}

// 3. РЕЗЕРВИРОВАНИЕ ТОВАРА (С ОСТАТКАМИ И ВОЗВРАТОМ)
func primer3() {
	fmt.Println("\n--- 3. Резервирование товара (инвентаризация) ---")

	stockKey := "inventory:product:123"
	reservedKey := "inventory:product:123:reserved"
	rdb.Set(ctx, stockKey, 10, 0)
	rdb.Set(ctx, reservedKey, 0, 0)

	reserve := func(qty int) error {
		for {
			err := rdb.Watch(ctx, func(tx *redis.Tx) error {
				stock, err := tx.Get(ctx, stockKey).Int64()
				if err != nil {
					return err
				}
				reserved, err := tx.Get(ctx, reservedKey).Int64()
				if err != nil && err != redis.Nil {
					return err
				}
				if stock-reserved < int64(qty) {
					return fmt.Errorf("недостаточно свободного товара: %d", stock-reserved)
				}
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.IncrBy(ctx, reservedKey, int64(qty))
					return nil
				})
				return err
			}, stockKey, reservedKey)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				continue
			}
			return err
		}
	}

	release := func(qty int) error {
		for {
			err := rdb.Watch(ctx, func(tx *redis.Tx) error {
				reserved, err := tx.Get(ctx, reservedKey).Int64()
				if err != nil {
					return err
				}
				if reserved < int64(qty) {
					return fmt.Errorf("нечего освобождать")
				}
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.IncrBy(ctx, reservedKey, -int64(qty))
					return nil
				})
				return err
			}, reservedKey)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				continue
			}
			return err
		}
	}

	// Резервируем 3 товара
	err := reserve(3)
	if err != nil {
		fmt.Printf("Ошибка резервирования: %v\n", err)
	} else {
		fmt.Println("Зарезервировано 3 товара")
	}

	// Освобождаем 1
	err = release(1)
	if err != nil {
		fmt.Printf("Ошибка освобождения: %v\n", err)
	} else {
		fmt.Println("Освобождено 1 товар")
	}

	// Проверяем состояние
	stock, _ := rdb.Get(ctx, stockKey).Int64()
	reserved, _ := rdb.Get(ctx, reservedKey).Int64()
	fmt.Printf("stock=%d, reserved=%d, available=%d\n", stock, reserved, stock-reserved)
	rdb.Del(ctx, stockKey, reservedKey)
}

// 4. ПАКЕТНОЕ ОБНОВЛЕНИЕ С ВАЛИДАЦИЕЙ (WATCH + MULTI)
func primer4() {
	fmt.Println("\n--- 4. Пакетное обновление с проверкой условий ---")

	keys := []string{"user:1:score", "user:2:score", "user:3:score"}
	for _, k := range keys {
		rdb.Set(ctx, k, 0, 0)
	}

	updateScores := func(scores map[string]int) error {
		for {
			err := rdb.Watch(ctx, func(tx *redis.Tx) error {
				// Проверяем, что все ключи существуют
				for _, key := range keys {
					exists, err := tx.Exists(ctx, key).Result()
					if err != nil {
						return err
					}
					if exists == 0 {
						return fmt.Errorf("ключ %s не существует", key)
					}
				}
				// Выполняем обновление
				_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					for k, v := range scores {
						pipe.Set(ctx, k, v, 0)
					}
					return nil
				})
				return err
			}, keys...)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				continue
			}
			return err
		}
	}
	scores := map[string]int{
		"user:1:score": 10,
		"user:2:score": 20,
		"user:3:score": 15,
	}
	err := updateScores(scores)
	if err != nil {
		fmt.Printf("Ошибка обновления: %v\n", err)
	} else {
		fmt.Println("Очки обновлены")
	}
	for _, k := range keys {
		val, _ := rdb.Get(ctx, k).Result()
		fmt.Printf("%s: %s\n", k, val)
	}
	rdb.Del(ctx, keys...)
}

// 5. КЛАСТЕРНАЯ ТРАНЗАКЦИЯ С ХЕШ-ТЕГАМИ
func primer5() {
	fmt.Println("\n--- 5. Транзакция в кластере (хеш-теги) ---")

	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
	})
	defer clusterClient.Close()

	// Используем хеш-тег для гарантии одного слота
	prefix := "{user:123}"
	key1 := prefix + ".balance"
	key2 := prefix + ".history"

	// Инициализируем
	clusterClient.Set(ctx, key1, 100, 0)
	clusterClient.Del(ctx, key2)

	// Транзакция: списание со счета + запись в историю
	err := clusterClient.Watch(ctx, func(tx *redis.Tx) error {
		balance, err := tx.Get(ctx, key1).Int64()
		if err != nil {
			return err
		}
		if balance < 10 {
			return errors.New("недостаточно средств")
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key1, balance-10, 0)
			pipe.LPush(ctx, key2, time.Now().String())
			return nil
		})
		return err
	}, key1, key2)

	if err != nil {
		fmt.Printf("Ошибка транзакции в кластере: %v\n", err)
	} else {
		fmt.Println("Транзакция в кластере выполнена")
	}
	clusterClient.Del(ctx, key1, key2)
}

// 6. WATCH С ТАЙМАУТОМ (ИСПОЛЬЗОВАНИЕ КОНТЕКСТА)
func primer6() {
	fmt.Println("\n--- 6. WATCH с таймаутом через контекст ---")

	key := "timeout:test"
	rdb.Set(ctx, key, 0, 0)

	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// Имитируем долгую операцию, которая может занять >1 сек
	err := rdb.Watch(ctxTimeout, func(tx *redis.Tx) error {
		// Имитация задержки (например, обращение к внешнему сервису)
		time.Sleep(2 * time.Second)

		val, err := tx.Get(ctxTimeout, key).Int64()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctxTimeout, func(pipe redis.Pipeliner) error {
			pipe.Set(ctxTimeout, key, val+1, 0)
			return nil
		})
		return err
	}, key)

	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("Транзакция прервана по таймауту")
	} else if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		fmt.Println("Транзакция выполнена")
	}
	rdb.Del(ctx, key)
}

// 7. ОПТИМИСТИЧЕСКАЯ БЛОКИРОВКА СЧЁТЧИКА (GO-КОНКУРЕНТНОСТЬ)
func primer7() {
	fmt.Println("\n--- 7. Оптимистическая блокировка счётчика (100 горутин) ---")

	key := "counter:optimistic"
	rdb.Set(ctx, key, 0, 0)

	var wg sync.WaitGroup
	incr := func(amount int) {
		defer wg.Done()
		for {
			err := rdb.Watch(ctx, func(tx *redis.Tx) error {
				val, err := tx.Get(ctx, key).Int64()
				if err != nil {
					return err
				}
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.Set(ctx, key, val+int64(amount), 0)
					return nil
				})
				return err
			}, key)
			if err == nil {
				return
			}
			if errors.Is(err, redis.TxFailedErr) {
				// Повторяем
				continue
			}
			logger.Printf("Ошибка: %v", err)
			return
		}
	}
	// Запускаем 100 горутин с инкрементом на 1
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go incr(1)
	}
	wg.Wait()

	final, _ := rdb.Get(ctx, key).Int64()
	fmt.Printf("Финальный счётчик: %d (ожидалось 100)\n", final)
	rdb.Del(ctx, key)
}

// 8. ТРАНЗАКЦИЯ С ОЧИСТКОЙ ПРИ КОНФЛИКТЕ (ОТМЕНА ИЗМЕНЕНИЙ)
func primer8() {
	fmt.Println("\n--- 8. Транзакция с очисткой при конфликте ---")

	key := "resource:state"
	rdb.Set(ctx, key, "initial", 0)

	// Функция, которая пытается обновить состояние, но если конфликт,
	// откатывает изменения (удаляет промежуточные ключи)
	err := rdb.Watch(ctx, func(tx *redis.Tx) error {
		// Проверяем текущее состояние
		state, err := tx.Get(ctx, key).Result()
		if err != nil {
			return err
		}
		if state != "initial" {
			return fmt.Errorf("неожиданное состояние: %s", state)
		}
		// Выполняем сложное обновление (несколько ключей)
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, "processing", 0)
			pipe.Set(ctx, "backup:state", state, 0)
			pipe.Set(ctx, "lock:processing", "true", 0)
			return nil
		})
		if err != nil {
			return err
		}
		// Имитация успеха или ошибки (например, проверка внешнего условия)
		// Если ошибка, мы не делаем откат автоматически — нужно явно.
		return nil
	}, key)
	if err != nil {
		fmt.Printf("Транзакция не удалась: %v\n", err)
		// Если ошибка не из-за конфликта, можно вручную откатить
		if !errors.Is(err, redis.TxFailedErr) {
			// Ручной откат: удаляем промежуточные ключи
			rdb.Del(ctx, "backup:state", "lock:processing")
			fmt.Println("Произведён ручной откат изменений")
		}
	} else {
		fmt.Println("Транзакция выполнена успешно")
	}
	rdb.Del(ctx, key, "backup:state", "lock:processing")
}
