// main.go
// Пример: Дашборд — топ-5 пользователей по сумме доставленных заказов
// Запуск: go run main.go

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // Драйвер PostgreSQL
)

// 1. Модель (структура) для результата запроса
type UserSpending struct {
	UserID        int64     `json:"user_id" db:"user_id"`
	UserName      string    `json:"user_name" db:"user_name"`
	TotalSpent    float64   `json:"total_spent" db:"total_spent"`
	OrdersCount   int       `json:"orders_count" db:"orders_count"`
	LastOrderDate time.Time `json:"last_order_date,omitempty" db:"last_order_date"`
}

// 2. Ответ API (для передачи на фронтенд)
type DashboardResponse struct {
	TopUsers    []UserSpending `json:"top_users"`
	GeneratedAt time.Time      `json:"generated_at"`
}

func main() {
	// 3. Подключение к PostgreSQL
	connStr := "postgres://postgres:password@localhost:5432/mydb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Ошибка подключения:", err)
	}
	defer db.Close()

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Fatal("Не могу пинговать БД:", err)
	}
	fmt.Println("Успешно подключено к PostgreSQL!")

	// 4. Выполняем запрос
	topUsers, err := getTopSpenders(db, 5)
	if err != nil {
		log.Fatal("Ошибка выполнения запроса:", err)
	}

	// 5. Формируем ответ для API
	response := DashboardResponse{
		TopUsers:    topUsers,
		GeneratedAt: time.Now(),
	}

	// 6. Сериализуем в JSON
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatal("Ошибка JSON:", err)
	}

	fmt.Println("=== Dashboard Response ===")
	fmt.Println(string(jsonData))
}

// 7. Функция, которая выполняет SQL-запрос и возвращает слайс структур
func getTopSpenders(db *sql.DB, limit int) ([]UserSpending, error) {
	// Подготовленный запрос (Prepared Statement) — безопасно, защита от SQL-инъекций
	query := `
        SELECT 
            u.id AS user_id,
            u.name AS user_name,
            COALESCE(SUM(oi.quantity * oi.price), 0) AS total_spent,
            COUNT(DISTINCT o.id) AS orders_count,
            MAX(o.created_at) AS last_order_date
        FROM users u
        LEFT JOIN orders o ON u.id = o.user_id AND o.status = 'delivered'
        LEFT JOIN order_items oi ON o.id = oi.order_id
        GROUP BY u.id, u.name
        HAVING COALESCE(SUM(oi.quantity * oi.price), 0) > 0
        ORDER BY total_spent DESC
        LIMIT $1
    `

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer rows.Close()

	// Сканируем строки в слайс структур
	var users []UserSpending
	for rows.Next() {
		var u UserSpending
		// Обрати внимание: last_order_date может быть NULL, если нет заказов
		var lastOrder sql.NullTime
		err := rows.Scan(
			&u.UserID,
			&u.UserName,
			&u.TotalSpent,
			&u.OrdersCount,
			&lastOrder,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования: %w", err)
		}
		// Если дата не NULL, сохраняем
		if lastOrder.Valid {
			u.LastOrderDate = lastOrder.Time
		}
		users = append(users, u)
	}

	// Проверяем ошибки после итерации
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении строк: %w", err)
	}

	return users, nil
}
