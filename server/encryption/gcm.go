package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func InitGcm(key []byte) (cipher.AEAD, error) {
	cb, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to generate cipher block: %s", err)
	}

	// adding a digital seal to our encrypted data
	gcm, err := cipher.NewGCM(cb)
	if err != nil {
		return nil, fmt.Errorf("failes to choose mode of operation: %s", err)
	}
	return gcm, nil
} 

// heart of the encryption
// gcm seal function to convert the data into a protected stream
func EncryptData(gcm cipher.AEAD, plaintext []byte) ([]byte, error) {
	// adding an element of unpredictibility
	// stands for: number used once
	// ensures every single encryption is unique
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %s", err)
	}

	// encrypted data in bytes
	cipherText := gcm.Seal(nonce, nonce, plaintext, nil)

	// enc := hex.EncodeToString(cipherText)
	return cipherText, nil
}
