package __1_3_basic_CRUD

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type UserRepo struct {
	db pgx.Conn
}

type User struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       int       `db:"age"`
	CreatedAt time.Time `db:"created_at"`
}

// 1. CREATE — вставка с RETURNING
func (r *UserRepo) CreateUser(ctx context.Context, name, email string, age int) (*User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		INSERT TABLE users_test (name, email, age)
		VALUES ($1, $2, $3)
		RETURNING *`, name, email, age).
		Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &user, nil
}

// 2. READ — получение по ID
func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, age, created_at
        FROM users_test
        WHERE id = $1
        `, id).Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

// 3. UPDATE — обновление по ID
func (r *UserRepo) UpdateUser(ctx context.Context, id int64, name string, age int) error {
	cmdTag, err := r.db.Exec(ctx, `
        UPDATE users_test
        SET name = $1, age = $2
        WHERE id = $3
    `, name, age, id)

	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// 4. DELETE — удаление по ID
func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {
	cmdTag, err := r.db.Exec(ctx, `
        DELETE FROM users_test
        WHERE id = $1
    `, id)

	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
