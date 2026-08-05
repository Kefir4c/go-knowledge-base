package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

/*
УРОК 7.3. РАСПРЕДЕЛЁННАЯ БЛОКИРОВКА
0. ВВЕДЕНИЕ: ЗАЧЕМ НУЖНА РАСПРЕДЕЛЁННАЯ БЛОКИРОВКА?

В распределённых системах часто требуется синхронизировать доступ к общим ресурсам:
- Запись в общий файл или базу данных.
- Выполнение критической операции (например, обновление баланса).
- Предотвращение одновременной обработки одной и той же задачи (дедупликация).
- Обеспечение «эксклюзивного» доступа к ограниченному ресурсу.

В рамках одного процесса для этого используются мьютексы (sync.Mutex). Но когда
несколько экземпляров приложения работают на разных серверах, нужен внешний
координатор, видимый всем участникам. Redis — идеальный кандидат, потому что он
быстрый, атомарный и поддерживает операции с условиями.

1. ПРОСТАЯ БЛОКИРОВКА: КАК ЭТО РАБОТАЕТ

Самая простая реализация использует команду SET с опциями NX и PX/EX:

    SET resource_name my_random_value NX PX 30000

- **NX** — установить ключ, только если он не существует (Not eXists).
- **PX 30000** — установить время жизни 30 000 миллисекунд (30 секунд).
- **my_random_value** — уникальная строка, идентифицирующая клиента (например, UUID).

Это атомарная операция: Redis гарантирует, что либо ключ будет создан, либо
ничего не произойдёт. Если ключ уже существует, команда вернёт nil.

После захвата блокировки клиент может выполнять критическую операцию, а затем
освободить блокировку (удалить ключ). Но важно делать это безопасно.

2. БЕЗОПАСНОЕ ОСВОБОЖДЕНИЕ (SAFE UNLOCK)

Проблема: если клиент просто выполняет DEL по ключу, он может случайно удалить
блокировку, захваченную другим клиентом. Это происходит в двух случаях:
- TTL истёк, блокировка была перехвачена другим клиентом, а первый клиент всё
  ещё пытается её освободить.
- Клиент подвис, а затем «проснулся» и попытался освободить уже истекшую блокировку.

Решение: освобождать блокировку только если её значение совпадает с нашим.
Это делается через Lua-скрипт, который выполняется атомарно:

    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("del", KEYS[1])
    else
        return 0
    end

Таким образом, только клиент, владеющий блокировкой, может её удалить. Это
называется «безопасное освобождение» (Safe Unlock).

3. АВТОМАТИЧЕСКОЕ ПРОДЛЕНИЕ (AUTO-REFRESH / WATCHDOG)

Проблема: если операция выполняется дольше, чем TTL, блокировка истекает,
и другой клиент может её захватить, что приведёт к одновременному доступу.

Решение: клиент периодически продлевает TTL (например, каждые ⅓ от TTL).
Это делает фоновый процесс («сторож»). Продление также должно быть безопасным:
обновлять TTL можно только если мы всё ещё владеем блокировкой (проверка
значения). Lua-скрипт для продления:

    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("expire", KEYS[1], ARGV[2])
    else
        return 0
    end

Этот подход называется «watchdog» или «auto-refresh». Он позволяет удерживать
блокировку столько, сколько нужно, без риска её потери.

4. ПРОБЛЕМЫ И ПОДВОДНЫЕ КАМНИ (ОСОБО ВАЖНО НА СОБЕСЕДОВАНИЯХ!)

4.1. **Асинхронная репликация** (Master → Replica)
    - Мастер может принять SET NX и ответить клиенту, но не успеть реплицировать
      команду на реплику. Если мастер падает, реплика становится мастером,
      но блокировка на ней отсутствует. Другой клиент может захватить блокировку
      на новом мастере, создавая две блокировки одновременно.
    - Для критичных систем рекомендуется использовать несколько Redis-инстансов
      (Redlock) или синхронную репликацию с WAIT.

4.2. **Зависание клиента после захвата блокировки**
    - Клиент захватил блокировку, но затем подвис (например, GC пауза, сетевой
      таймаут). Блокировка истечёт по TTL, и другой клиент сможет её захватить.
      Когда первый клиент «проснётся», он может попытаться освободить уже чужую
      блокировку, но Safe Unlock предотвратит это (значение не совпадает).

4.3. **Перекос часов (clock drift)**
    - Если используется TTL в миллисекундах и часы на разных серверах расходятся,
      это может привести к преждевременному истечению блокировки.
    - Redlock учитывает эту проблему, используя запас времени (lock validity time).

4.4. **Гонка между TTL и продлением**
    - Если клиент продлевает блокировку, но TTL истёк до того, как продление
      выполнено, другой клиент может захватить блокировку. Авто-продление должно
      выполняться с запасом (например, каждые ⅓ TTL).

5. REDLOCK (АЛГОРИТМ РАСПРЕДЕЛЁННОЙ БЛОКИРОВКИ)

Сальваторе Санфилиппо (создатель Redis) предложил алгоритм Redlock для обеспечения
более высокой надёжности. Он работает так:

1. Клиент подключается к N независимым Redis-инстансам (обычно 5).
2. Клиент пытается захватить блокировку на всех инстансах с одинаковым ключом
   и значением, используя SET NX PX.
3. Время жизни (TTL) блокировки должно быть меньше, чем суммарное время,
   потраченное на захват (включая сетевые задержки).
4. Если клиенту удалось захватить блокировку на большинстве инстансов
   (N/2 + 1), то блокировка считается полученной.
5. Если большинство не достигнуто, клиент освобождает блокировку на всех
   инстансах, где она была установлена.

Преимущества: устойчивость к падению части узлов.
Недостатки:
- Сложность реализации и синхронизации (необходимы точные часы).
- Критика алгоритма (Мартин Клеппман указал на проблемы с GC паузами и
  приостановками, которые могут нарушить работу).
- Дополнительная задержка (много сетевых вызовов).

Рекомендация: В большинстве случаев достаточно простой блокировки с репликацией
и Sentinel/Cluster. Redlock стоит использовать только в системах, где критичны
отказы нескольких узлов, и вы готовы к дополнительной сложности.

6. СРАВНЕНИЕ ПОДХОДОВ К РАСПРЕДЕЛЁННОЙ БЛОКИРОВКЕ

┌──────────────────────────┬────────────────────────────┬────────────────────────────┐
│ ХАРАКТЕРИСТИКА           │ ОДИН REDIS + REPLICATION   │ REDLOCK (МНОГО INSTANCES) │
├──────────────────────────┼────────────────────────────┼────────────────────────────┤
│ Сложность реализации     │ Низкая                     │ Высокая                    │
│ Надёжность               │ Средняя (асинхронная       │ Высокая (дублирование)     │
│                          │ репликация)                │                            │
│ Производительность       │ Высокая (один вызов)       │ Низкая (N вызовов)         │
│ Защита от падения мастера│ Частичная (Sentinel)       │ Полная (если большинство)  │
│ Актуальность             │ Подходит для 95% задач     │ Только для критичных       │
│                          │                            │ систем                     │
└──────────────────────────┴────────────────────────────┴────────────────────────────┘

7. РАБОТА С LUA В БЛОКИРОВКАХ

Lua-скрипты обеспечивают атомарность при освобождении и продлении. Они выполняются
на сервере целиком, без прерываний, что гарантирует корректность операций
«проверка и действие».

Использование Lua обязательно для безопасного освобождения и продления.

8. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

1. **Всегда используйте уникальное значение** (UUID) для идентификации владельца.
2. **Применяйте Lua для освобождения** — это единственный безопасный способ.
3. **Устанавливайте TTL с запасом**, чтобы операция гарантированно укладывалась
   в время жизни (или используйте auto-refresh).
4. **Запускайте фоновый процесс для auto-refresh**, если операция может быть долгой.
5. **Используйте retry с экспоненциальной задержкой** при неудачном захвате.
6. **Устанавливайте таймауты на захват** (через контекст), чтобы не зависать.
7. **Мониторьте время удержания блокировки** (логируйте и алертьте при превышении).
8. **В Go используйте готовые библиотеки** (например, `github.com/bsm/redislock`),
   чтобы не изобретать велосипед.
9. **Для кластера используйте хеш-теги** (`{lock}`), чтобы ключ попадал в один слот.
10. **Тестируйте сценарии падения клиента и продления** в тестовой среде.

9. СВЯЗЬ С GO (go-redis) — ОБЗОР КЛЮЧЕВЫХ МЕТОДОВ

- `SetNX(ctx, key, value, ttl)` — захват блокировки (возвращает bool).
- `Eval` / `NewScript` — выполнение Lua-скриптов для освобождения и продления.
- `Watch` + `TxPipeline` — альтернативный способ для более сложных сценариев
  (но обычно достаточно Lua).

Рекомендуется оборачивать логику блокировки в отдельный тип (например, `DistributedLock`),
который инкапсулирует захват, освобождение, продление и повторные попытки.

10. ТИПИЧНЫЕ ОШИБКИ НА СОБЕСЕДОВАНИЯХ

«Освобождение блокировки через просто DEL безопасно» — НЕТ, нужно проверять владельца.
«TTL — это надёжная защита от deadlock» — НЕТ, оно защищает, но не решает проблему
   одновременного доступа при долгой операции.
«Redlock — всегда лучшее решение» — НЕТ, оно избыточно для большинства систем.
«Авто-продление не нужно, можно выставить большой TTL» — НЕТ, большой TTL увеличивает
   риск deadlock и замедляет восстановление.
«Я захватил блокировку, значит, она у меня до освобождения» — НЕТ, если TTL истёк,
   блокировка может перейти к другому.

11. ИТОГИ

Распределённая блокировка — важный инструмент, но её реализация требует понимания
множества нюансов: атомарность, репликация, безопасное освобождение, продление.
Простая блокировка на одном Redis подходит для 95% случаев. Redlock стоит применять
только при строгих требованиях к отказоустойчивости. В Go используйте готовые
библиотеки, чтобы избежать ошибок.
*/

