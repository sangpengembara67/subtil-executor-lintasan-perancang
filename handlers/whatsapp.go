package handlers

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"github.com/shiestapoi/teletowa/config"
)

// Global variable untuk embedded filesystem dari main
var globalEmbeddedFS embed.FS

// SetEmbeddedFS sets the embedded filesystem for handlers
func SetEmbeddedFS(fs embed.FS) {
	globalEmbeddedFS = fs
}

// Global variables and SetServices are now defined in telegram_client.go to avoid redeclaration

// WhatsAppClient adalah klien WhatsApp global yang digunakan oleh aplikasi
var (
	WhatsAppClient     *whatsmeow.Client
	WaClientMutex      sync.Mutex
	WhatsAppQRChan     chan string
	WhatsAppInitiated  bool
	WhatsAppLastQRCode string
)

// InitWhatsAppClient menginisialisasi klien WhatsApp
func InitWhatsAppClient() error {
	if WhatsAppInitiated {
		return nil
	}

	WaClientMutex.Lock()
	defer WaClientMutex.Unlock()

	// Inisialisasi logger untuk whatsmeow
	dbLog := waLog.Stdout("Database", "INFO", true)
	clientLog := waLog.Stdout("Client", "INFO", true)

	// Buat direktori data jika belum ada
	os.MkdirAll("./data", 0755)

	// Inisialisasi database SQLite untuk whatsmeow
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:./data/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		return fmt.Errorf("gagal membuat container database WhatsApp: %v", err)
	}

	// Ambil atau buat device store baru
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("gagal mendapatkan device WhatsApp: %v", err)
	}

	platformType := waCompanionReg.DeviceProps_PlatformType(18)
	osName := "Roterium Bot"
	store.DeviceProps.PlatformType = &platformType
	store.DeviceProps.Os = &osName

	// Buat klien WhatsApp baru
	WhatsAppClient = whatsmeow.NewClient(deviceStore, clientLog)

	// Tambahkan handler pesan
	WhatsAppClient.EnableAutoReconnect = true
	WhatsAppClient.AutoTrustIdentity = true
	WhatsAppClient.AddEventHandler(whatsAppEventHandler)

	// Buat channel untuk QR code
	WhatsAppQRChan = make(chan string, 1)

	WhatsAppInitiated = true

	// Cek apakah sudah login atau perlu login baru
	whatsappConfig, _ := config.GetWhatsAppConfig()
	if whatsappConfig == nil {
		whatsappConfig = &config.WhatsAppConfig{
			PhoneNumber:    "",
			LoggedIn:       false,
			SelectedChats:  []string{},
			SelectedGroups: []string{},
			Active:         false,
		}
		config.SaveWhatsAppConfig(whatsappConfig)
	}

	// Jika sudah login sebelumnya, coba koneksikan
	if WhatsAppClient.Store.ID != nil {
		whatsappConfig.LoggedIn = true
		config.SaveWhatsAppConfig(whatsappConfig)

		go func() {
			err := WhatsAppClient.Connect()
			if err != nil {
				fmt.Printf("Error connecting WhatsApp: %v\n", err)
				// Reset status login jika koneksi gagal
				whatsappConfig.LoggedIn = false
				config.SaveWhatsAppConfig(whatsappConfig)
			}
		}()
	}

	return nil
}

