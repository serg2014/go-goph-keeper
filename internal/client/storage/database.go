package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/sqlite3"
	_ "github.com/golang-migrate/migrate/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	// _ "modernc.org/sqlite"
)

type storageDB struct {
	db *sql.DB
}

func NewStorageDB(ctx context.Context, path string) (Storager, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// проверяем подключение к бд
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %v", err)
	}

	// миграции
	// driver, err := postgres.WithInstance(db, &postgres.Config{})
	// if err != nil {
	// 	return nil, err
	// }

	// TODO file://migrations путь задается относительно cwd
	// предполагается что запуск бинаря происходит в корне репозитория
	m, err := migrate.New(
		"file://migrations/client",
		fmt.Sprintf("sqlite3://%s", path),
	)
	if err != nil {
		return nil, err
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		// TODO тут корка
		logger.Logger.Error("failed to apply migrations", slog.String("error", err.Error()))
		return nil, err
	}
	return &storageDB{db: db}, nil
}

func (s *storageDB) Close() error {
	return s.db.Close()
}

func (s *storageDB) UpdateSecret(ctx context.Context, secretDB *models.SecretDB) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	//strftime('%s', '2025-11-23 17:27:00')
	//t := time.Now

	query := `UPDATE meta SET data=?, updated_at=strftime('%s', 'now'), need_update=?
	WHERE secret_id=?`
	need_update := 0
	if secretDB.ID > 0 {
		need_update = 1
	}
	_, err = s.db.ExecContext(ctx, query, secretDB.Meta, need_update, secretDB.ID)
	if err != nil {
		return err
	}

	if len(secretDB.Data) != 0 {
		query = `UPDATE data SET data=?, updated_at=strftime('%s', 'now'), need_update=? 
		WHERE secret_id=?`
		_, err = s.db.ExecContext(ctx, query, secretDB.Data, need_update, secretDB.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
func (s *storageDB) AddSecret(ctx context.Context, secretDB *models.SecretDB) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `SELECT min(id) FROM secrets`
	row := tx.QueryRowContext(ctx, query)
	var secretID sql.NullInt64
	err = row.Scan(&secretID)
	if err != nil {
		return err
	}
	// null
	if !secretID.Valid || secretID.Int64 > 0 {
		secretID.Int64 = 0
	}
	secretID.Int64--
	query = `INSERT INTO secrets (id, type) VALUES(?, ?)`
	_, err = tx.ExecContext(ctx, query, secretID.Int64, secretDB.Type)
	if err != nil {
		return err
	}

	query = `SELECT min(id) FROM meta`
	row = tx.QueryRowContext(ctx, query)
	var metaID sql.NullInt64
	err = row.Scan(&metaID)
	if err != nil {
		return err
	}
	// null
	if !metaID.Valid || metaID.Int64 > 0 {
		metaID.Int64 = 0
	}
	metaID.Int64--

	query = `INSERT INTO meta (id, secret_id, data) VALUES(?,?,?)`
	_, err = tx.ExecContext(ctx, query, metaID.Int64, secretID.Int64, secretDB.Meta)
	if err != nil {
		return nil
	}

	query = `SELECT min(id) FROM data`
	row = tx.QueryRowContext(ctx, query)
	var dataID sql.NullInt64
	err = row.Scan(&dataID)
	if err != nil {
		return err
	}
	// null
	if !dataID.Valid || dataID.Int64 > 0 {
		dataID.Int64 = 0
	}
	dataID.Int64--

	query = `INSERT INTO data (id, secret_id, data) VALUES(?,?,?)`
	_, err = tx.ExecContext(ctx, query, dataID.Int64, secretID.Int64, secretDB.Data)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *storageDB) SecretsList(ctx context.Context) ([]models.SecretDB, error) {
	list := make([]models.SecretDB, 0)
	query := `SELECT s.id, s.type, m."data" 
	FROM secrets as s 
	JOIN meta as m ON m.secret_id = s.id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()
	for rows.Next() {
		var item models.SecretDB
		err = rows.Scan(&item.ID, &item.Type, &item.Meta)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	// проверяем на ошибки
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *storageDB) GetSecret(ctx context.Context, id int) (*models.SecretDB, error) {
	// TODO не читать блоб для бинаря
	query := `SELECT s.id, s.type, m."data" as meta, d.data
	FROM secrets as s 
	JOIN meta as m ON m.secret_id = s.id
	JOIN data as d ON d.secret_id = s.id
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	secretDB := &models.SecretDB{}
	err := row.Scan(&secretDB.ID, &secretDB.Type, &secretDB.Meta, &secretDB.Data)
	if err != nil {
		return nil, err
	}
	return secretDB, nil
}
