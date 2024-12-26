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
	// Строка подключения с использованием конфигурации
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		AppConfig.DBHost, AppConfig.DBPort, AppConfig.DBUser, AppConfig.DBPassword, AppConfig.DBName, AppConfig.SSLMode)

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
