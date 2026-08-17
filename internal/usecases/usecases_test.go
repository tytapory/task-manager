//go:build integration

package usecases_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	domain_errors "manager/internal/domain/errors"
	"manager/internal/domain/models"
	dbmodels "manager/internal/infrastructure/storage/mysql/models"
	"manager/internal/infrastructure/storage/mysql/repository"
	"manager/internal/usecases"
)

var globalDB *gorm.DB

type mockHasher struct{}

func (m *mockHasher) Hash(password string) (string, error) { return password + "_hashed", nil }
func (m *mockHasher) Compare(hash, password string) error {
	if hash == password+"_hashed" {
		return nil
	}
	return errors.New("invalid password")
}

type mockTokenManager struct{}

func (m *mockTokenManager) GenerateToken(userID uuid.UUID) (string, error) {
	return "token_for_" + userID.String(), nil
}

type dummyTransactor struct{}

func (d *dummyTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type testData struct {
	OwnerID    uuid.UUID
	AdminID    uuid.UUID
	MemberID   uuid.UUID
	OutsiderID uuid.UUID
	TeamID     uuid.UUID
	TaskID     uuid.UUID
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysql.WithDatabase("test_db"),
		mysql.WithUsername("test_user"),
		mysql.WithPassword("test_pass"),
	)
	if err != nil {
		panic("Failed to start MySQL container: " + err.Error())
	}

	dsn, err := mysqlContainer.ConnectionString(ctx, "tls=false&parseTime=true")
	if err != nil {
		panic("Failed to get connection string: " + err.Error())
	}

	globalDB, err = gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect via GORM: " + err.Error())
	}

	err = globalDB.AutoMigrate(
		&dbmodels.User{}, &dbmodels.Team{}, &dbmodels.TeamMember{},
		&dbmodels.Task{}, &dbmodels.TaskHistory{}, &dbmodels.TaskComment{},
	)
	if err != nil {
		panic("Failed to run migrations: " + err.Error())
	}

	code := m.Run()

	if err := mysqlContainer.Terminate(ctx); err != nil {
		panic("Failed to terminate container: " + err.Error())
	}

	os.Exit(code)
}

func setupTestCase(t *testing.T) testData {
	globalDB.Exec("SET FOREIGN_KEY_CHECKS = 0;")

	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.TaskComment{})
	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.TaskHistory{})
	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.Task{})
	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.TeamMember{})
	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.Team{})
	globalDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&dbmodels.User{})

	globalDB.Exec("SET FOREIGN_KEY_CHECKS = 1;")

	td := testData{
		OwnerID:    uuid.New(),
		AdminID:    uuid.New(),
		MemberID:   uuid.New(),
		OutsiderID: uuid.New(),
		TeamID:     uuid.New(),
		TaskID:     uuid.New(),
	}

	globalDB.Create(&dbmodels.User{ID: td.OwnerID, Email: "owner@test.com", PasswordHash: "pass_hashed"})
	globalDB.Create(&dbmodels.User{ID: td.AdminID, Email: "admin@test.com", PasswordHash: "pass_hashed"})
	globalDB.Create(&dbmodels.User{ID: td.MemberID, Email: "member@test.com", PasswordHash: "pass_hashed"})
	globalDB.Create(&dbmodels.User{ID: td.OutsiderID, Email: "outsider@test.com", PasswordHash: "pass_hashed"})

	globalDB.Create(&dbmodels.Team{ID: td.TeamID, Name: "Alpha Team", CreatedBy: td.OwnerID})

	globalDB.Create(&dbmodels.TeamMember{TeamID: td.TeamID, UserID: td.OwnerID, Role: models.Owner})
	globalDB.Create(&dbmodels.TeamMember{TeamID: td.TeamID, UserID: td.AdminID, Role: models.Admin})
	globalDB.Create(&dbmodels.TeamMember{TeamID: td.TeamID, UserID: td.MemberID, Role: models.Member})

	globalDB.Create(&dbmodels.Task{
		ID: td.TaskID, TeamID: td.TeamID, Title: "Initial Task", Description: "Initial Description",
		Status: "todo", CreatedBy: td.OwnerID, AssigneeID: &td.MemberID, Version: 1,
	})

	return td
}

