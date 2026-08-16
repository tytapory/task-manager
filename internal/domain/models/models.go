package models

import (
	"time"

	"github.com/google/uuid"
)

type TeamRole string

const (
	Owner  TeamRole = "owner"
	Admin  TeamRole = "admin"
	Member TeamRole = "member"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
}

type Team struct {
	ID        uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

type TeamMember struct {
	TeamID uuid.UUID
	UserID uuid.UUID
	Role   TeamRole
}

type Task struct {
	ID          uuid.UUID
	TeamID      uuid.UUID
	Title       string
	Description string
	Status      string
	CreatedBy   uuid.UUID
	AssigneeID  uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    time.Time
	Version     uint
}

type TaskHistory struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	ChangedBy uuid.UUID
	Changes   string
	CreatedAt time.Time
}

type TaskComments struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	UserID    uuid.UUID
	Content   string
	CreatedAt time.Time
}
