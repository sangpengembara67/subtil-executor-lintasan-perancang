package database

import (
	"fmt"
	"log"
	"strings"
)

// MigrationVersion represents the current database schema version
const CurrentMigrationVersion = 3

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

// GetMigrations returns all available migrations
func GetMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create initial tables and indexes",
			UpSQL: `
				-- Create migration_history table to track applied migrations
				CREATE TABLE IF NOT EXISTS migration_history (
					id INT AUTO_INCREMENT PRIMARY KEY,
					version INT NOT NULL UNIQUE,
					description VARCHAR(255) NOT NULL,
					applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);

				-- Create chat_analysis table
				CREATE TABLE IF NOT EXISTS chat_analysis (
					id INT AUTO_INCREMENT PRIMARY KEY,
					chat_id VARCHAR(255) NOT NULL,
					platform ENUM('whatsapp', 'telegram') NOT NULL,
					message_count INT DEFAULT 0,
					stock_mentions JSON,
					sentiment_analysis JSON,
					analysis_summary TEXT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
					INDEX idx_chat_platform (chat_id, platform),
					INDEX idx_created_at (created_at)
				);

				-- Create stock_analysis table
				CREATE TABLE IF NOT EXISTS stock_analysis (
					id INT AUTO_INCREMENT PRIMARY KEY,
					stock_code VARCHAR(10) NOT NULL,
					analysis TEXT,
					prediction TEXT,
					confidence DECIMAL(5,4),
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					INDEX idx_stock_code (stock_code),
					INDEX idx_created_at (created_at)
				);

				-- Create schedules table
				CREATE TABLE IF NOT EXISTS schedules (
					id INT AUTO_INCREMENT PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					description TEXT,
					chat_id VARCHAR(255) NOT NULL,
					target_group VARCHAR(255),
					schedule_time VARCHAR(10) NOT NULL,
					days JSON NOT NULL,
					active BOOLEAN DEFAULT TRUE,
					last_run TIMESTAMP NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
				);

				-- Create users table
				CREATE TABLE IF NOT EXISTS users (
					id INT AUTO_INCREMENT PRIMARY KEY,
					username VARCHAR(50) UNIQUE NOT NULL,
					password VARCHAR(255) NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
				);

				-- Create stock_mentions table
				CREATE TABLE IF NOT EXISTS stock_mentions (
					id INT AUTO_INCREMENT PRIMARY KEY,
					chat_id VARCHAR(255) NOT NULL,
					platform ENUM('whatsapp', 'telegram') NOT NULL,
					stock_code VARCHAR(10) NOT NULL,
					original_text VARCHAR(255) NOT NULL,
					corrected_code VARCHAR(10) NOT NULL,
					confidence DECIMAL(5,4) NOT NULL,
					context TEXT,
					sentiment ENUM('positive', 'negative', 'neutral') DEFAULT 'neutral',
					timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					INDEX idx_chat_platform (chat_id, platform),
					INDEX idx_stock_code (stock_code),
					INDEX idx_timestamp (timestamp)
				);

				-- Create user_sessions table for session management
				CREATE TABLE IF NOT EXISTS user_sessions (
					id INT AUTO_INCREMENT PRIMARY KEY,
					user_id INT NOT NULL,
					session_token VARCHAR(255) UNIQUE NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					expires_at TIMESTAMP NOT NULL,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
					INDEX idx_session_token (session_token),
					INDEX idx_expires_at (expires_at)
				);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS user_sessions;
				DROP TABLE IF EXISTS stock_mentions;
				DROP TABLE IF EXISTS users;
				DROP TABLE IF EXISTS schedules;
				DROP TABLE IF EXISTS stock_analysis;
				DROP TABLE IF EXISTS chat_analysis;
				DROP TABLE IF EXISTS migration_history;
			`,
		},
		{
			Version:     2,
			Description: "Add name and description columns to schedules table",
			UpSQL: `
				-- Add missing columns to schedules table
				ALTER TABLE schedules 
				ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT 'Default Schedule',
				ADD COLUMN description TEXT;
			`,
			DownSQL: `
				ALTER TABLE schedules 
				DROP COLUMN name,
				DROP COLUMN description;
			`,
		},
		{
			Version:     3,
			Description: "Fix table name inconsistency - rename schedules to schedule_config",
			UpSQL: `
				-- Check if schedules table exists and schedule_config doesn't
				SET @table_exists = (SELECT COUNT(*) FROM information_schema.tables 
					WHERE table_schema = DATABASE() AND table_name = 'schedules');
				SET @target_exists = (SELECT COUNT(*) FROM information_schema.tables 
					WHERE table_schema = DATABASE() AND table_name = 'schedule_config');

				-- Create schedule_config table with correct structure
				CREATE TABLE IF NOT EXISTS schedule_config (
					id INT AUTO_INCREMENT PRIMARY KEY,
					group_id VARCHAR(255) NOT NULL,
					platform ENUM('whatsapp', 'telegram') NOT NULL,
					schedule_time TIME NOT NULL,
					days_of_week JSON NOT NULL,
					active BOOLEAN DEFAULT TRUE,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
					UNIQUE KEY unique_group_platform (group_id, platform)
				);

				-- Migrate data from schedules to schedule_config if schedules exists
				SET @sql = IF(@table_exists > 0 AND @target_exists = 0,
					'INSERT INTO schedule_config (group_id, platform, schedule_time, days_of_week, active, created_at, updated_at) SELECT COALESCE(target_group, chat_id) as group_id, "telegram" as platform, CAST(schedule_time AS TIME) as schedule_time, days as days_of_week, active, created_at, updated_at FROM schedules',
					'SELECT 1');
				PREPARE stmt FROM @sql;
				EXECUTE stmt;
				DEALLOCATE PREPARE stmt;

				-- Drop schedules table if it exists and data was migrated
				SET @drop_sql = IF(@table_exists > 0, 'DROP TABLE IF EXISTS schedules', 'SELECT 1');
				PREPARE drop_stmt FROM @drop_sql;
				EXECUTE drop_stmt;
				DEALLOCATE PREPARE drop_stmt;
			`,
			DownSQL: `
				-- Recreate schedules table
				CREATE TABLE IF NOT EXISTS schedules (
					id INT AUTO_INCREMENT PRIMARY KEY,
					name VARCHAR(255) NOT NULL DEFAULT 'Default Schedule',
					description TEXT,
					chat_id VARCHAR(255) NOT NULL,
					target_group VARCHAR(255),
					schedule_time VARCHAR(10) NOT NULL,
					days JSON NOT NULL,
					active BOOLEAN DEFAULT TRUE,
					last_run TIMESTAMP NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
				);

				-- Drop schedule_config table
				DROP TABLE IF EXISTS schedule_config;
			`,
		},
	}
}

