package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
ЗАЧЕМ НУЖЕН SLOWLOG?

Redis — это однопоточный сервер. Любая команда, выполняющаяся долго, блокирует
все остальные операции. В высоконагруженных системах даже одна медленная команда
может вызвать каскадные задержки, таймауты клиентов и общее падение пропускной
способности.

SLOWLOG — это встроенный инструмент мониторинга, который записывает команды,
выполняющиеся дольше заданного порога. Он позволяет:
- Обнаруживать узкие места в приложении.
- Оптимизировать запросы и структуры данных.
- Отслеживать изменения производительности с течением времени.
- Выявлять аномалии и потенциальные проблемы до того, как они повлияют на пользователей.

1. ВНУТРЕННЕЕ УСТРОЙСТВО SLOWLOG

1.1. Измерение времени выполнения:
    - Redis измеряет время выполнения команды от момента получения до отправки ответа.
    - Время включает сетевую задержку? Нет, только время обработки на сервере.
    - Используется высокоточный таймер (монотонные часы) для точности до микросекунд.

1.2. Механизм записи:
    - Когда команда завершается, Redis проверяет время выполнения.
    - Если оно превышает порог slowlog-log-slower-than, запись добавляется в SLOWLOG.
    - Запись хранится в циклическом буфере фиксированного размера (slowlog-max-len).
    - Старые записи перезаписываются новыми при переполнении.

1.3. Структура записи:
    - ID: уникальный инкрементальный идентификатор (сбрасывается при перезапуске).
    - Timestamp: Unix время выполнения (секунды).
    - Duration: время выполнения в микросекундах.
    - Command: массив аргументов (команда + её параметры).
    - Client IP/Port (Redis 7.0+): адрес клиента, отправившего команду.
    - (Redis 7.2+) Client name: имя клиента, если установлено через CLIENT SETNAME.

1.4. Особенности записи:
    - Запись происходит после выполнения команды, поэтому сама запись не влияет на время.
    - Для команд, выполняющихся в Lua-скрипте, логируется только вызов EVAL/EVALSHA,
      а не отдельные команды внутри скрипта.
    - Асинхронные команды (UNLINK, FLUSHALL ASYNC) не логируются, так как их выполнение
      происходит в фоновом потоке, но сама отправка может быть быстрой.

2. НАСТРОЙКА SLOWLOG

2.1. slowlog-log-slower-than:
    - Задаётся в микросекундах (по умолчанию 10000 = 10 мс).
    - Значение 0 включает логирование всех команд (не рекомендуется).
    - Отрицательное значение (-1) отключает SLOWLOG.

    Рекомендации:
    - Для высоконагруженных систем с требованием низкой задержки: 1000–2000 (1–2 мс).
    - Для обычных систем: 5000–10000 (5–10 мс).
    - Для систем, где допустимы большие задержки (например, аналитика): 20000+.

2.2. slowlog-max-len:
    - Максимальное количество записей (по умолчанию 128).
    - Чем больше значение, тем больше памяти потребляется, но больше данных для анализа.
    - Рекомендуется 256–1024 для производственных систем.

2.3. Изменение настроек (без перезапуска):
    CONFIG SET slowlog-log-slower-than 2000
    CONFIG SET slowlog-max-len 512

2.4. Постоянная настройка (в redis.conf):
    slowlog-log-slower-than 2000
    slowlog-max-len 512

3. ТИПЫ МЕДЛЕННЫХ КОМАНД

3.1. Команды, сканирующие все ключи:
    - KEYS: полный перебор всех ключей (O(N)).
    - FLUSHALL / FLUSHDB: удаление всех данных (O(N)).
    - SCAN (с большим COUNT или без паттерна): может быть медленным при N > 10 млн.

3.2. Команды, работающие с большими коллекциями:
    - LRANGE start stop: если диапазон большой, O(N).
    - LREM: удаление элементов по значению, O(N).
    - SMEMBERS: получение всех элементов множества, O(N).
    - HGETALL: получение всех полей хеша, O(N).
    - ZRANGE / ZREVRANGE: получение большого диапазона, O(N).
    - ZREMRANGEBYSCORE / ZREMRANGEBYRANK: удаление большого числа элементов, O(N log N).

3.3. Команды, удаляющие большие данные:
    - DEL / UNLINK: удаление больших коллекций (O(N)).
    - LPOP / RPOP: удаление всех элементов из списка, O(N).

