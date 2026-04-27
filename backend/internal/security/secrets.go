package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func EncryptString(masterKey, plaintext string) (string, error) {
	key := deriveKey(masterKey)
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
	return base64.StdEncoding.EncodeToString(nonce) + "." + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptString(masterKey, payload string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(payload), ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid encrypted payload")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	key := deriveKey(masterKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func deriveKey(masterKey string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(masterKey)))
	return sum[:]
}
