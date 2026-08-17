package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domain_errors "manager/internal/domain/errors"
	"manager/internal/domain/models"
	"manager/internal/repository"

	"github.com/google/uuid"
)

type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenManager interface {
	GenerateToken(userID uuid.UUID) (string, error)
}

type UserUsecase interface {
	Register(ctx context.Context, email, password, name string) (models.User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type TeamUsecase interface {
	CreateTeam(ctx context.Context, userID uuid.UUID, name string) (models.Team, error)
	ListTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error)
	InviteMember(ctx context.Context, inviterID, teamID, targetUserID uuid.UUID, role models.TeamRole) error
	ChangeMemberRole(ctx context.Context, requesterID, teamID, targetUserID uuid.UUID, newRole models.TeamRole) error
	GetTeamStats(ctx context.Context, userID, teamID uuid.UUID) (any, error)
}

type TaskUsecase interface {
	CreateTask(ctx context.Context, userID, teamID uuid.UUID, title, description string) (models.Task, error)
	GetTasks(ctx context.Context, userID, teamID uuid.UUID, status string, assigneeID *uuid.UUID, limit, offset int) ([]models.Task, error)
	UpdateTask(ctx context.Context, userID uuid.UUID, task models.Task) error
	GetTaskHistory(ctx context.Context, userID, taskID uuid.UUID) ([]models.TaskHistory, error)
	AddComment(ctx context.Context, userID, taskID uuid.UUID, content string) (models.TaskComment, error)
	GetComments(ctx context.Context, userID, taskID uuid.UUID) ([]models.TaskComment, error)
}

type userUsecase struct {
	userRepo     repository.UserRepository
	hasher       PasswordHasher
	tokenManager TokenManager
}

func NewUserUsecase(repo repository.UserRepository, h PasswordHasher, tm TokenManager) UserUsecase {
	return &userUsecase{
		userRepo:     repo,
		hasher:       h,
		tokenManager: tm,
	}
}

func (u *userUsecase) Register(ctx context.Context, email, password, name string) (models.User, error) {
	hash, err := u.hasher.Hash(password)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to hash password: %w", err)
	}

	user := models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         name,
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		return models.User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := u.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain_errors.ErrNotFound) {
			return "", fmt.Errorf("invalid credentials: %w", err)
		}
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	if err := u.hasher.Compare(user.PasswordHash, password); err != nil {
		return "", fmt.Errorf("invalid credentials: incorrect password: %w", domain_errors.ErrUnauthorized)
	}

	token, err := u.tokenManager.GenerateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate auth token: %w", err)
	}

	return token, nil
}

type teamUsecase struct {
	teamRepo repository.TeamRepository
	userRepo repository.UserRepository
	tx       Transactor
}

func NewTeamUsecase(tr repository.TeamRepository, ur repository.UserRepository, tx Transactor) TeamUsecase {
	return &teamUsecase{
		teamRepo: tr,
		userRepo: ur,
		tx:       tx,
	}
}

func (u *teamUsecase) CreateTeam(ctx context.Context, userID uuid.UUID, name string) (models.Team, error) {
	team := models.Team{
		ID:        uuid.New(),
		Name:      name,
		CreatedBy: userID,
	}

	member := models.TeamMember{
		TeamID: team.ID,
		UserID: userID,
		Role:   models.Owner,
	}

	err := u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := u.teamRepo.Create(txCtx, team); err != nil {
			return fmt.Errorf("failed to create team record: %w", err)
		}
		if err := u.teamRepo.AddMember(txCtx, member); err != nil {
			return fmt.Errorf("failed to assign team owner: %w", err)
		}
		return nil
	})

	if err != nil {
		return models.Team{}, err
	}

	return team, nil
}

func (u *teamUsecase) ListTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	return u.teamRepo.GetTeamsByUserID(ctx, userID)
}

func (u *teamUsecase) GetTeamStats(ctx context.Context, userID, teamID uuid.UUID) (any, error) {
	role, err := u.teamRepo.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("access denied: user is not a member of the team: %w", err)
	}

	if role != models.Owner && role != models.Admin {
		return nil, fmt.Errorf("access denied: only team owners and admins can view analytics: %w", domain_errors.ErrForbidden)
	}

	return u.teamRepo.GetStats(ctx, teamID)
}

