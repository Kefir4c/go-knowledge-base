package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 4.1. MASTER‑REPLICA РЕПЛИКАЦИЯ — ТЕОРИЯ + ПРАКТИКА
ТЕОРИЯ: ЧТО ТАКОЕ РЕПЛИКАЦИЯ?

Репликация (replication) — это механизм асинхронного копирования данных
с одного сервера Redis (master) на один или несколько других серверов (replica).

Цель:
- Повышение пропускной способности для чтения (read scaling).
- Обеспечение базовой отказоустойчивости (если мастер упал, можно переключиться на реплику).
- Резервное копирование.

1. КАК РАБОТАЕТ РЕПЛИКАЦИЯ?

1.1. Основные компоненты:
  - Мастер (master) — основной сервер, обрабатывающий запросы записи.
  - Реплика (replica) — сервер, который копирует данные с мастера (read-only).

1.2. Процесс настройки:

	Реплика подключается к мастеру через команду REPLICAOF (или параметр в конфиге).
	    REPLICAOF <master_ip> <master_port>

	При успешном подключении:
	1. Реплика отправляет мастеру команду PSYNC для синхронизации.
	2. Мастер отправляет полный снимок данных (RDB) реплике (полная синхронизация).
	3. После получения RDB, реплика загружает его в память.
	4. Мастер передаёт все новые команды (накопленные в буфере) реплике.
	5. Далее мастер отправляет все новые команды, поступающие от клиентов, в реальном времени.

1.3. Режимы синхронизации:

  - Полная синхронизация (full sync): мастер отправляет RDB + буфер команд.
    Используется при первом подключении или после потери соединения и частичной синхронизации.

  - Частичная синхронизация (partial sync): мастер отправляет только команды,
    накопленные в буфере после последнего отставания реплики.
    Используется, если реплика потеряла соединение ненадолго.

    Реплика хранит смещение репликации (replication offset) — номер байта в журнале.
    Мастер хранит backlog (буфер) команд для частичной синхронизации.

1.4. Асинхронность:
  - Запись на мастер: клиент получает подтверждение только после записи в память мастера.
  - Репликация происходит асинхронно: команды отправляются реплике уже после ответа клиенту.
  - Это означает, что мастер не ждёт подтверждения от реплик перед ответом клиенту,
    что делает репликацию быстрой, но создаёт риск потери данных при падении мастера.

2. НАСТРОЙКА РЕПЛИКАЦИИ

2.1. В файле конфигурации (redis.conf):

	replicaof <master_ip> <master_port>
	masteruser <username>      # если включена аутентификация
	masterauth <password>
	replica-read-only yes      # запретить запись на реплику (по умолчанию)
	repl-diskless-sync yes     # синхронизация без диска (прямая передача)
	repl-diskless-sync-delay 5 # задержка перед синхронизацией

2.2. Через команды:

	REPLICAOF <master_ip> <master_port>   # подключиться к мастеру
	REPLICAOF NO ONE                      # отключить репликацию (стать мастером)

2.3. Аутентификация:

	Если на мастере включена аутентификация, реплика должна указать пароль:
	    CONFIG SET masterauth <password>
	Или в конфиге: masterauth <password>

3. СОСТОЯНИЕ РЕПЛИКАЦИИ (INFO replication)

Ключевые поля:
- role: master / slave (устаревшее) → master/replica.
- master_host: IP мастера (на реплике).
- master_port: порт мастера.
- master_link_status: up / down — состояние соединения с мастером.
- master_last_io_seconds_ago: сколько секунд назад был последний обмен с мастером.
- master_sync_in_progress: 1/0 — идёт ли синхронизация.
- master_sync_last_io_seconds_ago: время последнего I/O при синхронизации.
- connected_slaves: количество подключенных реплик (на мастере).
- slave_repl_offset: смещение репликации на реплике.
- master_repl_offset: смещение репликации на мастере.
- repl_backlog_histlen: размер backlog (буфера частичной синхронизации).

4. ЗАДЕРЖКИ И ПОТЕРЯ ДАННЫХ

4.1. Задержка репликации (replication lag):
  - Определяется как разница между master_repl_offset и slave_repl_offset.
  - Если lag растёт → реплика не успевает за мастером (сеть, диск, нагрузка).

4.2. Потеря данных при падении мастера:
  - Если мастер падает, а реплика ещё не получила некоторые команды,
    эти данные теряются (поскольку запись на мастер уже подтверждена клиенту).
  - Для минимизации потерь можно использовать синхронную репликацию
    (WAIT команда — заставляет мастер ждать подтверждения от реплик).
  - WAIT <num_replicas> <timeout> — блокирует запись, пока N реплик не подтвердят.

4.3. Время отработки отказа (failover):
  - При ручном переключении (или с помощью Sentinel) есть время,
    пока клиент переключится на новую реплику.

5. РЕПЛИКАЦИЯ В КЛАСТЕРЕ (Redis Cluster)

