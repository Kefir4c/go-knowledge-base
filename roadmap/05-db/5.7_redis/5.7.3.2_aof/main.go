package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 3.2. AOF (APPEND-ONLY FILE) — РАСШИРЕННАЯ ТЕОРИЯ

0. ВВЕДЕНИЕ: ЗАЧЕМ AOF?

RDB даёт компактные снимки, но теряет изменения между ними.
AOF решает эту проблему: записывает каждую операцию записи в журнал.
Это позволяет восстановить данные с минимальной потерей (до 1 секунды при everysec).
В production часто используют RDB + AOF одновременно: RDB для быстрого восстановления,
AOF для точности.

1. ВНУТРЕННЕЕ УСТРОЙСТВО AOF

1.1. Формат AOF-файла:
    AOF — это текстовый файл, содержащий команды в протоколе Redis.
    Каждая команда представлена в виде:
        *<количество аргументов>\r\n$<длина аргумента1>\r\n<аргумент1>\r\n...
    Пример: *3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$6\r\nmyvalue\r\n
    Это позволяет воспроизводить команды при восстановлении.

1.2. Процесс записи:
    1. Клиент отправляет команду (SET, HSET, DEL, ...).
    2. Redis выполняет команду и изменяет память.
    3. Команда записывается в буфер (aof_buf).
    4. При срабатывании таймера (или сразу при always) буфер сбрасывается на диск.
    5. Если запись на диск завершена успешно, Redis отвечает клиенту.

    Важно: команда сначала выполняется, потом записывается в AOF.
    Это гарантирует, что AOF не будет содержать команды, которые не удалось выполнить.

1.3. Буферизация (aof_buf):
    Для повышения производительности Redis не записывает каждую команду на диск сразу.
    Команды накапливаются в буфере, а затем сбрасываются согласно appendfsync.
    Это позволяет уменьшить количество системных вызовов fsync.

1.4. Синхронизация с диском (appendfsync):
    - always: fsync после каждой команды. Самый медленный, но самый надёжный.
    - everysec: fsync раз в секунду. Баланс скорости и надёжности (по умолчанию).
    - no: fsync не вызывается. ОС сама решает, когда записать данные.
      Быстро, но риск потери данных (при падении могут потеряться несколько секунд).

    Внутренний механизм: при everysec Redis каждую секунду вызывает fsync.
    Если в течение секунды пришло много команд, они будут записаны в буфер,
    а затем сброшены разом.

2. ПЕРЕЗАПИСЬ AOF (BGREWRITEAOF)

AOF растёт бесконечно, т.к. каждая новая команда добавляется в конец.
Перезапись создаёт новый AOF-файл, содержащий только текущее состояние данных.

2.1. Принцип работы (fork + COW):
    1. Redis вызывает fork() -> дочерний процесс.
    2. Дочерний процесс строит новый AOF, содержащий все текущие данные.
       Он читает данные из памяти и записывает команды, необходимые для их воссоздания.
    3. Родитель продолжает записывать новые команды в старый AOF.
    4. Когда дочерний процесс завершает перезапись, он уведомляет родителя.
    5. Родитель добавляет команды, которые пришли во время перезаписи,
       в новый AOF (это небольшой буфер).
    6. Новый AOF заменяет старый (атомарно через rename).

2.2. Когда запускается перезапись:
    Автоматически:
    - Размер AOF превысил auto-aof-rewrite-min-size.
    - И размер вырос на auto-aof-rewrite-percentage процентов
      относительно размера после последней перезаписи.

    Вручную: команда BGREWRITEAOF.

2.3. Нагрузка при перезаписи:
    - Fork: копирование таблиц страниц (быстро, но при больших данных может занять время).
    - COW: копирование изменяемых страниц (память может временно вырасти на 20-30%).
    - Запись на диск: дочерний процесс пишет новый файл (I/O нагрузка).
    - CPU: создание нового AOF (сериализация данных).

