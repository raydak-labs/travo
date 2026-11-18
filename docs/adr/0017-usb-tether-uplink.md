# 0017 USB tethering uplink

Status: accepted

## Context

A phone on USB is the third candidate uplink (`usbtether`, invariants §7). Device facts §12:
Android tethers over RNDIS or CDC-NCM, iPhones need `ipheth` plus `usbmuxd`, and the uplink is
identified by medium, not by IP. The old roadmap said the interface "joins the wan zone → a
direct firewall commit exception": old Travo bound the phone from a hotplug script and committed
firewall config without a confirm.

Kernel facts that shape the design (drivers/net/usb): `usbnet` names a link `eth%d` instead of
`usb%d` when the driver flags Ethernet framing and the MAC is universally administered, and
`ipheth` always uses `eth%d`. A name match (`usb0`) therefore misses real phones. sysfs
(`/sys/class/net/<if>/device/{subsystem,driver}`) names the bus and driver for every link
(capture: `testdata/device/axt1800-25.12.6/sysfs-net/`).

## Decision

- Detection is read-only and by medium: a link on bus `usb` whose driver is `rndis_host`,
  `cdc_ncm`, `cdc_ether` or `ipheth` is a tether link. The dashboard names the medium of the
  active uplink the same way (`ethernet`, `wifi`, `usb`, `virtual`).
- Drivers are installed only on request, per phone type, through the `PackageManager` port under
  guard `usbtether-install-in-progress` (kept on failure; the next request is the retry). Android:
  `kmod-usb-net-rndis`, `kmod-usb-net-cdc-ncm`. iOS: `kmod-usb-net-ipheth`, `usbmuxd` (enabled and
  started; its package ships a procd init and a USB hotplug script).
- The uplink is a user action, not hotplug: enable stages `network.usbtether` (proto dhcp, device
  = the chosen tether link, metric 10) and adds `usbtether` to the `wan` zone's network list in
  ONE `network` + `firewall` transaction with watchdog and user confirm (ADR 0005). Disable
  removes both. Travo owns only that interface and that list entry. A wan zone whose networks are
  an option string, no wan zone, or a non-interface `network.usbtether` → 409 before any write.
- Because the change goes through the apply flow, no direct firewall commit exception is added
  (invariants §7 keeps its list of named exceptions empty for this feature).
- Metric 10 keeps wan and wwan (netifd default metric 0) in front: the phone carries the kernel
  default route only while they have none. Ordering uplinks is failover's job (#34).
- Probe: carrier on the link and netifd `usbtether` up with a DHCP address within 30 s; disable
  proves the netifd interface gone or down.

## Consequences

- A phone that comes back under another link name (iPhone after `eth0` was taken, an Android
  phone with a different MAC policy) is not rebound automatically: status shows the bound device
  is absent and the user enables again. A hotplug rebinder would need its own guarded direct
  commit exception and an ADR.
- Kill switch rules already cover the wan zone, so a tethered uplink is blocked like wan/wwan
  while the kill switch is on (invariants §11).
- Tether link payloads (sysfs, `network.device status`, netifd status) are not captured yet; the
  hardware playbook captures them with a real Android phone and iPhone.
