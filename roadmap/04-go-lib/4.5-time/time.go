package __5_time

import (
	"fmt"
	"log"
	"time"
)

/*
ПАКЕТ TIME — ПОЛНОЕ РУКОВОДСТВО

Пакет time — один из самых важных в Go. Он отвечает за:
  1. Получение текущего времени
  2. Форматирование и парсинг дат
  3. Работу с таймерами и тикерами
  4. Измерение времени выполнения
  5. Работу с часовыми поясами

1. ТИПЫ ПАКЕТА TIME

type Time struct {
    // содержит внутреннее представление времени
    // НЕЛЬЗЯ создавать Time вручную! Всегда через функции
}

Тип Time — структура, но её поля не экспортируются.
Всегда используй функции для создания и работы с Time.

ПОЛЕЗНЫЕ КОНСТАНТЫ:
  time.ANSIC        = "Mon Jan _2 15:04:05 2006"
  time.UnixDate     = "Mon Jan _2 15:04:05 MST 2006"
  time.RubyDate     = "Mon Jan 02 15:04:05 -0700 2006"
  time.RFC822       = "02 Jan 06 15:04 MST"
  time.RFC822Z      = "02 Jan 06 15:04 -0700"
  time.RFC850       = "Monday, 02-Jan-06 15:04:05 MST"
  time.RFC1123      = "Mon, 02 Jan 2006 15:04:05 MST"
  time.RFC1123Z     = "Mon, 02 Jan 2006 15:04:05 -0700"
  time.RFC3339      = "2006-01-02T15:04:05Z07:00"  // JSON (самый популярный)
  time.RFC3339Nano  = "2006-01-02T15:04:05.999999999Z07:00"
  time.Kitchen      = "3:04PM"
  time.Stamp        = "Jan _2 15:04:05"
  time.StampMilli   = "Jan _2 15:04:05.000"
  time.StampMicro   = "Jan _2 15:04:05.000000"
  time.StampNano    = "Jan _2 15:04:05.000000000"

2. РЕФЕРЕНСНАЯ ДАТА (ВАЖНО!)

В Go для форматирования используется МАГИЧЕСКАЯ ДАТА:

  Mon Jan 2 15:04:05 MST 2006

  01  02  03  04  05  06  07  08  09  10  11  12
  |   |   |   |   |   |   |   |   |   |   |   |
  месяц день час мин сек год часы часовой пояс

ПОЧЕМУ ИМЕННО ЭТА ДАТА?
  01/02 03:04:05 '06 -0700  (MST = GMT-7)
  1-2-3-4-5-6-7

  Это позволяет запоминать формат как:
  2006-01-02T15:04:05Z07:00

  ВСЕГДА используй эту дату для форматирования!

ПРИМЕРЫ ФОРМАТОВ:
  2006-01-02         // 2024-01-15
  02.01.2006         // 15.01.2024
  15:04:05           // 14:30:45
  2006-01-02 15:04:05 // 2024-01-15 14:30:45
  Mon, 02 Jan 2006 15:04:05 MST // Mon, 15 Jan 2024 14:30:45 MST
*/

