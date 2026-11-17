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
