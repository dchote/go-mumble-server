#!/bin/sh
set -e

DATA_DIR="/var/lib/go-mumble-server"
CONFIG_DIR="/etc/go-mumble-server"
LOG_DIR="/var/log/go-mumble-server"

mkdir -p "${DATA_DIR}" "${CONFIG_DIR}" "${LOG_DIR}"

if ! getent group mumble-server > /dev/null 2>&1; then
    groupadd --system mumble-server
fi

if ! getent passwd mumble-server > /dev/null 2>&1; then
    useradd --system --gid mumble-server --home-dir "${DATA_DIR}" --shell /usr/sbin/nologin mumble-server
fi

chown mumble-server:mumble-server "${DATA_DIR}" "${LOG_DIR}"
chmod 750 "${DATA_DIR}" "${LOG_DIR}"
chmod 750 "${CONFIG_DIR}"

if [ ! -f "${CONFIG_DIR}/mumble-server.toml" ]; then
    if [ -f "/usr/share/go-mumble-server/mumble-server.toml" ]; then
        cp "/usr/share/go-mumble-server/mumble-server.toml" "${CONFIG_DIR}/mumble-server.toml"
        chown root:mumble-server "${CONFIG_DIR}/mumble-server.toml"
        chmod 640 "${CONFIG_DIR}/mumble-server.toml"
    fi
fi

if [ ! -f "${CONFIG_DIR}/mumble-server.crt" ] || [ ! -f "${CONFIG_DIR}/mumble-server.key" ]; then
    if command -v openssl > /dev/null 2>&1; then
        openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
            -keyout "${CONFIG_DIR}/mumble-server.key" \
            -out "${CONFIG_DIR}/mumble-server.crt" \
            -days 3650 -nodes \
            -subj "/CN=Mumble Server" 2>/dev/null
        chown root:mumble-server "${CONFIG_DIR}/mumble-server.key" "${CONFIG_DIR}/mumble-server.crt"
        chmod 640 "${CONFIG_DIR}/mumble-server.key" "${CONFIG_DIR}/mumble-server.crt"
        echo "Generated self-signed TLS certificate in ${CONFIG_DIR}/"
    fi
fi

if command -v systemctl > /dev/null 2>&1; then
    systemctl daemon-reload
    systemctl enable go-mumble-server.service || true
fi
