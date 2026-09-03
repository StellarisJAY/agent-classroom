package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// cipher 封装 AES-256-GCM 加解密，用于 API Key 等敏感字段的存储加密。

// NewGCMCipher 创建一个 GCM 加密器。key 长度须为 32 字节（AES-256）。
func NewGCMCipher(key []byte) (*GCMCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("aes key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &GCMCipher{gcm: gcm}, nil
}

type GCMCipher struct {
	gcm cipher.AEAD
}

// Encrypt 加密并返回 base64 编码串：nonce || ciphertext
func (c *GCMCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 base64 编码的密文
func (c *GCMCipher) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := c.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plain, err := c.gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
