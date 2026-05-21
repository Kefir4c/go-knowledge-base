package main

import (
	"fmt"
	"strings"
)

func main() {
	demHash()

	// Первый вызов — скачает из интернета
	fmt.Println(getPage("google.com"))

	// Второй вызов — достанет мгновенно из кэша за O(1)
	fmt.Println(getPage("google.com"))
}

func demHash() {
	hash := make(map[string]float64)
	hash["апельсин"] = 0.67
	hash["молоко"] = 1.49
	hash["авокадо"] = 1.49
	fmt.Println(hash)
	fmt.Println(hash["апельсин"])

	if _, ok := hash["банан"]; !ok {
		fmt.Println("нету банана")
	} else {
		fmt.Println("есть банана")
	}
}

var cache = make(map[string]string)

func getPage(url string) string {
	if page, ok := cache[url]; ok {
		fmt.Println("Достали из кэша!")
		return page
	}
	data := fmt.Sprintf("Данныйе сайта: %s", url)
	cache[url] = data

	fmt.Println("Скачали из интернета и сохранили в кэш!")
	return data
}

func hashPr4(arr []string) {
	// 1. Создаем карту "буква -> простое число" как в учебнике
	primes := map[rune]int{
		'a': 2, 'b': 3, 'c': 5, 'd': 7, 'e': 11, 'f': 13, 'g': 17, 'h': 19,
		'i': 23, 'j': 29, 'k': 31, 'l': 37, 'm': 41, 'n': 43, 'o': 47, 'p': 53,
		'q': 59, 'r': 61, 's': 67, 't': 71, 'u': 73, 'v': 79, 'w': 83, 'x': 89,
		'y': 97, 'z': 101,
	}
	hashTables := 10

	// Создаем "корзины" (buckets) для нашей таблицы
	buckets := make([][]string, hashTables)

	fmt.Println("Процесс хэширования:")
	for _, val := range arr {
		sum := 0
		// Считаем сумму простых чисел для каждой буквы слова
		for _, char := range strings.ToLower(val) {
			if val, ok := primes[char]; ok {
				sum += val
			}
		}

		// Вычисляем индекс: Остаток от деления суммы на размер таблицы
		index := sum % hashTables
		fmt.Printf("Слово: %-8s | Сумма: %3d | Индекс (Сумма %% 10): %d\n", val, sum, index)
		// Кладем слово в нужную ячейку
		buckets[index] = append(buckets[index], val)
	}

	// Выводим финальный результат
	fmt.Println("Финальная хэш-таблица в памяти:")
	for ind, count := range buckets {
		fmt.Printf("Ячейка %d: %v\n", ind, count)
	}
}
