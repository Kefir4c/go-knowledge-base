package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

/*
 ПАКЕТ ENCODING/JSON — ПОЛНОЕ РУКОВОДСТВО

 Пакет encoding/json — один из самых важных в Go. Используется в 90% проектов
 для обмена данными между сервисами, хранения конфигов, работы с API.

 ОСНОВНЫЕ ТЕРМИНЫ:
   - Marshal — преобразование Go-структуры в JSON (сериализация)
   - Unmarshal — преобразование JSON в Go-структуру (десериализация)
   - Теги (tags) — метаданные для управления сериализацией
   - RawMessage — отложенный парсинг JSON
   - Decoder/Encoder — потоковая обработка JSON
*/

/*
1. БАЗОВЫЕ ОПЕРАЦИИ: MARSHAL И UNMARSHAL

Marshal — превращает Go-структуру в JSON ([]byte).
Unmarshal — превращает JSON ([]byte) в Go-структуру.

ВАЖНЫЕ ПРАВИЛА:
  - Поля структуры должны быть экспортируемыми (с большой буквы)
  - Неэкспортируемые поля игнорируются
  - Для неэкспортируемых полей можно использовать теги, но это не сработает
  - Числа в JSON при Unmarshal в interface{} всегда становятся float64

СИНТАКСИС:
  data, err := json.Marshal(&myStruct)
  err := json.Unmarshal([]byte(jsonString), &myStruct)

ОШИБКИ:
  - json: unsupported type — попытка сериализовать chan, func или complex
  - json: cannot unmarshal object into Go struct field — несовпадение типов
  - runtime error: invalid memory address — передача nil указателя в Unmarshal
*/

/*
2. ТЕГИ JSON (STRUCT TAGS)

Теги управляют тем, как поля структуры отображаются в JSON.

СИНТАКСИС ТЕГОВ:
  `json:"имя_в_json,опции"`

ОПЦИИ:
  - omitempty  — пропускать поле, если оно пустое (zero value)
  - -          — полностью игнорировать поле (не включать в JSON)
  - string     — записывать число как строку (для совместимости с JS)
  - ,omitempty — только omitempty, имя оставить как в Go

ПРИМЕРЫ ТЕГОВ:
  type User struct {
      ID       int    `json:"id"`                 // всегда включать
      Name     string `json:"name,omitempty"`     // пропускать если ""
      Email    string `json:"email,omitempty"`    // пропускать если ""
      Password string `json:"-"`                  // никогда не включать
      Age      int    `json:"age,omitempty"`      // пропускать если 0
      Active   bool   `json:"active"`             // всегда (false тоже включается)
  }

ПРАВИЛА ОМИТЕМПТИ:
  Поле считается пустым, если:
    - для числовых типов: 0
    - для строк: ""
    - для булевых: false
    - для указателей: nil
    - для слайсов/мапов: nil или len=0
    - для структур: все поля пустые (рекурсивно)

ЧАСТАЯ ОШИБКА:
  omitempty не работает, если поле инициализировано не zero value.
  Например: Age int `json:"age,omitempty"` — если Age = 0, оно будет пропущено,
  но если Age = -1, оно будет включено в JSON.
*/

/*
3. ВЛОЖЕННЫЕ СТРУКТУРЫ И ANONYMOUS FIELDS

Вложенные структуры сериализуются как вложенные объекты в JSON.

ОБЫЧНОЕ ВЛОЖЕНИЕ:
  type Address struct {
      City   string `json:"city"`
      Street string `json:"street"`
  }

  type User struct {
      Name    string  `json:"name"`
      Address Address `json:"address"` // вложенный объект
  }

  JSON:
  {
      "name": "Alice",
      "address": {
          "city": "Moscow",
          "street": "Tverskaya"
      }
  }

ANONYMOUS FIELD (встраивание):
  Если поле структуры не имеет имени, его поля поднимаются на уровень выше.

  type User struct {
      Name string `json:"name"`
      Address `json:",inline"` // поля Address станут полями User
  }

  JSON:
  {
      "name": "Alice",
      "city": "Moscow",
      "street": "Tverskaya"
  }

  ВАЖНО: inline работает только при Unmarshal, при Marshal — тоже.
*/

