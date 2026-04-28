package _interface

import (
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"
)

// 1.ЧТО ТАКОЕ ИНТЕРФЕЙСЫ
/*
ЧТО ЭТО:
- Интерфейс — это набор методов, которые должен реализовать тип.
- Интерфейсы в Go — **неявные**: тип реализует интерфейс, если реализует все его методы.
- Это основной механизм полиморфизма в Go.

ПОЧЕМУ ЭТО ВАЖНО:
- Позволяет писать гибкий и расширяемый код.
- Упрощает тестирование через моки.
- Реализует принцип "программируй к интерфейсу, а не к типу".

Понимание концепции:
Неявная реализация
В отличие от других языков (Java, C#), в Go нет ключевого слова "implements".
Тип реализует интерфейс автоматически, если содержит все требуемые методы.
Это создаёт гибкость: можно добавить методы в существующий тип, чтобы он реализовывал новый интерфейс, без изменения кода, который его использует.

Композиция вместо наследования
Go не поддерживает наследование, но интерфейсы позволяют достичь похожего поведения через композицию.
Вместо class A extends B в Go: struct A { B } + интерфейсы.

Принцип Liskov Substitution
Интерфейсы в Go помогают соблюдать принцип подстановки Лисков:
"Объекты в программе должны быть заменяемы экземплярами их подтипов без изменения корректности программы".

ПРИМЕР БАЗОВОГО ИНТЕРФЕЙСА:
*/
// Интерфейс для работы с хранилищем данных
type Storage interface {
	Save(data []byte) error
	Load() ([]byte, error)
}

// Реализация для локального хранилища
type LocalStorage struct {
	filePath string
}

func (s LocalStorage) Save(data []byte) error {
	return os.WriteFile(s.filePath, data, 0644)
}

func (s LocalStorage) Load() ([]byte, error) {
	return os.ReadFile(s.filePath)
}

// Реализация для облачного хранилища
type CloudStorage struct {
	bucket string
}

func (s CloudStorage) Save(data []byte) error {
	// Код для сохранения в облако
	return nil
}

func (s CloudStorage) Load() ([]byte, error) {
	// Код для загрузки из облака
	return nil, nil
}

// Использование
func process(storage Storage) {
	data := []byte("Hello, world!")
	if err := storage.Save(data); err != nil {
		log.Fatal(err)
	}
	loadedData, err := storage.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(loadedData))
}

/*
func main() {
	// Можно использовать любой тип, реализующий Storage
	process(LocalStorage{filePath: "data.txt"})
	process(CloudStorage{bucket: "my-bucket"})
}
*/

//2. ЯВНАЯ И НЕЯВНАЯ РЕАЛИЗАЦИЯ ИНТЕРФЕЙСОВ

/*
НЕЯВНАЯ РЕАЛИЗАЦИЯ (IMPLICIT IMPLEMENTATION):

• В Go нет ключевого слова `implements`.
• Тип автоматически реализует интерфейс, если содержит все его методы.
• Это называется неявной реализацией.

ПРЕИМУЩЕСТВА:
- Гибкость: можно реализовать интерфейс, не трогая исходный тип.
- Декомпозиция: автор библиотеки может не знать о твоих интерфейсах.
- Принцип "программируй к интерфейсу".

ПРИМЕР:
*/
type Stringer interface {
	String() string
}

type MyInt int

// MyInt реализует Stringer, хотя никто об этом не знает!
func (m MyInt) String() string {
	return fmt.Sprintf("MyInt(%d)", m)
}

func demonstrateImplicit() {
	var s Stringer = MyInt(42)
	fmt.Println(s.String()) // "MyInt(42)"
}

/*
ЯВНАЯ РЕАЛИЗАЦИЯ (EXPLICIT CHECK):

Хотя Go не требует явного объявления, можно проверить реализацию на этапе компиляции:

var _ MyInterface = (*MyType)(nil)

Это гарантирует, что *MyType реализует MyInterface.

ПРИМЕР:
*/
type Speaker interface {
	Speak() string
}

