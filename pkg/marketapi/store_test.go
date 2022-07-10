package marketapi

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"testing"

	"github.com/matryer/is"
)

var cfg = Config{
	DBMigrationsPath: os.Getenv("DB_MIGRATIONS_PATH"),
	DBPath:           os.Getenv("TEST_DB_PATH"),
}

func initDB() {
	os.Remove(cfg.DBPath)
	EnsureMigrations(&cfg)
}

func addFixtures(fixtureFile string) {
	bts, err := ioutil.ReadFile(fixtureFile)
	if err != nil {
		panic(err)
	}
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	db.BeginTx(ctx, nil)

	_, err = db.ExecContext(ctx, string(bts))
	if err != nil {
		panic(err)
	}
}

func TestFulfillOrder(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, err := NewSqliteStore(cfg.DBPath)
	is.NoErr(err)
	err = s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 50, true)
	is.NoErr(err)
}
