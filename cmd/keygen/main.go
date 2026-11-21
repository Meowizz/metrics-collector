// Command keygen генерирует пару RSA-ключей для асимметричного шифрования
// сообщений от агента к серверу.
//
// # Запуск
//
//	go run ./cmd/keygen
//	go run ./cmd/keygen -private-key /etc/metrics/private.pem -public-key ./public.pem -length 4096
//
// Приватный ключ (PKCS#1, права 0600) передаётся серверу через -crypto-key или CRYPTO_KEY,
// публичный (PKIX, права 0644) — агенту тем же флагом или переменной окружения.
//
// Существующие файлы не перезаписываются: если любой из файлов уже есть,
// утилита завершается с ошибкой и оставляет диск в исходном состоянии.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Meowizz/metrics-collector/internal/crypto"
)

type config struct {
	pathPublic  string
	pathPrivate string
	length      int
}

// writeNewFile создаёт новый файл path с правами perm и записывает в него data.
// Существующий файл не трогает и возвращает ошибку.
// Если запись или закрытие не удались, удаляет созданный файл.
func writeNewFile(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("error opening %v file: %w", path, err)
	}

	_, err = file.Write(data)
	if err != nil {
		return errors.Join(
			fmt.Errorf("error writing %v file: %w", path, err),
			file.Close(),
			os.Remove(path),
		)
	}

	err = file.Close()
	if err != nil {
		return errors.Join(
			fmt.Errorf("error closing %v: %w", path, err),
			os.Remove(path),
		)
	}
	return nil
}

// run разбирает флаги, генерирует пару ключей и записывает её в файлы.
// Если публичный ключ записать не удалось, удаляет только что записанный приватный,
// чтобы на диске не осталось половины пары.
func run() error {
	cfg := &config{}

	flag.StringVar(&cfg.pathPrivate, "private-key", "private-key.pem", "Path to private key")
	flag.StringVar(&cfg.pathPublic, "public-key", "public-key.pem", "Path to public key")
	flag.IntVar(&cfg.length, "length", 2048, "Length of private key")

	flag.Parse()

	privateKey, err := crypto.GenerateKeyRSA(cfg.length)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	privatePEM, publicPEM, err := crypto.EncodeToPEM(privateKey)
	if err != nil {
		return fmt.Errorf("encode key: %w", err)
	}
	err = writeNewFile(cfg.pathPrivate, privatePEM, 0o600)
	if err != nil {
		return fmt.Errorf("error writing private key: %w", err)
	}
	err = writeNewFile(cfg.pathPublic, publicPEM, 0o644)
	if err != nil {
		return errors.Join(
			fmt.Errorf("error writing public key: %w", err),
			os.Remove(cfg.pathPrivate),
		)
	}
	fmt.Printf("Private key successfully written to %v\n", cfg.pathPrivate)
	fmt.Printf("Public key successfully written to %v\n", cfg.pathPublic)

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}
