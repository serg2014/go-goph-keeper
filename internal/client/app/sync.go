package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"google.golang.org/grpc"
)

type SyncStatus struct {
	Remote     Status
	Local      Status
	Conflicted []uuid.UUID
}

type Status struct {
	Added   int
	Updated int
	Deleted int
}

func (app *ClientApp) Sync(ctx context.Context) (*SyncStatus, []error) {
	/*
		* 1. Удаляем секреты на сервере
		* 2. Создаем секреты на сервере (все записи с отрицательными ключами)
		* 3. Обновляем секреты на сервере
		4. Получаем обновления с сервера и обновляем секреты локально
		5. Получаем обновления с сервера и удаляем секреты локально
		6. Получаем обновления с сервера и создаем секреты локально
		7. Решаем конфликты. Просто перезатираем значениями из сервера
	*/
	syncStatus := &SyncStatus{
		Conflicted: make([]uuid.UUID, 0),
	}
	errorList := make([]error, 0)

	err := app.syncCreateSecretOnServer(ctx, syncStatus)
	errorList = append(errorList, err)

	err = app.syncUpdateSecretOnServer(ctx, syncStatus)
	errorList = append(errorList, err)

	err = app.syncDeleteSecretOnServer(ctx, syncStatus)
	errorList = append(errorList, err)

	err = app.syncFromServer(ctx, syncStatus)
	errorList = append(errorList, err)
	// syncStatus содержит частично обновленные данные
	return syncStatus, errorList
}

// Create
func (app *ClientApp) syncCreateSecretOnServer(ctx context.Context, syncStatus *SyncStatus) error {
	list, err := app.store.GetSecretsIDsForCreate(ctx)
	if err != nil {
		return nil
	}

	// Устанавливаем соединение стрима
	stream, err := app.grpcKeep.CreateSecrets(ctx)
	if err != nil {
		return err
	}
	for _, id := range list {
		// получить данные по секрету
		secret, err := app.store.GetSecretForCreate(ctx, id)
		if err != nil {
			return err
		}
		resp, err := app.createSecretsWithRetry(ctx, stream, secret)
		if err != nil {
			return err
		}
		// TODO
		logger.Logger.Info(fmt.Sprintf("%+v", resp))
		if resp == nil {
			return errors.New("resp nil")
		}

		id, err := uuid.Parse(resp.Id)
		if err != nil {
			return fmt.Errorf("can not parse uuid: %w", err)
		}

		if resp.Conflict {
			syncStatus.Conflicted = append(syncStatus.Conflicted, id)
		} else {
			syncStatus.Remote.Added++
		}
		err = app.store.UpdateSecretVersionAfterCreate(ctx, id, resp.Conflict)
		if err != nil {
			return err
		}
	}
	stream.CloseSend()
	return nil
}
func (app *ClientApp) createSecretsWithRetry(ctx context.Context, stream grpc.BidiStreamingClient[pb.CreateSecretRequest, pb.CreateSecretResponse], secret *models.SecretDBCreateServer) (*pb.CreateSecretResponse, error) {
	var err error
	for range maxRetries {
		req := &pb.CreateSecretRequest{
			Secret: &pb.Secret{
				Id:   secret.ID.String(),
				Type: int32(secret.Type),
				Meta: &pb.SecretData{
					UpdatedAt: secret.MetaUpdated,
					Data:      secret.Meta,
				},
				Data: &pb.SecretData{
					UpdatedAt: secret.DataUpdated,
					Data:      secret.Data,
				},
			},
		}

		err = stream.Send(req)
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
			if errors.Is(err, auth.ErrNeedRetry) {
				continue // переходим к следующей попытке
			}
			// return fmt.Errorf("cannot send stream request: %v - %v", err, stream.RecvMsg(nil))
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("cannot send stream request: %v", err)
			}
		}

		var res *pb.CreateSecretResponse
		res, err = stream.Recv()
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))
		}
		if err == io.EOF {
			logger.RPCLogger.Debug("no more responses")
			err = nil
		}
		if errors.Is(err, auth.ErrNeedRetry) {
			continue // переходим к следующей попытке
		}
		if err != nil {
			return nil, fmt.Errorf("cannot receive stream response: %v", err)
		}
		logger.RPCLogger.Debug(fmt.Sprintf("received response: %v", res))
		return res, nil
	}

	logger.RPCLogger.Debug(fmt.Sprintf("createSecretsWithRetry error: %v", err))
	return nil, err
}