// whatsAppEventHandler menangani event dari WhatsApp
func whatsAppEventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		senderID := v.Info.MessageSource.Sender.User
		chatID := v.Info.Chat.User
		messageContent := v.Message.GetConversation()

		// Jika tidak ada konten teks biasa, coba cek jenis pesan lain
		if messageContent == "" {
			if v.Message.GetImageMessage() != nil {
				messageContent = "[Gambar]"
			} else if v.Message.GetVideoMessage() != nil {
				messageContent = "[Video]"
			} else if v.Message.GetAudioMessage() != nil {
				messageContent = "[Audio]"
			} else if v.Message.GetDocumentMessage() != nil {
				messageContent = "[Dokumen]"
			} else if v.Message.GetStickerMessage() != nil {
				messageContent = "[Stiker]"
			} else if v.Message.GetContactMessage() != nil {
				messageContent = "[Kontak]"
			} else if v.Message.GetLocationMessage() != nil {
				messageContent = "[Lokasi]"
			} else {
				messageContent = "[Pesan lainnya]"
			}
			if v.Message == nil {
				messageContent = "[Diabaikan]"
			} else if v.Message.GetConversation() != "" || v.Message.GetExtendedTextMessage() != nil {
				messageContent = "[Teks]"
			} else if v.Message.GetImageMessage() != nil {
				messageContent = fmt.Sprintf("[Gambar %s]", v.Message.GetImageMessage().GetMimetype())
			} else if v.Message.GetStickerMessage() != nil {
				messageContent = fmt.Sprintf("[Stiker %s]", v.Message.GetStickerMessage().GetMimetype())
			} else if v.Message.GetVideoMessage() != nil {
				messageContent = fmt.Sprintf("[Video %s]", v.Message.GetVideoMessage().GetMimetype())
			} else if v.Message.GetAudioMessage() != nil {
				messageContent = fmt.Sprintf("[Audio %s]", v.Message.GetAudioMessage().GetMimetype())
			} else if v.Message.GetDocumentMessage() != nil {
				messageContent = fmt.Sprintf("[Dokumen %s]", v.Message.GetDocumentMessage().GetMimetype())
			} else if v.Message.GetContactMessage() != nil {
				messageContent = "[Kontak]"
			} else if v.Message.GetContactsArrayMessage() != nil {
				messageContent = "[Array Kontak]"
			} else if v.Message.GetLocationMessage() != nil {
				messageContent = "[Lokasi]"
			} else if v.Message.GetLiveLocationMessage() != nil {
				messageContent = "[Lokasi Langsung]"
			} else if v.Message.GetGroupInviteMessage() != nil {
				messageContent = "[Undangan Grup]"
			} else if v.Message.GetReactionMessage() != nil {
				if v.Message.GetReactionMessage().GetText() == "" {
					messageContent = "[Reaksi Dihapus]"
				} else {
					messageContent = "[Reaksi]"
				}
			} else if v.Message.GetPollCreationMessage() != nil {
				messageContent = "[Pembuatan Polling]"
			} else if v.Message.GetPollUpdateMessage() != nil {
				messageContent = "[Pembaruan Polling]"
			} else if v.Message.GetProtocolMessage() != nil {
				switch v.Message.GetProtocolMessage().GetType() {
				case waE2E.ProtocolMessage_REVOKE:
					messageContent = "[Pesan Dicabut]"
				case waE2E.ProtocolMessage_MESSAGE_EDIT:
					messageContent = "[Pesan Diedit]"
				case waE2E.ProtocolMessage_EPHEMERAL_SETTING:
					messageContent = "[Pengaturan Timer Menghilang]"
				default:
					messageContent = "[Protokol Tidak Dikenal]"
				}
			} else if v.Message.GetButtonsMessage() != nil {
				messageContent = "[Pesan Tombol]"
			} else if v.Message.GetTemplateMessage() != nil {
				messageContent = "[Template]"
			} else if v.Message.GetInteractiveMessage() != nil {
				messageContent = "[Interaktif]"
			} else if v.Message.GetListMessage() != nil {
				messageContent = "[Daftar]"
			} else if v.Message.GetProductMessage() != nil {
				messageContent = "[Produk]"
			} else if v.Message.GetOrderMessage() != nil {
				messageContent = "[Pesanan]"
			} else if v.Message.GetInvoiceMessage() != nil {
				messageContent = "[Invoice]"
			} else if v.Message.GetCall() != nil {
				messageContent = "[Panggilan]"
			} else if v.Message.GetChat() != nil {
				messageContent = "[Obrolan]"
			} else if v.Message.GetPlaceholderMessage() != nil {
				messageContent = "[Placeholder]"
			} else {
				messageContent = "[Pesan Tidak Dikenal]"
			}
		}
		// Dapatkan nama pengirim jika tersedia
		var senderName string
		if v.Info.MessageSource.Sender.Server != "" {
			if contact, err := WhatsAppClient.Store.Contacts.GetContact(context.Background(), v.Info.MessageSource.Sender); err == nil {
				senderName = contact.PushName
				if senderName == "" {
					senderName = contact.FullName
				}
			}
		}

		if senderName == "" {
			senderName = senderID
		}
		//debug
		// if chatID == "6282114586072" {
		// 	log.Printf("DEBUG: JSON data msg: %s", func() string {
		// 		data, _ := json.MarshalIndent(v, "", "  ")
		// 		return string(data)
		// 	}())
		// }

		color.New(color.FgHiGreen, color.Bold).Print("📱 [WhatsApp] ")
		color.New(color.FgHiCyan).Print("Pesan baru dari ")
		color.New(color.FgHiMagenta, color.Underline).Print(senderName)
		color.New(color.FgHiCyan).Print(" (")
		color.New(color.FgHiYellow).Print(chatID)
		color.New(color.FgHiCyan).Print(") • ")
		color.New(color.FgHiBlue).Println(v.Info.Timestamp.Local().Format("15:04:05"))
		color.New(color.FgHiWhite).Print("💬 ")
		color.New(color.FgHiGreen).Println(messageContent)

		// Tambahkan logging untuk extended message
		// if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil {
		// 	color.New(color.FgHiCyan).Print("↪️  Context Info: ")
		// 	extMsgJSON, _ := json.MarshalIndent(extMsg, "", "  ")
		// 	color.New(color.FgHiWhite).Println(string(extMsgJSON))
		// 	fmt.Println()
		// }

		// Ekstraksi dan analisis pesan otomatis
		if globalMediaExtractor != nil {
			go func() {
				if err := globalMediaExtractor.ExtractWhatsAppMessage(v); err != nil {
					fmt.Printf("Error extracting WhatsApp message: %v\n", err)
				}
			}()
		}

		// Meneruskan pesan ke platform lain jika aktif

	case *events.Connected:
		fmt.Println("WhatsApp connected")
		// Update status login
		whatsappConfig, _ := config.GetWhatsAppConfig()
		if whatsappConfig != nil {
			whatsappConfig.LoggedIn = true
			// Kirim pesan ke WhatsApp
			cfg := config.LoadConfig()
			chatID := cfg.PhoneOwner + "@s.whatsapp.net"
			formattedMessage := "> 🔔 *Event:* *Notification*\n💬 *Pesan: Whatsapp Connected*"
			if err := SendWhatsAppMessage(chatID, formattedMessage, ""); err != nil {
				fmt.Printf("Gagal mengirim ke %s: %v", chatID, err)
			}
			telegramConfig, _ := config.GetTelegramConfig()
			if telegramConfig != nil && telegramConfig.StatusConnected {
				formattedMessage := "> 🔔 *Event:* *Notification*\n💬 *Pesan: Telegram Connected*"
				if err := SendWhatsAppMessage(chatID, formattedMessage, ""); err != nil {
					fmt.Printf("Gagal mengirim ke %s: %v", chatID, err)
				}
			}
			whatsappConfig.PhoneNumber = WhatsAppClient.Store.ID.User
			config.SaveWhatsAppConfig(whatsappConfig)

			// Broadcast ke WebSocket
			WSManager.BroadcastMessage("whatsappLogin", gin.H{
				"logged_in": true,
			})
		}
	case *events.Disconnected:
		fmt.Println("WhatsApp disconnected")
	case *events.LoggedOut:
		fmt.Println("WhatsApp logged out")
		// Update status login
		whatsappConfig, _ := config.GetWhatsAppConfig()
		if whatsappConfig != nil {
			whatsappConfig.LoggedIn = false
			whatsappConfig.Active = false
			config.SaveWhatsAppConfig(whatsappConfig)

			// Broadcast ke WebSocket
			WSManager.BroadcastMessage("whatsappLogin", gin.H{
				"logged_in": false,
			})
		}
	}
}

