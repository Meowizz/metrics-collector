package repository

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStorage{
		db: db,
	}, nil
}

func (p *PostgresStorage) Ping() error {
	return p.db.Ping()
}

func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

func (p *PostgresStorage) UpdateGauge(name string, value float64) error {
	// TODO
	return nil
}

func (p *PostgresStorage) UpdateCounter(name string, value int64) error {
	// TODO
	return nil
}

func (p *PostgresStorage) GetGauge(name string) (float64, bool) {
	// TODO
	return 0, false
}

func (p *PostgresStorage) GetCounter(name string) (int64, bool) {
	// TODO
	return 0, false
}