В кластере репликация является частью архитектуры:
- Каждый мастер-узел имеет одну или несколько реплик.
- Реплики автоматически реплицируют данные с мастера.
- При падении мастера, кластер автоматически повышает одну из реплик до мастера.
- В кластере реплики также обслуживают чтение (если разрешено).

6. СВЯЗЬ С GO

6.1. go-redis поддерживает чтение с реплик через:
  - В Cluster: NewClusterClient с опцией ReadOnly.
    Клиент автоматически маршрутизирует GET/запросы на реплики (если разрешено).
  - В обычном режиме: можно явно создать отдельный клиент для реплики.

6.2. Мониторинг репликации:
  - Через INFO replication.
  - Получение статуса и задержек.

6.3. Обработка падения мастера:
  - При использовании Sentinel клиент автоматически переключается на нового мастера.
  - В противном случае нужно вручную переключить клиент на другую реплику.

7. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ

1. Всегда включайте репликацию для production-систем (минимум одна реплика).
2. Используйте чтение с реплик для снижения нагрузки на мастер (но учитывайте задержки).
3. Мониторьте задержку репликации (lag) и алертьте при её росте.
4. Настраивайте repl-backlog-size достаточно большим, чтобы избежать полной синхронизации.
5. Для критичных данных используйте WAIT для синхронного подтверждения.
6. Используйте Sentinel или Cluster для автоматического failover.
7. Тестируйте сценарии падения мастера и восстановления.
*/

// Глобальные клиенты
var (
	masterClient  *redis.Client
	replicaClient *redis.Client
	clusterClient *redis.ClusterClient
	ctx           = context.Background()
)

// Инициализация (подразумевается, что реплика на порту 6380, кластер на 7000-7002)
func init() {
	masterClient = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     10,
		MinIdleConns: 2,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	replicaClient = redis.NewClient(&redis.Options{
		Addr:         "localhost:6380",
		PoolSize:     10,
		MinIdleConns: 2,
		ReadTimeout:  3 * time.Second,
	})
	clusterClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{"localhost:7000", "localhost:7001", "localhost:7002"},
		ReadOnly: true, // разрешить чтение с реплик
		PoolSize: 10,
	})
}

func main() {
	fmt.Println("=== MASTER-REPLICA РЕПЛИКАЦИЯ: ПРАКТИКА ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
}

// 1. Создание клиентов для мастера и реплик
func primer1() {
	fmt.Println("--- 1. Настройка клиентов для мастера и реплики ---")
	// Уже созданы в init, просто выводим информацию
	ping := func(c *redis.Client, name string) {
		if err := c.Ping(ctx).Err(); err != nil {
			fmt.Printf("%s недоступен: %v\n", name, err)
		} else {
			fmt.Printf("%s готов\n", name)
		}
	}
	ping(masterClient, "master1")
	ping(replicaClient, "replica1")
}

// 2. Безопасное чтение: сначала реплика, при ошибке — мастер
func primer2() {
	fmt.Println("--- 2. Чтение с реплики с fallback на мастер ---")

	getWithFallback := func(key string) (string, error) {
		val, err := replicaClient.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				log.Printf("Реплика недоступна: %v, переключаемся на мастер", err)
			}
			return masterClient.Get(ctx, key).Result()
		}
		return val, err
	}

	// Тестируем
	masterClient.Set(ctx, "testkey", "value_from_master", 0)
	val, err := getWithFallback("testkey")
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		fmt.Printf("Получено: %s\n", val)
	}
	masterClient.Del(ctx, "testkey")
}

// 3. Запись с синхронным подтверждением реплик (WAIT)
func primer3() {
	fmt.Println("--- 3. Запись с WAIT (ожидание подтверждения от реплик) ---")
	writeWithWait := func(key, value string, waitReplicas int, timeout time.Duration) error {
		// Пишем в мастер
		err := masterClient.Set(ctx, key, value, 0).Err()
		if err != nil {
			return fmt.Errorf("SET failed: %w", err)
		}
		// Ждём подтверждения от реплик
		res, err := masterClient.Do(ctx, "WAIT", waitReplicas, timeout.Milliseconds()).Int()
		if err != nil {
			return fmt.Errorf("WAIT failed: %w", err)
		}
		if res < waitReplicas {
			return fmt.Errorf("only %d replicas confirmed (need %d)", res, waitReplicas)
		}
		return nil
	}
	err := writeWithWait("critical_key", "important_data", 1, 2*time.Second)
	if err != nil {
		fmt.Printf("Ошибка записи: %v\n", err)
	} else {
		fmt.Println("Данные записаны и подтверждены репликой")
	}
	masterClient.Del(ctx, "critical_key")
}