2.4. Оптимизация: aof-use-rdb-preamble (Redis 6+)
    При включении, Redis в начале AOF записывает снимок RDB (в бинарном виде),
    а затем добавляет команды, произошедшие после снимка.
    Преимущества:
    - Ускорение восстановления (RDB загружается быстрее).
    - Уменьшение размера AOF (RDB сжатый).
    - Сохранение всех преимуществ AOF (минимальная потеря данных).

3. ВОССТАНОВЛЕНИЕ ИЗ AOF

3.1. Процесс восстановления при старте:
    1. Redis проверяет, включён ли AOF (appendonly yes).
    2. Если AOF существует, он загружается и воспроизводится (execute commands).
    3. Если AOF отсутствует, но есть RDB, загружается RDB.
    4. Если AOF повреждён, Redis может не запуститься.

3.2. Инструменты для работы с AOF:
    - redis-check-aof: проверка целостности AOF-файла.
      redis-check-aof --fix appendonly.aof  # удаляет повреждённые части.
    - redis-cli --pipe: массовая загрузка команд (может использоваться для импорта).

3.3. Восстановление при повреждении:
    Если AOF повреждён, и Redis не запускается:
    1. Скопируйте повреждённый AOF.
    2. Запустите redis-check-aof --fix.
    3. Если не помогает, удалите AOF и загрузитесь из RDB (потеря части данных).
    4. В крайнем случае, можно удалить AOF и запустить Redis без персистентности.

4. ВЛИЯНИЕ AOF НА ПРОИЗВОДИТЕЛЬНОСТЬ И ЗАДЕРЖКИ

4.1. appendfsync always:
    - Каждая команда вызывает fsync — задержка может быть 100-1000 мкс.
    - При высокой нагрузке это может снизить пропускную способность в 10-100 раз.
    - Используется только для критичных данных (банковские операции).

4.2. appendfsync everysec (рекомендуемый):
    - fsync раз в секунду, задержка минимальна.
    - Даже при падении теряется только до 1 секунды данных.
    - Нагрузка на диск: ~1 операция fsync в секунду.
    - Для большинства систем это оптимально.

4.3. appendfsync no:
    - fsync не вызывается, полагается на ОС.
    - При падении могут потеряться данные за несколько секунд.
    - Используется, когда скорость важнее надёжности (например, кэш).

4.4. Влияние на задержки (latency):
    - При everysec Redis раз в секунду синхронизирует буфер с диском.
    - Если во время этой синхронизации приходит команда, она может подождать.
    - Пиковые задержки могут достигать 1-2 мс, что обычно приемлемо.

4.5. no-appendfsync-on-rewrite:
    - Запрещает синхронизацию во время BGREWRITEAOF, чтобы снизить нагрузку на диск.
    - По умолчанию yes (т.е. запрещает), что безопаснее.

5. СРАВНЕНИЕ AOF И RDB: КОГДА ЧТО ВЫБРАТЬ?

┌──────────────────────────┬────────────────────────────────────────────────────┐
│ Сценарий                 │ Рекомендация                                       │
├──────────────────────────┼────────────────────────────────────────────────────┤
│ Кэш, данные не критичны  │ Только RDB (редкие снимки)                         │
├──────────────────────────┼────────────────────────────────────────────────────┤
│ Сессии пользователей     │ AOF (everysec) + RDB (для быстрого старта)         │
├──────────────────────────┼────────────────────────────────────────────────────┤
│ Финансовые транзакции    │ AOF always + RDB (для бэкапа)                      │
├──────────────────────────┼────────────────────────────────────────────────────┤
│ Репликация               │ RDB для начального копирования                     │
├──────────────────────────┼────────────────────────────────────────────────────┤
│ Высокая нагрузка на диск │ AOF no (или только RDB)                            │
└──────────────────────────┴────────────────────────────────────────────────────┘

6. ТЮНИНГ AOF ДЛЯ ПРОДАКШНА

6.1. Рекомендуемые настройки:
    appendonly yes
    appendfsync everysec
    no-appendfsync-on-rewrite yes
    auto-aof-rewrite-percentage 100
    auto-aof-rewrite-min-size 64mb
    aof-use-rdb-preamble yes

