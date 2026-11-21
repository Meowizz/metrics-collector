package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/Meowizz/metrics-collector/internal/crypto"
)

// maxEncryptedBodySize ограничивает размер зашифрованного тела: оно читается в память целиком.
const maxEncryptedBodySize = 10 << 20 // 10 МБ

// DecryptMiddleware — middleware для расшифровки тел запросов приватным ключом сервера.
// Расшифровывает только запросы с заголовком crypto.EncryptedHeader; остальные пропускает как есть,
// чтобы незашифрованные клиенты продолжали работать.
// Отвечает 400, если запрос помечен как зашифрованный, а ключ не задан (key == nil),
// тело не читается или не расшифровывается.
// Должен стоять снаружи GzipMiddleware и HashMiddleware: агент сначала сжимает тело,
// потом шифрует, поэтому сервер сначала расшифровывает, потом распаковывает и проверяет хеш.
func DecryptMiddleware(key *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(crypto.EncryptedHeader) == "" {
				next.ServeHTTP(w, r)
				return
			}
			if key == nil {
				http.Error(w, "encryption key is not configured on server", http.StatusBadRequest)
				return
			}
			reqBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEncryptedBodySize))
			if err != nil {
				http.Error(w, "failed to read encrypted body", http.StatusBadRequest)
				return
			}

			decryptedBody, err := crypto.Decrypt(key, reqBody)
			if err != nil {
				http.Error(w, "failed to decrypt body", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(decryptedBody))
			r.ContentLength = int64(len(decryptedBody))

			next.ServeHTTP(w, r)
		})
	}
}
