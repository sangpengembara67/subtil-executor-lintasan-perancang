package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/go-faster/errors"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"github.com/shiestapoi/teletowa/config"
	"github.com/shiestapoi/teletowa/services"
)

var (
	telegramClient         *telegram.Client
	telegramClientLock     sync.Mutex
	telegramContext        context.Context
	telegramCancel         context.CancelFunc
	globalMediaExtractor   *services.MediaExtractor
	globalSchedulerService *services.SchedulerService
	globalAIAnalyzer       interface{}
)

// SetServices sets the global services for use in handlers
func SetServices(mediaExtractor *services.MediaExtractor, schedulerService *services.SchedulerService, aiAnalyzer interface{}) {
	globalMediaExtractor = mediaExtractor
	globalSchedulerService = schedulerService
	globalAIAnalyzer = aiAnalyzer
}

// MemStorage adalah implementasi sederhana dari PeerStorage menggunakan map di memori
type MemStorage struct {
	peers map[int64]tg.InputPeerClass
	mu    sync.RWMutex
}

// NewMemStorage membuat instance baru MemStorage
func NewMemStorage() *MemStorage {
	return &MemStorage{
		peers: make(map[int64]tg.InputPeerClass),
	}
}

// Save menyimpan peer ke storage
func (s *MemStorage) Save(ctx context.Context, id int64, peer tg.InputPeerClass) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[id] = peer
	return nil
}

// Find mencari peer dari storage
func (s *MemStorage) Find(ctx context.Context, id int64) (tg.InputPeerClass, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[id]
	return p, ok
}

// DebugUpdateWrapper wraps standard dispatcher with debug
func DebugUpdateWrapper(dispatcher tg.UpdateDispatcher) {
	// Gunakan wrapper function untuk debugging
	log.Println("INFO: Menambahkan debug wrapper ke dispatcher")
}

