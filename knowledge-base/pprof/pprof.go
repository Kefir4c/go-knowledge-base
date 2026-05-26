package pprof

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

/*
БЛОК 1: ОСНОВЫ ПРОФИЛИРОВАНИЯ И ПАКЕТ PPROF (JUNIOR)

ТЕОРИЯ

1.1 ЧТО ТАКОЕ ПРОФИЛИРОВАНИЕ

профилирование — сбор метрик о работе программы: где тратится cpu, сколько памяти занято,
сколько горутин висит, где происходят блокировки.

без профилирования ты гадаешь, где узкое место. с профилированием — точно знаешь.

1.2 ПАКЕТ PPROF

pprof — стандартный пакет go для профилирования. состоит из двух частей:

   - runtime/pprof: для профилирования cli утилит и коротких программ
   - net/http/pprof: для профилирования http сервисов (обёртка над runtime/pprof)

1.3 ТИПЫ ПРОФИЛЕЙ (ОСНОВНЫЕ)

cpu    — время выполнения функций (сэмплирование каждые 10ms)
heap   — текущая память (живые объекты, поиск утечек)
goroutine — стеки всех горутин (поиск зависших)
allocs — все аллокации за время работы (оптимизация gc)
block  — ожидание на mutex и каналах (конкурентные узкие места)

1.4 КАК ЭТО РАБОТАЕТ

cpu профиль: каждые 10ms ядро прерывает программу и записывает текущий стек вызовов.
чем больше сэмплов на функции — тем больше процессорного времени она заняла.

heap профиль: gc знает, какие объекты живые, а какие нет. профиль показывает только живые.
перед записью полезно вызвать runtime.GC(), чтобы очистить временные объекты.
*/
/*
ПРИМЕР 1: НАХОДИМ МЕДЛЕННУЮ ФУНКЦИЮ В ПРОДЕ
ситуация: после добавления новой фичи сервис стал отвечать за 500ms вместо 50ms.
подозреваем базу или внешний api, но точно не знаем.
решение: включаем cpu профиль на 30 секунд под нагрузкой, смотрим top5.
*/

func slowBusinessLogic() {
	// представь, что здесь бизнес-логика
	time.Sleep(10 * time.Millisecond)
	// какая-то тяжёлая операция
	sum := 0
	for i := 0; i < 10000000; i++ {
		sum += i
	}
	_ = sum
}

func handleRequest() {
	slowBusinessLogic()
}

func mainWithfunc() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	for i := 0; i < 100; i++ {
		handleRequest()
	}
}

/*
ПРИМЕР 2:ИЩЕМ УТЕЧКУ ПАМЯТИ
ситуация: память растёт на 100mb в час, через 10 часов сервис падает по ram.
обычные тесты не находят проблему, потому что утечка накапливается медленно.

решение: снимаем heap профиль через час после старта и ещё через 3 часа.
сравниваем — видим, какие объекты появились и не ушли.
*/
type userSession struct {
	id   string
	data []byte
}

var globalSessions = make(map[string]*userSession)

func createSessions(id string) {
	// баг: никогда не удаляем старые сессии
	globalSessions[id] = &userSession{
		id:   id,
		data: make([]byte, 1024*1024),
	}
}

func captureHeapProfile(filename string) {
	runtime.GC()
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal(err)
	}
}

func mainWithLeak() {
	for i := 0; i < 1000; i++ {
		createSessions(string(rune(i)))
	}
	captureHeapProfile("heap.prof")
}

/*
ПРИМЕР 3: РЕАЛЬНЫЙ КЕЙС — НАХОДИМ ЗАВИСШИЕ ГОРУТИНЫ
ситуация: сервис перестал отвечать, но cpu почти 0%.
явно не в вычислениях проблема, скорее всего горутины зависли на каналах.

решение: снимаем профиль горутин и смотрим, где они блокируются.
*/

func leakingWorker() {
	ch := make(chan int)
	go func() {
		ch <- 42
	}()
}

func captureGoroutineProfile(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
		log.Fatal(err)
	}
}

func mainWithGoroutineLeak() {
	for i := 0; i < 1000; i++ {
		leakingWorker()
	}
	captureGoroutineProfile("goroutine.prof")
}

/*
ПРИМЕР 4: РЕАЛЬНЫЙ КЕЙС — ОПТИМИЗАЦИЯ GC ЧЕРЕЗ ALLOCS

ситуация: сервис работает быстро, но частота gc высокая (видно в метриках).
значит, кто-то создаёт много временных объектов.

решение: снимаем allocs профиль — он показывает ВСЕ аллокации, даже временные.
*/
func badStringConcat(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += "x"
	}
	return result
}

