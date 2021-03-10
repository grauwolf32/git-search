package database

import (
	"database/sql"
	"fmt"

	"../config"

	"github.com/labstack/echo"
	_ "github.com/lib/pq"
)

type DBContext struct {
	echo.Context
	Db *sql.DB
}

var DB *sql.DB

func Connect() *sql.DB {
	DBCredentials := config.Settings.DBCredentials
	ConnectString := fmt.Sprintf("postgres://%s:%s@localhost/%s?sslmode=disable", DBCredentials.Name, DBCredentials.Password, DBCredentials.Database)

	db, err := sql.Open("postgres", ConnectString)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Connected")
	DB = db
	return db
}
