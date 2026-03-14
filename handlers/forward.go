package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shiestapoi/teletowa/config"
)

// ForwardMessage - Meneruskan pesan dari satu platform ke platform lain
func ForwardMessage(c *gin.Context) {
	// Parse request
	var request struct {
		SourcePlatform string `json:"source_platform" binding:"required"`
		TargetPlatform string `json:"target_platform" binding:"required"`
		Message        string `json:"message" binding:"required"`
		SourceName     string `json:"source_name"`
		SourceID       string `json:"source_id"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	// Validasi platform
	if request.SourcePlatform != "telegram" && request.SourcePlatform != "whatsapp" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Source platform tidak valid, harus telegram atau whatsapp",
		})
		return
	}

	if request.TargetPlatform != "telegram" && request.TargetPlatform != "whatsapp" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Target platform tidak valid, harus telegram atau whatsapp",
		})
		return
	}

	// Validasi target platform aktif
	if request.TargetPlatform == "whatsapp" {
		whatsappConfig, err := config.GetWhatsAppConfig()
		if err != nil || !whatsappConfig.Active || !whatsappConfig.LoggedIn {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "WhatsApp tidak aktif atau belum login",
			})
			return
		}
	} else if request.TargetPlatform == "telegram" {
		telegramConfig, err := config.GetTelegramConfig()
		if err != nil || !telegramConfig.UserMode {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Telegram tidak aktif atau user mode tidak diaktifkan",
			})
			return
		}
	}

	// Format pesan
	sourceName := request.SourceName
	if sourceName == "" {
		sourceName = "User"
	}

	formattedMessage := fmt.Sprintf("> 📨 *Service:* *%s*\n> 👤 *Pengirim:* %s\n💬 *Pesan:* %s\n\n*_Pesan ini diteruskan otomatis oleh sistem_*",
		strings.Title(strings.ToLower(request.SourcePlatform)),
		sourceName,
		request.Message)

	// Variabel untuk menyimpan status pengiriman

	var success bool
	var messageID string
	var errorMsg string

	// Teruskan pesan berdasarkan target platform
	if request.TargetPlatform == "whatsapp" {
		// Ambil konfigurasi WhatsApp
		whatsappConfig, _ := config.GetWhatsAppConfig()

		// Kirim ke semua kontak dan grup yang dipilih
		for _, chatID := range whatsappConfig.SelectedChats {
			// Send text message first
			if err := SendWhatsAppMessage(chatID, formattedMessage, ""); err != nil {
				success = false
				errorMsg = fmt.Sprintf("Gagal mengirim ke %s: %v", chatID, err)
				break
			}

			// Send media if available - use default image path
			if err := SendWhatsAppMessage(chatID, formattedMessage, "public/images/default.png"); err != nil {
				fmt.Printf("Warning: Gagal mengirim media ke %s: %v\n", chatID, err)
			}
			success = true
		}

		for _, groupID := range whatsappConfig.SelectedGroups {
			// Send text message first
			if err := SendWhatsAppMessage(groupID, formattedMessage, ""); err != nil {
				success = false
				errorMsg = fmt.Sprintf("Gagal mengirim ke grup %s: %v", groupID, err)
				break
			}

			// Send media if available - use default image path
			if err := SendWhatsAppMessage(groupID, formattedMessage, "public/images/default.png"); err != nil {
				fmt.Printf("Warning: Gagal mengirim media ke grup %s: %v\n", groupID, err)
			}
			success = true
		}
	} else if request.TargetPlatform == "telegram" {
		success = false
		errorMsg = "Pengiriman ke Telegram belum diimplementasikan"
	}

	// Kirim response
	if success {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message_id": messageID,
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errorMsg,
		})
	}
}

// TelegramForwardSettingsPage - Menampilkan halaman pengaturan penerusan pesan dari Telegram ke WhatsApp
func TelegramForwardSettingsPage(c *gin.Context) {
	// Ambil konfigurasi Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			UserMode:         false,
			Active:           false,
			SelectedChats:    []string{},
			SelectedGroups:   []string{},
			SelectedChannels: []string{},
		}
	}

	// Ambil konfigurasi WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		whatsappConfig = &config.WhatsAppConfig{
			LoggedIn:       false,
			Active:         false,
			SelectedChats:  []string{},
			SelectedGroups: []string{},
		}
	}

	// Ambil parameter pesan sukses atau error dari query string
	success := c.Query("success")
	errorMsg := c.Query("error")

	// Dummy riwayat (untuk contoh tampilan)
	// Di implementasi sebenarnya, ini akan diambil dari database/log
	forwardHistory := []gin.H{}

	c.HTML(http.StatusOK, "telegram/forward-settings.html", gin.H{
		"Title":                    "Pengaturan Forward Telegram - TeleTowa",
		"TelegramConnected":        telegramConfig.UserMode,
		"WhatsappConnected":        whatsappConfig.LoggedIn,
		"SelectedTelegramChats":    len(telegramConfig.SelectedChats),
		"SelectedTelegramGroups":   len(telegramConfig.SelectedGroups),
		"SelectedTelegramChannels": len(telegramConfig.SelectedChannels),
		"SelectedWhatsappChats":    len(whatsappConfig.SelectedChats),
		"SelectedWhatsappGroups":   len(whatsappConfig.SelectedGroups),
		"ForwardActive":            telegramConfig.Active,
		"ForwardHistory":           forwardHistory,
		"Success":                  success,
		"Error":                    errorMsg,
	})
}

// SaveTelegramForwardSettings - Menyimpan pengaturan penerusan pesan dari Telegram ke WhatsApp
func SaveTelegramForwardSettings(c *gin.Context) {
	// Ambil konfigurasi dari form
	forwardActive := c.PostForm("forward_active") == "on"

	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			Active: false,
		}
	}

	// Update konfigurasi
	telegramConfig.Active = forwardActive

	// Simpan konfigurasi
	if err := config.SaveTelegramConfig(telegramConfig); err != nil {
		c.Redirect(http.StatusFound, "/telegram/forward-settings?error=Gagal menyimpan konfigurasi: "+err.Error())
		return
	}

	// Update WebSocket clients dengan status baru
	WSManager.BroadcastMessage("telegramForward", gin.H{
		"active": telegramConfig.Active,
	})

	// Redirect kembali ke halaman pengaturan dengan pesan sukses
	c.Redirect(http.StatusFound, "/telegram/forward-settings?success=Pengaturan berhasil disimpan")
}
