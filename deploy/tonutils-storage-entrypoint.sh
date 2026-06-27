#!/bin/sh
set -eu

DB_PATH="${TONUTILS_STORAGE_DB:-/data/db}"
CONFIG_PATH="${DB_PATH}/config.json"
EXTERNAL="${TONUTILS_STORAGE_EXTERNAL_IP:-}"
UDP_PORT="${TONUTILS_STORAGE_UDP_PORT:-47431}"

resolve_ipv4() {
	host="$1"
	case "$host" in
	*[!0-9.]*|'')
		getent ahostsv4 "$host" 2>/dev/null | awk 'NR==1 {print $1; exit}'
		;;
	*)
		printf '%s' "$host"
		;;
	esac
}

patch_config() {
	ip="$1"
	if [ ! -f "$CONFIG_PATH" ]; then
		echo "tonutils-storage-entrypoint: config not found at ${CONFIG_PATH}, daemon will create it" >&2
		return 0
	fi
	if [ -z "$ip" ]; then
		echo "tonutils-storage-entrypoint: TONUTILS_STORAGE_EXTERNAL_IP is empty, overlay seeding may not work" >&2
		return 0
	fi

	tmp="${CONFIG_PATH}.tmp"
	sed \
		-e "s/\"ExternalIP\": \"[^\"]*\"/\"ExternalIP\": \"${ip}\"/" \
		-e "s/\"ListenAddr\": \"[^\"]*\"/\"ListenAddr\": \"0.0.0.0:${UDP_PORT}\"/" \
		"$CONFIG_PATH" >"$tmp"
	mv "$tmp" "$CONFIG_PATH"
	echo "tonutils-storage-entrypoint: ExternalIP=${ip} ListenAddr=0.0.0.0:${UDP_PORT}" >&2
}

if [ -n "$EXTERNAL" ]; then
	IP="$(resolve_ipv4 "$EXTERNAL")"
	if [ -z "$IP" ]; then
		echo "tonutils-storage-entrypoint: failed to resolve ${EXTERNAL}" >&2
		exit 1
	fi
	patch_config "$IP"
fi

exec /usr/local/bin/tonutils-storage "$@"
