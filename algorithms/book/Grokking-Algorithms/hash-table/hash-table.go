package main

import "fmt"

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
