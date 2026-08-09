package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgconn"
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

	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	query := `
		INSERT INTO metrics (id, type, value)
		VALUES ($1, 'gauge', $2)
		ON CONFLICT (id)
		DO UPDATE SET value = $2
	`
	for attempt := 0; attempt <= 3; attempt++ {
		_, err := p.db.ExecContext(context.Background(), query, name, value)
		if err == nil {
			return nil
		}
		lastErr = err
		if isRetriablePgError(err) {
			return fmt.Errorf("Non retriable database error:%w", err)
		}
		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}
	return fmt.Errorf("database operation failed after 3 retries: %w", lastErr)
}

func (p *PostgresStorage) UpdateCounter(name string, value int64) error {

	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	query := `
		INSERT INTO metrics (id, type, value)
		VALUES ($1, 'counter', $2)
		ON CONFLICT (id)
		DO UPDATE SET value = metrics.value + $2
	`
	for attempt := 0; attempt <= 3; attempt++ {
		_, err := p.db.ExecContext(context.Background(), query, name, float64(value))
		if err == nil {
			return nil
		}
		lastErr = err
		if isRetriablePgError(err) {
			return fmt.Errorf("Non retriable database error:%w", err)
		}
		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}
	return fmt.Errorf("database operation failed after 3 retries: %w", lastErr)
}

func (p *PostgresStorage) GetGauge(name string) (float64, bool) {
	query := `SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`

	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		var value float64
		err := p.db.QueryRowContext(context.Background(), query, name).Scan(&value)
		if err == nil {
			return value, true
		}
		if err == sql.ErrNoRows {
			return 0, false
		}

		lastErr = err

		if !isRetriablePgError(err) {
			log.Printf("Non-retriable error getting gauge %s: %v", name, err)
			return 0, false
		}

		log.Printf("Retriable error getting gauge %s (attempt %d): %v", name, attempt+1, err)

		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}
	log.Printf("Failed to get gauge %s after 3 retries: %v", name, lastErr)
	return 0, false
}

func (p *PostgresStorage) GetCounter(name string) (int64, bool) {
	query := `SELECT value FROM metrics WHERE id = $1 AND type = 'counter'`
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		var valueFloat float64
		err := p.db.QueryRowContext(context.Background(), query, name).Scan(&valueFloat)

		if err == nil {
			return int64(valueFloat), true
		}
		if err == sql.ErrNoRows {
			return 0, false
		}

		lastErr = err

		if !isRetriablePgError(err) {
			log.Printf("Non-retriable error getting counter %s: %v", name, err)
			return 0, false
		}

		log.Printf("Retriable error getting counter %s (attempt %d): %v", name, attempt+1, err)

		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}
	log.Printf("Failed to get counter %s after 3 retries: %v", name, lastErr)
	return 0, false
}

func (p *PostgresStorage) UpdateBatch(metrics []models.Metrics) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		err := p.executeBatchTx(context.Background(), metrics)

		if err == nil {
			return nil
		}

		lastErr = err

		if !isRetriablePgError(err) {
			return fmt.Errorf("non-retriable database error: %w", err)
		}

		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}

	return fmt.Errorf("database operation failed after 3 retries: %w", lastErr)
}

func (p *PostgresStorage) executeBatchTx(ctx context.Context, metrics []models.Metrics) (err error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("Warning: failed to rollback transaction: %v", rollbackErr)
			}
		}
	}()

	stmtGauge, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, type, value) VALUES ($1, 'gauge', $2)
		ON CONFLICT (id) DO UPDATE SET value = $2
	`)
	if err != nil {
		return err
	}
	defer stmtGauge.Close()

	stmtCounter, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, type, value) VALUES ($1, 'counter', $2)
		ON CONFLICT (id) DO UPDATE SET value = metrics.value + $2
	`)
	if err != nil {
		return err
	}
	defer stmtCounter.Close()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				if _, err := stmtGauge.ExecContext(ctx, metric.ID, *metric.Value); err != nil {
					return err
				}
			}
		case models.Counter:
			if metric.Delta != nil {
				if _, err := stmtCounter.ExecContext(ctx, metric.ID, float64(*metric.Delta)); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func isRetriablePgError(err error) bool {

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "08")
	}

	var netErr interface {
		Timeout() bool
		Temporary() bool
	}
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}
