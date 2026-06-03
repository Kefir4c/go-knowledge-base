package main

import (
	"fmt"
	"reflect"
	"strings"
)

//Рефлексия в Go: reflect.Type, reflect.Value, теги и цена
/*
Рефлексия — это способность программы исследовать свои собственные типы
и значения во время выполнения. В Go рефлексия реализована в пакете reflect.
*/

//1. Основные понятия: reflect.Type и reflect.Value

/*
reflect.type - Представляет тип Go-переменной (статическая информация) - int,string,struct{...}
reflect.value - Представляет значение переменной (динамические данные) - 141, "okey"
*/

//Получение Type и Value:

func tv() {
	var x float64 = 15.421
	t := reflect.TypeOf(x)  // Type
	v := reflect.ValueOf(x) // Value

	fmt.Println(t)        // float64
	fmt.Println(v)        // 15.421
	fmt.Println(v.Type()) // float64

	v = reflect.ValueOf(42)
	t = v.Type() // Получить тип из значения
	// Обратно: из Type нельзя получить Value.
}

//2. Kind

/*
Kind (род) — это мета-тип, который классифицирует Go-типы на ограниченное множество:
Int, Float64, String, Struct, Slice, Ptr, Interface, Func, Map, Chan и т.д.
(всего 26 значений)
*/

func inspect(v any) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Struct:
		fmt.Println("Это структура")
	case reflect.Slice:
		fmt.Println("Это слайс, длина:", rv.Len())
	case reflect.String:
		fmt.Println("Строка:", rv.String())
	}
}

// 3. Чтение тегов структур
/*
Теги (json:"name,omitempty") — это строки, прикреплённые
к полям структуры. Рефлексия позволяет их читать.
*/

type User struct {
	ID   int    `json:"id" db:"user_id"`
	Name string `json:"name,omitempty" db:"user_name"`
}

func readTags(v any) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fmt.Printf("Field: %s\n", field.Name)
		fmt.Printf("  json tag: %s\n", field.Tag.Get("json"))
		fmt.Printf("  db tag:   %s\n", field.Tag.Get("db"))
	}
}

func pr() {
	u := User{ID: 1, Name: "Alice"}
	readTags(u)
}

//4. Цена рефлексии
/*
Почему рефлексия медленная?

1. Динамическое приведение типов — компилятор не может оптимизировать код,
все операции идут через рантайм.

2. Проверка безопасности — рефлексия должна проверять, что вызов SetInt
действительно можно сделать на значении.

3. Выделение памяти — многие операции возвращают новые reflect.Value, создавая мусор.

4. Отсутствие inlining — компилятор не встраивает функции пакета reflect.
*/

//5. Когда рефлексия оправдана
/*
Сериализация/десериализация - Библиотека не может знать все типы заранее -
encoding/json, encoding/xml, gob

ORM - Маппинг структур на SQL без кодогенерации - GORM, Ent (частично)

Валидация - Универсальный валидатор, читающий теги - go-playground/validator

Инъекция зависимостей - Фреймворки, которые строят граф объектов - Google Wire (частично), Dig, Fx

Сравнение глубокое - reflect.DeepEqual - Тестирование, клонирование

Профилирование/интроспекция - Анализ типов во время выполнения - pprof, дебаггеры
*/

//1. Глубокое сравнение (reflect.DeepEqual)

// Стандартная функция из пакета reflect
// Позволяет сравнивать два любых значения, включая вложенные структуры, слайсы, карты

type ComplexData struct {
	Name string
	Tags []string
	Meta map[string]int
}

func primer() {
	a := ComplexData{
		Name: "test",
		Tags: []string{"go", "reflect"},
		Meta: map[string]int{"a": 1, "b": 2},
	}
	b := ComplexData{
		Name: "test",
		Tags: []string{"go", "reflect"},
		Meta: map[string]int{"a": 1, "b": 2},
	}
	fmt.Println(reflect.DeepEqual(a, b)) // true
	// Без рефлексии пришлось бы писать сложную рекурсивную логику для каждого типа
}

