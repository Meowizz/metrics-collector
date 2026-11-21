package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyRSA(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{name: "слишком короткий ключ", keyLen: 512, wantErr: true},
		{name: "нулевая длина", keyLen: 0, wantErr: true},
		{name: "2048 бит", keyLen: 2048, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateKeyRSA(tt.keyLen)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ожидали ошибку для длины %d, получили nil", tt.keyLen)
				}
				if key != nil {
					t.Error("при ошибке ключ должен быть nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if got := key.N.BitLen(); got != tt.keyLen {
				t.Errorf("длина ключа: ожидали %d, получили %d", tt.keyLen, got)
			}
		})
	}
}

func TestEncodeToPEM(t *testing.T) {
	key, err := GenerateKeyRSA(2048)
	if err != nil {
		t.Fatalf("генерация ключа: %v", err)
	}

	privPEM, pubPEM, err := EncodeToPEM(key)
	if err != nil {
		t.Fatalf("EncodeToPEM: %v", err)
	}

	t.Run("приватный ключ", func(t *testing.T) {
		block, rest := pem.Decode(privPEM)
		if block == nil {
			t.Fatal("PEM-блок приватного ключа не найден")
		}
		if len(rest) != 0 {
			t.Errorf("после PEM-блока остались лишние данные: %d байт", len(rest))
		}
		if block.Type != "RSA PRIVATE KEY" {
			t.Errorf("тип блока: ожидали %q, получили %q", "RSA PRIVATE KEY", block.Type)
		}

		parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			t.Fatalf("разбор приватного ключа: %v", err)
		}
		if !parsed.Equal(key) {
			t.Error("приватный ключ после разбора не совпадает с исходным")
		}
	})

	t.Run("публичный ключ", func(t *testing.T) {
		block, rest := pem.Decode(pubPEM)
		if block == nil {
			t.Fatal("PEM-блок публичного ключа не найден")
		}
		if len(rest) != 0 {
			t.Errorf("после PEM-блока остались лишние данные: %d байт", len(rest))
		}
		if block.Type != "PUBLIC KEY" {
			t.Errorf("тип блока: ожидали %q, получили %q", "PUBLIC KEY", block.Type)
		}

		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			t.Fatalf("разбор публичного ключа: %v", err)
		}
		pub, ok := parsed.(*rsa.PublicKey)
		if !ok {
			t.Fatalf("ожидали *rsa.PublicKey, получили %T", parsed)
		}
		if !pub.Equal(&key.PublicKey) {
			t.Error("публичный ключ после разбора не совпадает с исходным")
		}
	})
}

// writePEM записывает PEM-блок с типом blockType и содержимым der во временный файл
// и возвращает путь к нему.
func writePEM(t *testing.T, name, blockType string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("запись %s: %v", path, err)
	}
	return path
}

// writeRaw записывает произвольные байты во временный файл и возвращает путь к нему.
func writeRaw(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("запись %s: %v", path, err)
	}
	return path
}

// mustRSAKey генерирует RSA-ключ для теста.
func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := GenerateKeyRSA(2048)
	if err != nil {
		t.Fatalf("генерация RSA-ключа: %v", err)
	}
	return key
}

// mustECDSAKey генерирует ECDSA-ключ: валидный ключ, но не RSA.
func mustECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("генерация ECDSA-ключа: %v", err)
	}
	return key
}

func TestLoadPublicKey(t *testing.T) {
	key := mustRSAKey(t)

	pkix, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}

	t.Run("PKIX из EncodeToPEM", func(t *testing.T) {
		_, pubPEM, err := EncodeToPEM(key)
		if err != nil {
			t.Fatalf("EncodeToPEM: %v", err)
		}
		got, err := LoadPublicKey(writeRaw(t, "public.pem", pubPEM))
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if !got.Equal(&key.PublicKey) {
			t.Error("загруженный ключ не совпадает с исходным")
		}
	})

	t.Run("PKCS#1", func(t *testing.T) {
		path := writePEM(t, "public.pem", "RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&key.PublicKey))
		got, err := LoadPublicKey(path)
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if !got.Equal(&key.PublicKey) {
			t.Error("загруженный ключ не совпадает с исходным")
		}
	})

	ecdsaPKIX, err := x509.MarshalPKIXPublicKey(&mustECDSAKey(t).PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey(ECDSA): %v", err)
	}

	errorCases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			name: "файла нет",
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.pem") },
		},
		{
			name: "не PEM",
			path: func(t *testing.T) string { return writeRaw(t, "garbage.pem", []byte("это не ключ")) },
		},
		{
			name: "передан приватный ключ",
			path: func(t *testing.T) string {
				return writePEM(t, "private.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
			},
		},
		{
			name: "испорченные байты в блоке PUBLIC KEY",
			path: func(t *testing.T) string { return writePEM(t, "broken.pem", "PUBLIC KEY", pkix[:len(pkix)/2]) },
		},
		{
			name: "ключ ECDSA вместо RSA",
			path: func(t *testing.T) string { return writePEM(t, "ecdsa.pem", "PUBLIC KEY", ecdsaPKIX) },
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoadPublicKey(tc.path(t))
			if err == nil {
				t.Fatal("ожидали ошибку, получили nil")
			}
			if got != nil {
				t.Error("при ошибке ключ должен быть nil")
			}
		})
	}
}

func TestLoadPrivateKey(t *testing.T) {
	key := mustRSAKey(t)

	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}

	t.Run("PKCS#1 из EncodeToPEM", func(t *testing.T) {
		privPEM, _, err := EncodeToPEM(key)
		if err != nil {
			t.Fatalf("EncodeToPEM: %v", err)
		}
		got, err := LoadPrivateKey(writeRaw(t, "private.pem", privPEM))
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if !got.Equal(key) {
			t.Error("загруженный ключ не совпадает с исходным")
		}
	})

	t.Run("PKCS#8", func(t *testing.T) {
		got, err := LoadPrivateKey(writePEM(t, "private.pem", "PRIVATE KEY", pkcs8))
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if !got.Equal(key) {
			t.Error("загруженный ключ не совпадает с исходным")
		}
	})

	ecdsaPKCS8, err := x509.MarshalPKCS8PrivateKey(mustECDSAKey(t))
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey(ECDSA): %v", err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pkcs1 := x509.MarshalPKCS1PrivateKey(key)

	errorCases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			name: "файла нет",
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.pem") },
		},
		{
			name: "не PEM",
			path: func(t *testing.T) string { return writeRaw(t, "garbage.pem", []byte("это не ключ")) },
		},
		{
			name: "передан публичный ключ",
			path: func(t *testing.T) string { return writePEM(t, "public.pem", "PUBLIC KEY", pkix) },
		},
		{
			name: "испорченные байты в блоке RSA PRIVATE KEY",
			path: func(t *testing.T) string {
				return writePEM(t, "broken.pem", "RSA PRIVATE KEY", pkcs1[:len(pkcs1)/2])
			},
		},
		{
			name: "испорченные байты в блоке PRIVATE KEY",
			path: func(t *testing.T) string { return writePEM(t, "broken.pem", "PRIVATE KEY", pkcs8[:len(pkcs8)/2]) },
		},
		{
			name: "ключ ECDSA вместо RSA",
			path: func(t *testing.T) string { return writePEM(t, "ecdsa.pem", "PRIVATE KEY", ecdsaPKCS8) },
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoadPrivateKey(tc.path(t))
			if err == nil {
				t.Fatal("ожидали ошибку, получили nil")
			}
			if got != nil {
				t.Error("при ошибке ключ должен быть nil")
			}
		})
	}
}
