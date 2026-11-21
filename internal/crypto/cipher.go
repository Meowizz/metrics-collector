package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Параметры гибридной схемы: AES-256-GCM для данных, RSA-OAEP (SHA-256) для AES-ключа.
const (
	aesKeySize = 32 // размер AES-ключа: 32 байта — AES-256
	nonceSize  = 12 // стандартный размер nonce GCM (gcm.NonceSize())
	tagSize    = 16 // размер тега аутентификации GCM (gcm.Overhead())
)

// Encrypt шифрует openData публичным ключом получателя по гибридной схеме.
// На каждое сообщение генерируются новый AES-ключ и nonce: данные шифруются AES-256-GCM,
// а AES-ключ — RSA-OAEP с SHA-256, поэтому размер данных не ограничен пределом RSA.
//
// Формат результата (длины частей получатель вычисляет сам, разделителей нет):
//
//	| AES-ключ, зашифрованный RSA | nonce    | данные, зашифрованные AES-GCM, + тег |
//	| publicKey.Size() байт       | 12 байт  | len(openData) + 16 байт              |
//
// Одинаковые данные каждый раз дают разный результат.
func Encrypt(publicKey *rsa.PublicKey, openData []byte) ([]byte, error) {
	aesKey := make([]byte, aesKeySize)
	_, err := rand.Read(aesKey)
	if err != nil {
		return nil, fmt.Errorf("generate aes key: %w", err)
	}

	cphrBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(cphrBlock)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	encryptedData := gcm.Seal(nil, nonce, openData, nil)

	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt aes key: %w", err)
	}

	encryptedMessage := make([]byte, 0, len(encryptedKey)+len(nonce)+len(encryptedData))

	encryptedMessage = append(encryptedMessage, encryptedKey...)
	encryptedMessage = append(encryptedMessage, nonce...)
	encryptedMessage = append(encryptedMessage, encryptedData...)

	return encryptedMessage, nil
}

// Decrypt расшифровывает сообщение в формате Encrypt приватным ключом получателя:
// достаёт AES-ключ через RSA-OAEP с SHA-256 и открывает данные AES-256-GCM.
// Возвращает ошибку, если сообщение короче минимально возможного,
// ключ не подходит или хотя бы один байт сообщения изменён (не сошёлся тег GCM).
// На произвольных входных данных не паникует, поэтому подходит для тел HTTP-запросов.
func Decrypt(privateKey *rsa.PrivateKey, encryptedMessage []byte) ([]byte, error) {
	keySize := privateKey.Size()
	if len(encryptedMessage) < keySize+nonceSize+tagSize {
		return nil, errors.New("encrypted message is too short")
	}

	encryptedKey := encryptedMessage[:keySize]
	nonce := encryptedMessage[keySize : keySize+nonceSize]
	encryptedData := encryptedMessage[keySize+nonceSize:]

	aesKey, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, encryptedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("incorrect key or corrupt bytes: %w", err)
	}

	cphrBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(cphrBlock)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	openData, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("corrupted bytes: %w", err)
	}
	return openData, nil
}
