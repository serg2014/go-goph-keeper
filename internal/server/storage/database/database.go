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

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
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

func (s *storageDB) Close() error {
	return s.db.Close()
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

func (s *storageDB) CreateSecret(ctx context.Context, userID models.UserID, req *pb.CreateSecretRequest) (*pb.CreateSecretResponse, error) {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transaction in CreateUser: %w", err)
	}
	defer tx.Rollback()

	res := &pb.CreateSecretResponse{
		Id:       req.Secret.Id,
		Conflict: false,
	}

	query := `INSERT INTO secrets (id, user_id) VALUES ($1,$2)`
	// TODO string -> uuid
	_, err = tx.ExecContext(ctx, query, req.Secret.Id, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				res.Conflict = true
				return res, nil
			}
		}
		return nil, fmt.Errorf("failed create secret: %w", err)
	}

	query = `INSERT INTO meta (user_id, secret_id, updated_at, data) 
	VALUES ($1, $2, $3, $4)`
	_, err = tx.ExecContext(ctx, query, userID, req.Secret.Id, req.Secret.Meta.UpdatedAt, req.Secret.Meta.Data)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				res.Conflict = true
				return res, nil
			}
		}
		return nil, fmt.Errorf("failed create meta: %w", err)
	}

	query = `INSERT INTO data (user_id, secret_id, updated_at, data) 
	VALUES ($1, $2, $3, $4)`
	_, err = tx.ExecContext(ctx, query, userID, req.Secret.Id, req.Secret.Data.UpdatedAt, req.Secret.Data.Data)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				res.Conflict = true
				return res, nil
			}
		}
		return nil, fmt.Errorf("failed create data: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("create secret. failed commit transaction: %w", err)
	}
	return res, nil
}

func (s *storageDB) UpdateSecret(ctx context.Context, userID models.UserID, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error) {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transaction in CreateUser: %w", err)
	}
	defer tx.Rollback()

	if req.Meta == nil && req.Data == nil {
		return nil, storage.ErrMetaAndDataEmpty
	}

	res := pb.UpdateSecretResponse{
		Id: req.Id,
	}

	if req.Meta != nil {
		query := `SELECT version
		FROM meta
		WHERE secret_id=$1 and user_id=$2
		FOR UPDATE`
		_, err := tx.ExecContext(ctx, query, req.Id, userID)
		if err != nil {
			// TODO сюда попадаем когда секрет на сервере был удален, а локально изменен
			// либо нам прислали кривой секрет(попытка взлома)
			return nil, fmt.Errorf("failed select for update meta: %w", err)
		}

		query = `UPDATE meta
		SET version=$1, updated_at=$2, data=$3
		WHERE secret_id=$4 and user_id=$5 and version=$6`
		sqlRes, err := tx.ExecContext(ctx, query,
			req.Meta.Version+1,
			req.Meta.UpdatedAt,
			req.Meta.Data,
			req.Id,
			userID,
			req.Meta.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed update meta: %w", err)
		}

		ra, err := sqlRes.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("update meta. RowsAffected: %w", err)
		}
		if ra == 0 {
			res.Conflict = true
			return &res, nil
		}

		res.Meta = &pb.UpdateResponseInfo{
			Version: req.Meta.Version + 1,
		}
	}

	if req.Data != nil {
		query := `SELECT version
		FROM data
		WHERE secret_id=$1 and user_id=$2
		FOR UPDATE`
		_, err := tx.ExecContext(ctx, query, req.Id, userID)
		if err != nil {
			return nil, fmt.Errorf("failed update data: %w", err)
		}

		query = `UPDATE data
		SET version=$1, updated_at=$2, data=$3
		WHERE secret_id=$4 and user_id=$5 and version=$6`
		sqlRes, err := tx.ExecContext(ctx, query,
			req.Data.Version+1,
			req.Data.UpdatedAt,
			req.Data.Data,
			req.Id,
			userID,
			req.Data.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed update data: %w", err)
		}

		ra, err := sqlRes.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("update data. RowsAffected: %w", err)
		}

		if ra == 0 {
			res.Conflict = true
			res.Meta = nil
			return &res, nil
		}

		res.Data = &pb.UpdateResponseInfo{
			Version: req.Data.Version + 1,
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("update secret. failed commit transaction: %w", err)
	}
	return &res, nil
}
