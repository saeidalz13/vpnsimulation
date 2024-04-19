package db

import (
	"database/sql"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

const (
	maxOpenConns = 300
	maxIdleConns = 100
	connMaxLife  = time.Minute * 15
)

func mustExecMigrations(db *sql.DB, migrationDir string) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		panic(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationDir,
		// databaseName (random string for logging)
		"zapvpn",
		// Driver
		driver,
	)
	if err != nil {
		panic(err)
	}
	// or m.Step(2) if you want to explicitly set the number of migrations to run
	if err = m.Up(); err != nil {
		if err.Error() == "no change" {
			// If no new migration, just start the server
			return
		}
		panic(err)
	}

	log.Println("migration successful...")
}

func MustConnectToPsql(psqlUrl string) *sql.DB {
	db, err := sql.Open("postgres", psqlUrl)
	if err != nil {
		panic(err)
	}

	// ping db to check connection
	if err := db.Ping(); err != nil {
		panic(err)
	}

	// set db pool custom configs
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLife)

	// migrate
	mustExecMigrations(db, "file:db/migration")

	log.Println("connected to database...")
	return db
}
