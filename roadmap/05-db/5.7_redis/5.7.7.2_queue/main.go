package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 7.2. ОЧЕРЕДИ ЗАДАЧ
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖНЫ ОЧЕРЕДИ В REDIS?

Очереди задач — один из самых частых сценариев использования Redis.
Они позволяют асинхронно обрабатывать задачи, разгружая основные компоненты
системы, сглаживая пиковые нагрузки и обеспечивая отказоустойчивость.

Redis предлагает два основных механизма для построения очередей:
1. Списки (Lists) + BLPOP / BRPOP — простой, быстрый, но с ограничениями.
2. Streams (Redis 5.0+) — мощный, надёжный, с поддержкой групп потребителей.

Выбор между ними определяет архитектуру вашей системы.

1. ОЧЕРЕДИ НА СПИСКАХ (LISTS + BLPOP)

1.1. Базовый механизм
    - Producer: RPUSH key task — добавляет задачу в конец списка.
    - Consumer: LPOP key — забирает задачу с начала (FIFO).
    - Блокирующая версия: BLPOP key timeout — ждёт появления задачи.
    - Стек (LIFO): LPUSH + LPOP или RPUSH + RPOP.

1.2. Преимущества
    + Простота — минимум команд, легко понять.
    + Высокая производительность — операции O(1).
    + Блокировка BLPOP не нагружает CPU (ожидание без активного опроса).
    + Поддержка множества ключей — можно реализовать приоритеты.

1.3. Недостатки и ограничения
    - Нет подтверждения обработки: если воркер упал после получения задачи,
      задача теряется безвозвратно (at-most-once).
    - Нет механизма повторной обработки при сбоях.
    - Нет группировки потребителей — каждый воркер видит всю очередь.
    - Нет идентификаторов сообщений — сложно отслеживать дубликаты.
    - Нет возможности отложить задачу (кроме как через внешний механизм).
    - Нет истории выполненных задач.

1.4. Когда использовать списки
    - Простые очереди с низкими требованиями к надёжности (логи, уведомления).
    - Задачи, потеря которых не критична (можно повторить).
    - Высокая пропускная способность и минимальная задержка важнее гарантий.
    - Нет необходимости в группировке потребителей.

1.5. Паттерны на списках
    - RPOPLPUSH / BRPOPLPUSH — надёжная очередь с промежуточным списком
      (processing list). Задача перемещается в processing, и только после
      успешной обработки удаляется оттуда. При падении воркера задача
      остаётся в processing и может быть возвращена в основную очередь.
    - Приоритеты через несколько списков + BLPOP на них.
    - Ограничение размера очереди через LTRIM.

2. ОЧЕРЕДИ НА STREAMS (REDIS 5.0+)

2.1. Что такое Streams?
    - Это структура данных для ведения журналов (логов) событий.
    - Каждое сообщение имеет уникальный ID (обычно временная метка).
    - Сообщения хранятся в хронологическом порядке.
    - Поддержка групп потребителей для балансировки нагрузки.

2.2. Базовые команды
    - XADD stream * field value [field value ...] — добавляет сообщение.
      * — автоматический ID (временная метка + порядковый номер).
    - XREAD [BLOCK ms] [COUNT n] STREAMS stream [ID] — читает сообщения.
      ID = "0-0" — с начала, ">" — только новые для группы.
    - XREADGROUP GROUP group consumer STREAMS stream ">" — чтение в группе.
    - XACK stream group id — подтверждение обработки сообщения.
    - XPENDING stream group — список неподтверждённых сообщений.
    - XCLAIM stream group consumer min-idle-time ids — забирает зависшие.

2.3. Группы потребителей (Consumer Groups)
    - Позволяют нескольким потребителям обрабатывать сообщения параллельно.
    - Каждое сообщение доставляется только одному потребителю в группе.
    - Сообщения можно подтверждать (XACK) после успешной обработки.
    - Если потребитель упал, сообщение остаётся в pending и может быть
      перераспределено (XCLAIM).
    - Потребители могут динамически подключаться и отключаться.

2.4. Преимущества Streams
    + Гарантия доставки: at-least-once (при использовании XACK).
    + Подтверждение обработки — надёжность.
    + Группы потребителей — балансировка нагрузки.
    + Идентификаторы сообщений — отслеживание дубликатов.
    + Возможность повторной обработки (XCLAIM).
    + Отложенные задачи (сохраняем сообщение со временем выполнения).
    + История сообщений (можно вернуться к старым).
    + Поддержка дискового хранения (зависит от настройки).