/*
4. JSON.RAWMESSAGE — ОТЛОЖЕННЫЙ ПАРСИНГ

RawMessage — это тип, который хранит сырые JSON-данные ([ ]byte)
и позволяет отложить их парсинг до момента, когда станет известна структура.

КОГДА ИСПОЛЬЗОВАТЬ:
  - Структура JSON зависит от значения другого поля (discriminator)
  - Нужно сначала прочитать одно поле, чтобы понять, как парсить остальные
  - Парсинг части данных, а остальное оставить для дальнейшей обработки

ПРИМЕР:
  type Event struct {
      Type string          `json:"type"`
      Data json.RawMessage `json:"data"` // сырые данные
  }

  type UserEvent struct {
      ID   int    `json:"id"`
      Name string `json:"name"`
  }

  type OrderEvent struct {
      OrderID string  `json:"order_id"`
      Total   float64 `json:"total"`
  }

  func processEvent(data []byte) {
      var event Event
      json.Unmarshal(data, &event)

      switch event.Type {
      case "user":
          var user UserEvent
          json.Unmarshal(event.Data, &user)
          // обрабатываем user
      case "order":
          var order OrderEvent
          json.Unmarshal(event.Data, &order)
          // обрабатываем order
      }
  }

ВАЖНО:
  - RawMessage не парсится автоматически, нужно вызывать Unmarshal явно
  - RawMessage можно использовать несколько раз для разных структур
*/

/*
5. КАСТОМНЫЙ MARSHALJSON И UNMARSHALJSON

Когда стандартное поведение не подходит, можно реализовать свои методы.

КОГДА НУЖНО:
  - Нестандартный формат дат/времени
  - Специальная обработка null значений
  - Расшифровка/шифрование полей
  - Валидация данных при Unmarshal
  - Преобразование между разными форматами

ПРИМЕР С ДАТОЙ (RFC3339):
  type CustomTime time.Time

  func (t CustomTime) MarshalJSON() ([]byte, error) {
      if time.Time(t).IsZero() {
          return []byte("null"), nil
      }
      return []byte(`"` + time.Time(t).Format(time.RFC3339) + `"`), nil
  }

  func (t *CustomTime) UnmarshalJSON(data []byte) error {
      if string(data) == "null" {
          *t = CustomTime(time.Time{})
          return nil
      }
      // Убираем кавычки
      str := string(data)
      if len(str) < 2 || str[0] != '"' || str[len(str)-1] != '"' {
          return fmt.Errorf("invalid time format: %s", str)
      }
      str = str[1 : len(str)-1]
      parsed, err := time.Parse(time.RFC3339, str)
      if err != nil {
          return err
      }
      *t = CustomTime(parsed)
      return nil
  }

ПРИМЕР С ВАЛИДАЦИЕЙ:
  type User struct {
      Name string `json:"name"`
  }

  func (u *User) UnmarshalJSON(data []byte) error {
      type Alias User // чтобы избежать рекурсии
      var aux Alias
      if err := json.Unmarshal(data, &aux); err != nil {
          return err
      }
      if aux.Name == "" {
          return fmt.Errorf("name is required")
      }
      *u = User(aux)
      return nil
  }

ВАЖНО:
  - MarshalJSON должен возвращать []byte с валидным JSON
  - UnmarshalJSON должен принимать указатель на структуру
  - Чтобы избежать рекурсии, используют вспомогательный тип-алиас
*/

