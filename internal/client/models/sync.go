package models

type SecretDBCreateServer struct {
	ID          int64
	Type        SecretType
	MetaID      int64
	MetaUpdated int64
	Meta        []byte
	DataID      int64
	DataUpdated int64
	Data        []byte
}

type SecretDBUpdateServerBlock struct {
	ID      int64
	Updated int64
	Version int64
	Data    []byte
}
type SecretDBUpdateServer struct {
	ID   int64
	Type SecretType
	Meta *SecretDBUpdateServerBlock
	Data *SecretDBUpdateServerBlock
}
