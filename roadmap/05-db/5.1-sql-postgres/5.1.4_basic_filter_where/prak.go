package __1_4_basic_filter_where

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5"
)

/*
В Go часто нужно строить WHERE-часть динамически, в зависимости от того,
какие фильтры передал пользователь.

Пример: поиск пользователей с опциональными фильтрами.
*/

type User struct {
	ID        int64
	Name      string
	Email     string
	Age       int
	City      sql.NullString
	IsActive  bool
	CreatedAt time.Time
}

type UserFilters struct {
	Name   string
	MinAge int
	MaxAge int
	City   string
	Active *bool // nil означает "не фильтровать"
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) ListUsers(ctx context.Context, filters UserFilters) ([]User, error) {
	query := "SELECT id, name, email, age, city, is_active, created_at FROM users_test WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filters.Name != "" {
		query += fmt.Sprintf(" AND name = $%d", argIdx)
		args = append(args, filters.Name)
		argIdx++
	}

	if filters.MinAge > 0 {
		query += fmt.Sprintf(" AND age >= $%d", argIdx)
		args = append(args, filters.MinAge)
		argIdx++
	}

	if filters.MaxAge > 0 {
		query += fmt.Sprintf(" AND age <= $%d", argIdx)
		args = append(args, filters.MaxAge)
		argIdx++
	}

	if filters.City != "" {
		query += fmt.Sprintf(" AND city = $%d", argIdx)
		args = append(args, filters.City)
		argIdx++
	}

	if filters.Active != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *filters.Active)
		argIdx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(
			&u.ID,
			&u.Name,
			&u.Email,
			&u.Age,
			&u.City,
			&u.IsActive,
			&u.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return users, nil
}
