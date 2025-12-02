package models

import "github.com/google/uuid"

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
