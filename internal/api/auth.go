package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey int

const userIDKey contextKey = iota

type RegisterRequest struct {
	Name          string `json:"name" validate:"required"`
	Email         string `json:"email" validate:"required,email"`
	Password      string `json:"password" validate:"required"`
	WorkspaceName string `json:"workspace" validate:"required"`
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

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid JSON body",
		})
		return
	}

	if err := h.validate.Struct(payload); err != nil {
		log.Printf("could not validate the payload struct: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid register fields",
		})
		return
	}

	hash, err := HashPassword(payload.Password)

	if err != nil {
		log.Printf("error hashing the password: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	suffix, err := randomSuffix()
	if err != nil {
		log.Printf("could not generate slug suffix: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	baseSlug := strings.ReplaceAll(strings.ToLower(payload.WorkspaceName), " ", "-")

	slug := baseSlug + "-" + suffix

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		log.Printf("could not start the transaction: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}
	defer tx.Rollback(r.Context())

	var workspaceID string

	if err := tx.QueryRow(
		r.Context(),
		`INSERT INTO workspaces (name, slug)
		 VALUES ($1, $2)
		 RETURNING id`,
		payload.WorkspaceName,
		slug,
	).Scan(&workspaceID); err != nil {
		log.Printf("could not create workspace for the user: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
	}

	log.Printf("created new org with id: %v", workspaceID)

	var id int
	if err := tx.QueryRow(
		r.Context(),
		"INSERT INTO users (name, email, password, workspace_id) VALUES ($1, $2, $3, $4) RETURNING id",
		payload.Name,
		payload.Email,
		hash,
		workspaceID,
	).Scan(&id); err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
			log.Printf("email already exists")

			render.Status(r, http.StatusConflict)
			render.JSON(w, r, map[string]string{
				"error": "Email already registered",
			})
			return
		}

		log.Printf("could not insert new user: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
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

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("failed to commit registration transaction: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
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

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{
		"status": "Ok",
	})
}

type MeResponse struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())

	if !ok {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
			"error": "Authentication required",
		})
		return
	}

	var me MeResponse

	err := h.DB.QueryRow(
		r.Context(),
		`SELECT u.id, u.name, u.email, w.name, w.slug
		 FROM users AS u
		 JOIN workspaces AS w ON u.workspace_id = w.id
		 WHERE u.id = $1`,
		userID,
	).Scan(
		&me.ID,
		&me.Name,
		&me.Email,
		&me.WorkspaceName,
		&me.WorkspaceSlug,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("user left join workspaces failed: %v", err)

			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{
				"error": "User not found",
			})
			return
		}

		log.Printf("fetch current user: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]MeResponse{
		"me": me,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var payload LoginRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		log.Printf("invalid json body: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid JSON body",
		})
		return
	}

	if err := h.validate.Struct(payload); err != nil {
		log.Printf("could not validate the payload struct: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
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

			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		log.Printf("failed to fetch user during login: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return

	}

	passwordMatches := CheckPasswordHash(payload.Password, passwordHash)

	if !passwordMatches {
		log.Printf("wrong password")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)

		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
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

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
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
	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{
		"status": "Ok",
	})
}

func RequireSession(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")

			log.Printf("AUTH MIDDLEWARE")

			if err != nil {

				// delete
				cookie := http.Cookie{
					Name:     "session",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				}
				http.SetCookie(w, &cookie)

				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{
					"error": "Session expired or invalid",
				})
				return
			}

			tokenHash := hashToken(cookie.Value)

			var userId int
			var expiresAt time.Time

			if err := db.QueryRow(
				r.Context(),
				"SELECT user_id, expires_at FROM sessions WHERE token_hash = $1",
				tokenHash,
			).Scan(&userId, &expiresAt); err != nil {
				cookie := http.Cookie{
					Name:     "session",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				}
				http.SetCookie(w, &cookie)

				if errors.Is(pgx.ErrNoRows, err) {
					render.Status(r, http.StatusUnauthorized)
					render.JSON(w, r, map[string]string{
						"error": "Session expired or invalid",
					})
					return
				}

				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{
					"error": "Session expired or invalid",
				})
				return
			}

			if !expiresAt.After(time.Now()) {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{
					"error": "Session expired or invalid",
				})
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userId)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

func randomSuffix() (string, error) {
	b := make([]byte, 4)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
