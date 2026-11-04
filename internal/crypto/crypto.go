package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

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
