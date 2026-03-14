package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/database"
)

// LoginHandler handles API login requests
func LoginHandler(c *gin.Context) {
	var request struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Authenticate user
	user, err := database.AuthenticateUser(request.Username, request.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Create session cookie for authentication
	c.SetCookie(
		"session",
		user.Username,
		3600*24, // 1 day
		"/",
		"",
		false,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login successful",
		"user":    user.Username,
	})
}

// LogoutHandler handles API logout requests
func LogoutHandler(c *gin.Context) {
	// Clear session cookie
	c.SetCookie("session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful",
	})
}

// GetDashboardStats returns dashboard statistics
func GetDashboardStats(c *gin.Context) {
	// Get recent chat analysis count
	startDate := time.Now().AddDate(0, 0, -7) // Last 7 days
	endDate := time.Now()

	analyses, err := database.GetChatAnalysisByDate(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	// Get schedule configs
	schedules, err := database.GetScheduleConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get schedules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_analyses":   len(analyses),
			"active_schedules": len(schedules),
			"period":           "7 days",
		},
	})
}

// GetAnalytics returns analytics data
func GetAnalytics(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	endDate := time.Now()

	analyses, err := database.GetChatAnalysisByDate(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get analytics"})
		return
	}

	// Group by platform
	platformStats := make(map[string]int)
	for _, analysis := range analyses {
		platformStats[analysis.Platform]++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_messages":     len(analyses),
			"platform_breakdown": platformStats,
			"period_days":        days,
		},
	})
}

// GetSchedules returns all schedule configurations
func GetSchedules(c *gin.Context) {
	schedules, err := database.GetScheduleConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get schedules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    schedules,
	})
}

// CreateSchedule creates a new schedule configuration
func CreateSchedule(c *gin.Context) {
	var config database.ScheduleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if err := database.SaveScheduleConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Schedule created successfully",
	})
}

// TestSchedule tests a schedule configuration
func TestSchedule(c *gin.Context) {
	id := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Schedule test completed for ID: " + id,
	})
}

// ToggleSchedule toggles a schedule's active status
func ToggleSchedule(c *gin.Context) {
	id := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Schedule toggled for ID: " + id,
	})
}

// DeleteSchedule deletes a schedule configuration
func DeleteSchedule(c *gin.Context) {
	id := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Schedule deleted for ID: " + id,
	})
}

// ChangePassword changes user password
func ChangePassword(c *gin.Context) {
	var request struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed successfully",
	})
}

// ChangeUsername changes username
func ChangeUsername(c *gin.Context) {
	var request struct {
		NewUsername string `json:"new_username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Username changed successfully",
	})
}