// InitTelegramUserClient - Inisialisasi client Telegram sebagai user (bukan bot)
func InitTelegramUserClient() error {
	telegramClientLock.Lock()
	defer telegramClientLock.Unlock()

	// Ambil konfigurasi Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		return fmt.Errorf("failed to get Telegram config: %w", err)
	}

	// Cek jika konfigurasi user mode aktif
	if !telegramConfig.UserMode || telegramConfig.ApiID == 0 || telegramConfig.ApiHash == "" {
		return fmt.Errorf("telegram user mode not configured correctly")
	}

	// Pastikan direktori session ada
	sessionDir := filepath.Dir(telegramConfig.SessionFile)
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Buat session storage
	sessionStorage := &session.FileStorage{Path: telegramConfig.SessionFile}

	// Setup penanganan updates
	dispatcher := tg.NewUpdateDispatcher()
	DebugUpdateWrapper(dispatcher)
	log.Println("INFO: Mendaftarkan handlers untuk updates")

	// Helper function untuk mendeteksi command atau mention
	commandHandler := func(_ context.Context, e tg.Entities, msg *tg.Message) error {
		// Cek apakah pesan ini dari chat yang dipilih untuk di-forward
		telegramConfig, _ := config.GetTelegramConfig()
		shouldForward := false

		if telegramConfig != nil && telegramConfig.Active {
			// Konversi ID peer ke string untuk perbandingan
			var peerIDStr string
			if msg.PeerID != nil {
				switch peer := msg.PeerID.(type) {
				case *tg.PeerUser:
					peerIDStr = fmt.Sprintf("%d", peer.UserID)
					// Cek apakah ada di SelectedChats
					for _, selectedID := range telegramConfig.SelectedChats {
						if selectedID == peerIDStr {
							shouldForward = true
							break
						}
					}
				case *tg.PeerChat:
					peerIDStr = fmt.Sprintf("%d", peer.ChatID)
					// Cek apakah ada di SelectedGroups
					for _, selectedID := range telegramConfig.SelectedGroups {
						if selectedID == peerIDStr {
							shouldForward = true
							break
						}
					}
				case *tg.PeerChannel:
					peerIDStr = fmt.Sprintf("%d", peer.ChannelID)
					// Cek apakah ada di SelectedChannels
					for _, selectedID := range telegramConfig.SelectedChannels {
						if selectedID == peerIDStr {
							shouldForward = true
							break
						}
					}
				}
			}

			// Jika pesan tidak termasuk dalam daftar chat yang dipilih, keluar dari handler
			if !shouldForward {
				return nil
			}
		} else {
			return nil // Tidak perlu memproses pesan jika forwarding tidak aktif
		}
		//debug
		// log.Printf("DEBUG: JSON data msg: %s", func() string {
		// 	data, _ := json.MarshalIndent(msg, "", "  ")
		// 	return string(data)
		// }())

		// Format dan catat pesan
		var fromInfo, chatInfo string

		// Dapatkan informasi pengirim
		if msg.FromID != nil {
			switch from := msg.FromID.(type) {
			case *tg.PeerUser:
				// Coba dapatkan info user dari entities
				if user, ok := e.Users[from.UserID]; ok {
					username := user.Username
					if username == "" {
						username = "no_username"
					}
					lastName := user.LastName
					if lastName == "" {
						fromInfo = fmt.Sprintf("%s (@%s)", user.FirstName, username)
					} else {
						fromInfo = fmt.Sprintf("%s %s (@%s)", user.FirstName, lastName, username)
					}
				} else {
					fromInfo = fmt.Sprintf("User %d", from.UserID)
				}
			case *tg.PeerChannel:
				// Handle channel posts
				if channel, ok := e.Channels[from.ChannelID]; ok {
					fromInfo = fmt.Sprintf("Channel: %s", channel.Title)
				} else {
					fromInfo = fmt.Sprintf("Channel %d", from.ChannelID)
				}
			case *tg.PeerChat:
				// Handle group messages
				if chat, ok := e.Chats[from.ChatID]; ok {
					fromInfo = fmt.Sprintf("Group: %s", chat.Title)
				} else {
					fromInfo = fmt.Sprintf("Group %d", from.ChatID)
				}
			default:
				fromInfo = "Unknown sender"
			}
		} else {
			fromInfo = "System"
		}

		// Dapatkan informasi chat
		if msg.PeerID != nil {
			switch peer := msg.PeerID.(type) {
			case *tg.PeerUser:
				if user, ok := e.Users[peer.GetUserID()]; ok {
					chatInfo = fmt.Sprintf("Private chat with %s %s (@%s)", user.FirstName, user.LastName, user.Username)
				} else {
					chatInfo = fmt.Sprintf("Private chat with User %d", peer.UserID)
				}
			case *tg.PeerChat:
				if chat, ok := e.Chats[peer.ChatID]; ok {
					chatInfo = fmt.Sprintf("Group '%s'", chat.Title)
				} else {
					chatInfo = fmt.Sprintf("Group %d", peer.ChatID)
				}
			case *tg.PeerChannel:
				if channel, ok := e.Channels[peer.ChannelID]; ok {
					if channel.Broadcast {
						chatInfo = fmt.Sprintf("Channel '%s'", channel.Title)
					} else {
						chatInfo = fmt.Sprintf("Supergroup '%s'", channel.Title)
					}
				} else {
					chatInfo = fmt.Sprintf("Channel %d", peer.ChannelID)
				}
			default:
				chatInfo = "Unknown chat"
			}
		} else {
			chatInfo = "Unknown chat"
		}

		color.New(color.FgHiMagenta, color.Bold).Print("🤖 [Telegram] ")
		color.New(color.FgHiCyan).Printf("Menerima %s dari ", msg.PeerID.TypeName())
		color.New(color.FgHiYellow, color.Bold).Printf("%s", fromInfo)
		color.New(color.FgHiWhite).Print(" • ")
		color.New(color.FgHiGreen).Printf("%s\n", time.Now().Format("15:04:05"))
		color.New(color.FgHiWhite).Print("💬 ")
		color.New(color.FgHiMagenta, color.Bold).Printf("%s\n", msg.Message)

		// Kirim pesan ke WhatsApp jika forwarding aktif

		// Hanya forward jika chat dipilih dan WhatsApp aktif
		whatsappConfig, _ := config.GetWhatsAppConfig()
		if shouldForward && whatsappConfig != nil && whatsappConfig.Active && whatsappConfig.LoggedIn {
			// Format pesan berdasarkan tipe konten
			var messageContent string
			var forwardedInfo string

			// Ambil informasi forwarded jika ada
			if msg.FwdFrom.FromID != nil {
				switch f := msg.FwdFrom.FromID.(type) {
				case *tg.PeerUser:
					if user, ok := e.Users[f.UserID]; ok {
						forwardedInfo = fmt.Sprintf("%s %s (@%s)", user.FirstName, user.LastName, user.Username)
					} else {
						forwardedInfo = fmt.Sprintf("User %d", f.UserID)
					}
				case *tg.PeerChannel:
					if channel, ok := e.Channels[f.ChannelID]; ok {
						forwardedInfo = fmt.Sprintf("Channel: %s", channel.Title)
					} else {
						forwardedInfo = fmt.Sprintf("Channel %d", f.ChannelID)
					}
				case *tg.PeerChat:
					if chat, ok := e.Chats[f.ChatID]; ok {
						forwardedInfo = fmt.Sprintf("Group: %s", chat.Title)
					} else {
						forwardedInfo = fmt.Sprintf("Group %d", f.ChatID)
					}
				default:
					forwardedInfo = "Unknown source"
				}
			}

			// Kirim ke semua chat dan grup WhatsApp yang dipilih
			var targets []string
			targets = append(targets, whatsappConfig.SelectedChats...)
			targets = append(targets, whatsappConfig.SelectedGroups...)

			// Tentukan tipe pesan dan format konten
			if msg.Media != nil {
				switch media := msg.Media.(type) {
				case *tg.MessageMediaPhoto:
					// Download foto dari Telegram
					if forwardedInfo != "" {
						messageContent = fmt.Sprintf("📷 *[Foto Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", forwardedInfo, chatInfo, msg.Message)
					} else {
						messageContent = fmt.Sprintf("👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", fromInfo, chatInfo, msg.Message)
					}

					// Download dan kirim foto
					photo := media.Photo.(*tg.Photo)
					if len(photo.Sizes) > 0 {
						// Cari ukuran foto terbesar yang tersedia
						var largestSize tg.PhotoSizeClass
						var maxSize int

						for _, size := range photo.Sizes {
							switch s := size.(type) {
							case *tg.PhotoSize:
								if s.Size > maxSize {
									maxSize = s.Size
									largestSize = s
								}
							case *tg.PhotoSizeProgressive:
								// PhotoSizeProgressive memiliki array sizes, ambil yang terbesar
								if len(s.Sizes) > 0 {
									totalSize := 0
									for _, progSize := range s.Sizes {
										totalSize += progSize
									}
									if totalSize > maxSize {
										maxSize = totalSize
										largestSize = s
									}
								}
							case *tg.PhotoStrippedSize:
								// PhotoStrippedSize biasanya berukuran kecil, tapi tetap pertimbangkan
								if len(s.Bytes) > maxSize {
									maxSize = len(s.Bytes)
									largestSize = s
								}
							case *tg.PhotoCachedSize:
								// PhotoCachedSize untuk thumbnail kecil
								if len(s.Bytes) > maxSize {
									maxSize = len(s.Bytes)
									largestSize = s
								}
							case *tg.PhotoPathSize:
								// PhotoPathSize untuk vector path, skip untuk download
								continue
							}
						}

						// Tangani berbagai tipe PhotoSize
						var location tg.InputFileLocationClass
						var thumbSize string

						switch s := largestSize.(type) {
						case *tg.PhotoSize:
							thumbSize = s.Type
							location = &tg.InputPhotoFileLocation{
								ID:            photo.ID,
								AccessHash:    photo.AccessHash,
								FileReference: photo.FileReference,
								ThumbSize:     thumbSize,
							}
						case *tg.PhotoSizeProgressive:
							thumbSize = s.Type
							location = &tg.InputPhotoFileLocation{
								ID:            photo.ID,
								AccessHash:    photo.AccessHash,
								FileReference: photo.FileReference,
								ThumbSize:     thumbSize,
							}
						case *tg.PhotoStrippedSize:
							// PhotoStrippedSize sudah berisi data gambar, langsung gunakan
							tempFile := fmt.Sprintf("temp_photo_%d_stripped.jpg", photo.ID)
							if err := os.WriteFile(tempFile, s.Bytes, 0644); err == nil {
								// Kirim foto stripped dengan media
								for _, target := range targets {
									log.Printf("DEBUG: Menggunakan stripped photo: %s", tempFile)
									if err := SendWhatsAppMessage(target, messageContent, tempFile, "Telegram"); err != nil {
										color.New(color.FgHiRed).Printf("❌ Gagal mengirim foto stripped ke WhatsApp %s: %v\n", target, err)
									} else {
										color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim foto stripped ke WhatsApp %s\n", target)
									}
								}
								os.Remove(tempFile) // Hapus file temp
								break               // Keluar dari case untuk menghindari download ulang
							}
						case *tg.PhotoCachedSize:
							// PhotoCachedSize sudah berisi data gambar, langsung gunakan
							tempFile := fmt.Sprintf("temp_photo_%d_cached.jpg", photo.ID)
							if err := os.WriteFile(tempFile, s.Bytes, 0644); err == nil {
								// Kirim foto cached dengan media
								for _, target := range targets {
									log.Printf("DEBUG: Menggunakan cached photo: %s", tempFile)
									if err := SendWhatsAppMessage(target, messageContent, tempFile, "Telegram"); err != nil {
										color.New(color.FgHiRed).Printf("❌ Gagal mengirim foto cached ke WhatsApp %s: %v\n", target, err)
									} else {
										color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim foto cached ke WhatsApp %s\n", target)
									}
								}
								os.Remove(tempFile) // Hapus file temp
								break               // Keluar dari case untuk menghindari download ulang
							}
						default:
							log.Printf("DEBUG: Unsupported photo size type: %T", largestSize)
							location = nil
						}

						// Download foto jika memerlukan download dari server
						if location != nil {
							tempFile := fmt.Sprintf("temp_photo_%d_%s.jpg", photo.ID, thumbSize)
							log.Printf("DEBUG: Attempting to download photo to: %s", tempFile)

							if err := downloadTelegramMedia(telegramClient, location, tempFile); err == nil {
								// Kirim foto dengan media
								for _, target := range targets {
									log.Printf("DEBUG: Sending downloaded photo: %s to %s", tempFile, target)
									if err := SendWhatsAppMessage(target, messageContent, tempFile, "Telegram"); err != nil {
										color.New(color.FgHiRed).Printf("❌ Gagal mengirim foto ke WhatsApp %s: %v\n", target, err)
									} else {
										color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim foto ke WhatsApp %s\n", target)
									}
								}
								os.Remove(tempFile) // Hapus file temp
							} else {
								log.Printf("DEBUG: Photo download failed: %v", err)
								// Fallback ke teks jika download gagal
								for _, target := range targets {
									if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
										color.New(color.FgHiRed).Printf("❌ Gagal mengirim foto (fallback) ke WhatsApp %s: %v\n", target, err)
									} else {
										color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim foto (fallback) ke WhatsApp %s\n", target)
									}
								}
							}
						}
					} else {
						log.Printf("DEBUG: Photo has no sizes available")
						// Fallback ke teks jika tidak ada sizes
						for _, target := range targets {
							if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
								color.New(color.FgHiRed).Printf("❌ Gagal mengirim foto ke WhatsApp %s: %v\n", target, err)
							} else {
								color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim foto ke WhatsApp %s\n", target)
							}
						}
					}

				case *tg.MessageMediaDocument:
					doc := media.Document.(*tg.Document)

					// Ambil nama file jika ada
					var fileName string
					for _, attr := range doc.Attributes {
						if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
							fileName = fn.FileName
							break
						}
					}
					if fileName == "" {
						fileName = "Unknown file"
					}

					if forwardedInfo != "" {
						messageContent = fmt.Sprintf("*[Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", forwardedInfo, chatInfo, msg.Message)
					} else {
						messageContent = fmt.Sprintf("👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", fromInfo, chatInfo, msg.Message)
					}

					// Download dan kirim dokumen
					location := &tg.InputDocumentFileLocation{
						ID:            doc.ID,
						AccessHash:    doc.AccessHash,
						FileReference: doc.FileReference,
					}

					if err := downloadTelegramDocument(telegramClient, location, fileName); err == nil {
						// Kirim dokumen dengan media
						for _, target := range targets {
							if err := SendWhatsAppMessage(target, messageContent, fileName, "Telegram"); err != nil {
								color.New(color.FgHiRed).Printf("❌ Gagal mengirim ke WhatsApp %s: %v\n", target, err)
							} else {
								color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim ke WhatsApp %s\n", target)
							}
						}
						os.Remove(fileName) // Hapus file temp
					} else {
						// Fallback ke teks jika download gagal
						for _, target := range targets {
							if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
								color.New(color.FgHiRed).Printf("❌ Gagal mengirim ke WhatsApp %s: %v\n", target, err)
							} else {
								color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim ke WhatsApp %s\n", target)
							}
						}
					}

				case *tg.MessageMediaContact:
					contact := media.FirstName + " " + media.LastName
					if media.PhoneNumber != "" {
						contact += " (" + media.PhoneNumber + ")"
					}
					if forwardedInfo != "" {
						messageContent = fmt.Sprintf("👤 *[Kontak Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n📞 *Kontak:* %s", forwardedInfo, chatInfo, contact)
					} else {
						messageContent = fmt.Sprintf("👤 *[Kontak]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n📞 *Kontak:* %s", fromInfo, chatInfo, contact)
					}

					// Kirim kontak sebagai teks biasa
					for _, target := range targets {
						if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
							color.New(color.FgHiRed).Printf("❌ Gagal mengirim kontak ke WhatsApp %s: %v\n", target, err)
						} else {
							color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim kontak ke WhatsApp %s\n", target)
						}
					}

				case *tg.MessageMediaGeo:
					geoPoint, ok := media.Geo.(*tg.GeoPoint)
					if !ok {
						break
					}
					if forwardedInfo != "" {
						messageContent = fmt.Sprintf("📍 *[Lokasi Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n🌍 *Koordinat:* %f, %f", forwardedInfo, chatInfo, geoPoint.Lat, geoPoint.Long)
					} else {
						messageContent = fmt.Sprintf("📍 *[Lokasi]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n🌍 *Koordinat:* %f, %f", fromInfo, chatInfo, geoPoint.Lat, geoPoint.Long)
					}

					// Kirim lokasi sebagai teks
					for _, target := range targets {
						if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
							color.New(color.FgHiRed).Printf("❌ Gagal mengirim lokasi ke WhatsApp %s: %v\n", target, err)
						} else {
							color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim lokasi ke WhatsApp %s\n", target)
						}
					}

				default:
					if forwardedInfo != "" {
						messageContent = fmt.Sprintf("📎 *[Media Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", forwardedInfo, chatInfo, msg.Message)
					} else {
						messageContent = fmt.Sprintf("📎 *[Media]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", fromInfo, chatInfo, msg.Message)
					}

					// Kirim sebagai teks biasa untuk media yang tidak dikenal
					for _, target := range targets {
						if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
							color.New(color.FgHiRed).Printf("❌ Gagal mengirim media ke WhatsApp %s: %v\n", target, err)
						} else {
							color.New(color.FgHiGreen).Printf("✅ Berhasil mengirim media ke WhatsApp %s\n", target)
						}
					}
				}
			} else {
				// Pesan teks biasa
				if forwardedInfo != "" {
					messageContent = fmt.Sprintf("💬 *[Pesan Diteruskan]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", forwardedInfo, chatInfo, msg.Message)
				} else {
					messageContent = fmt.Sprintf("💬 *[Pesan Baru]*\n👤 *Dari:* %s\n💬 *Chat:* %s\n-----------\n%s", fromInfo, chatInfo, msg.Message)
				}

				// Kirim teks ke WhatsApp
				for _, target := range targets {
					if err := SendWhatsAppMessage(target, messageContent, "", "Telegram"); err != nil {
						color.New(color.FgHiRed).Printf("❌ Gagal mengirim ke WhatsApp %s: %v\n", target, err)
					} else {
						color.New(color.FgHiGreen).Printf("✅ Berhasil diteruskan ke WhatsApp %s\n", target)
					}
				}
			}
		}

		// Ekstraksi dan analisis pesan otomatis
		if globalMediaExtractor != nil {
			go func() {
				if err := globalMediaExtractor.ExtractTelegramMessage(msg, e); err != nil {
					log.Printf("Error extracting Telegram message: %v", err)
				}
			}()
		}

		// Kirim notifikasi melalui WebSocket
		WSManager.BroadcastMessage("telegramNotification", gin.H{
			"from":    fromInfo,
			"chat":    chatInfo,
			"content": msg.Message,
			"type":    msg.PeerID.TypeName(),
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})

		return nil
	}

	// Tambahkan handler pesan biasa (private chats and groups, NOT channels)
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		//debug
		fmt.Println("DEBUG: Received new message update")
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}

		// Skip channel messages here since they're handled by OnNewChannelMessage
		if msg.PeerID != nil {
			if _, isChannel := msg.PeerID.(*tg.PeerChannel); isChannel {
				fmt.Println("DEBUG: Skipping channel message in OnNewMessage handler")
				return nil
			}
		}

		// Pastikan entities terisi dengan lengkap
		if len(e.Users) == 0 {
			if msg.FromID != nil {
				switch from := msg.FromID.(type) {
				case *tg.PeerUser:
					// Dapatkan info user dari API
					users, err := telegramClient.API().UsersGetUsers(ctx, []tg.InputUserClass{
						&tg.InputUser{UserID: from.UserID, AccessHash: 0},
					})
					if err == nil && len(users) > 0 {
						for _, user := range users {
							if u, ok := user.(*tg.User); ok {
								if e.Users == nil {
									e.Users = make(map[int64]*tg.User)
								}
								e.Users[u.ID] = u
							}
						}
					}
				}
			}
		}

		return commandHandler(ctx, e, msg)
	})

	// Tambahkan handler pesan channel
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		fmt.Println("DEBUG: Received new channel message update")
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}

		// Pastikan entities terisi dengan lengkap untuk channel messages
		if len(e.Users) == 0 || len(e.Channels) == 0 {
			if msg.FromID != nil {
				switch from := msg.FromID.(type) {
				case *tg.PeerUser:
					// Dapatkan info user dari API
					users, err := telegramClient.API().UsersGetUsers(ctx, []tg.InputUserClass{
						&tg.InputUser{UserID: from.UserID, AccessHash: 0},
					})
					if err == nil && len(users) > 0 {
						for _, user := range users {
							if u, ok := user.(*tg.User); ok {
								if e.Users == nil {
									e.Users = make(map[int64]*tg.User)
								}
								e.Users[u.ID] = u
							}
						}
					}
				case *tg.PeerChannel:
					// Dapatkan info channel dari API
					channels, err := telegramClient.API().ChannelsGetChannels(ctx, []tg.InputChannelClass{
						&tg.InputChannel{ChannelID: from.ChannelID, AccessHash: 0},
					})
					if err == nil {
						switch c := channels.(type) {
						case *tg.MessagesChats:
							for _, chat := range c.Chats {
								if ch, ok := chat.(*tg.Channel); ok {
									if e.Channels == nil {
										e.Channels = make(map[int64]*tg.Channel)
									}
									e.Channels[ch.ID] = ch
								}
							}
						}
					}
				}
			}
		}

		return commandHandler(ctx, e, msg)
	})

	// Tambahkan handler untuk pesan yang di edit
	// dispatcher.OnEditMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateEditMessage) error {
	// 	fmt.Println("DEBUG: Received edit message update")
	// 	msg, ok := update.Message.(*tg.Message)
	// 	if !ok {
	// 		return nil
	// 	}

	// 	// Pastikan entities terisi dengan lengkap untuk channel messages
	// 	if len(e.Users) == 0 || len(e.Channels) == 0 {
	// 		if msg.FromID != nil {
	// 			switch from := msg.FromID.(type) {
	// 			case *tg.PeerUser:
	// 				// Dapatkan info user dari API
	// 				users, err := telegramClient.API().UsersGetUsers(ctx, []tg.InputUserClass{
	// 					&tg.InputUser{UserID: from.UserID, AccessHash: 0},
	// 				})
	// 				if err == nil && len(users) > 0 {
	// 					for _, user := range users {
	// 						if u, ok := user.(*tg.User); ok {
	// 							if e.Users == nil {
	// 								e.Users = make(map[int64]*tg.User)
	// 							}
	// 							e.Users[u.ID] = u
	// 						}
	// 					}
	// 				}
	// 			case *tg.PeerChannel:
	// 				// Dapatkan info channel dari API
	// 				channels, err := telegramClient.API().ChannelsGetChannels(ctx, []tg.InputChannelClass{
	// 					&tg.InputChannel{ChannelID: from.ChannelID, AccessHash: 0},
	// 				})
	// 				if err == nil {
	// 					switch c := channels.(type) {
	// 					case *tg.MessagesChats:
	// 						for _, chat := range c.Chats {
	// 							if ch, ok := chat.(*tg.Channel); ok {
	// 								if e.Channels == nil {
	// 									e.Channels = make(map[int64]*tg.Channel)
	// 								}
	// 								e.Channels[ch.ID] = ch
	// 							}
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}

	// 	return commandHandler(ctx, e, msg)
	// })

	// Tambahkan handler untuk pesan yang diedit di channel
	// dispatcher.OnEditChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateEditChannelMessage) error {
	// 	fmt.Println("DEBUG: Received edit channel message update")
	// 	msg, ok := update.Message.(*tg.Message)
	// 	if !ok {
	// 		return nil
	// 	}

	// 	// Pastikan entities terisi dengan lengkap untuk channel messages
	// 	if len(e.Users) == 0 || len(e.Channels) == 0 {
	// 		if msg.FromID != nil {
	// 			switch from := msg.FromID.(type) {
	// 			case *tg.PeerUser:
	// 				// Dapatkan info user dari API
	// 				users, err := telegramClient.API().UsersGetUsers(ctx, []tg.InputUserClass{
	// 					&tg.InputUser{UserID: from.UserID, AccessHash: 0},
	// 				})
	// 				if err == nil && len(users) > 0 {
	// 					for _, user := range users {
	// 						if u, ok := user.(*tg.User); ok {
	// 							if e.Users == nil {
	// 								e.Users = make(map[int64]*tg.User)
	// 							}
	// 							e.Users[u.ID] = u
	// 						}
	// 					}
	// 				}
	// 			case *tg.PeerChannel:
	// 				// Dapatkan info channel dari API
	// 				channels, err := telegramClient.API().ChannelsGetChannels(ctx, []tg.InputChannelClass{
	// 					&tg.InputChannel{ChannelID: from.ChannelID, AccessHash: 0},
	// 				})
	// 				if err == nil {
	// 					switch c := channels.(type) {
	// 					case *tg.MessagesChats:
	// 						for _, chat := range c.Chats {
	// 							if ch, ok := chat.(*tg.Channel); ok {
	// 								if e.Channels == nil {
	// 									e.Channels = make(map[int64]*tg.Channel)
	// 								}
	// 								e.Channels[ch.ID] = ch
	// 							}
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}

	// 	return commandHandler(ctx, e, msg)
	// })

	// Setup penyimpanan peer (untuk cache entitas)
	_ = NewMemStorage()
	log.Println("INFO: Update handlers telah didaftarkan")

	// Buat opsi Telegram dengan semua komponen yang diatur
	opts := telegram.Options{
		SessionStorage: sessionStorage,
		UpdateHandler:  dispatcher,
		Device: telegram.DeviceConfig{
			DeviceModel:    "TeleTowa App",
			SystemVersion:  "Windows 10",
			AppVersion:     "1.0.0",
			SystemLangCode: "id",
			LangPack:       "id",
			LangCode:       "id",
		},
		NoUpdates: false, // Pastikan ini false agar handler update dipanggil
		Logger:    nil,   // Biarkan default logger
	}

	// Hentikan client yang sudah ada jika ada
	if telegramClient != nil && telegramCancel != nil {
		log.Println("Stopping existing Telegram client")
		telegramCancel()
		telegramClient = nil
		telegramContext = nil
		telegramCancel = nil
		time.Sleep(time.Second) // Berikan waktu untuk cleanup
	}

	// Buat client Telegram baru dengan context yang baru
	log.Println("INFO: Membuat client Telegram baru")
	telegramContext, telegramCancel = context.WithCancel(context.Background())
	telegramClient = telegram.NewClient(telegramConfig.ApiID, telegramConfig.ApiHash, opts)
	log.Printf("INFO: Client Telegram dibuat dengan ApiID: %d", telegramConfig.ApiID)

	// Log informasi konfigurasi
	log.Printf("INFO: Konfigurasi Telegram - Active: %v, UserMode: %v, SessionFile: %s",
		telegramConfig.Active, telegramConfig.UserMode, telegramConfig.SessionFile)

	// Tampilkan pesan di layar tentang status client
	color.New(color.FgHiGreen, color.Bold).Println("=== TELEGRAM CLIENT INFO ===")
	color.New(color.FgHiYellow).Printf("- Nomor: %s\n", telegramConfig.PhoneNumber)
	color.New(color.FgHiYellow).Printf("- Status: %v\n", telegramConfig.StatusConnected)
	color.New(color.FgHiYellow).Printf("- Mode User: %v\n", telegramConfig.UserMode)
	color.New(color.FgHiYellow).Printf("- Active: %v\n", telegramConfig.Active)
	color.New(color.FgHiGreen, color.Bold).Println("===========================")

	// Variabel untuk melacak status client
	var authInitialized bool
	var clientErr error
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		log.Println("Starting Telegram client")

		clientErr = telegramClient.Run(telegramContext, func(ctx context.Context) error {
			authInitialized = true
			log.Println("INFO: Telegram client middleware berjalan")

			// Cek status autentikasi
			status, err := telegramClient.Auth().Status(ctx)
			if err != nil {
				log.Printf("Error getting auth status: %v", err)
				return nil // Jangan hentikan client jika gagal mendapatkan status
			}

			if !status.Authorized {
				log.Println("Telegram client not authorized, waiting for authentication")
				// Tunggu sampai konteks dibatalkan atau 12 jam
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(12 * time.Hour):
					return nil
				}
			}

			log.Println("Telegram client authorized successfully")
			log.Println("INFO: Client Telegram mulai menerima updates")

			// Dapatkan info user saat ini
			telegramConfig, err := config.GetTelegramConfig()
			if err != nil {
				telegramConfig = &config.TelegramConfig{
					UserMode:         false,
					Active:           false,
					StatusConnected:  false,
					SelectedChats:    []string{},
					SelectedGroups:   []string{},
					SelectedChannels: []string{},
				}
			}
			self, err := telegramClient.Self(ctx)
			if err == nil {
				telegramConfig.StatusConnected = true
				// Simpan status koneksi
				config.SaveTelegramConfig(telegramConfig)
				log.Printf("Logged in as: %s %s (@%s), ID: %d",
					self.FirstName, self.LastName, self.Username, self.ID)
			}

			// Periksa konfigurasi filter aktif
			log.Printf("INFO: Status chat aktif: %v", telegramConfig.Active)
			if len(telegramConfig.SelectedChats) > 0 {
				log.Printf("INFO: Chat yang dipilih: %v", telegramConfig.SelectedChats)
			}
			if len(telegramConfig.SelectedGroups) > 0 {
				log.Printf("INFO: Grup yang dipilih: %v", telegramConfig.SelectedGroups)
			}
			if len(telegramConfig.SelectedChannels) > 0 {
				log.Printf("INFO: Channel yang dipilih: %v", telegramConfig.SelectedChannels)
			}

			// Tetap jalankan sampai konteks dibatalkan
			<-ctx.Done()
			return ctx.Err()
		})

		if clientErr != nil && !errors.Is(clientErr, context.Canceled) {
			log.Printf("Telegram client error: %v", clientErr)
		} else {
			log.Println("Telegram client stopped normally")
		}
	}()

	// Tunggu maksimal 5 detik sampai autentikasi siap atau error terjadi
	ready := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ { // 50 * 100ms = 5 detik
			if authInitialized || clientErr != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		close(ready)
	}()

	<-ready

	if clientErr != nil {
		return fmt.Errorf("client initialization failed: %w", clientErr)
	}

	if !authInitialized {
		return fmt.Errorf("client initialization timeout")
	}

	return nil
}

