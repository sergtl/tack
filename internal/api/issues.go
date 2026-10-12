package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/render"
)

type Issue struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (h *Handler) GetIssues(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
			"error": "Authentication required",
		})
	}

	var workspaceId int

	// we fetch user to get workspace_id
	if err := h.DB.QueryRow(
		r.Context(),
		"SELECT workspace_id FROM users WHERE id = $1",
		userID,
	).Scan(&workspaceId); err != nil {

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
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

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong.",
		})
		return
	}
	defer rows.Close()

	issues := make([]Issue, 0)

	for rows.Next() {
		var issue Issue

		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status); err != nil {
			log.Printf("could not scan results from the DB: %v", err)

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{
				"error": "Something went wrong. DB.",
			})
			return
		}

		issues = append(issues, issue)
	}

	if err := rows.Err(); err != nil {
		log.Printf("could not read issues from the DB: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string][]Issue{
		"data": issues,
	})
}

type CreateIssueRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
	ProjectID   int    `json:"project_id"`
}

func (h *Handler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	var payload CreateIssueRequest

	if err := h.decodeAndValidate(r, &payload); err != nil {
		log.Printf("create issue: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid request body or fields",
		})
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
			"error": "Authentication required",
		})
	}

	var id int

	err := h.DB.QueryRow(
		r.Context(),
		`INSERT INTO issues (title, description, workspace_id, project_id)
		 SELECT $1, $2, users.workspace_id, $3
		 FROM users
		 WHERE users.id = $4
		 RETURNING id`,
		payload.Title,
		payload.Description,
		payload.ProjectID,
		userID,
	).Scan(&id)

	if err != nil {
		log.Printf("create issue: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Failed to create issue",
		})
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]int{
		"id": id,
	})
}

type DeleteIssueRequest struct {
	ID int `json:"id"`
}

func (h *Handler) DeleteIssue(w http.ResponseWriter, r *http.Request) {
	var payload DeleteIssueRequest

	if err := h.decodeAndValidate(r, &payload); err != nil {
		log.Printf("delete issue: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid request body or fields",
		})
		return
	}

	var id int

	err := h.DB.QueryRow(
		r.Context(),
		`DELETE FROM issues WHERE id = $1 RETURNING id`,
		payload.ID,
	).Scan(&id)

	if err != nil {
		log.Printf("delete issue: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Failed to delete issue",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{
		"data": "Ok",
	})
}

type AttachIssueRequest struct {
	IssueID   int `json:"issue_id"`
	ProjectID int `json:"project_id"`
}

func (h *Handler) AttachIssueToProject(w http.ResponseWriter, r *http.Request) {
	// TODO
	var payload AttachIssueRequest

	if err := h.decodeAndValidate(r, &payload); err != nil {
		log.Printf("attach issue: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid request body or fields",
		})
		return
	}

	var id int

	err := h.DB.QueryRow(
		r.Context(),
		`UPDATE issues
		 SET project_id = $1
		 WHERE id = $2
		 RETURNING id
		`,
		payload.ProjectID,
		payload.IssueID,
	).Scan(&id)

	if err != nil {
		log.Printf("attach issue: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Failed to attach issue",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{
		"data": "Ok",
	})
}

func (h *Handler) decodeAndValidate(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	if err := h.validate.Struct(dst); err != nil {
		return fmt.Errorf("validate request: %w", err)
	}

	return nil
}
