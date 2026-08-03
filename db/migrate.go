// Package db owns the news schema.
//
// It lives beside the SQL because go:embed cannot reach up out of a package directory,
// and mirrors anime-api's layout so the two are recognisable to each other.
package db

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/httpfs"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/store"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const sourceName = "news-embed"

var registerOnce sync.Once

type embedded struct{ httpfs.PartialDriver }

func (d *embedded) Open(string) (source.Driver, error) {
	if err := d.PartialDriver.Init(http.FS(migrationFiles), "migrations"); err != nil {
		return nil, err
	}
	return d, nil
}

func newMigrator(cfg config.DBConfig) (*migrate.Migrate, error) {
	st, err := store.Open(cfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := st.DB()
	if err != nil {
		return nil, err
	}
	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{
		DatabaseName:    cfg.DataBase,
		MigrationsTable: cfg.MigrationTableName,
	})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}
	// Register exactly once per process: golang-migrate PANICS on a duplicate driver name,
	// so a second call to MigrateUp — a retry, or a test suite that migrates more than once —
	// would crash rather than return an error.
	registerOnce.Do(func() { source.Register(sourceName, &embedded{}) })
	return migrate.NewWithDatabaseInstance(sourceName+"://", cfg.DataBase, driver)
}

// alreadyApplied is the highest migration that anime-api had already run before ownership
// of these tables moved here. Migrations 1-3 are verbatim copies of its 37, 38 and 39, so on
// every existing database that work is done — re-running them would fail on CREATE TABLE
// against tables full of live rows.
const alreadyApplied = 3

// adoptIfInherited marks the copied migrations as applied WITHOUT running them, when this
// database already has the tables but no news migration history.
//
// This is the one-off handover from anime-api, and it runs automatically rather than as a
// separate command someone has to remember. Requiring `adopt` before `migrate` looked
// tidier but was a trap: the deployment runs `migrate`, which would fail on CREATE TABLE
// against live tables AND leave the migration table dirty — after which adopt refused,
// because history now existed. One command that decides for itself cannot be run in the
// wrong order.
func adoptIfInherited(m *migrate.Migrate, cfg config.DBConfig) error {
	_, _, err := m.Version()
	if err != migrate.ErrNilVersion {
		return err // has history (or a real error): nothing to adopt
	}
	inherited, err := newsTableExists(cfg)
	if err != nil {
		return err
	}
	if !inherited {
		return nil // genuinely fresh: let the migrations create everything
	}
	slog.Info("adopting news tables previously owned by anime-api",
		"marking_applied_through", alreadyApplied)
	return m.Force(alreadyApplied)
}

func newsTableExists(cfg config.DBConfig) (bool, error) {
	st, err := store.Open(cfg)
	if err != nil {
		return false, err
	}
	var n int
	err = st.Raw(
		"SELECT COUNT(*) FROM information_schema.tables "+
			"WHERE table_schema = ? AND table_name = 'anime_news'", cfg.DataBase).
		Scan(&n).Error
	return n > 0, err
}

// MigrateUp applies pending migrations. Safe to run repeatedly.
func MigrateUp(cfg config.DBConfig) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	if err := adoptIfInherited(m, cfg); err != nil {
		return err
	}
	version, dirty, _ := m.Version()
	slog.Info("migrating news schema", "from_version", version, "dirty", dirty,
		"table", cfg.MigrationTableName)

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	version, dirty, _ = m.Version()
	slog.Info("news schema up to date", "version", version, "dirty", dirty)
	return nil
}

// MigrateDown rolls back one step, not everything.
//
// A full Down() would drop the news tables — on a database shared with anime-api, holding
// every item the research pipeline has ever published. One step at a time is recoverable;
// dropping the lot on a typo is not.
func MigrateDown(cfg config.DBConfig) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