// Update
func (app *ClientApp) syncUpdateSecretOnServer(ctx context.Context, syncStatus *SyncStatus) error {
	list, err := app.store.GetSecretsIDsForServerUpdate(ctx)
	if err != nil {
		return nil
	}

	// Устанавливаем соединение стрима
	stream, err := app.grpcKeep.UpdateSecrets(ctx)
	if err != nil {
		return err
	}
	for _, secret_id := range list {
		logger.Logger.Debug(fmt.Sprintf("try update secret id: %s", secret_id.String()))
		// получить данные по секрету
		secret, err := app.store.GetSecretForUpdate(ctx, secret_id)
		if err != nil {
			return err
		}
		resp, err := app.updateSecretsWithRetry(ctx, stream, secret)
		if err != nil {
			return err
		}

		id, err := uuid.Parse(resp.Id)
		if err != nil {
			return fmt.Errorf("parse uuid: %w", err)
		}

		if resp.Conflict {
			syncStatus.Conflicted = append(syncStatus.Conflicted, id)
		} else {
			syncStatus.Remote.Updated++
		}

		err = app.store.UpdateSecretVersionAfterUpdate(ctx, resp)
		if err != nil {
			return err
		}

	}
	stream.CloseSend()
	return nil
}
func (app *ClientApp) updateSecretsWithRetry(
	ctx context.Context,
	stream grpc.BidiStreamingClient[pb.UpdateSecretRequest, pb.UpdateSecretResponse],
	secret *models.SecretDBUpdateServer) (*pb.UpdateSecretResponse, error) {
	var err error
	for range maxRetries {
		req := &pb.UpdateSecretRequest{
			Id: secret.ID.String(),
		}
		if secret.Meta != nil {
			req.Meta = &pb.UpdateRequestInfo{
				UpdatedAt: secret.Meta.Updated,
				Version:   secret.Meta.Version,
				Data:      secret.Meta.Data,
			}
		}
		if secret.Data != nil {
			req.Data = &pb.UpdateRequestInfo{
				UpdatedAt: secret.Data.Updated,
				Version:   secret.Data.Version,
				Data:      secret.Data.Data,
			}
		}

		err = stream.Send(req)
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
			if errors.Is(err, auth.ErrNeedRetry) {
				continue // переходим к следующей попытке
			}
			// return fmt.Errorf("cannot send stream request: %v - %v", err, stream.RecvMsg(nil))
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("cannot send stream request: %v", err)
			}
		}

		var res *pb.UpdateSecretResponse
		res, err = stream.Recv()
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))
		}
		if err == io.EOF {
			logger.RPCLogger.Debug("no more responses")
			err = nil
		}
		if errors.Is(err, auth.ErrNeedRetry) {
			continue // переходим к следующей попытке
		}
		if err != nil {
			return nil, fmt.Errorf("cannot receive stream response: %v", err)
		}
		logger.RPCLogger.Debug(fmt.Sprintf("received response: %v", res))
		return res, nil
	}

	logger.RPCLogger.Debug(fmt.Sprintf("updateSecretsWithRetry error: %v", err))
	return nil, err
}

// Delete
func (app *ClientApp) syncDeleteSecretOnServer(ctx context.Context, syncStatus *SyncStatus) error {
	list, err := app.store.GetSecretsIDsForServerDelete(ctx)
	if err != nil {
		return nil
	}

	// Устанавливаем соединение стрима
	stream, err := app.grpcKeep.DeleteSecrets(ctx)
	if err != nil {
		return err
	}
	for _, secret_id := range list {
		logger.Logger.Debug(fmt.Sprintf("try delete secret id: %s", secret_id.String()))
		// получить данные по секрету
		secret, err := app.store.GetSecretForDelete(ctx, secret_id)
		if err != nil {
			return err
		}
		resp, err := app.deleteSecretsWithRetry(ctx, stream, secret)
		if err != nil {
			return err
		}

		id, err := uuid.Parse(resp.Id)
		if err != nil {
			return fmt.Errorf("parse uuid: %w", err)
		}

		if resp.Conflict {
			syncStatus.Conflicted = append(syncStatus.Conflicted, id)
		} else {
			syncStatus.Remote.Deleted++
		}

		err = app.store.UpdateSecretAfterDelete(ctx, resp)
		if err != nil {
			return err
		}

	}
	stream.CloseSend()
	return nil
}

func (app *ClientApp) deleteSecretsWithRetry(
	ctx context.Context,
	stream grpc.BidiStreamingClient[pb.DeleteSecretRequest, pb.DeleteSecretResponse],
	secret *models.SecretDBDeleteServer) (*pb.DeleteSecretResponse, error) {
	var err error
	for range maxRetries {
		req := &pb.DeleteSecretRequest{
			Id:          secret.ID.String(),
			MetaVersion: secret.MetaVersion,
			DataVersion: secret.DataVersion,
		}

		err = stream.Send(req)
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
			if errors.Is(err, auth.ErrNeedRetry) {
				continue // переходим к следующей попытке
			}
			// return fmt.Errorf("cannot send stream request: %v - %v", err, stream.RecvMsg(nil))
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("cannot send stream request: %v", err)
			}
		}

		var res *pb.DeleteSecretResponse
		res, err = stream.Recv()
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))
		}
		if err == io.EOF {
			logger.RPCLogger.Debug("no more responses")
			err = nil
		}
		if errors.Is(err, auth.ErrNeedRetry) {
			continue // переходим к следующей попытке
		}
		if err != nil {
			return nil, fmt.Errorf("cannot receive stream response: %v", err)
		}
		logger.RPCLogger.Debug(fmt.Sprintf("received response: %v", res))
		return res, nil
	}

	logger.RPCLogger.Debug(fmt.Sprintf("updateSecretsWithRetry error: %v", err))
	return nil, err
}

