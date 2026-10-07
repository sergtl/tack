package api

import (
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB       *pgxpool.Pool
	validate *validator.Validate
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{
		DB:       pool,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}
