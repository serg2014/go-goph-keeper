package models

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type SecretDB struct {
	ID       uuid.UUID
	Type     SecretType
	Conflict bool
	Meta     []byte
	Data     []byte
}

type Secret struct {
	ID       uuid.UUID
	Type     SecretType
	Conflict bool
	Meta     BlockMeta
	Data     Data
}

type SecretType int

const (
	SecretTypeUnknown SecretType = iota
	SecretTypeLogingPassword
	SecretTypeCreditCard
	SecretTypeText
	SecretTypeFile
	SecretTypeMax // Must be last
)

func (s SecretType) String() string {
	switch s {
	case SecretTypeLogingPassword:
		return fmt.Sprintf("%c", rune(0x1F511)) // key
	case SecretTypeCreditCard:
		return fmt.Sprintf("%c", rune(0x1F4B3)) // credit card
	case SecretTypeText:
		return fmt.Sprintf("%c", rune(0x1F4C4)) // text
	case SecretTypeFile:
		return fmt.Sprintf("%c", rune(0x1F4C1)) // file folder
	default:
		return ""
	}
}

type BlockMeta struct {
	Meta         Meta         `json:"meta,omitempty"`
	InternalMeta InternalMeta `json:"internal"`
}
type Meta map[string]string

func (m *Meta) PrettyString() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type InternalMeta struct {
	SecretName string `json:"secret_name"`
}

type Data struct {
	LoginPassword *LoginPassword `json:"login_password,omitempty"`
	CreditCard    *CreditCard    `json:"credit_card,omitempty"`
	Text          string         `json:"-"`
	FilePath      *FilePath      `json:"file_path,omitempty"`
}

type FilePath struct {
	Path     string `json:"-"`
	OrigName string `json:"orig_name,omitempty"`
	OldPath  string `json:"-"`
	TmpPath  string `json:"-"`
}

type LoginPassword struct {
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
}

type CreditCard struct {
	Number string `json:"number,omitempty"`
	Exp    string `json:"exp,omitempty"`
	Cvv    string `json:"cvv,omitempty"`
}
