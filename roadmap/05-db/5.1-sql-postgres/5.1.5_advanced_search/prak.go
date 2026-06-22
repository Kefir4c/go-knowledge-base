package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type UserRepo struct {
	db *sql.DB
}

type User struct {
	ID        int64          `db:"id"`
	Name      string         `db:"name"`
	Email     string         `db:"email"`
	Age       int            `db:"age"`
	City      sql.NullString `db:"city"`
	IsActive  bool           `db:"is_active"`
	CreatedAt time.Time      `db:"created_at"`
}

type UserFilters struct {
	// Junior фильтры (простые)
	Name   string
	MinAge int
	MaxAge int
	City   string
	Active *bool

	// фильтры (продвинутые)
	NamePrefix   string   // LIKE 'prefix%'
	NameContains string   // LIKE '%contains%'
	NameSuffix   string   // LIKE '%suffix'
	Cities       []string // IN (...)
	ExcludeCity  string   // !=
	MinAgeInCity int      // для коррелированных подзапросов
}

// 1. MIDDLE: ПОИСК С LIKE, BETWEEN, IN

func (r *UserRepo) ListUsersMiddle(ctx context.Context, filters UserFilters) ([]User, error) {
	query := "SELECT id, name, email, age, city, is_active, created_at FROM users_test WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	// LIKE с фиксированным началом (использует индекс)
	if filters.NamePrefix != "" {
		query += fmt.Sprintf(" AND name LIKE $%d || '%%'", argIdx)
		args = append(args, filters.NamePrefix)
		argIdx++
	}

	// LIKE с % в середине (индекс не используется)
	if filters.NameContains != "" {
		query += fmt.Sprintf(" AND name LIKE '%%' || $%d || '%%'", argIdx)
		args = append(args, filters.NameContains)
		argIdx++
	}

	// LIKE с % в конце
	if filters.NameSuffix != "" {
		query += fmt.Sprintf(" AND name LIKE '%%' || $%d", argIdx)
		args = append(args, filters.NameSuffix)
		argIdx++
	}

	// BETWEEN
	if filters.MinAge > 0 && filters.MaxAge > 0 {
		query += fmt.Sprintf(" AND age BETWEEN $%d AND $%d", argIdx, argIdx+1)
		args = append(args, filters.MinAge, filters.MaxAge)
		argIdx += 2
	} else if filters.MinAge > 0 {
		query += fmt.Sprintf(" AND age >= $%d", argIdx)
		args = append(args, filters.MinAge)
		argIdx++
	} else if filters.MaxAge > 0 {
		query += fmt.Sprintf(" AND age <= $%d", argIdx)
		args = append(args, filters.MaxAge)
		argIdx++
	}

	// IN с динамическим списком городов
	if len(filters.Cities) > 0 {
		placeholders := make([]string, len(filters.Cities))
		for i := range filters.Cities {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, filters.Cities[i])
			argIdx++
		}
		query += fmt.Sprintf(" AND city IN (%s)", strings.Join(placeholders, ", "))
	}

	// Исключение города
	if filters.ExcludeCity != "" {
		query += fmt.Sprintf(" AND city != $%d", argIdx)
		args = append(args, filters.ExcludeCity)
		argIdx++
	}

	// Активность
	if filters.Active != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *filters.Active)
		argIdx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

// 2. MIDDLE+: КОРРЕЛИРОВАННЫЙ ПОДЗАПРОС (возраст > среднего в городе)

func (r *UserRepo) GetUsersOlderThanAvgInCity(ctx context.Context) ([]User, error) {
	query := `
        SELECT u1.id, u1.name, u1.email, u1.age, u1.city, u1.is_active, u1.created_at
        FROM users_test u1
        WHERE u1.age > (SELECT AVG(u2.age) FROM users_test u2 WHERE u2.city = u1.city)
        ORDER BY u1.city, u1.age DESC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

// 3. MIDDLE+: ПОЛЬЗОВАТЕЛИ С ТАКИМ ЖЕ ВОЗРАСТОМ (EXISTS)

func (r *UserRepo) GetUsersWithSameAge(ctx context.Context) ([]User, error) {
	query := `
        SELECT u1.id, u1.name, u1.email, u1.age, u1.city, u1.is_active, u1.created_at
        FROM users_test u1
        WHERE EXISTS (
            SELECT 1 FROM users_test u2
            WHERE u2.age = u1.age AND u2.id != u1.id
        )
        ORDER BY age
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

func scanUsers(rows *sql.Rows) ([]User, error) {
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
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}
