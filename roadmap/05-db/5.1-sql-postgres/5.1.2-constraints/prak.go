package __1_2_constraints

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserRepo struct {
	db *pgx.Conn
}

func (r *UserRepo) CreateUser(email string) error {
	_, err := r.db.Exec(context.Background(), "INSERT INTO users (email) VALUES ($1)", email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique violation
				return fmt.Errorf("user with email %s already exists", email)
			case "23502": // not null violation
				return fmt.Errorf("email is required")
			default:
				return err
			}
		}
		return err
	}
	return nil
}
