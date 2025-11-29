package models

import "encoding/json"

type SecretDB struct {
	ID       int
	Type     SecretType
	Meta     []byte
	Data     []byte
	FilePath string
}

type Secret struct {
	ID   int
	Type SecretType
	Name string
	Meta Meta
	Data Data
}

type SecretType int

const (
	SecretTypeLogingPassword SecretType = iota
	SecretTypeCreditCard
	SecretTypeText
	SecretTypeFile
	SecretTypeMax // Must be last
)

func (s SecretType) String() string {
	switch s {
	case SecretTypeLogingPassword:
		return "Login and Password"
	case SecretTypeCreditCard:
		return "Credit/debit card"
	case SecretTypeText:
		return "Text"
	case SecretTypeFile:
		return "Binary"
	default:
		return ""
	}
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
	SecretName   string `json:"secret_name"`
	OrigFileName string `json:"orig_filename,omitempty"`
}

const (
	MetaKeyInternal = "__internal__"
)

type Data struct {
	LoginPassword *LoginPassword `json:"login_password,omitempty"`
	CreditCard    *CreditCard    `json:"credit_card,omitempty"`
	Text          string         `json:"-"`
	FilePath      FilePath       `json:"-"`
}

type FilePath struct {
	Path     string
	OldPath  string
	TmpPath  string
	OrigName string
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