3.4. Команды с высоким потреблением CPU:
    - SORT: сортировка больших списков, O(N log N).
    - EVAL / EVALSHA: Lua-скрипты с большим числом операций.

3.5. Команды, вызывающие репликацию и запись на диск:
    - BGSAVE / BGREWRITEAOF: не логируются, но могут вызвать задержки при fork.

4. АНАЛИЗ SLOWLOG НА ПРАКТИКЕ

4.1. Просмотр записей:
    > SLOWLOG GET 10
    Возвращает последние 10 записей (каждая — массив из 4–6 элементов).

4.2. Очистка:
    > SLOWLOG RESET

4.3. Количество записей:
    > SLOWLOG LEN

4.4. Пример интерпретации:
    1) (integer) 45
    2) (integer) 1634567890
    3) (integer) 12345
    4) 1) "KEYS"
        2) "*"
    5) "127.0.0.1:52345"

    Это означает, что команда KEYS * выполнялась 12.345 мс, от клиента 127.0.0.1:52345.

4.5. Частота записей:
    - Если SLOWLOG быстро заполняется, это признак того, что команды часто превышают порог.
    - Это может указывать на системную проблему, которую нужно решать.

5. КАК ИСПОЛЬЗОВАТЬ SLOWLOG ДЛЯ ОПТИМИЗАЦИИ

5.1. Выявление проблемных паттернов:
    - Команды с большим временем выполнения часто связаны с большими коллекциями.
    - Решение: уменьшите размер коллекции, используйте более эффективные структуры.

5.2. Замена медленных команд:
    - KEYS → SCAN (для итерации).
    - SMEMBERS → SSCAN (для больших множеств).
    - HGETALL → HSCAN (для больших хешей).
    - LRANGE с большим диапазоном → LRANGE с ограничением и пагинацией.

5.3. Оптимизация запросов:
    - Используйте MGET вместо множества GET (сокращает количество RTT).
    - Используйте Pipeline для пакетных операций.
    - Для Lua-скриптов убедитесь, что они не выполняют слишком много операций.

5.4. Мониторинг и алертинг:
    - Настройте оповещения, если появляются записи с duration > 100 мс.
    - Интегрируйте SLOWLOG с системами мониторинга (Prometheus, ELK, Grafana).

5.5. Периодический аудит:
    - Раз в неделю/месяц анализируйте SLOWLOG для выявления трендов.
    - Сравнивайте с предыдущими периодами для оценки эффективности оптимизаций.

6. SLOWLOG В КЛАСТЕРЕ

- Каждый мастер-узел кластера хранит свой локальный SLOWLOG.
- Реплики также имеют свой SLOWLOG (если на них выполняются команды чтения).
- Для сбора данных со всего кластера необходимо опрашивать каждый мастер-узел.

7. ВЛИЯНИЕ SLOWLOG НА ПРОИЗВОДИТЕЛЬНОСТЬ

- SLOWLOG не замедляет команды (запись происходит после выполнения).
- Небольшое потребление памяти (slowlog-max-len * размер записи, обычно < 1 МБ).
- Чтение SLOWLOG (SLOWLOG GET) — быстрая операция O(1).

8. ОШИБКИ И ЛОВУШКИ

Слишком низкий порог (например, 100 мкс) → SLOWLOG будет переполнен и бесполезен.
Слишком высокий порог (например, 100 мс) → пропустите команды, которые всё равно вызывают задержки.
Игнорирование SLOWLOG в кластере → пропустите проблемы на отдельных узлах.
Не учитывание пиковых нагрузок: если в SLOWLOG появляются записи только в часы пик,
   это нормально, если они не слишком долгие.
Использование SLOWLOG как единственного источника информации — комбинируйте с мониторингом
   загрузки CPU, памяти, сети.

9. НОВОВВЕДЕНИЯ В REDIS 7.0+

- Добавлена информация об IP и порте клиента в записи SLOWLOG.
- Возможность логирования медленных команд в Lua (EVAL).
- Улучшена точность измерения времени.

10. ИНТЕГРАЦИЯ С МОНИТОРИНГОМ (PROMETHEUS, GRAFANA)

Пример сбора метрик:
    - slowlog_count — количество записей в SLOWLOG.
    - slowlog_max_duration — максимальное время среди записей.
    - slowlog_avg_duration — среднее время среди записей.
    - slowlog_commands — распределение по командам.