/*
3. ОСНОВНЫЕ ФУНКЦИИ

func Now() Time
  • Возвращает текущее локальное время.
  • Использует монотонные часы (для измерения интервалов).

func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time
  • Создаёт время из компонентов.
  • month: time.January, time.February, ...
  • loc: time.Local, time.UTC, или свой Location.

func Parse(layout, value string) (Time, error)
  • Парсит строку в время по указанному формату.

func ParseInLocation(layout, value string, loc *Location) (Time, error)
  • Парсит строку в время в указанном часовом поясе.

func (t Time) Format(layout string) string
  • Форматирует время в строку по указанному формату.

func (t Time) Unix() int64
  • Возвращает Unix-время (секунды с 1970-01-01 UTC).

func (t Time) UnixNano() int64
  • Возвращает Unix-время в наносекундах.

func (t Time) IsZero() bool
  • Проверяет, является ли время нулевым (0001-01-01 00:00:00 UTC).

func (t Time) Equal(u Time) bool
  • Сравнивает время (учитывает таймзоны).

func (t Time) Before(u Time) bool
  • Проверяет, что t раньше u.

func (t Time) After(u Time) bool
  • Проверяет, что t позже u.

func (t Time) Sub(u Time) Duration
  • Возвращает разницу между временами.

func (t Time) Add(d Duration) Time
  • Добавляет длительность ко времени.

func (t Time) AddDate(years, months, days int) Time
  • Добавляет годы, месяцы, дни ко времени.

func (t Time) Truncate(d Duration) Time
  • Округляет время вниз до кратности d.

func (t Time) Round(d Duration) Time
  • Округляет время до ближайшего кратного d.

func (t Time) In(loc *Location) Time
  • Конвертирует время в указанный часовой пояс.

func (t Time) Location() *Location
  • Возвращает часовой пояс времени.

func (t Time) Zone() (name string, offset int)
  • Возвращает название и смещение часового пояса.

func (t Time) Clock() (hour, min, sec int)
  • Возвращает часы, минуты, секунды.

func (t Time) Date() (year int, month Month, day int)
  • Возвращает год, месяц, день.

func (t Time) YearDay() int
  • Возвращает день года (1-365/366).

func (t Time) ISOWeek() (year, week int)
  • Возвращает год и неделю по ISO 8601.

4. ДЛИТЕЛЬНОСТИ (DURATION)

type Duration int64

Длительность измеряется в наносекундах.

КОНСТАНТЫ:
  time.Nanosecond  = 1
  time.Microsecond = 1000 * time.Nanosecond
  time.Millisecond = 1000 * time.Microsecond
  time.Second      = 1000 * time.Millisecond
  time.Minute      = 60 * time.Second
  time.Hour        = 60 * time.Minute

СОЗДАНИЕ DURATION:
  d := 5 * time.Second
  d := 10 * time.Minute
  d := 2*time.Hour + 30*time.Minute

МЕТОДЫ DURATION:
  func (d Duration) String() string
  func (d Duration) Hours() float64
  func (d Duration) Minutes() float64
  func (d Duration) Seconds() float64
  func (d Duration) Milliseconds() int64
  func (d Duration) Microseconds() int64
  func (d Duration) Nanoseconds() int64

5. ТАЙМЕРЫ, ТИКЕРЫ, AFTER

TIMER — однократный таймер
  timer := time.NewTimer(5 * time.Second)
  <-timer.C // ждём 5 секунд

  // Остановка таймера (если не сработал)
  timer.Stop()

TICKER — периодический таймер
  ticker := time.NewTicker(1 * time.Second)
  for range ticker.C {
      // выполняется каждую секунду
  }
  ticker.Stop() // обязательно остановить!

AFTER — канал, который сработает через время
  <-time.After(5 * time.Second) // ждём 5 секунд

  // НЕЛЬЗЯ остановить! Используй только для простых случаев.

AFTERFUNC — выполнить функцию через время
  time.AfterFunc(5*time.Second, func() {
      fmt.Println("Прошло 5 секунд")
  })

  // Можно отменить:
  timer := time.AfterFunc(...)
  timer.Stop()

SLEEP — засыпаем на время
  time.Sleep(5 * time.Second)

6. МОНОТОННЫЕ ЧАСЫ

Монотонные часы — это часы, которые только растут вперёд.
Они НЕ зависят от системного времени (не сбиваются при переводе часов).

Go автоматически использует монотонные часы для:
  • time.Now() — сохраняет монотонный счётчик внутри Time
  • time.Since(t) — использует монотонные часы
  • time.Until(t) — использует монотонные часы
  • time.Duration — вычисляется через монотонные часы

ПРИМЕР ИЗМЕРЕНИЯ ВРЕМЕНИ:
  start := time.Now()
  // выполняем работу
  elapsed := time.Since(start) // использует монотонные часы!

ВАЖНО:
  • Монотонные часы НЕ сохраняются при сериализации
  • При сравнении через Equal() монотонные часы НЕ учитываются
  • При конвертации в Unix() монотонные часы теряются

7. ЧАСОВЫЕ ПОЯСА (TIMEZONE)

LOCATION — представляет часовой пояс
  loc, err := time.LoadLocation("America/New_York")
  loc := time.UTC
  loc := time.Local

ПОЛУЧЕНИЕ ВРЕМЕНИ В ПОЯСЕ:
  t := time.Now().In(loc)

СОЗДАНИЕ ВРЕМЕНИ В ПОЯСЕ:
  t := time.Date(2024, 1, 1, 0, 0, 0, 0, loc)

ПАРСИНГ С УЧЁТОМ ПОЯСА:
  t, err := time.ParseInLocation("2006-01-02", "2024-01-15", loc)

КОНВЕРТАЦИЯ МЕЖДУ ПОЯСАМИ:
  nyc, _ := time.LoadLocation("America/New_York")
  london, _ := time.LoadLocation("Europe/London")

  t := time.Now().In(nyc)
  tLondon := t.In(london)

ПОЛУЧЕНИЕ ИНФОРМАЦИИ О ПОЯСЕ:
  name, offset := t.Zone()
  // name: "MST", "EST", "CEST" и т.д.
  // offset: смещение в секундах от UTC

8. ТИПИЧНЫЕ ОШИБКИ

1. ИСПОЛЬЗОВАНИЕ НЕПРАВИЛЬНОГО ФОРМАТА:
   ❌ time.Parse("2024-01-15", "2006-01-02")
   ✅ time.Parse("2006-01-02", "2024-01-15")

2. ИГНОРИРОВАНИЕ ЧАСОВЫХ ПОЯСОВ:
   ❌ time.Parse("2006-01-02", "2024-01-15") // UTC
   ✅ time.ParseInLocation("2006-01-02", "2024-01-15", time.Local)

3. НЕЗАКРЫТИЕ TICKER:
   ❌ ticker := time.NewTicker(1 * time.Second)
   ✅ ticker.Stop()

4. ИСПОЛЬЗОВАНИЕ TIME.AFTER В ЦИКЛЕ:
   ❌ for { <-time.After(1 * time.Second) } // утечка!
   ✅ timer := time.NewTimer(1 * time.Second); timer.Reset()

5. СРАВНЕНИЕ TIME С НУЛЁМ:
   ❌ if t == time.Time{} // так можно, но лучше IsZero()
   ✅ if t.IsZero()

6. ЗАБЫТЬ ПРОВЕРИТЬ ОШИБКУ ПРИ ПАРСИНГЕ:
   ❌ t, _ := time.Parse("2006-01-02", "2024-01-15")
   ✅ t, err := time.Parse("2006-01-02", "2024-01-15"); if err != nil { ... }

9. ШПАРГАЛКА ДЛЯ СОБЕСЕДОВАНИЯ

КАК ПОЛУЧИТЬ ТЕКУЩЕЕ ВРЕМЯ?
  time.Now()

КАК ОТФОРМАТИРОВАТЬ ВРЕМЯ?
  t.Format("2006-01-02 15:04:05")

КАК ПРОПАРСИТЬ ВРЕМЯ?
  time.Parse("2006-01-02", "2024-01-15")

ЧТО ТАКОЕ МАГИЧЕСКАЯ ДАТА?
  Mon Jan 2 15:04:05 MST 2006
  Используется для форматирования.

ЧТО ТАКОЕ МОНОТОННЫЕ ЧАСЫ?
  Часы, которые только растут вперёд (не сбиваются при переводе).
  Используются для измерения интервалов.

КАК ИЗМЕРИТЬ ВРЕМЯ ВЫПОЛНЕНИЯ?
  start := time.Now()
  // работа
  elapsed := time.Since(start)

КАК СДЕЛАТЬ ТАЙМЕР?
  timer := time.NewTimer(5 * time.Second)
  <-timer.C

КАК СДЕЛАТЬ ПЕРИОДИЧЕСКУЮ ЗАДАЧУ?
  ticker := time.NewTicker(1 * time.Second)
for range ticker.C { * делаем *}

ЗАЧЕМ ЗАКРЫВАТЬ TICKER?
Чтобы остановить горутину и освободить ресурсы.

КАК ПЕРЕВЕСТИ ВРЕМЯ В ДРУГОЙ ЧАСОВОЙ ПОЯС?
t.In(loc)

КАК ПОЛУЧИТЬ UNIX-ВРЕМЯ?
t.Unix()   // секунды
t.UnixNano() // наносекунды
*/

