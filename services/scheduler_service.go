package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// SchedulerService handles scheduled tasks and analysis reports
type SchedulerService struct {
	db             *sql.DB
	mediaExtractor *MediaExtractor
	aiAnalyzer     interface{}
	running        bool
	stopChan       chan bool
}

// NewSchedulerService creates a new SchedulerService instance
func NewSchedulerService(db *sql.DB, mediaExtractor *MediaExtractor) *SchedulerService {
	return &SchedulerService{
		db:             db,
		mediaExtractor: mediaExtractor,
		running:        false,
		stopChan:       make(chan bool),
	}
}

// SetAIAnalyzer sets the AI analyzer for the scheduler service
func (ss *SchedulerService) SetAIAnalyzer(analyzer interface{}) {
	ss.aiAnalyzer = analyzer
}

// ScheduleData represents a scheduled task
type ScheduleData struct {
	ID           int        `json:"id"`
	GroupID      string     `json:"group_id"`
	Platform     string     `json:"platform"`
	ScheduleTime string     `json:"schedule_time"` // Format: "15:04" (24-hour)
	DaysOfWeek   string     `json:"days_of_week"`  // JSON array of days: ["monday", "tuesday", ...]
	Active       bool       `json:"active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Start begins the scheduler service
func (ss *SchedulerService) Start() {
	if ss.running {
		return
	}

	ss.running = true
	log.Println("Starting SchedulerService...")

	go ss.run()
}

// Stop stops the scheduler service
func (ss *SchedulerService) Stop() {
	if !ss.running {
		return
	}

	log.Println("Stopping SchedulerService...")
	ss.running = false
	ss.stopChan <- true
}

// run is the main scheduler loop
func (ss *SchedulerService) run() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ss.stopChan:
			log.Println("SchedulerService stopped")
			return
		case <-ticker.C:
			ss.checkSchedules()
		}
	}
}

// checkSchedules checks and executes due schedules
func (ss *SchedulerService) checkSchedules() {
	schedules, err := ss.GetActiveSchedules()
	if err != nil {
		log.Printf("Error getting active schedules: %v", err)
		return
	}

	now := time.Now()
	currentDay := strings.ToLower(now.Weekday().String())
	currentTime := now.Format("15:04")

	for _, schedule := range schedules {
		// Check if today is in the scheduled days
		var days []string
		if err := json.Unmarshal([]byte(schedule.DaysOfWeek), &days); err != nil {
			log.Printf("Error parsing days for schedule %d: %v", schedule.ID, err)
			continue
		}

		dayMatch := false
		for _, day := range days {
			if strings.ToLower(day) == currentDay {
				dayMatch = true
				break
			}
		}

		if !dayMatch {
			continue
		}

		// Check if it's time to run
		if schedule.ScheduleTime != currentTime {
			continue
		}

		// Execute the schedule
		log.Printf("Executing schedule for group: %s", schedule.GroupID)
		if err := ss.executeSchedule(&schedule); err != nil {
			log.Printf("Error executing schedule for group %s: %v", schedule.GroupID, err)
		} else {
			// Update last run time
			ss.updateLastRun(schedule.ID, now)
		}
	}
}

// executeSchedule executes a scheduled task
func (ss *SchedulerService) executeSchedule(schedule *ScheduleData) error {
	// Generate analysis report
	report, err := ss.generateAnalysisReport(schedule.GroupID)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Send report to target group based on platform
	if schedule.Platform == "whatsapp" {
		if SendWhatsAppMessage != nil {
			if err := SendWhatsAppMessage(schedule.GroupID, report, "", "Scheduler"); err != nil {
				return fmt.Errorf("failed to send report: %w", err)
			}
		} else {
			return fmt.Errorf("SendWhatsAppMessage function not initialized")
		}
	} else if schedule.Platform == "telegram" {
		// TODO: Implement telegram message sending
		log.Printf("Telegram messaging not yet implemented for group: %s", schedule.GroupID)
		return fmt.Errorf("telegram messaging not yet implemented")
	}

	log.Printf("Successfully sent scheduled report to %s group: %s", schedule.Platform, schedule.GroupID)
	return nil
}

// generateAnalysisReport generates a comprehensive analysis report
func (ss *SchedulerService) generateAnalysisReport(chatID string) (string, error) {
	// Get message statistics
	stats, err := ss.mediaExtractor.GetMessageStats()
	if err != nil {
		return "", fmt.Errorf("failed to get message stats: %w", err)
	}

	// Get content analysis for the last 24 hours
	analysis, err := ss.mediaExtractor.AnalyzeContent(chatID, 1)
	if err != nil {
		return "", fmt.Errorf("failed to analyze content: %w", err)
	}

	// Get recent messages
	recentMessages, err := ss.mediaExtractor.GetExtractedMessages("", chatID, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get recent messages: %w", err)
	}

	// Get AI analysis if available
	var aiAnalysis interface{}
	if ss.aiAnalyzer != nil {
		if analyzer, ok := ss.aiAnalyzer.(*AIAnalyzer); ok {
			aiResult, err := analyzer.AnalyzeChatMessages(chatID, "", 1)
			if err == nil {
				aiAnalysis = aiResult
			}
		}
	}

	// Build report
	report := ss.buildReport(stats, analysis, recentMessages, aiAnalysis)
	return report, nil
}

// buildReport builds a formatted analysis report
func (ss *SchedulerService) buildReport(stats map[string]interface{}, analysis map[string]interface{}, recentMessages []MessageData, aiAnalysis interface{}) string {
	report := "📊 *LAPORAN ANALISIS HARIAN*\n"
	report += "═══════════════════════════\n\n"

	// Statistics section
	report += "📈 *STATISTIK PESAN*\n"
	if totalMessages, ok := stats["total_messages"].(int); ok {
		report += fmt.Sprintf("• Total Pesan: %d\n", totalMessages)
	}

	if recent24h, ok := stats["recent_24h"].(int); ok {
		report += fmt.Sprintf("• Pesan 24 Jam Terakhir: %d\n", recent24h)
	}

	if platformStats, ok := stats["by_platform"].(map[string]int); ok {
		report += "\n📱 *Per Platform:*\n"
		for platform, count := range platformStats {
			report += fmt.Sprintf("• %s: %d pesan\n", strings.Title(platform), count)
		}
	}

	if typeStats, ok := stats["by_type"].(map[string]int); ok {
		report += "\n📝 *Per Jenis Pesan:*\n"
		for msgType, count := range typeStats {
			report += fmt.Sprintf("• %s: %d\n", strings.Title(msgType), count)
		}
	}

	// Analysis section
	report += "\n\n🔍 *ANALISIS KONTEN*\n"
	if totalAnalyzed, ok := analysis["total_analyzed"].(int); ok {
		report += fmt.Sprintf("• Pesan Dianalisis: %d\n", totalAnalyzed)
	}

	if totalWords, ok := analysis["total_words"].(int); ok {
		report += fmt.Sprintf("• Total Kata: %d\n", totalWords)
	}

	if avgWords, ok := analysis["avg_words_per_message"].(float64); ok {
		report += fmt.Sprintf("• Rata-rata Kata per Pesan: %.1f\n", avgWords)
	}

	// Top words section
	if topWordsInterface, ok := analysis["top_words"]; ok {
		if topWordsJSON, err := json.Marshal(topWordsInterface); err == nil {
			var topWords []struct {
				Word  string `json:"word"`
				Count int    `json:"count"`
			}
			if err := json.Unmarshal(topWordsJSON, &topWords); err == nil && len(topWords) > 0 {
				report += "\n🏆 *Kata Paling Sering:*\n"
				for i, word := range topWords {
					if i >= 5 { // Limit to top 5
						break
					}
					report += fmt.Sprintf("• %s (%d kali)\n", word.Word, word.Count)
				}
			}
		}
	}

	// Recent activity section
	if len(recentMessages) > 0 {
		report += "\n\n📋 *AKTIVITAS TERBARU*\n"
		for i, msg := range recentMessages {
			if i >= 3 { // Limit to 3 recent messages
				break
			}
			timeStr := msg.Timestamp.Format("15:04")
			content := msg.Content
			if len(content) > 50 {
				content = content[:47] + "..."
			}
			report += fmt.Sprintf("• [%s] %s: %s\n", timeStr, msg.SenderName, content)
		}
	}

	// AI Analysis section
	if aiAnalysis != nil {
		report += "\n\n🤖 *ANALISIS AI*\n"
		if analysisResult, ok := aiAnalysis.(*AnalysisResult); ok {
			if analysisResult.Summary != "" {
				report += fmt.Sprintf("• Ringkasan: %s\n", analysisResult.Summary)
			}
			if analysisResult.Sentiment != "" {
				report += fmt.Sprintf("• Sentimen: %s\n", analysisResult.Sentiment)
			}
			if len(analysisResult.Keywords) > 0 {
				report += "• Kata Kunci: " + analysisResult.Keywords + "\n"
			}
			if len(analysisResult.StockMentions) > 0 {
				report += "• Saham Disebutkan: " + analysisResult.StockMentions + "\n"
			}
		}
	}

	// Footer
	report += "\n═══════════════════════════\n"
	report += fmt.Sprintf("📅 Laporan dibuat: %s\n", time.Now().Format("02/01/2006 15:04"))
	report += "🤖 *TeleTowa Analytics Bot*"

	return report
}

// CreateSchedule creates a new schedule
func (ss *SchedulerService) CreateSchedule(schedule *ScheduleData) error {
	query := `
		INSERT INTO schedule_config (
			group_id, platform, schedule_time,
			days_of_week, active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			schedule_time = VALUES(schedule_time),
			days_of_week = VALUES(days_of_week),
			active = VALUES(active),
			updated_at = VALUES(updated_at)
	`

	now := time.Now()
	result, err := ss.db.Exec(
		query,
		schedule.GroupID,
		schedule.Platform,
		schedule.ScheduleTime,
		schedule.DaysOfWeek,
		schedule.Active,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create schedule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get schedule ID: %w", err)
	}

	schedule.ID = int(id)
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	log.Printf("Created schedule for group: %s (ID: %d)", schedule.GroupID, schedule.ID)
	return nil
}

// GetSchedules retrieves all schedules
func (ss *SchedulerService) GetSchedules() ([]ScheduleData, error) {
	query := `
		SELECT id, group_id, platform, schedule_time,
		       days_of_week, active, created_at, updated_at
		FROM schedule_config
		ORDER BY created_at DESC
	`

	rows, err := ss.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schedules: %w", err)
	}
	defer rows.Close()

	var schedules []ScheduleData
	for rows.Next() {
		var schedule ScheduleData

		err := rows.Scan(
			&schedule.ID,
			&schedule.GroupID,
			&schedule.Platform,
			&schedule.ScheduleTime,
			&schedule.DaysOfWeek,
			&schedule.Active,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// GetActiveSchedules retrieves only active schedules
func (ss *SchedulerService) GetActiveSchedules() ([]ScheduleData, error) {
	query := `
		SELECT id, group_id, platform, schedule_time,
		       days_of_week, active, created_at, updated_at
		FROM schedule_config
		WHERE active = 1
		ORDER BY schedule_time
	`

	rows, err := ss.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active schedules: %w", err)
	}
	defer rows.Close()

	var schedules []ScheduleData
	for rows.Next() {
		var schedule ScheduleData

		err := rows.Scan(
			&schedule.ID,
			&schedule.GroupID,
			&schedule.Platform,
			&schedule.ScheduleTime,
			&schedule.DaysOfWeek,
			&schedule.Active,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// UpdateSchedule updates an existing schedule
func (ss *SchedulerService) UpdateSchedule(schedule *ScheduleData) error {
	query := `
		UPDATE schedule_config SET
			group_id = ?, platform = ?,
			schedule_time = ?, days_of_week = ?, active = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := ss.db.Exec(
		query,
		schedule.GroupID,
		schedule.Platform,
		schedule.ScheduleTime,
		schedule.DaysOfWeek,
		schedule.Active,
		now,
		schedule.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	schedule.UpdatedAt = now
	log.Printf("Updated schedule for group: %s (ID: %d)", schedule.GroupID, schedule.ID)
	return nil
}

// DeleteSchedule deletes a schedule
func (ss *SchedulerService) DeleteSchedule(id int) error {
	query := "DELETE FROM schedule_config WHERE id = ?"
	_, err := ss.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	log.Printf("Deleted schedule ID: %d", id)
	return nil
}

// ToggleSchedule toggles the active status of a schedule
func (ss *SchedulerService) ToggleSchedule(id int) error {
	query := `
		UPDATE schedule_config SET
			active = NOT active,
			updated_at = ?
		WHERE id = ?
	`

	_, err := ss.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to toggle schedule: %w", err)
	}

	log.Printf("Toggled schedule ID: %d", id)
	return nil
}

// TestSchedule manually executes a schedule for testing
func (ss *SchedulerService) TestSchedule(id int) error {
	query := `
		SELECT id, group_id, platform, schedule_time,
		       days_of_week, active, created_at, updated_at
		FROM schedule_config
		WHERE id = ?
	`

	var schedule ScheduleData

	err := ss.db.QueryRow(query, id).Scan(
		&schedule.ID,
		&schedule.GroupID,
		&schedule.Platform,
		&schedule.ScheduleTime,
		&schedule.DaysOfWeek,
		&schedule.Active,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}

	// Execute the schedule
	log.Printf("Testing schedule for group: %s", schedule.GroupID)
	return ss.executeSchedule(&schedule)
}

// updateLastRun updates the last run time for a schedule
func (ss *SchedulerService) updateLastRun(id int, lastRun time.Time) error {
	query := "UPDATE schedule_config SET last_run = ? WHERE id = ?"
	_, err := ss.db.Exec(query, lastRun, id)
	if err != nil {
		return fmt.Errorf("failed to update last run: %w", err)
	}
	return nil
}

// GetScheduleStats returns statistics about schedules
func (ss *SchedulerService) GetScheduleStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total schedules
	var totalSchedules int
	err := ss.db.QueryRow("SELECT COUNT(*) FROM schedule_config").Scan(&totalSchedules)
	if err != nil {
		return nil, fmt.Errorf("failed to get total schedules: %w", err)
	}
	stats["total_schedules"] = totalSchedules

	// Active schedules
	var activeSchedules int
	err = ss.db.QueryRow("SELECT COUNT(*) FROM schedule_config WHERE active = 1").Scan(&activeSchedules)
	if err != nil {
		return nil, fmt.Errorf("failed to get active schedules: %w", err)
	}
	stats["active_schedules"] = activeSchedules

	// Schedules run today
	var schedulesToday int
	todayQuery := `
		SELECT COUNT(*) FROM schedule_config
		WHERE last_run IS NOT NULL
		AND date(last_run) = date('now')
	`
	err = ss.db.QueryRow(todayQuery).Scan(&schedulesToday)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules run today: %w", err)
	}
	stats["schedules_run_today"] = schedulesToday

	return stats, nil
}

// GetScheduleStatus returns the current status of the scheduler
func (ss *SchedulerService) GetScheduleStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":          ss.running,
		"active_schedules": 0, // Placeholder
	}
}
