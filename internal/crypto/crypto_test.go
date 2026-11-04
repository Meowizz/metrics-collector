package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
