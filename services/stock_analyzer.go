package services

import (
	"database/sql"
	"log"
	"regexp"
	"strings"
	"time"
)

// StockAnalyzer handles stock mention detection and analysis
type StockAnalyzer struct {
	db *sql.DB
}

// StockMention represents a detected stock mention
type StockMention struct {
	ID            int       `json:"id"`
	ChatID        string    `json:"chat_id"`
	Platform      string    `json:"platform"`
	StockCode     string    `json:"stock_code"`
	OriginalText  string    `json:"original_text"`
	CorrectedCode string    `json:"corrected_code"`
	Confidence    float64   `json:"confidence"`
	Context       string    `json:"context"`
	Sentiment     string    `json:"sentiment"`
	Timestamp     time.Time `json:"timestamp"`
}

// StockStats represents stock analysis statistics
type StockStats struct {
	TotalMentions      int              `json:"total_mentions"`
	UniqueStocks       int              `json:"unique_stocks"`
	TopStocks          []StockFrequency `json:"top_stocks"`
	SentimentBreakdown map[string]int   `json:"sentiment_breakdown"`
	TimeDistribution   map[string]int   `json:"time_distribution"`
}

// StockFrequency represents stock mention frequency
type StockFrequency struct {
	StockCode string `json:"stock_code"`
	Count     int    `json:"count"`
}

// NewStockAnalyzer creates a new StockAnalyzer instance
func NewStockAnalyzer(db *sql.DB) *StockAnalyzer {
	return &StockAnalyzer{
		db: db,
	}
}

// Indonesian stock codes and common typos
var stockCodes = map[string]string{
	// Banking
	"BBCA": "BBCA", "BCA": "BBCA", "bca": "BBCA",
	"BBRI": "BBRI", "BRI": "BBRI", "bri": "BBRI",
	"BMRI": "BMRI", "MANDIRI": "BMRI", "mandiri": "BMRI",
	"BBNI": "BBNI", "BNI": "BBNI", "bni": "BBNI",
	"BTPS": "BTPS", "BTPN": "BTPS",

	// Telco
	"TLKM": "TLKM", "TELKOM": "TLKM", "telkom": "TLKM",
	"EXCL": "EXCL", "XL": "EXCL", "xl": "EXCL",
	"ISAT": "ISAT", "INDOSAT": "ISAT", "indosat": "ISAT",

	// Mining
	"ANTM": "ANTM", "ANEKA": "ANTM", "aneka": "ANTM",
	"INCO": "INCO", "VALE": "INCO", "vale": "INCO",
	"PTBA": "PTBA", "BUKIT": "PTBA", "bukit": "PTBA",
	"ADRO": "ADRO", "ADARO": "ADRO", "adaro": "ADARO",

	// Consumer
	"UNVR": "UNVR", "UNILEVER": "UNVR", "unilever": "UNVR",
	"INDF": "INDF", "INDOFOOD": "INDF", "indofood": "INDF",
	"ICBP": "ICBP", "INDOFOOD CBP": "ICBP",
	"KLBF": "KLBF", "KALBE": "KLBF", "kalbe": "KLBF",

	// Energy
	"PGAS": "PGAS", "PGN": "PGAS", "pgn": "PGAS",
	"AKRA": "AKRA", "AKR": "AKRA", "akr": "AKRA",

	// Cement
	"SMGR": "SMGR", "SEMEN": "SMGR", "semen": "SMGR",
	"INTP": "INTP", "INDOCEMENT": "INTP", "indocement": "INTP",

	// Property
	"ASRI": "ASRI", "ALAM SUTERA": "ASRI",
	"BSDE": "BSDE", "BSD": "BSDE", "bsd": "BSDE",
	"LPKR": "LPKR", "LIPPO KARAWACI": "LPKR",

	// Technology
	"GOTO": "GOTO", "GOJEK": "GOTO", "gojek": "GOTO",
	"BUKA": "BUKA", "BUKALAPAK": "BUKA", "bukalapak": "BUKA",
	"EMTK": "EMTK", "ELANG": "EMTK",

	// Common typos and variations
	"bbca": "BBCA", "bbri": "BBRI", "bmri": "BMRI", "bbni": "BBNI",
	"tlkm": "TLKM", "excl": "EXCL", "isat": "ISAT",
	"antm": "ANTM", "inco": "INCO", "ptba": "PTBA", "adro": "ADRO",
	"unvr": "UNVR", "indf": "INDF", "icbp": "ICBP", "klbf": "KLBF",
	"pgas": "PGAS", "akra": "AKRA",
	"smgr": "SMGR", "intp": "INTP",
	"asri": "ASRI", "bsde": "BSDE", "lpkr": "LPKR",
	"goto": "GOTO", "buka": "BUKA", "emtk": "EMTK",
}

