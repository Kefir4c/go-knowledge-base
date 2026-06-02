package deepgo

/*
Интерфейсы должны быть маленькими — один-два метода, максимум.
io.Reader и io.Writer — эталон такого подхода. Функции принимают
интерфейсы и возвращают структуры — это даёт гибкость и свободу
расширения. Интерфейс определяет потребитель, а не поставщик —
твой код сам решает, какие методы ему нужны, и сторонние
библиотеки подстраиваются под твой интерфейс через адаптеры.
Эти правила делают Go-код гибким, тестируемым и легко поддерживаемым
*/

// Внешняя библиотека (представь, что это github.com/some/sms)
type SMSClient struct{}

func (c *SMSClient) Send(phone, text string) error { return nil }
func (c *SMSClient) GetBalance() (int, error)      { return 100, nil }
func (c *SMSClient) SetFrom(from string)           {}
func (c *SMSClient) SetProvider(p string)          {}

// ============================================
// ТВОЙ КОД: интерфейс на стороне потребителя
// ============================================

// Интерфейс, который нужен ТВОЕМУ сервису (только Send)
type SMSSender interface {
	Send(phone, text string) error
}

// Адаптер под внешнюю библиотеку
type SMSAdapter struct {
	client *SMSClient
}

func NewSMSAdapter() *SMSAdapter {
	client := &SMSClient{}
	client.SetFrom("MyService")
	client.SetProvider("twilio")
	return &SMSAdapter{client: client}
}

func (a *SMSAdapter) Send(phone, text string) error {
	return a.client.Send(phone, text)
}

// Внешняя библиотека (github.com/some/httpclient)
type HTTPClient struct{}

func (c *HTTPClient) Get(url string) ([]byte, error)     { return nil, nil }
func (c *HTTPClient) Post(url string, body []byte) error { return nil }
func (c *HTTPClient) SetTimeout(seconds int)             {}
func (c *HTTPClient) SetHeader(key, value string)        {}
func (c *HTTPClient) Close() error                       { return nil }
func (c *HTTPClient) GetStatus() int                     { return 200 }

type MyHTTPClient interface {
	Get(url string) ([]byte, error)
	Post(url string, body []byte) error
}

type HTTPAdapter struct {
	client *HTTPClient
}

func NewHTTPAdapter() *HTTPAdapter {
	c := &HTTPClient{}
	c.SetTimeout(10)
	c.SetHeader("Content-Type", "application/json")
	return &HTTPAdapter{client: c}
}

func (h *HTTPAdapter) Get(url string) ([]byte, error) {
	return h.client.Get(url)
}

func (a *HTTPAdapter) Post(url string, body []byte) error {
	return a.client.Post(url, body)
}

// 3. Функция, использующая интерфейс
type User struct {
	Name string
	Age  int
}

func FetchUser(client MyHTTPClient, id string) (*User, error) {
	data, err := client.Get("https://api.example.com/users/" + id)
	if err != nil {
		return nil, err
	}
	_ = data
	// парсим data в User...
	return &User{}, nil
}

// ========== ТВОЁ ЗАДАНИЕ ==========
// 1. Определи интерфейс MyHTTPClient с методами, которые реально нужны
//    (например, только Get и Post)
//
// 2. Напиши адаптер, который реализует этот интерфейс, оборачивая HTTPClient
//
// 3. Напиши функцию FetchUser, которая использует интерфейс
//    (должна принимать MyHTTPClient и возвращать User)
