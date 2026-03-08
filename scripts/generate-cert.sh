#!/bin/bash
set -e
cd "$(dirname "$0")/.."

CERT_DIR="${1:-.}"
CERT_FILE="${CERT_DIR}/mumble-server.crt"
KEY_FILE="${CERT_DIR}/mumble-server.key"
DAYS="${DAYS:-3650}"

if [ -f "${CERT_FILE}" ] && [ -f "${KEY_FILE}" ]; then
    echo "Certificate already exists at ${CERT_FILE}"
    echo "Remove existing files to regenerate."
    exit 0
fi

echo "Generating self-signed TLS certificate..."
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -keyout "${KEY_FILE}" \
    -out "${CERT_FILE}" \
    -days "${DAYS}" \
    -nodes \
    -subj "/CN=Mumble Server" \
    2>/dev/null

chmod 600 "${KEY_FILE}"

echo "Certificate: ${CERT_FILE}"
echo "Private key: ${KEY_FILE} (mode 0600)"
echo "Valid for ${DAYS} days."
