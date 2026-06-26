package join

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// МОДЕЛИ (СТРУКТУРЫ ДЛЯ РАЗНЫХ СЦЕНАРИЕВ JOIN)

// User — основная модель пользователя
type User struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
}

// Order — основная модель заказа
type Order struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Amount    float64   `db:"amount"`
	CreatedAt time.Time `db:"created_at"`
}

// UserWithOrders — пользователь с его заказами (для JOIN)
type UserWithOrders struct {
	User
	Orders     []Order
	TotalSpent float64 // вычисляемое поле
}

// OrderWithItems — заказ с товарами и пользователем (для JOIN трёх таблиц)
type OrderWithItems struct {
	Order
	UserName string `db:"user_name"`
	Items    []OrderItemWithProduct
}

// OrderItemWithProduct — товар в заказе с названием продукта
type OrderItemWithProduct struct {
	ID          int64   `db:"id"`
	OrderID     int64   `db:"order_id"`
	ProductID   int64   `db:"product_id"`
	Quantity    int     `db:"quantity"`
	Price       float64 `db:"price"`
	ProductName string  `db:"product_name"`
}

// EmployeeWithManager — сотрудник с именем руководителя (SELF JOIN)
type EmployeeWithManager struct {
	ID          int64   `db:"id"`
	Name        string  `db:"name"`
	Position    string  `db:"position"`
	Salary      float64 `db:"salary"`
	ManagerID   *int64  `db:"manager_id"`
	ManagerName string  `db:"manager_name"`
}

// UserStats — статистика по пользователю (JOIN + агрегация)
type UserStats struct {
	Name       string  `db:"name"`
	OrderCount int     `db:"order_count"`
	TotalSpent float64 `db:"total_spent"`
	AvgOrder   float64 `db:"avg_order"`
}