var rdb *redis.Client
var ctx = context.Background()
var logger = log.New(os.Stdout, "[LOCK] ", log.LstdFlags|log.Lshortfile)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     20,
		MinIdleConns: 5,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

func main() {
	fmt.Println("=== РАСПРЕДЕЛЁННАЯ БЛОКИРОВКА\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
}

// 1. БАЗОВАЯ БЛОКИРОВКА (SET NX + БЕЗОПАСНОЕ ОСВОБОЖДЕНИЕ)
func primer1() {
	fmt.Println("--- 1. Базовая блокировка с безопасным освобождением ---")

	type Lock struct {
		client *redis.Client
		key    string
		value  string
		ttl    time.Duration
	}

	// Функция захвата блокировки
	acquire := func(l *Lock) (bool, error) {
		ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
		if err != nil {
			return false, fmt.Errorf("ошибка SETNX: %w", err)
		}
		return ok, nil
	}

	release := func(l *Lock) error {
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`)
		res, err := script.Run(ctx, l.client, []string{l.key}, l.value).Int()
		if err != nil {
			return fmt.Errorf("ошибка Lua: %w", err)
		}
		if res == 0 {
			return errors.New("блокировка не принадлежит этому клиенту или уже истекла")
		}
		return nil
	}

	// Использование
	lock := &Lock{
		client: rdb,
		key:    "lock:resource",
		value:  uuid.New().String(),
		ttl:    10 * time.Second,
	}

	ok, err := acquire(lock)
	if err != nil {
		logger.Printf("Ошибка захвата: %v", err)
		return
	}
	if !ok {
		fmt.Println("❌ Блокировка уже занята")
		return
	}
	fmt.Printf("✅ Блокировка захвачена (value: %s)\n", lock.value)

	// Имитация работы
	time.Sleep(2 * time.Second)

	// Освобождение
	if err := release(lock); err != nil {
		logger.Printf("Ошибка освобождения: %v", err)
	} else {
		fmt.Println("✅ Блокировка освобождена")
	}
}

// 2. ДЕДУПЛИКАЦИЯ ЗАДАЧ (ПРЕДОТВРАЩЕНИЕ ПОВТОРОВ)
type DedupLock struct {
	client *redis.Client
	ttl    time.Duration
}

func (d *DedupLock) TryProcess(ctx context.Context, taskID string, process func() error) (bool, error) {
	key := "dedup:" + taskID
	value := uuid.New().String()
	ok, err := d.client.SetNX(ctx, key, value, d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("ошибка захвата: %w", err)
	}
	if !ok {
		return false, nil
	}
	defer func() {
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`)
		script.Run(ctx, d.client, []string{key}, value)
	}()
	if err := process(); err != nil {
		return true, err
	}
	return true, nil
}
func primer2() {
	fmt.Println("--- 2. Дедупликация задач ---")
	lock := &DedupLock{client: rdb, ttl: 10 * time.Second}
	taskID := "task:123"
	rdb.Del(ctx, "dedup:"+taskID)
	var wg sync.WaitGroup
	results := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := lock.TryProcess(ctx, taskID, func() error {
				time.Sleep(1 * time.Second)
				fmt.Println("Обработка выполнена")
				return nil
			})
			if err != nil {
				logger.Printf("Ошибка: %v", err)
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	processed := 0
	for ok := range results {
		if ok {
			processed++
		}
	}
	fmt.Printf("Успешно обработано (уникальных выполнений): %d\n", processed)
	rdb.Del(ctx, "dedup:"+taskID)
	fmt.Println()
}

// 3. СЕМАФОР (ОГРАНИЧЕНИЕ ОДНОВРЕМЕННЫХ ОПЕРАЦИЙ)
type Semaphore struct {
	client   *redis.Client
	key      string
	limit    int
	ttl      time.Duration
	mu       sync.Mutex
	acquired map[string]struct{}
}

func (s *Semaphore) Acquire(ctx context.Context) (bool, error) {
	script := redis.NewScript(`	
	local key = KEYS[1]
	local limit = tonumber(ARGV[1])
	local ttl = tonumber(ARGV[2])
	local current = redis.call("INCR",key)
	if current == 1 then
		redis.call("EXPIRE", key, ttl)
	end
	if current <- limit then
		return current
	else
		redis.call("DECR",key)
		return 0
	end
`)
	res, err := script.Run(ctx, s.client, []string{s.key}, s.limit, int(s.ttl.Seconds())).Int()
	if err != nil {
		return false, err
	}
	if res == 0 {
		return false, nil
	}
	s.mu.Lock()
	if s.acquired == nil {
		s.acquired = make(map[string]struct{})
	}
	s.acquired[uuid.New().String()] = struct{}{}
	s.mu.Unlock()
	return true, nil
}

func (s *Semaphore) Release(ctx context.Context) error {
	s.mu.Lock()
	if len(s.acquired) == 0 {
		s.mu.Unlock()
		return errors.New("нет захваченных слотов")
	}
	s.mu.Unlock()
	_, err := s.client.Decr(ctx, s.key).Result()
	return err
}

func primer3() {
	fmt.Println("--- 3. Семафор ---")
	sem := &Semaphore{client: rdb, key: "sem:api", limit: 2, ttl: 30 * time.Second}
	rdb.Del(ctx, sem.key)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ok, err := sem.Acquire(ctx)
			if err != nil {
				logger.Printf("Горутина %d ошибка: %v", id, err)
				return
			}
			if !ok {
				logger.Printf("Горутина %d: не удалось занять слот", id)
				return
			}
			logger.Printf("Горутина %d: слот занят", id)
			time.Sleep(1 * time.Second)
			if err := sem.Release(ctx); err != nil {
				logger.Printf("Горутина %d ошибка освобождения: %v", id, err)
			} else {
				logger.Printf("Горутина %d: освобождён слот", id)
			}
		}(i)
	}
	wg.Wait()
	rdb.Del(ctx, sem.key)
	fmt.Println()
}

