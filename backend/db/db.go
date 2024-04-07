package db

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

const (
	maxOpenConns = 300
	maxIdleConns = 100
	connMaxLife  = time.Minute * 15
)

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

	return db
}
