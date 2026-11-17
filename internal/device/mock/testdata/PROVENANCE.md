system-board.json and system-info.json are byte-identical copies of testdata/device/axt1800-25.12.6/ (see its PROVENANCE.md); seed_test.go enforces it.

net-counters.txt is a byte-identical copy of testdata/device/axt1800-25.12.6/wwan-down/net-counters.txt (see its PROVENANCE.md). The mock uses the latest counter row for each interface.

- `device-status-phy0-sta0.json` is copied verbatim from `testdata/device/axt1800-25.12.6/device-status-phy0-sta0.json` (connected STA). The default-route tuple comes from the adjacent `ip-route-default.txt`.
- `wireless-status.json` and `iwinfo-scan-phy0-sta0.json` are byte-identical copies from `testdata/device/axt1800-25.12.6/wifi-scan/connected/`.
- `iwinfo-scan-phy1-ap0.json` is a byte-identical copy from `testdata/device/axt1800-25.12.6/wifi-scan/no-sta/`.

The WiFi seeds use the connected summary with one scan per radio. Per-interface `network.device status` continues to use `SeedConnectivity`'s connected STA capture for every name, including the AP.

- `uci-export-dhcp.txt` is a byte-identical copy of `testdata/device/axt1800-25.12.6/uci-export-dhcp.txt`; mock mode loads it as the `dhcp` config so the DNS layer stack has a real dnsmasq section.