// 4. ИСПОЛЬЗОВАНИЕ ГОТОВОЙ БИБЛИОТЕКИ (BSM/REDISLOCK)
type MigrationLock struct {
	client *redis.Client
	key    string
}

func (m *MigrationLock) Lock(ctx context.Context, owner string, ttl time.Duration) (bool, error) {
	ok, err := m.client.SetNX(ctx, m.key, owner, ttl).Result()
	return ok, err
}

func (m *MigrationLock) Unlock(ctx context.Context, owner string) error {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, m.client, []string{m.key}, owner).Int()
	if err != nil {
		return err
	}
	if res == 0 {
		return errors.New("не удалось освободить")
	}
	return nil
}

func primer4() {
	fmt.Println("--- 4. Блокировка для миграции ---")
	mig := &MigrationLock{client: rdb, key: "lock:migration"}
	owner := uuid.New().String()
	ok, err := mig.Lock(ctx, owner, 30*time.Second)
	if err != nil {
		logger.Printf("Ошибка: %v", err)
		return
	}
	if !ok {
		fmt.Println("❌ Миграция уже выполняется")
		return
	}
	fmt.Printf("✅ Миграция захвачена владельцем %s\n", owner)
	time.Sleep(2 * time.Second)
	if err := mig.Unlock(ctx, owner); err != nil {
		logger.Printf("Ошибка освобождения: %v", err)
	} else {
		fmt.Println("✅ Миграция завершена, блокировка освобождена")
	}
	fmt.Println()
}