Можно экспортировать эти метрики через экспортер или писать их в систему логирования.

11. ПРАКТИЧЕСКИЙ ЧЕК-ЛИСТ ДЛЯ ПРОДАКШНА

1. Настройте порог slowlog-log-slower-than = 2000 (2 мс) для чувствительных систем.
2. Установите slowlog-max-len = 256–512.
3. Регулярно проверяйте SLOWLOG через Cron-задачу или встроенный мониторинг.
4. Настройте оповещение при появлении записей > 100 мс.
5. Интегрируйте SLOWLOG с централизованной системой логирования (ELK, Loki).
6. Проводите аудит SLOWLOG еженедельно.
7. Для кластера собирайте данные с каждого мастер-узла.
8. При обнаружении медленных команд:
   - Замените KEYS на SCAN.
   - Ограничьте размер возвращаемых данных (LRANGE, SMEMBERS, HGETALL).
   - Используйте более эффективные структуры данных.
9. Сохраняйте историю SLOWLOG для анализа трендов.
10. Тестируйте оптимизации и сравнивайте результаты до и после.

12. ИТОГИ

SLOWLOG — это незаменимый инструмент для поддержания производительности Redis.
Он помогает выявлять проблемные команды, оценивать эффективность оптимизаций
и предотвращать деградацию системы. Правильная настройка и регулярный анализ
SLOWLOG позволяют держать задержки под контролем и обеспечивать стабильную
работу приложений.

Запомните: медленные команды — это не только проблема, но и возможность улучшить
систему. Используйте SLOWLOG как источник знаний о том, как ваше приложение
взаимодействует с Redis, и оптимизируйте его.

13. СВЯЗЬ С GO (go-redis) — МЕТОДЫ ДЛЯ РАБОТЫ СО SLOWLOG

Библиотека go-redis предоставляет удобные методы для работы со SLOWLOG.
Их понимание обязательно для продакшен-разработки.

13.1. SlowLogGet — получение записей SLOWLOG
    func (c *Client) SlowLogGet(ctx context.Context, n int64) ([]SlowLogEntry, error)
    - Параметры:
        * ctx — контекст для таймаутов.
        * n — количество последних записей (если n <= 0, возвращаются все записи).
    - Возвращает: срез структур SlowLogEntry.
    - Структура SlowLogEntry:
        type SlowLogEntry struct {
            ID         int64       // уникальный идентификатор
            Time       int64       // Unix timestamp (секунды)
            Duration   int64       // время выполнения в микросекундах
            Args       []interface{} // аргументы команды (первый — имя команды)
            ClientAddr string      // IP:Port клиента (Redis 7.0+)
            ClientName string      // имя клиента (если установлено)
        }
    - Пример:
        entries, err := client.SlowLogGet(ctx, 10).Result()
        if err != nil { ... }
        for _, e := range entries {
            fmt.Printf("Команда: %v, Время: %d мкс\n", e.Args, e.Duration)
        }

13.2. SlowLogLen — количество записей в SLOWLOG
    В go-redis нет прямой функции SlowLogLen, но её можно получить через Do:
        count, err := client.Do(ctx, "SLOWLOG", "LEN").Int()
    - Возвращает количество записей (int64).

13.3. SlowLogReset — очистка SLOWLOG
    Аналогично, через Do:
        err := client.Do(ctx, "SLOWLOG", "RESET").Err()
    - Удаляет все записи и сбрасывает счётчик ID.

13.4. Настройка параметров SLOWLOG:
    - Конфигурация осуществляется через ConfigSet и ConfigGet.
    - SlowlogLogSlowerThan — порог в микросекундах:
        err := client.ConfigSet(ctx, "slowlog-log-slower-than", "5000").Err()
    - SlowlogMaxLen — максимальное количество записей:
        err := client.ConfigSet(ctx, "slowlog-max-len", "256").Err()
    - Чтение настроек:
        val, err := client.ConfigGet(ctx, "slowlog-log-slower-than").Result()
        // val — map[string]string, например, {"slowlog-log-slower-than": "5000"}

13.5. Получение информации о клиенте (ClientAddr, ClientName):
    - Эти поля доступны в SlowLogEntry (в Redis 7.0+), но только если клиент
      установил имя через CLIENT SETNAME.
    - Установка имени клиента в go-redis:
        client.ClientSetName(ctx, "my-app").Err()
    - Это полезно для идентификации источника медленных запросов.