/*
6. СТРИМИНГ: JSON.DECODER И JSON.ENCODER

Decoder и Encoder позволяют обрабатывать JSON по частям, не загружая
всё в память. Это критично для больших файлов или потоковых данных.

КОГДА ИСПОЛЬЗОВАТЬ:
  - Большие файлы (GB)
  - Стриминговые API
  - Постоянные потоки данных (websocket, log streams)
  - Когда нужно обрабатывать данные по частям

JSON.DECODER (чтение):
  dec := json.NewDecoder(reader)
  for {
      var item Item
      if err := dec.Decode(&item); err == io.EOF {
          break
      } else if err != nil {
          return err
      }
      // обрабатываем item
  }

JSON.ENCODER (запись):
  enc := json.NewEncoder(writer)
  enc.SetIndent("", "  ")   // красивое форматирование
  enc.SetEscapeHTML(false)  // отключить экранирование HTML (для производительности)
  for _, item := range items {
      if err := enc.Encode(item); err != nil {
          return err
      }
  }

СРАВНЕНИЕ С MARSHAL/UNMARSHAL:
  Marshal/Unmarshal   — всё в памяти, просто, быстро для маленьких данных
  Decoder/Encoder     — потоково, память не растёт, но чуть медленнее

ПРИМЕР: ПАРСИНГ БОЛЬШОГО ФАЙЛА ПОСТРОЧНО
  file, _ := os.Open("large_file.json")
  defer file.Close()
  dec := json.NewDecoder(file)

  // Читаем открывающую скобку массива
  if _, err := dec.Token(); err != nil {
      return err
  }

  for dec.More() {
      var item Item
      if err := dec.Decode(&item); err != nil {
          return err
      }
      // обрабатываем item
  }

  // Читаем закрывающую скобку массива
  if _, err := dec.Token(); err != nil {
      return err
  }

НАСТРОЙКИ DECODER:
  dec.DisallowUnknownFields()  // строгий режим — ошибка на неизвестные поля
  dec.UseNumber()              // использовать json.Number вместо float64
*/

/*
7. РАБОТА С MAP И INTERFACE{}

Когда структура неизвестна заранее или динамическая, используют
map[string]interface{} или interface{}.

ПРИМЕР С MAP:
  jsonStr := `{"name":"Alice","age":30,"active":true}`
  var result map[string]interface{}
  json.Unmarshal([]byte(jsonStr), &result)

  name := result["name"].(string)
  age := result["age"].(float64)  // числа всегда float64!
  active := result["active"].(bool)

ВАЖНО:
  - Числа в JSON всегда становятся float64 при Unmarshal в interface{}
  - Чтобы получить int, нужно преобразовывать: int(age)
  - Для больших чисел используй json.Number

ИСПОЛЬЗОВАНИЕ JSON.NUMBER:
  dec := json.NewDecoder(reader)
  dec.UseNumber()  // числа будут json.Number, а не float64

  var data map[string]interface{}
  dec.Decode(&data)
  age := data["age"].(json.Number)
  ageInt, _ := age.Int64()

КОГДА ИСПОЛЬЗОВАТЬ:
  - Неизвестная структура ответа API
  - Динамические данные
  - Прототипирование (лучше потом переписать на структуры)

КОГДА НЕ ИСПОЛЬЗОВАТЬ:
  - Известная структура — всегда используй структуры (типобезопасно)
  - Продакшен-код — map и interface{} сложнее поддерживать
=============================================================================
*/

/*
8. НАСТРОЙКИ И ОПЦИИ
ПОЛЕЗНЫЕ НАСТРОЙКИ:

Encoder:
  enc.SetIndent("", "  ")          // красивое форматирование
  enc.SetEscapeHTML(false)         // отключить экранирование <,>,&

Decoder:
  dec.DisallowUnknownFields()      // строгий режим
  dec.UseNumber()                  // числа как json.Number

ГЛОБАЛЬНЫЕ НАСТРОЙКИ:
  json.Unmarshal(data, &v)          // стандартный
  json.Marshal(v)                   // стандартный

СРАВНЕНИЕ СКОРОСТИ:
  json.Marshal/Unmarshal — быстрее для маленьких данных
  json.Encoder/Decoder   — быстрее для больших данных (потоково)
=============================================================================
*/

