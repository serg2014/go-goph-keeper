package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

// TODO
func crypt(in []byte) error {
	return nil
}

// TODO
func decrypt(in []byte) error {
	return nil
}

type CryptFile struct {
	file *os.File
}

func NewCryptFile(file *os.File) *CryptFile {
	return &CryptFile{file: file}
}

func (c *CryptFile) Write(p []byte) (int, error) {
	err := crypt(p)
	if err != nil {
		return 0, err
	}
	return c.file.Write(p)
}

func (c *CryptFile) Read(p []byte) (int, error) {
	err := decrypt(p)
	if err != nil {
		return 0, err
	}
	return c.file.Read(p)
}

func (app *ClientApp) CopyFileToLocalStorage(filePath string, cryptFilePath string) (string, error) {
	fileR, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fileR.Close()

	if cryptFilePath == "" {
		cryptFilePath = path.Join(app.config.DataDir(), strconv.FormatInt(time.Now().Unix(), 10))
	}
	fileW, err := os.OpenFile(cryptFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cryptFilePath, err)
	}
	defer fileW.Close()
	cryptFile := NewCryptFile(fileW)

	_, err = io.Copy(cryptFile, fileR)
	if err != nil {
		return "", err
	}

	_, name := path.Split(cryptFilePath)
	return name, nil
}

func (app *ClientApp) DescryptFileFromLocalStorage(cryptPath string, origName string) (string, error) {
	fileR, err := os.Open(cryptPath)
	if err != nil {
		return "", err
	}
	defer fileR.Close()
	cryptFile := NewCryptFile(fileR)

	path := path.Join(app.config.TmpDirPath(), origName)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	fileW, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer fileW.Close()

	io.Copy(fileW, cryptFile)
	return absPath, nil
}

func (app *ClientApp) NewSecretFromSecretDB(secretDB *models.SecretDB) (*models.Secret, error) {
	secret := &models.Secret{
		ID:   secretDB.ID,
		Type: secretDB.Type,
		Meta: models.BlockMeta{
			Meta: make(models.Meta),
		},
	}
	err := app.transformDBToMeta(secret, secretDB)
	if err != nil {
		return nil, err
	}

	// when use in list we do not have data
	if secretDB.Data != nil {
		err = app.transformDBToData(secret, secretDB)
		if err != nil {
			return nil, fmt.Errorf("transform db.data: %w", err)
		}
	}

	return secret, nil
}

func (app *ClientApp) NewSecretDBFromSecret(secret *models.Secret) (*models.SecretDB, error) {
	secretDB := &models.SecretDB{
		ID:   secret.ID,
		Type: secret.Type,
	}

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
		// for edit secret, when file not change
		if secret.Data.FilePath.Path == "" {
			break
		}

		// TODO копировать в tmp, потом переименовать
		cryptName, err := app.CopyFileToLocalStorage(secret.Data.FilePath.Path, secret.Data.FilePath.OldPath)
		if err != nil {
			return err
		}
		// в базе храним относительные пути
		secret.Data.FilePath.Path = cryptName
		secretDB.Data, err = json.Marshal(secret.Data)
		if err != nil {
			return err
		}
	case models.SecretTypeText:
		secretDB.Data = []byte(secret.Data.Text)
	default:
		secretDB.Data, err = json.Marshal(secret.Data)
		if err != nil {
			return err
		}
	}

	if len(secretDB.Data) != 0 {
		err = crypt(secretDB.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *ClientApp) transformDBToData(secret *models.Secret, secretDB *models.SecretDB) error {
	if len(secretDB.Data) != 0 {
		err := decrypt(secretDB.Data)
		if err != nil {
			return fmt.Errorf("decrypt secret.data: %w", err)
		}
	}

	switch secret.Type {
	case models.SecretTypeFile:
		err := json.Unmarshal(secretDB.Data, &secret.Data)
		if err != nil {
			return fmt.Errorf("unmarshal secret.data: %w", err)
		}
		// в базе пути хранятся относительно DataDir
		secret.Data.FilePath.Path = path.Join(app.config.DataDir(), secret.Data.FilePath.Path)
		secret.Data.FilePath.OldPath = secret.Data.FilePath.Path
	case models.SecretTypeText:
		secret.Data.Text = string(secretDB.Data)
	default:
		err := json.Unmarshal(secretDB.Data, &secret.Data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *ClientApp) transformMetaToDB(secret *models.Secret, secretDB *models.SecretDB) error {
	var err error
	secretDB.Meta, err = json.Marshal(secret.Meta)
	if err != nil {
		return err
	}
	err = crypt(secretDB.Meta)
	if err != nil {
		return err
	}
	return nil
}

func (app *ClientApp) transformDBToMeta(secret *models.Secret, secretDB *models.SecretDB) error {
	var err error
	err = decrypt(secretDB.Meta)
	if err != nil {
		return err
	}

	err = json.Unmarshal(secretDB.Meta, &secret.Meta)
	if err != nil {
		return fmt.Errorf("unmarshal meta: %w", err)
	}

	return nil
}