// StopTelegramUserClient - Menghentikan client Telegram
func StopTelegramUserClient() {
	telegramClientLock.Lock()
	defer telegramClientLock.Unlock()

	if telegramCancel != nil {
		log.Println("Stopping Telegram client")
		telegramCancel()

		// Tunggu sedikit untuk memastikan client berhenti
		time.Sleep(time.Second)
	}

	// Reset semua variabel global
	telegramClient = nil
	telegramContext = nil
	telegramCancel = nil

	log.Println("Telegram client stopped and resources released")
}

// ShowTelegramLoginPage - Menampilkan halaman login Telegram
func ShowTelegramLoginPage(c *gin.Context) {
	// Ambil konfigurasi Telegram
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			ApiID:       0,
			ApiHash:     "",
			PhoneNumber: "",
			UserMode:    false,
		}
	}

	c.HTML(200, "telegram/login.html", gin.H{
		"Title":          "Login Telegram User - TeleTowa",
		"ApiID":          telegramConfig.ApiID,
		"ApiHash":        telegramConfig.ApiHash,
		"PhoneNumber":    telegramConfig.PhoneNumber,
		"UserModeActive": telegramConfig.UserMode,
	})
}

// SaveTelegramUserConfig - Menyimpan konfigurasi user Telegram
func SaveTelegramUserConfig(c *gin.Context) {
	// Ambil data dari form
	apiIDStr := c.PostForm("api_id")
	apiHash := c.PostForm("api_hash")
	phoneNumber := c.PostForm("phone_number")
	userMode := c.PostForm("user_mode") == "on"

	apiID := 0
	fmt.Sscanf(apiIDStr, "%d", &apiID)

	// Validasi input
	if apiID == 0 || apiHash == "" || phoneNumber == "" {
		c.HTML(400, "telegram/login.html", gin.H{
			"Title":       "Login Telegram User - TeleTowa",
			"Error":       "Semua field harus diisi dengan benar",
			"ApiID":       apiID,
			"ApiHash":     apiHash,
			"PhoneNumber": phoneNumber,
			"UserMode":    userMode,
		})
		return
	}

	// Ambil konfigurasi yang ada
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		telegramConfig = &config.TelegramConfig{
			ApiID:            0,
			ApiHash:          "",
			PhoneNumber:      "",
			UserMode:         false,
			SessionFile:      "./data/telegram.session",
			Active:           false,
			SelectedChats:    []string{},
			SelectedGroups:   []string{},
			SelectedChannels: []string{},
		}
	}

	// Update konfigurasi
	telegramConfig.ApiID = apiID
	telegramConfig.ApiHash = apiHash
	telegramConfig.PhoneNumber = phoneNumber
	telegramConfig.UserMode = userMode

	// Simpan konfigurasi
	if err := config.SaveTelegramConfig(telegramConfig); err != nil {
		c.HTML(500, "telegram/login.html", gin.H{
			"Title":       "Login Telegram User - TeleTowa",
			"Error":       "Gagal menyimpan konfigurasi: " + err.Error(),
			"ApiID":       apiID,
			"ApiHash":     apiHash,
			"PhoneNumber": phoneNumber,
			"UserMode":    userMode,
		})
		return
	}

	// Jika mode user diaktifkan, inisialisasi client
	if userMode {
		if err := InitTelegramUserClient(); err != nil {
			c.HTML(500, "telegram/login.html", gin.H{
				"Title":       "Login Telegram User - TeleTowa",
				"Error":       "Gagal menginisialisasi client: " + err.Error(),
				"ApiID":       apiID,
				"ApiHash":     apiHash,
				"PhoneNumber": phoneNumber,
				"UserMode":    userMode,
			})
			return
		}
	} else {
		// Jika mode user dinonaktifkan, hentikan client
		StopTelegramUserClient()
	}

	// Redirect ke halaman QR atau kode verifikasi
	c.Redirect(302, "/telegram/login/verify")
}

