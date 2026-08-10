package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
УРОК 3.1. RDB (СНИМКИ) — РАСШИРЕННАЯ ТЕОРИЯ
0. ВВЕДЕНИЕ В ПЕРСИСТЕНТНОСТЬ

Redis — это in-memory база данных. Это её главная сила (скорость) и главная слабость
(потеря данных при перезапуске). Чтобы данные не исчезали, Redis предлагает два
механизма сохранения: RDB (снимки) и AOF (журнал операций).

RDB — это полный снимок (snapshot) всех данных на определённый момент времени.

Они могут использоваться по отдельности или вместе (рекомендуется для критичных данных).

1. АРХИТЕКТУРА RDB: КАК ЭТО РАБОТАЕТ ВНУТРИ

1.1. Триггеры создания RDB:
  - По расписанию (настройки save).
  - Вручную (BGSAVE или SAVE).
  - При репликации (мастер отправляет RDB репликам при первом подключении).
  - При завершении работы (SHUTDOWN SAVE).

1.2. Процесс создания снимка (BGSAVE):
 1. Redis вызывает fork() для создания дочернего процесса.
 2. Дочерний процесс получает копию памяти (copy-on-write).
 3. Дочерний процесс пишет RDB-файл на диск (в фоновом режиме).
 4. Родительский процесс продолжает обрабатывать запросы.
 5. Когда дочерний процесс завершает запись, он уведомляет родителя.
 6. Родитель заменяет старый RDB-файл новым (атомарно, через rename).

1.3. Copy-on-Write (COW):
  - fork() не копирует физическую память сразу — только таблицы страниц.
  - Пока дочерний процесс пишет, родитель может изменять данные.
  - При изменении страницы родителем, страница копируется в дочерний процесс
    (copy-on-write), чтобы дочерний видел согласованную версию.
  - Это означает, что во время BGSAVE память может временно вырасти на 20-30%
    (из-за копирования изменяемых страниц).

1.4. Формат RDB-файла:
  - Бинарный, компактный, сжатый (опционально).
  - Содержит все данные: ключи, значения, TTL, типы данных.
  - Включает контрольную сумму для проверки целостности.
  - Структура: заголовок (версия) → данные (key-value) → TTL → конец.

1.5. Атомарность записи:
  - Сначала записывается во временный файл (temp-XXXXXX.rdb).
  - После успешной записи — переименовывается в dump.rdb.
  - Это предотвращает повреждение данных, если процесс упадёт во время записи.

2. НАСТРОЙКИ RDB (ПОДРОБНО)

2.1. save <seconds> <changes>
  - Определяет условия автоматического создания снимка.
  - Можно указать несколько условий.
  - Пример: save 900 1 — если прошло 900 секунд и хотя бы 1 ключ изменился.
  - Пример: save 300 10 — 300 секунд и 10 изменений.
  - Пример: save 60 10000 — 60 секунд и 10000 изменений.
  - Если save не указан, автоматические снимки отключены.
  - Важно: если изменения происходят чаще, чем save условие, снимки будут чаще.

2.2. stop-writes-on-bgsave-error
  - Если BGSAVE завершился ошибкой (например, не хватает места на диске),
    запрещает запись в Redis.
  - По умолчанию yes — защищает от потери данных, но может привести к ошибкам.
  - В некоторых случаях можно отключить (no), если важнее доступность.

2.3. rdbcompression
  - Сжимать ли RDB-файл (алгоритм LZF).
  - yes — экономит место, но использует CPU при записи и чтении.
  - no — быстрее, но файл больше.

2.4. rdbchecksum
  - Добавлять ли контрольную сумму в конец файла.
  - yes — проверка целостности при загрузке (защита от повреждений).
  - no — чуть быстрее, но без проверки.

2.5. dbfilename
  - Имя файла (по умолчанию dump.rdb).

2.6. dir
  - Директория для хранения RDB и AOF файлов.
  - Должна быть на быстром диске (SSD) и с достаточным местом.

3. ВЛИЯНИЕ RDB НА ПРОИЗВОДИТЕЛЬНОСТЬ

3.1. fork() latency
  - fork() может быть медленным при больших объёмах памяти (особенно в Linux).
  - На больших инстансах (50+ ГБ) fork может занимать несколько секунд.
  - В это время Redis не отвечает на запросы (блокировка).
  - Решение: использовать репликацию для создания снимков на реплике.

3.2. Запись на диск (I/O)
  - BGSAVE пишет на диск, что может вызывать задержки.
  - Если диск медленный (HDD), это может замедлить все операции.
  - Используйте быстрые диски (SSD, NVMe) и контролируйте I/O.

