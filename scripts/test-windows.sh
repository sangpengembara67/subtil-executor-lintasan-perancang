#!/bin/bash

# Pastikan script berhenti jika ada command yang gagal
set -e

ARCH="amd64"
DOWNLOAD_URL="https://git.shiestapoi.biz.id/public/windows/takatax-windows-x86_64.exe"

echo "🔍 Target pengujian: Windows (${ARCH})"

# 1. Mengunduh file Takatax (.exe)
echo "⬇️ Mengunduh Takatax dari: ${DOWNLOAD_URL}"
# Tambahkan -f agar curl gagal jika URL 404
curl -f -L -o takatax-x86_64.exe "${DOWNLOAD_URL}"

# Verifikasi apakah file berhasil diunduh dan tidak kosong
if [ ! -s "./takatax-x86_64.exe" ]; then
    echo "❌ ERROR: File takatax-x86_64.exe tidak ditemukan atau kosong!"
    exit 1
fi

chmod +x takatax-x86_64.exe

# 2. Menjalankan aplikasi Takatax
echo "🚀 Menjalankan takatax-x86_64.exe dengan path absolut..."

# Menggunakan $(pwd) memastikan path dikenal oleh Windows & Bash
"$(pwd)/takatax-x86_64.exe" --dev --debug --test --level=high --phase=500

# Simpan exit status
EXIT_STATUS=$?

# 3. Evaluasi hasil test
if [ $EXIT_STATUS -eq 0 ]; then
    echo "---"
    echo "✅ Test Takatax di Windows ${ARCH} berhasil (Exit Code: $EXIT_STATUS)"
    exit 0
else
    echo "---"
    echo "❌ Error Terdeteksi!"
    echo "Aplikasi keluar dengan kode kesalahan: $EXIT_STATUS"
    exit 1
fi