// ShowTelegramVerifyPage - Menampilkan halaman verifikasi login Telegram
func ShowTelegramVerifyPage(c *gin.Context) {
	c.HTML(200, "telegram/verify.html", gin.H{
		"Title": "Verifikasi Telegram - TeleTowa",
	})
}

// ProcessTelegramVerification - Memproses verifikasi kode login Telegram
func ProcessTelegramVerification(c *gin.Context) {
	code := c.PostForm("verification_code")

	if code == "" {
		c.HTML(400, "telegram/verify.html", gin.H{
			"Title": "Verifikasi Telegram - TeleTowa",
			"Error": "Kode verifikasi tidak boleh kosong",
		})
		return
	}

	// Ambil konfigurasi
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		c.HTML(500, "telegram/verify.html", gin.H{
			"Title": "Verifikasi Telegram - TeleTowa",
			"Error": "Gagal mengambil konfigurasi: " + err.Error(),
		})
		return
	}

	// Periksa apakah phone_code_hash tersedia
	if telegramConfig.PhoneCodeHash == "" {
		c.HTML(400, "telegram/verify.html", gin.H{
			"Title": "Verifikasi Telegram - TeleTowa",
			"Error": "Kode hash tidak tersedia, silakan kirim kode verifikasi terlebih dahulu",
		})
		return
	}

	// Pastikan client tersedia dan berjalan
	if telegramClient == nil {
		log.Println("Initializing Telegram client for verification")
		// Hentikan client yang mungkin masih ada tapi error
		if telegramCancel != nil {
			telegramCancel()
			telegramClient = nil
			telegramContext = nil
			telegramCancel = nil
			time.Sleep(time.Second) // Tunggu cleanup
		}

		if err := InitTelegramUserClient(); err != nil {
			log.Printf("Error initializing client: %v", err)
			c.HTML(500, "telegram/verify.html", gin.H{
				"Title": "Verifikasi Telegram - TeleTowa",
				"Error": "Gagal menginisialisasi client: " + err.Error(),
			})
			return
		}

		// Tunggu lebih lama untuk memastikan client siap
		time.Sleep(time.Second * 3)
	}

	// Pastikan kita memiliki konteks
	if telegramContext == nil {
		c.HTML(500, "telegram/verify.html", gin.H{
			"Title": "Verifikasi Telegram - TeleTowa",
			"Error": "Client context tidak tersedia",
		})
		return
	}

	// Gunakan context dengan timeout
	ctx, cancel := context.WithTimeout(telegramContext, 60*time.Second)
	defer cancel()

	// Coba langsung melakukan sign in dengan kode verifikasi
	var authErr error
	var authResult *tg.AuthAuthorization

	// Coba beberapa kali jika gagal
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("Mencoba verifikasi kode (percobaan %d/3)", attempt)

		// Langsung gunakan SignIn dari Client
		authResult, authErr = telegramClient.Auth().SignIn(
			ctx,
			telegramConfig.PhoneNumber,
			code,
			telegramConfig.PhoneCodeHash,
		)

		if authErr == nil {
			log.Println("Sign in berhasil")
			break
		}

		// Tangani kasus khusus
		if errors.Is(authErr, auth.ErrPasswordAuthNeeded) {
			// 2FA diperlukan
			c.HTML(400, "telegram/verify.html", gin.H{
				"Title": "Verifikasi Telegram - TeleTowa",
				"Error": "Akun memerlukan password 2FA. Saat ini fitur ini belum didukung.",
			})
			return
		}

		// Cek untuk SignUpRequired
		var signUpRequired *auth.SignUpRequired
		if errors.As(authErr, &signUpRequired) {
			// Perlu register
			log.Println("Akun tidak terdaftar, mencoba sign up...")

			// Buat SignUp request dengan struktur yang sesuai
			signup := auth.SignUp{
				PhoneNumber:   telegramConfig.PhoneNumber,
				PhoneCodeHash: telegramConfig.PhoneCodeHash,
				FirstName:     "TeleTowa",
				LastName:      "User",
			}

			authResult, authErr = telegramClient.Auth().SignUp(ctx, signup)

			if authErr == nil {
				log.Println("Sign up berhasil")
				break
			}
		} else if strings.Contains(authErr.Error(), "PHONE_CODE_EXPIRED") ||
			strings.Contains(authErr.Error(), "PHONE_CODE_INVALID") {
			// Kode kedaluwarsa atau salah
			c.HTML(400, "telegram/verify.html", gin.H{
				"Title": "Verifikasi Telegram - TeleTowa",
				"Error": "Kode verifikasi salah atau sudah kedaluwarsa. Silakan kirim kode baru.",
			})
			return
		}

		log.Printf("Gagal sign in/sign up (percobaan %d/3): %v", attempt, authErr)

		if attempt < 3 {
			time.Sleep(time.Second * 2)
		}
	}

	if authErr != nil {
		log.Printf("Final error autentikasi: %v", authErr)
		c.HTML(500, "telegram/verify.html", gin.H{
			"Title": "Verifikasi Telegram - TeleTowa",
			"Error": "Gagal melakukan autentikasi: " + authErr.Error(),
		})
		return
	}

	log.Println("Telegram authentication successful")
	if authResult != nil {
		log.Printf("Authentication successful with result: %+v", authResult)
	}

	// Update WebSocket clients dengan status baru
	WSManager.BroadcastMessage("telegramUserAuth", gin.H{
		"authenticated": true,
	})

	// Redirect ke dashboard Telegram
	c.Redirect(302, "/telegram")
}