// 1. INNER JOIN: ПОЛЬЗОВАТЕЛИ С ЗАКАЗАМИ
// GetUsersWithOrders возвращает пользователей, у которых есть хотя бы один заказ.
// Использует INNER JOIN + группировку.
func (r *UserRepo) GetUsersWithOrders(ctx context.Context) ([]UserWithOrders, error) {
	// Вариант 1: два запроса (просто, но два Round-Trip)
	// 1. Получаем пользователей
	// 2. Получаем их заказы

	// Вариант 2: один запрос с JOIN и сборкой в коде (рекомендуется)
	query := `
        SELECT
            u.id, u.name, u.email, u.created_at,
            o.id AS order_id,
            o.user_id AS order_user_id,
            o.amount AS order_amount,
            o.created_at AS order_created_at
        FROM users u
        INNER JOIN orders o ON u.id = o.user_id
        ORDER BY u.id, o.created_at DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users with orders: %w", err)
	}
	defer rows.Close()

	// Карта для сборки пользователей
	usersMap := make(map[int64]*UserWithOrders)

	for rows.Next() {
		var u UserWithOrders
		var o Order

		err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.CreatedAt,
			&o.ID, &o.UserID, &o.Amount, &o.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		// Если пользователь ещё не добавлен в карту — добавляем
		if _, exists := usersMap[u.ID]; !exists {
			usersMap[u.ID] = &UserWithOrders{User: u.User}
		}

		// Добавляем заказ к пользователю
		usersMap[u.ID].Orders = append(usersMap[u.ID].Orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	// Превращаем карту в слайс и вычисляем TotalSpent
	result := make([]UserWithOrders, 0, len(usersMap))
	for _, u := range usersMap {
		total := 0.0
		for _, o := range u.Orders {
			total += o.Amount
		}
		u.TotalSpent = total
		result = append(result, *u)
	}

	return result, nil
}

// ============================================================================
// 2. LEFT JOIN: ВСЕ ПОЛЬЗОВАТЕЛИ (ВКЛЮЧАЯ БЕЗ ЗАКАЗОВ)
// ============================================================================

// GetAllUsersWithOrders возвращает ВСЕХ пользователей с их заказами.
// Использует LEFT JOIN.
func (r *UserRepo) GetAllUsersWithOrders(ctx context.Context) ([]UserWithOrders, error) {
	query := `
        SELECT
            u.id, u.name, u.email, u.created_at,
            o.id AS order_id,
            o.user_id AS order_user_id,
            o.amount AS order_amount,
            o.created_at AS order_created_at
        FROM users u
        LEFT JOIN orders o ON u.id = o.user_id
        ORDER BY u.id, o.created_at DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all users with orders: %w", err)
	}
	defer rows.Close()

	usersMap := make(map[int64]*UserWithOrders)

	for rows.Next() {
		var u UserWithOrders
		var o Order

		// Используем sql.Null для полей заказа (могут быть NULL)
		var orderID, orderUserID *int64
		var orderAmount *float64
		var orderCreatedAt *time.Time

		err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.CreatedAt,
			&orderID, &orderUserID, &orderAmount, &orderCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if _, exists := usersMap[u.ID]; !exists {
			usersMap[u.ID] = &UserWithOrders{User: u.User}
		}

		// Если заказ есть (не NULL) — добавляем
		if orderID != nil {
			o.ID = *orderID
			o.UserID = *orderUserID
			o.Amount = *orderAmount
			o.CreatedAt = *orderCreatedAt
			usersMap[u.ID].Orders = append(usersMap[u.ID].Orders, o)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	result := make([]UserWithOrders, 0, len(usersMap))
	for _, u := range usersMap {
		total := 0.0
		for _, o := range u.Orders {
			total += o.Amount
		}
		u.TotalSpent = total
		result = append(result, *u)
	}

	return result, nil
}

// ============================================================================
// 3. JOIN ТРЁХ ТАБЛИЦ (ПОЛЬЗОВАТЕЛИ → ЗАКАЗЫ → ТОВАРЫ)
// ============================================================================

// GetOrdersWithItems возвращает заказы с товарами и именем пользователя.
func (r *UserRepo) GetOrdersWithItems(ctx context.Context) ([]OrderWithItems, error) {
	query := `
        SELECT
            o.id AS order_id,
            o.user_id AS order_user_id,
            o.amount AS order_amount,
            o.created_at AS order_created_at,
            u.name AS user_name,
            oi.id AS item_id,
            oi.product_id AS item_product_id,
            oi.quantity AS item_quantity,
            oi.price AS item_price,
            p.name AS product_name
        FROM orders o
        JOIN users u ON o.user_id = u.id
        JOIN order_items oi ON o.id = oi.order_id
        JOIN products p ON oi.product_id = p.id
        ORDER BY o.id, oi.id
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query orders with items: %w", err)
	}
	defer rows.Close()

	ordersMap := make(map[int64]*OrderWithItems)

	for rows.Next() {
		var o OrderWithItems
		var item OrderItemWithProduct

		err := rows.Scan(
			&o.ID, &o.UserID, &o.Amount, &o.CreatedAt,
			&o.UserName,
			&item.ID, &item.ProductID, &item.Quantity, &item.Price,
			&item.ProductName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if _, exists := ordersMap[o.ID]; !exists {
			// Запоминаем order_id, чтобы не потерять его
			// (при сканировании мы не сканируем отдельно order_id, оно уже в o.ID)
			ordersMap[o.ID] = &OrderWithItems{Order: o.Order}
		}

		// Добавляем товар к заказу
		ordersMap[o.ID].Items = append(ordersMap[o.ID].Items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	result := make([]OrderWithItems, 0, len(ordersMap))
	for _, o := range ordersMap {
		result = append(result, *o)
	}

	return result, nil
}

// ============================================================================
// 4. SELF JOIN: ИЕРАРХИЯ СОТРУДНИКОВ
// ============================================================================

// GetEmployeeHierarchy возвращает список сотрудников с их руководителями.
// Использует SELF JOIN.
func (r *UserRepo) GetEmployeeHierarchy(ctx context.Context) ([]EmployeeWithManager, error) {
	// Для SELF JOIN нужна отдельная таблица employees
	// Создадим её заранее или используем существующую
	// В реальном проекте она создаётся отдельно

	query := `
        SELECT
            e1.id,
            e1.name,
            e1.position,
            e1.salary,
            e1.manager_id,
            e2.name AS manager_name
        FROM employees e1
        LEFT JOIN employees e2 ON e1.manager_id = e2.id
        ORDER BY e1.id
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query employee hierarchy: %w", err)
	}
	defer rows.Close()

	var result []EmployeeWithManager
	for rows.Next() {
		var e EmployeeWithManager
		err := rows.Scan(
			&e.ID, &e.Name, &e.Position, &e.Salary,
			&e.ManagerID, &e.ManagerName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result = append(result, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

// ============================================================================
// 5. JOIN С АГРЕГАЦИЕЙ (СТАТИСТИКА ПОЛЬЗОВАТЕЛЕЙ)
// ============================================================================

// GetUserStats возвращает статистику по пользователям.
// Использует JOIN + GROUP BY + агрегатные функции.
func (r *UserRepo) GetUserStats(ctx context.Context) ([]UserStats, error) {
	query := `
        SELECT
            u.name,
            COUNT(o.id) AS order_count,
            COALESCE(SUM(o.amount), 0) AS total_spent,
            COALESCE(AVG(o.amount), 0) AS avg_order
        FROM users u
        LEFT JOIN orders o ON u.id = o.user_id
        GROUP BY u.id, u.name
        ORDER BY total_spent DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query user stats: %w", err)
	}
	defer rows.Close()

	var result []UserStats
	for rows.Next() {
		var s UserStats
		err := rows.Scan(&s.Name, &s.OrderCount, &s.TotalSpent, &s.AvgOrder)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result = append(result, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