// DetectStockMentions detects stock mentions in text with typo correction
func (sa *StockAnalyzer) DetectStockMentions(text, chatID, platform string) []StockMention {
	var mentions []StockMention

	// Normalize text
	normalizedText := strings.ToLower(text)

	// Pattern for potential stock codes (3-4 uppercase letters)
	stockPattern := regexp.MustCompile(`\b[A-Za-z]{3,4}\b`)
	matches := stockPattern.FindAllString(text, -1)

	// Check each potential match
	for _, match := range matches {
		original := match
		matchLower := strings.ToLower(match)
		matchUpper := strings.ToUpper(match)

		// Direct match
		if corrected, exists := stockCodes[matchUpper]; exists {
			mention := StockMention{
				ChatID:        chatID,
				Platform:      platform,
				StockCode:     corrected,
				OriginalText:  original,
				CorrectedCode: corrected,
				Confidence:    1.0,
				Context:       sa.extractContext(text, original),
				Sentiment:     sa.analyzeSentiment(text),
				Timestamp:     time.Now(),
			}
			mentions = append(mentions, mention)
			continue
		}

		// Check lowercase version
		if corrected, exists := stockCodes[matchLower]; exists {
			mention := StockMention{
				ChatID:        chatID,
				Platform:      platform,
				StockCode:     corrected,
				OriginalText:  original,
				CorrectedCode: corrected,
				Confidence:    0.9,
				Context:       sa.extractContext(text, original),
				Sentiment:     sa.analyzeSentiment(text),
				Timestamp:     time.Now(),
			}
			mentions = append(mentions, mention)
			continue
		}

		// Fuzzy matching for typos
		if corrected := sa.findBestMatch(matchUpper); corrected != "" {
			confidence := sa.calculateConfidence(matchUpper, corrected)
			if confidence > 0.7 { // Only accept high confidence matches
				mention := StockMention{
					ChatID:        chatID,
					Platform:      platform,
					StockCode:     corrected,
					OriginalText:  original,
					CorrectedCode: corrected,
					Confidence:    confidence,
					Context:       sa.extractContext(text, original),
					Sentiment:     sa.analyzeSentiment(text),
					Timestamp:     time.Now(),
				}
				mentions = append(mentions, mention)
			}
		}
	}

	// Check for company names
	for name, code := range stockCodes {
		if len(name) > 4 && strings.Contains(normalizedText, strings.ToLower(name)) {
			mention := StockMention{
				ChatID:        chatID,
				Platform:      platform,
				StockCode:     code,
				OriginalText:  name,
				CorrectedCode: code,
				Confidence:    0.95,
				Context:       sa.extractContext(text, name),
				Sentiment:     sa.analyzeSentiment(text),
				Timestamp:     time.Now(),
			}
			mentions = append(mentions, mention)
		}
	}

	return sa.deduplicateMentions(mentions)
}