// WhatsAppDashboard - Menampilkan dashboard WhatsApp
func WhatsAppDashboard(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		InitWhatsAppClient()
	}

	// Ambil konfigurasi WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		whatsappConfig = &config.WhatsAppConfig{
			PhoneNumber:    "",
			LoggedIn:       false,
			SelectedChats:  []string{},
			SelectedGroups: []string{},
			Active:         false,
		}
	}

	// Update status login berdasarkan WhatsAppClient
	if WhatsAppClient != nil && WhatsAppClient.Store.ID != nil {
		whatsappConfig.LoggedIn = true
	} else {
		whatsappConfig.LoggedIn = false
	}

	c.HTML(http.StatusOK, "whatsapp/index.html", gin.H{
		"Title":          "WhatsApp - TeleTowa",
		"WhatsAppConfig": whatsappConfig,
		"LoggedIn":       whatsappConfig.LoggedIn,
		"PhoneNumber":    whatsappConfig.PhoneNumber,
		"SelectedChats":  len(whatsappConfig.SelectedChats),
		"SelectedGroups": len(whatsappConfig.SelectedGroups),
	})
}

// WhatsAppLoginPage - Menampilkan halaman login WhatsApp
func WhatsAppLoginPage(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		InitWhatsAppClient()
	}

	// Ambil konfigurasi WhatsApp untuk cek status login
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		whatsappConfig = &config.WhatsAppConfig{
			LoggedIn: false,
		}
	}

	// Update status login berdasarkan WhatsAppClient
	if WhatsAppClient != nil && WhatsAppClient.Store.ID != nil {
		whatsappConfig.LoggedIn = true
	} else {
		whatsappConfig.LoggedIn = false
	}

	c.HTML(http.StatusOK, "whatsapp/login.html", gin.H{
		"Title":    "Login WhatsApp - TeleTowa",
		"LoggedIn": whatsappConfig.LoggedIn,
	})
}

// GenerateWhatsAppQR - Menghasilkan kode QR untuk login WhatsApp
func GenerateWhatsAppQR(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gagal menginisialisasi WhatsApp: %v", err)})
			return
		}
	}

	// Jika sudah login, kirim pesan bahwa tidak perlu QR code
	if WhatsAppClient.Store.ID != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": "Anda sudah login ke WhatsApp",
		})
		return
	}

	// Tutup klien yang ada jika ada
	if WhatsAppClient != nil {
		WhatsAppClient.Disconnect()
	}

	// Dapatkan channel QR code
	qrChan, _ := WhatsAppClient.GetQRChannel(context.Background())
	WhatsAppClient.EnableAutoReconnect = true
	WhatsAppClient.AutoTrustIdentity = true
	// Hubungkan ke WhatsApp
	go func() {
		err := WhatsAppClient.Connect()
		if err != nil {
			fmt.Printf("Error connecting to WhatsApp: %v\n", err)
		}
	}()

	// Tunggu QR code
	var qrCode string
	select {
	case evt := <-qrChan:
		if evt.Event == "code" {
			qrCode = evt.Code
			WhatsAppLastQRCode = qrCode

			// Buat QR code image
			qr, err := qrcode.Encode(qrCode, qrcode.Medium, 256)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghasilkan QR code"})
				return
			}

			// Konversi ke base64 untuk ditampilkan di browser
			qrBase64 := base64.StdEncoding.EncodeToString(qr)

			// Kirim response
			c.JSON(http.StatusOK, gin.H{
				"qr_code": "data:image/png;base64," + qrBase64,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan QR code dari WhatsApp"})
		}
	case <-c.Request.Context().Done():
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Waktu menunggu QR code habis"})
	}
}

// ProcessWhatsAppLogin - Memproses login WhatsApp
func ProcessWhatsAppLogin(c *gin.Context) {
	// Login diproses secara otomatis oleh whatsAppEventHandler
	// Fungsi ini dipanggil untuk cek status login

	// Ambil konfigurasi WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil konfigurasi"})
		return
	}

	// Cek status login dari WhatsApp client
	if WhatsAppClient != nil && WhatsAppClient.Store.ID != nil {
		whatsappConfig.LoggedIn = true
		config.SaveWhatsAppConfig(whatsappConfig)
		c.JSON(http.StatusOK, gin.H{"success": true})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Anda belum login ke WhatsApp",
		})
	}
}