// SendAuthCode - Mengirim kode autentikasi
func SendAuthCode(c *gin.Context) {
	// Ambil konfigurasi
	telegramConfig, err := config.GetTelegramConfig()
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "Gagal mengambil konfigurasi: " + err.Error()})
		return
	}

	// Pastikan client tersedia dan berjalan
	if telegramClient == nil {
		log.Println("Initializing Telegram client for sending code")
		// Hentikan client yang mungkin masih ada tapi error
		if telegramCancel != nil {
			telegramCancel()
			telegramClient = nil
			telegramContext = nil
			telegramCancel = nil
			time.Sleep(time.Second) // Tunggu cleanup
		}

		if err := InitTelegramUserClient(); err != nil {
			log.Printf("Error initializing client: %v", err)
			c.JSON(500, gin.H{"success": false, "error": "Gagal menginisialisasi client: " + err.Error()})
			return
		}

		// Tunggu lebih lama untuk memastikan client sudah berjalan
		time.Sleep(time.Second * 3)
	}

	// Pastikan kita menggunakan konteks dari client yang sedang berjalan
	if telegramContext == nil {
		c.JSON(500, gin.H{"success": false, "error": "Client context tidak tersedia"})
		return
	}

	// Buat context dengan timeout yang cukup
	ctx, cancel := context.WithTimeout(telegramContext, 30*time.Second)
	defer cancel()

	// Coba mengirim kode
	var sendErr error
	var sentCodeClass tg.AuthSentCodeClass

	// Coba beberapa kali
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("Mencoba mengirim kode verifikasi (percobaan %d/3)", attempt)

		// Buat opsi untuk SendCode
		sendCodeOptions := auth.SendCodeOptions{
			AllowFlashCall: false,
			CurrentNumber:  true,
			AllowAppHash:   true,
		}

		// Langsung menggunakan auth client dengan tipe yang benar
		sentCodeClass, sendErr = telegramClient.Auth().SendCode(
			ctx,
			telegramConfig.PhoneNumber,
			sendCodeOptions,
		)

		if sendErr == nil {
			// Berhasil mendapatkan kode, ekstrak phone_code_hash
			sc, ok := sentCodeClass.(*tg.AuthSentCode)
			if ok {
				telegramConfig.PhoneCodeHash = sc.PhoneCodeHash
				if err := config.SaveTelegramConfig(telegramConfig); err != nil {
					log.Printf("Gagal menyimpan PhoneCodeHash: %v", err)
				} else {
					log.Printf("Berhasil menyimpan PhoneCodeHash: %s", telegramConfig.PhoneCodeHash)
				}
				break
			} else {
				log.Printf("Type assertion gagal: %T", sentCodeClass)
				sendErr = fmt.Errorf("tipe sentCode tidak valid: %T", sentCodeClass)
			}
		} else {
			log.Printf("Gagal mengirim kode (percobaan %d/3): %v", attempt, sendErr)
		}

		if attempt < 3 {
			time.Sleep(time.Second * 2)
		}
	}

	if sendErr != nil {
		c.JSON(500, gin.H{"success": false, "error": "Gagal mengirim kode: " + sendErr.Error()})
		return
	}

	log.Printf("Kode verifikasi berhasil dikirim ke %s", telegramConfig.PhoneNumber)
	c.JSON(200, gin.H{
		"success":         true,
		"phone_code_hash": telegramConfig.PhoneCodeHash,
	})
}

