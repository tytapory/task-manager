package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain_errors "manager/internal/domain/errors"
	domain "manager/internal/domain/models"
	"manager/internal/infrastructure/storage/mysql"
	dbmodels "manager/internal/infrastructure/storage/mysql/models"
	core_repo "manager/internal/repository"
)

func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain_errors.ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain_errors.ErrConflict
	}
	return err
}

func getDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx := mysql.ExtractTx(ctx); tx != nil {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) core_repo.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbUser := dbmodels.UserFromDomain(user)
	return wrapErr(getDB(ctx, r.db).Create(&dbUser).Error)
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	var dbUser dbmodels.User
	err := getDB(ctx, r.db).Where("id = ?", id).First(&dbUser).Error
	if err != nil {
		return domain.User{}, wrapErr(err)
	}
	return dbUser.ToDomain(), nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	var dbUser dbmodels.User
	err := getDB(ctx, r.db).Where("email = ?", email).First(&dbUser).Error
	if err != nil {
		return domain.User{}, wrapErr(err)
	}
	return dbUser.ToDomain(), nil
}

type teamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) core_repo.TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) GetTeamsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var dbTeams []dbmodels.Team
	err := getDB(ctx, r.db).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ?", userID).
		Find(&dbTeams).Error
	if err != nil {
		return nil, wrapErr(err)
	}

	teams := make([]domain.Team, len(dbTeams))
	for i, t := range dbTeams {
		teams[i] = t.ToDomain()
	}
	return teams, nil
}

func (r *teamRepository) GetStats(ctx context.Context, teamID uuid.UUID) (domain.TeamStats, error) {
	if err := ctx.Err(); err != nil {
		return domain.TeamStats{}, err
	}
	query := `
	WITH status_counts AS (
		SELECT team_id, JSON_OBJECTAGG(status, count) as tasks_by_status
		FROM (SELECT team_id, status, COUNT(*) as count FROM tasks WHERE team_id = ? GROUP BY team_id, status) t
		GROUP BY team_id
	),
	top_assignees AS (
		SELECT team_id, JSON_ARRAYAGG(JSON_OBJECT('user_id', assignee_id, 'count', closed_count)) as top_users
		FROM (
			SELECT team_id, assignee_id, COUNT(*) as closed_count
			FROM tasks
			WHERE team_id = ? AND closed_at >= DATE_SUB(NOW(), INTERVAL 30 DAY) AND assignee_id IS NOT NULL
			GROUP BY team_id, assignee_id
			ORDER BY closed_count DESC
			LIMIT 3
		) t
		GROUP BY team_id
	),
	avg_time AS (
		SELECT team_id, AVG(TIMESTAMPDIFF(SECOND, created_at, closed_at)) as avg_seconds
		FROM tasks
		WHERE team_id = ? AND closed_at IS NOT NULL
		GROUP BY team_id
	),
	comments_count AS (
		SELECT t.team_id, COUNT(tc.id) as total_comments
		FROM tasks t
		LEFT JOIN task_comments tc ON t.id = tc.task_id
		WHERE t.team_id = ?
		GROUP BY t.team_id
	)
	SELECT
		COALESCE(s.tasks_by_status, '{}') as tasks_by_status,
		COALESCE(a.top_users, '[]') as top_assignees,
		COALESCE(av.avg_seconds, 0) as average_close_time_seconds,
		COALESCE(c.total_comments, 0) as total_comments
	FROM (SELECT ? as team_id) base
	LEFT JOIN status_counts s ON s.team_id = base.team_id
	LEFT JOIN top_assignees a ON a.team_id = base.team_id
	LEFT JOIN avg_time av ON av.team_id = base.team_id
	LEFT JOIN comments_count c ON c.team_id = base.team_id;
	`

	var raw struct {
		TasksByStatus           string
		TopAssignees            string
		AverageCloseTimeSeconds float64
		TotalComments           int
	}

	err := getDB(ctx, r.db).Raw(query, teamID, teamID, teamID, teamID, teamID).Scan(&raw).Error
	if err != nil {
		return domain.TeamStats{}, wrapErr(err)
	}

	var stats domain.TeamStats
	stats.AverageCloseTimeSeconds = raw.AverageCloseTimeSeconds
	stats.TotalComments = raw.TotalComments

	if raw.TasksByStatus != "" {
		_ = json.Unmarshal([]byte(raw.TasksByStatus), &stats.TasksByStatus)
	}
	if raw.TopAssignees != "" {
		_ = json.Unmarshal([]byte(raw.TopAssignees), &stats.TopAssignees)
	}

	if stats.TasksByStatus == nil {
		stats.TasksByStatus = make(map[string]int)
	}
	if stats.TopAssignees == nil {
		stats.TopAssignees = make([]domain.AssigneeStat, 0)
	}

	return stats, nil
}

func (r *teamRepository) Create(ctx context.Context, team domain.Team) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbTeam := dbmodels.TeamFromDomain(team)
	return wrapErr(getDB(ctx, r.db).Create(&dbTeam).Error)
}

