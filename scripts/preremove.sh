#!/bin/sh
set -e

if command -v systemctl > /dev/null 2>&1; then
    systemctl stop go-mumble-server.service || true
    systemctl disable go-mumble-server.service || true
fi