// GetTelegramAuthStatus - Mendapatkan status autentikasi Telegram
func GetTelegramAuthStatus(c *gin.Context) {
	// Jika client belum diinisialisasi
	if telegramClient == nil {
		c.JSON(200, gin.H{"authenticated": false})
		return
	}

	// Buat context dengan timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cek status autentikasi
	status, err := telegramClient.Auth().Status(ctx)
	if err != nil {
		log.Printf("Error mendapatkan status autentikasi: %v", err)
		c.JSON(200, gin.H{"authenticated": false, "error": err.Error()})
		return
	}

	// Respons dengan status autentikasi dan info user jika diautentikasi
	response := gin.H{"authenticated": status.Authorized}

	if status.Authorized && status.User != nil {
		response["user"] = gin.H{
			"id":        status.User.ID,
			"firstName": status.User.FirstName,
			"lastName":  status.User.LastName,
			"username":  status.User.Username,
		}
	}

	c.JSON(200, response)
}

// LogoutTelegramUser - Logout dari akun Telegram
func LogoutTelegramUser(c *gin.Context) {
	if telegramClient == nil {
		c.JSON(400, gin.H{"success": false, "error": "Client belum diinisialisasi"})
		return
	}

	// Reset session - metode aman untuk logout
	telegramClientLock.Lock()
	defer telegramClientLock.Unlock()

	// Batalkan konteks yang ada dan buat ulang client
	if telegramCancel != nil {
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
		telegramCancel()
		telegramConfig.StatusConnected = false
		config.SaveTelegramConfig(telegramConfig)
	}

	// Hapus file session
	telegramConfig, err := config.GetTelegramConfig()
	if err == nil && telegramConfig.SessionFile != "" {
		if _, err := os.Stat(telegramConfig.SessionFile); err == nil {
			if err := os.Remove(telegramConfig.SessionFile); err != nil {
				c.JSON(500, gin.H{"success": false, "error": "Gagal menghapus file session: " + err.Error()})
				return
			}
		}
	}

	// Reset client
	telegramClient = nil

	// Update konfigurasi
	if err == nil {
		telegramConfig.UserMode = false
		config.SaveTelegramConfig(telegramConfig)
	}

	// Update WebSocket clients dengan status baru
	WSManager.BroadcastMessage("telegramUserAuth", gin.H{
		"authenticated": false,
	})

	c.JSON(200, gin.H{"success": true})
}

