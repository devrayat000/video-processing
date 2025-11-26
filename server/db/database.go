package db

import (
	"fmt"
	"log"
	"os"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/devrayat000/video-process/models"
	server_utils "github.com/devrayat000/video-process/utils"
)

var (
	dbHost                 = server_utils.GetEnv("DB_HOST", "localhost")
	dbPort                 = server_utils.GetEnv("DB_PORT", "5555")
	dbUser                 = server_utils.GetEnv("DB_USER", "user")
	dbPass                 = server_utils.GetEnv("DB_PASSWORD", "password")
	dbName                 = server_utils.GetEnv("DB_NAME", "videodb")
	instanceConnectionName = os.Getenv("INSTANCE_CONNECTION_NAME")
)

func InitDB() (*gorm.DB, error) {
	var (
		conn    gorm.Dialector
		connStr string
	)
	if instanceConnectionName != "" {
		connStr = fmt.Sprintf("host=%s user=%s password=%s dbname=%s",
			instanceConnectionName, dbUser, dbPass, dbName)
		conn = postgres.New(postgres.Config{
			DriverName: "cloudsqlpostgres",
			DSN:        connStr,
		})
	} else {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPass, dbName)
		conn = postgres.Open(connStr)
	}

	log.Println("Connecting to database with connection string:", connStr)

	gormDB, err := gorm.Open(conn, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB from gorm: %w", err)
	}
	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")

	if err = gormDB.AutoMigrate(&models.Video{}, &models.VideoResolution{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database schema: %w", err)
	}

	return gormDB, nil
}