3.3. Потребление памяти (COW)
  - Во время BGSAVE память может вырасти на 20-30% (из-за копирования страниц).
  - Важно убедиться, что памяти достаточно, иначе Redis может быть убит OOM Killer.
  - Мониторьте used_memory и used_memory_rss во время BGSAVE.

3.4. Частота снимков (продолжительность)
  - Чем чаще снимки, тем меньше данных теряется, но выше нагрузка.
  - Компромисс: для кэша можно редко (например, раз в час),
    для критичных данных — чаще + AOF.

4. СРАВНЕНИЕ RDB И AOF

┌──────────────────────┬─────────────────────────────────────────────────────┐
│ ХАРАКТЕРИСТИКА       │ RDB                                                 │
├──────────────────────┼─────────────────────────────────────────────────────┤
│ Размер файла         │ Компактный (сжатый)                                 │
│ Скорость записи      │ Средняя (создаётся редко)                           │
│ Восстановление       │ Быстрое (загружается один файл)                     │
│ Потеря данных        │ До следующего снимка (может быть много)             │
│ Нагрузка на CPU      │ Высокая при создании снимка (fork, сжатие)          │
│ Нагрузка на диск     │ Высокая при записи (особенно на HDD)                │
│ Сложность            │ Простая настройка                                   │
│ Типичное применение  │ Кэш, репликация, резервное копирование              │
└──────────────────────┴─────────────────────────────────────────────────────┘

Рекомендация: для production используйте RDB + AOF одновременно,
чтобы получить и скорость восстановления, и минимальную потерю данных.

5. МОНИТОРИНГ RDB (INFO persistence)

Важные метрики:
- rdb_last_save_time — время последнего успешного снимка (timestamp).
- rdb_last_bgsave_status — статус последнего BGSAVE (ok/err).
- rdb_bgsave_in_progress — идёт ли BGSAVE сейчас (1/0).
- rdb_current_bgsave_time_sec — сколько секунд длится текущий BGSAVE.
- rdb_last_bgsave_time_sec — сколько секунд длился предыдущий BGSAVE.
- rdb_last_cow_size — сколько памяти было скопировано (COW) при последнем BGSAVE.

Пример мониторинга:

	> INFO persistence
	# Persistence
	loading:0
	rdb_last_save_time:1698765432
	rdb_last_bgsave_status:ok
	rdb_bgsave_in_progress:0
	rdb_last_bgsave_time_sec:3
	rdb_current_bgsave_time_sec:-1
	rdb_last_cow_size:20971520

6. ПРОБЛЕМЫ И ИХ РЕШЕНИЯ

Проблема 1: "BGSAVE не запускается, ошибка forks"

	Причина: не хватает памяти (COW), система отклоняет fork.
	Решение: увеличить память, настроить vm.overcommit_memory=1,
	использовать реплику для создания снимков.

Проблема 2: "RDB-файл повреждён"

	Причина: дисковая ошибка, сбой питания.
	Решение: использовать rdbchecksum yes, восстановить из backup.
	Утилита redis-check-rdb для проверки.

Проблема 3: "RDB-файл слишком большой"

	Причина: много данных, часто создаются снимки.
	Решение: увеличить интервалы save, использовать сжатие (rdbcompression yes),
	рассмотреть AOF (он компактнее при частых изменениях).

Проблема 4: "Падение Redis во время BGSAVE"

	Если Redis упал во время записи, временный файл удаляется,
	старый RDB остаётся нетронутым — данные не теряются.

7. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ ДЛЯ ПРОДАКШНА

1. Для кэша (можно потерять данные) — используйте только RDB с редкими снимками.
2. Для критичных данных — включите AOF + RDB для быстрого восстановления.
3. Регулярно копируйте RDB-файлы в S3/бэкап.
4. Мониторьте размер RDB-файла и время создания.
5. Используйте реплику для создания снимков (чтобы не нагружать мастер).
6. Настройте save разумно: например, save 900 1, save 300 10, save 60 10000.
7. Включайте rdbcompression и rdbchecksum для надёжности.
8. Проверяйте целостность RDB после каждого бэкапа (redis-check-rdb).

8. ИТОГИ ПО RDB

- RDB — это быстрый и компактный механизм сохранения данных.
- Он хорошо подходит для кэширования, репликации и бэкапов.
- Основной недостаток — потеря изменений между снимками.
- Используйте RDB в сочетании с AOF для максимальной надёжности.
- Мониторинг и настройка критичны для стабильной работы.

9. СВЯЗЬ С GO