func TestUserUsecase_Register(t *testing.T) {
	uc := usecases.NewUserUsecase(repository.NewUserRepository(globalDB), &mockHasher{}, &mockTokenManager{})

	tests := []struct {
		name    string
		email   string
		pass    string
		uname   string
		wantErr bool
	}{
		{"Successful registration", "newuser@test.com", "123456", "John", false},
		{"Duplicate email failure", "owner@test.com", "123456", "Clone", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestCase(t)
			_, err := uc.Register(context.Background(), tt.email, tt.pass, tt.uname)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserUsecase_Login(t *testing.T) {
	uc := usecases.NewUserUsecase(repository.NewUserRepository(globalDB), &mockHasher{}, &mockTokenManager{})

	tests := []struct {
		name    string
		email   string
		pass    string
		wantErr bool
	}{
		{"Successful login", "owner@test.com", "pass", false},
		{"Incorrect password", "owner@test.com", "wrongpass", true},
		{"Non-existent user", "unknown@test.com", "pass", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestCase(t)
			token, err := uc.Login(context.Background(), tt.email, tt.pass)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
			}
		})
	}
}

func TestTeamUsecase_CreateTeam(t *testing.T) {
	uc := usecases.NewTeamUsecase(repository.NewTeamRepository(globalDB), repository.NewUserRepository(globalDB), &dummyTransactor{})

	tests := []struct {
		name    string
		userID  func(td testData) uuid.UUID
		tname   string
		wantErr bool
	}{
		{
			name:    "Successful team creation",
			userID:  func(td testData) uuid.UUID { return td.OutsiderID },
			tname:   "Beta Team",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			team, err := uc.CreateTeam(context.Background(), tt.userID(td), tt.tname)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.tname, team.Name)
			}
		})
	}
}

func TestTeamUsecase_ListTeams(t *testing.T) {
	uc := usecases.NewTeamUsecase(repository.NewTeamRepository(globalDB), repository.NewUserRepository(globalDB), &dummyTransactor{})

	t.Run("Owner sees their team", func(t *testing.T) {
		td := setupTestCase(t)
		teams, err := uc.ListTeams(context.Background(), td.OwnerID)
		assert.NoError(t, err)
		assert.Len(t, teams, 1)
	})

	t.Run("Outsider sees no teams", func(t *testing.T) {
		td := setupTestCase(t)
		teams, err := uc.ListTeams(context.Background(), td.OutsiderID)
		assert.NoError(t, err)
		assert.Len(t, teams, 0)
	})
}