// WhatsAppLogout - Logout dari WhatsApp
func WhatsAppLogout(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menginisialisasi WhatsApp"})
			return
		}
	}

	// Cek apakah sudah login
	if WhatsAppClient == nil || WhatsAppClient.Store.ID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Anda belum login ke WhatsApp",
		})
		return
	}

	// Logout dari WhatsApp
	err := WhatsAppClient.Logout(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Gagal logout: %v", err),
		})
		return
	}

	// Update konfigurasi
	whatsappConfig, _ := config.GetWhatsAppConfig()
	if whatsappConfig != nil {
		whatsappConfig.LoggedIn = false
		whatsappConfig.Active = false
		config.SaveWhatsAppConfig(whatsappConfig)
	}

	// Update WebSocket clients
	WSManager.BroadcastMessage("whatsappLogin", gin.H{
		"logged_in": false,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// ListWhatsAppChats - Menampilkan daftar chat WhatsApp
func ListWhatsAppChats(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.HTML(http.StatusInternalServerError, "whatsapp/chats.html", gin.H{
				"Title": "Daftar Chat WhatsApp - TeleTowa",
				"Error": "Gagal menginisialisasi WhatsApp",
			})
			return
		}
	}

	// Ambil konfigurasi WhatsApp untuk cek status login
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "whatsapp/chats.html", gin.H{
			"Title": "Daftar Chat WhatsApp - TeleTowa",
			"Error": "Gagal mengambil konfigurasi WhatsApp",
		})
		return
	}

	// Cek apakah sudah login
	if WhatsAppClient == nil || WhatsAppClient.Store.ID == nil || !whatsappConfig.LoggedIn {
		c.HTML(http.StatusBadRequest, "whatsapp/chats.html", gin.H{
			"Title": "Daftar Chat WhatsApp - TeleTowa",
			"Error": "Anda belum login ke WhatsApp",
		})
		return
	}

	// Ambil daftar kontak dari WhatsApp client
	chats := []gin.H{}

	// Mendapatkan kontak dari store
	if WhatsAppClient.Store.Contacts != nil {
		contacts, err := WhatsAppClient.Store.Contacts.GetAllContacts(context.Background())
		if err != nil {
			fmt.Printf("Error saat mengambil kontak: %v\n", err)
		} else {
			fmt.Printf("Jumlah kontak yang ditemukan: %d\n", len(contacts))

			// Memproses kontak yang ditemukan
			for jid, contact := range contacts {
				// Hanya tambahkan kontak pribadi (bukan grup)
				if jid.Server != "g.us" && jid.Server == "s.whatsapp.net" {
					displayName := contact.PushName
					if displayName == "" {
						displayName = contact.FullName
					}
					if displayName == "" {
						displayName = jid.User
					}

					// Cek apakah kontak sudah dipilih
					isSelected := false
					for _, selectedID := range whatsappConfig.SelectedChats {
						if selectedID == fmt.Sprintf("%s@%s", jid.User, jid.Server) {
							isSelected = true
							break
						}
					}

					chats = append(chats, gin.H{
						"id":       fmt.Sprintf("%s@%s", jid.User, jid.Server),
						"title":    displayName,
						"type":     "private",
						"selected": isSelected,
					})
				}
			}
		}
	} else {
		fmt.Println("WhatsApp contacts store is nil")
	}

	// Jika kontak kosong, tidak ada data dummy untuk demo
	if len(chats) == 0 {
		fmt.Println("Tidak ada kontak untuk ditampilkan")
	}

	// Ambil daftar grup
	groups := []gin.H{}

	// Mencoba mendapatkan grup jika tersedia melalui client
	if WhatsAppClient != nil {
		// Dapatkan daftar grup yang diikuti dari WhatsApp API
		joinedGroups, err := WhatsAppClient.GetJoinedGroups()
		if err != nil {
			fmt.Printf("Error saat mengambil daftar grup: %v\n", err)
		} else {
			fmt.Printf("Jumlah grup yang ditemukan: %d\n", len(joinedGroups))

			// Proses grup yang ditemukan
			for _, group := range joinedGroups {
				// Cek apakah grup sudah dipilih
				isSelected := false
				groupJID := fmt.Sprintf("%s@%s", group.JID.User, group.JID.Server)
				for _, selectedID := range whatsappConfig.SelectedGroups {
					if selectedID == groupJID {
						isSelected = true
						break
					}
				}

				groups = append(groups, gin.H{
					"id":       groupJID,
					"title":    group.Name,
					"type":     "group",
					"selected": isSelected,
				})
			}
		}
	}

	// Jika tidak ada grup yang ditemukan
	if len(groups) == 0 {
		fmt.Println("Tidak ada grup untuk ditampilkan")
	}

	c.HTML(http.StatusOK, "whatsapp/chats.html", gin.H{
		"Title":          "Pilih Kontak WhatsApp - TeleTowa",
		"Chats":          chats,
		"Groups":         groups,
		"SelectedChats":  whatsappConfig.SelectedChats,
		"SelectedGroups": whatsappConfig.SelectedGroups,
	})
}

