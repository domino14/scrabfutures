package marketapi

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
)

type Config struct {
	DBMigrationsPath string
	DBPath           string
}

func EnsureMigrations(cfg *Config) {
	sqliteDb, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		panic(err)
	}
	driver, err := sqlite3.WithInstance(sqliteDb, &sqlite3.Config{})
	if err != nil {
		panic(err)
	}
	m, err := migrate.NewWithDatabaseInstance(cfg.DBMigrationsPath, cfg.DBPath, driver)
	if err != nil {
		panic(err)
	}
	log.Info().Msg("bringing up migration")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		panic(err)
	}
	e1, e2 := m.Close()
	log.Err(e1).Msg("close-source")
	log.Err(e2).Msg("close-database")
}