2.5. Недостатки Streams
    - Более сложная модель — выше порог входа.
    - Меньшая производительность по сравнению со списками (из-за гарантий).
    - Потребление памяти — сообщения хранятся до удаления или усечения.
    - Управление группами требует дополнительных команд.

2.6. Когда использовать Streams
    - Критичные задачи, где потеря недопустима (обработка заказов, платежей).
    - Несколько потребителей одного типа (горизонтальное масштабирование).
    - Требуется повторная обработка при сбоях.
    - Нужна история сообщений для аудита.
    - Системы с высокой нагрузкой, но с требованиями к согласованности.

3. СРАВНЕНИЕ МЕХАНИЗМОВ ОЧЕРЕДЕЙ
┌───────────────────────────┬───────────────────────────┬───────────────────────────┐
│ ХАРАКТЕРИСТИКА            │ СПИСКИ (Lists + BLPOP)    │ STREAMS (Consumer Groups) │
├───────────────────────────┼───────────────────────────┼───────────────────────────┤
│ Простота                  │ Высокая                   │ Средняя / Сложная         │
│ Производительность        │ Очень высокая (O(1))      │ Высокая, но с оверхедом   │
│ Гарантия доставки         │ At-most-once (потеря при  │ At-least-once (с XACK)    │
│                           │ падении воркера)          │                           │
│ Подтверждение обработки   │ Нет                       │ Да (XACK)                 │
│ Группы потребителей       │ Нет (все видят всё)       │ Да (балансировка)         │
│ Идентификаторы сообщений  │ Нет (только значение)     │ Да (уникальный ID)        │
│ Повторная обработка       │ Нет                       │ Да (XCLAIM)               │
│ Отложенные задачи         │ Через внешний ZSET        │ Встроенный механизм с     │
│                           │                           │ временными метками        │
│ История сообщений         │ Нет (только текущие)      │ Да (хранятся до удаления) │
│ Масштабирование           │ Простое (добавление       │ Группы упрощают           │
│                           │ воркеров)                 │ горизонтальное            │
│                           │                           │ масштабирование           │
└───────────────────────────┴───────────────────────────┴───────────────────────────┘

4. ГАРАНТИИ ДОСТАВКИ И ОБРАБОТКИ ОШИБОК

4.1. At-most-once (не более одного раза)
    - Задача может быть потеряна.
    - Используется в списках без подтверждения.
    - Подходит для некритичных данных.

4.2. At-least-once (как минимум один раз)
    - Задача гарантированно будет обработана, но возможно дублирование.
    - Достигается через XACK: задача удаляется только после подтверждения.
    - Если воркер упал до XACK, задача остаётся в pending и будет передана другому.
    - Требует идемпотентности обработчиков.

4.3. Exactly-once (ровно один раз)
    - Redis не предоставляет такой гарантии.
    - Достигается внешними механизмами (идентификаторы, транзакции).

4.4. Обработка ошибок
    - При ошибке в обработке: не делать XACK → задача останется в pending.
    - Другой воркер через XCLAIM заберёт её после таймаута.
    - Важно: использовать мониторинг XPENDING для обнаружения зависших задач.

4.5. Отказоустойчивость
    - Redis репликация и Sentinel/Cluster обеспечивают доступность.
    - При переключении мастера сообщения в памяти могут потеряться (если AOF не включён).
    - Настройте AOF и периодический RDB для защиты.

5. МАСШТАБИРОВАНИЕ ОЧЕРЕДЕЙ

5.1. Вертикальное масштабирование
    - Увеличение памяти и CPU для Redis.
    - Ограничено физическими ресурсами.

5.2. Горизонтальное масштабирование (шардирование)
    - Разделение очередей по логическим группам (например, по типу задачи).
    - Использование Redis Cluster с хеш-тегами.
    - Для Streams: можно создавать отдельные стримы для разных типов задач.

5.3. Множество потребителей
    - Для списков: просто добавляем воркеров.
    - Для Streams: добавляем потребителей в группу — балансировка автоматическая.

5.4. Мониторинг размера очереди
    - LLEN (для списков) или XLEN (для Streams).
    - Используйте экспоненциальный бэкофф при переполнении.

6. ОПТИМИЗАЦИЯ И ПРОИЗВОДИТЕЛЬНОСТЬ

6.1. Размер пачки (COUNT)
    - Для Streams: XREADGROUP с COUNT > 1 уменьшает число запросов.
    - Для списков: LRANGE + LTRIM для пакетной обработки.

