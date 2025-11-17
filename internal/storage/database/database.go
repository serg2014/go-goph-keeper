package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/google/uuid"
	"github.com/serg2014/go-goph-keeper/internal/logger"
	"github.com/serg2014/go-goph-keeper/internal/models"
	"github.com/serg2014/go-goph-keeper/internal/storage"
)

type storageDB struct {
	db *sql.DB
}

func NewStorageDB(ctx context.Context, dsn string) (storage.Storager, error) {
	// dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
	//  `localhost`, `video`, `XXXXXXXX`, `video`)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// проверяем подключение к бд
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %v", err)
	}
	logger.Logger.Info("Connected to db")

	// миграции
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	// TODO file://migrations путь задается относительно cwd
	// предполагается что запуск бинаря происходит в корне репозитория
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		dsn,
		driver,
	)
	if err != nil {
		return nil, err
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Logger.Error("failed to apply migrations", err)
		return nil, err
	}

	return &storageDB{db: db}, nil
}

func (s *storageDB) CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error) {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transaction in CreateUser: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO users (login, hash) VALUES($1, $2) RETURNING user_id`
	row := tx.QueryRowContext(ctx, query, login, passwordHash)
	var userID uuid.UUID
	err = row.Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, storage.ErrUserExists
			}
		}
		return nil, fmt.Errorf("failed CreateUser. can not insert users: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed commit transaction: %w", err)
	}
	muid := models.UserID(userID)
	return &muid, nil
}
