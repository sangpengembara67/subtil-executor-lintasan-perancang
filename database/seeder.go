package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// SeedData contains all seeding operations
type SeedData struct {
	Name        string
	Description string
	SeedFunc    func() error
}

// GetSeeders returns all available seeders
func GetSeeders() []SeedData {
	return []SeedData{
		{
			Name:        "admin_user",
			Description: "Create default admin user",
			SeedFunc:    seedAdminUser,
		},
		{
			Name:        "demo_users",
			Description: "Create demo users for testing",
			SeedFunc:    seedDemoUsers,
		},
		{
			Name:        "sample_schedule_configs",
			Description: "Create sample schedule configurations",
			SeedFunc:    seedSampleScheduleConfigs,
		},
	}
}

// RunSeeders executes all seeding operations
func RunSeeders() error {
	log.Println("Starting database seeding...")

	seeders := GetSeeders()
	for _, seeder := range seeders {
		log.Printf("Running seeder: %s - %s", seeder.Name, seeder.Description)

		if err := seeder.SeedFunc(); err != nil {
			log.Printf("Warning: Seeder %s failed: %v", seeder.Name, err)
			// Continue with other seeders even if one fails
			continue
		}

		log.Printf("Seeder %s completed successfully", seeder.Name)
	}

	log.Println("Database seeding completed")
	return nil
}

// RunSpecificSeeder runs a specific seeder by name
func RunSpecificSeeder(seederName string) error {
	seeders := GetSeeders()
	for _, seeder := range seeders {
		if seeder.Name == seederName {
			log.Printf("Running seeder: %s - %s", seeder.Name, seeder.Description)
			return seeder.SeedFunc()
		}
	}
	return fmt.Errorf("seeder %s not found", seederName)
}

// seedAdminUser creates the default admin user
func seedAdminUser() error {
	// Check if admin user already exists
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check admin user existence: %v", err)
	}

	if count > 0 {
		log.Println("Admin user already exists, skipping creation")
		return nil
	}

	// Create admin user with default password
	hashedPassword := hashPassword("shiina")
	_, err = DB.Exec(
		"INSERT INTO users (username, password, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
		"admin", hashedPassword,
	)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %v", err)
	}

	log.Println("Default admin user created (username: admin, password: shiina)")
	return nil
}

// seedDemoUsers creates demo users for testing
func seedDemoUsers() error {
	demoUsers := []struct {
		username string
		password string
	}{
		{"demo", "demo123"},
		{"test", "test123"},
		{"user1", "user123"},
	}

	for _, user := range demoUsers {
		// Check if user already exists
		var count int
		err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", user.username).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check user %s existence: %v", user.username, err)
		}

		if count > 0 {
			log.Printf("User %s already exists, skipping creation", user.username)
			continue
		}

		// Create demo user
		hashedPassword := hashPassword(user.password)
		_, err = DB.Exec(
			"INSERT INTO users (username, password, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
			user.username, hashedPassword,
		)
		if err != nil {
			return fmt.Errorf("failed to create demo user %s: %v", user.username, err)
		}

		log.Printf("Demo user created: %s (password: %s)", user.username, user.password)
	}

	return nil
}

