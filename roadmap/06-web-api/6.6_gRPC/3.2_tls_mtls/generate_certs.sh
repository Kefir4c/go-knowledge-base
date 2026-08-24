#!/bin/bash
# generate_certs.sh — правильный скрипт для Windows/Git Bash

set -e  # остановка при любой ошибке

# Переходим в папку certs
mkdir -p certs
cd certs

# Если есть старые сертификаты — удалим их (чтобы не мешали)
rm -f *.pem *.srl

# Генерация CA
openssl genrsa -out ca_key.pem 2048
openssl req -new -x509 -days 365 -key ca_key.pem -out ca_cert.pem -subj "//C=RU/ST=Moscow/L=Moscow/O=Example/CN=Example CA"

# Генерация серверного ключа и CSR
openssl genrsa -out server_key.pem 2048
openssl req -new -key server_key.pem -out server_csr.pem -subj "//C=RU/ST=Moscow/L=Moscow/O=Example/CN=localhost"

# Генерация клиентского ключа и CSR
openssl genrsa -out client_key.pem 2048
openssl req -new -key client_key.pem -out client_csr.pem -subj "//C=RU/ST=Moscow/L=Moscow/O=Example/CN=client-1"

# Создаём файл конфигурации для SAN
cat > san.cnf <<EOF
[req_ext]
subjectAltName=DNS:localhost,IP:127.0.0.1
EOF

# Подписываем серверный сертификат CA
openssl x509 -req -days 365 -in server_csr.pem -CA ca_cert.pem -CAkey ca_key.pem -CAcreateserial -out server_cert.pem -extensions req_ext -extfile san.cnf

# Подписываем клиентский сертификат CA
openssl x509 -req -days 365 -in client_csr.pem -CA ca_cert.pem -CAkey ca_key.pem -CAcreateserial -out client_cert.pem

# Удаляем временные файлы
rm -f *.csr *.srl

echo "✅ Все сертификаты созданы!"
ls -la