6.2. Мониторинг:
    - aof_current_size: текущий размер AOF.
    - aof_base_size: размер после последней перезаписи.
    - aof_rewrite_in_progress: идёт ли перезапись.
    - aof_last_rewrite_time_sec: сколько времени заняла последняя перезапись.
    - aof_last_bgrewrite_status: статус последней перезаписи (ok/err).

6.3. При высокой нагрузке:
    - Увеличьте auto-aof-rewrite-min-size (чтобы реже перезаписывать).
    - Убедитесь, что диск быстрый (SSD).
    - Рассмотрите использование реплики для выполнения BGREWRITEAOF.

7. СВЯЗЬ С GO (РЕЗЮМЕ)

- Нет прямого API для настройки AOF в go-redis (это серверная конфигурация).
- Но вы можете читать настройки через ConfigGet.
- Мониторить состояние через Info.
- Инициировать BGREWRITEAOF через Do("BGREWRITEAOF").
- При использовании AOF always важно настроить таймауты клиента (WriteTimeout),
  т.к. команды могут выполняться дольше.

8. ИТОГИ ПО AOF

- AOF обеспечивает минимальную потерю данных (до 1 сек).
- Основной недостаток — больший размер и нагрузка на диск.
- Используйте RDB + AOF для максимальной надёжности.
- Настройка appendfsync everysec — золотая середина.
- Регулярно мониторьте размер AOF и перезаписывайте при необходимости.

9. ПРОДАКШН-СОВЕТЫ (ЧЕК-ЛИСТ)

Включите AOF на всех production-серверах (если данные критичны).
Установите appendfsync everysec.
Настройте auto-aof-rewrite-percentage и min-size под вашу нагрузку.
Включите aof-use-rdb-preamble (для быстрого восстановления).
Регулярно проверяйте целостность AOF (в скриптах бэкапа).
Мониторьте размер AOF и алертьте при его быстром росте.
Имейте план восстановления при повреждении AOF (redis-check-aof).
Рассмотрите использование реплики для выполнения перезаписи.
Тестируйте восстановление из AOF в тестовой среде.
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
	fmt.Println("=== AOF (APPEND-ONLY FILE) ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
}

// 1. Проверка настроек AOF
func primer1() {
	fmt.Println("--- 1. Проверка настроек AOF ---")

	appendonly, _ := rdb.ConfigGet(ctx, "appendonly").Result()
	fmt.Printf("appendonly: %s\n", appendonly["appendonly"])

	appendfsync, _ := rdb.ConfigGet(ctx, "appendfsync").Result()
	fmt.Printf("appendfsync: %s\n", appendfsync["appendfsync"])

	appendfilename, _ := rdb.ConfigGet(ctx, "appendfilename").Result()
	fmt.Printf("appendfilename: %s\n", appendfilename["appendfilename"])

	autoAofRewritePercentage, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-percentage").Result()
	fmt.Printf("auto-aof-rewrite-percentage: %s\n", autoAofRewritePercentage["auto-aof-rewrite-percentage"])

	autoAofRewriteMinSize, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-min-size").Result()
	fmt.Printf("auto-aof-rewrite-min-size: %s\n", autoAofRewriteMinSize["auto-aof-rewrite-min-size"])

	aofUseRdbPreamble, _ := rdb.ConfigGet(ctx, "aof-use-rdb-preamble").Result()
	fmt.Printf("aof-use-rdb-preamble: %s\n", aofUseRdbPreamble["aof-use-rdb-preamble"])

	// Проверяем, существует ли AOF-файл
	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	aofPath := fmt.Sprintf("%s%s", dir["dir"], appendfilename["appendfilename"])
	if _, err := os.Stat(aofPath); err != nil {
		fmt.Printf("AOF-файл не найден (возможно отключён)\n")
	}
	fmt.Printf("AOF-файл существует: %s\n", aofPath)
}