В Go-приложении мы не управляем созданием RDB напрямую, но мы можем:
- Читать настройки через ConfigGet.
- Проверять статус и время последнего снимка через Info.
- Инициировать BGSAVE через Do("BGSAVE").
- Автоматизировать резервное копирование RDB-файлов (как в примерах).
- Моделировать сценарии потери данных и тестировать восстановление.
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
	fmt.Println("=== RDB (СНИМКИ) ===\n")

	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
}

// 1. Проверка настроек RDB (save, dbfilename, dir)
func primer1() {
	fmt.Println("--- 1. Проверка настроек RDB ---")
	save, _ := rdb.ConfigGet(ctx, "save").Result()
	fmt.Printf("save: %v\n", save["save"]) // например "900 1 300 10"

	filename, _ := rdb.ConfigGet(ctx, "dbfilename").Result()
	fmt.Printf("dbfilename: %s\n", filename["dbfilename"])

	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	fmt.Printf("dir: %s\n", dir["dir"])

	// Проверяем, существует ли RDB-файл
	rdbPath := fmt.Sprintf("%s/%s", dir["dir"], filename["dbfilename"])
	if _, err := os.Stat(rdbPath); err == nil {
		fmt.Printf("RDB-файл существует: %s\n", rdbPath)
	} else {
		fmt.Printf("RDB-файл не найден: %s\n", rdbPath)
	}
}

