package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
	"github.com/serg2014/go-goph-keeper/internal/server/storage"
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
		"file://migrations/server",
		dsn,
		driver,
	)
	if err != nil {
		return nil, err
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Logger.Error("failed to apply migrations", slog.String("error", err.Error()))
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
	var userID models.UserID
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
	return &userID, nil
}

func (s *storageDB) GetUser(ctx context.Context, login, passwordHash string) (*models.UserID, error) {
	query := `SELECT user_id FROM users WHERE login=$1 AND hash=$2`
	row := s.db.QueryRowContext(ctx, query, login, passwordHash)
	var userID models.UserID
	err := row.Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUserOrPassword
		}
		return nil, fmt.Errorf("failed GetUser. can not select: %w", err)
	}
	return &userID, nil
}
