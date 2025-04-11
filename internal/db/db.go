package db

import (
	"fmt"
	"log"
	"strings"

	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/Suhaibinator/SProto/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database connection instance
var DB *gorm.DB

// Init initializes the database connection and runs migrations based on config.
func Init(cfg config.Config) (*gorm.DB, error) { // Updated signature
	var err error
	var dialector gorm.Dialector // Use interface for flexibility
	dbType := strings.ToLower(cfg.DbType)
	log.Printf("Initializing database connection: type=%s", dbType)

	switch dbType {
	case "postgres":
		if cfg.DbDsn == "" {
			return nil, fmt.Errorf("DB_DSN must be set for postgres database type")
		}
		dialector = postgres.Open(cfg.DbDsn)
		// Avoid logging potentially sensitive DSN in production logs
		log.Printf("Using PostgreSQL DSN (details omitted for security)")
	case "sqlite":
		if cfg.SqlitePath == "" {
			return nil, fmt.Errorf("SQLITE_PATH must be set for sqlite database type")
		}
		// Ensure the directory for the SQLite file exists (optional but good practice)
		// dir := filepath.Dir(cfg.SqlitePath)
		// if err := os.MkdirAll(dir, 0755); err != nil {
		// 	 log.Printf("Failed to create directory for SQLite database: %v", err)
		// 	 return nil, fmt.Errorf("failed to create directory for SQLite DB: %w", err)
		// }
		dialector = sqlite.Open(cfg.SqlitePath)
		log.Printf("Using SQLite database file: %s", cfg.SqlitePath)
	default:
		return nil, fmt.Errorf("invalid DB_TYPE: %s. Must be 'postgres' or 'sqlite'", cfg.DbType)
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Log SQL queries
	})

	if err != nil {
		log.Printf("Failed to connect to database (%s): %v", dbType, err)
		return nil, fmt.Errorf("failed to connect database (%s): %w", dbType, err)
	}

	log.Printf("Database connection established (%s).", dbType)

	// Run migrations
	log.Println("Running database migrations...")
	err = DB.AutoMigrate(&models.Module{}, &models.ModuleVersion{})
	if err != nil {
		log.Printf("Failed to migrate database (%s): %v", dbType, err)
		return nil, fmt.Errorf("failed to migrate database (%s): %w", dbType, err)
	}
	log.Println("Database migrations completed.")

	// Optional: Enable uuid-ossp extension if not already enabled - ONLY FOR POSTGRES
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

// SetDB is a test helper function to replace the global DB instance with a mock.
// !! Use only in tests !!
func SetDB(mockDB *gorm.DB) {
	DB = mockDB
}
