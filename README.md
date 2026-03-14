# TeleTowa - Aplikasi Integrasi Telegram dan WhatsApp

TeleTowa adalah aplikasi berbasis Go yang memungkinkan integrasi antara platform Telegram dan WhatsApp untuk pengelolaan pesan.

## Persyaratan Sistem

- Go 1.16 atau lebih baru
- SQLite3
- Koneksi internet yang stabil
- Akun Telegram dan WhatsApp

## Langkah-langkah Menjalankan Aplikasi

### 1. Persiapan Awal

1. Clone repositori ini ke komputer lokal Anda:
   ```bash
   git clone https://github.com/username/teletowa.git
   cd teletowa
   ```

2. Instal semua dependensi yang diperlukan:
   ```bash
   go mod download
   ```

### 2. Konfigurasi Aplikasi

1. Salin file konfigurasi contoh (jika ada):
   ```bash
   cp config.example.json config.json
   ```

2. Edit file konfigurasi sesuai kebutuhan Anda, seperti port server, kredensial database, dll.

### 3. Menjalankan Aplikasi

1. Build aplikasi:
   ```bash
   go build -o teletowa
   ```

2. Jalankan aplikasi:
   ```bash
   ./teletowa
   ```

   Atau langsung menggunakan Go:
   ```bash
   go run main.go
   ```

3. Aplikasi akan berjalan pada alamat dan port yang ditentukan dalam konfigurasi (default: http://localhost:8080)

### 4. Login dan Konfigurasi Layanan

1. Buka browser dan akses `http://localhost:8080/login`
2. Login menggunakan kredensial yang telah diatur
3. Untuk mengonfigurasi WhatsApp:
   - Kunjungi `http://localhost:8080/whatsapp/login`
   - Scan QR code yang muncul dengan aplikasi WhatsApp di ponsel Anda
4. Untuk mengonfigurasi Telegram:
   - Kunjungi `http://localhost:8080/telegram/login`
   - Ikuti petunjuk untuk memasukkan nomor telepon dan kode verifikasi

### 5. Penggunaan Fitur

- Dashboard utama: `http://localhost:8080/`
- Pengelolaan WhatsApp: `http://localhost:8080/whatsapp/`
- Pengelolaan Telegram: `http://localhost:8080/telegram/`
- Panel pengiriman pesan kustom: `http://localhost:8080/whatsapp/panelchat`

## Pemecahan Masalah

- Jika aplikasi tidak dapat terhubung ke WhatsApp, coba logout dan login kembali
- Untuk masalah koneksi Telegram, periksa kredensial dan status otentikasi di halaman status

## Informasi Tambahan

- Aplikasi ini menggunakan library go-whatsapp untuk integrasi WhatsApp
- Untuk Telegram, aplikasi menggunakan API resmi Telegram
- Data konfigurasi disimpan dalam database SQLite lokal
