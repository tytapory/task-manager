package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	domain_errors "manager/internal/domain/errors"
	"manager/internal/domain/models"
	"manager/internal/usecases"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Handler interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	CreateTeam(w http.ResponseWriter, r *http.Request)
	ListTeams(w http.ResponseWriter, r *http.Request)
	InviteMember(w http.ResponseWriter, r *http.Request)
	GetTeamStats(w http.ResponseWriter, r *http.Request)
	CreateTask(w http.ResponseWriter, r *http.Request)
	GetTasks(w http.ResponseWriter, r *http.Request)
	UpdateTask(w http.ResponseWriter, r *http.Request)
	GetTaskHistory(w http.ResponseWriter, r *http.Request)
	AddComment(w http.ResponseWriter, r *http.Request)
	GetComments(w http.ResponseWriter, r *http.Request)
	ChangeMemberRole(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	userUC usecases.UserUsecase
	teamUC usecases.TeamUsecase
	taskUC usecases.TaskUsecase
}

func NewHandler(u usecases.UserUsecase, te usecases.TeamUsecase, ta usecases.TaskUsecase) Handler {
	return &handler{userUC: u, teamUC: te, taskUC: ta}
}

func NewRouter(h Handler, jwtSecret string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/register", h.Register)
	mux.HandleFunc("POST /api/v1/login", h.Login)

	mux.Handle("POST /api/v1/teams", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.CreateTeam))))
	mux.Handle("GET /api/v1/teams", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.ListTeams))))
	mux.Handle("POST /api/v1/teams/{id}/invite", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.InviteMember))))
	mux.Handle("GET /api/v1/teams/{id}/stats", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.GetTeamStats))))
	mux.Handle("PUT /api/v1/teams/{id}/members/{user_id}/role", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.ChangeMemberRole))))

	mux.Handle("POST /api/v1/tasks", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.CreateTask))))
	mux.Handle("GET /api/v1/tasks", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.GetTasks))))
	mux.Handle("PUT /api/v1/tasks/{id}", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.UpdateTask))))
	mux.Handle("GET /api/v1/tasks/{id}/history", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.GetTaskHistory))))
	mux.Handle("POST /api/v1/tasks/{id}/comments", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.AddComment))))
	mux.Handle("GET /api/v1/tasks/{id}/comments", loggingMiddleware(authMiddleware(jwtSecret, http.HandlerFunc(h.GetComments))))

	return mux
}

func sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func respondWithError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))

	var statusCode int
	var clientMsg string

	switch {
	case errors.Is(err, domain_errors.ErrNotFound):
		statusCode = http.StatusNotFound
		clientMsg = err.Error()
	case errors.Is(err, domain_errors.ErrInvalid):
		statusCode = http.StatusBadRequest
		clientMsg = err.Error()
	case errors.Is(err, domain_errors.ErrForbidden):
		statusCode = http.StatusForbidden
		clientMsg = err.Error()
	case errors.Is(err, domain_errors.ErrUnauthorized):
		statusCode = http.StatusUnauthorized
		clientMsg = err.Error()
	case errors.Is(err, domain_errors.ErrConflict):
		statusCode = http.StatusConflict
		clientMsg = err.Error()
	default:
		statusCode = http.StatusInternalServerError
		clientMsg = "internal server error"
	}

	sendJSONError(w, statusCode, clientMsg)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.InfoContext(r.Context(), "http request processed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func authMiddleware(secretKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			slog.WarnContext(r.Context(), "missing authorization header")
			sendJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secretKey), nil
		})

		if err != nil || !token.Valid {
			slog.WarnContext(r.Context(), "invalid or expired token", slog.String("error", err.Error()))
			sendJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			slog.WarnContext(r.Context(), "invalid token claims")
			sendJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		sub, ok := claims["sub"].(string)
		if !ok {
			slog.WarnContext(r.Context(), "missing sub claim in token")
			sendJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		userID, err := uuid.Parse(sub)
		if err != nil {
			slog.WarnContext(r.Context(), "invalid user id in token sub", slog.String("error", err.Error()))
			sendJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value("userID").(uuid.UUID)
	return id
}

// Register godoc
// @Summary Register a new user
// @Description Creates a new user account with email, password, and name.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "User registration request"
// @Success 200 {object} UserResponse "User successfully registered"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/register [post]
func (h *handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.userUC.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to register user")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toUserResponse(user))
}

// Login godoc
// @Summary User login
// @Description Authenticates a user and returns a JWT token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "User credentials"
// @Success 200 {object} map[string]string "JWT Token"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/login [post]
func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token, err := h.userUC.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to login user")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// CreateTeam godoc
// @Summary Create a new team
// @Description Creates a new team for the authenticated user.
// @Tags Teams
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateTeamRequest true "Team creation payload"
// @Success 200 {object} TeamResponse "Team created"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/teams [post]
func (h *handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	team, err := h.teamUC.CreateTeam(r.Context(), getUserID(r.Context()), req.Name)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to create team")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTeamResponse(team))
}

// ListTeams godoc
// @Summary List user teams
// @Description Retrieves a list of teams the authenticated user belongs to.
// @Tags Teams
// @Security BearerAuth
// @Produce json
// @Success 200 {array} TeamResponse "List of teams"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/teams [get]
func (h *handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.teamUC.ListTeams(r.Context(), getUserID(r.Context()))
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to list teams")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTeamResponseList(teams))
}

// InviteMember godoc
// @Summary Invite a member to a team
// @Description Invites another user to the specified team with a specific role.
// @Tags Teams
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param request body InviteMemberRequest true "Invitation details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/teams/{id}/invite [post]
func (h *handler) InviteMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid team id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var req InviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	targetID, err := uuid.Parse(req.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid target user id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid target user id")
		return
	}
	if err := h.teamUC.InviteMember(r.Context(), getUserID(r.Context()), teamID, targetID, models.TeamRole(req.Role)); err != nil {
		respondWithError(r.Context(), w, err, "failed to invite member")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GetTeamStats godoc
// @Summary Get team statistics
// @Description Retrieves statistics for a specific team.
// @Tags Teams
// @Security BearerAuth
// @Produce json
// @Param id path string true "Team ID"
// @Success 200 {object} TeamStatsResponse "Team statistics"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/teams/{id}/stats [get]
func (h *handler) GetTeamStats(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid team id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	stats, err := h.teamUC.GetTeamStats(r.Context(), getUserID(r.Context()), teamID)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to get team stats")
		return
	}

	domainStats, ok := stats.(models.TeamStats)
	if !ok {
		slog.ErrorContext(r.Context(), "failed to cast stats to domain model")
		sendJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTeamStatsResponse(domainStats))
}

// CreateTask godoc
// @Summary Create a new task
// @Description Creates a new task within a specific team.
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Task creation details"
// @Success 200 {object} TaskResponse "Task created"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks [post]
func (h *handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	teamID, err := uuid.Parse(req.TeamID)
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid team id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	task, err := h.taskUC.CreateTask(r.Context(), getUserID(r.Context()), teamID, req.Title, req.Description)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to create task")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTaskResponse(task))
}

// GetTasks godoc
// @Summary Get list of tasks
// @Description Retrieves tasks for a team with optional filtering and pagination.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param team_id query string true "Team ID"
// @Param limit query int false "Pagination limit" default(20)
// @Param offset query int false "Pagination offset" default(0)
// @Param status query string false "Filter by task status"
// @Param assignee_id query string false "Filter by assignee ID"
// @Success 200 {array} TaskResponse "List of tasks"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks [get]
func (h *handler) GetTasks(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.URL.Query().Get("team_id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid team id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid team id")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	status := r.URL.Query().Get("status")

	var assigneeID *uuid.UUID
	if a := r.URL.Query().Get("assignee_id"); a != "" {
		if parsed, err := uuid.Parse(a); err == nil {
			assigneeID = &parsed
		}
	}

	tasks, err := h.taskUC.GetTasks(r.Context(), getUserID(r.Context()), teamID, status, assigneeID, limit, offset)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to get tasks")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTaskResponseList(tasks))
}

// UpdateTask godoc
// @Summary Update an existing task
// @Description Updates the details of a specific task.
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body UpdateTaskRequest true "Task update data"
// @Success 200 {string} string "OK"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks/{id} [put]
func (h *handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid task id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	task := models.Task{
		ID:          taskID,
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		AssigneeID:  req.AssigneeID,
		Version:     req.Version,
	}

	if err := h.taskUC.UpdateTask(r.Context(), getUserID(r.Context()), task); err != nil {
		respondWithError(r.Context(), w, err, "failed to update task")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GetTaskHistory godoc
// @Summary Get task history
// @Description Retrieves the history of changes for a specific task.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {array} TaskHistoryResponse "Task history"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks/{id}/history [get]
func (h *handler) GetTaskHistory(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid task id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	history, err := h.taskUC.GetTaskHistory(r.Context(), getUserID(r.Context()), taskID)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to get task history")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTaskHistoryResponseList(history))
}

// AddComment godoc
// @Summary Add comment to a task
// @Description Adds a new comment to a specific task.
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body AddCommentRequest true "Comment content"
// @Success 200 {object} TaskCommentResponse "Created comment"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks/{id}/comments [post]
func (h *handler) AddComment(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid task id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	var req AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	comment, err := h.taskUC.AddComment(r.Context(), getUserID(r.Context()), taskID, req.Content)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to add comment")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTaskCommentResponse(comment))
}

// GetComments godoc
// @Summary Get task comments
// @Description Retrieves all comments for a specific task.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {array} TaskCommentResponse "List of comments"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tasks/{id}/comments [get]
func (h *handler) GetComments(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid task id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	comments, err := h.taskUC.GetComments(r.Context(), getUserID(r.Context()), taskID)
	if err != nil {
		respondWithError(r.Context(), w, err, "failed to get comments")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTaskCommentResponseList(comments))
}

// ChangeMemberRole godoc
// @Summary Change team member role
// @Description Changes the role of a user within a team.
// @Tags Teams
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param user_id path string true "Target User ID"
// @Param request body ChangeRoleRequest true "New role details"
// @Success 200 {string} string "OK"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/teams/{id}/members/{user_id}/role [put]
func (h *handler) ChangeMemberRole(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid team id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid team id")
		return
	}

	targetID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		slog.ErrorContext(r.Context(), "invalid user id", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid target user id")
		return
	}

	var req ChangeRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", err.Error()))
		sendJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.teamUC.ChangeMemberRole(r.Context(), getUserID(r.Context()), teamID, targetID, models.TeamRole(req.Role)); err != nil {
		respondWithError(r.Context(), w, err, "failed to change member role")
		return
	}

	w.WriteHeader(http.StatusOK)
}