// SelectWhatsAppChat - Memilih chat WhatsApp untuk tujuan forward
func SelectWhatsAppChat(c *gin.Context) {
	// Fungsi ini tidak berubah dari implementasi sebelumnya
	// Ambil ID dan tipe dari form
	id := c.PostForm("id")
	chatType := c.PostForm("type")
	action := c.PostForm("action") // "select" atau "unselect"

	if id == "" || chatType == "" || action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter tidak lengkap"})
		return
	}

	// Ambil konfigurasi WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil konfigurasi"})
		return
	}

	// Update daftar yang dipilih
	switch chatType {
	case "private":
		if action == "select" {
			// Cek apakah sudah dipilih
			alreadySelected := false
			for _, selectedID := range whatsappConfig.SelectedChats {
				if selectedID == id {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				whatsappConfig.SelectedChats = append(whatsappConfig.SelectedChats, id)
			}
		} else {
			// Hapus dari daftar yang dipilih
			newSelectedChats := []string{}
			for _, selectedID := range whatsappConfig.SelectedChats {
				if selectedID != id {
					newSelectedChats = append(newSelectedChats, selectedID)
				}
			}
			whatsappConfig.SelectedChats = newSelectedChats
		}
	case "group":
		if action == "select" {
			// Cek apakah sudah dipilih
			alreadySelected := false
			for _, selectedID := range whatsappConfig.SelectedGroups {
				if selectedID == id {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				whatsappConfig.SelectedGroups = append(whatsappConfig.SelectedGroups, id)
			}
		} else {
			// Hapus dari daftar yang dipilih
			newSelectedGroups := []string{}
			for _, selectedID := range whatsappConfig.SelectedGroups {
				if selectedID != id {
					newSelectedGroups = append(newSelectedGroups, selectedID)
				}
			}
			whatsappConfig.SelectedGroups = newSelectedGroups
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe chat tidak valid"})
		return
	}

	// Simpan konfigurasi
	if err := config.SaveWhatsAppConfig(whatsappConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan konfigurasi"})
		return
	}

	// Update WebSocket clients
	WSManager.BroadcastMessage("whatsappChatSelected", gin.H{
		"id":              id,
		"type":            chatType,
		"action":          action,
		"selected_chats":  len(whatsappConfig.SelectedChats),
		"selected_groups": len(whatsappConfig.SelectedGroups),
	})

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ToggleWhatsAppActive - Mengaktifkan atau menonaktifkan WhatsApp
func ToggleWhatsAppActive(c *gin.Context) {
	// Parse request JSON
	var request struct {
		Active bool `json:"active"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Gagal menginisialisasi WhatsApp",
			})
			return
		}
	}

	// Ambil konfigurasi WhatsApp
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Gagal mengambil konfigurasi WhatsApp",
		})
		return
	}

	// Periksa apakah sudah login
	if WhatsAppClient == nil || WhatsAppClient.Store.ID == nil || !whatsappConfig.LoggedIn {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Anda harus login ke WhatsApp terlebih dahulu",
		})
		return
	}

	// Update status active
	whatsappConfig.Active = request.Active

	// Simpan konfigurasi
	if err := config.SaveWhatsAppConfig(whatsappConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Gagal menyimpan konfigurasi WhatsApp",
		})
		return
	}

	// Update WebSocket clients
	WSManager.BroadcastMessage("whatsappActiveToggle", gin.H{
		"active": request.Active,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"active":  request.Active,
	})
}
func SendWhatsAppMessageCustom(recipient string, message string, quote string, replynumber string) error {
	if WhatsAppClient == nil {
		return fmt.Errorf("client WhatsApp belum diinisialisasi")
	}

	// Parse JID penerima menggunakan ParseJID langsung
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return fmt.Errorf("format JID tidak valid: %v", err)
	}

	// Validasi server
	if jid.Server != "s.whatsapp.net" && jid.Server != "g.us" {
		return fmt.Errorf("server tidak valid, harus s.whatsapp.net atau g.us")
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(message),
			ContextInfo: &waProto.ContextInfo{
				IsForwarded: proto.Bool(false),
				Participant: proto.String(replynumber + "@s.whatsapp.net"),
				QuotedMessage: &waProto.Message{
					Conversation: proto.String(quote),
				},
				StanzaID: proto.String(generateUniqueID()),
			},
			InviteLinkGroupTypeV2: waE2E.ExtendedTextMessage_DEFAULT.Enum(),
		},
	}

	// Kirim pesan
	_, err = WhatsAppClient.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return fmt.Errorf("gagal mengirim pesan: %v", err)
	}

	return nil
}

// SendWhatsAppMessage - Unified function to send WhatsApp messages (text or media)
func SendWhatsAppMessage(recipient string, message string, mediaPath string, options ...string) error {
	if WhatsAppClient == nil {
		return fmt.Errorf("client WhatsApp belum diinisialisasi")
	}

	// Parse JID penerima menggunakan ParseJID langsung
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return fmt.Errorf("format JID tidak valid: %v", err)
	}

	// Validasi server
	if jid.Server != "s.whatsapp.net" && jid.Server != "g.us" {
		return fmt.Errorf("server tidak valid, harus s.whatsapp.net atau g.us")
	}

	// Read media file if path is provided
	var mediaData []byte
	var mediaType string
	if mediaPath != "" {
		// Check if it's a file from globalEmbeddedFS (starts with "public/")
		if strings.HasPrefix(mediaPath, "public/") {
			mediaData, err = globalEmbeddedFS.ReadFile(mediaPath)
			if err != nil {
				return fmt.Errorf("gagal membaca file media dari globalEmbeddedFS: %v", err)
			}
		} else {
			// Read from file system
			mediaData, err = os.ReadFile(mediaPath)
			if err != nil {
				return fmt.Errorf("gagal membaca file media dari filesystem: %v", err)
			}
		}
		mediaType = detectMediaType(mediaData)
	}

	// log.Printf("DEBUG: JSON data: %s", func() string {
	// 	data, _ := json.MarshalIndent(uploaded1, "", "  ")
	// 	return string(data)
	// }())
	// fmt.Printf("DEBUG: thumb: %s\n", thumbnailImageBytes)
	// If no media data, send as text message
	if len(mediaData) == 0 || mediaType == "" {
		ContactName := "Telegram Message"
		ContactPhone := "639465441388"
		msgVCard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%v;;;\nFN:%v\nTEL;type=CELL;waid=%v:+%v\nEND:VCARD",
			ContactName, ContactName, ContactPhone, ContactPhone)
		msg := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(message),
				PreviewType: waE2E.ExtendedTextMessage_NONE.Enum(),
				ContextInfo: &waProto.ContextInfo{
					IsForwarded:     proto.Bool(true),
					ForwardingScore: proto.Uint32(999999),
					Participant:     proto.String("0@s.whatsapp.net"),
					QuotedMessage: &waProto.Message{
						ContactMessage: &waProto.ContactMessage{
							DisplayName: proto.String(ContactName),
							Vcard:       proto.String(msgVCard),
						},
					},
					RemoteJID: proto.String("status@broadcast"),
					StanzaID:  proto.String(generateUniqueID()),
				},
				InviteLinkGroupTypeV2: waE2E.ExtendedTextMessage_DEFAULT.Enum(),
			},
		}
		// Kirim pesan
		_, err = WhatsAppClient.SendMessage(context.Background(), jid, msg)
		if err != nil {
			return fmt.Errorf("gagal mengirim pesan: %v", err)
		}
		return nil
	}

	// Upload media to WhatsApp servers
	var mediaTypeEnum whatsmeow.MediaType
	switch mediaType {
	case "image":
		mediaTypeEnum = whatsmeow.MediaImage
	case "video":
		mediaTypeEnum = whatsmeow.MediaVideo
	case "audio":
		mediaTypeEnum = whatsmeow.MediaAudio
	case "document":
		mediaTypeEnum = whatsmeow.MediaDocument
	default:
		mediaTypeEnum = whatsmeow.MediaDocument
	}

	// Upload media
	uploaded, err := WhatsAppClient.Upload(context.Background(), mediaData, mediaTypeEnum)
	if err != nil {
		return fmt.Errorf("gagal upload media: %v", err)
	}

	// Create message based on media type
	var msg *waProto.Message
	ContactName := "Telegram Message"
	ContactPhone := "639465441388"
	msgVCard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%v;;;\nFN:%v\nTEL;type=CELL;waid=%v:+%v\nEND:VCARD",
		ContactName, ContactName, ContactPhone, ContactPhone)
	switch mediaType {
	case "image":
		msg = &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Caption:       proto.String(message),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(detectMimeType(mediaData)),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				ContextInfo: &waProto.ContextInfo{
					IsForwarded:     proto.Bool(true),
					ForwardingScore: proto.Uint32(999999),
					Participant:     proto.String("0@s.whatsapp.net"),
					QuotedMessage: &waProto.Message{
						ContactMessage: &waProto.ContactMessage{
							DisplayName: proto.String(ContactName),
							Vcard:       proto.String(msgVCard),
						},
					},
					RemoteJID: proto.String("status@broadcast"),
					StanzaID:  proto.String(generateUniqueID()),
				},
			},
		}
	case "video":
		msg = &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				Caption:       proto.String(message),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(detectMimeType(mediaData)),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				ContextInfo: &waProto.ContextInfo{
					IsForwarded:     proto.Bool(true),
					ForwardingScore: proto.Uint32(999999),
					Participant:     proto.String("0@s.whatsapp.net"),
					QuotedMessage: &waProto.Message{
						ContactMessage: &waProto.ContactMessage{
							DisplayName: proto.String(ContactName),
							Vcard:       proto.String(msgVCard),
						},
					},
					RemoteJID: proto.String("status@broadcast"),
					StanzaID:  proto.String(generateUniqueID()),
				},
			},
		}
	case "audio":
		msg = &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(detectMimeType(mediaData)),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				ContextInfo: &waProto.ContextInfo{
					IsForwarded:     proto.Bool(true),
					ForwardingScore: proto.Uint32(999999),
					Participant:     proto.String("0@s.whatsapp.net"),
					QuotedMessage: &waProto.Message{
						ContactMessage: &waProto.ContactMessage{
							DisplayName: proto.String(ContactName),
							Vcard:       proto.String(msgVCard),
						},
					},
					RemoteJID: proto.String("status@broadcast"),
					StanzaID:  proto.String(generateUniqueID()),
				},
			},
		}
	case "document":
		fileName := mediaPath
		msg = &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				Title:         proto.String(fileName),
				FileName:      proto.String(fileName),
				Caption:       proto.String(message),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(detectMimeType(mediaData)),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				ContextInfo: &waProto.ContextInfo{
					IsForwarded:     proto.Bool(true),
					ForwardingScore: proto.Uint32(999999),
					Participant:     proto.String("0@s.whatsapp.net"),
					QuotedMessage: &waProto.Message{
						ContactMessage: &waProto.ContactMessage{
							DisplayName: proto.String(ContactName),
							Vcard:       proto.String(msgVCard),
						},
					},
					RemoteJID: proto.String("status@broadcast"),
					StanzaID:  proto.String(generateUniqueID()),
				},
			},
		}
	default:
		// Fallback to text message if media type is unknown
		return SendWhatsAppMessage(recipient, message, "", options...)
	}

	// Send message
	_, err = WhatsAppClient.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return fmt.Errorf("gagal mengirim pesan media: %v", err)
	}

	return nil
}

// detectMediaType detects media type from byte data
func detectMediaType(data []byte) string {
	if len(data) < 12 {
		return "document"
	}

	// Check common image signatures
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image" // JPEG
	}
	if len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG" {
		return "image" // PNG
	}
	if len(data) >= 6 && string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a" {
		return "image" // GIF
	}

	// Check video signatures
	if len(data) >= 12 && string(data[4:12]) == "ftypmp4" {
		return "video" // MP4
	}
	if len(data) >= 4 && string(data[0:4]) == "RIFF" {
		return "video" // AVI or WebM
	}

	// Check audio signatures
	if len(data) >= 3 && string(data[0:3]) == "ID3" {
		return "audio" // MP3
	}
	if len(data) >= 4 && string(data[0:4]) == "OggS" {
		return "audio" // OGG
	}

	return "document"
}

// detectMimeType detects MIME type from byte data
func detectMimeType(data []byte) string {
	if len(data) < 12 {
		return "application/octet-stream"
	}

	// Check common image signatures
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG" {
		return "image/png"
	}
	if len(data) >= 6 && (string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a") {
		return "image/gif"
	}

	// Check video signatures
	if len(data) >= 12 && string(data[4:12]) == "ftypmp4" {
		return "video/mp4"
	}

	// Check audio signatures
	if len(data) >= 3 && string(data[0:3]) == "ID3" {
		return "audio/mpeg"
	}
	if len(data) >= 4 && string(data[0:4]) == "OggS" {
		return "audio/ogg"
	}

	return "application/octet-stream"
}

// SendWhatsAppMessageHandler - Handler untuk API pengiriman pesan
func SendWhatsAppMessageHandler(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Gagal menginisialisasi WhatsApp",
			})
			return
		}
	}

	// Cek apakah sudah login
	if WhatsAppClient == nil || WhatsAppClient.Store.ID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Anda belum login ke WhatsApp",
		})
		return
	}

	// Parse request
	var request struct {
		Recipient string `json:"recipient" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	// Gunakan SendWhatsAppMessage untuk mengirim pesan
	err := SendWhatsAppMessage(request.Recipient, request.Message, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Gagal mengirim pesan: %v", err),
		})
		return
	}

	// Berhasil
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pesan berhasil dikirim",
	})
}