// 2. Время последнего снимка (из INFO persistence)
func primer2() {
	fmt.Println("--- 2. Время последнего снимка ---")
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	rdbLastSave := info["persistence"]["rdb_last_save_time"]
	lastSaveInt, _ := strconv.ParseInt(rdbLastSave, 10, 64)
	lastSaveTime := time.Unix(lastSaveInt, 0)

	fmt.Printf("Время последнего снимка: %s\n", lastSaveTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Прошло секунд: %d\n", time.Now().Unix()-lastSaveInt)

	// Проверяем, был ли снимок успешным
	rdbLastBgsaveStatus := info["persistence"]["rdb_last_bgsave_status"]
	fmt.Printf("Статус последнего BGSAVE: %s\n", rdbLastBgsaveStatus)
}

// 3. Симуляция потери данных между снимками
func primer3() {
	fmt.Println("--- 3. Симуляция потери данных между снимками ---")

	// Устанавливаем ключ
	rdb.Set(ctx, "important_key", "value_1", 0)
	fmt.Println("Установлен important_key = value_1")

	// Получаем время последнего снимка
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	lastSave, _ := strconv.ParseInt(info["persistence"]["rdb_last_save_time"], 10, 64)
	fmt.Printf("Последний снимок был в %s\n", time.Unix(lastSave, 0).Format("15:04:05"))

	// Ждём несколько секунд, чтобы время между снимками прошло
	fmt.Println("Ждём 5 секунд... (имитация работы)")
	time.Sleep(5 * time.Second)

	// Меняем ключ
	rdb.Set(ctx, "important_key", "value_2", 0)
	fmt.Println("Обновлён important_key = value_2")

	// Симуляция падения Redis (мы просто не вызываем снимок)
	fmt.Println("Redis падает, но снимок не был сделан!")

	// При перезапуске значение было бы "value_1", так как "value_2" не сохранилось
	fmt.Println("После восстановления будет потеряно обновление (value_2)")

	// В реальном коде можно проверить разницу между текущими данными и RDB
	rdb.Del(ctx, "important_key")
}

// 4. Принудительный вызов BGSAVE (создание снимка)
func primer4() {
	// В go-redis нет прямого метода, используем Do
	err := rdb.Do(ctx, "BGSAVE").Err()
	if err != nil {
		// Если уже идёт BGSAVE, Redis вернёт ошибку
		if strings.Contains(err.Error(), "BGSAVE already in progress") {
			fmt.Println("⚠️  BGSAVE уже выполняется, подождём...")
			// Ждём завершения
			time.Sleep(2 * time.Second)
		}
		panic(err)
	}
	fmt.Println("✅ Команда BGSAVE отправлена")

	// Проверяем статус
	info, _ := rdb.InfoMap(ctx, "persistence").Result()
	status := info["persistence"]["rdb_bgsave_in_progress"]
	fmt.Printf("BGSAVE в процессе: %s\n", status)

	// Ждём завершения (в реальности можно проверять по статусу)
	time.Sleep(1 * time.Second)

	// Проверяем время последнего снимка после BGSAVE
	info2, _ := rdb.InfoMap(ctx, "persistence").Result()
	lastSave, _ := strconv.ParseInt(info2["persistence"]["rdb_last_save_time"], 10, 64)
	fmt.Printf("Время последнего снимка после BGSAVE: %s\n", time.Unix(lastSave, 0).Format("15:04:05"))
	fmt.Println()
}

// 5. Проверка контрольной суммы RDB-файла (rdbchecksum)
func primer5() {
	fmt.Println("--- 5. Проверка контрольной суммы RDB ---")

	// Проверяем, включена ли проверка
	checksum, _ := rdb.ConfigGet(ctx, "rdbchecksum").Result()
	fmt.Printf("rdbchecksum: %s\n", checksum["rdbchecksum"])

	// Получаем путь к RDB-файлу
	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	filename, _ := rdb.ConfigGet(ctx, "dbfilename").Result()
	rdbPath := fmt.Sprintf("%s/%s", dir["dir"], filename["dbfilename"])

	// Проверяем размер файла
	if fileInfo, err := os.Stat(rdbPath); err == nil {
		fmt.Printf("Размер RDB-файла: %d байт (%.2f MB)\n", fileInfo.Size(), float64(fileInfo.Size())/(1024*1024))
	} else {
		fmt.Println("RDB-файл не найден")
	}
	// Проверка целостности через redis-check-rdb (внешняя утилита)
	// В Go можно вызвать, но здесь просто демонстрация
	fmt.Println("Для проверки целостности используйте: redis-check-rdb dump.rdb")
}

// 6. Сравнение размера RDB и AOF (если AOF включён)
func primer6() {
	fmt.Println("--- 6. Сравнение размера RDB и AOF ---")

	// Проверяем, включён ли AOF
	aofEnabled, _ := rdb.ConfigGet(ctx, "appendonly").Result()
	fmt.Printf("appendonly: %s\n", aofEnabled["appendonly"])

	if aofEnabled["appendonly"] == "yes" {
		// Получаем пути
		dir, _ := rdb.ConfigGet(ctx, "dir").Result()
		filename, _ := rdb.ConfigGet(ctx, "dbfilename").Result()
		rdbPath := fmt.Sprintf("%s/%s", dir["dir"], filename["dbfilename"])
		aofPath := fmt.Sprintf("%s/appendonly.aof", dir["dir"])

		// Размеры
		rdbSize, _ := os.Stat(rdbPath)
		aofSize, _ := os.Stat(aofPath)

		if rdbSize != nil && aofSize != nil {
			fmt.Printf("RDB размер: %.2f MB\n", float64(rdbSize.Size())/(1024*1024))
			fmt.Printf("AOF размер: %.2f MB\n", float64(aofSize.Size())/(1024*1024))
			fmt.Printf("RDB/AOF отношение: %.2f\n", float64(rdbSize.Size())/float64(aofSize.Size()))
		}
	} else {
		fmt.Println("AOF отключён, сравнение невозможно")
	}
	fmt.Println()
}

// 7. Автоматическое резервное копирование RDB-файла
func primer7() {
	fmt.Println("--- 7. Автоматическое резервное копирование RDB ---")

	dir, _ := rdb.ConfigGet(ctx, "dir").Result()
	filename, _ := rdb.ConfigGet(ctx, "dbfilename").Result()
	rdbPath := fmt.Sprintf("%s/%s", dir["dir"], filename["dbfilename"])

	if _, err := os.Stat(rdbPath); err == nil {
		data, err := os.ReadFile(rdbPath)
		if err != nil {
			fmt.Printf("Ошибка чтения: %v\n", err)
			return
		}
		backupName := fmt.Sprintf("dump_%s.rdb", time.Now().Format("2006-01-02_15-04-05"))
		err = os.WriteFile(backupName, data, 0644)
		if err != nil {
			fmt.Printf("Ошибка записи backup: %v\n", err)
			return
		}
		fmt.Printf("Бэкап создан: %s (размер: %d байт)\n", backupName, len(data))
		cleanupOldBackups()
	} else {
		fmt.Println("RDB-файл не найден")
	}
	fmt.Println()
}

// cleanupOldBackups
func cleanupOldBackups() {
	files, err := os.ReadDir(".")
	if err != nil {
		return
	}
	var backups []os.DirEntry
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "dump_") && strings.HasSuffix(f.Name(), ".rdb") {
			backups = append(backups, f)
		}
	}
	if len(backups) <= 5 {
		return
	}
	// Сортируем по времени модификации (старые — первые)
	sort.Slice(backups, func(i, j int) bool {
		infoI, _ := backups[i].Info()
		infoJ, _ := backups[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})
	// Удаляем лишние (все, кроме 5 самых новых)
	for i := 0; i < len(backups)-5; i++ {
		if err := os.Remove(backups[i].Name()); err == nil {
			fmt.Printf("Удалён старый бэкап: %s\n", backups[i].Name())
		}
	}
}