// findBestMatch finds the best matching stock code using Levenshtein distance
func (sa *StockAnalyzer) findBestMatch(input string) string {
	bestMatch := ""
	minDistance := 999

	for code := range stockCodes {
		if len(code) == 4 { // Only check 4-letter stock codes
			distance := sa.levenshteinDistance(input, code)
			if distance < minDistance && distance <= 1 { // Allow max 1 character difference
				minDistance = distance
				bestMatch = stockCodes[code]
			}
		}
	}

	return bestMatch
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (sa *StockAnalyzer) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}

	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(min(
				matrix[i-1][j]+1,  // deletion
				matrix[i][j-1]+1), // insertion
				matrix[i-1][j-1]+cost) // substitution
		}
	}

	return matrix[len(s1)][len(s2)]
}

// calculateConfidence calculates confidence score based on similarity
func (sa *StockAnalyzer) calculateConfidence(original, corrected string) float64 {
	distance := sa.levenshteinDistance(original, corrected)
	maxLen := max(len(original), len(corrected))

	if maxLen == 0 {
		return 1.0
	}

	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity
}

// extractContext extracts surrounding context for the stock mention
func (sa *StockAnalyzer) extractContext(text, mention string) string {
	index := strings.Index(strings.ToLower(text), strings.ToLower(mention))
	if index == -1 {
		return text // Return full text if mention not found
	}

	start := max(0, index-30)
	end := min(len(text), index+len(mention)+30)

	context := text[start:end]
	if start > 0 {
		context = "..." + context
	}
	if end < len(text) {
		context = context + "..."
	}

	return context
}

// analyzeSentiment performs basic sentiment analysis
func (sa *StockAnalyzer) analyzeSentiment(text string) string {
	text = strings.ToLower(text)

	positiveWords := []string{"naik", "bullish", "bagus", "profit", "untung", "buy", "beli", "target", "breakout", "rally"}
	negativeWords := []string{"turun", "bearish", "jelek", "rugi", "loss", "sell", "jual", "drop", "crash", "correction"}

	positiveCount := 0
	negativeCount := 0

	for _, word := range positiveWords {
		if strings.Contains(text, word) {
			positiveCount++
		}
	}

	for _, word := range negativeWords {
		if strings.Contains(text, word) {
			negativeCount++
		}
	}

	if positiveCount > negativeCount {
		return "positive"
	} else if negativeCount > positiveCount {
		return "negative"
	}
	return "neutral"
}

// deduplicateMentions removes duplicate mentions
func (sa *StockAnalyzer) deduplicateMentions(mentions []StockMention) []StockMention {
	seen := make(map[string]bool)
	var unique []StockMention

	for _, mention := range mentions {
		key := mention.StockCode + "_" + mention.Platform + "_" + mention.ChatID
		if !seen[key] {
			seen[key] = true
			unique = append(unique, mention)
		}
	}

	return unique
}

