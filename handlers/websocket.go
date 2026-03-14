package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shiestapoi/teletowa/config"
)

// WebSocketManager - Struktur untuk mengelola koneksi WebSocket
type WebSocketManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.Mutex
}

// Upgrader untuk upgrade HTTP ke WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Izinkan semua origin (dalam produksi sebaiknya dibatasi)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Buat instance global dari WebSocketManager
var WSManager = WebSocketManager{
	clients:    make(map[*websocket.Conn]bool),
	broadcast:  make(chan []byte),
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
}

// Start - Memulai goroutine untuk memproses pesan
func (manager *WebSocketManager) Start() {
	for {
		select {
		case conn := <-manager.register:
			// Daftarkan koneksi baru
			manager.mutex.Lock()
			manager.clients[conn] = true
			manager.mutex.Unlock()
			log.Printf("Client connected: %v", conn.RemoteAddr())

		case conn := <-manager.unregister:
			// Hapus koneksi yang terputus
			manager.mutex.Lock()
			if _, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				conn.Close()
			}
			manager.mutex.Unlock()
			log.Printf("Client disconnected: %v", conn.RemoteAddr())

		case message := <-manager.broadcast:
			// Broadcast pesan ke semua client
			manager.mutex.Lock()
			for client := range manager.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Printf("Error broadcasting message: %v", err)
					client.Close()
					delete(manager.clients, client)
				}
			}
			manager.mutex.Unlock()
		}
	}
}

// BroadcastMessage - Broadcast pesan ke semua client yang terhubung
func (manager *WebSocketManager) BroadcastMessage(messageType string, data interface{}) {
	// Buat struktur pesan
	message := struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}{
		Type: messageType,
		Data: data,
	}

	// Marshal ke JSON
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	// Kirim ke channel broadcast
	manager.broadcast <- jsonMessage
}

// HandleWebSocket - Handler untuk permintaan WebSocket
func HandleWebSocket(c *gin.Context) {
	// Upgrade koneksi HTTP ke WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	// Daftarkan koneksi baru
	WSManager.register <- conn

	// Goroutine untuk menangani pesan masuk
	go handleMessages(conn)
}

// handleMessages - Menangani pesan dari client
func handleMessages(conn *websocket.Conn) {
	defer func() {
		WSManager.unregister <- conn
	}()

	for {
		// Baca pesan dari client
		_, message, err := conn.ReadMessage()
		if err != nil {
			// Connection closed or error
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		// Process message
		log.Printf("Received message: %s", message)

		// Implementasi yang lebih kompleks dapat memproses pesan berdasarkan tipe
		var data map[string]interface{}
		if err := json.Unmarshal(message, &data); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Handle pesan berdasarkan jenisnya
		handleMessageByType(data)
	}
}

// handleMessageByType - Memproses pesan berdasarkan jenisnya
func handleMessageByType(data map[string]interface{}) {
	msgType, ok := data["type"].(string)
	if !ok {
		log.Println("Message type not found or not a string")
		return
	}

	switch msgType {
	case "ping":
		// Handle ping (contoh)
		WSManager.BroadcastMessage("pong", nil)
	case "getStatus":
		// Ambil status Telegram dan WhatsApp dari config
		telegramConfig, err := config.GetTelegramConfig()
		if err != nil {
			log.Printf("Error getting Telegram config: %v", err)
			telegramConfig = &config.TelegramConfig{
				StatusConnected: false,
				Active:          false,
			}
		}

		whatsappConfig, err := config.GetWhatsAppConfig()
		if err != nil {
			log.Printf("Error getting WhatsApp config: %v", err)
			whatsappConfig = &config.WhatsAppConfig{
				LoggedIn: false,
				Active:   false,
			}
		}

		// Buat response status
		status := map[string]interface{}{
			"telegramConnected": telegramConfig.StatusConnected,
			"telegramActive":    telegramConfig.Active,
			"whatsappConnected": whatsappConfig.LoggedIn,
			"whatsappActive":    whatsappConfig.Active,
		}
		WSManager.BroadcastMessage("status", status)
	default:
		log.Printf("Unknown message type: %s", msgType)
	}
}