// GetGroupInfo - Mendapatkan informasi detail grup WhatsApp
func GetGroupInfo(c *gin.Context) {
	// Periksa status client WhatsApp
	if WhatsAppClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "WhatsApp client belum diinisialisasi",
		})
		return
	}

	// Ambil GroupJID dari parameter query
	groupJID := c.Query("groupJID")
	if groupJID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Parameter groupJID diperlukan",
		})
		return
	}

	// Parse JID grup menggunakan types.ParseJID langsung
	jid, err := types.ParseJID(groupJID)
	if err != nil || jid.Server != "g.us" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format GroupJID tidak valid",
		})
		return
	}

	// Dapatkan informasi grup
	groupInfo, err := WhatsAppClient.GetGroupInfo(jid)
	if err != nil {
		errMsg := fmt.Sprintf("Gagal mendapatkan informasi grup: %v", err)
		fmt.Println(errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errMsg,
		})
		return
	}

	// Kembalikan informasi grup
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groupInfo,
	})
}

// GetGroupInviteLink - Mendapatkan tautan undangan untuk grup WhatsApp
func GetGroupInviteLink(c *gin.Context) {
	// Periksa status client WhatsApp
	if WhatsAppClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "WhatsApp client belum diinisialisasi",
		})
		return
	}

	// Ambil GroupJID dari parameter query
	groupJID := c.Query("groupJID")
	if groupJID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Parameter groupJID diperlukan",
		})
		return
	}

	// Ambil parameter reset
	resetParam := c.Query("reset")
	reset := false
	if resetParam != "" {
		var err error
		reset, err = strconv.ParseBool(resetParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Parameter reset tidak valid, harus true atau false",
			})
			return
		}
	}

	// Parse JID grup
	jid, err := types.ParseJID(groupJID)
	if err != nil || jid.Server != "g.us" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format GroupJID tidak valid",
		})
		return
	}

	// Dapatkan tautan undangan grup
	inviteLink, err := WhatsAppClient.GetGroupInviteLink(jid, reset)
	if err != nil {
		errMsg := fmt.Sprintf("Gagal mendapatkan tautan undangan grup: %v", err)
		fmt.Println(errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errMsg,
		})
		return
	}

	// Kembalikan tautan undangan
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"inviteLink": inviteLink,
	})
}

