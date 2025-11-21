package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	dbHost := getEnv("DB_HOST", "postgres")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "video_processing")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)
	log.Println("Connecting to database with connection string:", connStr)
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")
	return createTables()
}

func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS videos (
		id VARCHAR(255) PRIMARY KEY,
		original_name VARCHAR(500) NOT NULL,
		s3_path TEXT NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		source_height INTEGER DEFAULT 0,
		source_width INTEGER DEFAULT 0,
		duration FLOAT DEFAULT 0,
		file_size BIGINT DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		error_message TEXT
	);

	CREATE TABLE IF NOT EXISTS resolutions (
		id VARCHAR(255) PRIMARY KEY,
		video_id VARCHAR(255) NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
		height INTEGER NOT NULL,
		width INTEGER NOT NULL,
		s3_key TEXT NOT NULL,
		s3_url TEXT NOT NULL,
		file_size BIGINT DEFAULT 0,
		bitrate INTEGER DEFAULT 0,
		processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
	CREATE INDEX IF NOT EXISTS idx_resolutions_video_id ON resolutions(video_id);
	`

	_, err := DB.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database tables created successfully")
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
