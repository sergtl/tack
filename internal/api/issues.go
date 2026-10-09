package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type Issue struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (h *Handler) GetIssues(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value("userID")

	var workspaceId int

	// we fetch user to get workspace_id
	if err := h.DB.QueryRow(
		r.Context(),
		"SELECT workspace_id FROM users WHERE id = $1",
		userId,
	).Scan(&workspaceId); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	// fetch all issues belonging to this workspace
	rows, err := h.DB.Query(
		r.Context(),
		"SELECT id, title, description, status FROM issues WHERE workspace_id = $1 ORDER BY id",
		workspaceId,
	)

	if err != nil {
		log.Printf("could not fetch issues from the DB: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong. DB.",
		})
		return
	}
	defer rows.Close()

	issues := make([]Issue, 0)

	for rows.Next() {
		var issue Issue

		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status); err != nil {
			log.Printf("could not scan results from the DB: %v", err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Something went wrong. DB.",
			})
			return
		}

		issues = append(issues, issue)
	}

	if err := rows.Err(); err != nil {
		log.Printf("could not read issues from the DB: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]Issue{
		"data": issues,
	})
}

func (h *Handler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func (h *Handler) DeleteIssue(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func (h *Handler) AttachIssueToProject(w http.ResponseWriter, r *http.Request) {
	// TODO
}
