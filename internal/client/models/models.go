package models

type SecretDB struct {
	ID   int
	Type SecretType
	Meta []byte
	Data []byte
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
	SecretTypeMax // Must be last
)

func (s SecretType) String() string {
	switch s {
	case SecretTypeLogingPassword:
		return "Login and Password"
	case SecretTypeCreditCard:
		return "Credit/debit card"
	default:
		return ""
	}
}

type Meta map[string]string

const (
	MetaKeyName = "__name__"
)

type Data struct {
	LoginPassword *LoginPassword `json:"login_password,omitempty"`
	CreditCard    *CreditCard    `json:"credit_card,omitempty"`
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
