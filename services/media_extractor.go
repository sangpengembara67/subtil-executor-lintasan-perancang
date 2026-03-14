package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"go.mau.fi/whatsmeow/types/events"
)

// MediaExtractor handles extraction and analysis of messages from WhatsApp and Telegram
type MediaExtractor struct {
	db *sql.DB
}

// NewMediaExtractor creates a new MediaExtractor instance
func NewMediaExtractor(db *sql.DB) *MediaExtractor {
	return &MediaExtractor{
		db: db,
	}
}

// MessageData represents extracted message data
type MessageData struct {
	ID            string    `json:"id"`
	Platform      string    `json:"platform"`
	ChatID        string    `json:"chat_id"`
	ChatName      string    `json:"chat_name"`
	SenderID      string    `json:"sender_id"`
	SenderName    string    `json:"sender_name"`
	MessageType   string    `json:"message_type"`
	Content       string    `json:"content"`
	ExtractedText string    `json:"extracted_text,omitempty"`
	MediaPath     string    `json:"media_path,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	Metadata      string    `json:"metadata,omitempty"`
}

// ExtractWhatsAppMessage extracts and stores WhatsApp message data
func (me *MediaExtractor) ExtractWhatsAppMessage(evt *events.Message) error {
	if evt == nil || evt.Message == nil {
		return fmt.Errorf("invalid WhatsApp message event")
	}

	msg := evt.Message
	msgData := &MessageData{
		ID:        evt.Info.ID,
		Platform:  "whatsapp",
		ChatID:    evt.Info.Chat.String(),
		SenderID:  evt.Info.Sender.String(),
		Timestamp: evt.Info.Timestamp,
	}

	// Get chat name
	if evt.Info.Chat.Server == "g.us" {
		msgData.ChatName = "Group Chat"
		msgData.MessageType = "group"
	} else {
		msgData.ChatName = "Private Chat"
		msgData.MessageType = "private"
	}

	// Get sender name
	if evt.Info.PushName != "" {
		msgData.SenderName = evt.Info.PushName
	} else {
		msgData.SenderName = "Unknown"
	}

	// Extract content based on message type
	switch {
	case msg.GetConversation() != "":
		msgData.Content = msg.GetConversation()
		msgData.MessageType = "text"

	case msg.GetImageMessage() != nil:
		imgMsg := msg.GetImageMessage()
		msgData.Content = imgMsg.GetCaption()
		msgData.MessageType = "image"
		if imgMsg.URL != nil {
			msgData.MediaPath = *imgMsg.URL
		}

	case msg.GetVideoMessage() != nil:
		vidMsg := msg.GetVideoMessage()
		msgData.Content = vidMsg.GetCaption()
		msgData.MessageType = "video"
		if vidMsg.URL != nil {
			msgData.MediaPath = *vidMsg.URL
		}

	case msg.GetAudioMessage() != nil:
		audioMsg := msg.GetAudioMessage()
		msgData.Content = "[Audio Message]"
		msgData.MessageType = "audio"
		if audioMsg.URL != nil {
			msgData.MediaPath = *audioMsg.URL
		}

	case msg.GetDocumentMessage() != nil:
		docMsg := msg.GetDocumentMessage()
		msgData.Content = fmt.Sprintf("[Document: %s]", docMsg.GetTitle())
		msgData.MessageType = "document"
		if docMsg.URL != nil {
			msgData.MediaPath = *docMsg.URL
		}

	case msg.GetStickerMessage() != nil:
		msgData.Content = "[Sticker]"
		msgData.MessageType = "sticker"

	case msg.GetContactMessage() != nil:
		contactMsg := msg.GetContactMessage()
		msgData.Content = fmt.Sprintf("[Contact: %s]", contactMsg.GetDisplayName())
		msgData.MessageType = "contact"

	case msg.GetLocationMessage() != nil:
		locMsg := msg.GetLocationMessage()
		msgData.Content = fmt.Sprintf("[Location: %f, %f]", locMsg.GetDegreesLatitude(), locMsg.GetDegreesLongitude())
		msgData.MessageType = "location"

	default:
		msgData.Content = "[Unsupported Message Type]"
		msgData.MessageType = "unknown"
	}

	// Store in database
	return me.storeMessage(msgData)
}

// ExtractTelegramMessage extracts and stores Telegram message data
func (me *MediaExtractor) ExtractTelegramMessage(msg *tg.Message, entities tg.Entities) error {
	if msg == nil {
		return fmt.Errorf("invalid Telegram message")
	}

	msgData := &MessageData{
		ID:        fmt.Sprintf("%d", msg.ID),
		Platform:  "telegram",
		Timestamp: time.Unix(int64(msg.Date), 0),
	}

	// Get chat information
	if msg.PeerID != nil {
		switch peer := msg.PeerID.(type) {
		case *tg.PeerUser:
			msgData.ChatID = fmt.Sprintf("user_%d", peer.UserID)
			msgData.MessageType = "private"
			if user, ok := entities.Users[peer.UserID]; ok {
				msgData.ChatName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			} else {
				msgData.ChatName = "Private Chat"
			}

		case *tg.PeerChat:
			msgData.ChatID = fmt.Sprintf("chat_%d", peer.ChatID)
			msgData.MessageType = "group"
			if chat, ok := entities.Chats[peer.ChatID]; ok {
				msgData.ChatName = chat.Title
			} else {
				msgData.ChatName = "Group Chat"
			}

		case *tg.PeerChannel:
			msgData.ChatID = fmt.Sprintf("channel_%d", peer.ChannelID)
			if channel, ok := entities.Channels[peer.ChannelID]; ok {
				msgData.ChatName = channel.Title
				if channel.Broadcast {
					msgData.MessageType = "channel"
				} else {
					msgData.MessageType = "supergroup"
				}
			} else {
				msgData.ChatName = "Channel"
				msgData.MessageType = "channel"
			}
		}
	}

	// Get sender information
	if msg.FromID != nil {
		switch from := msg.FromID.(type) {
		case *tg.PeerUser:
			msgData.SenderID = fmt.Sprintf("user_%d", from.UserID)
			if user, ok := entities.Users[from.UserID]; ok {
				msgData.SenderName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			} else {
				msgData.SenderName = "Unknown User"
			}

		case *tg.PeerChannel:
			msgData.SenderID = fmt.Sprintf("channel_%d", from.ChannelID)
			if channel, ok := entities.Channels[from.ChannelID]; ok {
				msgData.SenderName = channel.Title
			} else {
				msgData.SenderName = "Unknown Channel"
			}
		}
	} else {
		msgData.SenderName = "System"
		msgData.SenderID = "system"
	}

	// Extract content based on message type
	if msg.Message != "" {
		msgData.Content = msg.Message
		if msgData.MessageType == "" {
			msgData.MessageType = "text"
		}
	}

	// Handle media messages
	if msg.Media != nil {
		switch media := msg.Media.(type) {
		case *tg.MessageMediaPhoto:
			msgData.MessageType = "photo"
			if msgData.Content == "" {
				msgData.Content = "[Photo]"
			}

		case *tg.MessageMediaDocument:
			if doc, ok := media.Document.(*tg.Document); ok {
				// Check if it's a video, audio, or document based on MIME type
				if strings.HasPrefix(doc.MimeType, "video/") {
					msgData.MessageType = "video"
					if msgData.Content == "" {
						msgData.Content = "[Video]"
					}
				} else if strings.HasPrefix(doc.MimeType, "audio/") {
					msgData.MessageType = "audio"
					if msgData.Content == "" {
						msgData.Content = "[Audio]"
					}
				} else {
					msgData.MessageType = "document"
					fileName := "Unknown File"
					for _, attr := range doc.Attributes {
						if docAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
							fileName = docAttr.FileName
							break
						}
					}
					if msgData.Content == "" {
						msgData.Content = fmt.Sprintf("[Document: %s]", fileName)
					}
				}
			}

		case *tg.MessageMediaContact:
			msgData.MessageType = "contact"
			contactName := media.FirstName + " " + media.LastName
			if msgData.Content == "" {
				msgData.Content = fmt.Sprintf("[Contact: %s]", contactName)
			}

		case *tg.MessageMediaGeo:
			msgData.MessageType = "location"
			if geo, ok := media.Geo.(*tg.GeoPoint); ok {
				if msgData.Content == "" {
					msgData.Content = fmt.Sprintf("[Location: %f, %f]", geo.Lat, geo.Long)
				}
			}

		default:
			if msgData.MessageType == "" {
				msgData.MessageType = "media"
			}
			if msgData.Content == "" {
				msgData.Content = "[Media Message]"
			}
		}
	}

	// Store metadata
	metadata := map[string]interface{}{
		"message_id": msg.ID,
		"date":       msg.Date,
		"out":        msg.Out,
		"mentioned":  msg.Mentioned,
		"silent":     msg.Silent,
	}

	if metadataJSON, err := json.Marshal(metadata); err == nil {
		msgData.Metadata = string(metadataJSON)
	}

	// Store in database
	return me.storeMessage(msgData)
}

// storeMessage stores message data in the database
func (me *MediaExtractor) storeMessage(msgData *MessageData) error {
	query := `
		INSERT INTO extracted_messages (
			message_id, platform, chat_id, chat_name, sender_id, sender_name,
			message_type, content, media_path, timestamp, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := me.db.Exec(
		query,
		msgData.ID,
		msgData.Platform,
		msgData.ChatID,
		msgData.ChatName,
		msgData.SenderID,
		msgData.SenderName,
		msgData.MessageType,
		msgData.Content,
		msgData.MediaPath,
		msgData.Timestamp,
		msgData.Metadata,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	log.Printf("Stored %s message from %s in %s", msgData.Platform, msgData.SenderName, msgData.ChatName)
	return nil
}

// GetExtractedMessages retrieves extracted messages with optional filters
func (me *MediaExtractor) GetExtractedMessages(platform, chatID string, limit int) ([]MessageData, error) {
	query := `
		SELECT message_id, platform, chat_id, chat_name, sender_id, sender_name,
		       message_type, content, media_path, timestamp, metadata, created_at
		FROM extracted_messages
		WHERE 1=1
	`
	args := []interface{}{}

	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}

	if chatID != "" {
		query += " AND chat_id = ?"
		args = append(args, chatID)
	}

	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := me.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []MessageData
	for rows.Next() {
		var msg MessageData
		var createdAt time.Time

		err := rows.Scan(
			&msg.ID,
			&msg.Platform,
			&msg.ChatID,
			&msg.ChatName,
			&msg.SenderID,
			&msg.SenderName,
			&msg.MessageType,
			&msg.Content,
			&msg.MediaPath,
			&msg.Timestamp,
			&msg.Metadata,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// GetMessageStats returns statistics about extracted messages
func (me *MediaExtractor) GetMessageStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total messages
	var totalMessages int
	err := me.db.QueryRow("SELECT COUNT(*) FROM extracted_messages").Scan(&totalMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to get total messages: %w", err)
	}
	stats["total_messages"] = totalMessages

	// Messages by platform
	platformQuery := `
		SELECT platform, COUNT(*) as count
		FROM extracted_messages
		GROUP BY platform
	`
	rows, err := me.db.Query(platformQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform stats: %w", err)
	}
	defer rows.Close()

	platformStats := make(map[string]int)
	for rows.Next() {
		var platform string
		var count int
		if err := rows.Scan(&platform, &count); err != nil {
			return nil, fmt.Errorf("failed to scan platform stats: %w", err)
		}
		platformStats[platform] = count
	}
	stats["by_platform"] = platformStats

	// Messages by type
	typeQuery := `
		SELECT message_type, COUNT(*) as count
		FROM extracted_messages
		GROUP BY message_type
	`
	rows, err = me.db.Query(typeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get type stats: %w", err)
	}
	defer rows.Close()

	typeStats := make(map[string]int)
	for rows.Next() {
		var msgType string
		var count int
		if err := rows.Scan(&msgType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan type stats: %w", err)
		}
		typeStats[msgType] = count
	}
	stats["by_type"] = typeStats

	// Recent activity (last 24 hours)
	var recentMessages int
	recentQuery := `
		SELECT COUNT(*) FROM extracted_messages
		WHERE created_at >= datetime('now', '-24 hours')
	`
	err = me.db.QueryRow(recentQuery).Scan(&recentMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}
	stats["recent_24h"] = recentMessages

	return stats, nil
}

// AnalyzeContent performs basic content analysis on extracted messages
func (me *MediaExtractor) AnalyzeContent(chatID string, days int) (map[string]interface{}, error) {
	analysis := make(map[string]interface{})

	// Build query with date filter
	query := `
		SELECT content, message_type, timestamp
		FROM extracted_messages
		WHERE content != '' AND content NOT LIKE '[%]'
	`
	args := []interface{}{}

	if chatID != "" {
		query += " AND chat_id = ?"
		args = append(args, chatID)
	}

	if days > 0 {
		query += " AND timestamp >= datetime('now', '-" + fmt.Sprintf("%d", days) + " days')"
	}

	query += " ORDER BY timestamp DESC"

	rows, err := me.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query content: %w", err)
	}
	defer rows.Close()

	var contents []string
	msgTypeCount := make(map[string]int)
	totalMessages := 0

	for rows.Next() {
		var content, msgType string
		var timestamp time.Time

		if err := rows.Scan(&content, &msgType, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan content: %w", err)
		}

		contents = append(contents, content)
		msgTypeCount[msgType]++
		totalMessages++
	}

	analysis["total_analyzed"] = totalMessages
	analysis["message_types"] = msgTypeCount

	// Basic text analysis
	if len(contents) > 0 {
		wordCount := 0
		wordFreq := make(map[string]int)

		for _, content := range contents {
			words := strings.Fields(strings.ToLower(content))
			wordCount += len(words)

			// Count word frequency (simple implementation)
			for _, word := range words {
				// Skip very short words
				if len(word) > 2 {
					wordFreq[word]++
				}
			}
		}

		analysis["total_words"] = wordCount
		analysis["avg_words_per_message"] = float64(wordCount) / float64(len(contents))

		// Get top 10 most frequent words
		type WordFrequency struct {
			Word  string `json:"word"`
			Count int    `json:"count"`
		}

		var topWords []WordFrequency
		for word, count := range wordFreq {
			if count > 1 { // Only include words that appear more than once
				topWords = append(topWords, WordFrequency{Word: word, Count: count})
			}
		}

		// Simple sorting (top 10)
		for i := 0; i < len(topWords)-1; i++ {
			for j := i + 1; j < len(topWords); j++ {
				if topWords[j].Count > topWords[i].Count {
					topWords[i], topWords[j] = topWords[j], topWords[i]
				}
			}
		}

		if len(topWords) > 10 {
			topWords = topWords[:10]
		}

		analysis["top_words"] = topWords
	}

	return analysis, nil
}
