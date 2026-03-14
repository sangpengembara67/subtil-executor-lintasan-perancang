package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/config"
)

// Auth middleware untuk memeriksa cookie sesi
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Jika mengakses halaman login atau API login/logout, lewati
		if c.Request.URL.Path == "/login" || c.Request.URL.Path == "/api/login" || c.Request.URL.Path == "/api/logout" {
			c.Next()
			return
		}

		// Periksa cookie sesi
		_, err := c.Cookie("session")
		if err != nil {
			// Redirect ke halaman login jika tidak ada sesi
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAuth middleware untuk memastikan user sudah login
func RequireAuth(c *gin.Context) {
	// Periksa cookie sesi
	_, err := c.Cookie("session")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	c.Next()
}

// ValidateCredentials - Memvalidasi kredensial login
func ValidateCredentials(username, password string) bool {
	cfg := config.LoadConfig()
	return username == cfg.AdminUsername && password == cfg.AdminPassword
}
