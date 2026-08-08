package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

func RunMigrations(dsn string, migrationPath string) error {
	m, err := migrate.New(
		"file://"+migrationPath,
		dsn,
	)

	if err != nil {
		return fmt.Errorf("Failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("Database schema is up to date")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Migrations applied successfully")
	return nil
}

func (p *PostgresStorage) Ping() error {
	return p.db.Ping()
}

func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

func (p *PostgresStorage) UpdateGauge(name string, value float64) error {
	query := `
		INSERT INTO metrics (id, type, value)
		VALUES ($1, 'gauge', $2)
		ON CONFLICT (id)
		DO UPDATE SET value = $2
	`

	_, err := p.db.ExecContext(context.Background(), query, name, value)
	return err
}

func (p *PostgresStorage) UpdateCounter(name string, value int64) error {
	query := `
		INSERT INTO metrics (id, type, value)
		VALUES ($1, 'counter', $2)
		ON CONFLICT (id)
		DO UPDATE SET value = metrics.value + $2
	`

	_, err := p.db.ExecContext(context.Background(), query, name, float64(value))
	return err
}

func (p *PostgresStorage) GetGauge(name string) (float64, bool) {
	query := `SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`

	var value float64
	err := p.db.QueryRowContext(context.Background(), query, name).Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Printf("Error getting gauge %s: %v", name, err)
		return 0, false
	}
	return value, true
}

func (p *PostgresStorage) GetCounter(name string) (int64, bool) {
	query := `SELECT value FROM metrics WHERE id = $1 AND type = 'counter'`

	var valueFloat float64

	err := p.db.QueryRowContext(context.Background(), query, name).Scan(&valueFloat)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Printf("Error getting gauge %s: %v", name, err)
		return 0, false
	}
	return int64(valueFloat), true
}

func (p *PostgresStorage) UpdateBatch(metrics []models.Metrics) error {
	ctx := context.Background()

	tx, err := p.db.BeginTx(ctx, nil)

	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmtGauge, err := tx.PrepareContext(ctx, `
	INSERT INTO metrics (id, type, value)
	VALUES ($1, 'gauge', $2)
	ON CONFLICT (id) DO UPDATE SET value = $2`)

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare gauge stmt: %w", err)
	}

	defer stmtGauge.Close()

	stmtCounter, err := tx.PrepareContext(ctx, `
	INSERT INTO metrics (id, type, value)
	VALUES ($1, 'counter', $2)
	ON CONFLICT (id) DO UPDATE SET value = metrics.value + $2
`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare counter stmt: %w", err)
	}

	defer stmtCounter.Close()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				if _, err := stmtGauge.ExecContext(ctx, metric.ID, *metric.Value); err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to insert gauge %s: %w", metric.ID, err)
				}
			}
		case models.Counter:
			if metric.Delta != nil {
				if _, err := stmtCounter.ExecContext(ctx, metric.ID, float64(*metric.Delta)); err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to insert counter %s: %w", metric.ID, err)
				}
			}
		}
	}
	return tx.Commit()
}