type Person struct{ Name string }

func (p Person) Speak() string { return "Hello" }

// Компилятор проверит, что *Person реализует Speaker
var _ Speaker = (*Person)(nil)

/*
ПОЧЕМУ ЭТО ВАЖНО?

1. Безопасность: если ты удалишь метод Speak(), код не скомпилируется.
2. Самодокументирование: любой разработчик сразу видит, какие интерфейсы реализует тип.
3. Лучше, чем runtime-ошибки!

АНТИПАТТЕРН:
- Не делай присваивание в main() или init() — это не проверка на этапе компиляции.
- Не используй пустые переменные без `_`.

ПРАВИЛЬНО:
var _ io.Reader = (*MyReader)(nil)
*/

func demonstrateExplicitCheck() {
	// Этот вызов не нужен — проверка уже сделана выше через var _ = ...
	p := Person{Name: "Ivan"}
	fmt.Println(p.Speak())
}

//3. Пустой интерфейс и типовые Ассерции
/*
ЧТО ТАКОЕ ПУСТОЙ ИНТЕРФЕЙС?

• Синтаксис: `interface{}` (в Go 1.18+ можно писать `any`)
• Это интерфейс без методов → может содержать значение любого типа.
• НЕ является "универсальным супертипом" — это просто механизм упаковки значения + типа.

ВНУТРЕННЕЕ УСТРОЙСТВО:
Пустой интерфейс в памяти — это две части:
1. Указатель на описание типа (type descriptor)
2. Указатель на данные (или само значение, если оно помещается в 8 байт)

→ Всегда занимает 16 байт на 64-битных системах, даже для `bool`!

Пример:
*/

func demEmptyInterface() {
	var b bool = true
	var i interface{} = b
	fmt.Printf("Размер bool: %d байт\n", unsafe.Sizeof(b))               // 1
	fmt.Printf("Размер interface{} с bool: %d байт\n", unsafe.Sizeof(i)) // 16
}

/*
ЗАЧЕМ НУЖЕН ПУСТОЙ ИНТЕРФЕЙС?

1. Универсальные функции (fmt.Printf, json.Marshal, context.WithValue)
2. Хранение значений неизвестного типа (например, в кэшах)
3. Работа с динамическими данными (парсинг JSON, YAML)

НО! Это всегда последнее средство — лучше использовать конкретные типы или параметризованные функции (generics).

ПРИМЕР ИСПОЛЬЗОВАНИЯ:
*/
func logEvent(event interface{}) {
	// Логируем любое событие
	fmt.Printf("Событие: %+v (тип: %T)\n", event, event)
}

type userCreated struct {
	ID string
}
type orderPlaced struct {
	Amount float64
}

func demonstrateUniversalFunction() {
	logEvent(userCreated{ID: "user123"})
	logEvent(orderPlaced{Amount: 99.99})
}

/*
ТИПОВАЯ АССЕРЦИЯ (TYPE ASSERTION)

СИНТАКСИС:
value, ok := interfaceValue.(ConcreteType)

• Безопасная форма: возвращает `(value, ok)` → проверяй `ok`!
• Опасная форма: `value := interfaceValue.(ConcreteType)` → паника при ошибке!

ПОЧЕМУ БЕЗОПАСНАЯ ФОРМА ОБЯЗАТЕЛЬНА?
Потому что компилятор не знает, какой тип внутри интерфейса во время выполнения.

Подводные камни:
Паника при неверно ассерции:
var v interface{} = "hello"
   n := v.(int)

ПРИМЕР:
*/