// StoreMentions stores stock mentions in database
func (sa *StockAnalyzer) StoreMentions(mentions []StockMention) error {
	if len(mentions) == 0 {
		return nil
	}

	query := `INSERT INTO stock_mentions (chat_id, platform, stock_code, original_text, corrected_code, confidence, context, sentiment, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, mention := range mentions {
		_, err := sa.db.Exec(query, mention.ChatID, mention.Platform, mention.StockCode, mention.OriginalText, mention.CorrectedCode, mention.Confidence, mention.Context, mention.Sentiment, mention.Timestamp)
		if err != nil {
			log.Printf("Error storing stock mention: %v", err)
			return err
		}
	}

	return nil
}

// GetStockMentions retrieves stock mentions from database
func (sa *StockAnalyzer) GetStockMentions(chatID, stockCode string, days int, limit int) ([]StockMention, error) {
	var mentions []StockMention

	query := `SELECT id, chat_id, platform, stock_code, original_text, corrected_code, confidence, context, sentiment, timestamp FROM stock_mentions WHERE 1=1`
	args := []interface{}{}

	if chatID != "" {
		query += " AND chat_id = ?"
		args = append(args, chatID)
	}

	if stockCode != "" {
		query += " AND stock_code = ?"
		args = append(args, stockCode)
	}

	if days > 0 {
		query += " AND timestamp >= ?"
		args = append(args, time.Now().AddDate(0, 0, -days))
	}

	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := sa.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mention StockMention
		err := rows.Scan(&mention.ID, &mention.ChatID, &mention.Platform, &mention.StockCode, &mention.OriginalText, &mention.CorrectedCode, &mention.Confidence, &mention.Context, &mention.Sentiment, &mention.Timestamp)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, mention)
	}

	return mentions, nil
}

// GetStockStats retrieves stock analysis statistics
func (sa *StockAnalyzer) GetStockStats(chatID string, days int) (*StockStats, error) {
	stats := &StockStats{
		SentimentBreakdown: make(map[string]int),
		TimeDistribution:   make(map[string]int),
	}

	// Get total mentions
	totalQuery := `SELECT COUNT(*) FROM stock_mentions WHERE 1=1`
	args := []interface{}{}

	if chatID != "" {
		totalQuery += " AND chat_id = ?"
		args = append(args, chatID)
	}

	if days > 0 {
		totalQuery += " AND timestamp >= ?"
		args = append(args, time.Now().AddDate(0, 0, -days))
	}

	err := sa.db.QueryRow(totalQuery, args...).Scan(&stats.TotalMentions)
	if err != nil {
		return nil, err
	}

	// Get unique stocks count
	uniqueQuery := `SELECT COUNT(DISTINCT stock_code) FROM stock_mentions WHERE 1=1`
	if chatID != "" {
		uniqueQuery += " AND chat_id = ?"
	}
	if days > 0 {
		uniqueQuery += " AND timestamp >= ?"
	}

	err = sa.db.QueryRow(uniqueQuery, args...).Scan(&stats.UniqueStocks)
	if err != nil {
		return nil, err
	}

	// Get top stocks
	topStocksQuery := `SELECT stock_code, COUNT(*) as count FROM stock_mentions WHERE 1=1`
	if chatID != "" {
		topStocksQuery += " AND chat_id = ?"
	}
	if days > 0 {
		topStocksQuery += " AND timestamp >= ?"
	}
	topStocksQuery += " GROUP BY stock_code ORDER BY count DESC LIMIT 10"

	rows, err := sa.db.Query(topStocksQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var freq StockFrequency
		err := rows.Scan(&freq.StockCode, &freq.Count)
		if err != nil {
			return nil, err
		}
		stats.TopStocks = append(stats.TopStocks, freq)
	}

	// Get sentiment breakdown
	sentimentQuery := `SELECT sentiment, COUNT(*) FROM stock_mentions WHERE 1=1`
	if chatID != "" {
		sentimentQuery += " AND chat_id = ?"
	}
	if days > 0 {
		sentimentQuery += " AND timestamp >= ?"
	}
	sentimentQuery += " GROUP BY sentiment"

	rows, err = sa.db.Query(sentimentQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sentiment string
		var count int
		err := rows.Scan(&sentiment, &count)
		if err != nil {
			return nil, err
		}
		stats.SentimentBreakdown[sentiment] = count
	}

	// Get time distribution (by hour)
	timeQuery := `SELECT strftime('%H', timestamp) as hour, COUNT(*) FROM stock_mentions WHERE 1=1`
	if chatID != "" {
		timeQuery += " AND chat_id = ?"
	}
	if days > 0 {
		timeQuery += " AND timestamp >= ?"
	}
	timeQuery += " GROUP BY hour ORDER BY hour"

	rows, err = sa.db.Query(timeQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hour string
		var count int
		err := rows.Scan(&hour, &count)
		if err != nil {
			return nil, err
		}
		stats.TimeDistribution[hour] = count
	}

	return stats, nil
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
