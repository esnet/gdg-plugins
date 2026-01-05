//go:build tinygo
// +build tinygo

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/extism/go-pdk"
	"golang.org/x/crypto/pbkdf2"
)

const (
	configPassphrase = "passphrase"
	keySize          = 32 // AES-256
	nonceSize        = 12 // GCM standard nonce size
	saltSize         = 16
	pbkdf2Iterations = 100000
)

func getPassphrase() (string, error) {
	passphrase, ok := pdk.GetConfig(configPassphrase)
	if !ok || passphrase == "" {
		return "", errors.New("passphrase not configured")
	}
	return passphrase, nil
}

// deriveKey uses PBKDF2 to derive a key from passphrase and salt
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, keySize, sha256.New)
}

//export Encode
func Encode() int32 {
	pdk.Log(pdk.LogInfo, "=== Encode started ===")

	input := pdk.Input()
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Input length: %d bytes", len(input)))

	passphrase, err := getPassphrase()
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Passphrase error: %v", err))
		pdk.SetError(errors.New("passphrase not configured"))
		return 1
	}

	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Salt generation error: %v", err))
		pdk.SetError(err)
		return 1
	}
	pdk.Log(pdk.LogDebug, "Salt generated")

	// Derive key from passphrase
	key := deriveKey(passphrase, salt)
	pdk.Log(pdk.LogDebug, "Key derived")

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Cipher creation error: %v", err))
		pdk.SetError(err)
		return 1
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("GCM creation error: %v", err))
		pdk.SetError(err)
		return 1
	}
	pdk.Log(pdk.LogDebug, "GCM cipher created")

	// Generate random nonce
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Nonce generation error: %v", err))
		pdk.SetError(err)
		return 1
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, input, nil)
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Encrypted: %d bytes", len(ciphertext)))

	// Combine: salt + nonce + ciphertext
	result := make([]byte, 0, saltSize+nonceSize+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	// Base64 encode for JSON/YAML safety
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(result)))
	base64.StdEncoding.Encode(encoded, result)

	mem := pdk.AllocateString(string(encoded))
	pdk.OutputMemory(mem)

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Encrypted %d bytes to %d bytes (base64)", len(input), len(encoded)))
	return 0
}

//export Decode
func Decode() int32 {
	pdk.Log(pdk.LogInfo, "=== Decode started ===")

	input := pdk.InputString()
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Input length: %d", len(input)))

	passphrase, err := getPassphrase()
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Passphrase error: %v", err))
		pdk.SetError(errors.New("passphrase not configured"))
		return 1
	}

	// Base64 decode
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(input)))
	n, err := base64.StdEncoding.Decode(decoded, []byte(input))
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Base64 decode error: %v", err))
		pdk.SetError(err)
		return 1
	}
	decoded = decoded[:n]
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Base64 decoded: %d bytes", len(decoded)))

	// Extract salt, nonce, and ciphertext
	if len(decoded) < saltSize+nonceSize {
		pdk.Log(pdk.LogError, "Invalid encrypted data: too short")
		pdk.SetError(errors.New("invalid encrypted data"))
		return 1
	}

	salt := decoded[:saltSize]
	nonce := decoded[saltSize : saltSize+nonceSize]
	ciphertext := decoded[saltSize+nonceSize:]
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Extracted salt (%d), nonce (%d), ciphertext (%d)", len(salt), len(nonce), len(ciphertext)))

	// Derive key from passphrase
	key := deriveKey(passphrase, salt)
	pdk.Log(pdk.LogDebug, "Key derived")

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Cipher creation error: %v", err))
		pdk.SetError(err)
		return 1
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("GCM creation error: %v", err))
		pdk.SetError(err)
		return 1
	}
	pdk.Log(pdk.LogDebug, "GCM cipher created")

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Decryption error: %v", err))
		pdk.SetError(err)
		return 1
	}
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Decrypted: %d bytes", len(plaintext)))

	mem := pdk.AllocateBytes(plaintext)
	pdk.OutputMemory(mem)

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Decrypted %d bytes (base64) to %d bytes", len(input), len(plaintext)))
	return 0
}

func main() {}
