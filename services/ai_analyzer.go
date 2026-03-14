package services

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// AIAnalyzer handles AI-powered analysis of extracted messages
type AIAnalyzer struct {
	db             *sql.DB
	mediaExtractor *MediaExtractor
	stockAnalyzer  *StockAnalyzer
	inferenceURL   string
	apiKey         string
	client         *http.Client
}

// NewAIAnalyzer creates a new AIAnalyzer instance
func NewAIAnalyzer(db *sql.DB, mediaExtractor *MediaExtractor) *AIAnalyzer {
	return &AIAnalyzer{
		db:             db,
		mediaExtractor: mediaExtractor,
		stockAnalyzer:  NewStockAnalyzer(db),
		inferenceURL:   os.Getenv("AI_INFERENCE_URL"),
		apiKey:         os.Getenv("AI_API_KEY"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetStockAnalyzer returns the stock analyzer instance
func (ai *AIAnalyzer) GetStockAnalyzer() *StockAnalyzer {
	return ai.stockAnalyzer
}

// AnalysisResult represents the result of AI analysis
type AnalysisResult struct {
	ID              int       `json:"id"`
	ChatID          string    `json:"chat_id"`
	Platform        string    `json:"platform"`
	AnalysisType    string    `json:"analysis_type"`
	Sentiment       string    `json:"sentiment"`
	Keywords        string    `json:"keywords"`       // JSON array
	StockMentions   string    `json:"stock_mentions"` // JSON array
	Summary         string    `json:"summary"`
	Insights        string    `json:"insights"`
	ConfidenceScore float64   `json:"confidence_score"`
	AnalyzedAt      time.Time `json:"analyzed_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// InferenceRequest represents a request to the AI inference endpoint
type InferenceRequest struct {
	Model       string             `json:"model"`
	Messages    []InferenceMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

// InferenceMessage represents a message in the inference request
type InferenceMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// InferenceResponse represents the response from AI inference
type InferenceResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// AnalyzeChatMessages analyzes messages from a specific chat
func (ai *AIAnalyzer) AnalyzeChatMessages(chatID string, platform string, days int) (*AnalysisResult, error) {
	// Get recent messages for analysis
	messages, err := ai.mediaExtractor.GetExtractedMessages(platform, chatID, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages found for analysis")
	}

	// Filter messages by date if specified
	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days)
		var filteredMessages []MessageData
		for _, msg := range messages {
			if msg.Timestamp.After(cutoff) {
				filteredMessages = append(filteredMessages, msg)
			}
		}
		messages = filteredMessages
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no recent messages found for analysis")
	}

	// Prepare content for analysis
	content := ai.prepareContentForAnalysis(messages)

	// Detect and store stock mentions
	var allStockMentions []StockMention
	for _, msg := range messages {
		if msg.Content != "" {
			mentions := ai.stockAnalyzer.DetectStockMentions(msg.Content, chatID, platform)
			allStockMentions = append(allStockMentions, mentions...)
		}
		if msg.ExtractedText != "" {
			mentions := ai.stockAnalyzer.DetectStockMentions(msg.ExtractedText, chatID, platform)
			allStockMentions = append(allStockMentions, mentions...)
		}
	}

	// Store stock mentions
	if len(allStockMentions) > 0 {
		if err := ai.stockAnalyzer.StoreMentions(allStockMentions); err != nil {
			log.Printf("Warning: Failed to store stock mentions: %v", err)
		}
	}

	// Perform AI analysis
	analysisResult, err := ai.performAIAnalysis(content, chatID, platform)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Add detected stock mentions to analysis result
	if len(allStockMentions) > 0 {
		stockMentionsJSON, _ := json.Marshal(allStockMentions)
		analysisResult.StockMentions = string(stockMentionsJSON)
	}

	// Store analysis result
	if err := ai.storeAnalysisResult(analysisResult); err != nil {
		return nil, fmt.Errorf("failed to store analysis: %w", err)
	}

	return analysisResult, nil
}

// prepareContentForAnalysis prepares message content for AI analysis
func (ai *AIAnalyzer) prepareContentForAnalysis(messages []MessageData) string {
	var content strings.Builder
	content.WriteString("Chat Messages for Analysis:\n\n")

	for i, msg := range messages {
		if i >= 30 { // Limit to 30 messages to avoid token limits
			break
		}

		// Skip media-only messages without text content
		if strings.HasPrefix(msg.Content, "[") && strings.HasSuffix(msg.Content, "]") {
			continue
		}

		timeStr := msg.Timestamp.Format("15:04")
		content.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.SenderName, msg.Content))
	}

	return content.String()
}

// performAIAnalysis performs the actual AI analysis using the inference endpoint
func (ai *AIAnalyzer) performAIAnalysis(content, chatID, platform string) (*AnalysisResult, error) {
	if ai.inferenceURL == "" {
		return nil, fmt.Errorf("AI inference URL not configured")
	}

	// Create analysis prompt
	prompt := ai.createAnalysisPrompt(content)

	// Prepare inference request
	request := InferenceRequest{
		Model: "gpt-3.5-turbo", // Default model, can be configured
		Messages: []InferenceMessage{
			{
				Role:    "system",
				Content: "You are an expert chat analyzer. Analyze the provided chat messages and return a structured JSON response with sentiment analysis, key insights, stock mentions, and summary.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
	}

	// Send request to inference endpoint
	response, err := ai.sendInferenceRequest(request)
	if err != nil {
		return nil, fmt.Errorf("inference request failed: %w", err)
	}

	// Parse AI response
	analysisResult, err := ai.parseAIResponse(response, chatID, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return analysisResult, nil
}

// createAnalysisPrompt creates a structured prompt for AI analysis
func (ai *AIAnalyzer) createAnalysisPrompt(content string) string {
	prompt := `Analyze the following chat messages and provide a comprehensive analysis in JSON format.

Chat Content:
` + content + `

Please provide your analysis in the following JSON structure:
{
  "sentiment": "positive/negative/neutral",
  "keywords": ["keyword1", "keyword2", "keyword3"],
  "stock_mentions": ["BBCA", "TLKM", "GOTO"],
  "summary": "Brief summary of the main topics discussed",
  "insights": "Key insights and patterns observed",
  "confidence_score": 0.85
}

Focus on:
1. Overall sentiment of the conversation
2. Key topics and keywords mentioned
3. Any stock codes or financial instruments mentioned (Indonesian stock codes like BBCA, TLKM, GOTO, etc.)
4. Main themes and insights
5. Your confidence in the analysis (0.0 to 1.0)

Return only the JSON response, no additional text.`

	return prompt
}

// sendInferenceRequest sends a request to the AI inference endpoint
func (ai *AIAnalyzer) sendInferenceRequest(request InferenceRequest) (*InferenceResponse, error) {
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", ai.inferenceURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if ai.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+ai.apiKey)
	}

	// Send request
	resp, err := ai.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inference API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var inferenceResp InferenceResponse
	if err := json.Unmarshal(respBody, &inferenceResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &inferenceResp, nil
}

// parseAIResponse parses the AI response and creates an AnalysisResult
func (ai *AIAnalyzer) parseAIResponse(response *InferenceResponse, chatID, platform string) (*AnalysisResult, error) {
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in AI response")
	}

	content := response.Choices[0].Message.Content

	// Try to extract JSON from the response
	var aiResult struct {
		Sentiment       string   `json:"sentiment"`
		Keywords        []string `json:"keywords"`
		StockMentions   []string `json:"stock_mentions"`
		Summary         string   `json:"summary"`
		Insights        string   `json:"insights"`
		ConfidenceScore float64  `json:"confidence_score"`
	}

	// Clean the content to extract JSON
	jsonContent := ai.extractJSON(content)
	if err := json.Unmarshal([]byte(jsonContent), &aiResult); err != nil {
		return nil, fmt.Errorf("failed to parse AI JSON response: %w", err)
	}

	// Convert arrays to JSON strings
	keywordsJSON, _ := json.Marshal(aiResult.Keywords)
	stockMentionsJSON, _ := json.Marshal(aiResult.StockMentions)

	// Create analysis result
	analysisResult := &AnalysisResult{
		ChatID:          chatID,
		Platform:        platform,
		AnalysisType:    "comprehensive",
		Sentiment:       aiResult.Sentiment,
		Keywords:        string(keywordsJSON),
		StockMentions:   string(stockMentionsJSON),
		Summary:         aiResult.Summary,
		Insights:        aiResult.Insights,
		ConfidenceScore: aiResult.ConfidenceScore,
		AnalyzedAt:      time.Now(),
		CreatedAt:       time.Now(),
	}

	return analysisResult, nil
}

// extractJSON extracts JSON content from AI response
func (ai *AIAnalyzer) extractJSON(content string) string {
	// Try to find JSON block
	jsonRegex := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonRegex.FindAllString(content, -1)

	if len(matches) > 0 {
		return matches[0]
	}

	// If no JSON block found, return the content as is
	return content
}

// storeAnalysisResult stores the analysis result in the database
func (ai *AIAnalyzer) storeAnalysisResult(result *AnalysisResult) error {
	query := `
		INSERT INTO analysis_results (
			chat_id, platform, analysis_type, sentiment, keywords,
			stock_mentions, summary, insights, confidence_score,
			analyzed_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	resultDB, err := ai.db.Exec(
		query,
		result.ChatID,
		result.Platform,
		result.AnalysisType,
		result.Sentiment,
		result.Keywords,
		result.StockMentions,
		result.Summary,
		result.Insights,
		result.ConfidenceScore,
		result.AnalyzedAt,
		result.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to store analysis result: %w", err)
	}

	id, err := resultDB.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get analysis result ID: %w", err)
	}

	result.ID = int(id)
	log.Printf("Stored AI analysis result for chat %s (ID: %d)", result.ChatID, result.ID)
	return nil
}

// GetAnalysisResults retrieves analysis results with optional filters
func (ai *AIAnalyzer) GetAnalysisResults(chatID, platform string, limit int) ([]AnalysisResult, error) {
	query := `
		SELECT id, chat_id, platform, analysis_type, sentiment, keywords,
		       stock_mentions, summary, insights, confidence_score,
		       analyzed_at, created_at
		FROM analysis_results
		WHERE 1=1
	`
	args := []interface{}{}

	if chatID != "" {
		query += " AND chat_id = ?"
		args = append(args, chatID)
	}

	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}

	query += " ORDER BY analyzed_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := ai.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query analysis results: %w", err)
	}
	defer rows.Close()

	var results []AnalysisResult
	for rows.Next() {
		var result AnalysisResult

		err := rows.Scan(
			&result.ID,
			&result.ChatID,
			&result.Platform,
			&result.AnalysisType,
			&result.Sentiment,
			&result.Keywords,
			&result.StockMentions,
			&result.Summary,
			&result.Insights,
			&result.ConfidenceScore,
			&result.AnalyzedAt,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan analysis result: %w", err)
		}

		results = append(results, result)
	}

	return results, nil
}

