#!/bin/bash

# 1. Deteksi arsitektur sistem
MACHINE_TYPE=$(uname -m)

if [ "${MACHINE_TYPE}" == "x86_64" ]; then
  ARCH="amd64"
  # URL Khusus Linux AMD64
  DOWNLOAD_URL="https://git.shiestapoi.biz.id/public/linux/takatax-linux-amd64"
elif [ "${MACHINE_TYPE}" == "aarch64" ] || [ "${MACHINE_TYPE}" == "arm64" ]; then
  ARCH="arm64"
  # URL Khusus Linux ARM64
  DOWNLOAD_URL="https://git.shiestapoi.biz.id/public/linux/takatax-linux-arm64"
else
  echo "❌ Arsitektur tidak didukung: ${MACHINE_TYPE}"
  exit 1
fi

echo "🔍 Sistem terdeteksi: Linux (${ARCH})"

# 2. Mengunduh file Takatax
echo "⬇️ Mengunduh Takatax dari: ${DOWNLOAD_URL}"
# Menggunakan curl -L (untuk mengikuti redirect jika ada) dan -o (untuk nama output)
curl -L -o takatax "${DOWNLOAD_URL}"

# 3. Memberikan akses eksekusi agar bisa dijalankan di Linux
chmod +x takatax

# 4. Menjalankan aplikasi Takatax dengan argumen khusus
echo "🚀 Menjalankan Takatax dengan argumen..."
./takatax --dev --debug --test --level=high --phase=500

# 5. Menangkap hasil keluaran (Exit Code)
if [ $? -eq 0 ]; then
  echo "✅ Test Takatax di Linux ${ARCH} berhasil!"
  exit 0
else
  echo "❌ Test Takatax di Linux ${ARCH} gagal!"
  exit 1
fi