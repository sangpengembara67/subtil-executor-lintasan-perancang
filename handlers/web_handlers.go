package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/database"
	"github.com/shiestapoi/teletowa/services"
)

// Global variables are now defined in telegram_client.go to avoid redeclaration

// WebHandler handles web interface requests
type WebHandler struct {
	scheduler *services.SchedulerService
}

// NewWebHandler creates a new web handler
func NewWebHandler(scheduler *services.SchedulerService) *WebHandler {
	return &WebHandler{
		scheduler: scheduler,
	}
}

// LoginRequest represents login request data
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangeUsernameRequest represents username change request
type ChangeUsernameRequest struct {
	NewUsername string `json:"new_username" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

// ScheduleRequest represents schedule configuration request
type ScheduleRequest struct {
	ChatID       string `json:"chat_id" binding:"required"`
	Platform     string `json:"platform" binding:"required"`
	ScheduleTime string `json:"schedule_time" binding:"required"`
	DaysOfWeek   string `json:"days_of_week" binding:"required"`
	IsActive     bool   `json:"is_active"`
}

// LoginPage serves the login page
func (h *WebHandler) LoginPage(c *gin.Context) {
	// Check if already logged in
	if h.isAuthenticated(c) {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login - TeleToWa Control Panel",
	})
}

// Login handles login requests
func (h *WebHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Authenticate user
	user, err := database.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Create session
	sessionToken := h.generateSessionToken()
	if err := database.CreateUserSession(user.ID, sessionToken); err != nil {
		log.Printf("Failed to create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Set session cookie
	c.SetCookie("session_token", sessionToken, 86400, "/", "", false, true) // 24 hours

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    user.Username,
	})
}

// Logout handles logout requests
func (h *WebHandler) Logout(c *gin.Context) {
	sessionToken, err := c.Cookie("session_token")
	if err == nil {
		database.DeleteUserSession(sessionToken)
	}

	c.SetCookie("session_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

// Dashboard serves the main dashboard
func (h *WebHandler) Dashboard(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get current user
	user := h.getCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get scheduler status
	status := h.scheduler.GetScheduleStatus()

	// Get all schedules
	schedules, err := h.scheduler.GetSchedules()
	if err != nil {
		log.Printf("Failed to get schedules: %v", err)
		schedules = []services.ScheduleData{}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":     "Dashboard - TeleToWa Control Panel",
		"user":      user,
		"status":    status,
		"schedules": schedules,
	})
}

// GetSchedules returns all schedule configurations
func (h *WebHandler) GetSchedules(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	schedules, err := h.scheduler.GetSchedules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get schedules"})
		return
	}

	c.JSON(http.StatusOK, schedules)
}

// CreateSchedule creates a new schedule configuration
func (h *WebHandler) CreateSchedule(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Create schedule data
	schedule := &services.ScheduleData{
		GroupID:      req.ChatID,
		Platform:     req.Platform,
		ScheduleTime: req.ScheduleTime,
		DaysOfWeek:   req.DaysOfWeek,
		Active:       req.IsActive,
	}

	// Create schedule
	err := h.scheduler.CreateSchedule(schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule created successfully"})
}

// UpdateSchedule updates an existing schedule configuration
func (h *WebHandler) UpdateSchedule(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule ID"})
		return
	}

	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Create schedule data
	schedule := &services.ScheduleData{
		ID:           id,
		GroupID:      req.ChatID,
		Platform:     req.Platform,
		ScheduleTime: req.ScheduleTime,
		DaysOfWeek:   req.DaysOfWeek,
		Active:       req.IsActive,
	}

	// Update schedule
	err = h.scheduler.UpdateSchedule(schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule updated successfully"})
}

// DeleteSchedule deletes a schedule configuration
func (h *WebHandler) DeleteSchedule(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule ID"})
		return
	}

	err = h.scheduler.DeleteSchedule(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule deleted successfully"})
}

// TestSchedule sends a test report
func (h *WebHandler) TestSchedule(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule ID"})
		return
	}

	err = h.scheduler.TestSchedule(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send test report: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test report sent successfully"})
}

// ChangePassword handles password change requests
func (h *WebHandler) ChangePassword(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user := h.getCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Verify current password
	if !database.VerifyPassword(user.ID, req.CurrentPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Update password
	err := database.UpdateUserPassword(user.ID, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

// ChangeUsername handles username change requests
func (h *WebHandler) ChangeUsername(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user := h.getCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Verify password
	if !database.VerifyPassword(user.ID, req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is incorrect"})
		return
	}

	// Update username
	err := database.UpdateUsername(user.ID, req.NewUsername)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Username updated successfully"})
}

// GetAnalytics returns analytics data
func (h *WebHandler) GetAnalytics(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get analytics data from database
	startDate := time.Now().AddDate(0, 0, -30) // Last 30 days
	endDate := time.Now()
	analytics, err := database.GetAnalytics(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetChatAnalysis returns analysis data for a specific chat
func (h *WebHandler) GetChatAnalysis(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	chatID := c.Param("chat_id")
	platform := c.Query("platform")

	analysis, err := database.GetChatAnalysisByChat(chatID, platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat analysis"})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// GetStockAnalysis returns stock analysis data
func (h *WebHandler) GetStockAnalysis(c *gin.Context) {
	if !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	analysis, err := database.GetStockAnalysis(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stock analysis"})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// isAuthenticated checks if the user is authenticated
func (h *WebHandler) isAuthenticated(c *gin.Context) bool {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return false
	}

	return database.IsValidSession(sessionToken)
}

// getCurrentUser gets the current authenticated user
func (h *WebHandler) getCurrentUser(c *gin.Context) *database.User {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return nil
	}

	user, err := database.GetUserBySession(sessionToken)
	if err != nil {
		return nil
	}

	return user
}

// generateSessionToken generates a random session token
func (h *WebHandler) generateSessionToken() string {
	// Simple session token generation
	// In production, use a more secure method
	return fmt.Sprintf("session_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}

// AuthMiddleware is a middleware for authentication
func (h *WebHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.isAuthenticated(c) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AnalyzeChatHandler handles AI analysis requests for specific chats
func AnalyzeChatHandler(c *gin.Context) {
	chatID := c.Param("chat_id")
	platform := c.DefaultQuery("platform", "")
	daysStr := c.DefaultQuery("days", "7")

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid days parameter"})
		return
	}

	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer type assertion failed"})
		return
	}

	result, err := aiAnalyzer.AnalyzeChatMessages(chatID, platform, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetAnalysisResultsHandler retrieves stored analysis results
func GetAnalysisResultsHandler(c *gin.Context) {
	chatID := c.DefaultQuery("chat_id", "")
	platform := c.DefaultQuery("platform", "")
	limitStr := c.DefaultQuery("limit", "10")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer type assertion failed"})
		return
	}

	results, err := aiAnalyzer.GetAnalysisResults(chatID, platform, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// GetAnalysisStatsHandler retrieves analysis statistics
func GetAnalysisStatsHandler(c *gin.Context) {
	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer type assertion failed"})
		return
	}

	stats, err := aiAnalyzer.GetAnalysisStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// AnalyzeStockMentionsHandler analyzes stock mentions in chat
func AnalyzeStockMentionsHandler(c *gin.Context) {
	chatID := c.Param("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID is required"})
		return
	}

	// Get days parameter (optional, default to 7)
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer type assertion failed"})
		return
	}

	results, err := aiAnalyzer.AnalyzeStockMentions(chatID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// GetStockMentionsHandler handles GET /api/stocks/mentions/:chat_id
func GetStockMentionsHandler(c *gin.Context) {
	chatID := c.Param("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID is required"})
		return
	}

	// Get query parameters
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	stockCode := c.Query("stock_code")
	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer not available"})
		return
	}

	// Get stock analyzer from AI analyzer
	stockAnalyzer := aiAnalyzer.GetStockAnalyzer()
	if stockAnalyzer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stock analyzer not available"})
		return
	}

	mentions, err := stockAnalyzer.GetStockMentions(chatID, stockCode, days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    mentions,
	})
}

// GetStockStatsHandler handles GET /api/stocks/stats/:chat_id
func GetStockStatsHandler(c *gin.Context) {
	chatID := c.Param("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID is required"})
		return
	}

	// Get days parameter (optional, default to 30)
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer not available"})
		return
	}

	// Get stock analyzer from AI analyzer
	stockAnalyzer := aiAnalyzer.GetStockAnalyzer()
	if stockAnalyzer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stock analyzer not available"})
		return
	}

	stats, err := stockAnalyzer.GetStockStats(chatID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// DetectStockCodesHandler handles POST /api/stocks/detect
func DetectStockCodesHandler(c *gin.Context) {
	var request struct {
		Text     string `json:"text" binding:"required"`
		ChatID   string `json:"chat_id"`
		Platform string `json:"platform"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if globalAIAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI analyzer not available"})
		return
	}

	aiAnalyzer, ok := globalAIAnalyzer.(*services.AIAnalyzer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analyzer not available"})
		return
	}

	// Get stock analyzer from AI analyzer
	stockAnalyzer := aiAnalyzer.GetStockAnalyzer()
	if stockAnalyzer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stock analyzer not available"})
		return
	}

	detections := stockAnalyzer.DetectStockMentions(request.Text, request.ChatID, request.Platform)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    detections,
	})
}