// 5. ЗАЩИТА КРИТИЧЕСКОЙ ОПЕРАЦИИ
type OperationGuard struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

func (g *OperationGuard) Execute(ctx context.Context, op func() error) error {
	value := uuid.New().String()
	ok, err := g.client.SetNX(ctx, g.key, value, g.ttl).Result()
	if err != nil {
		return fmt.Errorf("ошибка захвата блокировки: %w", err)
	}
	if !ok {
		return errors.New("операция уже выполняется")
	}
	defer func() {
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`)
		script.Run(ctx, g.client, []string{g.key}, value)
	}()
	return op()
}
func primer5() {
	fmt.Println("--- 5. Защита критической операции ---")
	guard := &OperationGuard{client: rdb, key: "guard:critical", ttl: 10 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := guard.Execute(ctx, func() error {
				logger.Printf("Горутина %d: выполняет критическую операцию", id)
				time.Sleep(2 * time.Second)
				return nil
			})
			if err != nil {
				logger.Printf("Горутина %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	rdb.Del(ctx, guard.key)
	fmt.Println()
}

// 6. REDLOCK (ИМИТАЦИЯ МНОЖЕСТВЕННЫХ ИНСТАНСОВ)
type RateLimiter struct {
	client *redis.Client
	key    string
	limit  int
	window time.Duration
}

func (r *RateLimiter) Allow(ctx context.Context) (bool, error) {
	script := redis.NewScript(`
	local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local current = redis.call('INCR', key)
		if current == 1 then
			redis.call('EXPIRE', key, window)
		end
		if current <= limit then
			return 1
		else
			return 0
		end
`)
	res, err := script.Run(ctx, r.client, []string{r.key}, r.limit, int(r.window.Seconds())).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func primer6() {
	fmt.Println("--- 6. Распределённый Rate Limiter ---")
	limiter := &RateLimiter{client: rdb, key: "rate:expensive", limit: 3, window: 10 * time.Second}
	rdb.Del(ctx, limiter.key)
	for i := 0; i < 10; i++ {
		ok, err := limiter.Allow(ctx)
		if err != nil {
			logger.Printf("Ошибка: %v", err)
			continue
		}
		if ok {
			fmt.Printf("Запрос %d разрешён\n", i+1)
		} else {
			fmt.Printf("Запрос %d отклонён (лимит)\n", i+1)
		}
	}
	rdb.Del(ctx, limiter.key)
	fmt.Println()
}

// 7. БЛОКИРОВКА С МЕТРИКАМИ
type LockWithMetrics struct {
	client   *redis.Client
	key      string
	ttl      time.Duration
	value    string
	held     bool
	mu       sync.Mutex
	acquired time.Time
}

func (l *LockWithMetrics) Acquire(ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return false, errors.New("уже захвачена")
	}
	val := uuid.New().String()
	ok, err := l.client.SetNX(ctx, l.key, val, l.ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	l.value = val
	l.held = true
	l.acquired = time.Now()
	logger.Printf("Блокировка захвачена в %v", l.acquired)
	return true, nil
}

func (l *LockWithMetrics) Release(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.held {
		return errors.New("не захвачена")
	}
	elapsed := time.Since(l.acquired)
	logger.Printf("Блокировка удерживалась %v", elapsed)
	if elapsed > l.ttl {
		logger.Printf("ПРЕВЫШЕНИЕ TTL! (%v > %v)", elapsed, l.ttl)
	}
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, l.client, []string{l.key}, l.value).Int()
	l.held = false
	l.value = ""
	if err != nil {
		return err
	}
	if res == 0 {
		return errors.New("не удалось освободить (чужая или истекла)")
	}
	return nil
}
func primer7() {
	fmt.Println("--- 8. Блокировка с метриками ---")
	lock := &LockWithMetrics{client: rdb, key: "lock:metrics", ttl: 5 * time.Second}
	ok, err := lock.Acquire(ctx)
	if err != nil {
		logger.Printf("Ошибка: %v", err)
		return
	}
	if !ok {
		fmt.Println("Блокировка занята")
		return
	}
	time.Sleep(3 * time.Second)
	if err := lock.Release(ctx); err != nil {
		logger.Printf("Ошибка освобождения: %v", err)
	} else {
		fmt.Println("Блокировка освобождена")
	}
	fmt.Println()
}
