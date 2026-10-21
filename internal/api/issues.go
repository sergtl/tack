package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
)

type IssueStatus string

const (
	IssueStatusTodo       IssueStatus = "todo"
	IssueStatusInProgress IssueStatus = "in_progress"
	IssueStatusCompleted  IssueStatus = "completed"
	IssueStatusCanceled   IssueStatus = "canceled"
)

func (s IssueStatus) Valid() bool {
	switch s {
	case IssueStatusTodo, IssueStatusInProgress, IssueStatusCompleted, IssueStatusCanceled:
		return true
	default:
		return false
	}
}

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

func (h *Handler) GetIssue(w http.ResponseWriter, r *http.Request) {
	issueId := chi.URLParam(r, "id")

	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
			"error": "Authentication required",
		})
	}

	var workspaceId int
	var issue Issue

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

	err := h.DB.QueryRow(
		r.Context(),
		`SELECT id, title, description, status FROM issues WHERE id = $1 AND workspace_id = $2`,
		issueId,
		workspaceId,
	).Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status)

	if err != nil {
		log.Printf("could not fetch issue %v from the DB: %v", issueId, err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong.",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]Issue{
		"data": issue,
	})
}

type CreateIssueRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
	ProjectID   *int   `json:"project_id"`
}

type CreateIssueResponse struct {
	Data struct {
		ID int `json:"id"`
	} `json:"data"`
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

	response := CreateIssueResponse{}
	response.Data.ID = id

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
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

type UpdateIssueRequest struct {
	Title       *string      `json:"title" validate:"omitnil,min=1,max=255"`
	Description *string      `json:"description" validate:"omitnil,max=255"`
	Status      *IssueStatus `json:"status"`
}

func (h *Handler) UpdateIssue(w http.ResponseWriter, r *http.Request) {
	issueId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)

	if err != nil || issueId <= 0 {

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid issue ID",
		})
		return
	}

	var payload UpdateIssueRequest

	if err := h.decodeAndValidate(r, &payload); err != nil {
		log.Printf("update issue: %v", err)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid request body or fields",
		})
		return
	}

	if payload.Status != nil && !payload.Status.Valid() {
		log.Printf("invalid issue status: %v", *payload.Status)

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Invalid issue status",
		})

		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{
			"error": "Authentication required",
		})
		return
	}

	var workspaceId int

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

	var issue Issue

	err = h.DB.QueryRow(
		r.Context(),
		`UPDATE issues
		SET title = COALESCE($1, title),
			description = COALESCE($2, description),
			status = COALESCE($3, status)
		WHERE id = $4 AND workspace_id = $5
		RETURNING id, title, description, status`,
		payload.Title,
		payload.Description,
		payload.Status,
		issueId,
		workspaceId,
	).Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": "Issue not found",
		})
		return
	}

	if err != nil {
		log.Printf("could not update the issue: %v", err)

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Something went wrong",
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]Issue{
		"data": issue,
	})
}

type AttachIssueRequest struct {
	IssueID   int `json:"issue_id"`
	ProjectID int `json:"project_id"`
}

func (h *Handler) AttachIssueToProject(w http.ResponseWriter, r *http.Request) {
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