func TestTeamUsecase_InviteMember(t *testing.T) {
	uc := usecases.NewTeamUsecase(repository.NewTeamRepository(globalDB), repository.NewUserRepository(globalDB), &dummyTransactor{})

	tests := []struct {
		name      string
		inviterID func(td testData) uuid.UUID
		targetID  func(td testData) uuid.UUID
		role      models.TeamRole
		wantErr   bool
	}{
		{
			name:      "Admin invites user as member",
			inviterID: func(td testData) uuid.UUID { return td.AdminID },
			targetID:  func(td testData) uuid.UUID { return td.OutsiderID },
			role:      models.Member,
			wantErr:   false,
		},
		{
			name:      "Regular member attempts invite",
			inviterID: func(td testData) uuid.UUID { return td.MemberID },
			targetID:  func(td testData) uuid.UUID { return td.OutsiderID },
			role:      models.Member,
			wantErr:   true,
		},
		{
			name:      "Attempt to assign owner role",
			inviterID: func(td testData) uuid.UUID { return td.OwnerID },
			targetID:  func(td testData) uuid.UUID { return td.OutsiderID },
			role:      models.Owner,
			wantErr:   true,
		},
		{
			name:      "Inviter not in team",
			inviterID: func(td testData) uuid.UUID { return td.OutsiderID },
			targetID:  func(td testData) uuid.UUID { return td.MemberID },
			role:      models.Member,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			err := uc.InviteMember(context.Background(), tt.inviterID(td), td.TeamID, tt.targetID(td), tt.role)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTeamUsecase_ChangeMemberRole(t *testing.T) {
	uc := usecases.NewTeamUsecase(repository.NewTeamRepository(globalDB), repository.NewUserRepository(globalDB), &dummyTransactor{})

	tests := []struct {
		name        string
		requesterID func(td testData) uuid.UUID
		targetID    func(td testData) uuid.UUID
		newRole     models.TeamRole
		wantErr     bool
	}{
		{
			name:        "Owner promotes member to admin",
			requesterID: func(td testData) uuid.UUID { return td.OwnerID },
			targetID:    func(td testData) uuid.UUID { return td.MemberID },
			newRole:     models.Admin,
			wantErr:     false,
		},
		{
			name:        "Admin attempts role change",
			requesterID: func(td testData) uuid.UUID { return td.AdminID },
			targetID:    func(td testData) uuid.UUID { return td.MemberID },
			newRole:     models.Admin,
			wantErr:     true,
		},
		{
			name:        "Attempt to assign owner role",
			requesterID: func(td testData) uuid.UUID { return td.OwnerID },
			targetID:    func(td testData) uuid.UUID { return td.MemberID },
			newRole:     models.Owner,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			err := uc.ChangeMemberRole(context.Background(), tt.requesterID(td), td.TeamID, tt.targetID(td), tt.newRole)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTeamUsecase_GetTeamStats(t *testing.T) {
	uc := usecases.NewTeamUsecase(repository.NewTeamRepository(globalDB), repository.NewUserRepository(globalDB), &dummyTransactor{})

	tests := []struct {
		name    string
		userID  func(td testData) uuid.UUID
		wantErr bool
	}{
		{
			name:    "Owner views stats",
			userID:  func(td testData) uuid.UUID { return td.OwnerID },
			wantErr: false,
		},
		{
			name:    "Admin views stats",
			userID:  func(td testData) uuid.UUID { return td.AdminID },
			wantErr: false,
		},
		{
			name:    "Member attempts viewing stats",
			userID:  func(td testData) uuid.UUID { return td.MemberID },
			wantErr: true,
		},
		{
			name:    "Outsider attempts viewing stats",
			userID:  func(td testData) uuid.UUID { return td.OutsiderID },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			stats, err := uc.GetTeamStats(context.Background(), tt.userID(td), td.TeamID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, stats)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stats)
			}
		})
	}
}

func TestTaskUsecase_CreateTask(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	tests := []struct {
		name    string
		userID  func(td testData) uuid.UUID
		wantErr bool
	}{
		{
			name:    "Team member creates task",
			userID:  func(td testData) uuid.UUID { return td.MemberID },
			wantErr: false,
		},
		{
			name:    "Outsider attempts creation",
			userID:  func(td testData) uuid.UUID { return td.OutsiderID },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			_, err := uc.CreateTask(context.Background(), tt.userID(td), td.TeamID, "Task Title", "Task Description")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskUsecase_GetTasks(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	t.Run("Member lists tasks", func(t *testing.T) {
		td := setupTestCase(t)
		tasks, err := uc.GetTasks(context.Background(), td.MemberID, td.TeamID, "", nil, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, tasks, 1)
	})

	t.Run("Outsider lists tasks", func(t *testing.T) {
		td := setupTestCase(t)
		_, err := uc.GetTasks(context.Background(), td.OutsiderID, td.TeamID, "", nil, 10, 0)
		assert.Error(t, err)
	})
}

func TestTaskUsecase_UpdateTask(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	tests := []struct {
		name     string
		userID   func(td testData) uuid.UUID
		modifier func(t *models.Task)
		wantErr  bool
		errType  error
	}{
		{
			name:   "Owner updates all fields",
			userID: func(td testData) uuid.UUID { return td.OwnerID },
			modifier: func(t *models.Task) {
				t.Title = "Updated Title"
				t.Status = "in_progress"
			},
			wantErr: false,
		},
		{
			name:   "Assignee updates status only",
			userID: func(td testData) uuid.UUID { return td.MemberID },
			modifier: func(t *models.Task) {
				t.Status = "done"
			},
			wantErr: false,
		},
		{
			name:   "Assignee attempts title update",
			userID: func(td testData) uuid.UUID { return td.MemberID },
			modifier: func(t *models.Task) {
				t.Title = "Unauthorized Title Change"
			},
			wantErr: true,
			errType: domain_errors.ErrForbidden,
		},
		{
			name:   "Version mismatch conflict",
			userID: func(td testData) uuid.UUID { return td.OwnerID },
			modifier: func(t *models.Task) {
				t.Version = 999
			},
			wantErr: true,
			errType: domain_errors.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			taskRepo := repository.NewTaskRepository(globalDB)
			task, _ := taskRepo.GetByID(context.Background(), td.TaskID)

			tt.modifier(&task)

			err := uc.UpdateTask(context.Background(), tt.userID(td), task)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.True(t, errors.Is(err, tt.errType), "Expected error %v, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskUsecase_GetTaskHistory(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	t.Run("Member gets history", func(t *testing.T) {
		td := setupTestCase(t)
		_, err := uc.GetTaskHistory(context.Background(), td.MemberID, td.TaskID)
		assert.NoError(t, err)
	})

	t.Run("Outsider gets history", func(t *testing.T) {
		td := setupTestCase(t)
		_, err := uc.GetTaskHistory(context.Background(), td.OutsiderID, td.TaskID)
		assert.Error(t, err)
	})
}

func TestTaskUsecase_AddComment(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	tests := []struct {
		name    string
		userID  func(td testData) uuid.UUID
		wantErr bool
	}{
		{"Owner adds comment", func(td testData) uuid.UUID { return td.OwnerID }, false},
		{"Assignee adds comment", func(td testData) uuid.UUID { return td.MemberID }, false},
		{"Outsider attempts comment", func(td testData) uuid.UUID { return td.OutsiderID }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := setupTestCase(t)
			_, err := uc.AddComment(context.Background(), tt.userID(td), td.TaskID, "Test comment content")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskUsecase_GetComments(t *testing.T) {
	uc := usecases.NewTaskUsecase(
		repository.NewTaskRepository(globalDB), repository.NewTeamRepository(globalDB),
		repository.NewTaskHistoryRepository(globalDB), repository.NewTaskCommentRepository(globalDB),
		&dummyTransactor{},
	)

	t.Run("Member gets comments", func(t *testing.T) {
		td := setupTestCase(t)
		_, err := uc.GetComments(context.Background(), td.MemberID, td.TaskID)
		assert.NoError(t, err)
	})

	t.Run("Outsider gets comments", func(t *testing.T) {
		td := setupTestCase(t)
		_, err := uc.GetComments(context.Background(), td.OutsiderID, td.TaskID)
		assert.Error(t, err)
	})
}
