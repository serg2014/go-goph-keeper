package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/sqlite3"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	// _ "modernc.org/sqlite"
)

type Action int

const (
	CreateAction Action = iota
	UpdateAction
	DeleteAction
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

	if len(secretDB.Meta) == 0 && len(secretDB.Data) == 0 {
		return ErrMetaAndDataEmpty
	}

	if len(secretDB.Meta) != 0 {
		query := `UPDATE meta SET data=?, updated_at=strftime('%s', 'now'), need_update=1 WHERE secret_id=?`
		_, err := tx.ExecContext(ctx, query, secretDB.Meta, secretDB.ID.String())
		if err != nil {
			return err
		}
	}

	if len(secretDB.Data) != 0 {
		query := `UPDATE data SET data=?, updated_at=strftime('%s', 'now'), need_update=1 WHERE secret_id=?`
		_, err := tx.ExecContext(ctx, query, secretDB.Data, secretDB.ID.String())

		if err != nil {
			return err
		}
	}

	query := `INSERT INTO actions (secret_id, action_type) VALUES(?,?)
	ON CONFLICT (secret_id) DO NOTHING`
	_, err = tx.ExecContext(ctx, query, secretDB.ID.String(), UpdateAction)
	if err != nil {
		return nil
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

	query := `INSERT INTO secrets (id, type) VALUES(?, ?)`
	_, err = tx.ExecContext(ctx, query, secretDB.ID.String(), secretDB.Type)
	if err != nil {
		return err
	}

	query = `INSERT INTO meta (secret_id, data) VALUES(?,?)`
	_, err = tx.ExecContext(ctx, query, secretDB.ID.String(), secretDB.Meta)
	if err != nil {
		return nil
	}

	query = `INSERT INTO data (secret_id, data) VALUES(?,?)`
	_, err = tx.ExecContext(ctx, query, secretDB.ID.String(), secretDB.Data)
	if err != nil {
		return err
	}

	query = `INSERT INTO actions (secret_id, action_type) VALUES(?,?)`
	_, err = tx.ExecContext(ctx, query, secretDB.ID.String(), CreateAction)
	if err != nil {
		return nil
	}

	return tx.Commit()
}

func (s *storageDB) SecretsList(ctx context.Context) ([]models.SecretDB, error) {
	list := make([]models.SecretDB, 0)
	query := `SELECT s.id, s.type, m."data", a.conflict 
	FROM secrets as s 
	JOIN meta as m ON m.secret_id = s.id
	LEFT JOIN actions as a ON a.secret_id = s.id
	WHERE a.action_type isNull or a.action_type != ?`
	rows, err := s.db.QueryContext(ctx, query, DeleteAction)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()
	for rows.Next() {
		var item models.SecretDB
		var conflict sql.NullBool
		err = rows.Scan(&item.ID, &item.Type, &item.Meta, &conflict)
		if err != nil {
			return nil, err
		}
		if conflict.Valid {
			item.Conflict = conflict.Bool
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

func (s *storageDB) GetSecret(ctx context.Context, secret_id uuid.UUID) (*models.SecretDB, error) {
	query := `SELECT s.id, s.type, m."data" as meta, d.data
	FROM secrets as s 
	JOIN meta as m ON m.secret_id = s.id
	JOIN data as d ON d.secret_id = s.id
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, secret_id.String())
	secretDB := &models.SecretDB{}
	err := row.Scan(&secretDB.ID, &secretDB.Type, &secretDB.Meta, &secretDB.Data)
	if err != nil {
		return nil, err
	}
	return secretDB, nil
}

func (s *storageDB) DeleteSecret(ctx context.Context, secret_id uuid.UUID) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `SELECT version FROM meta WHERE secret_id =?`
	row := tx.QueryRowContext(ctx, query, secret_id.String())
	var version sql.NullInt64
	err = row.Scan(&version)
	if err != nil {
		return err
	}

	// если уже синкали тогда делаем вставку
	if version.Valid {
		query = `INSERT INTO actions (secret_id, action_type) VALUES(?,?)
		ON CONFLICT (secret_id) DO UPDATE
		SET action_type = EXCLUDED.action_type`
		_, err = tx.ExecContext(ctx, query, secret_id.String(), DeleteAction)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	// если не синкали то можно удалять
	query = `DELETE FROM actions WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String(), DeleteAction)
	if err != nil {
		return nil
	}

	query = `DELETE FROM secrets WHERE id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	query = `DELETE FROM meta WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	query = `DELETE FROM data WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// For sync
func (s *storageDB) GetSecretsIDsForCreate(ctx context.Context) ([]uuid.UUID, error) {
	query := `SELECT secret_id
	FROM actions 
	WHERE action_type = ? and conflict = 0`
	rows, err := s.db.QueryContext(ctx, query, CreateAction)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()

	list := make([]uuid.UUID, 0, 10)
	for rows.Next() {
		var id uuid.UUID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		list = append(list, id)
	}
	// проверяем на ошибки
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return list, nil

}

func (s *storageDB) GetSecretForCreate(ctx context.Context, secret_id uuid.UUID) (*models.SecretDBCreateServer, error) {
	query := `SELECT s.id, s.type,
	m.updated_at as meta_updated, m."data" as meta,
	d.updated_at as data_updated, d."data"
	FROM secrets as s
	JOIN meta as m ON m.secret_id = s.id
	JOIN data as d ON d.secret_id = s.id
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, secret_id.String())
	item := &models.SecretDBCreateServer{}
	err := row.Scan(&item.ID, &item.Type, &item.MetaUpdated, &item.Meta, &item.DataUpdated, &item.Data)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateSecretVersionAfterCreate выставляет версию и чистит таблицу actions
func (s *storageDB) UpdateSecretVersionAfterCreate(ctx context.Context, secret_id uuid.UUID, conflict bool) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if conflict {
		logger.Logger.Debug("after create: update actions on conflict")
		query := `UPDATE actions SET conflict=1 WHERE secret_id=?`
		_, err = tx.ExecContext(ctx, query, secret_id.String())
		if err != nil {
			return err
		}
		return nil
	}

	query := `UPDATE meta SET version=0, need_update=0 WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	query = `UPDATE data SET version=0, need_update=0 WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	query = `DELETE FROM actions WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, secret_id.String())
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *storageDB) GetSecretsIDsForServerUpdate(ctx context.Context) ([]uuid.UUID, error) {
	query := `SELECT secret_id
	FROM actions 
	WHERE action_type = ? and conflict = 0`
	rows, err := s.db.QueryContext(ctx, query, UpdateAction)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()

	list := make([]uuid.UUID, 0, 10)
	for rows.Next() {
		var id uuid.UUID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		list = append(list, id)
	}
	// проверяем на ошибки
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *storageDB) GetSecretForUpdate(ctx context.Context, secret_id uuid.UUID) (*models.SecretDBUpdateServer, error) {
	query := `SELECT s.id, s.type,
	m.updated_at as meta_updated, m.version as meta_version, m."data" as meta,
	d.updated_at as data_updated, d.version as data_version, d."data"
	FROM secrets as s
	LEFT JOIN meta as m ON m.secret_id = s.id and m.need_update = 1
	LEFT JOIN data as d ON d.secret_id = s.id and d.need_update = 1
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, secret_id)
	item := &models.SecretDBUpdateServer{}
	var (
		metaUpdated sql.NullInt64
		metaVersion sql.NullInt64
		meta        []byte

		dataUpdated sql.NullInt64
		dataVersion sql.NullInt64
		data        []byte
	)
	err := row.Scan(&item.ID, &item.Type,
		&metaUpdated, &metaVersion, &meta,
		&dataUpdated, &dataVersion, &data)
	if err != nil {
		return nil, err
	}

	if !metaUpdated.Valid && !dataUpdated.Valid {
		return nil, ErrMetaAndDataEmpty
	}

	if metaUpdated.Valid {
		item.Meta = &models.SecretDBUpdateServerBlock{
			Updated: metaUpdated.Int64,
			Version: metaVersion.Int64,
			Data:    meta,
		}
	}
	if dataUpdated.Valid {
		item.Data = &models.SecretDBUpdateServerBlock{
			Updated: dataUpdated.Int64,
			Version: dataVersion.Int64,
			Data:    data,
		}
	}
	return item, nil
}

// UpdateSecretVersionAfterUpdate обновляет версию и сбрасываею флаг need_update
func (s *storageDB) UpdateSecretVersionAfterUpdate(ctx context.Context, resp *pb.UpdateSecretResponse) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if resp.Conflict {
		logger.Logger.Debug("after update: update actions on conflict")
		query := `UPDATE actions SET conflict=1 WHERE secret_id=?`
		_, err = tx.ExecContext(ctx, query, resp.Id)
		if err != nil {
			return err
		}
		return nil
	}

	if resp.Meta != nil {
		logger.Logger.Debug("after update: update meta")
		query := `UPDATE meta SET version=?, need_update=0 WHERE secret_id=?`
		sqlRes, err := tx.ExecContext(ctx, query, resp.Meta.Version, resp.Id)
		if err != nil {
			return err
		}
		i, _ := sqlRes.RowsAffected()
		logger.Logger.Debug("update meta version", slog.String("secret_id", resp.Id), slog.Bool("updated", i != 0))
	}
	if resp.Data != nil {
		logger.Logger.Debug("after update: update data")
		query := `UPDATE data SET version=?, need_update=0 WHERE secret_id=?`
		sqlRes, err := tx.ExecContext(ctx, query, resp.Data.Version, resp.Id)
		if err != nil {
			return err
		}
		i, _ := sqlRes.RowsAffected()
		logger.Logger.Debug("update data version", slog.String("secret_id", resp.Id), slog.Bool("updated", i != 0))
	}

	logger.Logger.Debug("after update: delete actions")
	query := `DELETE FROM actions WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, resp.Id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// func (s *storageDB) GetSecretsForCreate(ctx context.Context) ([]*models.SecretDBCreateServer, error) {
// 	list := make([]*models.SecretDBCreateServer, 0, 10)
// 	query := `SELECT s.id, s.type,
// 	m.id as meta_id, m.updated_at as meta_updated, m."data" as meta,
// 	d.id as data_id, d.updated_at as data_updated, d."data"
// 	FROM secrets as s
// 	JOIN meta as m ON m.secret_id = s.id
// 	JOIN data as d ON d.secret_id = s.id
// 	WHERE s.id < 0
// 	LIMIT 10`
// 	rows, err := s.db.QueryContext(ctx, query)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// обязательно закрываем перед возвратом функции
// 	defer rows.Close()
// 	for rows.Next() {
// 		var item models.SecretDBCreateServer
// 		err = rows.Scan(&item.ID, &item.Type, &item.MetaID, &item.MetaUpdated, &item.Meta, &item.DataID, &item.DataUpdated, &item.Data)
// 		if err != nil {
// 			return nil, err
// 		}
// 		list = append(list, &item)
// 	}
// 	// проверяем на ошибки
// 	err = rows.Err()
// 	if err != nil {
// 		return nil, err
// 	}
// 	return list, nil
// }