func (u *teamUsecase) InviteMember(ctx context.Context, inviterID, teamID, targetUserID uuid.UUID, role models.TeamRole) error {
	inviterRole, err := u.teamRepo.GetMemberRole(ctx, teamID, inviterID)
	if err != nil {
		return fmt.Errorf("access denied: inviter is not a member of the team: %w", err)
	}

	if inviterRole != models.Owner && inviterRole != models.Admin {
		return fmt.Errorf("access denied: insufficient permissions to invite members: %w", domain_errors.ErrForbidden)
	}

	if role == models.Owner {
		return fmt.Errorf("invalid operation: owner role cannot be assigned via invitation: %w", domain_errors.ErrInvalid)
	}

	member := models.TeamMember{
		TeamID: teamID,
		UserID: targetUserID,
		Role:   role,
	}

	if err := u.teamRepo.AddMember(ctx, member); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	return nil
}

func (u *teamUsecase) ChangeMemberRole(ctx context.Context, requesterID, teamID, targetUserID uuid.UUID, newRole models.TeamRole) error {
	requesterRole, err := u.teamRepo.GetMemberRole(ctx, teamID, requesterID)
	if err != nil {
		return fmt.Errorf("access denied: requester is not a member of the team: %w", err)
	}

	if requesterRole != models.Owner {
		return fmt.Errorf("access denied: only team owner can change roles: %w", domain_errors.ErrForbidden)
	}

	if newRole == models.Owner {
		return fmt.Errorf("invalid operation: cannot assign owner role: %w", domain_errors.ErrInvalid)
	}

	if err := u.teamRepo.UpdateMemberRole(ctx, teamID, targetUserID, newRole); err != nil {
		return fmt.Errorf("failed to change team member role: %w", err)
	}

	return nil
}

type taskUsecase struct {
	taskRepo    repository.TaskRepository
	teamRepo    repository.TeamRepository
	historyRepo repository.TaskHistoryRepository
	commentRepo repository.TaskCommentRepository
	tx          Transactor
}

func NewTaskUsecase(
	taskRepo repository.TaskRepository,
	teamRepo repository.TeamRepository,
	historyRepo repository.TaskHistoryRepository,
	commentRepo repository.TaskCommentRepository,
	tx Transactor,
) TaskUsecase {
	return &taskUsecase{
		taskRepo:    taskRepo,
		teamRepo:    teamRepo,
		historyRepo: historyRepo,
		commentRepo: commentRepo,
		tx:          tx,
	}
}

func (u *taskUsecase) CreateTask(ctx context.Context, userID, teamID uuid.UUID, title, description string) (models.Task, error) {
	_, err := u.teamRepo.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		return models.Task{}, fmt.Errorf("access denied: cannot create tasks in a team you do not belong to: %w", err)
	}

	task := models.Task{
		ID:          uuid.New(),
		TeamID:      teamID,
		Title:       title,
		Description: description,
		Status:      "todo",
		CreatedBy:   userID,
		Version:     1,
	}

	err = u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := u.taskRepo.Create(txCtx, task); err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}

		changes, _ := json.Marshal(map[string]string{"action": "created"})
		history := models.TaskHistory{
			TaskID:    task.ID,
			ChangedBy: userID,
			Changes:   string(changes),
		}
		if err := u.historyRepo.Create(txCtx, history); err != nil {
			return fmt.Errorf("failed to record task history: %w", err)
		}

		return nil
	})

	if err != nil {
		return models.Task{}, err
	}

	return task, nil
}

func (u *taskUsecase) GetTasks(ctx context.Context, userID, teamID uuid.UUID, status string, assigneeID *uuid.UUID, limit, offset int) ([]models.Task, error) {
	_, err := u.teamRepo.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("access denied: user is not a member of the requested team: %w", err)
	}

	return u.taskRepo.ListByTeam(ctx, teamID, status, assigneeID, limit, offset)
}