// 1. ОСНОВЫ — NOW, FORMAT, PARSE
func primer1() {
	//текущее время
	now := time.Now()
	fmt.Println("Now:", now)

	// Форматирование
	fmt.Println("RFC3339:", now.Format(time.RFC3339))
	fmt.Println("Custom:", now.Format("2006-01-02 15:04:05"))
	fmt.Println("Date only:", now.Format("02.01.2006"))
	fmt.Println("Time only:", now.Format("15:04:05"))

	// Парсинг
	str := "2024-01-15 14:30:45"
	t, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Parsed:", t)
}

// 2. КОМПОНЕНТЫ ВРЕМЕНИ
func primer2() {
	t := time.Now()

	year, month, day := t.Date()
	hour, min, sec := t.Clock()

	fmt.Printf("Date: %d-%02d-%02d\n", year, month, day)
	fmt.Printf("Time: %02d:%02d:%02d\n", hour, min, sec)
	fmt.Printf("Weekday: %s\n", t.Weekday())
	fmt.Printf("YearDay: %d\n", t.YearDay())
}

// 3. ДЛИТЕЛЬНОСТИ
func primer3() {
	d1 := 5 * time.Second
	d2 := 2*time.Minute + 30*time.Second

	fmt.Printf("d1: %v (%f seconds)\n", d1, d1.Seconds())
	fmt.Printf("d2: %v (%f minutes)\n", d2, d2.Minutes())

	// Сложение
	d3 := d1 + d2
	fmt.Printf("d1 + d2 = %v\n", d3)

	// Сравнение
	fmt.Printf("d1 < d2: %v\n", d1 < d2)
}

