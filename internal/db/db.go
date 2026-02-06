// Package db provides database connectivity and migrations.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds database configuration.
type Config struct {
	URL             string        `toml:"url"`
	MaxConnections  int32         `toml:"max_connections"`
	MinConnections  int32         `toml:"min_connections"`
	MaxConnLifetime time.Duration `toml:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `toml:"max_conn_idle_time"`
}

// DefaultConfig returns sensible defaults for database configuration.
func DefaultConfig() Config {
	return Config{
		URL:             "postgres://localhost:5432/dagryn?sslmode=disable",
		MaxConnections:  25,
		MinConnections:  5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// DB wraps the pgxpool.Pool with additional functionality.
type DB struct {
	pool *pgxpool.Pool
	cfg  Config
}

// New creates a new database connection pool.
func New(ctx context.Context, cfg Config) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Set pool configuration
	if cfg.MaxConnections > 0 {
		poolCfg.MaxConns = cfg.MaxConnections
	}
	if cfg.MinConnections > 0 {
		poolCfg.MinConns = cfg.MinConnections
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	// Add OpenTelemetry tracer
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().
		Int32("max_conns", poolCfg.MaxConns).
		Int32("min_conns", poolCfg.MinConns).
		Msg("Database connection pool created")

	return &DB{
		pool: pool,
		cfg:  cfg,
	}, nil
}

// Pool returns the underlying connection pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Close closes the database connection pool.
func (db *DB) Close() {
	db.pool.Close()
	log.Info().Msg("Database connection pool closed")
}

// Ping verifies the database connection.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// BeginTx starts a new transaction.
func (db *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

// Exec executes a query without returning any rows.
func (db *DB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

// Migration represents a database migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	rows, err := db.pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}

	// Load migrations from embedded files
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		log.Info().Int("version", m.Version).Str("name", m.Name).Msg("Applying migration")

		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %d (%s): %w", m.Version, m.Name, err)
		}

		// Record migration
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		log.Info().Int("version", m.Version).Str("name", m.Name).Msg("Migration applied successfully")
	}

	return nil
}

// MigrateDown rolls back the last migration.
func (db *DB) MigrateDown(ctx context.Context) error {
	// Get the last applied migration
	var version int
	var name string
	err := db.pool.QueryRow(ctx, "SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &name)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Info().Msg("No migrations to roll back")
			return nil
		}
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Look for down migration file
	downSQL, err := loadDownMigration(version)
	if err != nil {
		return fmt.Errorf("no down migration found for version %d: %w", version, err)
	}

	log.Info().Int("version", version).Str("name", name).Msg("Rolling back migration")

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute down migration
	if _, err := tx.Exec(ctx, downSQL); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to execute down migration: %w", err)
	}

	// Remove migration record
	if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	log.Info().Int("version", version).Str("name", name).Msg("Migration rolled back successfully")
	return nil
}

// MigrationStatus returns the status of all migrations.
func (db *DB) MigrationStatus(ctx context.Context) ([]MigrationInfo, error) {
	// Get applied migrations
	rows, err := db.pool.Query(ctx, "SELECT version, name, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]MigrationInfo)
	for rows.Next() {
		var info MigrationInfo
		if err := rows.Scan(&info.Version, &info.Name, &info.AppliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		info.Applied = true
		applied[info.Version] = info
	}

	// Load all migrations
	migrations, err := loadMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	// Build status list
	result := make([]MigrationInfo, 0, len(migrations))
	for _, m := range migrations {
		if info, ok := applied[m.Version]; ok {
			result = append(result, info)
		} else {
			result = append(result, MigrationInfo{
				Version: m.Version,
				Name:    m.Name,
				Applied: false,
			})
		}
	}

	return result, nil
}

// MigrationInfo contains information about a migration.
type MigrationInfo struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// loadMigrations loads all migration files from the embedded filesystem.
func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only process .up.sql files or files without .down.sql suffix
		if strings.HasSuffix(name, ".down.sql") {
			continue
		}

		// Parse version from filename (e.g., "001_users.sql" -> 1)
		var version int
		var migrationName string
		if _, err := fmt.Sscanf(name, "%d_%s", &version, &migrationName); err != nil {
			continue // Skip files that don't match the pattern
		}

		// Remove .sql or .up.sql suffix
		migrationName = strings.TrimSuffix(migrationName, ".up.sql")
		migrationName = strings.TrimSuffix(migrationName, ".sql")

		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    migrationName,
			SQL:     string(content),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// loadDownMigration loads a down migration file.
func loadDownMigration(version int) (string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("%03d_", version)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".down.sql") {
			content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("down migration not found for version %d", version)
}
