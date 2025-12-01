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
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	// _ "modernc.org/sqlite"
)

type TypeId int

const (
	MetaID TypeId = iota + 1
	DataID
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

	// если изменили только meta
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

func (s *storageDB) GetSecret(ctx context.Context, id int64) (*models.SecretDB, error) {
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

func (s *storageDB) DeleteSecret(ctx context.Context, id int64) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if id > 0 {
		query := `INSERT INTO deleted (secret_id, id, type_id, version) 
		SELECT secret_id, id, ?, version 
		FROM meta 
		WHERE secret_id=?`
		_, err = tx.ExecContext(ctx, query, MetaID, id, id)
		if err != nil {
			return err
		}

		query = `INSERT INTO deleted (secret_id, id, type_id, version) 
		SELECT secret_id, id, ?, version 
		FROM data 
		WHERE secret_id=?`
		_, err = tx.ExecContext(ctx, query, DataID, id, id)
		if err != nil {
			return err
		}
	}

	query := `DELETE FROM secrets WHERE id=?`
	_, err = tx.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	query = `DELETE FROM meta WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	query = `DELETE FROM data WHERE secret_id=?`
	_, err = tx.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *storageDB) GetSecretsIDsForCreate(ctx context.Context) ([]int64, error) {
	list := make([]int64, 0, 10)
	query := `SELECT s.id
	FROM secrets as s 
	WHERE s.id < 0`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()
	for rows.Next() {
		var id int64
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

func (s *storageDB) GetSecretForCreate(ctx context.Context, id int64) (*models.SecretDBCreateServer, error) {
	query := `SELECT s.id, s.type,
	m.id as meta_id, m.updated_at as meta_updated, m."data" as meta,
	d.id as data_id, d.updated_at as data_updated, d."data"
	FROM secrets as s
	JOIN meta as m ON m.secret_id = s.id
	JOIN data as d ON d.secret_id = s.id
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	item := &models.SecretDBCreateServer{}
	err := row.Scan(&item.ID, &item.Type, &item.MetaID, &item.MetaUpdated, &item.Meta, &item.DataID, &item.DataUpdated, &item.Data)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// MoveSecret обновляет локальные id(отрицательные) на id полученные с сервера(положительные)
func (s *storageDB) MoveSecret(ctx context.Context, data *pb.CreateSecretResponse) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `UPDATE secrets SET id=? WHERE id=?`
	_, err = s.db.ExecContext(ctx, query, data.Secret.Secret.ServerId, data.Secret.Secret.Id)
	if err != nil {
		return err
	}

	query = `UPDATE meta SET id=?, secret_id=?, version=0 WHERE id=?`
	_, err = s.db.ExecContext(ctx, query, data.Secret.Meta.ServerId, data.Secret.Secret.ServerId, data.Secret.Meta.Id)
	if err != nil {
		return err
	}

	query = `UPDATE data SET id=?, secret_id=?, version=0 WHERE id=?`
	_, err = s.db.ExecContext(ctx, query, data.Secret.Data.ServerId, data.Secret.Secret.ServerId, data.Secret.Data.Id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *storageDB) GetSecretsIDsForServerUpdate(ctx context.Context) ([]int64, error) {
	list := make([]int64, 0, 10)
	query := `SELECT m.secret_id
	FROM meta as m
	JOIN data as d ON d.secret_id = m.secret_id
	WHERE m.need_update = 1 or d.need_update = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	// обязательно закрываем перед возвратом функции
	defer rows.Close()

	for rows.Next() {
		var id int64
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

func (s *storageDB) GetSecretForUpdate(ctx context.Context, id int64) (*models.SecretDBUpdateServer, error) {
	query := `SELECT s.id, s.type,
	m.id as meta_id, m.updated_at as meta_updated, m.version as meta_version, m."data" as meta,
	d.id as data_id, d.updated_at as data_updated, d.version as data_version, d."data"
	FROM secrets as s
	LEFT JOIN meta as m ON m.secret_id = s.id and m.need_update = 1
	LEFT JOIN data as d ON d.secret_id = s.id and d.need_update = 1
	WHERE s.id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	item := &models.SecretDBUpdateServer{}
	var (
		metaID      sql.NullInt64
		metaUpdated sql.NullInt64
		metaVersion sql.NullInt64
		meta        []byte

		dataID      sql.NullInt64
		dataUpdated sql.NullInt64
		dataVersion sql.NullInt64
		data        []byte
	)
	err := row.Scan(&item.ID, &item.Type,
		&metaID, &metaUpdated, &metaVersion, &meta,
		&dataID, &dataUpdated, &dataVersion, &data)
	if err != nil {
		return nil, err
	}

	if metaID.Valid {
		item.Meta = &models.SecretDBUpdateServerBlock{
			ID:      metaID.Int64,
			Updated: metaUpdated.Int64,
			Version: metaVersion.Int64,
			Data:    meta,
		}
	}
	if dataID.Valid {
		item.Data = &models.SecretDBUpdateServerBlock{
			ID:      dataID.Int64,
			Updated: dataUpdated.Int64,
			Version: dataVersion.Int64,
			Data:    data,
		}
	}
	return item, nil
}

// UpdateSecretVersion обновляет версию и сбрасываею флаг need_update
func (s *storageDB) UpdateSecretVersion(ctx context.Context, data *pb.UpdateSecretResponse) error {
	// начинаем транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if data.Secret.Meta != nil {
		query := `UPDATE meta SET version=?, need_update=0 WHERE id=?`
		sqlRes, err := tx.ExecContext(ctx, query, data.Secret.Meta.Version, data.Secret.Meta.Id)
		if err != nil {
			return err
		}
		i, _ := sqlRes.RowsAffected()
		logger.Logger.Debug("update version", slog.Int64("meta_id", data.Secret.Meta.Id), slog.Bool("updated", i != 0))
	}
	if data.Secret.Data != nil {
		query := `UPDATE data SET version=?, need_update=0 WHERE id=?`
		sqlRes, err := tx.ExecContext(ctx, query, data.Secret.Data.Version, data.Secret.Data.Id)
		if err != nil {
			return err
		}
		i, _ := sqlRes.RowsAffected()
		logger.Logger.Debug("update version", slog.Int64("data_id", data.Secret.Meta.Id), slog.Bool("updated", i != 0))
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
