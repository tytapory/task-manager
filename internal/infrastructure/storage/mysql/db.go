package mysql

import (
	"context"
	"fmt"
	"log/slog"
	"manager/internal/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.Charset,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newSlogGormLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	slog.Info("successfully connected to mysql database (migrations are handled externally)")
	return db, nil
}

type slogGormLogger struct{}

func newSlogGormLogger() gormlogger.Interface {
	return &slogGormLogger{}
}

func (l *slogGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *slogGormLogger) Info(ctx context.Context, msg string, data ...any) {
	slog.Info(fmt.Sprintf(msg, data...))
}

func (l *slogGormLogger) Warn(ctx context.Context, msg string, data ...any) {
	slog.Warn(fmt.Sprintf(msg, data...))
}

func (l *slogGormLogger) Error(ctx context.Context, msg string, data ...any) {
	slog.Error(fmt.Sprintf(msg, data...))
}

func (l *slogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	if err != nil {
		slog.Error("gorm query error",
			"err", err,
			"elapsed", elapsed,
			"rows", rows,
			"sql", sql,
		)
		return
	}

	if elapsed > 200*time.Millisecond {
		slog.Warn("gorm slow query",
			"elapsed", elapsed,
			"rows", rows,
			"sql", sql,
		)
		return
	}

	slog.Debug("gorm query execution",
		"elapsed", elapsed,
		"rows", rows,
		"sql", sql,
	)
}
