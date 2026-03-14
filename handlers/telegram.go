package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/config"
)

// TelegramDashboard - Menampilkan dashboard Telegram
func TelegramDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "telegram/index.html", gin.H{
		"Title": "Telegram - TeleTowa",
	})
}

// TelegramConfigPage - Menampilkan halaman konfigurasi Telegram
func TelegramConfigPage(c *gin.Context) {
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

	c.HTML(http.StatusOK, "telegram/config.html", gin.H{
		"Title":          "Konfigurasi Telegram - TeleTowa",
		"TelegramConfig": telegramConfig,
		"UserConfigured": telegramConfig.UserMode,
	})
}

// SaveTelegramConfig - Menyimpan konfigurasi Telegram
func SaveTelegramConfig(c *gin.Context) {
	// Ambil konfigurasi dari form
	userMode := c.PostForm("user_mode") == "on"
	active := c.PostForm("active") == "on"

	// Ambil konfigurasi yang ada
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

	// Update konfigurasi
	telegramConfig.UserMode = userMode
	telegramConfig.Active = active

	// Simpan konfigurasi
	if err := config.SaveTelegramConfig(telegramConfig); err != nil {
		c.HTML(http.StatusInternalServerError, "telegram/config.html", gin.H{
			"Title":          "Konfigurasi Telegram - TeleTowa",
			"Error":          "Gagal menyimpan konfigurasi: " + err.Error(),
			"TelegramConfig": telegramConfig,
			"UserConfigured": telegramConfig.UserMode,
		})
		return
	}

	// Update WebSocket clients dengan status baru
	WSManager.BroadcastMessage("telegramConfig", gin.H{
		"active":          telegramConfig.Active,
		"user_configured": telegramConfig.UserMode,
	})

	// Redirect ke halaman Telegram
	c.Redirect(http.StatusFound, "/telegram")
}

// ListTelegramChats - Menampilkan daftar chat Telegram
func ListTelegramChats(c *gin.Context) {
	// Ambil konfigurasi Telegram untuk memeriksa apakah user mode aktif
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		c.HTML(http.StatusBadRequest, "telegram/chats.html", gin.H{
			"Title": "Daftar Chat Telegram - TeleTowa",
			"Error": "Gagal mendapatkan konfigurasi Telegram",
		})
		return
	}

	// Periksa apakah user mode aktif
	if !telegramConfig.UserMode {
		// Jika tidak dalam user mode, tampilkan pesan error
		c.HTML(http.StatusBadRequest, "telegram/chats.html", gin.H{
			"Title": "Daftar Chat Telegram - TeleTowa",
			"Error": "User Mode belum dikonfigurasi",
		})
		return
	}

	// Pastikan client tersedia dan user sudah login
	if telegramClient == nil {
		c.HTML(http.StatusBadRequest, "telegram/chats.html", gin.H{
			"Title": "Daftar Chat Telegram - TeleTowa",
			"Error": "Client Telegram belum diinisialisasi, silakan login terlebih dahulu",
		})
		return
	}

	// Buat context dengan timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Coba mendapatkan chat dari API
	chats, groups, channels, err := GetTelegramChats(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "telegram/chats.html", gin.H{
			"Title": "Daftar Chat Telegram - TeleTowa",
			"Error": "Gagal mendapatkan daftar chat: " + err.Error(),
		})
		return
	}

	// Tandai chat yang sudah dipilih
	for i, chat := range chats {
		for _, selectedID := range telegramConfig.SelectedChats {
			if chat["id"] == selectedID {
				chats[i]["selected"] = true
				break
			}
		}
	}

	// Tandai grup yang sudah dipilih
	for i, group := range groups {
		for _, selectedID := range telegramConfig.SelectedGroups {
			if group["id"] == selectedID {
				groups[i]["selected"] = true
				break
			}
		}
	}

	// Tandai channel yang sudah dipilih
	for i, channel := range channels {
		for _, selectedID := range telegramConfig.SelectedChannels {
			if channel["id"] == selectedID {
				channels[i]["selected"] = true
				break
			}
		}
	}

	// Hitung total yang dipilih
	selectedCount := len(telegramConfig.SelectedChats) + len(telegramConfig.SelectedGroups) + len(telegramConfig.SelectedChannels)

	c.HTML(http.StatusOK, "telegram/chats.html", gin.H{
		"Title":         "Daftar Chat Telegram - TeleTowa",
		"Chats":         chats,
		"Groups":        groups,
		"Channels":      channels,
		"SelectedCount": selectedCount,
	})
}

// SelectTelegramChat - Memilih chat Telegram untuk di-forward
func SelectTelegramChat(c *gin.Context) {
	// Ambil ID dan tipe dari form
	id := c.PostForm("id")
	chatType := c.PostForm("type")
	action := c.PostForm("action") // "select" atau "unselect"

	if id == "" || chatType == "" || action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Parameter tidak lengkap"})
		return
	}

	// Ambil konfigurasi Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil konfigurasi"})
		return
	}

	// Update daftar yang dipilih
	switch chatType {
	case "private":
		if action == "select" {
			// Cek apakah sudah dipilih
			alreadySelected := false
			for _, selectedID := range telegramConfig.SelectedChats {
				if selectedID == id {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				telegramConfig.SelectedChats = append(telegramConfig.SelectedChats, id)
			}
		} else {
			// Hapus dari daftar yang dipilih
			newSelectedChats := []string{}
			for _, selectedID := range telegramConfig.SelectedChats {
				if selectedID != id {
					newSelectedChats = append(newSelectedChats, selectedID)
				}
			}
			telegramConfig.SelectedChats = newSelectedChats
		}
	case "group":
		if action == "select" {
			// Cek apakah sudah dipilih
			alreadySelected := false
			for _, selectedID := range telegramConfig.SelectedGroups {
				if selectedID == id {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				telegramConfig.SelectedGroups = append(telegramConfig.SelectedGroups, id)
			}
		} else {
			// Hapus dari daftar yang dipilih
			newSelectedGroups := []string{}
			for _, selectedID := range telegramConfig.SelectedGroups {
				if selectedID != id {
					newSelectedGroups = append(newSelectedGroups, selectedID)
				}
			}
			telegramConfig.SelectedGroups = newSelectedGroups
		}
	case "channel":
		if action == "select" {
			// Cek apakah sudah dipilih
			alreadySelected := false
			for _, selectedID := range telegramConfig.SelectedChannels {
				if selectedID == id {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				telegramConfig.SelectedChannels = append(telegramConfig.SelectedChannels, id)
			}
		} else {
			// Hapus dari daftar yang dipilih
			newSelectedChannels := []string{}
			for _, selectedID := range telegramConfig.SelectedChannels {
				if selectedID != id {
					newSelectedChannels = append(newSelectedChannels, selectedID)
				}
			}
			telegramConfig.SelectedChannels = newSelectedChannels
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Tipe chat tidak valid"})
		return
	}

	// Simpan konfigurasi
	if err := config.SaveTelegramConfig(telegramConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal menyimpan konfigurasi"})
		return
	}

	// Update WebSocket clients
	WSManager.BroadcastMessage("telegramChatSelected", gin.H{
		"id":                id,
		"type":              chatType,
		"action":            action,
		"selected_chats":    len(telegramConfig.SelectedChats),
		"selected_groups":   len(telegramConfig.SelectedGroups),
		"selected_channels": len(telegramConfig.SelectedChannels),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"selected_chats":    len(telegramConfig.SelectedChats),
		"selected_groups":   len(telegramConfig.SelectedGroups),
		"selected_channels": len(telegramConfig.SelectedChannels),
	})
}
