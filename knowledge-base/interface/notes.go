package _interface

import "fmt"

/*
ЛОВУШКА С NIL-ИНТЕРФЕЙСАМИ

  - Если присвоить nil-значение (например, error = nil) в переменную типа interface{},
    то интерфейс НЕ будет равен nil!
  - Причина: интерфейс хранит и тип, и значение. Если тип != nil → интерфейс != nil.

ПРИМЕР:
*/
func demonstrateNilInterfaceTrap() {
	var err error = nil
	var i interface{} = err

	fmt.Println("err == nil:", err == nil) // true
	fmt.Println("i == nil:", i == nil)     // false ← ЛОВУШКА!

	// Правильная проверка:
	if err != nil {
		// ...
	}
}

/*
ПРОИЗВОДИТЕЛЬНОСТЬ ВЫЗОВОВ ЧЕРЕЗ ИНТЕРФЕЙС

• Вызов метода через интерфейс требует поиска в itab (interface table).
• Go кэширует itab, но всё равно это медленнее прямого вызова (~2x).
• В hot-path (циклы, критичный код) — избегай интерфейсов.

*/