// 2. Измерение задержки при разных appendfsync
func primer2() {
	fmt.Println("--- 2. Измерение задержки при разных appendfsync ---")

	// Сохраняем текущую настройку
	oldSync, _ := rdb.ConfigGet(ctx, "appendfsync").Result()
	defer rdb.ConfigSet(ctx, "appendfsync", oldSync["appendfsync"])

	testAppendfsync := func(syncPolicy string) time.Duration {
		rdb.ConfigSet(ctx, "appendfsync", syncPolicy)
		// Даём время на применение
		time.Sleep(100 * time.Millisecond)

		// Измеряем время 100 SET
		start := time.Now()
		for i := 0; i < 100; i++ {
			rdb.Set(ctx, fmt.Sprintf("perf:%s:%d", syncPolicy, i), "value", 0)
		}
		return time.Since(start)
	}

	policies := []string{"always", "everysec", "no"}
	for _, p := range policies {
		elapsed := testAppendfsync(p)
		fmt.Printf("appendfsync = %-7s : 100 SET за %v\n", p, elapsed)
	}

	// Очистка тестовых ключей
	keys, _ := rdb.Keys(ctx, "perf:*").Result()
	rdb.Del(ctx, keys...)
}

// 3. Мониторинг размера AOF
func primer3() {
	fmt.Println("--- 3. Мониторинг размера AOF ---")

	// Получаем путь к AOF
	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	filename, _ := rdb.ConfigGet(ctx, "appendfilename").Result()
	aofPath := fmt.Sprintf("%s/%s", dir["dir"], filename["appendfilename"])

	if info, err := os.Stat(aofPath); err == nil {
		sizeMB := float64(info.Size()) / (1024 * 1024)
		fmt.Printf("Текущий размер AOF: %.2f MB\n", sizeMB)
	} else {
		fmt.Println("AOF-файл не найден (возможно отключён)")
	}

	// Получаем метрики через INFO persistence
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	aofCurrentSize, _ := strconv.Atoi(info["persistence"]["aof_current_size"])
	aofBaseSize, _ := strconv.Atoi(info["persistence"]["aof_base_size"])
	pendingRewrite := info["persistence"]["aof_rewrite_in_progress"]

	fmt.Printf("aof_current_size: %d байт (%.2f MB)\n", aofCurrentSize, float64(aofCurrentSize)/(1024*1024))
	fmt.Printf("aof_base_size: %d байт (%.2f MB)\n", aofBaseSize, float64(aofBaseSize)/(1024*1024))
	fmt.Printf("aof_rewrite_in_progress: %s\n", pendingRewrite)

	// Проверяем, не пора ли перезаписывать
	rewritePercent, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-percentage").Result()
	minSize, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-min-size").Result()
	fmt.Printf("Условия автоперезаписи: процент=%s, мин. размер=%s\n",
		rewritePercent["auto-aof-rewrite-percentage"],
		minSize["auto-aof-rewrite-min-size"])
}

// 4. Ручной вызов BGREWRITEAOF
func primer4() {
	fmt.Println("--- 4. Ручной вызов BGREWRITEAOF ---")

	// Проверяем, не выполняется ли уже перезапись
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	if info["persistence"]["aof_rewrite_in_progress"] == "1" {
		fmt.Println(" Перезапись уже выполняется, пропускаем")
		return
	}

	err := rdb.Do(ctx, "BGREWRITEAOF").Err()
	if err != nil {
		fmt.Printf("Ошибка при вызове BGREWRITEAOF: %v\n", err)
		return
	}
	fmt.Println("Команда BGREWRITEAOF отправлена")

	// Ждём завершения (можно мониторить статус)
	time.Sleep(2 * time.Second)
	info2, _ := rdb.InfoMap(ctx, "persistence").Result()
	status := info2["persistence"]["aof_rewrite_in_progress"]
	fmt.Printf("Текущий статус: aof_rewrite_in_progress = %s\n", status)
}