func demProcessValue(v interface{}) {
	// В данном примере можно использоват: switch x:= v.(type){}
	//  Безопасная ассерция
	if str, ok := v.(string); ok {
		fmt.Println("Строка:", str)
		return
	}
	//  Безопасная ассерция для числа
	if num, ok := v.(int); ok {
		fmt.Println("Число:", num)
		return
	}
	fmt.Println("Неизвестный тип")
}

/*
сравнение интерфейса с nil
Правильно: проверяй исходную переменную (err == nil), а не упакованную.

var err error = nil
var i interface{} = err
fmt.Println(i == nil) false
*/

/*
Чрезмерное использование вместо generics:
До Go 1.18 было оправдано, но сейчас:

ПРАКТИЧЕСКИЙ СОВЕТ:
Если ты пишешь код после Go 1.18 — используй generics вместо interface{}, где это возможно.

Плохой пример:
func Print(v interface{}){...}
Хороший пример:
func Print([T any](v T)){...}

Если возикает вопрос- когда использовать пустой интерфейс, а когда дженерики:
Если операции над значениями не зависят от их конкретного типа (только хранение или передача) → interface{}.
Если операции зависят от типа и одинаковы для разных типов → Generics.
*/

//3.ВСТРАИВАНИЕ ИНТЕРФЕЙСОВ
/*
ВСТРАИВАНИЕ ИНТЕРФЕЙСОВ:
- Интерфейсы можно встраивать друг в друга для создания иерархии.
- Позволяет строить более сложные интерфейсы из базовых.

ПРИМЕР:
*/
type Reader interface {
	Read() string
}
type Writer interface {
	Write(string2 string)
}

type ReadWriter interface {
	Reader
	Writer
}
type file struct {
}

func (f file) Read() string {
	return "data"
}

func (f file) Write(s string) {
	fmt.Println("Writing:", s)
}

func demonstrateEmbedding() {
	var rw ReadWriter = file{}
	rw.Read()
	rw.Write("Hello")
}

/*
АНТИПАТТЕРН: БОЛЬШИЕ ИНТЕРФЕЙСЫ

ПЛОХО:
type Service interface {
    Start()
    Stop()
    Restart()
    GetConfig()
    SetConfig()
    Log()
    Metrics()
    HealthCheck()
}

ХОРОШО:
type Starter interface { Start() }
type Stopper interface { Stop() }
type Configurable interface { GetConfig(); SetConfig() }
*/

/*
Глубокое понимание встраивания:
1.Создание иерархии интерфейсов
Встраивание интерфейсов позволяет создавать иерархию абстракций, аналогичную наследованию в ООП.
Однако, в отличие от наследования, встраивание не создаёт "родитель-дочерний" отношения, а просто комбинирует методы.
Пример 1-го пункта
*/
type Base interface {
	Method1()
}
type Intermediate interface {
	Base
	Method2()
}
type Advanced interface {
	Intermediate
	Method3()
}

// Advanced Эквивалент:
/*
type Advanced interface{
Method1()
Method2()
Method3()
}
*/

/*
2. Реализация через композицию:
Интерфейсы в Go прекрасно работают с композицией структур:
Пример 2-го пункта
*/
type Logger struct {
	//Поля для логгера
}

func (l Logger) Log(message string) {
	// Реализация логгирования
}

type Application struct {
	Logger // Встраивание структуры
}

func (a Application) Run() {
	a.Log("Starting application...")
}

