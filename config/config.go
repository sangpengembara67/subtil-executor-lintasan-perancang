package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

// Config - Struktur konfigurasi aplikasi
type Config struct {
	ServerAddr     string `json:"server_addr"`
	TelegramToken  string `json:"telegram_token"`
	WhatsAppConfig string `json:"whatsapp_config"` // Path ke file konfigurasi WhatsApp
	AdminUsername  string `json:"admin_username"`
	AdminPassword  string `json:"admin_password"`
	PhoneOwner     string `json:"phone_owner"` // Nama pemilik nomor telepon
}

// TelegramConfig - Struktur konfigurasi Telegram
type TelegramConfig struct {
	// Data yang dibutuhkan untuk login user
	ApiID            int      `json:"api_id"`
	ApiHash          string   `json:"api_hash"`
	PhoneNumber      string   `json:"phone_number"`
	PhoneCodeHash    string   `json:"phone_code_hash"`
	UserMode         bool     `json:"user_mode"`
	SessionFile      string   `json:"session_file"`
	Active           bool     `json:"active"`
	StatusConnected  bool     `json:"status_connected"`
	SelectedChats    []string `json:"selected_chats"`
	SelectedGroups   []string `json:"selected_groups"`
	SelectedChannels []string `json:"selected_channels"`
}

// WhatsAppConfig - Struktur konfigurasi WhatsApp
type WhatsAppConfig struct {
	LoggedIn       bool     `json:"logged_in"`
	SelectedChats  []string `json:"selected_chats"`
	SelectedGroups []string `json:"selected_groups"`
	Active         bool     `json:"active"`
	PhoneNumber    string   `json:"phone_number"`
}

// ForwardConfig - Struktur konfigurasi forward message
type ForwardConfig struct {
	TelegramToWhatsApp map[string]string `json:"telegram_to_whatsapp"`
	Active             bool              `json:"active"`
	MessageFormat      string            `json:"message_format"`
}

// Default config
var defaultConfig = Config{
	ServerAddr:     ":8080",
	AdminUsername:  "admin",
	AdminPassword:  "admin",
	TelegramToken:  "",
	WhatsAppConfig: "./data/whatsapp.json",
	PhoneOwner:     "6282179438863",
}

// LoadConfig - Memuat konfigurasi dari file, jika tidak ada akan membuat default
func LoadConfig() *Config {
	// Cek apakah file config.json ada
	if _, err := os.Stat("./config.json"); os.IsNotExist(err) {
		// File tidak ada, buat default
		saveConfig(&defaultConfig)
		return &defaultConfig
	}

	// Baca file konfigurasi
	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Printf("Error reading config file: %v, using defaults", err)
		return &defaultConfig
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		log.Printf("Error parsing config file: %v, using defaults", err)
		return &defaultConfig
	}

	return &config
}

// SaveConfig - Menyimpan konfigurasi ke file
func saveConfig(config *Config) error {
	// Convert ke JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Tulis ke file
	if err := ioutil.WriteFile("./config.json", jsonData, 0644); err != nil {
		return err
	}

	return nil
}

// GetTelegramConfig - Memuat konfigurasi Telegram
func GetTelegramConfig() (*TelegramConfig, error) {
	// Cek apakah file telegram.json ada
	if _, err := os.Stat("./data/telegram.json"); os.IsNotExist(err) {
		// File tidak ada, buat default
		defaultTelegramConfig := TelegramConfig{
			ApiID:            0,
			ApiHash:          "",
			PhoneNumber:      "",
			PhoneCodeHash:    "",
			UserMode:         false,
			SessionFile:      "./data/telegram_session.json",
			Active:           true,
			StatusConnected:  false,
			SelectedChats:    []string{},
			SelectedGroups:   []string{},
			SelectedChannels: []string{},
		}

		// Pastikan direktori data ada
		if err := os.MkdirAll("./data", 0755); err != nil {
			return nil, err
		}

		jsonData, err := json.MarshalIndent(defaultTelegramConfig, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := ioutil.WriteFile("./data/telegram.json", jsonData, 0644); err != nil {
			return nil, err
		}

		return &defaultTelegramConfig, nil
	}

	// Baca file konfigurasi
	file, err := ioutil.ReadFile("./data/telegram.json")
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var config TelegramConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveTelegramConfig - Menyimpan konfigurasi Telegram
func SaveTelegramConfig(config *TelegramConfig) error {
	// Pastikan direktori data ada
	if err := os.MkdirAll("./data", 0755); err != nil {
		return err
	}

	// Convert ke JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Tulis ke file
	if err := ioutil.WriteFile("./data/telegram.json", jsonData, 0644); err != nil {
		return err
	}

	return nil
}

// GetWhatsAppConfig - Memuat konfigurasi WhatsApp
func GetWhatsAppConfig() (*WhatsAppConfig, error) {
	// Cek apakah file whatsapp.json ada
	if _, err := os.Stat("./data/whatsapp.json"); os.IsNotExist(err) {
		// File tidak ada, buat default
		defaultWhatsAppConfig := WhatsAppConfig{
			LoggedIn:       false,
			SelectedChats:  []string{},
			SelectedGroups: []string{},
			Active:         false,
		}

		// Pastikan direktori data ada
		if err := os.MkdirAll("./data", 0755); err != nil {
			return nil, err
		}

		jsonData, err := json.MarshalIndent(defaultWhatsAppConfig, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := ioutil.WriteFile("./data/whatsapp.json", jsonData, 0644); err != nil {
			return nil, err
		}

		return &defaultWhatsAppConfig, nil
	}

	// Baca file konfigurasi
	file, err := ioutil.ReadFile("./data/whatsapp.json")
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var config WhatsAppConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveWhatsAppConfig - Menyimpan konfigurasi WhatsApp
func SaveWhatsAppConfig(config *WhatsAppConfig) error {
	// Pastikan direktori data ada
	if err := os.MkdirAll("./data", 0755); err != nil {
		return err
	}

	// Convert ke JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Tulis ke file
	if err := ioutil.WriteFile("./data/whatsapp.json", jsonData, 0644); err != nil {
		return err
	}

	return nil
}
