package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"manager/internal/domain/models"
	core_repo "manager/internal/repository"
)

type userCacheRepo struct {
	base  core_repo.UserRepository
	redis *redis.Client
	ttl   time.Duration
}

func NewUserCacheRepository(base core_repo.UserRepository, rdb *redis.Client, ttl time.Duration) core_repo.UserRepository {
	return &userCacheRepo{
		base:  base,
		redis: rdb,
		ttl:   ttl,
	}
}

func (r *userCacheRepo) Create(ctx context.Context, user models.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.Create(ctx, user)
}

func (r *userCacheRepo) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	if err := ctx.Err(); err != nil {
		var zero models.User
		return zero, err
	}

	key := fmt.Sprintf("user:id:%s", id.String())
	var user models.User

	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(val), &user); err == nil {
			return user, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			var zero models.User
			return zero, ctxErr
		}
	}

	user, err = r.base.GetByID(ctx, id)
	if err != nil {
		return user, err
	}

	if data, marshalErr := json.Marshal(user); marshalErr == nil {
		_ = r.redis.Set(ctx, key, data, r.ttl).Err()
	}

	return user, nil
}

func (r *userCacheRepo) GetByEmail(ctx context.Context, email string) (models.User, error) {
	if err := ctx.Err(); err != nil {
		var zero models.User
		return zero, err
	}

	key := fmt.Sprintf("user:email:%s", email)
	var user models.User

	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(val), &user); err == nil {
			return user, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			var zero models.User
			return zero, ctxErr
		}
	}

	user, err = r.base.GetByEmail(ctx, email)
	if err != nil {
		return user, err
	}

	if data, marshalErr := json.Marshal(user); marshalErr == nil {
		_ = r.redis.Set(ctx, key, data, r.ttl).Err()
	}

	return user, nil
}

type taskCacheRepo struct {
	base  core_repo.TaskRepository
	redis *redis.Client
	ttl   time.Duration
}

func NewTaskCacheRepository(base core_repo.TaskRepository, rdb *redis.Client, ttl time.Duration) core_repo.TaskRepository {
	return &taskCacheRepo{
		base:  base,
		redis: rdb,
		ttl:   ttl,
	}
}

func (r *taskCacheRepo) invalidateTeamCache(ctx context.Context, teamID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	pattern := "task:team:" + teamID.String() + ":*"
	iter := r.redis.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.redis.Del(ctx, iter.Val())
	}
	return iter.Err()
}

func (r *taskCacheRepo) Create(ctx context.Context, task models.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.base.Create(ctx, task)
	if err == nil {
		_ = r.invalidateTeamCache(ctx, task.TeamID)
	}
	return err
}

func (r *taskCacheRepo) GetByID(ctx context.Context, id uuid.UUID) (models.Task, error) {
	if err := ctx.Err(); err != nil {
		var zero models.Task
		return zero, err
	}

	key := fmt.Sprintf("task:id:%s", id.String())
	var task models.Task

	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(val), &task); err == nil {
			return task, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			var zero models.Task
			return zero, ctxErr
		}
	}

	task, err = r.base.GetByID(ctx, id)
	if err != nil {
		return task, err
	}

	if data, marshalErr := json.Marshal(task); marshalErr == nil {
		_ = r.redis.Set(ctx, key, data, r.ttl).Err()
	}

	return task, nil
}

func (r *taskCacheRepo) Update(ctx context.Context, task models.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.base.Update(ctx, task)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("task:id:%s", task.ID.String())
	r.redis.Del(ctx, key)
	_ = r.invalidateTeamCache(ctx, task.TeamID)

	return nil
}

func (r *taskCacheRepo) ListByTeam(ctx context.Context, teamID uuid.UUID, status string, assigneeID *uuid.UUID, limit, offset int) ([]models.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	assIDStr := "nil"
	if assigneeID != nil {
		assIDStr = assigneeID.String()
	}

	key := fmt.Sprintf("task:team:%s:s:%s:a:%s:l:%d:o:%d", teamID.String(), status, assIDStr, limit, offset)

	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		var tasks []models.Task
		if err := json.Unmarshal([]byte(val), &tasks); err == nil {
			return tasks, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
	}

	tasks, err := r.base.ListByTeam(ctx, teamID, status, assigneeID, limit, offset)
	if err != nil {
		return nil, err
	}

	if data, marshalErr := json.Marshal(tasks); marshalErr == nil {
		_ = r.redis.Set(ctx, key, data, r.ttl).Err()
	}

	return tasks, nil
}

type teamCacheRepo struct {
	base  core_repo.TeamRepository
	redis *redis.Client
	ttl   time.Duration
}

func NewTeamCacheRepository(base core_repo.TeamRepository, rdb *redis.Client, ttl time.Duration) core_repo.TeamRepository {
	return &teamCacheRepo{
		base:  base,
		redis: rdb,
		ttl:   ttl,
	}
}

func (r *teamCacheRepo) Create(ctx context.Context, team models.Team) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.Create(ctx, team)
}

func (r *teamCacheRepo) GetByID(ctx context.Context, id uuid.UUID) (models.Team, error) {
	if err := ctx.Err(); err != nil {
		var zero models.Team
		return zero, err
	}

	key := fmt.Sprintf("team:id:%s", id.String())
	var team models.Team

	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(val), &team); err == nil {
			return team, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			var zero models.Team
			return zero, ctxErr
		}
	}

	team, err = r.base.GetByID(ctx, id)
	if err != nil {
		return team, err
	}

	if data, marshalErr := json.Marshal(team); marshalErr == nil {
		_ = r.redis.Set(ctx, key, data, r.ttl).Err()
	}

	return team, nil
}

func (r *teamCacheRepo) AddMember(ctx context.Context, member models.TeamMember) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.AddMember(ctx, member)
}

func (r *teamCacheRepo) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (models.TeamRole, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return r.base.GetMemberRole(ctx, teamID, userID)
}

func (r *teamCacheRepo) GetTeamsByUserID(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.base.GetTeamsByUserID(ctx, userID)
}

func (r *teamCacheRepo) GetStats(ctx context.Context, teamID uuid.UUID) (models.TeamStats, error) {
	if err := ctx.Err(); err != nil {
		return models.TeamStats{}, err
	}
	return r.base.GetStats(ctx, teamID)
}

func (r *teamCacheRepo) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.UpdateMemberRole(ctx, teamID, userID, role)
}

type taskHistoryCacheRepo struct {
	base core_repo.TaskHistoryRepository
}

func NewTaskHistoryCacheRepository(base core_repo.TaskHistoryRepository) core_repo.TaskHistoryRepository {
	return &taskHistoryCacheRepo{
		base: base,
	}
}

func (r *taskHistoryCacheRepo) Create(ctx context.Context, history models.TaskHistory) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.Create(ctx, history)
}

func (r *taskHistoryCacheRepo) GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskHistory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.base.GetByTaskID(ctx, taskID)
}

type taskCommentCacheRepo struct {
	base core_repo.TaskCommentRepository
}

func NewTaskCommentCacheRepository(base core_repo.TaskCommentRepository) core_repo.TaskCommentRepository {
	return &taskCommentCacheRepo{
		base: base,
	}
}

func (r *taskCommentCacheRepo) Create(ctx context.Context, comment models.TaskComment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.base.Create(ctx, comment)
}

func (r *taskCommentCacheRepo) GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskComment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.base.GetByTaskID(ctx, taskID)
}