13.6. Обработка ошибок:
    - Все методы могут вернуть ошибку (таймаут, сеть, синтаксис).
    - Рекомендуется всегда обрабатывать ошибки и логировать их.
    - При использовании контекста можно установить таймаут на получение SLOWLOG.

13.7. Практические рекомендации по использованию методов в продакшне:
    - Регулярно (раз в минуту или час) вызывайте SlowLogGet для мониторинга.
    - Используйте фильтрацию на клиенте (например, по Duration > порога).
    - При обнаружении проблемных команд, логируйте их в отдельный файл или отправляйте в систему оповещений.
    - Настройте сбор метрик из SLOWLOG для Grafana/Prometheus.
    - Используйте ClientSetName для каждого микросервиса, чтобы знать, какой сервис создаёт нагрузку.

13.8. Пример полного мониторинга с использованием методов:
    - Ежеминутная проверка: GetSlowLog(10) → анализ Duration → если > alertThreshold → alert.
    - Ежечасная проверка: GetSlowLog(1000) → агрегация по командам → отчёт.
    - Ежедневная очистка: если записей > maxLen → Reset (или перезапись автоматическая).

14. ИТОГИ ПО МЕТОДАМ

Основные методы go-redis для работы со SLOWLOG:
- SlowLogGet(n) — получить N последних записей.
- Do("SLOWLOG", "LEN") — количество записей.
- Do("SLOWLOG", "RESET") — очистить лог.
- ConfigSet / ConfigGet — управление порогом и размером.
- ClientSetName — установка имени клиента для идентификации в записях.

Знание этих методов и их правильное применение позволяет эффективно мониторить
и оптимизировать работу Redis в продакшн-окружении.
*/

var (
	rdb    *redis.Client
	ctx    = context.Background()
	logger = log.Default()
)

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		PoolSize:     10,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Redis не отвечает: %v", err)
	}
}

// Подготовка данных для демонстрации медленных команд
func prepareTestData() {
	// Создаём 2000 ключей, чтобы KEYS была медленной
	for i := 0; i < 2000; i++ {
		rdb.Set(ctx, fmt.Sprintf("test:%d", i), i, 0)
	}
	// Создаём большой список для LRANGE
	for i := 0; i < 1000; i++ {
		rdb.RPush(ctx, "biglist", fmt.Sprintf("item_%d", i))
	}
	// Создаём большой хеш для HGETALL
	for i := 0; i < 500; i++ {
		rdb.HSet(ctx, "bighash", fmt.Sprintf("field_%d", i), fmt.Sprintf("value_%d", i))
	}
}

func main() {
	fmt.Println("=== SLOWLOG: ПРОДАКШН-ПРИМЕРЫ ===\n")

	// Создаём тестовые данные для демонстрации
	prepareTestData()

	Primer1()
	Primer2()
	Primer3()
	Primer4()
	Primer5()
	Primer6()
	Primer7()
	Primer8()
}

// 1. НАСТРОЙКА SLOWLOG С ОПТИМАЛЬНЫМИ ПАРАМЕТРАМИ
func Primer1() {
	fmt.Println("--- 1. Настройка SLOWLOG для продакшна ---")

	// Читаем текущие настройки
	threshold, err := rdb.ConfigGet(ctx, "slowlog-log-slower-than").Result()
	if err != nil {
		logger.Printf("ConfigGet ошибка: %v", err)
	} else {
		fmt.Printf("Текущий порог: %s мкс\n", threshold["slowlog-log-slower-than"])
	}

	maxLen, err := rdb.ConfigGet(ctx, "slowlog-max-len").Result()
	if err != nil {
		logger.Printf("ConfigGet ошибка: %v", err)
	} else {
		fmt.Printf("Текущий размер: %s записей\n", maxLen["slowlog-max-len"])
	}

	// Устанавливаем оптимальные значения для продакшна (2 мс, 256 записей)
	err = rdb.ConfigSet(ctx, "slowlog-log-slower-than", "2000").Err()
	if err != nil {
		logger.Printf("ConfigSet ошибка: %v", err)
	} else {
		fmt.Println("Установлен порог: 2000 мкс (2 мс)")
	}

	err = rdb.ConfigSet(ctx, "slowlog-max-len", "256").Err()
	if err != nil {
		logger.Printf("ConfigSet ошибка: %v", err)
	} else {
		fmt.Println("Установлен размер: 256 записей")
	}

	// Перепроверяем
	thresholdNew, _ := rdb.ConfigGet(ctx, "slowlog-log-slower-than").Result()
	maxLenNew, _ := rdb.ConfigGet(ctx, "slowlog-max-len").Result()
	fmt.Printf("Новый порог: %s мкс\n", thresholdNew["slowlog-log-slower-than"])
	fmt.Printf("Новый размер: %s записей\n", maxLenNew["slowlog-max-len"])
}

