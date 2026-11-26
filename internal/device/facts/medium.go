package facts

import "github.com/blackdark-org/travo-greenfield/internal/device"

// Medium is the physical medium of a netdev, from sysfs, never from its name (device-facts §12).
type Medium string

const (
	MediumEthernet Medium = "ethernet"
	MediumWiFi     Medium = "wifi"
	MediumUSB      Medium = "usb"
	// MediumVirtual: no backing device (bridge, loopback, tunnel).
	MediumVirtual Medium = "virtual"
)

// ClassifyMedium: a USB bus is USB whatever the driver; `DEVTYPE=wlan` is WiFi; any other
// bus-backed netdev (platform on the AXT1800, pci elsewhere) is Ethernet.
func ClassifyMedium(l device.NetLink) Medium {
	switch {
	case l.Bus == "usb":
		return MediumUSB
	case l.DevType == "wlan":
		return MediumWiFi
	case l.Bus != "":
		return MediumEthernet
	default:
		return MediumVirtual
	}
}

// TetherKind is the USB tethering protocol a driver speaks.
type TetherKind string

const (
	TetherRNDIS  TetherKind = "rndis"  // Android, rndis_host
	TetherNCM    TetherKind = "ncm"    // Android, cdc_ncm
	TetherECM    TetherKind = "ecm"    // CDC Ethernet, cdc_ether
	TetherIPheth TetherKind = "ipheth" // iPhone Personal Hotspot
)

// tetherDrivers maps Linux driver names (drivers/net/usb) to the tether kind.
var tetherDrivers = map[string]TetherKind{
	"rndis_host": TetherRNDIS,
	"cdc_ncm":    TetherNCM,
	"cdc_ether":  TetherECM,
	"ipheth":     TetherIPheth,
}

// Tether reports the tether kind of a USB netdev; ok is false for anything else
// (a USB Ethernet adapter is a USB link but not a phone).
func Tether(l device.NetLink) (TetherKind, bool) {
	if l.Bus != "usb" {
		return "", false
	}
	k, ok := tetherDrivers[l.Driver]
	return k, ok
}