// 5. Восстановление из AOF (симуляция)
func primer5() {
	fmt.Println("--- 5. Восстановление из AOF (симуляция) ---")

	// Сохраняем текущее состояние AOF
	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	filename, _ := rdb.ConfigGet(ctx, "appendfilename").Result()
	aofPath := fmt.Sprintf("%s/%s", dir["dir"], filename["appendfilename"])

	fmt.Println("Симуляция процесса восстановления из AOF:")
	fmt.Println("1. Redis падает (имитация)")
	fmt.Println("2. При перезапуске Redis проверяет наличие AOF-файла")
	fmt.Println("3. Если AOF найден, он загружается и все команды воспроизводятся")

	// Проверяем, существует ли AOF
	if _, err := os.Stat(aofPath); err == nil {
		fmt.Printf("AOF-файл найден: %s\n", aofPath)
		fmt.Println("Восстановление из AOF будет выполнено при старте")
	} else {
		fmt.Println("AOF-файл не найден. Будет использован RDB (если есть)")
	}

	// В реальности можно проверить, был ли AOF загружен успешно
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	loading := info["persistence"]["loading"]
	fmt.Printf("Статус загрузки (при старте): loading=%s\n", loading)
}

// 6. Проверка условий автоматической перезаписи AOF
func primer6() {
	fmt.Println("--- 6. Проверка условий автоматической перезаписи ---")

	percent, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-percentage").Result()
	minSize, _ := rdb.ConfigGet(ctx, "auto-aof-rewrite-min-size").Result()
	fmt.Printf("Условия: процент=%s, мин. размер=%s\n",
		percent["auto-aof-rewrite-percentage"],
		minSize["auto-aof-rewrite-min-size"])

	// Получаем текущий размер AOF
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	currentSize, _ := strconv.ParseInt(info["persistence"]["aof_current_size"], 10, 64)
	baseSize, _ := strconv.ParseInt(info["persistence"]["aof_base_size"], 10, 64)

	fmt.Printf("Текущий размер AOF: %d байт\n", currentSize)
	fmt.Printf("Базовый размер (последней перезаписи): %d байт\n", baseSize)

	// Проверяем, нужно ли перезаписывать
	if currentSize < baseSize {
		fmt.Println("Tекущий размер меньше базового (возможно, AOF не синхронизирован)")
		return
	}
	growth := float64(currentSize-baseSize) / float64(baseSize) * 100
	fmt.Printf("Рост относительно базового: %.2f%%\n", growth)

	// Определяем, нужно ли перезаписывать
	percentVal, _ := strconv.ParseFloat(percent["auto-aof-rewrite-percentage"], 64)
	minSizeVal, _ := strconv.ParseInt(minSize["auto-aof-rewrite-min-size"], 10, 64)
	if currentSize >= minSizeVal && growth >= percentVal {
		fmt.Printf("Условия выполнены: текущий размер >= %d байт и рост >= %.0f%%\n",
			minSizeVal, percentVal)
		fmt.Println("Будет запущена автоматическая перезапись")
	} else {
		fmt.Println("Условия не выполнены, перезапись не требуется")
	}
}

// 7. Влияние AOF на задержки (имитация нагрузки)
func primer7() {
	fmt.Println("--- 7. Влияние AOF на задержки (нагрузка) ---")

	// Сохраняем текущую настройку
	oldSync, _ := rdb.ConfigGet(ctx, "appendfsync").Result()
	defer rdb.ConfigSet(ctx, "appendfsync", oldSync["appendfsync"])

	testLatency := func(policy string) time.Duration {
		rdb.ConfigSet(ctx, "appendfsync", policy)
		time.Sleep(100 * time.Millisecond)

		// Измеряем latency для 1000 команд
		start := time.Now()
		for i := 0; i < 1000; i++ {
			rdb.Set(ctx, fmt.Sprintf("lat:%s:%d", policy, i), "value", 0)
		}
		return time.Since(start)
	}

	// Сравниваем everysec и always (no пропускаем, т.к. не надёжен)
	fmt.Println("Latency 1000 SET команд:")
	latEverysec := testLatency("everysec")
	fmt.Printf("appendfsync everysec: %v\n", latEverysec)

	latAlways := testLatency("always")
	fmt.Printf("appendfsync always: %v (может быть значительно выше)\n", latAlways)

	// Очистка
	keys, _ := rdb.Keys(ctx, "lat:*").Result()
	rdb.Del(ctx, keys...)
}