// 2. ПОЛУЧЕНИЕ SLOWLOG С ФИЛЬТРАЦИЕЙ ПО ВРЕМЕНИ
func Primer2() {
	fmt.Println("\n--- 2. Получение записей SLOWLOG с фильтром по времени ---")

	// Выполняем медленную команду для демонстрации
	rdb.Keys(ctx, "test:*")
	rdb.LRange(ctx, "biglist", 0, -1)

	entries, err := rdb.SlowLogGet(ctx, 10).Result()
	if err != nil {
		logger.Printf("SlowLogGet ошибка: %v", err)
		return

	}

	// Фильтруем записи, которые > 5 мс (5000 мкс)
	var filtered []redis.SlowLogEntry
	for _, e := range entries {
		if e.Duration > 5000 {
			filtered = append(filtered, e)
		}
	}
	fmt.Printf("Всего записей: %d, из них >5 мс: %d\n", len(entries), len(filtered))
	for _, e := range filtered {
		fmt.Printf("  Команда: %v, Duration: %.2f мс\n", e.Args, float64(e.Duration)/1000)
	}
}

// 3. ОЧИСТКА И УПРАВЛЕНИЕ SLOWLOG
func Primer3() {
	fmt.Println("\n--- 3. Очистка и управление размером SLOWLOG ---")

	// Проверяем длину
	lenBefore, err := rdb.Do(ctx, "SLOWLOG", "LEN").Int()
	if err != nil {
		logger.Printf("SLOWLOG LEN ошибка: %v", err)
	} else {
		fmt.Printf("Записей до очистки: %d\n", lenBefore)
	}

	// Очищаем только если записей > 100 (чтобы не терять данные при малом количестве)
	if lenBefore > 100 {
		err = rdb.Do(ctx, "SLOWLOG", "RESET").Err()
		if err != nil {
			logger.Printf("SLOWLOG RESET ошибка: %v", err)
		} else {
			fmt.Println("SLOWLOG очищен (превышен лимит 100)")
		}
	} else {
		fmt.Printf("SLOWLOG не очищен (записей %d < 100)\n", lenBefore)
	}
	lenAfter, _ := rdb.Do(ctx, "SLOWLOG", "LEN").Int()
	fmt.Printf("Записей после: %d\n", lenAfter)
}

// 4. МОНИТОРИНГ SLOWLOG И АЛЕРТИНГ
func Primer4() {
	fmt.Println("\n--- 4. Мониторинг SLOWLOG и алертинг ---")

	// Определяем порог алерта (в микросекундах)
	const alertThresholdMs = 100 // 100 мс
	alertThresholdUs := alertThresholdMs * 1000

	// Проверяем последние 20 записей
	entries, err := rdb.SlowLogGet(ctx, 20).Result()
	if err != nil {
		logger.Printf("SlowLogGet ошибка: %v", err)
		return
	}

	var alerts []string
	for _, e := range entries {
		if e.Duration.Microseconds() > int64(alertThresholdUs) {
			cmdStr := fmt.Sprintf("%v", e.Args)
			// Ограничим длину для читаемости
			if len(cmdStr) > 100 {
				cmdStr = cmdStr[:100] + "..."
			}
			alerts = append(alerts, fmt.Sprintf("Команда: %s, Duration: %.2f мс, Time: %s",
				cmdStr, float64(e.Duration)/1000, time.Unix(e.Time.Unix(), 0).Format("15:04:05")))
		}
	}

	if len(alerts) > 0 {
		fmt.Printf("⚠️  Обнаружено %d медленных команд > %d мс:\n", len(alerts), alertThresholdMs)
		for _, msg := range alerts {
			fmt.Println("  ", msg)
		}
		// Здесь можно отправить алерт в Slack, Telegram, Email и т.д.
	} else {
		fmt.Printf("✅ Нет команд, превышающих %d мс\n", alertThresholdMs)
	}
}