// 4. ТАЙМЕРЫ И ТИКЕРЫ
func primer4() {
	// Timer
	fmt.Println("Start timer (2s)...")
	timer := time.NewTimer(2 * time.Second)
	<-timer.C
	fmt.Println("Timer fired!")

	// Ticker
	fmt.Println("\nStart ticker (3 ticks, 200ms each)...")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < 3; i++ {
		<-ticker.C
		fmt.Printf("Tick %d\n", i+1)
	}

	// After
	fmt.Println("\nWaiting 1s with After...")
	<-time.After(1 * time.Second)
	fmt.Println("After fired!")

	// AfterFunc
	fmt.Println("\nAfterFunc (500ms)...")
	time.AfterFunc(500*time.Millisecond, func() {
		fmt.Println("AfterFunc fired!")
	})
	time.Sleep(1 * time.Second)

	// Sleep
	fmt.Println("\nSleeping 200ms...")
	time.Sleep(200 * time.Millisecond)
	fmt.Println("Slept!")
}

// 5. ИЗМЕРЕНИЕ ВРЕМЕНИ
func primer5() {
	start := time.Now()

	// Имитация работы
	time.Sleep(150 * time.Millisecond)

	elapsed := time.Since(start)
	fmt.Printf("Elapsed: %v (%d µs)\n", elapsed, elapsed.Microseconds())

	// Until
	future := time.Now().Add(5 * time.Second)
	remaining := time.Until(future)
	fmt.Printf("Until future: %v\n", remaining)
}

// 6. ЧАСОВЫЕ ПОЯСА
func primer6() {
	// Текущее время в разных поясах
	now := time.Now()
	fmt.Printf("Local: %v\n", now)

	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("NYC: %v\n", now.In(nyc))

	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("London: %v\n", now.In(london))

	// Информация о поясе
	name, offset := now.Zone()
	fmt.Printf("Zone: %s (offset: %d seconds)\n", name, offset)

	// Создание времени в UTC
	utc := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	fmt.Printf("UTC time: %v\n", utc)
	fmt.Printf("In NYC: %v\n", utc.In(nyc))
}

// 7. ДОБАВЛЕНИЕ ВРЕМЕНИ
func primer7() {
	now := time.Now()
	fmt.Printf("Now: %v\n", now)

	// Add
	fmt.Printf("+5s: %v\n", now.Add(5*time.Second))
	fmt.Printf("-1h: %v\n", now.Add(-1*time.Hour))

	// AddDate
	fmt.Printf("+1 year: %v\n", now.AddDate(1, 0, 0))
	fmt.Printf("+1 month: %v\n", now.AddDate(0, 1, 0))
	fmt.Printf("+1 day: %v\n", now.AddDate(0, 0, 1))

	// Truncate и Round
	fmt.Printf("\nTruncate to minute: %v\n", now.Truncate(time.Minute))
	fmt.Printf("Round to minute: %v\n", now.Round(time.Minute))
}

// 8. СРАВНЕНИЕ И ПРОВЕРКИ
func primer8() {
	t1 := time.Now()
	t2 := t1.Add(5 * time.Second)

	fmt.Printf("t1: %v\n", t1)
	fmt.Printf("t2: %v\n", t2)

	fmt.Printf("t1 == t2: %v\n", t1.Equal(t2))
	fmt.Printf("t1.Before(t2): %v\n", t1.Before(t2))
	fmt.Printf("t1.After(t2): %v\n", t1.After(t2))

	// Разница
	fmt.Printf("t2 - t1: %v\n", t2.Sub(t1))

	// IsZero
	var zero time.Time
	fmt.Printf("zero.IsZero(): %v\n", zero.IsZero())
}

// 9. ПАРСИНГ С УЧЁТОМ ТАЙМЗОНЫ
func primer9() {
	// RFC3339 — стандартный JSON-формат
	str := "2026-01-15T14:30:45Z"
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("RFC3339: %v\n", t)

	// С датой без таймзоны (предполагаем локальную)
	str2 := "2024-01-15 14:30:45"
	t2, err := time.ParseInLocation("2006-01-02 15:04:05", str2, time.Local)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ParseInLocation (Local): %v\n", t2)

	// Unix timestamp
	unix := time.Now().Unix()
	t3 := time.Unix(unix, 0)
	fmt.Printf("Unix: %d -> %v\n", unix, t3)
}

func main() {
	fmt.Println("primer1")
	primer1()
	fmt.Println("primer2")
	primer2()
	fmt.Println("primer3")
	primer3()
	fmt.Println("primer4")
	primer4()
	fmt.Println("primer5")
	primer5()
	fmt.Println("primer6")
	primer6()
	fmt.Println("primer7")
	primer7()
	fmt.Println("primer8")
	primer8()
	fmt.Println("primer9")
	primer9()
}
