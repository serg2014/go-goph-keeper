package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"google.golang.org/grpc"
)

const (
	MaxChunkSize = 128 * 1024 // 128kb
)

func (app *ClientApp) uploadFile(ctx context.Context, stream grpc.BidiStreamingClient[pb.CreateSecretRequest, pb.CreateSecretResponse], secret_id uuid.UUID) error {
	offset := int64(0)
	fileR, err := os.Open(app.SecretFilePath(secret_id.String()))
	if err != nil {
		return err
	}
	defer fileR.Close()

	buf, _ := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	chunkSize := MaxChunkSize
	buf.Grow(int(chunkSize))

	copyData := func(w io.Writer) (int64, error) {
		n, err := io.CopyN(w, fileR, int64(chunkSize))
		if err != nil && n <= 0 {
			if err == io.EOF {
				return n, io.EOF
			}
			return n, err
		}
		return n, nil
	}

	for range maxRetries {
		_, err = fileR.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}
		buf.Reset()
		for {
			n, err := copyData(buf)
			if err != nil && n <= 0 {
				if err == io.EOF {
					// все отправили. надо отправить серверу признак конца
					reqFine := &pb.CreateSecretRequest{}
					err = stream.Send(reqFine)
					if err != nil {
						return err
					}
					return nil
				}
			}

			req := &pb.CreateSecretRequest{
				Secret: &pb.Secret{
					Id: secret_id.String(),
				},
				File: &pb.FileUpload{
					Chunk: buf.Bytes(),
				},
			}

			err = stream.Send(req)
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
				if errors.Is(err, auth.ErrNeedRetry) {
					break // переходим к следующей попытке
				}
				// io.EOF тут невозможен в нормальной ситуации
				// io.EOF возможен на первом Send, когда сервер закрыл соединение раньше чем клиент сделал Send
				// например при проверка авторизации
				// в остальных случаях это сетевые ошибки.
				// игнорируем их, получим ошибку из Recv
				if !errors.Is(err, io.EOF) {
					return fmt.Errorf("cannot send stream request: %v", err)
				}
			}
			buf.Reset()

			// получаем подтверждение что сервер записл данные
			var res *pb.CreateSecretResponse
			res, err = stream.Recv()
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))

				if errors.Is(err, auth.ErrNeedRetry) {
					break // переходим к следующей попытке
				}
				// io.EOF тут невозможен в нормальной ситуации
				return fmt.Errorf("cannot receive stream response: %v", err)
			}
			logger.RPCLogger.Debug(fmt.Sprintf("uploadFile received response: %v", res))

			offset += n
		}
	}
	logger.RPCLogger.Debug(fmt.Sprintf("uploadFile error: %v", err))
	return err
}

func (app *ClientApp) uploadFileForUpdate(ctx context.Context, stream grpc.BidiStreamingClient[pb.UpdateSecretRequest, pb.UpdateSecretResponse], secret_id uuid.UUID) error {
	offset := int64(0)
	fileR, err := os.Open(app.SecretFilePath(secret_id.String()))
	if err != nil {
		return err
	}
	defer fileR.Close()

	buf, _ := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	chunkSize := MaxChunkSize
	buf.Grow(int(chunkSize))

	copyData := func(w io.Writer) (int64, error) {
		n, err := io.CopyN(w, fileR, int64(chunkSize))
		if err != nil && n <= 0 {
			if err == io.EOF {
				return n, io.EOF
			}
			return n, err
		}
		return n, nil
	}

	for range maxRetries {
		_, err = fileR.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}
		buf.Reset()
		for {
			n, err := copyData(buf)
			if err != nil && n <= 0 {
				if err == io.EOF {
					// все отправили. надо отправить серверу признак конца
					reqFine := &pb.UpdateSecretRequest{}
					err = stream.Send(reqFine)
					if err != nil {
						return err
					}
					return nil
				}
			}

			req := &pb.UpdateSecretRequest{
				Id: secret_id.String(),
				File: &pb.FileUpload{
					Chunk: buf.Bytes(),
				},
			}

			err = stream.Send(req)
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
				if errors.Is(err, auth.ErrNeedRetry) {
					break // переходим к следующей попытке
				}
				// io.EOF тут невозможен в нормальной ситуации
				// io.EOF возможен на первом Send, когда сервер закрыл соединение раньше чем клиент сделал Send
				// например при проверка авторизации
				// в остальных случаях это сетевые ошибки.
				// игнорируем их, получим ошибку из Recv
				if !errors.Is(err, io.EOF) {
					return fmt.Errorf("cannot send stream request: %v", err)
				}
			}
			buf.Reset()

			// получаем подтверждение что сервер записл данные
			var res *pb.UpdateSecretResponse
			res, err = stream.Recv()
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))

				if errors.Is(err, auth.ErrNeedRetry) {
					break // переходим к следующей попытке
				}
				// io.EOF тут невозможен в нормальной ситуации
				return fmt.Errorf("cannot receive stream response: %v", err)
			}
			logger.RPCLogger.Debug(fmt.Sprintf("uploadFile received response: %v", res))

			offset += n
		}
	}

	logger.RPCLogger.Debug(fmt.Sprintf("uploadFile error: %v", err))
	return err
}

func (app *ClientApp) downloadFile(ctx context.Context, stream grpc.BidiStreamingClient[pb.GetSecretsRequest, pb.GetSecretsResponse], secret_id string) error {
	tmpPath := app.SecretFilePath(secret_id) + ".tmp"
	fileW, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer fileW.Close()

	offset := int64(0)
	_, err = fileW.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}

	var chunkReader io.Reader
	for range maxRetries {
		err = stream.Send(&pb.GetSecretsRequest{
			Id: secret_id,
			File: &pb.StreamFileRequest{
				Offset: offset,
			},
		})
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
			if errors.Is(err, auth.ErrNeedRetry) {
				break // переходим к следующей попытке
			}
			// io.EOF тут невозможен в нормальной ситуации
			// io.EOF возможен на первом Send, когда сервер закрыл соединение раньше чем клиент сделал Send
			// например при проверка авторизации
			// в остальных случаях это сетевые ошибки.
			// игнорируем их, получим ошибку из Recv
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("cannot send stream request: %v", err)
			}
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))

				if errors.Is(err, auth.ErrNeedRetry) {
					break // переходим к следующей попытке
				}
				// io.EOF тут невозможен в нормальной ситуации
				return fmt.Errorf("cannot receive stream response: %v", err)
			}
			logger.RPCLogger.Debug(fmt.Sprintf("downloadFile received response: %v", resp))

			// дочитали файл до конца
			if resp.Id == "" {
				logger.Logger.Debug("read file end")
				return nil
			}

			chunkReader = bytes.NewReader(resp.File.Chunk)
			n, err := io.Copy(fileW, chunkReader)
			if err != nil {
				return err
			}
			offset += n
		}
	}
	return nil
}
