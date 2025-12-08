package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

const nonceSize = aes.BlockSize
const hmacSize = sha256.Size
const metadataSize = nonceSize + hmacSize

// TODO вынести в env
var Password = "test"

var (
	ErrFileTooSmall = errors.New("file too smal")
)

func generateKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func prepareCrypt() (cipher.AEAD, error) {
	// ключ из password, используя sha256.Sum256
	key := sha256.Sum256([]byte(Password))

	// NewCipher создает и возвращает новый cipher.Block.
	// Ключевым аргументом должен быть ключ AES, 16, 24 или 32 байта
	// для выбора AES-128, AES-192 или AES-256.
	aesblock, err := aes.NewCipher(key[:])
	if err != nil {
		logger.Logger.Error("NewCipher", slog.String("error", err.Error()))
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(aesblock)
	if err != nil {
		logger.Logger.Error("NewGCM", slog.String("error", err.Error()))
		return nil, err
	}

	return aesgcm, nil
}

func crypt(in []byte) ([]byte, error) {
	aesgcm, err := prepareCrypt()
	if err != nil {
		return nil, err
	}

	// создаём вектор инициализации
	nonce, err := generateKey(aesgcm.NonceSize())
	if err != nil {
		return nil, err
	}
	// добавляем nonce в данные, первый аргумент
	dst := aesgcm.Seal(nonce, nonce, in, nil) // зашифровываем
	return dst, nil
}

func decrypt(in []byte) ([]byte, error) {
	aesgcm, err := prepareCrypt()
	if err != nil {
		return nil, err
	}

	// достаем nonce из данных
	nonce := in[:aesgcm.NonceSize()]
	src, err := aesgcm.Open(nil, nonce, in[aesgcm.NonceSize():], nil) // расшифровываем
	if err != nil {
		logger.Logger.Error("aesgcm.Open", slog.String("error", err.Error()))
		return nil, err
	}

	return src, nil
}

func (app *ClientApp) SecretFilePath(secret_id string) string {
	return path.Join(app.config.DataDir(), secret_id)
}

func (app *ClientApp) DeleteFileFromLocalStorage(secretID uuid.UUID) error {
	return os.Remove(app.SecretFilePath(secretID.String()))
}

func (app *ClientApp) CopyFileToLocalStorage(filePath string, cryptFileName string) error {
	fileR, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fileR.Close()

	cryptFilePath := app.SecretFilePath(cryptFileName)
	fileW, err := os.OpenFile(cryptFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("%s: %w", cryptFilePath, err)
	}
	defer fileW.Close()

	// ключ из password, используя sha256.Sum256
	key := sha256.Sum256([]byte(Password))

	// Настраиваем AES в режиме CTR
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}

	// Генерируем Nonce (IV) для CTR. В CTR это начальное значение счетчика.
	// Nonce должен быть уникальным для каждой операции с данным ключом.
	nonce, err := generateKey(aes.BlockSize)
	if err != nil {
		return err
	}

	// Записываем Nonce в начало файла назначения (открыто)
	if _, err := fileW.Write(nonce); err != nil {
		return err
	}

	// Настраиваем HMAC для вычисления подписи всего *зашифрованного* потока
	mac := hmac.New(sha256.New, key[:])

	// Мы создаем "цепь" io.Writer: srcFile -> CTR шифратор -> destFile И mac writer
	// CTR шифратор будет записывать данные и в файл, и в HMAC sum.

	// MultiWriter позволяет писать одновременно в destFile и в mac (hash generator)
	// ВАЖНО: Мы начинаем считать HMAC с *зашифрованного* потока, который идет после Nonce.
	hashedWriter := io.MultiWriter(fileW, mac)

	// Создаем потоковый шифратор/дешифратор CTR
	stream := cipher.NewCTR(block, nonce)

	// Создаем Writer, который будет шифровать данные, проходящие через него
	writer := cipher.StreamWriter{S: stream, W: hashedWriter}

	// Копируем данные из источника через шифратор в назначение (потоково, чанками)
	// io.Copy использует буфер для чтения чанками и записи их через writer
	if _, err := io.Copy(writer, fileR); err != nil {
		return fmt.Errorf("error stream cipher: %w", err)
	}

	// Закрываем writer, чтобы убедиться, что все буферы сброшены
	if err := writer.Close(); err != nil {
		return fmt.Errorf("error close writer: %w", err)
	}

	// Вычисляем итоговый HMAC тег и дописываем его в конец файла
	finalHMAC := mac.Sum(nil)
	if _, err := fileW.Write(finalHMAC); err != nil {
		return fmt.Errorf("error write HMAC tag: %w", err)
	}

	// cryptFile := NewCryptFile(fileW)

	// _, err = io.Copy(cryptFile, fileR)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func (app *ClientApp) DescryptFileFromLocalStorage(cryptPath string, origName string) (string, error) {
	fileR, err := os.Open(cryptPath)
	if err != nil {
		return "", err
	}
	defer fileR.Close()

	stat, err := fileR.Stat()
	if err != nil {
		return "", err
	}
	if stat.Size() < metadataSize {
		return "", ErrFileTooSmall
	}

	// ключ из password, используя sha256.Sum256
	key := sha256.Sum256([]byte(Password))

	// 1. Читаем Nonce
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(fileR, nonce); err != nil {
		return "", fmt.Errorf("error nonce: %w", err)
	}
	// 2. Определяем длину зашифрованного тела данных и тега HMAC
	encryptedDataLen := stat.Size() - int64(metadataSize)

	// Оставшийся читатель для всего тела файла, включая HMAC в конце
	remainingReader := fileR

	// 3. Создаем Reader, который читает *только* данные для HMAC проверки
	// (без Nonce и без самого тега HMAC)
	hmacDataReader := io.LimitReader(remainingReader, encryptedDataLen)

	// 4. Вычисляем HMAC "на лету" во время чтения.
	// Здесь мы вынуждены прочитать *всю* часть данных в "никуда" (io.Discard)
	// только для того, чтобы убедиться, что они прошли через хеш-функцию HMAC.
	mac := hmac.New(sha256.New, key[:])

	// io.Copy использует внутренний буфер (чанки) для перемещения данных
	_, err = io.Copy(mac, hmacDataReader)
	if err != nil {
		return "", fmt.Errorf("error hmac: %w", err)
	}

	// 5. Читаем ожидаемый тег HMAC из конца файла (мы находимся в нужной позиции благодаря io.Copy)
	expectedHMAC := make([]byte, hmacSize)
	if _, err := io.ReadFull(remainingReader, expectedHMAC); err != nil {
		return "", fmt.Errorf("error hmac tag: %w", err)
	}

	// 6. ПРОВЕРКА АУТЕНТИЧНОСТИ: Сравниваем
	calculatedHMAC := mac.Sum(nil)
	if !hmac.Equal(calculatedHMAC, expectedHMAC) {
		return "", fmt.Errorf("hmac tag not equal")
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	stream := cipher.NewCTR(block, nonce)

	// Переходим на позицию сразу после Nonce
	if _, err := fileR.Seek(int64(nonceSize), io.SeekStart); err != nil {
		return "", fmt.Errorf("error seek: %w", err)
	}

	// Ограничиваем чтение только зашифрованными данными (без HMAC тега в конце)
	decryptionDataReader := io.LimitReader(fileR, encryptedDataLen)

	// Создаем Writer, который будет расшифровывать данные потоком
	reader := cipher.StreamReader{S: stream, R: decryptionDataReader}

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

	// 8. Копируем расшифрованные чанки из Reader'а в файл назначения
	if _, err := io.Copy(fileW, reader); err != nil {
		return "", fmt.Errorf("error write decrypt data: %w", err)
	}

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
		err := app.CopyFileToLocalStorage(secret.Data.FilePath.Path, secret.ID.String())
		if err != nil {
			return err
		}
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
		data, err := crypt(secretDB.Data)
		if err != nil {
			return err
		}
		secretDB.Data = data
	}
	return nil
}

