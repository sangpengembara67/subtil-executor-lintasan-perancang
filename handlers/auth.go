package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shiestapoi/teletowa/middleware"
)

// ShowLoginPage - Menampilkan halaman login
func ShowLoginPage(c *gin.Context) {
	// Cek apakah sudah login
	_, err := c.Cookie("session")
	if err == nil {
		// Sudah login, redirect ke dashboard
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "Login - TeleTowa",
	})
}

// ProcessLogin - Memproses form login
func ProcessLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Validasi kredensial
	if middleware.ValidateCredentials(username, password) {
		// Set cookie sesi
		c.SetCookie(
			"session",
			username,
			3600*24, // 1 hari
			"/",
			"",
			false,
			true,
		)

		// Redirect ke dashboard
		c.Redirect(http.StatusFound, "/")
	} else {
		// Tampilkan error login
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title":    "Login - TeleTowa",
			"Error":    "Username atau password salah",
			"Username": username,
		})
	}
}

// Logout - Menghapus sesi dan logout
func Logout(c *gin.Context) {
	// Hapus cookie sesi
	c.SetCookie(
		"session",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	// Redirect ke halaman login
	c.Redirect(http.StatusFound, "/login")
}
