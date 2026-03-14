package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// GetDB returns the database connection
func GetDB() *sql.DB {
	return DB
}

// ChatAnalysis represents the structure for storing chat analysis results
type ChatAnalysis struct {
	ID            int       `json:"id"`
	ChatID        string    `json:"chat_id"`
	Platform      string    `json:"platform"` // "whatsapp" or "telegram"
	MessageID     string    `json:"message_id"`
	SenderID      string    `json:"sender_id"`
	SenderName    string    `json:"sender_name"`
	MessageType   string    `json:"message_type"` // "text", "image", "document", "pdf"
	Content       string    `json:"content"`
	ExtractedText string    `json:"extracted_text"` // For images/PDFs
	AIAnalysis    string    `json:"ai_analysis"`
	StockMentions string    `json:"stock_mentions"` // JSON array of detected stock codes
	StockAnalysis string    `json:"stock_analysis"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StockAnalysis represents stock analysis results
type StockAnalysis struct {
	ID           int       `json:"id"`
	StockCode    string    `json:"stock_code"`
	OriginalCode string    `json:"original_code"` // Before typo correction
	Analysis     string    `json:"analysis"`
	Prediction   string    `json:"prediction"`
	Confidence   float64   `json:"confidence"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScheduleConfig represents scheduled analysis delivery configuration
type ScheduleConfig struct {
	ID           int       `json:"id"`
	GroupID      string    `json:"group_id"`
	Platform     string    `json:"platform"`
	ScheduleTime string    `json:"schedule_time"` // Format: "19:00"
	DaysOfWeek   string    `json:"days_of_week"`  // JSON array: ["monday", "tuesday", ...]
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// User represents user authentication
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // Hashed
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InitDatabase initializes the MySQL database connection
func InitDatabase() error {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		return fmt.Errorf("database configuration incomplete")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=true",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test connection
	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Set connection pool settings
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Database connected successfully")

	// Run migrations instead of createTables
	if err := RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	// Run seeding for essential data (admin user)
	if err := RunSpecificSeeder("admin_user"); err != nil {
		log.Printf("Warning: Failed to seed admin user: %v", err)
	}

	return nil
}

// createTables creates the necessary database tables
func createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS chat_analysis (
			id INT AUTO_INCREMENT PRIMARY KEY,
			chat_id VARCHAR(255) NOT NULL,
			platform ENUM('whatsapp', 'telegram') NOT NULL,
			message_id VARCHAR(255) NOT NULL,
			sender_id VARCHAR(255) NOT NULL,
			sender_name VARCHAR(255) NOT NULL,
			message_type ENUM('text', 'image', 'document', 'pdf') NOT NULL,
			content TEXT,
			extracted_text TEXT,
			ai_analysis TEXT,
			stock_mentions JSON,
			stock_analysis TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_chat_platform (chat_id, platform),
			INDEX idx_created_at (created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_analysis (
			id INT AUTO_INCREMENT PRIMARY KEY,
			stock_code VARCHAR(10) NOT NULL,
			original_code VARCHAR(10) NOT NULL,
			analysis TEXT,
			prediction TEXT,
			confidence DECIMAL(5,4),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_stock_code (stock_code),
			INDEX idx_created_at (created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS schedule_config (
			id INT AUTO_INCREMENT PRIMARY KEY,
			group_id VARCHAR(255) NOT NULL,
			platform ENUM('whatsapp', 'telegram') NOT NULL,
			schedule_time TIME NOT NULL,
			days_of_week JSON NOT NULL,
			active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY unique_group_platform (group_id, platform)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stock_mentions (
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
		)`,
	}

	for _, table := range tables {
		if _, err := DB.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Insert default admin user if not exists
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %v", err)
	}

	if count == 0 {
		// Hash password (simple hash for demo - use bcrypt in production)
		hashedPassword := hashPassword("shiina")
		_, err = DB.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "admin", hashedPassword)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %v", err)
		}
		log.Println("Default admin user created")
	}

	return nil
}

// Simple password hashing (use bcrypt in production)
func hashPassword(password string) string {
	// This is a simple hash for demo purposes
	// In production, use bcrypt.GenerateFromPassword
	return fmt.Sprintf("%x", password) // Simple hex encoding
}

// SaveChatAnalysis saves chat analysis to database
func SaveChatAnalysis(analysis *ChatAnalysis) error {
	query := `INSERT INTO chat_analysis 
		(chat_id, platform, message_id, sender_id, sender_name, message_type, 
		 content, extracted_text, ai_analysis, stock_mentions, stock_analysis) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DB.Exec(query, analysis.ChatID, analysis.Platform, analysis.MessageID,
		analysis.SenderID, analysis.SenderName, analysis.MessageType,
		analysis.Content, analysis.ExtractedText, analysis.AIAnalysis,
		analysis.StockMentions, analysis.StockAnalysis)

	return err
}

// GetChatAnalysisByDate gets chat analysis for a specific date range
func GetChatAnalysisByDate(startDate, endDate time.Time) ([]ChatAnalysis, error) {
	query := `SELECT id, chat_id, platform, message_id, sender_id, sender_name, 
			message_type, content, extracted_text, ai_analysis, stock_mentions, 
			stock_analysis, created_at, updated_at 
			FROM chat_analysis 
			WHERE created_at BETWEEN ? AND ? 
			ORDER BY created_at DESC`

	rows, err := DB.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analyses []ChatAnalysis
	for rows.Next() {
		var analysis ChatAnalysis
		err := rows.Scan(&analysis.ID, &analysis.ChatID, &analysis.Platform,
			&analysis.MessageID, &analysis.SenderID, &analysis.SenderName,
			&analysis.MessageType, &analysis.Content, &analysis.ExtractedText,
			&analysis.AIAnalysis, &analysis.StockMentions, &analysis.StockAnalysis,
			&analysis.CreatedAt, &analysis.UpdatedAt)
		if err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}

	return analyses, nil
}

// SaveStockAnalysis saves stock analysis to database
func SaveStockAnalysis(analysis *StockAnalysis) error {
	query := `INSERT INTO stock_analysis (stock_code, original_code, analysis, prediction, confidence) 
			VALUES (?, ?, ?, ?, ?)`

	_, err := DB.Exec(query, analysis.StockCode, analysis.OriginalCode,
		analysis.Analysis, analysis.Prediction, analysis.Confidence)

	return err
}

// GetScheduleConfigs gets all active schedule configurations
func GetScheduleConfigs() ([]ScheduleConfig, error) {
	query := `SELECT id, group_id, platform, schedule_time, days_of_week, active, created_at, updated_at 
			FROM schedule_config WHERE active = TRUE`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []ScheduleConfig
	for rows.Next() {
		var config ScheduleConfig
		err := rows.Scan(&config.ID, &config.GroupID, &config.Platform,
			&config.ScheduleTime, &config.DaysOfWeek, &config.Active,
			&config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// SaveScheduleConfig saves or updates schedule configuration
func SaveScheduleConfig(config *ScheduleConfig) error {
	query := `INSERT INTO schedule_config (group_id, platform, schedule_time, days_of_week, active) 
			VALUES (?, ?, ?, ?, ?) 
			ON DUPLICATE KEY UPDATE 
			schedule_time = VALUES(schedule_time), 
			days_of_week = VALUES(days_of_week), 
			active = VALUES(active)`

	_, err := DB.Exec(query, config.GroupID, config.Platform, config.ScheduleTime,
		config.DaysOfWeek, config.Active)

	return err
}

// AuthenticateUser authenticates user login
func AuthenticateUser(username, password string) (*User, error) {
	hashedPassword := hashPassword(password)
	query := `SELECT id, username, password, created_at, updated_at FROM users WHERE username = ? AND password = ?`

	var user User
	err := DB.QueryRow(query, username, hashedPassword).Scan(
		&user.ID, &user.Username, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, err
	}

	return &user, nil
}

// UpdateUserPassword updates user password
func UpdateUserPassword(userID int, newPassword string) error {
	hashedPassword := hashPassword(newPassword)
	query := `UPDATE users SET password = ? WHERE id = ?`

	_, err := DB.Exec(query, hashedPassword, userID)
	return err
}

// CreateUserSession creates a new user session
func CreateUserSession(userID int, sessionToken string) error {
	// For simplicity, we'll store sessions in memory or use a simple approach
	// In production, you should use a proper session store
	return nil // Placeholder implementation
}

// DeleteUserSession deletes a user session
func DeleteUserSession(sessionToken string) error {
	// For simplicity, we'll store sessions in memory or use a simple approach
	// In production, you should use a proper session store
	return nil // Placeholder implementation
}

// IsValidSession checks if a session token is valid
func IsValidSession(sessionToken string) bool {
	// For simplicity, we'll always return true for non-empty tokens
	// In production, you should implement proper session validation
	return sessionToken != ""
}

// GetUserBySession gets user by session token
func GetUserBySession(sessionToken string) (*User, error) {
	// For simplicity, return admin user for any valid session
	// In production, you should implement proper session-to-user mapping
	if sessionToken == "" {
		return nil, fmt.Errorf("invalid session")
	}

	query := `SELECT id, username, password, created_at, updated_at FROM users WHERE username = ?`
	var user User
	err := DB.QueryRow(query, "admin").Scan(
		&user.ID, &user.Username, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// VerifyPassword verifies if the provided password matches the user's password
func VerifyPassword(userID int, password string) bool {
	var hashedPassword string
	query := "SELECT password FROM users WHERE id = ?"
	err := DB.QueryRow(query, userID).Scan(&hashedPassword)
	if err != nil {
		return false
	}
	return hashPassword(password) == hashedPassword
}

// UpdateUsername updates the username for a user
func UpdateUsername(userID int, newUsername string) error {
	query := "UPDATE users SET username = ?, updated_at = NOW() WHERE id = ?"
	_, err := DB.Exec(query, newUsername, userID)
	return err
}

// GetAnalytics returns analytics data for a date range
func GetAnalytics(startDate, endDate time.Time) (map[string]interface{}, error) {
	analytics := make(map[string]interface{})
	
	// Get total messages count
	var totalMessages int
	query := "SELECT COUNT(*) FROM chat_analysis WHERE created_at BETWEEN ? AND ?"
	err := DB.QueryRow(query, startDate, endDate).Scan(&totalMessages)
	if err != nil {
		return nil, err
	}
	analytics["total_messages"] = totalMessages
	
	// Get messages by platform
	platformQuery := "SELECT platform, COUNT(*) FROM chat_analysis WHERE created_at BETWEEN ? AND ? GROUP BY platform"
	rows, err := DB.Query(platformQuery, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	platformData := make(map[string]int)
	for rows.Next() {
		var platform string
		var count int
		err := rows.Scan(&platform, &count)
		if err != nil {
			return nil, err
		}
		platformData[platform] = count
	}
	analytics["by_platform"] = platformData
	
	return analytics, nil
}

// GetChatAnalysisByChat returns chat analysis for a specific chat
func GetChatAnalysisByChat(chatID, platform string) ([]ChatAnalysis, error) {
	query := `SELECT id, chat_id, platform, message_id, sender_id, sender_name, 
				 message_type, content, extracted_text, ai_analysis, stock_mentions, 
				 stock_analysis, created_at, updated_at 
			  FROM chat_analysis 
			  WHERE chat_id = ? AND platform = ? 
			  ORDER BY created_at DESC`
	
	rows, err := DB.Query(query, chatID, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var analyses []ChatAnalysis
	for rows.Next() {
		var analysis ChatAnalysis
		err := rows.Scan(
			&analysis.ID, &analysis.ChatID, &analysis.Platform, &analysis.MessageID,
			&analysis.SenderID, &analysis.SenderName, &analysis.MessageType,
			&analysis.Content, &analysis.ExtractedText, &analysis.AIAnalysis,
			&analysis.StockMentions, &analysis.StockAnalysis, &analysis.CreatedAt,
			&analysis.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}
	
	return analyses, nil
}

// GetStockAnalysis returns stock analysis data
func GetStockAnalysis(limit int) ([]StockAnalysis, error) {
	query := `SELECT id, stock_code, original_code, analysis, prediction, confidence, created_at 
			  FROM stock_analysis 
			  ORDER BY created_at DESC 
			  LIMIT ?`
	
	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var analyses []StockAnalysis
	for rows.Next() {
		var analysis StockAnalysis
		err := rows.Scan(
			&analysis.ID, &analysis.StockCode, &analysis.OriginalCode,
			&analysis.Analysis, &analysis.Prediction, &analysis.Confidence,
			&analysis.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}
	
	return analyses, nil
}