6.2. Блокирующие операции
    - BLPOP / XREADGROUP BLOCK — экономят CPU.
    - Используйте разумные таймауты (например, 1-5 секунд).

6.3. Ограничение размера очереди
    - LTRIM (для списков) или XTRIM / MAXLEN (для Streams).
    - Не давайте очереди разрастаться бесконтрольно.

6.4. Использование Pipeline
    - При массовой отправке задач используйте Pipeline.

6.5. Сжатие данных
    - Для больших задач сжимайте payload (gzip, zstd).

7. МОНИТОРИНГ И АЛЕРТЫ

7.1. Метрики для сбора
    - Размер очереди (LLEN / XLEN).
    - Количество pending-сообщений (XPENDING).
    - Задержка обработки (время между XADD и XACK).
    - Количество ошибок при обработке.
    - Использование памяти очередью.

7.2. Алерты
    - Размер очереди > порога (например, 100k задач).
    - Pending-сообщений > 0 (зависшие задачи).
    - Задержка > допустимого времени.

8. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ

1. Для простых задач с потерей допустимой — используйте списки + BLPOP.
2. Для критичных задач — используйте Streams с XACK и XCLAIM.
3. Всегда обрабатывайте ошибки и дублируйте задачи (идемпотентность).
4. Устанавливайте разумный TTL для сообщений (если они не нужны после обработки).
5. Используйте XTRIM или LTRIM для ограничения размера.
6. Мониторьте размер очереди и pending-сообщения.
7. Для отложенных задач используйте Sorted Set + периодический воркер.
8. В Streams используйте группы потребителей для балансировки.
9. Включайте AOF для сохранности данных (но учитывайте производительность).
10. Тестируйте сценарии падения воркеров и переподхвата задач.
*/

var rdb *redis.Client
var ctx = context.Background()
var logger = log.New(os.Stdout, "[QUEUE] ", log.LstdFlags|log.Lshortfile)

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
	fmt.Println("=== ОЧЕРЕДИ ЗАДАЧ: ПРОДАКШН-ПРИМЕРЫ ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. ПРОСТАЯ ОЧЕРЕДЬ НА СПИСКЕ С ВОРКЕР-ПУЛОМ
func primer1() {
	fmt.Println("--- 1. Простая очередь на списке с BLPOP и worker pool ---")

	const (
		queueKey   = "tasks:simple"
		numWorkers = 3
	)

	// Очищаем очередь перед запуском
	rdb.Del(ctx, queueKey)

	// Producer: добавляем задачи (можно запустить в отдельной горутине)
	go func() {
		for i := 1; i <= 21; i++ {
			tasks := fmt.Sprintf(`{"id":%d,"data":"task_%d"}`, i, i)
			err := rdb.RPush(ctx, queueKey, tasks).Err()
			if err != nil {
				logger.Printf("Ошибка добавления задачи: %v", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Consumer: пул воркеров
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	// Обработчик сигналов для graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		logger.Println("Получен сигнал остановки, завершаем воркеров...")
		cancel()
	}()

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					logger.Printf("Воркер %d остановлен", workerID)
					return
				default:
					result, err := rdb.BLPop(ctx, 1*time.Second, queueKey).Result()
					if errors.Is(err, redis.Nil) {
						continue // нет задач
					}
					if err != nil {
						logger.Printf("Воркер %d ошибка: %v", workerID, err)
						continue
					}
					taskJSON := result[1]
					logger.Printf("Воркер %d обрабатывает: %s", workerID, taskJSON)
					// Имитация обработки
					time.Sleep(200 * time.Millisecond)
					// Успешно обработано — не удаляем, т.к. уже удалили через LPop (BLPop удаляет)
					// При необходимости можно добавить логирование.
				}
			}
		}(w)
	}
	wg.Wait()
	logger.Println("Все воркеры завершены")
	rdb.Del(ctx, queueKey)
}