func (app *ClientApp) syncFromServer(ctx context.Context, syncStatus *SyncStatus) error {
	serverInfo, err := app.secretsListInfoWithRetry(ctx)
	if err != nil {
		return err
	}

	localInfo, err := app.store.SecretsListInfo(ctx)
	if err != nil {
		return err
	}
	for secret_id := range serverInfo {
		if info, ok := localInfo[secret_id]; !ok {
			// создаем
			err := app.createSecretFromServer(ctx, serverInfo[secret_id])
			if err != nil {
				return err
			}
			syncStatus.Local.Added++
		} else {
			if serverInfo[secret_id].DataVersion != info.DataVersion ||
				serverInfo[secret_id].MetaVersion != info.MetaVersion {
				// обновляем
				err := app.updateSecretFromServer(ctx, serverInfo[secret_id])
				if err != nil {
					return err
				}
				syncStatus.Local.Updated++
			}
			delete(localInfo, secret_id)
		}
	}
	// все что осталось в localInfo удаляем
	if len(localInfo) != 0 {
		deleteIDs := make([]string, 0, len(localInfo))
		for secret_id := range localInfo {
			deleteIDs = append(deleteIDs, secret_id)
		}
		err := app.store.ForceDelete(ctx, deleteIDs)
		if err != nil {
			return err
		}
		syncStatus.Local.Deleted += len(localInfo)
	}

	return nil
}

func (app *ClientApp) secretsListInfoWithRetry(ctx context.Context) (models.SecretListInfo, error) {
	var err error
	for range maxRetries {
		var stream grpc.ServerStreamingClient[pb.SecretsListResponse]
		// Устанавливаем соединение стрима
		stream, err = app.grpcKeep.SecretsListInfo(ctx, &pb.SecretsListRequest{})
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("SecretsListInfo err: %v", err))
			return nil, err
		}

		serverInfo := make(models.SecretListInfo)
		for {
			var resp *pb.SecretsListResponse
			resp, err = stream.Recv()
			if err != nil {
				logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))
			}
			if err == io.EOF {
				logger.RPCLogger.Debug("no more responses")
				return serverInfo, nil
			}
			if errors.Is(err, auth.ErrNeedRetry) {
				break // переходим к следующей попытке
			}
			if err != nil {
				return nil, fmt.Errorf("cannot receive stream response: %v", err)
			}
			serverInfo[resp.Id] = resp
		}
		if err == nil {
			return serverInfo, nil
		}
	}

	logger.RPCLogger.Debug(fmt.Sprintf("secretsListInfoWithRetry error: %v", err))
	return nil, err
}

func (app *ClientApp) createSecretFromServer(ctx context.Context, info *pb.SecretsListResponse) error {
	// Устанавливаем соединение стрима
	stream, err := app.grpcKeep.GetSecrets(ctx)
	if err != nil {
		return err
	}

	logger.Logger.Debug(fmt.Sprintf("try get secret from server id: %s", info.Id))
	// получить данные по секрету
	resp, err := app.getSecretsWithRetry(ctx, stream, info)
	if err != nil {
		return err
	}

	err = app.store.ForceCreateSecret(ctx, resp)
	if err != nil {
		return err
	}

	stream.CloseSend()
	return nil
}
func (app *ClientApp) getSecretsWithRetry(
	ctx context.Context,
	stream grpc.BidiStreamingClient[pb.GetSecretsRequest, pb.GetSecretsResponse],
	info *pb.SecretsListResponse) (*pb.GetSecretsResponse, error) {
	var err error
	for range maxRetries {
		req := &pb.GetSecretsRequest{
			Id: info.Id,
		}

		err = stream.Send(req)
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Send get error: %v", err))
			if errors.Is(err, auth.ErrNeedRetry) {
				continue // переходим к следующей попытке
			}
			// return fmt.Errorf("cannot send stream request: %v - %v", err, stream.RecvMsg(nil))
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("cannot send stream request: %v", err)
			}
		}

		var res *pb.GetSecretsResponse
		res, err = stream.Recv()
		if err != nil {
			logger.RPCLogger.Debug(fmt.Sprintf("Recv get error: %v", err))
		}
		if err == io.EOF {
			logger.RPCLogger.Debug("no more responses")
			err = nil
		}
		if errors.Is(err, auth.ErrNeedRetry) {
			continue // переходим к следующей попытке
		}
		if err != nil {
			return nil, fmt.Errorf("cannot receive stream response: %v", err)
		}
		logger.RPCLogger.Debug(fmt.Sprintf("received response: %v", res))
		return res, nil
	}

	logger.RPCLogger.Debug(fmt.Sprintf("updateSecretsWithRetry error: %v", err))
	return nil, err
}

func (app *ClientApp) updateSecretFromServer(ctx context.Context, info *pb.SecretsListResponse) error {
	return nil
}