/*
9. ТИПИЧНЫЕ ОШИБКИ И ИХ РЕШЕНИЕ

ОШИБКА 1: json: cannot unmarshal object into Go struct field
  ПРИЧИНА: Имена полей не совпадают или поле не экспортируемое
  РЕШЕНИЕ: Проверь теги и регистр первой буквы поля

ОШИБКА 2: panic: runtime error: invalid memory address
  ПРИЧИНА: Передал не указатель в Unmarshal
  РЕШЕНИЕ: Используй &myStruct

ОШИБКА 3: json: unsupported type
  ПРИЧИНА: Попытка сериализовать chan, func или complex
  РЕШЕНИЕ: Не используй такие типы в структурах для JSON

ОШИБКА 4: omitempty не работает
  ПРИЧИНА: Поле инициализировано не zero value
  РЕШЕНИЕ: Проверь, что поле действительно пустое

ОШИБКА 5: json: cannot unmarshal number into string
  ПРИЧИНА: Тип поля не совпадает с JSON
  РЕШЕНИЕ: Проверь типы полей

ОШИБКА 6: УТЕЧКА ПАМЯТИ ПРИ БОЛЬШИХ ФАЙЛАХ
  ПРИЧИНА: Использование Marshal/Unmarshal для больших данных
  РЕШЕНИЕ: Используй Decoder/Encoder

ОШИБКА 7: ПОТЕРЯ ТОЧНОСТИ ЧИСЕЛ
  ПРИЧИНА: Использование float64 для больших чисел
  РЕШЕНИЕ: Используй json.Number или int64

ОШИБКА 8: ЦИКЛИЧЕСКАЯ ССЫЛКА В МАРШАЛИНГЕ
  ПРИЧИНА: Структура ссылается сама на себя
  РЕШЕНИЕ: Используй RawMessage или избегай циклических ссылок
=============================================================================
*/

/*
10. ШПАРГАЛКА ДЛЯ СОБЕСЕДОВАНИЯ

Marshal vs Unmarshal:
  Marshal   — структура → JSON (сериализация)
  Unmarshal — JSON → структура (десериализация)

Теги:
  json:"name,omitempty" — переименовать и пропускать пустые
  json:"-"              — игнорировать поле

RawMessage:
  Отложенный парсинг. Полезно, когда структура зависит от другого поля.

Кастомный MarshalJSON/UnmarshalJSON:
  Для нестандартных форматов (даты, времена, валидация).

Decoder/Encoder:
  Для потоковой обработки (не загружать всё в память).

DisallowUnknownFields:
  Строгий режим — ошибка при неизвестных полях.

Числа в interface{}:
  Всегда float64, даже если были int.

Правила:
  1. Поля должны быть экспортируемыми (с большой буквы)
  2. Всегда передавай указатель в Unmarshal
  3. Для больших данных используй Decoder/Encoder
  4. Используй структуры, а не map, когда структура известна
=============================================================================
*/

// 1. БАЗОВЫЙ MARSHAL/UNMARSHAL
type User struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreadetAt string `json:"created_at"`
}

func primer1() {
	// Unmarshal (JSON → структура)
	jsonStr := `{"id":1,"name":"Alice","created_at":"2024-01-01T00:00:00Z"}`
	var user User
	if err := json.Unmarshal([]byte(jsonStr), &user); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User: %+v\n", user)

	user2 := User{ID: 2, Name: "Kolya", Email: "Sema.m@yandex.ru", CreadetAt: "2024-01-02T00:00:00Z"}
	data, err := json.Marshal(user2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("JSON:", string(data))
}

// 2. ВАЛИДАЦИЯ ПРИ UNMARSHAL (ЧАСТО НА СОБЕСАХ)
type Order struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

// Кастомный UnmarshalJSON с валидацией
func (o *Order) UnmarshalJSON(data []byte) error {
	// Алиас, чтобы избежать рекурсии
	type Alias Order
	var aux Alias

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Валидация: сумма не может быть отрицательной
	if aux.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: %f", aux.Amount)
	}

	*o = Order(aux)
	return nil
}

func primer2() {
	// Корректный JSON
	valid := `{"id":"123","amount":99.99}`
	var order Order
	json.Unmarshal([]byte(valid), &order)
	fmt.Printf("Valid order: %+v\n", order)

	// Отрицательная сумма → ошибка валидации
	invalid := `{"id":"124","amount":-10.50}`
	err := json.Unmarshal([]byte(invalid), &order)
	fmt.Printf("Invalid order error: %v\n", err)
}