// GetTelegramChats - Mendapatkan daftar chat, grup, dan channel dari Telegram API
func GetTelegramChats(ctx context.Context) ([]gin.H, []gin.H, []gin.H, error) {
	// Pastikan client tersedia
	if telegramClient == nil {
		return nil, nil, nil, fmt.Errorf("telegram client belum diinisialisasi")
	}

	// Cek status autentikasi
	status, err := telegramClient.Auth().Status(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gagal mendapatkan status auth: %w", err)
	}

	if !status.Authorized {
		return nil, nil, nil, fmt.Errorf("client belum diautentikasi")
	}

	// Dapatkan daftar dialog (chat, grup, channel)
	dialogs, err := telegramClient.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100, // Batasi maksimum 100 dialog
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gagal mendapatkan dialogs: %w", err)
	}

	// Klasifikasikan data berdasarkan tipe
	var chats []gin.H
	var groups []gin.H
	var channels []gin.H

	// Ekstrak data dialaog
	var users []tg.UserClass
	var chatEntities []tg.ChatClass
	var dialogItems []tg.DialogClass

	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		users = d.Users
		chatEntities = d.Chats
		dialogItems = d.Dialogs
	case *tg.MessagesDialogsSlice:
		users = d.Users
		chatEntities = d.Chats
		dialogItems = d.Dialogs
	case *tg.MessagesDialogsNotModified:
		return nil, nil, nil, fmt.Errorf("dialogs not modified")
	default:
		return nil, nil, nil, fmt.Errorf("unknown dialog type: %T", dialogs)
	}

	// Proses setiap dialog
	for _, dialog := range dialogItems {
		d, ok := dialog.(*tg.Dialog)
		if !ok {
			continue
		}

		peer := d.Peer

		// Dapatkan entity
		var entityID int64
		var entityType string
		var entityTitle string

		switch p := peer.(type) {
		case *tg.PeerUser:
			entityID = p.UserID
			entityType = "private"

			// Cari info user
			for _, userClass := range users {
				user, ok := userClass.(*tg.User)
				if !ok {
					continue
				}
				if user.ID == entityID {
					firstName := user.FirstName
					lastName := user.LastName
					if lastName != "" {
						entityTitle = firstName + " " + lastName
					} else {
						entityTitle = firstName
					}
					break
				}
			}

		case *tg.PeerChat:
			entityID = p.ChatID
			entityType = "group"

			// Cari info chat
			for _, chatClass := range chatEntities {
				if c, ok := chatClass.(*tg.Chat); ok && c.ID == entityID {
					entityTitle = c.Title
					break
				}
			}

		case *tg.PeerChannel:
			entityID = p.ChannelID
			entityType = "channel"

			// Cari info channel
			for _, chatClass := range chatEntities {
				if c, ok := chatClass.(*tg.Channel); ok && c.ID == entityID {
					entityTitle = c.Title
					if c.Broadcast {
						entityType = "channel"
					} else {
						entityType = "group" // Supergroup
					}
					break
				}
			}
		default:
			continue
		}

		// Skip jika tidak berhasil mendapatkan judul
		if entityTitle == "" {
			continue
		}

		// Format ID dalam bentuk string
		entityIDStr := fmt.Sprintf("%d", entityID)

		// Buat objek data
		chatData := gin.H{
			"id":    entityIDStr,
			"title": entityTitle,
			"type":  entityType,
		}

		// Tambahkan ke kategori yang sesuai
		switch entityType {
		case "private":
			chats = append(chats, chatData)
		case "group":
			groups = append(groups, chatData)
		case "channel":
			channels = append(channels, chatData)
		}
	}

	return chats, groups, channels, nil
}