// PrintStruct выводит все поля переданной структуры (или указателя на структуру)
// с их типами, значениями и опциональными тегами.
func PrintStruct(v any) {
	rv := reflect.ValueOf(v)

	// Если передан указатель, разыменовываем
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			fmt.Println("nil pointer")
			return
		}
		rv = rv.Elem()
	}

	// Проверяем, что в итоге структура
	if rv.Kind() != reflect.Struct {
		fmt.Printf("Not a struct: %s\n", rv.Kind())
		return
	}

	rt := rv.Type()
	fmt.Printf("=== %s ===\n", rt.Name())

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		// Имя поля
		name := fieldType.Name

		// Тип поля (строка)
		typ := field.Type().String()

		// Значение поля с учётом рекурсивного вывода для вложенных структур
		valueStr := formatValue(field)

		// Чтение тега json (как пример)
		tag := fieldType.Tag.Get("json")
		tagInfo := ""
		if tag != "" {
			tagInfo = fmt.Sprintf(" (json:\"%s\")", tag)
		}

		fmt.Printf("  %s: %s = %s%s\n", name, typ, valueStr, tagInfo)
	}
}

// formatValue возвращает строковое представление значения, обрабатывая
// вложенные структуры, слайсы, карты, указатели и т.д.
func formatValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Struct:
		// Рекурсивно выводим поля вложенной структуры в компактном виде
		fields := make([]string, 0, v.NumField())
		for i := 0; i < v.NumField(); i++ {
			fields = append(fields, fmt.Sprintf("%s:%s", v.Type().Field(i).Name, formatValue(v.Field(i))))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	case reflect.Slice, reflect.Array:
		// Для слайсов и массивов показываем длину и элементы (первые 3)
		len := v.Len()
		if len == 0 {
			return "[]"
		}
		elemStrs := make([]string, 0, min(len, 3))
		for i := 0; i < min(len, 3); i++ {
			elemStrs = append(elemStrs, formatValue(v.Index(i)))
		}
		more := ""
		if len > 3 {
			more = ", ..."
		}
		return fmt.Sprintf("[%s%s]", strings.Join(elemStrs, ", "), more)
	case reflect.Map:
		// Выводим пары ключ-значение (первые 3)
		keys := v.MapKeys()
		if len(keys) == 0 {
			return "map[]"
		}
		pairs := make([]string, 0, min(len(keys), 3))
		for i := 0; i < min(len(keys), 3); i++ {
			k := keys[i]
			val := v.MapIndex(k)
			pairs = append(pairs, fmt.Sprintf("%s:%s", formatValue(k), formatValue(val)))
		}
		more := ""
		if len(keys) > 3 {
			more = ", ..."
		}
		return fmt.Sprintf("map{%s%s}", strings.Join(pairs, ", "), more)
	case reflect.Ptr:
		if v.IsNil() {
			return "nil"
		}
		return "*" + formatValue(v.Elem())
	default:
		// Базовые типы, строки, bool и т.д.
		return fmt.Sprintf("%v", v.Interface())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========== Примеры использования ==========

type Address struct {
	City  string `json:"city"`
	Zip   int    `json:"zip"`
	Empty bool
}

type Person struct {
	Name    string  `json:"name"`
	Age     int     `json:"age"`
	Address Address `json:"address"`
	Phones  []string
	Parents map[string]string
	Manager *Person
}

func main() {
	p := Person{
		Name: "Alice",
		Age:  30,
		Address: Address{
			City: "Moscow",
			Zip:  101000,
		},
		Phones:  []string{"+7-123-4567890", "+7-987-6543210"},
		Parents: map[string]string{"mother": "Elena", "father": "Ivan", "stepfather": "Peter"},
		Manager: nil,
	}
	PrintStruct(p)

	fmt.Println("\n--- Передадим указатель ---")
	PrintStruct(&p)

	fmt.Println("\n--- Вложенная структура с глубоким указателем ---")
	manager := Person{Name: "Bob", Age: 45}
	p.Manager = &manager
	PrintStruct(p)
}
