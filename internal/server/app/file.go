package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/auth"
	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrFileUploading = errors.New("fioe already uploading")
)

// Получить или создать мьютекс для файла
func (app *MyApp) getUploadFlag(secret_id string) bool {
	app.mapMutex.Lock()
	defer app.mapMutex.Unlock()

	if _, ok := app.activeUploads[secret_id]; ok {
		return false
	}
	app.activeUploads[secret_id] = struct{}{}
	return true
}

func (app *MyApp) cleanUploadFlag(secret_id string) {
	app.mapMutex.Lock()
	defer app.mapMutex.Unlock()

	delete(app.activeUploads, secret_id)
}

func (app *MyApp) SecretFilePath(userID *models.UserID, secret_id string) string {
	u := userID.String()
	// предпоследний и последний байты (uuid в hex занимает два символа)
	return path.Join(app.config.DataDir(), u[len(u)-4:len(u)-2], u[len(u)-2:], secret_id)
}

func (app *MyApp) SecretFilePathTmp(userID *models.UserID, secret_id string) string {
	return app.SecretFilePath(userID, secret_id) + ".tmp"
}

func (app *MyApp) deleteSecretFile(userID *models.UserID, secret_id string) error {
	return os.Remove(app.SecretFilePath(userID, secret_id))
}

func (app *MyApp) CreateFileSecret(ctx context.Context, stream grpc.BidiStreamingServer[pb.CreateSecretRequest, pb.CreateSecretResponse], req *pb.CreateSecretRequest) error {
	// если файл уже загружается выходим
	if !app.getUploadFlag(req.Secret.Id) {
		return ErrFileUploading
	}
	defer app.cleanUploadFlag(req.Secret.Id)

	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}

	err = app.uploadFile(ctx, stream, req, userID)
	if err != nil {
		return err
	}
	tmpPath := app.SecretFilePathTmp(userID, req.Secret.Id)
	defer os.Remove(tmpPath)

	// в следующем чанке идут данные секрета связанные с файлом
	req, err = stream.Recv()
	if err != nil {
		return err
	}

	logger.Logger.Debug(fmt.Sprintf("got req: %+v", req.Secret))

	res, err := app.CreateSecret(ctx, req)
	if err != nil {
		return err
	}

	// переименовать файл
	if !res.Conflict {
		// TODO мы загружали напрасно, надо сделать проверку. проверить есть ли в базе токой secret_id
		err = os.Rename(tmpPath, app.SecretFilePath(userID, req.Secret.Id))
		if err != nil {
			return err
		}
	}

	err = stream.Send(res)
	if err != nil {
		code := codes.Unknown
		return status.Errorf(code, "cannot send stream response: %v", err)
	}

	return nil
}

func (app *MyApp) uploadFile(
	ctx context.Context,
	stream grpc.BidiStreamingServer[pb.CreateSecretRequest, pb.CreateSecretResponse],
	req *pb.CreateSecretRequest,
	userID *models.UserID,
) error {

	secret_id := req.Secret.Id
	tmpPath := app.SecretFilePathTmp(userID, secret_id)
	// создать директории
	dir, _ := path.Split(tmpPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	fileW, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer fileW.Close()

	var chunkReader io.Reader
	// первый чанк мы прочитали в родителькой ф-ции
	for {
		logger.Logger.Debug("read file chunk")
		chunkReader = bytes.NewReader(req.File.Chunk)
		_, err = io.Copy(fileW, chunkReader)
		if err != nil {
			return err
		}
		// отправляем сообщение что приняли чанк
		err = stream.Send(&pb.CreateSecretResponse{})
		if err != nil {
			return err
		}

		req, err = stream.Recv()
		if err != nil {
			return err
		}

		// дочитали файл до конца
		if req.Secret == nil {
			logger.Logger.Debug("read file end")
			break
		}
	}
	return nil
}

func (app *MyApp) UpdateFileSecret(ctx context.Context, stream grpc.BidiStreamingServer[pb.UpdateSecretRequest, pb.UpdateSecretResponse], req *pb.UpdateSecretRequest) error {
	// если файл уже загружается выходим
	if !app.getUploadFlag(req.Id) {
		return ErrFileUploading
	}
	defer app.cleanUploadFlag(req.Id)

	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}

	// загружаем файл и сохраняем с именем .tmp
	err = app.uploadFileForUpdate(ctx, stream, req, userID)
	if err != nil {
		return err
	}
	tmpPath := app.SecretFilePathTmp(userID, req.Id)
	defer os.Remove(tmpPath)

	// в следующем чанке идут данные секрета связанные с файлом
	req, err = stream.Recv()
	if err != nil {
		return err
	}

	logger.Logger.Debug(fmt.Sprintf("got req: %+v", req))

	res, err := app.UpdateSecret(ctx, req)
	if err != nil {
		return err
	}

	// переименовать файл
	if !res.Conflict {
		// TODO мы загружали напрасно, надо сделать проверку. проверить есть ли в базе токой secret_id
		err = os.Rename(tmpPath, app.SecretFilePath(userID, req.Id))
		if err != nil {
			return err
		}
	}

	err = stream.Send(res)
	if err != nil {
		code := codes.Unknown
		return status.Errorf(code, "cannot send stream response: %v", err)
	}

	return nil
}

func (app *MyApp) uploadFileForUpdate(
	ctx context.Context,
	stream grpc.BidiStreamingServer[pb.UpdateSecretRequest, pb.UpdateSecretResponse],
	req *pb.UpdateSecretRequest,
	userID *models.UserID,
) error {

	secret_id := req.Id
	tmpPath := app.SecretFilePathTmp(userID, secret_id)
	// создать директории
	dir, _ := path.Split(tmpPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	fileW, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer fileW.Close()

	var chunkReader io.Reader
	// первый чанк мы прочитали в родителькой ф-ции
	for {
		logger.Logger.Debug("read file chunk")
		chunkReader = bytes.NewReader(req.File.Chunk)
		_, err = io.Copy(fileW, chunkReader)
		if err != nil {
			return err
		}
		// отправляем сообщение что приняли чанк
		err = stream.Send(&pb.UpdateSecretResponse{})
		if err != nil {
			return err
		}

		req, err = stream.Recv()
		if err != nil {
			return err
		}

		// дочитали файл до конца
		if req.Id == "" {
			logger.Logger.Debug("read file end")
			break
		}
	}
	return nil
}