// SetupRoutes sets up all web routes
func (h *WebHandler) SetupRoutes(r *gin.Engine) {
	// Load HTML templates
	r.LoadHTMLGlob("templates/*")

	// Static files
	r.Static("/static", "./static")

	// Public routes
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})
	r.GET("/login", h.LoginPage)
	r.POST("/api/login", h.Login)
	r.POST("/api/logout", h.Logout)

	// Protected routes
	protected := r.Group("/")
	protected.Use(h.AuthMiddleware())
	{
		// Dashboard
		protected.GET("/dashboard", h.Dashboard)

		// API routes
		api := protected.Group("/api")
		{
			// Schedule management
			api.GET("/schedules", h.GetSchedules)
			api.POST("/schedules", h.CreateSchedule)
			api.PUT("/schedules/:chat_id/:platform", h.UpdateSchedule)
			api.DELETE("/schedules/:chat_id/:platform", h.DeleteSchedule)
			api.POST("/schedules/:chat_id/:platform/test", h.TestSchedule)

			// User management
			api.POST("/change-password", h.ChangePassword)
			api.POST("/change-username", h.ChangeUsername)

			// Analytics
			api.GET("/analytics", h.GetAnalytics)
			api.GET("/chat-analysis/:chat_id", h.GetChatAnalysis)
			api.GET("/stock-analysis", h.GetStockAnalysis)

			// AI Analysis API
			api.POST("/analyze/:chat_id", AnalyzeChatHandler)
			api.GET("/analysis/results", GetAnalysisResultsHandler)
			api.GET("/analysis/stats", GetAnalysisStatsHandler)
			api.GET("/analysis/stocks/:chat_id", AnalyzeStockMentionsHandler)

			// Stock Analysis API
			api.GET("/stocks/mentions/:chat_id", GetStockMentionsHandler)
			api.GET("/stocks/stats/:chat_id", GetStockStatsHandler)
			api.POST("/stocks/detect", DetectStockCodesHandler)
		}
	}
}