// 5. ПОИСК ПРОБЛЕМНЫХ КОМАНД (KEYS, LRANGE, HGETALL)
func Primer5() {
	fmt.Println("\n--- 5. Поиск проблемных команд (KEYS, LRANGE, HGETALL) ---")

	// Выполняем несколько медленных команд
	rdb.Keys(ctx, "test:*")
	rdb.LRange(ctx, "biglist", 0, -1)
	rdb.HGetAll(ctx, "bighash")

	entries, err := rdb.SlowLogGet(ctx, 50).Result()
	if err != nil {
		logger.Printf("Ошибка: %v", err)
		return
	}

	// Список подозрительных команд
	suspiciousCommands := map[string]bool{
		"KEYS":     true,
		"LRANGE":   true,
		"HGETALL":  true,
		"SMEMBERS": true,
		"ZRANGE":   true,
		"SORT":     true,
	}

	found := 0
	for _, e := range entries {
		cmd := e.Args[0]
		if suspiciousCommands[cmd] {
			found++
			fmt.Printf("Найдена подозрительная команда: %v (%.2f мс)\n",
				e.Args, float64(e.Duration)/1000)
		}
	}
	if found == 0 {
		fmt.Println("Подозрительных команд не найдено в SLOWLOG")
	}
}

// 6. ПРОФИЛИРОВАНИЕ ПРИЛОЖЕНИЯ (СРАВНЕНИЕ ВЕРСИЙ)
func Primer6() {
	fmt.Println("\n--- 6. Профилирование приложения через SLOWLOG ---")

	// Функция для сбора статистики по командам
	collectStats := func() map[string]float64 {
		entries, err := rdb.SlowLogGet(ctx, 100).Result()
		if err != nil {
			return nil
		}
		stats := make(map[string]float64)
		counts := make(map[string]int)
		for _, e := range entries {
			cmd := e.Args[0]
			counts[cmd]++
			stats[cmd] += float64(e.Duration) / 1000 // в миллисекундах
		}
		// Усредняем
		for cmd, total := range stats {
			stats[cmd] = total / float64(counts[cmd])
		}
		return stats
	}

	// Имитация разных версий приложения: выполняем команды
	simulateAppVersion := func(version string) {
		fmt.Printf("Симуляция версии %s...\n", version)
		// Версия 1: много KEYS
		if version == "v1" {
			for i := 0; i < 5; i++ {
				rdb.Keys(ctx, "test:*")
			}
		} else if version == "v2" {
			// Версия 2: использует SCAN вместо KEYS
			for i := 0; i < 5; i++ {
				iter := rdb.Scan(ctx, 0, "test:*", 100).Iterator()
				for iter.Next(ctx) {
					// Просто итерируем
				}
			}
		}
	}

	// Чистим SLOWLOG перед профилированием
	rdb.Do(ctx, "SLOWLOG", "RESET")

	simulateAppVersion("v1")
	statsV1 := collectStats()
	if statsV1 != nil {
		fmt.Printf("v1 среднее время KEYS: %.2f мс\n", statsV1["KEYS"])
	}

	// Чистим для второй версии
	rdb.Do(ctx, "SLOWLOG", "RESET")
	simulateAppVersion("v2")
	statsV2 := collectStats()
	if statsV2 != nil {
		fmt.Printf("v2 среднее время KEYS (SCAN): команда не будет в SLOWLOG, т.к. она не блокирует\n")
		fmt.Printf("v2 записи: %v\n", statsV2)
	}

	// Сравнение
	if statsV1 != nil && statsV2 != nil {
		if statsV1["KEYS"] > 0 && statsV2["KEYS"] == 0 {
			fmt.Println("Переход на SCAN устранил медленные KEYS")
		}
	}
}