/*
3. Множественная реализация интерфейсов
Одна структура может реализовывать несколько интерфейсов, что позволяет создавать гибкие и переиспользуемые компоненты.
Пример 2-го пункта

// Интерфейсы для работы с данными
type DataFetcher interface {
	Fetch() ([]byte, error)
}

type DataProcessor interface {
	Process(data []byte) ([]byte, error)
}

type DataSaver interface {
	Save(data []byte) error
}

// Комбинированный интерфейс
type DataPipeline interface {
	DataFetcher
	DataProcessor
	DataSaver
}

// Реализация для конкретного сценария
type ImageProcessor struct {
	source string
}

func (p ImageProcessor) Fetch() ([]byte, error) {
	return downloadImage(p.source)
}

func (p ImageProcessor) Process(data []byte) ([]byte, error) {
	return resizeImage(data)
}

func (p ImageProcessor) Save(data []byte) error {
	return saveImage(data, "output.jpg")
}

// Использование
func processImage(pipeline DataPipeline) {
	data, err := pipeline.Fetch()
	if err != nil {
		log.Fatal(err)
	}

	processed, err := pipeline.Process(data)
	if err != nil {
		log.Fatal(err)
	}

	if err := pipeline.Save(processed); err != nil {
		log.Fatal(err)
	}
}

func main() {
	processor := ImageProcessor{source: "input.jpg"}
	processImage(processor)
}
*/
/*
ПОДВОДНЫЕ КАМНИ:
Встраивание интерфейсов не решает проблему конфликтующих методов
Избыточное использование встраивания интерфейсов усложняет код
Встраивание не даёт доступа к внутренним деталям структуры
*/

// ВНУТРЕННЕЕ УСТРОЙСТВО И ПРОИЗВОДИТЕЛЬНОСТЬ
/*
ВНУТРЕННЕЕ УСТРОЙСТВО ИНТЕРФЕЙСОВ:
- Интерфейс в памяти — это два указателя:
   Указатель на тип (type)
   Указатель на данные (data)
- Это позволяет хранить данные любого типа.

ПОЧЕМУ ЭТО ВАЖНО:
- Понимание внутреннего устройства помогает избежать ошибок и оптимизировать код.
- Интерфейсы добавляют накладные расходы (вызов через интерфейс медленнее, чем прямой вызов).

ПРИМЕР АНАЛИЗА:
*/
func analyzeInterface() {
	var i interface{} = 42
	fmt.Printf("Size of interface{}: %d bytes\n", unsafe.Sizeof(i)) // 16 байт (2 указателя по 8 байт)

	// Интерфейс занимает 16 байт даже для малых типов
	var b bool = true
	var ib interface{} = b
	fmt.Printf("Size of bool: %d, Size of interface{} with bool: %d\n",
		unsafe.Sizeof(b), unsafe.Sizeof(ib)) // 1, 16
}

/*
ПРОИЗВОДИТЕЛЬНОСТЬ:
• Вызов метода через интерфейс требует поиска в itab (interface table).
• Go кэширует itab, но всё равно это медленнее прямого вызова.
• В hot-path (циклы, критичный код) — избегай интерфейсов.

ПРИМЕР:
*/
func benchmarkInterface() {
	var m interface{} = 42

	start := time.Now()
	for i := 0; i < 10000000; i++ {
		_ = m.(int) // Типовая ассерция
	}
	fmt.Printf("Type assertion: %v\n", time.Since(start))

	start = time.Now()
	for i := 0; i < 10000000; i++ {
		_ = i // Прямой доступ
	}
	fmt.Printf("Direct access: %v\n", time.Since(start))
}

// 4.ТЕСТИРОВАНИЕ ЧЕРЕЗ ИНТЕРФЕЙСЫ
/*
1. Определи интерфейс для зависимости
2. Реализуй настоящую и mock-версию
3. Внедри зависимость через конструктор

ПРИМЕР:
*/
/*
type User struct {
	name string
	age  int
}
type DB interface {
	GetUser(id int) (User, error)
}

type RealDB struct{ ... }
func (db RealDB) GetUser(id int) (User, error) { ... }

type MockDB struct{ Users map[int]User }
func (m MockDB) GetUser(id int) (User, error) {
	user, ok := m.Users[id]
	if !ok { return User{}, errors.New("not found") }
	return user, nil
}

func TestService(t *testing.T) {
	mock := MockDB{Users: map[int]User{1: {Name: "Test"}}}
	service := NewService(mock)
	// ...
}
*/
