// internal/config.go
package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

type Config struct {
	DBHost     string `json:"DBHost"`
	DBPort     string `json:"DBPort"`
	DBUser     string `json:"DBUser"`
	DBPassword string `json:"DBPassword"`
	DBName     string `json:"DBName"`
	SSLMode    string `json:"SSLMode"`
}

var AppConfig Config

func InitConfig(filePath string) {
	// Чтение конфигурационного файла
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal("Ошибка чтения конфигурационного файла:", err)
	}

	// Десериализация JSON в структуру Config
	err = json.Unmarshal(data, &AppConfig)
	if err != nil {
		log.Fatal("Ошибка при парсинге конфигурационного файла:", err)
	}

	fmt.Println("Конфигурация загружена успешно!")
}
