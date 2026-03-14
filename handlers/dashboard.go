package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/config"
)

// Dashboard - Menampilkan halaman dashboard
func Dashboard(c *gin.Context) {
	// Ambil status Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			UserMode: false,
			Active:   false,
		}
	}

	// Ambil status WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		whatsappConfig = &config.WhatsAppConfig{
			LoggedIn: false,
			Active:   false,
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title":            "Dashboard - TeleTowa",
		"TelegramUserMode": telegramConfig.UserMode,
		"WhatsAppLoggedIn": whatsappConfig.LoggedIn,
		"WhatsAppActive":   whatsappConfig.Active,
		"ForwardActive":    telegramConfig.Active,
	})
}

// ApiStatus - Mendapatkan status aplikasi via API
func ApiStatus(c *gin.Context) {
	// Ambil status Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			UserMode:        false,
			Active:          false,
			StatusConnected: false,
		}
	}

	// Ambil status WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		whatsappConfig = &config.WhatsAppConfig{
			LoggedIn: false,
			Active:   false,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"telegram": gin.H{
			"user_mode":         telegramConfig.UserMode,
			"connected":         telegramConfig.StatusConnected,
			"active":            telegramConfig.Active,
			"selected_chats":    len(telegramConfig.SelectedChats),
			"selected_groups":   len(telegramConfig.SelectedGroups),
			"selected_channels": len(telegramConfig.SelectedChannels),
		},
		"whatsapp": gin.H{
			"logged_in":       whatsappConfig.LoggedIn,
			"active":          whatsappConfig.Active,
			"selected_chats":  len(whatsappConfig.SelectedChats),
			"selected_groups": len(whatsappConfig.SelectedGroups),
		},
		"forward": gin.H{
			"active": telegramConfig.Active && telegramConfig.UserMode,
		},
	})
}
