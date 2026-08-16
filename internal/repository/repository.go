package repository

import (
	"context"

	"github.com/google/uuid"

	"manager/internal/domain/models"
)

type UserRepository interface {
	Create(ctx context.Context, user models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
}

type TeamRepository interface {
	Create(ctx context.Context, team models.Team) error
	GetByID(ctx context.Context, id uuid.UUID) (models.Team, error)

	AddMember(ctx context.Context, member models.TeamMember) error
	GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (models.TeamRole, error)
}

type TaskRepository interface {
	Create(ctx context.Context, task models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (models.Task, error)
	Update(ctx context.Context, task models.Task) error

	ListByTeam(ctx context.Context, teamID uuid.UUID, limit, offset int) ([]models.Task, error)
}

type TaskHistoryRepository interface {
	Create(ctx context.Context, history models.TaskHistory) error
	GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskHistory, error)
}

type TaskCommentRepository interface {
	Create(ctx context.Context, comment models.TaskComment) error
	GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskComment, error)
}
