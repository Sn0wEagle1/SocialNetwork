// internal/db.go
package internal

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	var err error
	connStr := "host=localhost port=5432 user=postgres password=Hfnfneq2005 dbname=SocialNetwork sslmode=disable"
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	// Проверка соединения
	err = DB.Ping()
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	fmt.Println("Успешное подключение к БД!")
}
