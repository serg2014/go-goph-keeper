package app

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

const (
	MaxChunkSizeBytes = 1000
)

// TODO
func (app *ClientApp) crypt(in []byte) error {
	return nil
}

// TODO
func (app *ClientApp) decrypt(in []byte) error {
	return nil
}

func (app *ClientApp) NewSecretFromSecretDB(secretDB *models.SecretDB) (*models.Secret, error) {
	secret := &models.Secret{
		ID:   secretDB.ID,
		Type: secretDB.Type,
		Meta: make(models.Meta),
	}
	err := app.transformDBToMeta(secret, secretDB)
	if err != nil {
		return nil, err
	}
	secret.Name = secret.Meta[models.MetaKeyName]
	delete(secret.Meta, models.MetaKeyName)

	if secretDB.Data != nil {
		err = app.transformDBToData(secret, secretDB)
		if err != nil {
			return nil, err
		}
	}

	return secret, nil
}

func (app *ClientApp) NewSecretDBFromSecret(secret *models.Secret) (*models.SecretDB, error) {
	secretDB := &models.SecretDB{
		ID:   secret.ID,
		Type: secret.Type,
	}
	secret.Meta[models.MetaKeyName] = secret.Name
	err := app.transformMetaToDB(secret, secretDB)
	if err != nil {
		return nil, err
	}

	err = app.transformDataToDB(secret, secretDB)
	if err != nil {
		return nil, err
	}

	return secretDB, nil
}

func (app *ClientApp) transformDataToDB(secret *models.Secret, secretDB *models.SecretDB) error {
	var err error
	switch secret.Type {
	case models.SecretTypeFile:
		if secret.Data.File.Path == "" {
			break
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		// read file
		file, err := os.Open(path.Join(cwd, secret.Data.File.Path))
		if err != nil {
			return err
		}
		defer file.Close()

		chunk := make([]byte, MaxChunkSizeBytes)
		off := 0
		work := true
		for work {
			n, err := file.ReadAt(chunk, int64(off))
			if err != nil {
				if err == io.EOF {
					work = false
				} else {
					return err
				}
			}
			off += n
			secretDB.Data = append(secretDB.Data, chunk[:n]...)
		}

	case models.SecretTypeText:
		secretDB.Data = []byte(secret.Data.Text)
	default:
		secretDB.Data, err = json.Marshal(secret.Data)
	}

	if err != nil {
		return err
	}

	if len(secretDB.Data) != 0 {
		err = app.crypt(secretDB.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *ClientApp) transformDBToData(secret *models.Secret, secretDB *models.SecretDB) error {
	err := app.decrypt(secretDB.Data)
	if err != nil {
		return err
	}

	switch secret.Type {
	case models.SecretTypeFile:
		secret.Data.File.Data = secretDB.Data
	case models.SecretTypeText:
		secret.Data.Text = string(secretDB.Data)
	default:
		err = json.Unmarshal(secretDB.Data, &secret.Data)
	}

	if err != nil {
		return err
	}

	return nil
}

func (app *ClientApp) transformMetaToDB(secret *models.Secret, secretDB *models.SecretDB) error {
	var err error
	secretDB.Meta, err = json.Marshal(secret.Meta)
	if err != nil {
		return err
	}
	err = app.crypt(secretDB.Meta)
	if err != nil {
		return err
	}
	return nil
}

func (app *ClientApp) transformDBToMeta(secret *models.Secret, secretDB *models.SecretDB) error {
	var err error
	err = app.decrypt(secretDB.Meta)
	if err != nil {
		return err
	}

	err = json.Unmarshal(secretDB.Meta, &secret.Meta)
	if err != nil {
		return err
	}

	return nil
}
