package main

import (
	"log/slog"
	"net/http"
	"os"

	"manager/internal/auth"
	"manager/internal/config"
	"manager/internal/infrastructure/cache"
	cacherepo "manager/internal/infrastructure/cache/repository"
	"manager/internal/infrastructure/storage/mysql"
	"manager/internal/infrastructure/storage/mysql/repository"
	transport "manager/internal/transport/http"
	"manager/internal/usecases"
)

// @title Task Manager API
// @version 1.0
// @description very cool api.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfgEnv, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	appCfg := cfgEnv.GetAppConfig()

	db, err := mysql.NewDatabase(appCfg.Database)
	if err != nil {
		slog.Error("failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	rdb, err := cache.NewRedisClient(appCfg.Redis)
	if err != nil {
		slog.Error("failed to initialize redis client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	tx := mysql.NewTransactor(db)
	hasher := auth.NewHasher()
	tokenManager := auth.NewTokenManager(appCfg.JWT.Secret, appCfg.JWT.TTL)

	userRepo := repository.NewUserRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	historyRepo := repository.NewTaskHistoryRepository(db)
	commentRepo := repository.NewTaskCommentRepository(db)

	userCacheRepo := cacherepo.NewUserCacheRepository(userRepo, rdb, appCfg.Redis.TTL)
	taskCacheRepo := cacherepo.NewTaskCacheRepository(taskRepo, rdb, appCfg.Redis.TTL)

	userUC := usecases.NewUserUsecase(userCacheRepo, hasher, tokenManager)
	teamUC := usecases.NewTeamUsecase(teamRepo, userRepo, tx)
	taskUC := usecases.NewTaskUsecase(taskCacheRepo, teamRepo, historyRepo, commentRepo, tx)

	handler := transport.NewHandler(userUC, teamUC, taskUC)
	router := transport.NewRouter(handler, appCfg.JWT.Secret)

	serverAddr := appCfg.Server.Host + ":" + appCfg.Server.Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	slog.Info("starting http server", slog.String("address", serverAddr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed to start or crashed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
