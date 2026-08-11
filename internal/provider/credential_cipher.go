package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// CredentialCipher protects provider credentials before they cross a durable
// persistence boundary. The deployment secret is reduced to a fixed AES-256 key.
type CredentialCipher struct{ key [32]byte }

func NewCredentialCipher(secret string) *CredentialCipher {
	if secret == "" {
		return nil
	}
	return &CredentialCipher{key: sha256.Sum256([]byte("kode-stream/provider-credentials/v1/" + secret))}
}

func (c *CredentialCipher) Encrypt(plaintext string) (string, error) {
	if c == nil || plaintext == "" {
		return "", errors.New("provider credential encryption is unavailable")
	}
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), []byte("kode-stream/provider-credential/v1"))
	return "v1:" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c *CredentialCipher) Decrypt(encoded string) (string, error) {
	if c == nil || len(encoded) < 4 || encoded[:3] != "v1:" {
		return "", errors.New("provider credential cannot be decrypted")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(encoded[3:])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("provider credential is truncated")
	}
	plaintext, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte("kode-stream/provider-credential/v1"))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
