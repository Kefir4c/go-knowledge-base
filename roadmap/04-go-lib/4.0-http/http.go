package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// напишем POST-запрос с использованием http.Client и функции http.NewRequest:

func primer1() {
	client := http.Client{}

	requestData := []byte(`{"somedata": "sometext"}`)

	request, err := http.NewRequest(http.MethodPost, "https://httpbin.org/post", bytes.NewBuffer(requestData))
	if err != nil {
		log.Fatal(err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(request.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(body))
}

// реализация интерфейса RoundTripper

type CustomTransport struct {
	Transport http.RoundTripper
}

func (c *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Логирование каждого запроса
	fmt.Println("Отправляется запрос на URL:", req.URL)

	// Дополнительная логика (например, добавление заголовков)
	req.Header.Add("X-Custom-Header", "CustomValue")

	return c.Transport.RoundTrip(req)
}

func primer2() {
	client := &http.Client{
		Transport: &CustomTransport{Transport: http.DefaultTransport}, // Используем кастомный транспорт
	}

	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Статус ответа:", resp.Status)
}

// настройка прокси-сервера для HTTP-запросов

func primer3() {
	// Вместо адреса ниже стоит указать адрес существующего прокси-сервера
	proxyURL, err := url.Parse("http://proxyserver:8080")
	if err != nil {
		fmt.Println("Ошибка при парсинге прокси:", err)
		return
	}

	// Структура клиента
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// Отправляем запрос через прокси
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Статус ответа:", resp.Status)

}

// пример простого HTTP-сервера с одним мультиплексором и обработчиком
func primer4() {
	// Создаем и возвращаем новый экземпляр структуры ServeMux
	mux := http.NewServeMux()

	// Создаем обработчик, перенаправляющий запросы по указанному URL с HTTP-кодом 308
	redirectHandler := http.RedirectHandler("http://google.com", http.StatusPermanentRedirect)

	// Регистрируем ранее созданный servemux по указанному URL
	mux.Handle("/redirect", redirectHandler)

	log.Print("Сервер запущен...")

	// Запуск сервера на порту 8080
	http.ListenAndServe(":8080", mux)
}

//напишем HTTP-сервер, иллюстрирующий передачу запроса по цепочке из двух middleware

// middlewareOne измеряет время выполнения запроса и логирует информацию о начале и конце обработки
func middlewaeOne(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now
		log.Printf("middlewareOne: начало обработки запроса в %s", start())

		// Передаём управление следующему обработчику
		next.ServeHTTP(w, r)

		// Рассчитываем и выводим время выполнения в наносекундах
		duration := time.Since(start()).Nanoseconds()
		log.Printf("middlewareOne: запрос обработан за %d наносекунд", duration)
	})
}

// middlewareTwo логирует информацию о маршруте и подтверждает успешную обработку
func middlewareTwo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("middlewareTwo: обработка маршрута %s", r.URL.Path)

		// Передаём управление следующему обработчику
		next.ServeHTTP(w, r)

		log.Print("middlewareTwo: запрос успешно обработан")
	})
}

// mainHandler — основной обработчик
func mainHandler(w http.ResponseWriter, r *http.Request) {
	log.Print("mainHandler: обработка запроса")
	w.Write([]byte("Запрос выполнен успешно\n"))
}

func primer5() {
	mux := http.NewServeMux()

	mh := http.HandlerFunc(mainHandler)

	// Формируем цепочку из middleware с основным запросом в конце
	mux.Handle("/", middlewaeOne(middlewareTwo(mh)))

	log.Print("Сервер запущен...")
	err := http.ListenAndServe(":8080", mux)
	log.Fatal(err)
}

func main() {}
