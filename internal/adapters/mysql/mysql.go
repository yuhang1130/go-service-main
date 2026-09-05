package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
	driver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Database struct {
	gorm *gorm.DB
	sql  *sql.DB
}

func Open(ctx context.Context, cfg config.MySQL) (*Database, error) {
	database, err := gorm.Open(driver.Open(cfg.DSN), &gorm.Config{
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
		TranslateError:         true,
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, err
	}
	connection, err := database.DB()
	if err != nil {
		return nil, err
	}
	connection.SetMaxOpenConns(cfg.MaxOpenConns)
	connection.SetMaxIdleConns(cfg.MaxIdleConns)
	connection.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	connection.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	result := &Database{gorm: database, sql: connection}
	if err := result.Ping(ctx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return result, nil
}

// NormalizeError translates adapter-specific constraint errors into the small
// persistence error vocabulary shared with application services.
func NormalizeError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return persistence.Conflict(err)
	}
	return err
}

func (d *Database) GORM() *gorm.DB { return d.gorm }

func (d *Database) Ping(ctx context.Context) error { return d.sql.PingContext(ctx) }

func (d *Database) Close() error { return d.sql.Close() }

type transactionKey struct{}

type Transactor struct{ database *gorm.DB }

func NewTransactor(database *gorm.DB) *Transactor { return &Transactor{database: database} }

func (t *Transactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return t.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionKey{}, tx))
	})
}

func FromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(transactionKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}