func goodStringConcat(n int) string {
	builder := strings.Builder{}
	for i := 0; i < n; i++ {
		builder.WriteString("x")
	}
	return builder.String()
}

func captureAllocsProfile(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
		log.Fatal(err)
	}
}

/*
БЛОК 2: HTTP ПРОФИЛИРОВАНИЕ И go tool pprof (JUNIOR+)

ТЕОРИЯ

2.1 ПАКЕТ net/http/pprof

для долго работающих сервисов (http, grpc) удобнее профилировать через http эндпоинты.
достаточно импортировать пакет, и он сам зарегистрирует хендлеры.

import _ "net/http/pprof"

эндпоинты регистрируются на стандартном http.DefaultServeMux:
   /debug/pprof/           - главная страница со списком профилей
   /debug/pprof/cpu        - cpu профиль (параметр ?seconds=30)
   /debug/pprof/heap       - память (живые объекты)
   /debug/pprof/goroutine  - стеки горутин
   /debug/pprof/allocs     - все аллокации
   /debug/pprof/block      - блокировки на mutex/каналах
   /debug/pprof/mutex      - контеншн на mutex

2.2 КОМАНДА go tool pprof

основные команды для работы с профилями:

снятие профиля с живого сервера:
   go tool pprof http://localhost:6060/debug/pprof/cpu?seconds=30

снятие профиля из файла:
   go tool pprof cpu.prof

запуск веб-интерфейса:
   go tool pprof -http=:8080 cpu.prof

получение топа функций без интерактива:
   go tool pprof -top cpu.prof

сравнение двух профилей (до/после оптимизации):
   go tool pprof -base before.prof after.prof

генерация графа в pdf:
   go tool pprof -pdf cpu.prof > profile.pdf

2.3 ОСОБЕННОСТИ CPU ПРОФИЛИРОВАНИЯ ЧЕРЕЗ HTTP

- нужно указывать параметр seconds=30 (иначе вернёт мгновенный снимок, который бесполезен)
- профилирование жрёт ресурсы (5-10% cpu), не держи включённым постоянно
- лучше снимать на отдельном порту, чтобы не влиять на основной трафик

2.4 ОСОБЕННОСТИ HEAP ПРОФИЛИРОВАНИЯ ЧЕРЕЗ HTTP

- /debug/pprof/heap показывает живые объекты
- можно добавить параметры:
   gc=1 - вызвать gc перед снятием (очистить временный мусор)
   debug=1 - человекочитаемый формат (не для go tool)
*/

/*
ПРИМЕР 1: ПОДКЛЮЧАЕМ PPROF К HTTP СЕРВЕРУ
ситуация: у тебя есть http сервис, который тормозит в проде.
нужно добавить pprof эндпоинты, чтобы снимать профили без остановки сервиса.

решение: импортируем net/http/pprof и запускаем отдельный http сервер для профилирования.
*/
func slowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(100 * time.Millisecond)
	w.Write([]byte("Hello"))
}

func startPprofServer() {
	// запускаем pprof на отдельном порту, не мешает основному серверу
	go func() {
		log.Println("pprof server started on :6060")
		log.Println(http.ListenAndServe(":6060", nil))
	}()
}

