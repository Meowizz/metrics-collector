package crypto

import (
	"bytes"
	"crypto/rsa"
	"testing"
)

// Ключи генерируются один раз на весь пакет тестов: генерация 4096-битного ключа заметно долгая.
var (
	testKey2048 = mustGenerate(2048)
	testKey4096 = mustGenerate(4096)
)

func mustGenerate(bits int) *rsa.PrivateKey {
	key, err := GenerateKeyRSA(bits)
	if err != nil {
		panic(err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	payloads := []struct {
		name string
		data []byte
	}{
		{name: "пустые данные", data: []byte{}},
		{name: "короткие данные", data: []byte("hello")},
		{name: "больше предела RSA (191 байт)", data: bytes.Repeat([]byte("a"), 191)},
		{name: "100 КБ", data: bytes.Repeat([]byte("metrics"), 100*1024/7)},
	}
	keys := []struct {
		name string
		key  *rsa.PrivateKey
	}{
		{name: "2048", key: testKey2048},
		{name: "4096", key: testKey4096},
	}

	for _, k := range keys {
		for _, p := range payloads {
			t.Run(k.name+"/"+p.name, func(t *testing.T) {
				enc, err := Encrypt(&k.key.PublicKey, p.data)
				if err != nil {
					t.Fatalf("Encrypt: %v", err)
				}

				wantLen := k.key.Size() + nonceSize + len(p.data) + tagSize
				if len(enc) != wantLen {
					t.Errorf("длина сообщения: ожидали %d, получили %d", wantLen, len(enc))
				}

				dec, err := Decrypt(k.key, enc)
				if err != nil {
					t.Fatalf("Decrypt: %v", err)
				}
				if !bytes.Equal(dec, p.data) {
					t.Error("расшифрованные данные не совпадают с исходными")
				}
			})
		}
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	data := []byte("одинаковые данные")

	first, err := Encrypt(&testKey2048.PublicKey, data)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := Encrypt(&testKey2048.PublicKey, data)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	size := testKey2048.Size()
	if bytes.Equal(first[:size], second[:size]) {
		t.Error("зашифрованный AES-ключ совпал: ключ или OAEP не случайны")
	}
	if bytes.Equal(first[size:size+nonceSize], second[size:size+nonceSize]) {
		t.Error("nonce совпал: nonce не случаен")
	}
	if bytes.Equal(first[size+nonceSize:], second[size+nonceSize:]) {
		t.Error("шифротекст данных совпал")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, err := Encrypt(&testKey2048.PublicKey, []byte("секрет"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	other := mustGenerate(2048)
	got, err := Decrypt(other, enc)
	if err == nil {
		t.Fatal("ожидали ошибку при расшифровке чужим ключом")
	}
	if got != nil {
		t.Error("при ошибке данные должны быть nil")
	}
}

func TestDecryptTampered(t *testing.T) {
	data := []byte(`{"id":"Alloc","type":"gauge","value":1.5}`)
	enc, err := Encrypt(&testKey2048.PublicKey, data)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	size := testKey2048.Size()
	positions := []struct {
		name  string
		index int
	}{
		{name: "зашифрованный AES-ключ", index: 0},
		{name: "nonce", index: size},
		{name: "шифротекст данных", index: size + nonceSize},
		{name: "тег GCM", index: len(enc) - 1},
	}

	for _, p := range positions {
		t.Run(p.name, func(t *testing.T) {
			tampered := bytes.Clone(enc)
			tampered[p.index] ^= 0x01

			got, err := Decrypt(testKey2048, tampered)
			if err == nil {
				t.Fatal("ожидали ошибку на изменённом сообщении")
			}
			if got != nil {
				t.Error("при ошибке данные должны быть nil")
			}
		})
	}
}

func TestDecryptMalformed(t *testing.T) {
	minLen2048 := testKey2048.Size() + nonceSize + tagSize

	cases := []struct {
		name string
		key  *rsa.PrivateKey
		msg  []byte
	}{
		{name: "nil", key: testKey2048, msg: nil},
		{name: "пустое сообщение", key: testKey2048, msg: []byte{}},
		{name: "10 байт мусора", key: testKey2048, msg: bytes.Repeat([]byte{0xff}, 10)},
		{name: "на байт короче минимума", key: testKey2048, msg: make([]byte, minLen2048-1)},
		{name: "минимальная длина из нулей", key: testKey2048, msg: make([]byte, minLen2048)},
		{name: "ключ 4096, 300 байт", key: testKey4096, msg: make([]byte, 300)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decrypt(tc.key, tc.msg)
			if err == nil {
				t.Fatal("ожидали ошибку")
			}
			if got != nil {
				t.Error("при ошибке данные должны быть nil")
			}
		})
	}
}

// Типичный батч метрик агента в JSON — порядка 2–4 КБ.
var benchPayload = bytes.Repeat([]byte(`{"id":"HeapAlloc","type":"gauge","value":123456.789},`), 64)

func BenchmarkEncrypt(b *testing.B) {
	b.SetBytes(int64(len(benchPayload)))
	for b.Loop() {
		if _, err := Encrypt(&testKey2048.PublicKey, benchPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	enc, err := Encrypt(&testKey2048.PublicKey, benchPayload)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(benchPayload)))
	for b.Loop() {
		if _, err := Decrypt(testKey2048, enc); err != nil {
			b.Fatal(err)
		}
	}
}
