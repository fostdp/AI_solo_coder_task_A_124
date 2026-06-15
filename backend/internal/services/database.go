package services

import (
	"fmt"
	"log"
	"lingqu-dou-gate/internal/config"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

var DB *gorm.DB

func InitDB() {
	cfg := config.AppConfig.DB
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	var err error
	DB, err = gorm.Open("postgres", dsn)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v", err)
		log.Println("Continuing with limited functionality...")
		return
	}

	DB.LogMode(true)
	log.Println("Database connection established successfully")
}

func GetDB() *gorm.DB {
	if DB == nil {
		log.Println("Warning: Database not initialized, attempting reconnection")
		InitDB()
	}
	return DB
}

func CloseDB() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection closed")
	}
}
