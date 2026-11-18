package mock

import (
	_ "embed"
)

//go:embed testdata/system-board.json
var systemBoard []byte

//go:embed testdata/system-info.json
var systemInfo []byte

// SeedSystem scripts the system ubus replies with the verbatim AXT1800 captures.
func SeedSystem(d *Device) {
	d.SetUbusReply("system", "board", systemBoard)
	d.SetUbusReply("system", "info", systemInfo)
}

// DHCPExport is the router's `uci export dhcp` (old Travo's AdGuard forwarding as base);
// the app parses it with the cli adapter's parser to seed the DNS layer stack in mock mode.
//
//go:embed testdata/uci-export-dhcp.txt
var DHCPExport []byte

// SystemExport is the router's `uci export system` (hostname, timezone, NTP); mock mode
// loads it as the `system` config.
//
//go:embed testdata/uci-export-system.txt
var SystemExport []byte

// NetworkExport and FirewallExport are the router's `uci export network` / `firewall`
// (wwan uplink, guest AP, VPN zone); mock mode loads them so uplink features read a real layout.
//
//go:embed testdata/uci-export-network.txt
var NetworkExport []byte

//go:embed testdata/uci-export-firewall.txt
var FirewallExport []byte

// MWAN3Stock is the mwan3 package's own /etc/config/mwan3 (25.12.6 feed); mock mode loads
// it so failover plans against the stock sections a real install leaves.
//
//go:embed testdata/mwan3-stock-config.txt
var MWAN3Stock []byte
