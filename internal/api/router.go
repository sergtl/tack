package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(protected chi.Router) {
		protected.Use(RequireSession(h.DB))
		protected.Get("/issues", h.GetIssues)
	})

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)

	return r
}