// 3. ИЗБЕГАНИЕ РЕКУРСИИ ПРИ КАСТОМНОМ MARSHAL (ЧАСТАЯ ЛОВУШКА)
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// Кастомный MarshalJSON (добавляет поле "currency")
func (p Product) MarshalJSON() ([]byte, error) {
	type Alias Product

	return json.Marshal(&struct {
		Alias
		Currency string `json:"currency"`
	}{
		Alias:    Alias(p),
		Currency: "USD",
	})
}

func primer3() {
	p := Product{ID: 1, Name: "Laptop", Price: 999.99}
	data, _ := json.Marshal(p)
	fmt.Println("Product with currency:", string(data))
}

// 4. СТРИМИНГ БОЛЬШОГО ФАЙЛА (ПРОДАКШЕН)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// Чтение большого JSON-файла (массива объектов) без загрузки в память
func readLargeJSON(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)

	// Проверяем, что это массив
	if _, err := dec.Token(); err != nil {
		return err
	}

	count := 0
	for dec.More() {
		var entry LogEntry
		if err := dec.Decode(&entry); err != nil {
			return err
		}
		count++
		// Обрабатываем entry (например, отправляем в БД)
		if count%1000 == 0 {
			fmt.Printf("Processed %d entries\n", count)
		}
	}

	// Читаем закрывающую скобку
	if _, err := dec.Token(); err != nil {
		return err
	}

	fmt.Printf("Total entries: %d\n", count)
	return nil
}

// Запись большого JSON-файла (массива объектов) без загрузки в память
func writeLargeJSON(filename string, entries []LogEntry) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)

	// Пишем начало массива
	file.WriteString("[\n")

	for i, entry := range entries {
		if i > 0 {
			file.WriteString(",\n")
		}
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}

	file.WriteString("\n]")
	return nil
}

func primer4() {
	// Имитация данных
	entries := []LogEntry{
		{"2024-01-01T00:00:00Z", "INFO", "Server started"},
		{"2024-01-01T00:00:01Z", "INFO", "Health check passed"},
	}

	err := writeLargeJSON("logs.json", entries)
	if err != nil {
		log.Fatal(err)
	}

	err = readLargeJSON("logs.json")
	if err != nil {
		log.Fatal(err)
	}

	os.Remove("logs.json")
}

// 5. HTTP-ХЕНДЛЕР С JSON
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Created string `json:"created"`
}

// Хендлер, который принимает JSON и возвращает JSON
func createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем JSON из тела запроса
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Читаем JSON из тела запроса
	}

	// Валидация
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Создаём пользователя (имитация)
	resp := CreateUserResponse{
		ID:      1,
		Name:    req.Name,
		Created: time.Now().Format(time.RFC3339),
	}

	// Отправляем JSON обратно (Encoder)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func primer5() {
	// Имитация HTTP-запроса к хендлеру
	reqBody := `{"name":"Alice","email":"alice@example.com"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	createUserHandler(w, req)

	fmt.Println("HTTP Response status:", w.Code)
	fmt.Println("HTTP Response body:", w.Body.String())
}

// 6. UNMARSHAL В INTERFACE{} (КОГДА НЕ ЗНАЕШЬ СТРУКТУРУ)
func primer6() {
	jsonStr := `{"type":"user","data":{"id":1,"name":"Alice"}}`

	var result map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &result)

	if typ, ok := result["type"].(string); ok && typ == "user" {
		data := result["data"].(map[string]interface{})
		id := data["id"].(float64) // числа всегда float64
		name := data["name"].(string)
		fmt.Printf("Dynamic parse: ID=%d, Name=%s\n", int(id), name)
	}
}

// 7. ОБРАБОТКА NULL В JSON
type Person struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Score *int   `json:"score,omitempty"` // указатель, чтобы отличить null от 0
}

func primer7() {
	// nil в JSON → nil в Go
	jsonWithNull := `{"name":"Alice","age":30,"score":null}`
	var p1 Person
	json.Unmarshal([]byte(jsonWithNull), &p1)
	fmt.Printf("With null: Name=%s, Age=%d, Score=%v\n", p1.Name, p1.Age, p1.Score)

	// Отсутствие поля → nil в Go
	jsonWithoutField := `{"name":"Bob","age":25}`
	var p2 Person
	json.Unmarshal([]byte(jsonWithoutField), &p2)
	fmt.Printf("Without field: Name=%s, Age=%d, Score=%v\n", p2.Name, p2.Age, p2.Score)

	// Число → указатель не nil
	jsonWithScore := `{"name":"Charlie","age":35,"score":100}`
	var p3 Person
	json.Unmarshal([]byte(jsonWithScore), &p3)
	fmt.Printf("With score: Name=%s, Age=%d, Score=%d\n", p3.Name, p3.Age, *p3.Score)
}

// 8. JSON.RAWМESSAGE
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"` // отложенный парсинг
}