// 2. НАДЁЖНАЯ ОЧЕРЕДЬ С BLMove (ГАРАНТИЯ ОБРАБОТКИ)
func primer2() {
	fmt.Println("\n--- 2. Надёжная очередь с RPOPLPUSH (атомарное перемещение) ---")

	const queueKey = "tasks:reliable"
	const processingKey = "tasks:processing"

	rdb.Del(ctx, queueKey, processingKey)

	// Producer
	go func() {
		for i := 1; i <= 10; i++ {
			task := fmt.Sprintf("task_%d", i)
			rdb.RPush(ctx, queueKey, task)
			time.Sleep(30 * time.Millisecond)
		}
	}()

	// Consumer
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	// Обработчик сигналов
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		logger.Println("Остановка...")
		cancel()
	}()

	// Воркер
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				logger.Println("Воркер завершён")
				return
			default:
				// BRPOPLPUSH — блокирует до 2 секунд, атомарно перемещает
				task, err := rdb.BLMove(ctx, queueKey, processingKey, "RIGHT", "LEFT", 2*time.Minute).Result()
				if errors.Is(err, redis.Nil) {
					continue
				}
				if err != nil {
					logger.Printf("Ошибка BRPOPLPUSH: %v", err)
					continue
				}
				logger.Printf("Обработка задачи: %s", task)
				// Имитация обработки
				success := true
				time.Sleep(100 * time.Millisecond)
				if success {
					// Удаляем из processing
					err = rdb.LRem(ctx, processingKey, 1, task).Err()
					if err != nil {
						logger.Printf("Ошибка удаления из processing: %v", err)
					}
				} else {
					// При ошибке можно вернуть задачу обратно в очередь
					rdb.LPush(ctx, queueKey, task)
					logger.Printf("Задача %s возвращена в очередь", task)
				}
			}

		}
	}()
	wg.Wait()
	logger.Println("Очередь завершена")
	rdb.Del(ctx, queueKey, processingKey)
}

// 3. ПРИОРИТЕТНАЯ ОЧЕРЕДЬ (НЕСКОЛЬКО СПИСКОВ)
func primer3() {
	fmt.Println("\n--- 3. Приоритетная очередь (high, medium, low) ---")

	const (
		highKey = "tasks:priority:high"
		medKey  = "tasks:priority:medium"
		lowKey  = "tasks:priority:low"
	)
	rdb.Del(ctx, highKey, medKey, lowKey)

	// Producer: добавляем задачи с разным приоритетом
	go func() {
		for i := 1; i <= 5; i++ {
			rdb.LPush(ctx, highKey, fmt.Sprintf("high_%d", i))
			rdb.LPush(ctx, medKey, fmt.Sprintf("med_%d", i))
			rdb.LPush(ctx, lowKey, fmt.Sprintf("low_%d", i))
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Consumer: забираем сначала из high, потом med, потом low
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	priorityKeys := []string{highKey, medKey, lowKey}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// BLPOP на списке ключей берёт из первого непустого
				res, err := rdb.BLPop(ctx, 1*time.Second, priorityKeys...).Result()
				if errors.Is(err, redis.Nil) {
					continue
				}
				if err != nil {
					logger.Printf("Ошибка: %v", err)
					continue
				}
				key := res[0]
				task := res[1]
				logger.Printf("Забрана задача: %s (из %s)", task, key)
				time.Sleep(50 * time.Millisecond) // обработка
			}
		}
	}()
	wg.Wait()
	logger.Println("Приоритетная очередь завершена")
	rdb.Del(ctx, highKey, medKey, lowKey)
}

// 4. ОТЛОЖЕННЫЕ ЗАДАЧИ (ЧЕРЕЗ SORTED SET)
func primer4() {
	fmt.Println("\n--- 4. Отложенные задачи через Sorted Set (ZRangeArgs) ---")

	const delayKey = "tasks:delayed"
	const queueKey = "tasks:main"
	rdb.Del(ctx, delayKey, queueKey)

	// Добавляем задачи с временем выполнения (через 5, 10, 15 секунд)
	now := time.Now().Unix()
	tasks := []struct {
		id    string
		delay time.Duration
	}{
		{"taskA", 5 * time.Second},
		{"taskB", 10 * time.Second},
		{"taskC", 15 * time.Second},
	}
	for _, t := range tasks {
		score := float64(now + int64(t.delay.Seconds()))
		payload := fmt.Sprintf(`{"id":"%s","data":"%s"}`, t.id, t.id)
		rdb.ZAdd(ctx, delayKey, redis.Z{Score: score, Member: payload})
		logger.Printf("Задача %s отложена на %v", t.id, t.delay)
	}

	// Воркер, который каждые 2 секунды проверяет готовые задачи
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().Unix()
				// Современный метод: ZRangeArgs с ByScore: true
				ready, err := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
					Key:     delayKey,
					Start:   "-inf",
					Stop:    fmt.Sprintf("%d", now),
					ByScore: true, // интерпретировать Start/Stop как score
					Offset:  0,
					Count:   10,
				}).Result()
				if err != nil {
					logger.Printf("Ошибка ZRangeArgs: %v", err)
					continue
				}
				if len(ready) == 0 {
					continue
				}
				// Атомарно удаляем и отправляем в основную очередь
				for _, task := range ready {
					// Удаляем из ZSET
					removed, err := rdb.ZRem(ctx, delayKey, task).Result()
					if err != nil || removed == 0 {
						continue // возможно, уже обработано другим воркером
					}
					logger.Printf("Задача готова: %s", task)
					// Отправляем в основную очередь
					rdb.RPush(ctx, queueKey, task)
				}
			}
		}
	}()

	// Забираем из основной очереди (задачи будут появляться постепенно)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, err := rdb.BLPop(ctx, 1*time.Second, queueKey).Result()
				if errors.Is(err, redis.Nil) {
					continue
				}
				if err != nil {
					logger.Printf("Ошибка BLPop: %v", err)
					continue
				}
				logger.Printf("Получена задача из основной очереди: %s", res[1])
			}
		}
	}()

	// Даём время на выполнение
	time.Sleep(20 * time.Second)
	cancel()
	wg.Wait()
	logger.Println("Отложенные задачи завершены")
	rdb.Del(ctx, delayKey, queueKey)
}

