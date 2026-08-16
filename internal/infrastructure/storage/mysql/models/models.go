package models

import (
	"manager/internal/domain/models"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	Name         string    `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (User) TableName() string { return "users" }

func (u *User) ToDomain() models.User {
	return models.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		CreatedAt:    u.CreatedAt,
	}
}

func UserFromDomain(u models.User) User {
	return User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		CreatedAt:    u.CreatedAt,
	}
}

type Team struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey"`
	Name      string    `gorm:"type:varchar(255);not null"`
	CreatedBy uuid.UUID `gorm:"type:char(36);not null;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Team) TableName() string { return "teams" }

func (t *Team) ToDomain() models.Team {
	return models.Team{
		ID:        t.ID,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
	}
}

func TeamFromDomain(t models.Team) Team {
	return Team{
		ID:        t.ID,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
	}
}

type TeamMember struct {
	TeamID uuid.UUID       `gorm:"type:char(36);primaryKey"`
	UserID uuid.UUID       `gorm:"type:char(36);primaryKey"`
	Role   models.TeamRole `gorm:"type:enum('owner','admin','member');not null;default:'member'"`
}

func (TeamMember) TableName() string { return "team_members" }

func (tm *TeamMember) ToDomain() models.TeamMember {
	return models.TeamMember{
		TeamID: tm.TeamID,
		UserID: tm.UserID,
		Role:   tm.Role,
	}
}

func TeamMemberFromDomain(tm models.TeamMember) TeamMember {
	return TeamMember{
		TeamID: tm.TeamID,
		UserID: tm.UserID,
		Role:   tm.Role,
	}
}

type Task struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey"`
	TeamID      uuid.UUID  `gorm:"type:char(36);not null;index:idx_tasks_team_status_assignee"`
	Title       string     `gorm:"type:varchar(255);not null"`
	Description string     `gorm:"type:text"`
	Status      string     `gorm:"type:varchar(50);not null;default:'todo';index:idx_tasks_team_status_assignee"`
	CreatedBy   uuid.UUID  `gorm:"type:char(36);not null"`
	AssigneeID  *uuid.UUID `gorm:"type:char(36);index:idx_tasks_team_status_assignee"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
	ClosedAt    *time.Time ``
	Version     uint       `gorm:"not null;default:1"`
}

func (Task) TableName() string { return "tasks" }

func (t *Task) ToDomain() models.Task {
	var (
		ClosedAt   time.Time
		AssigneeID uuid.UUID
	)

	if t.ClosedAt != nil {
		ClosedAt = *t.ClosedAt
	}

	if t.AssigneeID != nil {
		AssigneeID = *t.AssigneeID
	}

	return models.Task{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		ClosedAt:    ClosedAt,
		Version:     t.Version,
	}
}

func TaskFromDomain(t models.Task) Task {
	ClosedAt := t.ClosedAt
	AssigneeID := t.AssigneeID

	return Task{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		CreatedBy:   t.CreatedBy,
		AssigneeID:  &AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		ClosedAt:    &ClosedAt,
		Version:     t.Version,
	}
}

type TaskHistory struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID    uuid.UUID `gorm:"type:char(36);not null;index"`
	ChangedBy uuid.UUID `gorm:"type:char(36);not null"`
	Changes   string    `gorm:"type:json;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (TaskHistory) TableName() string { return "task_history" }

type TaskComment struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID    uuid.UUID `gorm:"type:char(36);not null;index"`
	UserID    uuid.UUID `gorm:"type:char(36);not null"`
	Content   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (TaskComment) TableName() string { return "task_comments" }
