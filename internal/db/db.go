package db

import (
	"fmt"
	"log" // Using standard log for simplicity, could switch to Zap later

	"github.com/Suhaibinator/SProto/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database connection instance
var DB *gorm.DB

// Init initializes the database connection and runs migrations.
func Init(dsn string) (*gorm.DB, error) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Log SQL queries
	})

	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	log.Println("Database connection established.")

	// Run migrations
	log.Println("Running database migrations...")
	err = DB.AutoMigrate(&models.Module{}, &models.ModuleVersion{})
	if err != nil {
		log.Printf("Failed to migrate database: %v", err)
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	log.Println("Database migrations completed.")

	// Optional: Enable uuid-ossp extension if not already enabled
	// You might need to run this manually or ensure the DB user has permissions
	// result := DB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
	// if result.Error != nil {
	//  log.Printf("Warning: Failed to ensure uuid-ossp extension exists: %v", result.Error)
	// }

	return DB, nil
}

// GetDB returns the initialized database instance.
// Panics if Init has not been called successfully.
func GetDB() *gorm.DB {
	if DB == nil {
		log.Fatal("Database has not been initialized. Call db.Init first.")
	}
	return DB
}
