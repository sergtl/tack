package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RegisterRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var payload RegisterRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		log.Printf("invalid json body: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid JSON body",
		})
		return
	}

	if err := h.validate.Struct(payload); err != nil {
		log.Printf("could not validate the payload struct: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid register fields",
		})
		return
	}

	hash, err := HashPassword(payload.Password)

	if err != nil {
		log.Printf("error hashing the password: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		log.Printf("could not start the transaction: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}
	defer tx.Rollback(r.Context())

	var id int
	if err := tx.QueryRow(
		r.Context(),
		"INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id",
		payload.Name,
		payload.Email,
		hash,
	).Scan(&id); err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
			log.Printf("email already exists")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Email already registered",
			})
			return
		}

		log.Printf("could not insert new user: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	token := rand.Text()
	tokenHash := hashToken(token)

	const sessionLifetime = 7 * 24 * time.Hour
	expiresAt := time.Now().Add(sessionLifetime)

	if _, err := tx.Exec(
		r.Context(),
		"INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		tokenHash,
		id,
		expiresAt,
	); err != nil {
		log.Printf("failed to create session during registration: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("failed to commit registration transaction: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	cookie := http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var payload LoginRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		log.Printf("invalid json body: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid JSON body",
		})
		return
	}

	if err := h.validate.Struct(payload); err != nil {
		log.Printf("could not validate the payload struct: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid login fields",
		})
		return
	}

	var id int
	var passwordHash string
	if err := h.DB.QueryRow(
		r.Context(),
		"SELECT id, password FROM users WHERE email = $1",
		payload.Email,
	).Scan(&id, &passwordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("user not found")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		log.Printf("failed to fetch user during login: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return

	}

	passwordMatches := CheckPasswordHash(payload.Password, passwordHash)

	if !passwordMatches {
		log.Printf("wrong password")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid email or password",
		})
		return
	}

	token := rand.Text()
	tokenHash := hashToken(token)

	const sessionLifetime = 7 * 24 * time.Hour
	expiresAt := time.Now().Add(sessionLifetime)

	if _, err := h.DB.Exec(
		r.Context(),
		"INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		tokenHash,
		id,
		expiresAt,
	); err != nil {
		log.Printf("failed to create session during login: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	cookie := http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)
	w.WriteHeader(http.StatusOK)
}