// GetAnalysisStats returns statistics about analysis results
func (ai *AIAnalyzer) GetAnalysisStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total analyses
	var totalAnalyses int
	err := ai.db.QueryRow("SELECT COUNT(*) FROM analysis_results").Scan(&totalAnalyses)
	if err != nil {
		return nil, fmt.Errorf("failed to get total analyses: %w", err)
	}
	stats["total_analyses"] = totalAnalyses

	// Analyses by sentiment
	sentimentQuery := `
		SELECT sentiment, COUNT(*) as count
		FROM analysis_results
		GROUP BY sentiment
	`
	rows, err := ai.db.Query(sentimentQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get sentiment stats: %w", err)
	}
	defer rows.Close()

	sentimentStats := make(map[string]int)
	for rows.Next() {
		var sentiment string
		var count int
		if err := rows.Scan(&sentiment, &count); err != nil {
			return nil, fmt.Errorf("failed to scan sentiment stats: %w", err)
		}
		sentimentStats[sentiment] = count
	}
	stats["by_sentiment"] = sentimentStats

	// Recent analyses (last 24 hours)
	var recentAnalyses int
	recentQuery := `
		SELECT COUNT(*) FROM analysis_results
		WHERE analyzed_at >= datetime('now', '-24 hours')
	`
	err = ai.db.QueryRow(recentQuery).Scan(&recentAnalyses)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent analyses: %w", err)
	}
	stats["recent_24h"] = recentAnalyses

	// Average confidence score
	var avgConfidence float64
	confidenceQuery := "SELECT AVG(confidence_score) FROM analysis_results"
	err = ai.db.QueryRow(confidenceQuery).Scan(&avgConfidence)
	if err != nil {
		return nil, fmt.Errorf("failed to get average confidence: %w", err)
	}
	stats["avg_confidence"] = avgConfidence

	return stats, nil
}

