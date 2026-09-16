package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// EncryptAESGCM encrypts plaintext using a base64-encoded 32-byte key.
func EncryptAESGCM(plaintext string, base64Key string) (string, error) {
	if base64Key == "" {
		return "", errors.New("encryption key is not configured")
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(key) != 32 {
		return "", errors.New("encryption key must be a base64-encoded 32-byte key")
	}

	block, err := aes.NewCipher(key)
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

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

// DecryptAESGCM decrypts ciphertext using a base64-encoded 32-byte key.
func DecryptAESGCM(ciphertext string, base64Key string) (string, error) {
	if base64Key == "" {
		return "", errors.New("encryption key is not configured")
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(key) != 32 {
		return "", errors.New("encryption key must be a base64-encoded 32-byte key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted token format")
	}

	nonceSize := gcm.NonceSize()
	nonce, actualCiphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
