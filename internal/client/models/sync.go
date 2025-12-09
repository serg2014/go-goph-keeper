package models

import (
	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
)

type SecretDBCreateServer struct {
	ID          uuid.UUID
	Type        SecretType
	MetaUpdated int64
	Meta        []byte
	DataUpdated int64
	Data        []byte
}

type SecretDBUpdateServerBlock struct {
	Updated int64
	Version int64
	Data    []byte
}
type SecretDBUpdateServer struct {
	ID   uuid.UUID
	Type SecretType
	Meta *SecretDBUpdateServerBlock
	Data *SecretDBUpdateServerBlock
}

type SecretDBDeleteServer struct {
	ID          uuid.UUID
	Type        SecretType
	MetaVersion int64
	DataVersion int64
}

type SecretListInfo map[string]*pb.SecretsListResponse