// SetGroupPhoto - Mengubah foto profil grup WhatsApp
func SetGroupPhoto(c *gin.Context) {
	// Periksa status client WhatsApp
	if WhatsAppClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "WhatsApp client belum diinisialisasi",
		})
		return
	}

	// Parse request body
	var request struct {
		GroupJID string `json:"groupJID" binding:"required"`
		Image    string `json:"image" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	// Validasi GroupJID
	jid, err := types.ParseJID(request.GroupJID)
	if err != nil || jid.Server != "g.us" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format GroupJID tidak valid",
		})
		return
	}

	// Decode gambar dari base64
	var filedata []byte
	if strings.HasPrefix(request.Image, "data:image/jp") {
		// Potong prefix dan decode base64
		parts := strings.Split(request.Image, ",")
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Format gambar tidak valid",
			})
			return
		}

		filedata, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Gagal decode data gambar",
			})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Data gambar harus dimulai dengan \"data:image/jpeg;base64,\"",
		})
		return
	}

	// Set foto grup
	pictureID, err := WhatsAppClient.SetGroupPhoto(jid, filedata)
	if err != nil {
		errMsg := fmt.Sprintf("Gagal mengubah foto grup: %v", err)
		fmt.Println(errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errMsg,
		})
		return
	}

	// Kembalikan hasil
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Foto grup berhasil diubah",
		"pictureID": pictureID,
	})
}

func PanelChat(c *gin.Context) {
	// Inisialisasi WhatsApp client jika belum
	if !WhatsAppInitiated {
		if err := InitWhatsAppClient(); err != nil {
			c.HTML(http.StatusInternalServerError, "whatsapp/panelchat.html", gin.H{
				"Title": "Daftar Chat WhatsApp - TeleTowa",
				"Error": "Gagal menginisialisasi WhatsApp",
			})
			return
		}
	}

	// Ambil konfigurasi WhatsApp untuk cek status login
	whatsappConfig, err := config.GetWhatsAppConfig()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "whatsapp/panelchat.html", gin.H{
			"Title": "Daftar Chat WhatsApp - TeleTowa",
			"Error": "Gagal mengambil konfigurasi WhatsApp",
		})
		return
	}

	// Cek apakah sudah login
	if WhatsAppClient == nil || WhatsAppClient.Store.ID == nil || !whatsappConfig.LoggedIn {
		c.HTML(http.StatusBadRequest, "whatsapp/panelchat.html", gin.H{
			"Title": "Daftar Chat WhatsApp - TeleTowa",
			"Error": "Anda belum login ke WhatsApp",
		})
		return
	}

	// Ambil daftar kontak dari WhatsApp client
	chats := []gin.H{}

	// Mendapatkan kontak dari store
	if WhatsAppClient.Store.Contacts != nil {
		contacts, err := WhatsAppClient.Store.Contacts.GetAllContacts(context.Background())
		if err != nil {
			fmt.Printf("Error saat mengambil kontak: %v\n", err)
		} else {
			fmt.Printf("Jumlah kontak yang ditemukan: %d\n", len(contacts))

			// Memproses kontak yang ditemukan
			for jid, contact := range contacts {
				// Hanya tambahkan kontak pribadi (bukan grup)
				if jid.Server != "g.us" && jid.Server == "s.whatsapp.net" {
					displayName := contact.PushName
					if displayName == "" {
						displayName = contact.FullName
					}
					if displayName == "" {
						displayName = jid.User
					}

					// Cek apakah kontak sudah dipilih
					isSelected := false
					for _, selectedID := range whatsappConfig.SelectedChats {
						if selectedID == fmt.Sprintf("%s@%s", jid.User, jid.Server) {
							isSelected = true
							break
						}
					}

					chats = append(chats, gin.H{
						"id":       fmt.Sprintf("%s@%s", jid.User, jid.Server),
						"title":    displayName,
						"type":     "private",
						"selected": isSelected,
					})
				}
			}
		}
	} else {
		fmt.Println("WhatsApp contacts store is nil")
	}

	// Jika kontak kosong, tidak ada data dummy untuk demo
	if len(chats) == 0 {
		fmt.Println("Tidak ada kontak untuk ditampilkan")
	}

	// Ambil daftar grup
	groups := []gin.H{}

	// Mencoba mendapatkan grup jika tersedia melalui client
	if WhatsAppClient != nil {
		// Dapatkan daftar grup yang diikuti dari WhatsApp API
		joinedGroups, err := WhatsAppClient.GetJoinedGroups()
		if err != nil {
			fmt.Printf("Error saat mengambil daftar grup: %v\n", err)
		} else {
			fmt.Printf("Jumlah grup yang ditemukan: %d\n", len(joinedGroups))

			// Proses grup yang ditemukan
			for _, group := range joinedGroups {
				// Cek apakah grup sudah dipilih
				isSelected := false
				groupJID := fmt.Sprintf("%s@%s", group.JID.User, group.JID.Server)
				for _, selectedID := range whatsappConfig.SelectedGroups {
					if selectedID == groupJID {
						isSelected = true
						break
					}
				}

				groups = append(groups, gin.H{
					"id":       groupJID,
					"title":    group.Name,
					"type":     "group",
					"selected": isSelected,
				})
			}
		}
	}

	// Jika tidak ada grup yang ditemukan
	if len(groups) == 0 {
		fmt.Println("Tidak ada grup untuk ditampilkan")
	}
	c.HTML(http.StatusOK, "whatsapp/panelchat.html", gin.H{
		"Title":  "Custom Send Chat - TeleTowa",
		"Chats":  chats,
		"Groups": groups,
	})
}

func PanelChatPost(c *gin.Context) {
	// Periksa status client WhatsApp
	if WhatsAppClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "WhatsApp client belum diinisialisasi",
		})
		return
	}

	// Parse request body
	var request struct {
		Recipient   string `form:"recipient" binding:"required"`
		Message     string `form:"message" binding:"required"`
		Quote       string `form:"quote" binding:"required"`
		ReplyNumber string `form:"replyNumber" binding:"required"`
	}
	if err := c.Bind(&request); err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}
	if err := SendWhatsAppMessageCustom(request.Recipient, request.Message, request.Quote, request.ReplyNumber); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pesan berhasil dikirim",
	})
}

func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