// AnalyzeStockMentions performs specialized analysis for stock mentions
func (ai *AIAnalyzer) AnalyzeStockMentions(chatID string, days int) ([]map[string]interface{}, error) {
	// Get stock mentions from database using StockAnalyzer
	mentions, err := ai.stockAnalyzer.GetStockMentions(chatID, "", days, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock mentions: %w", err)
	}

	// Get stock statistics
	stats, err := ai.stockAnalyzer.GetStockStats(chatID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock stats: %w", err)
	}

	// Convert to response format
	var results []map[string]interface{}

	// Add summary statistics
	summary := map[string]interface{}{
		"type":                "summary",
		"total_mentions":      stats.TotalMentions,
		"unique_stocks":       stats.UniqueStocks,
		"top_stocks":          stats.TopStocks,
		"sentiment_breakdown": stats.SentimentBreakdown,
		"time_distribution":   stats.TimeDistribution,
	}
	results = append(results, summary)

	// Add individual mentions
	for _, mention := range mentions {
		mentionData := map[string]interface{}{
			"type":           "mention",
			"id":             mention.ID,
			"stock_code":     mention.StockCode,
			"original_text":  mention.OriginalText,
			"corrected_code": mention.CorrectedCode,
			"confidence":     mention.Confidence,
			"context":        mention.Context,
			"sentiment":      mention.Sentiment,
			"timestamp":      mention.Timestamp,
			"platform":       mention.Platform,
		}
		results = append(results, mentionData)
	}

	return results, nil
}