func (r *teamRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Team, error) {
	if err := ctx.Err(); err != nil {
		return domain.Team{}, err
	}
	var dbTeam dbmodels.Team
	err := getDB(ctx, r.db).Where("id = ?", id).First(&dbTeam).Error
	if err != nil {
		return domain.Team{}, wrapErr(err)
	}
	return dbTeam.ToDomain(), nil
}

func (r *teamRepository) AddMember(ctx context.Context, member domain.TeamMember) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbMember := dbmodels.TeamMemberFromDomain(member)
	return wrapErr(getDB(ctx, r.db).Create(&dbMember).Error)
}

func (r *teamRepository) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.TeamRole, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var dbMember dbmodels.TeamMember
	err := getDB(ctx, r.db).Where("team_id = ? AND user_id = ?", teamID, userID).First(&dbMember).Error
	if err != nil {
		return "", wrapErr(err)
	}
	return dbMember.Role, nil
}

func (r *teamRepository) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role domain.TeamRole) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	res := getDB(ctx, r.db).Model(&dbmodels.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Update("role", role)

	if res.Error != nil {
		return wrapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain_errors.ErrNotFound
	}
	return nil
}

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) core_repo.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, task domain.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbTask := dbmodels.TaskFromDomain(task)
	return wrapErr(getDB(ctx, r.db).Create(&dbTask).Error)
}

func (r *taskRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	var dbTask dbmodels.Task
	err := getDB(ctx, r.db).Where("id = ?", id).First(&dbTask).Error
	if err != nil {
		return domain.Task{}, wrapErr(err)
	}
	return dbTask.ToDomain(), nil
}

func (r *taskRepository) Update(ctx context.Context, task domain.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbTask := dbmodels.TaskFromDomain(task)

	res := getDB(ctx, r.db).Model(&dbmodels.Task{}).
		Where("id = ? AND version = ?", dbTask.ID, task.Version-1).
		Updates(map[string]any{
			"title":       dbTask.Title,
			"description": dbTask.Description,
			"status":      dbTask.Status,
			"assignee_id": dbTask.AssigneeID,
			"closed_at":   dbTask.ClosedAt,
			"version":     dbTask.Version,
		})

	if res.Error != nil {
		return wrapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain_errors.ErrNotFound
	}
	return nil
}

func (r *taskRepository) ListByTeam(ctx context.Context, teamID uuid.UUID, status string, assigneeID *uuid.UUID, limit, offset int) ([]domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var dbTasks []dbmodels.Task

	query := getDB(ctx, r.db).Where("team_id = ?", teamID)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if assigneeID != nil {
		query = query.Where("assignee_id = ?", *assigneeID)
	}

	err := query.Limit(limit).Offset(offset).Find(&dbTasks).Error
	if err != nil {
		return nil, wrapErr(err)
	}

	tasks := make([]domain.Task, len(dbTasks))
	for i, t := range dbTasks {
		tasks[i] = t.ToDomain()
	}
	return tasks, nil
}

type taskHistoryRepository struct {
	db *gorm.DB
}

func NewTaskHistoryRepository(db *gorm.DB) core_repo.TaskHistoryRepository {
	return &taskHistoryRepository{db: db}
}

func (r *taskHistoryRepository) Create(ctx context.Context, history domain.TaskHistory) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbHistory := dbmodels.TaskHistory{
		TaskID:    history.TaskID,
		ChangedBy: history.ChangedBy,
		Changes:   history.Changes,
	}
	return wrapErr(getDB(ctx, r.db).Create(&dbHistory).Error)
}

func (r *taskHistoryRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskHistory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var dbHistories []dbmodels.TaskHistory
	err := getDB(ctx, r.db).Where("task_id = ?", taskID).Find(&dbHistories).Error
	if err != nil {
		return nil, wrapErr(err)
	}

	histories := make([]domain.TaskHistory, len(dbHistories))
	for i, h := range dbHistories {
		histories[i] = domain.TaskHistory{
			ID:        h.ID,
			TaskID:    h.TaskID,
			ChangedBy: h.ChangedBy,
			Changes:   h.Changes,
			CreatedAt: h.CreatedAt,
		}
	}
	return histories, nil
}

type taskCommentRepository struct {
	db *gorm.DB
}

func NewTaskCommentRepository(db *gorm.DB) core_repo.TaskCommentRepository {
	return &taskCommentRepository{db: db}
}

func (r *taskCommentRepository) Create(ctx context.Context, comment domain.TaskComment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dbComment := dbmodels.TaskComment{
		ID:      comment.ID,
		TaskID:  comment.TaskID,
		UserID:  comment.UserID,
		Content: comment.Content,
	}
	return wrapErr(getDB(ctx, r.db).Create(&dbComment).Error)
}

func (r *taskCommentRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskComment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var dbComments []dbmodels.TaskComment
	err := getDB(ctx, r.db).Where("task_id = ?", taskID).Find(&dbComments).Error
	if err != nil {
		return nil, wrapErr(err)
	}

	comments := make([]domain.TaskComment, len(dbComments))
	for i, c := range dbComments {
		comments[i] = domain.TaskComment{
			ID:        c.ID,
			TaskID:    c.TaskID,
			UserID:    c.UserID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt,
		}
	}
	return comments, nil
}
