package sortpagination

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Product struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Price       float64   `db:"price"`
	Stock       int       `db:"stock"`
	Category    string    `db:"category"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type ProductRepo struct {
	pool *pgxpool.Pool
}

// NewProductRepo создаёт новый репозиторий с пулом соединений.
func NewProductRepo(pool *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{pool: pool}
}

// 1. ПАГИНАЦИЯ С OFFSET (простой вариант)
// ListProducts возвращает список товаров с пагинацией через OFFSET.
// page - номер страницы (начиная с 1), limit - количество записей на страницу.
func (r *ProductRepo) ListProducts(ctx context.Context, page, limit int) ([]Product, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	query := `
        SELECT id, name, description, price, stock, category, is_active, created_at, updated_at
        FROM products
        ORDER BY id
        LIMIT $1 OFFSET $2
    `

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

// 2. KEYSET PAGINATION ПО ID (быстрый вариант)

// ListProductsKeyset возвращает список товаров, начиная с ID больше lastID.
// Использует WHERE id > $1, что эффективно использует индекс.
func (r *ProductRepo) ListProductsKeyset(ctx context.Context, lastID int64, limit int) ([]Product, error) {
	if limit < 1 {
		limit = 10
	}

	query := `
        SELECT id, name, description, price, stock, category, is_active, created_at, updated_at
        FROM products
        WHERE id > $1
        ORDER BY id
        LIMIT $2
    `

	rows, err := r.pool.Query(ctx, query, lastID, limit)
	if err != nil {
		return nil, fmt.Errorf("keyset query: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

// 3. KEYSET С СОРТИРОВКОЙ ПО НЕСКОЛЬКИМ ПОЛЯМ (КОРТЕЖИ)

// ListProductsByCategory возвращает товары, отсортированные по (category, id),
// начиная с позиции после (lastCategory, lastID).
func (r *ProductRepo) ListProductsByCategory(ctx context.Context, lastCategory string, lastID int64, limit int) ([]Product, error) {
	if limit < 1 {
		limit = 10
	}

	// Используем кортеж для сравнения: (category, id) > (lastCategory, lastID)
	query := `
        SELECT id, name, description, price, stock, category, is_active, created_at, updated_at
        FROM products
        WHERE (category, id) > ($1, $2)
        ORDER BY category, id
        LIMIT $3
    `

	rows, err := r.pool.Query(ctx, query, lastCategory, lastID, limit)
	if err != nil {
		return nil, fmt.Errorf("keyset tuple query: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

// 4. ПАГИНАЦИЯ С ОБЩИМ КОЛИЧЕСТВОМ (total)

// ListProductsWithTotal возвращает список товаров и общее количество записей.
// Это нужно для фронтенда, чтобы показывать общее количество страниц.
func (r *ProductRepo) ListProductsWithTotal(ctx context.Context, page, limit int) ([]Product, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Получаем данные
	dataQuery := `
        SELECT id, name, description, price, stock, category, is_active, created_at, updated_at
        FROM products
        ORDER BY id
        LIMIT $1 OFFSET $2
    `
	rows, err := r.pool.Query(ctx, dataQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query data: %w", err)
	}
	products, err := scanProducts(rows)
	if err != nil {
		return nil, 0, err
	}

	// 2. Получаем total
	var total int
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count total: %w", err)
	}

	return products, total, nil
}

func scanProducts(rows pgx.Rows) ([]Product, error) {
	var products []Product
	for rows.Next() {
		var p Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.Stock,
			&p.Category,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return products, nil
}

func main() {
	// Подключение к PostgreSQL (замените на свои данные)
	connString := "postgres://user:password@localhost:5432/mydb?sslmode=disable"
	// Создаём пул соединений
	pool, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	repo := NewProductRepo(pool)

	ctx := context.Background()

	// 1. OFFSET пагинация
	fmt.Println("=== OFFSET пагинация (страница 2, limit=5) ===")
	products, err := repo.ListProducts(ctx, 2, 5)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range products {
		fmt.Printf("ID: %d, Name: %s, Price: %.2f\n", p.ID, p.Name, p.Price)
	}

	// 2. Keyset по ID
	fmt.Println("\n=== Keyset по ID (после ID=5, limit=5) ===")
	products, err = repo.ListProductsKeyset(ctx, 5, 5)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range products {
		fmt.Printf("ID: %d, Name: %s, Price: %.2f\n", p.ID, p.Name, p.Price)
	}

	// 3. Keyset с кортежами (category, id)
	fmt.Println("\n=== Keyset по (category, id) после ('Accessories', 3) ===")
	products, err = repo.ListProductsByCategory(ctx, "Accessories", 3, 5)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range products {
		fmt.Printf("ID: %d, Category: %s, Name: %s\n", p.ID, p.Category, p.Name)
	}

	// 4. Пагинация с total
	fmt.Println("\n=== Пагинация с total (страница 1, limit=5) ===")
	products, total, err := repo.ListProductsWithTotal(ctx, 1, 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total records: %d\n", total)
	for _, p := range products {
		fmt.Printf("ID: %d, Name: %s\n", p.ID, p.Name)
	}
}