// 7. ИНТЕГРАЦИЯ С МЕТРИКАМИ (PROMETHEUS
func Primer7() {
	fmt.Println("\n--- 7. Интеграция с метриками (Prometheus экспорт) ---")

	// Сбор метрик для экспорта
	type SlowMetrics struct {
		TotalEntries  int64
		MaxDuration   int64   // микросекунды
		AvgDuration   float64 // микросекунды
		CommandCounts map[string]int
	}

	collectMetrics := func() (*SlowMetrics, error) {
		lent, err := rdb.Do(ctx, "SLOWLOG", "LEN").Int64()
		if err != nil {
			return nil, err
		}
		if lent == 0 {
			return &SlowMetrics{TotalEntries: 0, MaxDuration: 0, AvgDuration: 0}, nil
		}
		entries, err := rdb.SlowLogGet(ctx, lent).Result()
		if err != nil {
			return nil, err
		}
		var totalDuration int64
		var maxDuration int64
		cmdCounts := make(map[string]int)
		for _, e := range entries {
			totalDuration += e.Duration.Microseconds()
			if e.Duration.Microseconds() > maxDuration {
				maxDuration = e.Duration.Microseconds()
			}
			cmd := e.Args[0]
			cmdCounts[cmd]++
		}
		avgDuration := float64(totalDuration) / float64(lent)
		return &SlowMetrics{
			TotalEntries:  lent,
			MaxDuration:   maxDuration,
			AvgDuration:   avgDuration,
			CommandCounts: cmdCounts,
		}, nil
	}

	// Создаём медленные команды
	rdb.Keys(ctx, "test:*")

	metrics, err := collectMetrics()
	if err != nil {
		logger.Printf("Ошибка сбора метрик: %v", err)
	} else {
		fmt.Printf("Метрики SLOWLOG:\n")
		fmt.Printf("  Всего записей: %d\n", metrics.TotalEntries)
		fmt.Printf("  Макс. время: %.2f мс\n", float64(metrics.MaxDuration)/1000)
		fmt.Printf("  Среднее время: %.2f мс\n", metrics.AvgDuration/1000)
		fmt.Printf("  Распределение по командам: %v\n", metrics.CommandCounts)

		// Здесь можно экспортировать в Prometheus через метрики:
		// prometheus.NewGauge(...).Set(float64(metrics.TotalEntries))
		// prometheus.NewHistogram(...).Observe(float64(metrics.AvgDuration))
	}
}

// 8. АВТОМАТИЧЕСКАЯ ГЕНЕРАЦИЯ ОТЧЁТА ПО SLOWLOG
func Primer8() {
	fmt.Println("\n--- 8. Автоматическая генерация отчёта по SLOWLOG ---")

	generateReport := func() string {
		entries, err := rdb.SlowLogGet(ctx, 100).Result()
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err)
		}
		if len(entries) == 0 {
			return "SLOWLOG пуст"
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("Отчёт SLOWLOG (%s)\n", time.Now().Format("2006-01-02 15:04:05")))
		builder.WriteString(fmt.Sprintf("Всего записей: %d\n\n", len(entries)))

		// Группируем по командам
		cmdStats := make(map[string]struct {
			Count int
			Total int64
			Max   int64
			Min   int64
		})
		for _, e := range entries {
			cmd := e.Args[0]
			stat := cmdStats[cmd]
			stat.Count++
			stat.Total += e.Duration.Milliseconds()
			if e.Duration.Microseconds() > stat.Max {
				stat.Max = e.Duration.Microseconds()
			}
			if stat.Min == 0 || e.Duration.Microseconds() < stat.Min {
				stat.Min = e.Duration.Microseconds()
			}
			cmdStats[cmd] = stat
		}

		builder.WriteString("Статистика по командам:\n")
		for cmd, stat := range cmdStats {
			avg := float64(stat.Total) / float64(stat.Count)
			builder.WriteString(fmt.Sprintf("  %s: кол-во=%d, сред=%.2f мс, макс=%.2f мс, мин=%.2f мс\n",
				cmd, stat.Count, avg/1000, float64(stat.Max)/1000, float64(stat.Min)/1000))
		}

		// Топ-5 самых медленных команд
		builder.WriteString("\nТоп-5 самых медленных команд:\n")
		// Копируем записи в слайс и сортируем (для простоты берём первые 5)
		for i, e := range entries {
			if i >= 5 {
				break
			}
			builder.WriteString(fmt.Sprintf("  %v: %.2f мс\n", e.Args, float64(e.Duration)/1000))
		}

		return builder.String()
	}

	// Выполняем медленные команды, чтобы было что отображать
	rdb.Keys(ctx, "test:*")
	rdb.LRange(ctx, "biglist", 0, 100) // ограничим, чтобы не слишком долго

	report := generateReport()
	fmt.Println(report)
}