// seedSampleScheduleConfigs creates sample schedule configurations
func seedSampleScheduleConfigs() error {
	sampleConfigs := []struct {
		groupID      string
		platform     string
		scheduleTime string
		daysOfWeek   string
		active       bool
	}{
		{
			groupID:      "sample_whatsapp_group",
			platform:     "whatsapp",
			scheduleTime: "09:00:00",
			daysOfWeek:   `["monday", "tuesday", "wednesday", "thursday", "friday"]`,
			active:       true,
		},
		{
			groupID:      "sample_telegram_group",
			platform:     "telegram",
			scheduleTime: "17:00:00",
			daysOfWeek:   `["monday", "wednesday", "friday"]`,
			active:       true,
		},
		{
			groupID:      "weekend_analysis_group",
			platform:     "telegram",
			scheduleTime: "10:00:00",
			daysOfWeek:   `["saturday", "sunday"]`,
			active:       false,
		},
	}

	for _, config := range sampleConfigs {
		// Check if schedule config already exists
		var count int
		err := DB.QueryRow(
			"SELECT COUNT(*) FROM schedule_config WHERE group_id = ? AND platform = ?",
			config.groupID, config.platform,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check schedule config existence: %v", err)
		}

		if count > 0 {
			log.Printf("Schedule config for group %s on %s already exists, skipping creation",
				config.groupID, config.platform)
			continue
		}

		// Create schedule config
		_, err = DB.Exec(`
			INSERT INTO schedule_config 
			(group_id, platform, schedule_time, days_of_week, active, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		`, config.groupID, config.platform, config.scheduleTime, config.daysOfWeek, config.active)
		if err != nil {
			return fmt.Errorf("failed to create schedule config: %v", err)
		}

		log.Printf("Sample schedule config created: %s on %s at %s",
			config.groupID, config.platform, config.scheduleTime)
	}

	return nil
}

// CleanSeedData removes all seeded data (useful for testing)
func CleanSeedData() error {
	log.Println("Cleaning seeded data...")

	// Clean in reverse order of dependencies
	cleanQueries := []string{
		"DELETE FROM user_sessions WHERE user_id IN (SELECT id FROM users WHERE username IN ('admin', 'demo', 'test', 'user1'))",
		"DELETE FROM schedule_config WHERE group_id IN ('sample_whatsapp_group', 'sample_telegram_group', 'weekend_analysis_group')",
		"DELETE FROM users WHERE username IN ('admin', 'demo', 'test', 'user1')",
	}

	for _, query := range cleanQueries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Printf("Warning: Failed to execute clean query: %v", err)
			// Continue with other clean operations
		}
	}

	log.Println("Seed data cleaning completed")
	return nil
}

// generateSessionToken generates a random session token
func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// createSampleSession creates a sample session for testing
func createSampleSession(userID int) error {
	token, err := generateSessionToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(24 * time.Hour) // 24 hours from now

	_, err = DB.Exec(
		"INSERT INTO user_sessions (user_id, session_token, expires_at) VALUES (?, ?, ?)",
		userID, token, expiresAt,
	)
	if err != nil {
		return err
	}

	log.Printf("Sample session created for user ID %d: %s", userID, token)
	return nil
}

// SeedWithSampleData creates comprehensive sample data for development/testing
func SeedWithSampleData() error {
	log.Println("Creating comprehensive sample data...")

	// Run all seeders
	if err := RunSeeders(); err != nil {
		return err
	}

	// Create sample sessions for demo users
	usernames := []string{"admin", "demo", "test"}
	for _, username := range usernames {
		var userID int
		err := DB.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
		if err != nil {
			log.Printf("Warning: Could not find user %s for session creation: %v", username, err)
			continue
		}

		if err := createSampleSession(userID); err != nil {
			log.Printf("Warning: Could not create session for user %s: %v", username, err)
		}
	}

	log.Println("Comprehensive sample data creation completed")
	return nil
}

// VerifySeedData checks if all expected seed data exists
func VerifySeedData() error {
	log.Println("Verifying seed data...")

	// Check admin user
	var adminCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&adminCount)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %v", err)
	}
	if adminCount == 0 {
		return fmt.Errorf("admin user not found")
	}

	// Check total users
	var userCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		return fmt.Errorf("failed to count users: %v", err)
	}

	// Check schedule configs
	var scheduleCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM schedule_config").Scan(&scheduleCount)
	if err != nil {
		return fmt.Errorf("failed to count schedule configs: %v", err)
	}

	log.Printf("Seed data verification completed: %d users, %d schedule configs", userCount, scheduleCount)
	return nil
}