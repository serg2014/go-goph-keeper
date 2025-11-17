package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// TODO from env
var secretForPassword = []byte("somesecret")

func sign(value, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(value)
	return base64.RawStdEncoding.EncodeToString(h.Sum(nil))
}

func SignPassword(password string) string {
	return sign([]byte(password), secretForPassword)
}