// 5. STREAMS: ПРОСТОЙ PRODUCER / CONSUMER
func primer5() {
	fmt.Println("\n--- 5. Redis Streams: простой producer и consumer ---")

	const streamKey = "stream:simple"
	rdb.Del(ctx, streamKey)

	// Producer: отправляем 10 сообщений
	go func() {
		for i := 1; i <= 10; i++ {
			msg := map[string]interface{}{
				"id":   i,
				"data": fmt.Sprintf("msg_%d", i),
			}
			id, err := rdb.XAdd(ctx, &redis.XAddArgs{
				Stream: streamKey,
				Values: msg,
			}).Result()
			if err != nil {
				logger.Printf("Ошибка XAdd: %v", err)
			} else {
				logger.Printf("Добавлено сообщение ID: %s", id)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Consumer: читаем все сообщения
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Читаем с самого начала
		var lastID = "0-0"
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{streamKey, lastID},
					Count:   5,
					Block:   2 * time.Second,
				}).Result()
				if errors.Is(err, redis.Nil) {
					continue
				}
				if err != nil {
					logger.Printf("XRead ошибка: %v", err)
					continue
				}
				for _, stream := range res {
					for _, msg := range stream.Messages {
						logger.Printf("Получено: ID=%s, Данные=%v", msg.ID, msg.Values)
						lastID = msg.ID
					}
				}
			}
		}
	}()
	time.Sleep(5 * time.Second)
	cancel()
	wg.Wait()
	logger.Println("Streams простой пример завершён")
	rdb.Del(ctx, streamKey)
}

// 6. STREAMS: ГРУППА ПОТРЕБИТЕЛЕЙ (BALANCING + XACK)
func primer6() {
	fmt.Println("\n--- 6. Streams: группа потребителей с подтверждением ---")

	const streamKey = "stream:group"
	const groupName = "workers"
	rdb.Del(ctx, streamKey)

	// Создаём группу (если не существует)
	err := rdb.XGroupCreate(ctx, streamKey, groupName, "0").Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Fatalf("XGroupCreate ошибка: %v", err)
	}
	logger.Println("Группа создана")

	// Producer: 20 сообщений
	go func() {
		for i := 1; i <= 20; i++ {
			err := rdb.XAdd(ctx, &redis.XAddArgs{
				Stream: streamKey,
				Values: map[string]interface{}{
					"id":   i,
					"text": fmt.Sprintf("message_%d", i),
				},
			}).Err()
			if err != nil {
				logger.Printf("Ошибка XAdd: %v", err)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}()

	// Consumer: 3 воркера в группе
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			consumerName := fmt.Sprintf("worker-%d", workerID)
			for {
				select {
				case <-ctx.Done():
					logger.Printf("Воркер %d завершён", workerID)
					return
				default:
					// Читаем до 2 сообщений за раз
					res, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
						Group:    groupName,
						Consumer: consumerName,
						Streams:  []string{streamKey, ">"},
						Count:    2,
						Block:    1 * time.Second,
					}).Result()
					if errors.Is(err, redis.Nil) {
						continue
					}
					if err != nil {
						logger.Printf("XReadGroup ошибка: %v", err)
						continue
					}
					for _, stream := range res {
						for _, msg := range stream.Messages {
							logger.Printf("Воркер %d получил: ID=%s, Data=%v", workerID, msg.ID, msg.Values)
							// Имитация обработки
							time.Sleep(100 * time.Millisecond)
							// Подтверждаем
							err := rdb.XAck(ctx, streamKey, groupName, msg.ID).Err()
							if err != nil {
								logger.Printf("XACK ошибка: %v", err)
							} else {
								logger.Printf("Воркер %d подтвердил %s", workerID, msg.ID)
							}
						}
					}
				}
			}
		}(w)
	}
	time.Sleep(10 * time.Second)
	cancel()
	wg.Wait()
	logger.Println("Streams группа завершена")
	rdb.Del(ctx, streamKey)
	rdb.XGroupDestroy(ctx, streamKey, groupName)
}

