#!/bin/sh
# Runs the real watchdog and netifd in a disposable ARM64 OpenWrt container.
set -eu

cd "$(dirname "$0")/.."
dist=$(mktemp -d)
container=
trap 'if [ -n "$container" ]; then docker rm -f "$container" >/dev/null; fi; rm -rf "$dist"' EXIT
image=openwrt/rootfs:aarch64_generic-25.12.6@sha256:22f13387da208c0fb4150c41f3ec12e34cc5994a0717a49e50aa9f9ae6cb5e7b
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -tags integration -c -o "$dist/txn.test" ./internal/txn
container=$(docker run -d --platform linux/aarch64_generic --cap-add NET_ADMIN \
	-v "$dist:/t:ro" --entrypoint /bin/sh "$image" \
	-c 'apk add wireguard-tools >/tmp/wg-packages.log 2>&1 && exec /sbin/init')

for attempt in $(seq 1 60); do
	if docker exec "$container" ubus -t 1 list network.interface 2>/dev/null | grep -qx network.interface; then
		break
	fi
	if [ "$attempt" -eq 60 ]; then
		echo "OpenWrt netifd did not start" >&2
		exit 1
	fi
	sleep 1
done

# The isolated Docker network can lose its default route when netifd starts.
# Use nohostroute and documentation endpoints so the test needs no remote peer.
docker exec -i "$container" /bin/sh -s <<'SETUP'
set -eu
key=$(wg genkey)
peer=$(wg genkey | wg pubkey)
uci -q batch >/dev/null <<UCI
set network.wg0=interface
set network.wg0.proto=wireguard
set network.wg0.private_key=$key
set network.wg0.disabled=0
set network.wg0.nohostroute=1
add_list network.wg0.addresses=198.51.100.2/32
set network.wg0_peer0=wireguard_wg0
set network.wg0_peer0.public_key=$peer
set network.wg0_peer0.endpoint_host=192.0.2.10
set network.wg0_peer0.endpoint_port=51820
add_list network.wg0_peer0.allowed_ips=198.51.100.1/32
commit network
UCI
SETUP
docker exec -e TRAVO_WG_ROLLBACK_CONTAINER=1 "$container" /t/txn.test \
	-test.run '^TestRealWireGuardRollback$' -test.v -test.timeout 5m
