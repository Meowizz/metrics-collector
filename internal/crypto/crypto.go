package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// EncryptedHeader — заголовок, которым агент помечает запросы с зашифрованным телом.
// Сервер расшифровывает тело только у запросов с этим заголовком.
const EncryptedHeader = "X-Encrypted"

// GenerateKeyRSA генерирует приватный RSA-ключ длиной keyLen бит.
// Публичный ключ доступен через поле PublicKey результата.
// Возвращает ошибку, если keyLen меньше 1024 или генерация не удалась.
func GenerateKeyRSA(keyLen int) (*rsa.PrivateKey, error) {

	if keyLen < 1024 {
		return nil, errors.New("key length must be at least 1024")
	}

	prKey, err := rsa.GenerateKey(rand.Reader, keyLen)

	if err != nil {
		return nil, fmt.Errorf("error generating RSA key: %w", err)
	}

	return prKey, nil
}

// EncodeToPEM кодирует пару ключей в формат PEM для записи в файлы.
// Приватный ключ сериализуется в PKCS#1 (блок "RSA PRIVATE KEY"),
// публичный — в PKIX (блок "PUBLIC KEY").
// Возвращает PEM приватного ключа, PEM публичного ключа и ошибку сериализации.
func EncodeToPEM(privateKey *rsa.PrivateKey) ([]byte, []byte, error) {

	derPrivate := x509.MarshalPKCS1PrivateKey(privateKey)
	derPublic, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error serialize public key: %w", err)
	}

	blockPrivate := pem.Block{Type: "RSA PRIVATE KEY", Bytes: derPrivate, Headers: nil}
	blockPub := pem.Block{Type: "PUBLIC KEY", Bytes: derPublic, Headers: nil}

	pemPrivate := pem.EncodeToMemory(&blockPrivate)
	pemPub := pem.EncodeToMemory(&blockPub)

	return pemPrivate, pemPub, nil
}

// toBlockPEM читает файл filePath и возвращает первый PEM-блок из него как есть,
// с исходным типом. Тип блока не проверяет: допустимые типы решает вызывающая функция.
// Возвращает ошибку, если файл не читается или PEM-блок в нём не найден.
func toBlockPEM(filePath string) (*pem.Block, error) {
	fileByte, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	block, _ := pem.Decode(fileByte)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", filePath)
	}
	return block, nil
}

// LoadPrivateKey загружает приватный RSA-ключ сервера из PEM-файла filePath.
// Поддерживает PKCS#1 (блок "RSA PRIVATE KEY", формат утилиты keygen)
// и PKCS#8 (блок "PRIVATE KEY", формат openssl genrsa в OpenSSL 3).
// Возвращает ошибку, если файл не читается, тип блока не поддерживается
// или ключ в нём не RSA.
func LoadPrivateKey(filePath string) (*rsa.PrivateKey, error) {
	pemBlock, err := toBlockPEM(filePath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}
	switch pemBlock.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing private key %s: %w", filePath, err)
		}
		return key, nil
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing private key %s: %w", filePath, err)
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key %s is not RSA: %T", filePath, parsed)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM block %q in %s", pemBlock.Type, filePath)
	}
}

// LoadPublicKey загружает публичный RSA-ключ агента из PEM-файла filePath.
// Поддерживает PKIX (блок "PUBLIC KEY", формат утилиты keygen и openssl rsa -pubout)
// и PKCS#1 (блок "RSA PUBLIC KEY").
// Возвращает ошибку, если файл не читается, тип блока не поддерживается
// или ключ в нём не RSA.
func LoadPublicKey(filePath string) (*rsa.PublicKey, error) {
	pemBlock, err := toBlockPEM(filePath)
	if err != nil {
		return nil, fmt.Errorf("load public key: %w", err)
	}
	switch pemBlock.Type {
	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing public key %s: %w", filePath, err)
		}
		return key, nil
	case "PUBLIC KEY":
		parsed, err := x509.ParsePKIXPublicKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing public key %s: %w", filePath, err)
		}
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key %s is not RSA: %T", filePath, parsed)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported public key PEM block %q in %s", pemBlock.Type, filePath)
	}
}