func (app *ClientApp) transformDBToData(secret *models.Secret, secretDB *models.SecretDB) error {
	if len(secretDB.Data) != 0 {
		data, err := decrypt(secretDB.Data)
		if err != nil {
			return fmt.Errorf("decrypt secret.data: %w", err)
		}
		secretDB.Data = data
	}

	switch secret.Type {
	case models.SecretTypeFile:
		err := json.Unmarshal(secretDB.Data, &secret.Data)
		if err != nil {
			return fmt.Errorf("unmarshal secret.data: %w", err)
		}
		// TODO удалить
		secret.Data.FilePath.Path = app.SecretFilePath(secret.ID.String())
		// secret.Data.FilePath.OldPath = secret.Data.FilePath.Path
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
	data, err := crypt(secretDB.Meta)
	if err != nil {
		return err
	}
	secretDB.Meta = data
	return nil
}

func (app *ClientApp) transformDBToMeta(secret *models.Secret, secretDB *models.SecretDB) error {
	data, err := decrypt(secretDB.Meta)
	if err != nil {
		return err
	}
	secretDB.Meta = data

	err = json.Unmarshal(secretDB.Meta, &secret.Meta)
	if err != nil {
		return fmt.Errorf("unmarshal meta: %w", err)
	}

	return nil
}
