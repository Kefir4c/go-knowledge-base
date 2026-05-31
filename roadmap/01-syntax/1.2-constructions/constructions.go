package constructions

import (
	"fmt"
	"log"
	"os"
)

//ЗАДАЧА 1. ПЕРЕПИСАТЬ IF-ELSE ЧЕРЕЗ EARLY RETURN

// Исходный код (плохо)
func bedProcessUser(name string, age int) string {
	if name == "" {
		if age < 18 {
			return "invalid: empty name and underage"
		} else {
			return "invalid: empty name"
		}
	} else {
		if age < 18 {
			return "underage user"
		} else {
			return "valid user"
		}
	}
}

// Исправленный код (early return)
func goodProcessUser(name string, age int) string {
	// Early return для пустого имени
	if name == "" {
		if age < 18 {
			return "invalid: empty name and underage"
		}
		return "invalid: empty name"
	}

	// Early return для несовершеннолетних
	if age < 18 {
		return "underage user"
	}

	return "valid user"
}

//ЗАДАЧА 2. SWITCH С TYPE ASSERTION НА ANY (JSON-подобный парсер)

func extractStrings(data any) []string {
	var result []string

	switch v := data.(type) {
	case string:
		result = append(result, v)
	case []any:
		for _, item := range v {
			result = append(result, extractStrings(item)...)
		}
	case map[string]any:
		for _, value := range v {
			result = append(result, extractStrings(value)...)
		}
	}
	return result
}

// ЗАДАЧА 3. DEFER В ЦИКЛЕ — УТЕЧКА ФАЙЛОВЫХ ДЕСКРИПТОРОВ
func processFilesGood() {
	for i := 1; i < 1000; i++ {
		filename := fmt.Sprintf("file_%d.txt", i)

		//способ 1: анонимная функция
		func() {
			file, err := os.Open(filename)
			if err != nil {
				log.Printf("Error opening %s: %v", filename, err)
				return
			}
			defer file.Close()

			// Работа с файлом
			data := make([]byte, 1024)
			file.Read(data)
		}()

		// Правильный способ 2: явный Close (без defer)
		file, err := os.Open(filename)
		if err != nil {
			continue
		}
		// Работа с файлом
		data := make([]byte, 1024)
		file.Read(data)
		file.Close() // ✅ явное закрытие

	}
}