// 4. Мониторинг задержки репликации и алертинг
func primer4() {
	fmt.Println("--- 4. Мониторинг задержки репликации ---")

	checkReplicationLag := func() (int64, error) {
		info, err := masterClient.InfoMap(ctx, "replication").Result()
		if err != nil {
			return 0, err
		}
		repInfo := info["replication"]
		masterOffset, _ := strconv.ParseInt(repInfo["master_repl_offset"], 10, 64)
		// Получаем смещение реплики (можно запросить у реплики отдельно)
		// Для простоты считаем, что на реплике есть INFO
		repInfoSlave, err := replicaClient.InfoMap(ctx, "replication").Result()
		if err != nil {
			return 0, err
		}
		slaveOffset, _ := strconv.ParseInt(repInfoSlave["replication"]["slave_repl_offset"], 10, 64)
		lag := masterOffset - slaveOffset
		return lag, nil
	}

	lag, err := checkReplicationLag()
	if err != nil {
		fmt.Printf("Ошибка мониторинга: %v\n", err)
	} else {
		fmt.Printf("Задержка репликации: %d байт\n", lag)
		if lag > 1024*1024 { // более 1 МБ
			fmt.Println("ЗАДЕРЖКА КРИТИЧЕСКАЯ! Проверьте нагрузку или сеть.")
		} else {
			fmt.Println("Задержка в норме.")
		}
	}
}

// 5. Ручной failover (повышение реплики до мастера)
func primer5() {
	fmt.Println("--- 5. Ручной failover при падении мастера ---")

	// Проверяем мастер
	if err := masterClient.Ping(ctx).Err(); err != nil {
		fmt.Println("Мастер недоступен, начинаем failover")
		// Повышаем реплику до мастера
		if err := replicaClient.Do(ctx, "failover", "NO", "ONE").Err(); err != nil {
			fmt.Printf("Ошибка повышения реплики: %v\n", err)
			return
		}
		fmt.Println("Реплика повышена до мастера")
		// Переключаем клиенты: теперь мастер = бывшая реплика
		// В реальном коде нужно обновить указатели на клиенты или использовать Sentinel
		fmt.Println("Переключите клиенты на новый адрес мастера (вручную или через Sentinel)")
	} else {
		fmt.Println("Мастер работает, failover не требуется")
	}
}

// 6. Чтение с реплик в Redis Cluster (ReadOnly)
func primer6() {
	fmt.Println("--- 6. Чтение с реплик в Redis Cluster ---")

	// clusterClient уже настроен с ReadOnly: true
	// Все GET-запросы будут направляться на реплики, если они доступны
	key := "cluster:test"
	err := clusterClient.Set(ctx, key, "cluster_value", 0).Err()
	if err != nil {
		fmt.Printf("Ошибка записи в кластер: %v\n", err)
		return
	}
	val, err := clusterClient.Get(ctx, key).Result()
	if err != nil {
		fmt.Printf("Ошибка чтения из кластера: %v\n", err)
	} else {
		fmt.Printf("Значение из кластера (прочитано с реплики): %s\n", val)
	}
	clusterClient.Del(ctx, key)
}

// 7. Повторные попытки при ошибках репликации
func primer7() {
	fmt.Println("--- 7. Повторные попытки при ошибках репликации ---")

	retryRead := func(key string, maxRetries int) (string, error) {
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			val, err := replicaClient.Get(ctx, key).Result()
			if err == nil {
				return val, nil
			}
			lastErr = err
			// Если ошибка не временная (например, ключа нет), не повторяем
			if errors.Is(err, redis.Nil) {
				return "", lastErr
			}
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
		// Если все попытки неудачны, пробуем мастер
		return masterClient.Get(ctx, key).Result()
	}

	// Тестируем
	masterClient.Set(ctx, "retry_key", "value", 0)
	val, err := retryRead("retry_key", 3)
	if err != nil {
		fmt.Printf("Ошибка после всех попыток: %v\n", err)
	} else {
		fmt.Printf("Получено после повторов: %s\n", val)
	}
	masterClient.Del(ctx, "retry_key")
}

// 8. Балансировка чтения между несколькими репликами
func primer8() {
	fmt.Println("--- 8. Балансировка чтения между репликами (round-robin) ---")

	// Имитация нескольких реплик (для простоты используем список адресов)
	replicaAddrs := []string{"localhost:6380", "localhost:6381", "localhost:6382"}
	var counter uint32

	getReplicaClient := func() *redis.Client {
		idx := atomic.AddUint32(&counter, 1) % uint32(len(replicaAddrs))
		addr := replicaAddrs[idx]
		return redis.NewClient(&redis.Options{Addr: addr})
	}

	readWithBalance := func(key string) (string, error) {
		client := getReplicaClient()
		defer client.Close()
		return client.Get(ctx, key).Result()
	}

	// Тестируем (предполагаем, что реплики запущены)
	masterClient.Set(ctx, "balance_key", "balanced_value", 0)
	for i := 0; i < 5; i++ {
		val, err := readWithBalance("balance_key")
		if err != nil {
			fmt.Printf("Попытка %d: ошибка %v\n", i+1, err)
		} else {
			fmt.Printf("Попытка %d: %s\n", i+1, val)
		}
	}
	masterClient.Del(ctx, "balance_key")
}
