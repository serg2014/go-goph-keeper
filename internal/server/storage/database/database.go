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

	res := pb.CreateSecretResponse{
		Secret: &pb.SecretLite{
			Secret: &pb.SecretDataLite{
				Id: req.Secret.Id,
			},
			Meta: &pb.SecretDataLite{
				Id: req.Secret.Meta.Id,
			},
			Data: &pb.SecretDataLite{
				Id: req.Secret.Data.Id,
			},
		},
	}

	query := `INSERT INTO secrets (user_id) VALUES ($1) RETURNING id`
	row := tx.QueryRowContext(ctx, query, userID)
	err = row.Scan(&res.Secret.Secret.ServerId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, storage.ErrSecretExists
			}
		}
		return nil, fmt.Errorf("failed create secret: %w", err)
	}

	query = `INSERT INTO meta (user_id, secret_id, updated_at, data) 
	VALUES ($1, $2, $3, $4) RETURNING id`
	row = tx.QueryRowContext(ctx, query, userID, res.Secret.Secret.ServerId, req.Secret.Meta.UpdatedAt, req.Secret.Meta.Data)
	err = row.Scan(&res.Secret.Meta.ServerId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, storage.ErrMetaExists
			}
		}
		return nil, fmt.Errorf("failed create meta: %w", err)
	}

	query = `INSERT INTO data (user_id, secret_id, updated_at, data) 
	VALUES ($1, $2, $3, $4) RETURNING id`
	row = tx.QueryRowContext(ctx, query, userID, res.Secret.Secret.ServerId, req.Secret.Data.UpdatedAt, req.Secret.Data.Data)
	err = row.Scan(&res.Secret.Data.ServerId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, storage.ErrDataExists
			}
		}
		return nil, fmt.Errorf("failed create data: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("create secret. failed commit transaction: %w", err)
	}
	return &res, nil
}

func (s *storageDB) UpdateSecret(ctx context.Context, userID models.UserID, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error) {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transaction in CreateUser: %w", err)
	}
	defer tx.Rollback()

	res := pb.UpdateSecretResponse{
		Secret: &pb.SecretLiteUpdate{
			Id: req.Secret.Id,
		},
	}

	if req.Secret.Meta != nil {
		res.Secret.Meta = &pb.SecretDataLiteUpdate{
			Id: req.Secret.Meta.Id,
		}

		query := `SELECT version
		FROM meta
		WHERE id=$1 and user_id=$2
		FOR UPDATE`
		row := tx.QueryRowContext(ctx, query, req.Secret.Meta.Id, userID)
		err := row.Scan(&res.Secret.Meta.Version)
		if err != nil {
			return nil, fmt.Errorf("failed select for update meta: %w", err)
		}

		query = `UPDATE meta
		SET version=$1, updated_at=$2, data=$3
		WHERE id=$4 and user_id=$5 and version=$6`
		sqlRes, err := tx.ExecContext(ctx, query,
			req.Secret.Meta.Version+1,
			req.Secret.Meta.UpdatedAt,
			req.Secret.Meta.Data,
			req.Secret.Meta.Id,
			userID,
			req.Secret.Meta.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed update meta: %w", err)
		}

		ra, err := sqlRes.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("update meta. RowsAffected: %w", err)
		}
		if ra == 0 {
			res.Secret.Meta.Conflict = true
			return &res, nil
		}
	}

	if req.Secret.Data != nil {
		res.Secret.Data = &pb.SecretDataLiteUpdate{
			Id: req.Secret.Data.Id,
		}

		query := `SELECT version
		FROM data
		WHERE id=$1 and user_id=$2
		FOR UPDATE`
		row := tx.QueryRowContext(ctx, query, req.Secret.Data.Id, userID)
		err := row.Scan(&res.Secret.Data.Version)
		if err != nil {
			return nil, fmt.Errorf("failed update meta: %w", err)
		}

		query = `UPDATE data
		SET version=$1, updated_at=$2, data=$3
		WHERE id=$4 and user_id=$5 and version=$6`
		sqlRes, err := tx.ExecContext(ctx, query,
			req.Secret.Data.Version+1,
			req.Secret.Data.UpdatedAt,
			req.Secret.Data.Data,
			req.Secret.Data.Id,
			userID,
			req.Secret.Data.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed update data: %w", err)
		}

		ra, err := sqlRes.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("update data. RowsAffected: %w", err)
		}
		if ra != 0 {
			res.Secret.Data.Version++
		} else {
			res.Secret.Data.Conflict = true
			return &res, nil
		}
	}

	if res.Secret.Meta != nil {
		res.Secret.Meta.Version++
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("update secret. failed commit transaction: %w", err)
	}
	return &res, nil
}