func (u *taskUsecase) UpdateTask(ctx context.Context, userID uuid.UUID, update models.Task) error {
	return u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		oldTask, err := u.taskRepo.GetByID(txCtx, update.ID)
		if err != nil {
			if errors.Is(err, domain_errors.ErrNotFound) {
				return fmt.Errorf("task not found: %w", err)
			}
			return fmt.Errorf("failed to get task: %w", err)
		}

		if oldTask.Version != update.Version {
			return fmt.Errorf("conflict: task was updated by another process, please refresh: %w", domain_errors.ErrConflict)
		}

		userRole, err := u.teamRepo.GetMemberRole(txCtx, oldTask.TeamID, userID)
		if err != nil {
			return fmt.Errorf("access denied: user is not a member of the team: %w", err)
		}

		if update.AssigneeID != uuid.Nil && update.AssigneeID != oldTask.AssigneeID {
			_, err := u.teamRepo.GetMemberRole(txCtx, oldTask.TeamID, update.AssigneeID)
			if err != nil {
				return fmt.Errorf("invalid operation: assignee must be a member of the team: %w", domain_errors.ErrInvalid)
			}
		}

		isPrivileged := userRole == models.Owner || userRole == models.Admin
		isCreator := oldTask.CreatedBy == userID
		isAssignee := oldTask.AssigneeID == userID
		canUpdate := false

		if isPrivileged || isCreator {
			canUpdate = true
		} else if isAssignee {
			if update.Title != oldTask.Title || update.Description != oldTask.Description || update.AssigneeID != oldTask.AssigneeID {
				return fmt.Errorf("access denied: assignee can only change task status, don't touch other fucking fields: %w", domain_errors.ErrForbidden)
			}
			canUpdate = true
		}

		if !canUpdate {
			return fmt.Errorf("access denied: insufficient permissions to update this task: %w", domain_errors.ErrForbidden)
		}

		update.Version++

		if err := u.taskRepo.Update(txCtx, update); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}

		changesMap := make(map[string]any)
		if oldTask.Status != update.Status {
			changesMap["status"] = map[string]string{"old": oldTask.Status, "new": update.Status}
		}
		if oldTask.Title != update.Title {
			changesMap["title"] = map[string]string{"old": oldTask.Title, "new": update.Title}
		}
		if oldTask.Description != update.Description {
			changesMap["description"] = map[string]string{"old": oldTask.Description, "new": update.Description}
		}
		if oldTask.AssigneeID != update.AssigneeID {
			changesMap["assignee_id"] = map[string]any{"old": oldTask.AssigneeID, "new": update.AssigneeID}
		}

		if len(changesMap) > 0 {
			changes, _ := json.Marshal(changesMap)
			history := models.TaskHistory{
				TaskID:    update.ID,
				ChangedBy: userID,
				Changes:   string(changes),
			}

			if err := u.historyRepo.Create(txCtx, history); err != nil {
				return fmt.Errorf("failed to record task update history: %w", err)
			}
		}

		return nil
	})
}

func (u *taskUsecase) GetTaskHistory(ctx context.Context, userID, taskID uuid.UUID) ([]models.TaskHistory, error) {
	task, err := u.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve task: %w", err)
	}

	_, err = u.teamRepo.GetMemberRole(ctx, task.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("access denied: user is not a member of the task team: %w", err)
	}

	return u.historyRepo.GetByTaskID(ctx, taskID)
}

func (u *taskUsecase) AddComment(ctx context.Context, userID, taskID uuid.UUID, content string) (models.TaskComment, error) {
	task, err := u.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return models.TaskComment{}, fmt.Errorf("failed to retrieve task: %w", err)
	}

	userRole, err := u.teamRepo.GetMemberRole(ctx, task.TeamID, userID)
	if err != nil {
		return models.TaskComment{}, fmt.Errorf("access denied: user is not a member of the task team: %w", err)
	}

	if userRole != models.Owner && userRole != models.Admin && task.CreatedBy != userID && task.AssigneeID != userID {
		return models.TaskComment{}, fmt.Errorf("access denied: only admins, creators or assignees can comment: %w", domain_errors.ErrForbidden)
	}

	comment := models.TaskComment{
		TaskID:  taskID,
		UserID:  userID,
		Content: content,
	}

	if err := u.commentRepo.Create(ctx, comment); err != nil {
		return models.TaskComment{}, fmt.Errorf("failed to add comment: %w", err)
	}

	return comment, nil
}

func (u *taskUsecase) GetComments(ctx context.Context, userID, taskID uuid.UUID) ([]models.TaskComment, error) {
	task, err := u.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve task: %w", err)
	}

	_, err = u.teamRepo.GetMemberRole(ctx, task.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("access denied: user is not a member of the task team: %w", err)
	}

	return u.commentRepo.GetByTaskID(ctx, taskID)
}
