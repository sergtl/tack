package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	}))

	r.Group(func(protected chi.Router) {
		protected.Use(RequireSession(h.DB))

		protected.Get("/me", h.Me)
		protected.Post("/logout", h.Logout)

		protected.Get("/issues", h.GetIssues)
		protected.Get("/issues/{id}", h.GetIssue)

		protected.Post("/issues", h.CreateIssue)
		protected.Post("/attach-issue", h.AttachIssueToProject)

		protected.Patch("/issues/{id}", h.UpdateIssue)
	})

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusOK)
		render.JSON(w, r, map[string]string{
			"data": "Hello, world!",
		})
	})

	return r
}
