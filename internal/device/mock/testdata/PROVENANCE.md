system-board.json and system-info.json are byte-identical copies of testdata/device/axt1800-25.12.6/ (see its PROVENANCE.md); seed_test.go enforces it.

net-counters.txt is a byte-identical copy of testdata/device/axt1800-25.12.6/wwan-down/net-counters.txt (see its PROVENANCE.md). The mock uses the latest counter row for each interface.

- `device-status-phy0-sta0.json` is copied verbatim from `testdata/device/axt1800-25.12.6/device-status-phy0-sta0.json` (connected STA). The default-route tuple comes from the adjacent `ip-route-default.txt`.
- `wireless-status.json` and `iwinfo-scan-phy0-sta0.json` are byte-identical copies from `testdata/device/axt1800-25.12.6/wifi-scan/connected/`.
- `iwinfo-scan-phy1-ap0.json` is a byte-identical copy from `testdata/device/axt1800-25.12.6/wifi-scan/no-sta/`.
- `uci-show-wireless.txt` is a byte-identical copy from `testdata/device/axt1800-25.12.6/wifi-scan/connected/`; mock mode commits it as the wireless config so AP settings and join plans read the same layout as the seeded status.

The WiFi seeds use the connected summary with one scan per radio. Per-interface `network.device status` continues to use `SeedConnectivity`'s connected STA capture for every name, including the AP.

- `uci-export-dhcp.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/uci-export-dhcp.txt`; mock mode loads it as the `dhcp` config so the DNS layer stack has a real dnsmasq section.

- `uci-export-network.txt` and `uci-export-firewall.txt` are byte-identical copies from `testdata/device/axt1800-25.12.6/usbtether/` (already redacted there); mock mode loads them as the `network` and `firewall` configs.

- `mwan3-stock-config.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/mwan3/stock-config-mwan3.txt`; mock mode loads it as the `mwan3` config (the parser gets a `package mwan3` line prepended, as `uci export` prints it).

- `sysfs-net-links.tsv` is a byte-identical copy of `testdata/device/axt1800-25.12.6/sysfs-net/links.tsv`; mock mode lists these netdevs. No phone capture exists yet, so mock mode has no tether link until `AddNetLink` adds one (tests).
- `uci-export-system.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/uci-export-system.txt`; mock mode loads it as the `system` config so the device settings card reads and writes the real section shapes.
- `logread.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/logs/logread.txt`; mock mode answers `logread` with it.

- `dhcp-leases.txt`, `ip-neigh-clients.json` and `hostapd-phy1-ap0-get_clients.json` are byte-identical copies of `testdata/device/axt1800-25.12.6/with-ap-client/` (`dhcp-leases.txt`, `ip-neigh.json`, `hostapd-phy1-ap0-get_clients.json`); `SeedClients` scripts one leased WiFi station on phy1-ap0 and the wired capture host.
- `interface-dump.json` is a byte-identical copy of `testdata/device/axt1800-25.12.6/interface-dump.json`; `SeedClients` uses it so the client list keeps LAN-side neighbours and drops the uplink gateway.
- `diag-ping-ok.txt`, `diag-ping-bad-host.txt`, `diag-traceroute-ok.txt`, `diag-lookup-ok.txt` and `diag-lookup-nxdomain.txt` are byte-identical copies of `testdata/device/axt1800-25.12.6/diagnostics/` (without the `diag-` prefix); mock `Diagnose` answers with them (an `.invalid` target gets the failure captures) and drops the `exit=N` line the capture appended.
- `versions.json` is a byte-identical copy of `testdata/device/axt1800-25.12.6/updates/versions.json`; mock mode answers the OpenWrt release check with it.
- `sysfs-leds.tsv` is a byte-identical copy of `testdata/device/axt1800-25.12.6/leds/sysfs-default.tsv`; mock mode lists these LEDs and shows one dark while a committed `system` led section holds it off (`default 0`, trigger `none`).
- `devicetree-keys.tsv` is a byte-identical copy of `testdata/device/axt1800-25.12.6/buttons/devicetree-keys.tsv`; mock mode lists these buttons (reset key and slide switch).
- `validate-firmware-zero-image.json` is a byte-identical copy of `testdata/container/openwrt-25.12.6/sysupgrade/validate-zero-image.json` (container capture, a zero file: `valid:false`); mock `ValidateFirmware` answers with it. Tests that need a valid image flip only the `valid` value.
- `apk-world.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/sysupgrade/etc-apk-world.txt`; mock `PackageWorlds` returns it as both worlds (as on the router: nothing user-installed).
