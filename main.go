package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/shiestapoi/teletowa/database"
	"github.com/shiestapoi/teletowa/handlers"
	"github.com/shiestapoi/teletowa/middleware"
	"github.com/shiestapoi/teletowa/services"
)

//go:embed all:public
var publicFS embed.FS

//go:embed all:templates
var templatesFS embed.FS

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Initialize database
	if err := database.InitDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize database connection for services
	db := database.GetDB()

	// Initialize services
	mediaExtractor := services.NewMediaExtractor(db)
	schedulerService := services.NewSchedulerService(db, mediaExtractor)
	aiAnalyzer := services.NewAIAnalyzer(db, mediaExtractor)

	// Set AI analyzer for scheduler service
	schedulerService.SetAIAnalyzer(aiAnalyzer)

	// Start scheduler
	go schedulerService.Start()

	// Inisialisasi router Gin
	r := gin.Default()

	// Set embedded filesystem untuk handlers
	handlers.SetEmbeddedFS(publicFS)
	handlers.SetServices(mediaExtractor, schedulerService, aiAnalyzer)

	// Inisialisasi WhatsApp client
	if err := handlers.InitWhatsAppClient(); err != nil {
		log.Printf("Warning: Failed to initialize WhatsApp client: %v", err)
		// Lanjutkan eksekusi meskipun WhatsApp client gagal dimulai
	}

	// Inisialisasi fungsi SendWhatsAppMessage untuk scheduler
	services.SendWhatsAppMessage = handlers.SendWhatsAppMessage

	// Inisialisasi Telegram user client
	if err := handlers.InitTelegramUserClient(); err != nil {
		log.Printf("Warning: Failed to initialize Telegram user client: %v", err)
		// Lanjutkan eksekusi meskipun Telegram client gagal dimulai
	}

	// Siapkan middleware
	r.Use(middleware.Logger())
	r.Use(middleware.Auth())

	// Siapkan file statis dari embedded filesystem
	r.StaticFS("/public", http.FS(publicFS))
	r.Static("/static", "./static")

	// Load templates dari embedded filesystem
	templ := template.Must(template.New("").ParseFS(templatesFS, "templates/*.html", "templates/*/*.html"))
	r.SetHTMLTemplate(templ)

	// Route login
	r.GET("/login", handlers.ShowLoginPage)
	r.POST("/login", handlers.ProcessLogin)
	r.GET("/logout", handlers.Logout)

	// Dashboard
	r.GET("/", middleware.RequireAuth, handlers.Dashboard)

	// Telegram routes
	telegramGroup := r.Group("/telegram", middleware.RequireAuth)
	{
		telegramGroup.GET("/", handlers.TelegramDashboard)
		telegramGroup.GET("/chats", handlers.ListTelegramChats)
		telegramGroup.POST("/select-chat", handlers.SelectTelegramChat)
		telegramGroup.GET("/forward-settings", handlers.TelegramForwardSettingsPage)
		telegramGroup.POST("/forward-settings", handlers.SaveTelegramForwardSettings)

		// User Telegram Auth routes
		telegramGroup.GET("/login", handlers.ShowTelegramLoginPage)
		telegramGroup.POST("/login", handlers.SaveTelegramUserConfig)
		telegramGroup.GET("/login/verify", handlers.ShowTelegramVerifyPage)
		telegramGroup.POST("/login/verify", handlers.ProcessTelegramVerification)
		telegramGroup.POST("/login/send-code", handlers.SendAuthCode)
		telegramGroup.GET("/user/status", handlers.GetTelegramAuthStatus)
		telegramGroup.POST("/user/logout", handlers.LogoutTelegramUser)
	}

	// WhatsApp routes
	whatsappGroup := r.Group("/whatsapp", middleware.RequireAuth)
	{
		whatsappGroup.GET("/", handlers.WhatsAppDashboard)
		whatsappGroup.GET("/login", handlers.WhatsAppLoginPage)
		whatsappGroup.GET("/qr", handlers.GenerateWhatsAppQR)
		whatsappGroup.POST("/login", handlers.ProcessWhatsAppLogin)
		whatsappGroup.GET("/chats", handlers.ListWhatsAppChats)
		whatsappGroup.POST("/select-chat", handlers.SelectWhatsAppChat)
		whatsappGroup.POST("/send", handlers.SendWhatsAppMessageHandler)
		whatsappGroup.GET("/logout", handlers.WhatsAppLogout)
		whatsappGroup.GET("/group-info", handlers.GetGroupInfo)
		whatsappGroup.GET("/group-invite-link", handlers.GetGroupInviteLink)
		whatsappGroup.POST("/set-group-photo", handlers.SetGroupPhoto)
		whatsappGroup.GET("/panelchat", handlers.PanelChat)
		whatsappGroup.POST("/panelchat", handlers.PanelChatPost)
	}

	// API routes for integration
	apiGroup := r.Group("/api")
	{
		// Public API routes (no auth required)
		apiGroup.POST("/login", handlers.LoginHandler)
		apiGroup.POST("/logout", handlers.LogoutHandler)

		// Protected API routes
		protectedAPI := apiGroup.Group("/", middleware.RequireAuth)
		{
			protectedAPI.GET("/status", handlers.ApiStatus)
			protectedAPI.POST("/forward", handlers.ForwardMessage)

			// Dashboard API
			protectedAPI.GET("/dashboard/stats", handlers.GetDashboardStats)
			protectedAPI.GET("/analytics", handlers.GetAnalytics)

			// AI Analysis API
			protectedAPI.POST("/analyze/:chat_id", handlers.AnalyzeChatHandler)
			protectedAPI.GET("/analysis/results", handlers.GetAnalysisResultsHandler)
			protectedAPI.GET("/analysis/stats", handlers.GetAnalysisStatsHandler)
			protectedAPI.GET("/analysis/stocks/:chat_id", handlers.AnalyzeStockMentionsHandler)

			// Schedule management API
			protectedAPI.GET("/schedules", handlers.GetSchedules)
			protectedAPI.POST("/schedules", handlers.CreateSchedule)
			protectedAPI.POST("/schedules/:id/test", handlers.TestSchedule)
			protectedAPI.POST("/schedules/:id/toggle", handlers.ToggleSchedule)
			protectedAPI.DELETE("/schedules/:id", handlers.DeleteSchedule)

			// User management API
			protectedAPI.POST("/user/change-password", handlers.ChangePassword)
			protectedAPI.POST("/user/change-username", handlers.ChangeUsername)

			// WhatsApp API routes
			apiWhatsAppGroup := protectedAPI.Group("/whatsapp")
			{
				apiWhatsAppGroup.POST("/active", handlers.ToggleWhatsAppActive)
				apiWhatsAppGroup.POST("/logout", handlers.WhatsAppLogout)
				apiWhatsAppGroup.POST("/send", handlers.SendWhatsAppMessageHandler)
			}
		}
	}

	// WebSocket route
	r.GET("/ws", middleware.RequireAuth, handlers.HandleWebSocket)

	// Mulai WebSocket manager
	go handlers.WSManager.Start()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default local port
	}
	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