// isLikelyStockCode checks if a 4-letter word is likely a stock code
func (ai *AIAnalyzer) isLikelyStockCode(word string) bool {
	// Common Indonesian stock codes
	knownStocks := map[string]bool{
		"BBCA": true, "BBRI": true, "BMRI": true, "TLKM": true,
		"GOTO": true, "BUKA": true, "EMTK": true, "ACES": true,
		"ADRO": true, "ANTM": true, "ASII": true, "BYAN": true,
		"CPIN": true, "GGRM": true, "HRUM": true, "ICBP": true,
		"INDF": true, "INTP": true, "ITMG": true, "JPFA": true,
		"KLBF": true, "LPKR": true, "LPPF": true, "MAPI": true,
		"MDKA": true, "MNCN": true, "PGAS": true, "PTBA": true,
		"PWON": true, "SMGR": true, "TBIG": true, "UNTR": true,
		"UNVR": true, "WIKA": true, "WSKT": true, "YPAS": true,
	}

	if knownStocks[word] {
		return true
	}

	// Common non-stock words to exclude
	excludeWords := map[string]bool{
		"YANG": true, "DARI": true, "AKAN": true, "BISA": true,
		"JUGA": true, "SAJA": true, "KALO": true, "TAPI": true,
		"UDAH": true, "BELUM": true, "MASIH": true, "SUDAH": true,
		"SAMA": true, "LAGI": true, "DULU": true, "NANTI": true,
	}

	if excludeWords[word] {
		return false
	}

	// If not in known lists, assume it might be a stock code
	return true
}

// extractContext extracts surrounding text for context
func (ai *AIAnalyzer) extractContext(text, word string) string {
	words := strings.Fields(text)
	for i, w := range words {
		if strings.Contains(strings.ToUpper(w), word) {
			start := i - 2
			end := i + 3
			if start < 0 {
				start = 0
			}
			if end > len(words) {
				end = len(words)
			}
			return strings.Join(words[start:end], " ")
		}
	}
	return text
}

// PerformScheduledAnalysis performs analysis for scheduled reports
func (ai *AIAnalyzer) PerformScheduledAnalysis(chatID string) (*AnalysisResult, error) {
	// Perform comprehensive analysis for the last 24 hours
	return ai.AnalyzeChatMessages(chatID, "", 1)
}