// downloadTelegramMedia downloads media files from Telegram using official downloader
func downloadTelegramMedia(client *telegram.Client, location tg.InputFileLocationClass, outputPath string) error {
	// Validate inputs
	if client == nil {
		return fmt.Errorf("telegram client is nil")
	}
	if location == nil {
		return fmt.Errorf("location is nil")
	}

	// Create downloader instance
	d := downloader.NewDownloader()

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use the downloader to download the file
	_, err = d.Download(client.API(), location).Stream(ctx, outFile)
	if err != nil {
		// Remove partial file on error
		os.Remove(outputPath)
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

// downloadTelegramDocument downloads document files from Telegram using official downloader
func downloadTelegramDocument(client *telegram.Client, location tg.InputFileLocationClass, outputPath string) error {
	// Create downloader instance
	d := downloader.NewDownloader()

	// Generate unique filename if file already exists
	finalPath := generateUniqueFilename(outputPath)

	// Create output file
	outFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Use the downloader to download the file
	_, err = d.Download(client.API(), location).Stream(context.Background(), outFile)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

// generateUniqueFilename generates a unique filename if the file already exists
func generateUniqueFilename(originalPath string) string {
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return originalPath
	}

	dir := filepath.Dir(originalPath)
	filename := filepath.Base(originalPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	counter := 1
	for {
		newFilename := fmt.Sprintf("%s(%d)%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(dir, newFilename)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}