// RunMigrations executes all pending migrations
func RunMigrations() error {
	log.Println("Starting database migrations...")

	// Ensure migration_history table exists
	if err := createMigrationHistoryTable(); err != nil {
		return fmt.Errorf("failed to create migration_history table: %v", err)
	}

	// Get current migration version
	currentVersion, err := getCurrentMigrationVersion()
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %v", err)
	}

	log.Printf("Current migration version: %d", currentVersion)

	// Get all migrations
	migrations := GetMigrations()

	// Execute pending migrations
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			log.Printf("Applying migration %d: %s", migration.Version, migration.Description)

			if err := executeMigration(migration); err != nil {
				return fmt.Errorf("failed to execute migration %d: %v", migration.Version, err)
			}

			log.Printf("Migration %d applied successfully", migration.Version)
		}
	}

	log.Println("All migrations completed successfully")
	return nil
}

// createMigrationHistoryTable creates the migration_history table if it doesn't exist
func createMigrationHistoryTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migration_history (
			id INT AUTO_INCREMENT PRIMARY KEY,
			version INT NOT NULL UNIQUE,
			description VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := DB.Exec(query)
	return err
}

// getCurrentMigrationVersion returns the current migration version
func getCurrentMigrationVersion() (int, error) {
	var version int
	err := DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migration_history").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// executeMigration executes a single migration
func executeMigration(migration Migration) error {
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clean and parse SQL statements properly
	sql := strings.TrimSpace(migration.UpSQL)
	statements := parseSQLStatements(sql)
	
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		log.Printf("Executing statement %d: %s...", i+1, stmt[:min(100, len(stmt))])
		_, err := tx.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement %d: %v\nSQL: %s", i+1, err, stmt)
		}
	}

	// Record migration in history
	_, err = tx.Exec(
		"INSERT INTO migration_history (version, description) VALUES (?, ?)",
		migration.Version, migration.Description,
	)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit()
}

// parseSQLStatements parses SQL text and returns individual statements
func parseSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	lines := strings.Split(sql, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		
		// Add line to current statement
		current.WriteString(line)
		current.WriteString(" ")
		
		// If line ends with semicolon, we have a complete statement
		if strings.HasSuffix(line, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}
	
	// Add any remaining statement
	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	
	return statements
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckSchemaIntegrity verifies that all required tables and columns exist
func CheckSchemaIntegrity() error {
	log.Println("Checking database schema integrity...")

	requiredTables := []string{
		"migration_history",
		"chat_analysis",
		"stock_analysis",
		"schedule_config",
		"users",
		"stock_mentions",
		"user_sessions",
	}

	for _, table := range requiredTables {
		if exists, err := tableExists(table); err != nil {
			return fmt.Errorf("failed to check table %s: %v", table, err)
		} else if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	log.Println("Schema integrity check passed")
	return nil
}

// tableExists checks if a table exists in the database
func tableExists(tableName string) (bool, error) {
	var count int
	err := DB.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		tableName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RollbackMigration rolls back to a specific migration version
func RollbackMigration(targetVersion int) error {
	currentVersion, err := getCurrentMigrationVersion()
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %v", err)
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version %d is not less than current version %d", targetVersion, currentVersion)
	}

	migrations := GetMigrations()

	// Execute rollback migrations in reverse order
	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if migration.Version > targetVersion && migration.Version <= currentVersion {
			log.Printf("Rolling back migration %d: %s", migration.Version, migration.Description)

			if err := executeRollback(migration); err != nil {
				return fmt.Errorf("failed to rollback migration %d: %v", migration.Version, err)
			}

			log.Printf("Migration %d rolled back successfully", migration.Version)
		}
	}

	return nil
}

// executeRollback executes a migration rollback
func executeRollback(migration Migration) error {
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute rollback SQL
	statements := strings.Split(migration.DownSQL, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err := tx.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute rollback statement '%s': %v", stmt, err)
		}
	}

	// Remove migration from history
	_, err = tx.Exec("DELETE FROM migration_history WHERE version = ?", migration.Version)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit()
}