// 7. STREAMS: ОБРАБОТКА ЗАВИСШИХ СООБЩЕНИЙ (XPENDING + XCLAIM)
func primer7() {
	fmt.Println("\n--- 7. Streams: восстановление зависших (pending) сообщений ---")

	const streamKey = "stream:pending"
	const groupName = "recovery_group"
	rdb.Del(ctx, streamKey)

	// Создаём группу
	err := rdb.XGroupCreate(ctx, streamKey, groupName, "0").Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Fatalf("XGroupCreate ошибка: %v", err)
	}
	logger.Println("Группа создана")

	// Добавляем сообщения
	for i := 1; i <= 10; i++ {
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: map[string]interface{}{
				"id":   i,
				"data": fmt.Sprintf("data_%d", i),
			},
		}).Err()
	}

	// Симуляция падения воркера: забираем 5 сообщений, но не подтверждаем
	res, _ := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: "dead_worker",
		Streams:  []string{streamKey, ">"},
		Count:    5,
		Block:    1 * time.Second,
	}).Result()
	var deadIDs []string
	for _, stream := range res {
		for _, msg := range stream.Messages {
			logger.Printf("Забрано (но не подтверждено): ID=%s", msg.ID)
			deadIDs = append(deadIDs, msg.ID)
		}
	}
	logger.Printf("Зависших сообщений: %d", len(deadIDs))

	// Другой воркер проверяет pending
	pendingList, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  groupName,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		logger.Printf("XPENDING ошибка: %v", err)
	}
	var claimIDs []string
	for _, item := range pendingList {
		logger.Printf("Pending: ID=%s, Idle=%v", item.ID, item.Idle)
		claimIDs = append(claimIDs, item.ID)
	}

	if len(claimIDs) > 0 {
		// Забираем зависшие через XCLAIM
		claimed, err := rdb.XClaim(ctx, &redis.XClaimArgs{
			Stream:   streamKey,
			Group:    groupName,
			Consumer: "recovery_worker",
			MinIdle:  1 * time.Second,
			Messages: claimIDs,
		}).Result()
		if err != nil {
			logger.Printf("XClaim ошибка: %v", err)
		} else {
			for _, msg := range claimed {
				logger.Printf("Восстановлено сообщение: ID=%s, Data=%v", msg.ID, msg.Values)
				// Подтверждаем
				rdb.XAck(ctx, streamKey, groupName, msg.ID)
			}
		}
	}
	logger.Println("Восстановление завершено")
	rdb.Del(ctx, streamKey)
}

// 8. МОНИТОРИНГ И СТАТИСТИКА ОЧЕРЕДЕЙ
func primer8() {
	fmt.Println("\n--- 8. Мониторинг очереди и метрики ---")

	const queueKey = "tasks:monitor"
	rdb.Del(ctx, queueKey)

	// Добавляем задачи
	for i := 1; i <= 100; i++ {
		rdb.RPush(ctx, queueKey, fmt.Sprintf("task_%d", i))
	}
	logger.Println("Добавлено 100 задач")

	// Метрики: размер, количество, примерное время обработки
	length, _ := rdb.LLen(ctx, queueKey).Result()
	logger.Printf("Размер очереди: %d", length)

	// Парсим задачи, чтобы оценить нагрузку
	// Здесь можно было бы использовать LRANGE для выборки
	sample, _ := rdb.LRange(ctx, queueKey, 0, 4).Result()
	logger.Printf("Пример задач: %v", sample)

	// Мониторинг задержки (минимальное время)
	// Можно создать отдельный воркер, который собирает метрики
	logger.Println("Мониторинг завершён")
	rdb.Del(ctx, queueKey)
}