type UserCreatedEvent struct {
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
}

type OrderPlacedEvent struct {
	OrderID string  `json:"order_id"`
	Total   float64 `json:"total"`
}

func processEvent(eventData []byte) {
	var event Event
	json.Unmarshal(eventData, &event)

	switch event.Type {
	case "user_created":
		var data UserCreatedEvent
		json.Unmarshal(event.Data, &data)
		fmt.Printf("User created: ID=%d, Name=%s\n", data.UserID, data.Name)

	case "order_placed":
		var data OrderPlacedEvent
		json.Unmarshal(event.Data, &data)
		fmt.Printf("Order placed: ID=%s, Total=%.2f\n", data.OrderID, data.Total)

	default:
		fmt.Printf("Unknown event type: %s\n", event.Type)
	}
}
func primer8() {
	events := [][]byte{
		[]byte(`{"type":"user_created","data":{"user_id":1,"name":"Alice"}}`),
		[]byte(`{"type":"order_placed","data":{"order_id":"ORD-001","total":99.99}}`),
		[]byte(`{"type":"unknown","data":{"foo":"bar"}}`),
	}

	for _, e := range events {
		processEvent(e)
	}
}

// 9. JSON NUMBER

func primer9() {
	// Без UseNumber — число становится float64 (теряем точность)
	jsonStr := `{"big":12345678901234567890}`

	// Вариант 1: без UseNumber (потеря точности)
	var data1 map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &data1)
	big1 := data1["big"].(float64)
	fmt.Printf("Without UseNumber: %.0f (может потерять точность)\n", big1)

	// Вариант 2: с UseNumber (сохраняем точность)
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonStr)))
	dec.UseNumber()
	var data2 map[string]interface{}
	dec.Decode(&data2)
	big2 := data2["big"].(json.Number)
	intVal, _ := big2.Int64()
	fmt.Printf("With UseNumber: %d (точность сохранена)\n", intVal)
}

// 10. DISALLOWUNKNOWNFIELDS (СТРОГИЙ РЕЖИМ)
type StrictUser struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func primer10() {
	jsonStr := `{"name":"Alice","age":30,"unknown_field":"some_value"}`

	// Обычный Unmarshal — игнорирует неизвестные поля
	var user1 StrictUser
	json.Unmarshal([]byte(jsonStr), &user1)
	fmt.Printf("Without strict: %+v\n", user1)

	// Строгий режим — ошибка на неизвестные поля
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonStr)))
	dec.DisallowUnknownFields()

	var user2 StrictUser
	err := dec.Decode(&user2)
	fmt.Printf("With strict: error = %v\n", err)
}

// MarshalJSON для time.Time в формате dd.MM.yyyy
type Date time.Time

func (d Date) MarshalJSON() ([]byte, error) {
	t := time.Time(d)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Format("02.01.2026") + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = Date(time.Time{})
		return nil
	}

	str := string(data)
	if len(str) > 1 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	t, err := time.Parse("02.01.2006", str)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

func (d Date) String() string {
	return time.Time(d).Format("02.01.2006")
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
	fmt.Println("primer10")
	primer10()
}
