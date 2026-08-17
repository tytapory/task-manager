package http

import (
	"time"

	"manager/internal/domain/models"

	"github.com/google/uuid"
)

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type TeamResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskResponse struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedBy   uuid.UUID `json:"created_by"`
	AssigneeID  uuid.UUID `json:"assignee_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ClosedAt    time.Time `json:"closed_at"`
	Version     uint      `json:"version"`
}

type TaskHistoryResponse struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"task_id"`
	ChangedBy uuid.UUID `json:"changed_by"`
	Changes   string    `json:"changes"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskCommentResponse struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"task_id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type AssigneeStatResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Count  int       `json:"count"`
}

type TeamStatsResponse struct {
	TasksByStatus           map[string]int         `json:"tasks_by_status"`
	TopAssignees            []AssigneeStatResponse `json:"top_assignees"`
	AverageCloseTimeSeconds float64                `json:"average_close_time_seconds"`
	TotalComments           int                    `json:"total_comments"`
}

func toUserResponse(u models.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}

func toTeamResponse(t models.Team) TeamResponse {
	return TeamResponse{
		ID:        t.ID,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
	}
}

func toTeamResponseList(teams []models.Team) []TeamResponse {
	res := make([]TeamResponse, len(teams))
	for i, t := range teams {
		res[i] = toTeamResponse(t)
	}
	return res
}

func toTaskResponse(t models.Task) TaskResponse {
	return TaskResponse{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		ClosedAt:    t.ClosedAt,
		Version:     t.Version,
	}
}

func toTaskResponseList(tasks []models.Task) []TaskResponse {
	res := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		res[i] = toTaskResponse(t)
	}
	return res
}

func toTaskHistoryResponseList(history []models.TaskHistory) []TaskHistoryResponse {
	res := make([]TaskHistoryResponse, len(history))
	for i, h := range history {
		res[i] = TaskHistoryResponse{
			ID:        h.ID,
			TaskID:    h.TaskID,
			ChangedBy: h.ChangedBy,
			Changes:   h.Changes,
			CreatedAt: h.CreatedAt,
		}
	}
	return res
}

func toTaskCommentResponse(c models.TaskComment) TaskCommentResponse {
	return TaskCommentResponse{
		ID:        c.ID,
		TaskID:    c.TaskID,
		UserID:    c.UserID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
}

func toTaskCommentResponseList(comments []models.TaskComment) []TaskCommentResponse {
	res := make([]TaskCommentResponse, len(comments))
	for i, c := range comments {
		res[i] = toTaskCommentResponse(c)
	}
	return res
}

func toTeamStatsResponse(stats models.TeamStats) TeamStatsResponse {
	topAssignees := make([]AssigneeStatResponse, len(stats.TopAssignees))
	for i, a := range stats.TopAssignees {
		topAssignees[i] = AssigneeStatResponse{
			UserID: a.UserID,
			Count:  a.Count,
		}
	}
	return TeamStatsResponse{
		TasksByStatus:           stats.TasksByStatus,
		TopAssignees:            topAssignees,
		AverageCloseTimeSeconds: stats.AverageCloseTimeSeconds,
		TotalComments:           stats.TotalComments,
	}
}