func mainServer() {
	http.HandleFunc("/api", slowHandler)
	log.Println("main server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func startMain() {
	startPprofServer()
	mainServer()
}

/*
ПРИМЕР 2: СНИМАЕМ CPU ПРОФИЛЬ С ЖИВОГО СЕРВЕРА
ситуация: сервис уже работает в проде с включённым pprof.
нужно снять cpu профиль, чтобы понять, почему он тормозит.

решение: используем go tool pprof с параметром seconds.

команда:
   go tool pprof http://localhost:6060/debug/pprof/cpu?seconds=30

после выполнения программа соберёт профиль за 30 секунд и перейдёт в интерактивный режим.
в интерактиве:
   (pprof) top10     - показать 10 самых дорогих функций
   (pprof) list      - показать код с затратами
   (pprof) web       - открыть граф вызовов

пример вывода top10:
   flat  flat%   sum%        cum   cum%
   2.5s  50.00% 50.00%      2.5s 50.00%  time.Sleep
   1.0s  20.00% 70.00%      1.0s 20.00%  slowHandler
   0.5s  10.00% 80.00%      3.5s 70.00%  main.handleRequest
*/

/*
ПРИМЕР 3: СНИМАЕМ HEAP ПРОФИЛЬ ДЛЯ ПОИСКА УТЕЧКИ
ситуация: память сервиса растёт и не падает. подозреваем утечку.
нужно посмотреть, какие объекты занимают память.

решение: снимаем heap профиль и анализируем.

команда:
   go tool pprof http://localhost:6060/debug/pprof/heap

полезные флаги при анализе:
   -inuse_space     - показать память, которую занимают живые объекты (по умолчанию)
   -alloc_space     - показать все аллокации за всё время (помогает найти частые создания)
   -inuse_objects   - показать количество живых объектов

пример:
   go tool pprof -alloc_space http://localhost:6060/debug/pprof/heap

после анализа:
   (pprof) top
   (pprof) list createSession

увидим, какая функция создаёт больше всего объектов.
*/

/*
ПРИМЕР 4: ЗАПУСКАЕМ ВЕБ-ИНТЕРФЕЙС ДЛЯ АНАЛИЗА ПРОФИЛЯ
ситуация: терминальный режим pprof неудобен для изучения больших графов вызовов.
хочешь кликабельный интерфейс с сортировкой и поиском.

решение: запускаем веб-интерфейс.

команда:
   go tool pprof -http=:8081 cpu.prof

или сразу из живого сервера:
   go tool pprof -http=:8081 http://localhost:6060/debug/pprof/cpu?seconds=30

откроется браузер с интерфейсом:
   - вкладка top: самые дорогие функции
   - вкладка graph: граф вызовов (кликабельный)
   - вкладка flame: flame graph (очень удобно видеть пропорции)
   - вкладка source: исходный код с подсветкой затрат
*/

/*
ПРИМЕР 5: СРАВНИВАЕМ ДВА HEAP ПРОФИЛЯ (ДО/ПОСЛЕ)
ситуация: выпустили фикс утечки памяти. нужно убедиться, что помогло.
или наоборот, память начала течь после релиза — нужно понять, что изменилось.

решение: снимаем профиль до и после, сравниваем через -base.

шаги:
   1. снимаем профиль до фикса:
      go tool pprof -base heap_before.prof heap_after.prof

   2. снимаем профиль после фикса:
      curl -o heap_after.prof http://localhost:6060/debug/pprof/heap

   3. сравниваем:
      go tool pprof -base heap_before.prof heap_after.prof

   (pprof) top   # покажет только разницу: положительные цифры = память выросла
*/

/*
ПРИМЕР 6: АВТОМАТИЧЕСКОЕ СНЯТИЕ ПРОФИЛЯ ПРИ ПАНИКЕ
ситуация: программа падает не всегда, а при определённых условиях.
нужно снять профиль в момент паники, чтобы понять причину.

решение: используем recover и снимаем профиль при панике.
*/
func captureProfileOnPanic() {
	if r := recover(); r != nil {
		// снимаем heap профиль в момент паники
		f, _ := os.Create("panic_heap.prof")
		defer f.Close()
		runtime.GC()
		pprof.WriteHeapProfile(f)

		// снимаем goroutine профиль
		gf, _ := os.Create("panic_goroutine.prof")
		defer gf.Close()
		pprof.Lookup("goroutine").WriteTo(gf, 0)

		// после анализа можно понять, что привело к панике
		panic(r) // продолжаем падать
	}
}
func unstableFunction() {
	// какая-то нестабильная функция
	defer captureProfileOnPanic()

	// баг: при определённых условиях паника
	var data []int
	_ = data[100] // паника при пустом слайсе
}

/*
ПРИМЕР 7: ПРОФИЛИРУЕМ ТОЛЬКО КОНКРЕТНЫЙ УЧАСТОК КОДА
ситуация: программа большая, профиль зашумлён. нужно профилировать только конкретную операцию.
pprof позволяет включать и выключать профилирование в рантайме.

решение: используем runtime/pprof напрямую, даже если у нас http сервис.
*/
var cpuProfileFile *os.File

func startCustomCPUProfile() {
	var err error
	cpuProfileFile, err = os.Create("custom_cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(cpuProfileFile)
}

func stopCustomCPUProfile() {
	pprof.StopCPUProfile()
	cpuProfileFile.Close()
}

func expensiveOperation() {
	// какая-то дорогая операция
	sum := 0
	for i := 0; i < 100000000; i++ {
		sum += i
	}
	_ = sum
}

func handleImportantRequest(w http.ResponseWriter, r *http.Request) {
	// профилируем только этот запрос
	startCustomCPUProfile()
	defer stopCustomCPUProfile()

	expensiveOperation()
	w.Write([]byte("done"))
}
