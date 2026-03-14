package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger middleware untuk log request
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Waktu mulai
		t := time.Now()

		// Proses request
		c.Next()

		// Hitung waktu eksekusi
		latency := time.Since(t)

		// Log info
		log.Printf("[%s] %s %s %d %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
			c.Writer.Status(),
			latency,
		)
	}
}
