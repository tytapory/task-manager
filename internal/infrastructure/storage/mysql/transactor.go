package mysql

import (
	"context"
	"manager/internal/usecases"

	"gorm.io/gorm"
)

type gormTransactor struct {
	db *gorm.DB
}

func NewTransactor(db *gorm.DB) usecases.Transactor {
	return &gormTransactor{db: db}
}

func (t *gormTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, "tx", tx))
	})
}
